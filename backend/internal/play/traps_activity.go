package play

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
)

// What traps did outside a combat, as each member may read it (MR-035, RN-10): the
// firings and the searches of the open session that have no combat to put them in the
// log. The master gets all of it; a player only the lines of their own characters, with
// their own dice and "passou" or "falhou", never a DC, and never a trap their
// characters do not know (a trap that fired is public; a search line names only what
// that search found for its own character).

// activityLine is an event of the activity, with the firings that extend it merged in.
type activityLine struct {
	id        string
	at        time.Time
	character string // the searcher's character
	ev        actionEvent
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
	rows, err := s.queries.ListTrapEventsOfSession(ctx, session.ID)
	if err != nil {
		return nil, s.dbError(ctx, "list the trap events", err)
	}
	var lines []*activityLine
	byID := map[string]*activityLine{}
	for _, r := range rows {
		var ev actionEvent
		if err := json.Unmarshal(r.Payload, &ev); err != nil {
			continue // never: this package wrote it
		}
		if r.Kind == eventTrapTriggered && ev.Trap != nil && ev.Trap.ExtendsID != "" {
			if host := byID[ev.Trap.ExtendsID]; host != nil && host.ev.Trap != nil {
				host.ev.Trap.Caught = append(host.ev.Trap.Caught, ev.Trap.Caught...) // the creatures it added
			}
			continue
		}
		l := &activityLine{id: r.ID, at: r.CreatedAt, character: deref(r.CharacterID), ev: ev}
		if r.Kind == eventTrapSearched {
			l.ev.Key = ev.Key
		} else if ev.Trap == nil {
			continue
		}
		lines = append(lines, l)
		byID[r.ID] = l
	}

	// Who the caller plays: a player reads only the lines of their own characters.
	master := m.Role == authz.RoleMaster
	owned := map[string]bool{}
	if !master {
		party, err := s.roster.CombatParty(ctx, m.CampaignID)
		if err != nil {
			return nil, s.dbError(ctx, "read the party", err)
		}
		for _, c := range party {
			if c.PlayerUserID != "" && c.PlayerUserID == m.UserID {
				owned[c.ID] = true
			}
		}
	}

	var pointIDs, characterIDs, damageIDs, foundIDs []string
	for _, l := range lines {
		if l.ev.Trap != nil {
			pointIDs = append(pointIDs, l.ev.Trap.PointID)
			for _, cc := range l.ev.Trap.Caught {
				characterIDs = append(characterIDs, cc.Target)
				for _, d := range cc.Damages {
					if d.Pending != "" {
						damageIDs = append(damageIDs, d.Pending)
					}
				}
			}
		} else {
			characterIDs = append(characterIDs, l.character)
			foundIDs = append(foundIDs, l.ev.Found...)
		}
	}
	trapNames := map[string]string{}
	if s.traps != nil && len(pointIDs)+len(foundIDs) > 0 {
		all := slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(pointIDs), foundIDs...))))
		if trapNames, err = s.traps.TrapNames(ctx, m.CampaignID, all); err != nil {
			return nil, s.dbError(ctx, "read the traps' names", err)
		}
	}
	names := map[string]string{}
	if len(characterIDs) > 0 {
		chars, err := s.roster.SessionCharacters(ctx, m.CampaignID, slices.Compact(slices.Sorted(slices.Values(characterIDs))))
		if err != nil {
			return nil, s.dbError(ctx, "read the characters' names", err)
		}
		for _, c := range chars {
			names[c.ID] = c.Name
		}
	}
	statuses := map[string]playv1.PendingDamageStatus{}
	applied := map[string]int32{}
	if len(damageIDs) > 0 {
		rows, err := s.queries.ListTrapDamageStatuses(ctx, damageIDs)
		if err != nil {
			return nil, s.dbError(ctx, "read the trap damages", err)
		}
		for _, r := range rows {
			statuses[r.ID] = map[string]playv1.PendingDamageStatus{
				"rolled": playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED, "applied": playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED,
				"discarded": playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED,
			}[r.Status]
			if r.AppliedAmount != nil {
				applied[r.ID] = *r.AppliedAmount
			}
		}
	}

	res := &playv1.ListTrapActivityResponse{}
	for _, l := range lines {
		out := &playv1.TrapActivity{Id: l.id, At: timestamppb.New(l.at)}
		if tr := l.ev.Trap; tr != nil {
			shown := *tr
			shown.Caught = nil
			for _, cc := range tr.Caught {
				if master || owned[cc.Target] {
					shown.Caught = append(shown.Caught, cc)
				}
			}
			if len(shown.Caught) == 0 && !master {
				continue
			}
			out.Firing = firingProto(&shown, l.id, trapNames[tr.PointID], trapView{
				master: master,
				owns:   func(id string) bool { return owned[id] },
				label:  func(id string) string { return names[id] },
				status: func(id string) (playv1.PendingDamageStatus, bool) { st, ok := statuses[id]; return st, ok },
				amount: func(id string) (int32, bool) { a, ok := applied[id]; return a, ok },
			})
		} else {
			if !master && !owned[l.character] {
				continue
			}
			sr := &playv1.TrapSearchResult{
				CharacterId: l.character, CharacterName: names[l.character], FoundPointIds: l.ev.Found,
				Roll: diceRoll(1, 20, []int32{l.ev.D20}, l.ev.Modifier, l.ev.Total, l.ev.Physical),
			}
			if l.ev.Key == searchPerception {
				sr.Skill = playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_PERCEPTION
			} else {
				sr.Skill = playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_INVESTIGATION
			}
			if l.ev.D20B != 0 {
				sr.SecondRoll = diceRoll(1, 20, []int32{l.ev.D20B}, l.ev.Modifier, l.ev.D20B+l.ev.Modifier, l.ev.Physical)
			}
			for _, id := range l.ev.Found {
				sr.FoundNames = append(sr.FoundNames, trapNames[id])
			}
			out.Search = sr
		}
		res.Activity = append(res.Activity, out)
	}
	return connect.NewResponse(res), nil
}
