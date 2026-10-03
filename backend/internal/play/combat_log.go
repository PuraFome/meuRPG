package play

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The combat log, "Registro do combate" (D10, MR-012; the history screen
// ADR-0007 deferred). It is built on every read from the combat's
// session_events, never stored on its own, so the history and the log cannot
// disagree. Each entry is structured (kind, who, the roll, the outcome, the
// damage): the app writes the Portuguese sentence.
//
// What each viewer gets (RN-10, RN-20):
//
//	                    master                    player
//	an entry            all                       only those without a hidden combatant
//	                                              (when it happened, and now)
//	the dice            all                       their own character's only
//	hit points after    yes                       never
//	"hidden" mark       set on the ones the       never set
//	                    players do not get
//	undo                the last action's entry   never
//
// A combatant the master reveals later does not bring its old entries with
// it: each entry remembers, from the event, whether a hidden combatant was in
// it, and a player never gets that one.

// logEventLimit is how many of a combat's latest events the log reads. A
// variable only so a test can make a combat longer than it.
var logEventLimit int32 = 5000

// logEntry is an entry while it is being built: the event's numbers and who
// may see it.
type logEntry struct {
	id     string
	kind   playv1.CombatLogKind
	at     time.Time
	ev     actionEvent
	status playv1.PendingDamageStatus // the damage of an attack, once it hit
	dmg    *actionEvent               // the damage roll, once it was rolled
	// masterOnly is a line only the master gets, whatever the combatants.
	masterOnly bool
	// hosts are the events the entry shows: an attack's own and the ones of its
	// damage, so the entry of the last action is the one to undo.
	hosts []string
}

// ListCombatLog implements playv1connect.CombatServiceHandler.
func (s *Service) ListCombatLog(
	ctx context.Context,
	req *connect.Request[playv1.ListCombatLogRequest],
) (*connect.Response[playv1.ListCombatLogResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return nil, err
	}
	enc, err := s.queries.GetEncounterInSession(ctx, playdb.GetEncounterInSessionParams{GameSessionID: session.ID, ID: encID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("encounter not found"))
	}
	if err != nil {
		return nil, s.dbError(ctx, "find the encounter", err)
	}
	events, err := s.queries.ListEncounterEvents(ctx, playdb.ListEncounterEventsParams{EncounterID: &enc.ID, Limit: logEventLimit})
	if err != nil {
		return nil, s.dbError(ctx, "list the combat's events", err)
	}
	slices.Reverse(events) // oldest first, as buildLog reads them
	cs, err := s.queries.ListCombatants(ctx, enc.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the combatants", err)
	}
	v := viewerOf(m)
	entries := buildLog(events)

	res := &playv1.ListCombatLogResponse{}
	var lastID string
	if v.master {
		recent, err := s.queries.ListRecentSessionEvents(ctx, playdb.ListRecentSessionEventsParams{GameSessionID: session.ID, Limit: recentEvents})
		if err != nil {
			return nil, s.dbError(ctx, "read the latest events", err)
		}
		if last, ok := lastAction(recent, enc.ID); ok {
			lastID = last.ID
			res.UndoableEventId = last.ID
		}
	}

	names := &keyNames{s: s, campaignID: m.CampaignID, byCharacter: map[string]link.Sheet{}}
	byID := make(map[string]playdb.Combatant, len(cs))
	for _, c := range cs {
		byID[c.ID] = c
	}
	// Latest first, in groups by round.
	for _, e := range slices.Backward(entries) {
		out, ok := e.view(ctx, v, byID, names, lastID)
		if !ok {
			continue
		}
		if n := len(res.Rounds); n == 0 || res.Rounds[n-1].Round != out.Round {
			res.Rounds = append(res.Rounds, &playv1.CombatLogRound{Round: out.Round})
		}
		last := res.Rounds[len(res.Rounds)-1]
		last.Entries = append(last.Entries, out)
	}
	return connect.NewResponse(res), nil
}

// buildLog turns a combat's events, in order, into entries. An event an undo
// took back leaves no trace; the events of one attack (its roll, its damage,
// the master's apply or discard) merge into one entry.
func buildLog(events []playdb.ListEncounterEventsRow) []*logEntry {
	undone := map[string]bool{}
	for _, e := range events {
		if e.Kind != eventActionUndone {
			continue
		}
		if ev, err := readEvent(e.Payload); err == nil {
			undone[ev.Undone] = true
		}
	}
	var out []*logEntry
	byPending := map[string]*logEntry{}
	for _, e := range events {
		if undone[e.ID] || e.Kind == eventActionUndone {
			continue
		}
		ev, err := readEvent(e.Payload)
		if err != nil {
			continue // never: this package wrote it
		}
		if ev.Round == 0 {
			continue // the log starts when the combat begins: the setup is not in it
		}
		entry := &logEntry{id: e.ID, at: e.CreatedAt, ev: ev, hosts: []string{e.ID}}
		switch e.Kind {
		case eventCombatBegun:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_COMBAT_BEGUN
		case eventEncounterEnded:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_COMBAT_ENDED
		case eventCombatantMoved:
			if !ev.OnTurn || ev.DistanceFt == 0 {
				continue // placing a token is not a move of the fight
			}
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED
		case eventCombatantHiddenSet:
			entry.kind, entry.masterOnly = playv1.CombatLogKind_COMBAT_LOG_KIND_REVEAL_CHANGED, true
		case eventActionTaken:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION
		case eventHitPointsAdjusted:
			entry.kind, entry.masterOnly = playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_POINTS_ADJUSTED, true
		case eventAttackRolled:
			entry.kind = playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK
			if ev.Pending != "" {
				entry.status = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL
				byPending[ev.Pending] = entry
			}
		case eventDamageRolled, eventDamageApplied, eventDamageDiscarded:
			// The damage lands in the attack's entry.
			attack, ok := byPending[ev.Pending]
			if !ok {
				continue
			}
			attack.hosts = append(attack.hosts, e.ID)
			switch e.Kind {
			case eventDamageRolled:
				attack.dmg = &ev
				attack.status = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED
				if ev.Applied {
					attack.status = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED
				}
			case eventDamageApplied:
				attack.status = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED
				if attack.dmg != nil {
					attack.dmg.After = ev.After // the character's hit points after
				}
			case eventDamageDiscarded:
				attack.status = playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED
			}
			continue
		default:
			continue // initiative, turns, reinforcements: not lines of the log
		}
		out = append(out, entry)
	}
	return out
}

// view builds the entry the viewer gets, and false when they do not get it.
func (e *logEntry) view(ctx context.Context, v combatViewer, byID map[string]playdb.Combatant, names *keyNames, lastID string) (*playv1.CombatLogEntry, bool) {
	actor, target := byID[e.ev.Actor], byID[e.ev.Target]
	if e.kind == playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_POINTS_ADJUSTED {
		actor, target = playdb.Combatant{}, byID[e.ev.Actor] // the affected one is the target
	}
	// What a player may see: nothing with a hidden combatant in it, when it
	// happened or now (a combatant the master hides again takes its lines back).
	// A combatant that left the combat is unknown now, and an entry about it
	// would be anonymous: a player does not get it either.
	_, actorKnown := byID[e.ev.Actor]
	_, targetKnown := byID[e.ev.Target]
	known := (e.ev.Actor == "" || actorKnown) && (e.ev.Target == "" || targetKnown)
	visible := !e.masterOnly && !e.ev.Secret && !actor.Hidden && !target.Hidden && known
	if !v.master && !visible {
		return nil, false
	}

	// The dice are the master's and the owner's: the master's rolls are not the
	// players'.
	dice := v.master || v.owns(actor)
	out := &playv1.CombatLogEntry{
		Id: e.id, Kind: e.kind, At: timestamppb.New(e.at), Round: e.ev.Round,
		ActorId: actor.ID, ActorLabel: actor.Label, TargetId: target.ID, TargetLabel: target.Label,
		Key: e.ev.Key,
	}
	if v.master {
		out.Hidden = !visible
		out.Undoable = slices.Contains(e.hosts, lastID)
	}
	switch e.kind {
	case playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK, playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION:
		out.KeyNamePt = names.of(ctx, actor, e.ev.Key)
	}
	switch e.kind {
	case playv1.CombatLogKind_COMBAT_LOG_KIND_MOVED:
		out.DistanceFt = e.ev.DistanceFt
	case playv1.CombatLogKind_COMBAT_LOG_KIND_REVEAL_CHANGED:
		out.NowHidden = e.ev.NowHidden
		out.ActorId, out.ActorLabel = "", ""
		out.TargetId, out.TargetLabel = e.ev.Actor, byID[e.ev.Actor].Label
	case playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_POINTS_ADJUSTED:
		out.HitPointsDelta = e.ev.Delta
		if e.ev.After != nil {
			out.HitPointsAfter = &e.ev.After.HP
		}
	case playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK:
		out.Outcome = outcomeToProto[e.ev.Outcome]
		if dice {
			out.AttackRoll = diceRoll(1, 20, []int32{e.ev.D20}, e.ev.Modifier, e.ev.Total, e.ev.Physical)
		}
		if e.ev.Pending != "" {
			out.Damage = e.damage(dice, v.master, out)
		}
	}
	return out, true
}

// damage is the damage of an attack as the viewer gets it. The target's hit
// points after it are the master's alone, set on the entry.
func (e *logEntry) damage(dice, master bool, out *playv1.CombatLogEntry) *playv1.CombatLogDamage {
	d := &playv1.CombatLogDamage{Status: e.status}
	if e.dmg == nil {
		return d // the attack hit and its damage is not rolled yet
	}
	d.Amount, d.DamageTypeKey, d.DamageTypePt = e.dmg.Amount, e.dmg.DamageType, damageTypePT[e.dmg.DamageType]
	if dice {
		d.Roll = diceRoll(e.dmg.DiceCount, e.dmg.DiceSides, e.dmg.Faces, e.dmg.Modifier, e.dmg.Amount, e.dmg.Physical)
	}
	if after := e.dmg.After; after != nil && e.status == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED {
		d.TargetDefeated = after.Defeated
		d.TargetDown = !after.Defeated && after.HP == 0
		if master {
			out.HitPointsAfter = &after.HP
		}
	}
	return d
}

// keyNames finds the Portuguese names of the attacks and actions an entry
// mentions, from the actor's sheet, read once for each character.
type keyNames struct {
	s           *Service
	campaignID  string
	byCharacter map[string]link.Sheet
}

// of returns the name of the key on the combatant's sheet, or "" when the
// combatant left the combat or the sheet no longer has it.
func (n *keyNames) of(ctx context.Context, c playdb.Combatant, key string) string {
	if c.CharacterID == "" || key == "" {
		return ""
	}
	sheet, ok := n.byCharacter[c.CharacterID]
	if !ok {
		var err error
		if sheet, err = n.s.roster.CombatSheet(ctx, n.campaignID, c.CharacterID); err != nil {
			n.s.logger.WarnContext(ctx, "play: cannot read a sheet for the combat log") // no names or IDs in logs
		}
		n.byCharacter[c.CharacterID] = sheet
	}
	for _, a := range sheet.Attacks {
		if a.Key == key {
			return a.Name
		}
	}
	for _, a := range sheet.Actions {
		if a.Key == key {
			return a.Name
		}
	}
	return ""
}
