package play

import (
	"context"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// An undo writes the "before" of an action back as absolute numbers, so it would erase a change
// made after the action by something that left no event to close the undo chain. The action's
// event keeps the hit points it left (Holds), and the undo checks them before it writes anything.

// same says the two hit point states are the same numbers and the same Wild Shape form.
func (h hpState) same(o hpState) bool {
	if h.HP != o.HP || h.Temp != o.Temp || h.Defeated != o.Defeated {
		return false
	}
	if (h.Shape == nil) != (o.Shape == nil) {
		return false
	}
	return h.Shape == nil || *h.Shape == *o.Shape
}

// hpSnapshot reads the hit points of every combatant of the combat: its own for an NPC or a
// creature, its character's vitals and defeat for a player. A character that cannot be read (dead, gone)
// is left out. The second map has the maximum hit points of the characters.
func (s *Service) hpSnapshot(ctx context.Context, c *combatTx) (map[string]hpState, map[string]int32, error) {
	cs, err := listCombatants(ctx, c)
	if err != nil {
		return nil, nil, err
	}
	out, maxes := make(map[string]hpState, len(cs)), map[string]int32{}
	for _, who := range cs {
		if holdsHP(who) {
			out[who.ID] = hpOf(who)
			continue
		}
		if who.CharacterID == "" {
			continue
		}
		v, err := s.vitals.GetVitalsTx(ctx, c.tx, c.session.CampaignID, who.CharacterID)
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				continue
			}
			return nil, nil, err
		}
		st := hpStateOf(v)
		st.Defeated = who.Defeated
		out[who.ID], maxes[who.ID] = st, v.GetHitPointsMax()
	}
	return out, maxes, nil
}

// withHolds keeps in the event of an undoable change the hit points it left on each combatant
// whose hit points it changed. An action that left a combatant's numbers as they were holds none for
// it: its undo writes the same numbers back and a later change to them is not checked. Without a snapshot from before the change it keeps nothing.
func (s *Service) withHolds(ctx context.Context, c *combatTx, payload any) (any, error) {
	ev, ok := payload.(actionEvent)
	if !ok || c.hpBefore == nil {
		return payload, nil
	}
	after, _, err := s.hpSnapshot(ctx, c)
	if err != nil {
		return nil, err
	}
	for id, now := range after {
		if before, had := c.hpBefore[id]; had && !before.same(now) {
			if ev.Holds == nil {
				ev.Holds = map[string]hpState{}
			}
			ev.Holds[id] = now
		}
	}
	return ev, nil
}

// checkHolds refuses the undo of an action when a combatant's hit points are no longer the ones
// the action left: the undo would write the "before" over the change. Nothing has been written
// yet. The refusal names who changed and never the numbers. An event without Holds (older, or a
// change that moved no hit points) has nothing to check.
func (s *Service) checkHolds(ctx context.Context, c *combatTx, ev actionEvent) error {
	if len(ev.Holds) == 0 {
		return nil
	}
	now, maxes, err := s.hpSnapshot(ctx, c)
	if err != nil {
		return err
	}
	var changed []string
	for id, held := range ev.Holds {
		// A character's maximum may have dropped since (a level lost): the sheet cut the hit points,
		// nobody wrote them, and the undo cuts its "before" the same way (putVitals).
		if m, ok := maxes[id]; ok {
			held.HP = min(held.HP, m)
		}
		if cur, ok := now[id]; ok && !held.same(cur) {
			changed = append(changed, id)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	slices.Sort(changed)
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNDO_TARGET_CHANGED,
		"something changed hit points after the action: nothing was undone",
		func(b *playv1.EncounterBlocked) { b.CombatantIds = changed })
}
