package play

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The combat highlights, "Destaques do combate" (MR-032, D8; question 64 of
// the progress doc, with its default). Like the combat log, they are worked
// out on every read from the combat's session_events, never stored: the
// history and the highlights cannot disagree.
//
// What counts is what really changed, per player's character:
//
//	damage dealt  the hit points an NPC lost to its hits, temporary ones
//	              included; damage past 0 does not count (a 28 rolled on a
//	              7-hit-point goblin is 7)
//	healing done  the hit points it gave back, to anyone, up to the maximum
//	damage taken  the hit points and temporary hit points it lost, from anyone;
//	              a hit at 0 hit points is a death save failure, no hit points
//	final blows   the NPCs its hit brought to 0 hit points
//	critical hits its critical hits, with weapons and spell attacks
//
// An action the master undid counts for nothing (the events the undo names are
// skipped), nor does damage nobody applied: a player's character's damage
// counts when the master applies it (damage_applied), not when it is rolled. The
// master's own hand on an NPC's hit points counts for nobody.
//
// RN-20: the highlights name only the players' characters. The NPCs, hidden
// or not, are numbers in the players' totals and appear nowhere else.

// characterTally is the numbers of one player's character.
type characterTally struct {
	characterID, name string
	userID            string // the player it belongs to, empty for none
	damage, healing   int32
	taken             int32
	finalBlows, crits int32
	// checksPassed and checksTried are the checks rolled outside combat, only
	// in scenes that showed their DC (the session summary, summary.go); a
	// combat's tally leaves them at 0.
	checksPassed, checksTried int32
	// treasurePO is the gold pieces it found in the session (MR-041), also
	// only in the session summary.
	treasurePO int32
}

// highlightKind is a category with the number it ranks.
type highlightKind struct {
	kind  playv1.HighlightKind
	value func(t *characterTally) int32
}

// highlightKinds is the categories of a combat, in the order the app shows
// them. The session summary adds one (sessionHighlightKinds).
var highlightKinds = []highlightKind{
	{playv1.HighlightKind_HIGHLIGHT_KIND_MOST_DAMAGE, func(t *characterTally) int32 { return t.damage }},
	{playv1.HighlightKind_HIGHLIGHT_KIND_MOST_HEALING, func(t *characterTally) int32 { return t.healing }},
	{playv1.HighlightKind_HIGHLIGHT_KIND_TANK, func(t *characterTally) int32 { return t.taken }},
	{playv1.HighlightKind_HIGHLIGHT_KIND_FINAL_BLOW, func(t *characterTally) int32 { return t.finalBlows }},
	{playv1.HighlightKind_HIGHLIGHT_KIND_CRITICAL_HITS, func(t *characterTally) int32 { return t.crits }},
}

// hpLost is the hit points, temporary ones included, a change took off.
func hpLost(before, after *hpState) int32 {
	if before == nil || after == nil {
		return 0
	}
	return max(before.HP+before.Temp-after.HP-after.Temp, 0)
}

// tallyHighlights folds a combat's events, oldest first, into the numbers of
// each player's character, in the order of the combatants.
func tallyHighlights(events []playdb.ListEncounterCombatEventsRow, combatants []playdb.Combatant) []*characterTally {
	byCombatant := map[string]*characterTally{}
	ally := map[string]bool{} // the party's side: a hit on it is friendly fire and counts nowhere
	var out []*characterTally
	for _, c := range combatants {
		ally[c.ID] = inParty(c)
		if c.Kind != kindPlayer {
			continue
		}
		t := &characterTally{characterID: c.CharacterID, name: c.Label}
		if c.UserID != nil {
			t.userID = *c.UserID
		}
		byCombatant[c.ID] = t
		out = append(out, t)
	}

	// An undo names the event it took back: the first pass finds them all.
	undone := map[string]bool{}
	for _, e := range events {
		if e.Kind != eventActionUndone {
			continue
		}
		if ev, err := readEvent(e.Payload); err == nil {
			undone[ev.Undone] = true
		}
	}

	for _, e := range events {
		if e.Kind == eventActionUndone || undone[e.ID] {
			continue
		}
		var ev actionEvent
		if err := json.Unmarshal(e.Payload, &ev); err != nil {
			continue // a payload of this module never fails to decode; skip what does
		}
		actor := byCombatant[ev.Actor] // nil: an NPC's action, or a combatant that left
		switch e.Kind {
		case eventAttackRolled:
			if actor != nil && ev.Outcome == outcomeCrit && !ally[ev.Target] {
				actor.crits++
			}
		case eventSpellCast:
			for _, h := range ev.Hits {
				if actor != nil && h.Outcome == outcomeCrit && !ally[h.Target] {
					actor.crits++
				}
			}
		case eventActionTaken:
			// Retomar o fôlego and the like: a heal of the actor's own.
			if actor != nil && ev.Heal {
				actor.healing += ev.Amount
			}
		case eventDamageRolled:
			hits := ev.Settled
			if len(hits) == 0 {
				hits = []damageHit{{Pending: ev.Pending, Target: ev.Target, Amount: ev.Amount, Applied: ev.Applied, Before: ev.Before, After: ev.After}}
			}
			for _, h := range hits {
				switch {
				case actor == nil || !h.Applied:
					// Not a player's, or a player's character's damage still waiting
					// for the master (damage_applied counts it).
				case ev.Heal:
					actor.healing += h.Amount // what was regained, up to the maximum
				case ally[h.Target]:
					// Friendly fire (a hit on a creature of the party) counts nowhere.
				case h.Before != nil && h.After != nil:
					// Only an NPC takes a damage at once: this is damage dealt to one.
					actor.damage += hpLost(h.Before, h.After)
					if h.After.Defeated && !h.Before.Defeated {
						actor.finalBlows++
					}
				}
			}
		case eventDamageApplied:
			// The master applied a damage to a player's character: it is the target
			// that took it, whoever dealt it.
			if t := byCombatant[ev.Target]; t != nil {
				t.taken += hpLost(ev.Before, ev.After)
			}
		}
	}
	return out
}

// highlightCategories names the winner of each category of a combat: the most
// of it, with everyone tied. A category where nobody has anything is left out.
func highlightCategories(tallies []*characterTally) []*playv1.HighlightCategory {
	return categoriesOf(highlightKinds, tallies)
}

// categoriesOf does it for the given categories.
func categoriesOf(kinds []highlightKind, tallies []*characterTally) []*playv1.HighlightCategory {
	var out []*playv1.HighlightCategory
	for _, k := range kinds {
		var best int32
		for _, t := range tallies {
			best = max(best, k.value(t))
		}
		if best == 0 {
			continue
		}
		cat := &playv1.HighlightCategory{Kind: k.kind, Value: best}
		for _, t := range tallies {
			if k.value(t) == best {
				cat.Winners = append(cat.Winners, &playv1.HighlightWinner{CharacterId: t.characterID, Name: t.name})
			}
		}
		out = append(out, cat)
	}
	return out
}

// GetCombatHighlights implements playv1connect.CombatServiceHandler.
func (s *Service) GetCombatHighlights(
	ctx context.Context,
	req *connect.Request[playv1.GetCombatHighlightsRequest],
) (*connect.Response[playv1.GetCombatHighlightsResponse], error) {
	m, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId())
	if err != nil {
		return nil, err
	}
	encID, err := parseCombatID(req.Msg.GetEncounterId(), "encounter")
	if err != nil {
		return nil, err
	}
	// The encounter, its events and its combatants are one snapshot (audit D-03).
	var (
		events []playdb.ListEncounterCombatEventsRow
		cs     []playdb.Combatant
	)
	err = db.ReadTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		session, err := openSessionWith(ctx, q, m.CampaignID)
		if err != nil {
			return err
		}
		enc, err := encounterInSessionWith(ctx, q, session.ID, encID)
		if err != nil {
			return err
		}
		if enc.Status != statusEnded {
			return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_ENDED, "the combat has not ended")
		}
		if events, err = q.ListEncounterCombatEvents(ctx, &enc.ID); err != nil {
			return fmt.Errorf("list the combat's events: %w", err)
		}
		if cs, err = q.ListCombatants(ctx, enc.ID); err != nil {
			return fmt.Errorf("list the combatants: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, s.dbError(ctx, "read the combat's highlights", err)
	}
	tallies := tallyHighlights(events, cs)

	res := &playv1.GetCombatHighlightsResponse{Categories: highlightCategories(tallies)}
	for _, t := range tallies {
		// The master gets every row; a player only their own character's (RN-20),
		// zeros included, so "Seu resultado" is whole.
		if m.Role == authz.RoleMaster || (t.userID != "" && t.userID == m.UserID) {
			res.Characters = append(res.Characters, &playv1.CharacterHighlights{
				CharacterId: t.characterID, Name: t.name,
				DamageDealt: t.damage, HealingDone: t.healing, DamageTaken: t.taken, FinalBlows: t.finalBlows, CriticalHits: t.crits,
			})
		}
	}
	return connect.NewResponse(res), nil
}
