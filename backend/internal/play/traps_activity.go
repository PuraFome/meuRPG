package play

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// What traps did outside a combat, as each member may read it (MR-035, RN-10): the
// firings and the searches of the open session that have no combat to put them in the
// log. The master gets all of it; a player only the lines of their own characters, with
// their own dice and "passou" or "falhou", never a DC, and never a trap their
// characters do not know (a trap that fired is public; a search line names only what
// that search found for its own character).

// creatureLabels names the creatures a firing caught outside a combat (their owner's free text, read now and never
// kept in the event: docs/privacy.md), by creature ID.
func (s *Service) creatureLabels(ctx context.Context, campaignID string, caught []trapCaughtEvent) (map[string]string, error) {
	var owners []string
	for _, cc := range caught {
		if creatureOf(cc) {
			owners = append(owners, cc.Character)
		}
	}
	out := map[string]string{}
	if len(owners) == 0 {
		return out, nil
	}
	var found []link.Creature
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		found, err = s.roster.CharacterCreatures(ctx, tx, campaignID, slices.Compact(slices.Sorted(slices.Values(owners))))
		return err
	})
	for _, c := range found {
		out[c.ID] = c.Name
	}
	return out, err
}

// creatureOf says a caught target is a creature of a character (a creature token
// dropped into a trap outside a combat), not a character: it is caught under its
// owner's character ID.
func creatureOf(cc trapCaughtEvent) bool { return cc.Character != "" && cc.Character != cc.Target }

// activityLine is an event of the activity, with the firings that extend it merged in.
type activityLine struct {
	id        string
	at        time.Time
	character string // the searcher's character
	ev        actionEvent
	notice    *noticeLine // set for a passive notice
}

// noticeLine is a passive notice: the trap and the characters that noticed it.
type noticeLine struct {
	point      string
	characters []string
}

// ListTrapActivity implements playv1connect.PlayServiceHandler.
func (s *Service) ListTrapActivity(
	ctx context.Context,
	req *connect.Request[playv1.ListTrapActivityRequest],
) (*connect.Response[playv1.ListTrapActivityResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	lines, err := s.activityLines(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	v, err := s.activityViewer(ctx, m)
	if err != nil {
		return nil, err
	}
	facts, err := s.resolveActivity(ctx, m.CampaignID, lines)
	if err != nil {
		return nil, err
	}
	facts.campaignID = m.CampaignID
	return connect.NewResponse(&playv1.ListTrapActivityResponse{Activity: s.buildActivity(ctx, lines, v, facts)}), nil
}

// activityLines reads the trap events of the open session, oldest first, as lines: the
// firings that extend another are merged into it, and what cannot be read is left out.
func (s *Service) activityLines(ctx context.Context, sessionID string) ([]*activityLine, error) {
	rows, err := s.queries.ListTrapEventsOfSession(ctx, sessionID)
	if err != nil {
		return nil, s.dbError(ctx, "list the trap events", err)
	}
	slices.Reverse(rows) // the query returns the newest 500, newest first
	var lines []*activityLine
	byID := map[string]*activityLine{}
	for _, r := range rows {
		l := activityLineOf(r, byID)
		if l == nil {
			continue
		}
		lines = append(lines, l)
		byID[r.ID] = l
	}
	return lines, nil
}

// activityLineOf turns one event into a line, or nil when it adds none: an unreadable one, one
// of no use to the activity, or a firing that extends another (merged into it through byID).
func activityLineOf(r playdb.ListTrapEventsOfSessionRow, byID map[string]*activityLine) *activityLine {
	var ev actionEvent
	if err := json.Unmarshal(r.Payload, &ev); err != nil {
		return nil // never: this package wrote it
	}
	if r.Kind == eventTrapTriggered && ev.Trap != nil && ev.Trap.ExtendsID != "" {
		if host := byID[ev.Trap.ExtendsID]; host != nil && host.ev.Trap != nil {
			host.ev.Trap.Caught = append(host.ev.Trap.Caught, ev.Trap.Caught...) // the creatures it added
		}
		return nil
	}
	l := &activityLine{id: r.ID, at: r.CreatedAt, character: deref(r.CharacterID), ev: ev}
	switch {
	case r.Kind == eventTrapNoticed:
		// The maps module wrote it: the trap and the characters that noticed it.
		var n struct {
			PointID      string   `json:"point_id"`
			CharacterIDs []string `json:"character_ids"`
		}
		if err := json.Unmarshal(r.Payload, &n); err != nil || len(n.CharacterIDs) == 0 {
			return nil // never: the maps module wrote it
		}
		l.notice = &noticeLine{point: n.PointID, characters: n.CharacterIDs}
	case r.Kind == eventTrapSearched:
		l.ev.Key = ev.Key
	case ev.Trap == nil:
		return nil
	}
	return l
}

// activityViewer says who the caller plays: a player reads only the lines of their own
// characters.
func (s *Service) activityViewer(ctx context.Context, m authz.Membership) (activityViewer, error) {
	v := activityViewer{master: m.Role == authz.RoleMaster, owned: map[string]bool{}}
	if v.master {
		return v, nil
	}
	party, err := s.roster.CombatParty(ctx, nil, m.CampaignID)
	if err != nil {
		return v, s.dbError(ctx, "read the party", err)
	}
	for _, c := range party {
		if c.PlayerUserID != "" && c.PlayerUserID == m.UserID {
			v.owned[c.ID] = true
		}
	}
	return v, nil
}

// activityViewer is the caller of the activity: the master, or the player and their characters.
type activityViewer struct {
	master bool
	owned  map[string]bool
}

// activityFacts is what the lines name by ID and the activity reads at the time of the call.
type activityFacts struct {
	campaignID    string
	trapNames     map[string]string
	names         map[string]string // the characters'
	creatureNames map[string]string
	statuses      map[string]playv1.PendingDamageStatus
	applied       map[string]int32
}

// activityRefs are the IDs the lines name, by what they name.
type activityRefs struct {
	points, characters, damages, found, owners []string
}

// refsOf lists what the lines name.
func refsOf(lines []*activityLine) activityRefs {
	var refs activityRefs
	for _, l := range lines {
		switch {
		case l.notice != nil:
			refs.points = append(refs.points, l.notice.point)
			refs.characters = append(refs.characters, l.notice.characters...)
		case l.ev.Trap != nil:
			refs.addFiring(l.ev.Trap)
		default:
			refs.characters = append(refs.characters, l.character)
			refs.found = append(refs.found, l.ev.Found...)
		}
	}
	return refs
}

// addFiring lists what a firing names.
func (refs *activityRefs) addFiring(tr *trapFireEvent) {
	refs.points = append(refs.points, tr.PointID)
	for _, cc := range tr.Caught {
		if creatureOf(cc) {
			refs.owners = append(refs.owners, cc.Character) // its name is its owner's to give: read below
		} else {
			refs.characters = append(refs.characters, cc.Target)
		}
		for _, d := range cc.Damages {
			if d.Pending != "" {
				refs.damages = append(refs.damages, d.Pending)
			}
		}
	}
}

// resolveActivity reads the names and the damage statuses the lines name.
func (s *Service) resolveActivity(ctx context.Context, campaignID string, lines []*activityLine) (activityFacts, error) {
	refs := refsOf(lines)
	f := activityFacts{trapNames: map[string]string{}, names: map[string]string{}, creatureNames: map[string]string{}}
	var err error
	if s.traps != nil && len(refs.points)+len(refs.found) > 0 {
		all := slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(refs.points), refs.found...))))
		if f.trapNames, err = s.traps.TrapNames(ctx, campaignID, all); err != nil {
			return f, s.dbError(ctx, "read the traps' names", err)
		}
	}
	if len(refs.characters) > 0 {
		chars, err := s.roster.SessionCharacters(ctx, nil, campaignID, slices.Compact(slices.Sorted(slices.Values(refs.characters))))
		if err != nil {
			return f, s.dbError(ctx, "read the characters' names", err)
		}
		for _, c := range chars {
			f.names[c.ID] = c.Name
		}
	}
	if err := s.readCreatureNames(ctx, campaignID, refs.owners, f.creatureNames); err != nil {
		return f, err
	}
	if f.statuses, f.applied, err = s.readTrapDamageStatuses(ctx, refs.damages); err != nil {
		return f, err
	}
	return f, nil
}

// readCreatureNames fills out with the names of the creatures of the owners. A creature dropped
// into a trap is caught under its owner's character: its name is the owner's free text, read now
// and never kept in the event (docs/privacy.md).
func (s *Service) readCreatureNames(ctx context.Context, campaignID string, ownerIDs []string, out map[string]string) error {
	if len(ownerIDs) == 0 {
		return nil
	}
	var found []link.Creature
	err := db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		found, err = s.roster.CharacterCreatures(ctx, tx, campaignID, slices.Compact(slices.Sorted(slices.Values(ownerIDs))))
		return err
	})
	if err != nil {
		return s.dbError(ctx, "read the creatures' names", err)
	}
	for _, c := range found {
		out[c.ID] = c.Name
	}
	return nil
}

// damageStatusProto maps the status a trap damage is stored with.
var damageStatusProto = map[string]playv1.PendingDamageStatus{
	"rolled": playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED, "applied": playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED,
	"discarded": playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED,
}

// readTrapDamageStatuses reads where the trap damages are now and what the master applied.
func (s *Service) readTrapDamageStatuses(ctx context.Context, damageIDs []string) (map[string]playv1.PendingDamageStatus, map[string]int32, error) {
	statuses := map[string]playv1.PendingDamageStatus{}
	applied := map[string]int32{}
	if len(damageIDs) == 0 {
		return statuses, applied, nil
	}
	rows, err := s.queries.ListTrapDamageStatuses(ctx, damageIDs)
	if err != nil {
		return nil, nil, s.dbError(ctx, "read the trap damages", err)
	}
	for _, r := range rows {
		statuses[r.ID] = damageStatusProto[r.Status]
		if r.AppliedAmount != nil {
			applied[r.ID] = *r.AppliedAmount
		}
	}
	return statuses, applied, nil
}

// buildActivity writes the lines the viewer may read, in order.
func (s *Service) buildActivity(ctx context.Context, lines []*activityLine, v activityViewer, f activityFacts) []*playv1.TrapActivity {
	var out []*playv1.TrapActivity
	for _, l := range lines {
		line := &playv1.TrapActivity{Id: l.id, At: timestamppb.New(l.at)}
		var ok bool
		switch {
		case l.notice != nil:
			line.Notice, ok = noticeActivity(l.notice, v, f)
		case l.ev.Trap != nil:
			line.Firing, ok = s.firingActivity(ctx, l, v, f)
		default:
			line.Search, ok = searchActivity(l, v, f)
		}
		if ok {
			out = append(out, line)
		}
	}
	return out
}

// noticeActivity is a passive notice for the viewer. RN-10: a player reads only the notices of
// their own characters; the line names the trap because they know it now, and carries no DC.
func noticeActivity(nl *noticeLine, v activityViewer, f activityFacts) (*playv1.TrapNotice, bool) {
	notice := &playv1.TrapNotice{PointId: nl.point, TrapName: f.trapNames[nl.point]}
	for _, id := range nl.characters {
		if v.master || v.owned[id] {
			notice.CharacterIds = append(notice.CharacterIds, id)
			notice.CharacterNames = append(notice.CharacterNames, f.names[id])
		}
	}
	return notice, len(notice.CharacterIds) > 0
}

// firingActivity is a firing as the viewer reads it: the master all of it, a player only what
// caught their characters and their characters' creatures.
func (s *Service) firingActivity(ctx context.Context, l *activityLine, v activityViewer, f activityFacts) (*playv1.TrapFiring, bool) {
	tr := l.ev.Trap
	shown := *tr
	shown.Caught = nil
	ownedCreatures := map[string]bool{} // the creatures of the characters the viewer plays
	for _, cc := range tr.Caught {
		if creatureOf(cc) && v.owned[cc.Character] {
			ownedCreatures[cc.Target] = true
		}
	}
	for _, cc := range tr.Caught {
		if v.master || v.owned[cc.Target] || (creatureOf(cc) && v.owned[cc.Character]) {
			shown.Caught = append(shown.Caught, cc)
		}
	}
	if len(shown.Caught) == 0 && !v.master {
		return nil, false
	}
	return firingProto(&shown, l.id, f.trapNames[tr.PointID], trapView{
		master: v.master,
		names:  s.namesFor(ctx, f.campaignID),
		owns:   func(id string) bool { return v.owned[id] || ownedCreatures[id] },
		label: func(id string) string {
			if n, ok := f.creatureNames[id]; ok {
				return n
			}
			return f.names[id]
		},
		status: func(id string) (playv1.PendingDamageStatus, bool) { st, ok := f.statuses[id]; return st, ok },
		amount: func(id string) (int32, bool) { a, ok := f.applied[id]; return a, ok },
	}), true
}

// searchActivity is a search as the viewer reads it: the master's, or the player's own.
func searchActivity(l *activityLine, v activityViewer, f activityFacts) (*playv1.TrapSearchResult, bool) {
	if !v.master && !v.owned[l.character] {
		return nil, false
	}
	sr := &playv1.TrapSearchResult{
		CharacterId: l.character, CharacterName: f.names[l.character], FoundPointIds: l.ev.Found,
		Roll: checkRoll(l.ev.D20, l.ev.Modifier, l.ev.Physical, l.ev.ReliableTalent),
	}
	switch l.ev.Key {
	case searchPerception:
		sr.Skill = playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_PERCEPTION
	case searchInvestigation:
		sr.Skill = playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_INVESTIGATION
	default:
		sr.Skill, sr.OtherSkillKey = playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_OTHER, l.ev.Key
	}
	if l.ev.D20B != 0 {
		sr.SecondRoll = checkRoll(l.ev.D20B, l.ev.Modifier, l.ev.Physical, l.ev.ReliableTalent)
	}
	// The d20 the halfling's Lucky rolled again: the first or the second, each told here as a
	// roll of its own ("Sortudo: 1 → 14").
	if r := l.ev.Lucky; r != nil {
		one := (&luckyReroll{From: r.From, To: r.To}).proto()
		if r.Index == 0 {
			sr.Roll.Rerolled = one
		} else if sr.SecondRoll != nil {
			sr.SecondRoll.Rerolled = one
		}
	}
	for _, id := range l.ev.Found {
		sr.FoundNames = append(sr.FoundNames, f.trapNames[id])
	}
	return sr, true
}
