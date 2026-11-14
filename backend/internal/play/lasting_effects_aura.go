package play

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// An aura is a lasting effect on its bearer that reaches the creatures within a radius of the bearer, on one side
// (self, allies or enemies), where they stand when the roll or the damage happens. The row is the bearer's like
// any other (the clock, the concentration, the undo, the card), and holds the mark of the aura (the modifier of
// kind aura) with the modifiers it gives. Nothing about who is inside is ever written: withAuras answers it from the
// map each time a roll or a damage reads the effects, so a creature that moves in or out changes the next roll and
// nothing else.

// auraMark is the aura of an effect row; false for a row that is not one.
func auraMark(st playdb.CombatantState) (rules.EffectModifier, bool) {
	if !isEffect(st) {
		return rules.EffectModifier{}, false
	}
	for _, m := range storedModifiers(st) {
		if m.Kind == rules.ModifierAura {
			return m, true
		}
	}
	return rules.EffectModifier{}, false
}

// auraReaches says the creature is one the aura of the bearer reaches: on the side it names, in the radius. The
// sides are the combat's (Side), as Pack Tactics reads them. Distance is the map's, from the bearer's square;
// without a square for either (no grid, or a creature not on the map) nobody is reached but the bearer (of a self or an
// allies aura), and the master judges (RN-25).
func auraReaches(a rules.EffectModifier, bearer, c playdb.Combatant) bool {
	if c.Dismissed {
		return false
	}
	switch a.Affects {
	case rules.AuraAffectsSelf:
		return c.ID == bearer.ID
	case rules.AuraAffectsAllies:
		// The bearer and the creatures on its side (SRD 5.1: "you and friendly creatures"), the bearer wherever it is.
		if c.ID == bearer.ID {
			return true
		}
		if c.Side != bearer.Side {
			return false
		}
	case rules.AuraAffectsEnemies:
		if c.Side == bearer.Side {
			return false
		}
	default:
		return false
	}
	d, ok := distanceFt(bearer, c)
	return ok && int(d) <= a.RadiusFt
}

// withAuras is the states of the combat as a roll reads them: the aura rows give, to each creature they reach, a
// copy of their modifiers (the mark taken off) as an effect on that creature, once for each source (two auras of
// the same spell reach a creature once). A combat with no aura answers the map it was given. v is who reads the
// roll: a player does not read the name of an aura whose bearer they do not see, nor of one the master keeps from
// the players or that is owner-only, on a creature that is not the bearer, which read "Outra fonte" (RN-10).
func withAuras(states map[string][]playdb.CombatantState, cs []playdb.Combatant, v *combatViewer, down map[string]bool) map[string][]playdb.CombatantState {
	auras := aurasOf(states, cs, down)
	if len(auras) == 0 {
		return states
	}
	out := make(map[string][]playdb.CombatantState, len(states))
	for id, rows := range states {
		out[id] = slices.Clone(rows)
	}
	reached := map[[2]string]bool{} // the creature and the source an aura of which already reached it
	for _, a := range auras {
		a.spread(cs, v, reached, out)
	}
	return out
}

// aura is an aura row on the creature that bears it, with the mark that says how far it goes and to whom.
type aura struct {
	row    playdb.CombatantState
	mark   rules.EffectModifier
	bearer playdb.Combatant
}

// aurasOf are the aura rows of the creatures still in the fight. down holds the bearers (by combatant) that are at 0 hit
// points: a feature's aura needs its bearer up, so it is decided here, when the aura is read, from the character's hit
// points and the combatant's conditions, whoever last changed them.
func aurasOf(states map[string][]playdb.CombatantState, cs []playdb.Combatant, down map[string]bool) []aura {
	var auras []aura
	for _, b := range cs {
		if b.Defeated || b.Dismissed {
			continue
		}
		for _, st := range states[b.ID] {
			// A feature's aura needs its bearer up: not at 0 hit points and not incapacitated (Aura of Protection).
			if isFeatureAura(st) && (!fighting(b) || down[b.ID]) {
				continue
			}
			if m, ok := auraMark(st); ok {
				auras = append(auras, aura{row: st, mark: m, bearer: b})
			}
		}
	}
	return auras
}

// spread adds to out the view of the aura on each creature it reaches, unless the source reached it already. The source
// of a spell is the spell (two casters' auras of it reach a creature once, as the rule says of one effect); that of a
// feature is the bearer's feature, so two Paladins each give theirs.
func (a aura) spread(cs []playdb.Combatant, v *combatViewer, reached map[[2]string]bool, out map[string][]playdb.CombatantState) {
	given := givenByAura(a.row)
	if len(given) == 0 {
		return
	}
	source := a.row.ID
	if a.row.SourceKey != nil {
		source = *a.row.SourceKey
	}
	if isFeatureAura(a.row) {
		source = a.bearer.ID + "|" + source // two bearers of one feature each give; one bearer's two effects give once
	}
	for _, c := range cs {
		if !auraReaches(a.mark, a.bearer, c) || reached[[2]string{c.ID, source}] {
			continue
		}
		reached[[2]string{c.ID, source}] = true
		view := a.row
		view.CombatantID, view.Modifiers = c.ID, given
		if c.ID != a.bearer.ID && (a.row.Audience == audienceOwner || !seenBy(v, a.bearer)) {
			view.PlayerVisible = false
		}
		out[c.ID] = append(out[c.ID], view)
	}
}

// seenBy says the reader sees the bearer. A roll that knows no reader (v nil) reads as a player who sees no NPC: one
// the master hides, or one in the fog (which player sees it is not for the roll to say), and only a player's
// character or creature is seen.
func seenBy(v *combatViewer, bearer playdb.Combatant) bool {
	if v == nil {
		return !bearer.Hidden && bearer.Kind != kindNPC
	}
	return v.sees(bearer)
}

// readStatesAt is readStates as a roll reads it: with the auras of the combat (withAuras). A combat with no aura
// reads one query, as readStates does.
func (s *Service) readStatesAt(ctx context.Context, tx pgx.Tx, encID string, v *combatViewer) (map[string][]playdb.CombatantState, error) {
	return s.statesAt(ctx, tx, s.queriesIn(tx), encID, v)
}

// statesAt is readStatesAt on the queries of a change (tx is the change's transaction, nil for a read).
func (s *Service) statesAt(ctx context.Context, tx pgx.Tx, q *playdb.Queries, encID string, v *combatViewer) (map[string][]playdb.CombatantState, error) {
	rows, err := q.ListCombatantStates(ctx, encID)
	if err != nil {
		return nil, fmt.Errorf("list the states: %w", err)
	}
	states := statesOf(rows)
	if !slices.ContainsFunc(rows, isMarked) {
		return states, nil
	}
	cs, err := q.ListCombatants(ctx, encID)
	if err != nil {
		return nil, fmt.Errorf("list the combatants: %w", err)
	}
	campaignID, err := q.GetEncounterCampaignID(ctx, encID)
	if err != nil {
		return nil, fmt.Errorf("find the campaign of the combat: %w", err)
	}
	down, err := s.downFeatureBearers(ctx, tx, campaignID, cs, rows)
	if err != nil {
		return nil, err
	}
	return withAuras(states, cs, v, down), nil
}

func isMarked(st playdb.CombatantState) bool { _, ok := auraMark(st); return ok }

// givenByAura is what the aura gives: the modifiers of its row but the mark.
func givenByAura(st playdb.CombatantState) []byte {
	mods := slices.DeleteFunc(storedModifiers(st), func(m rules.EffectModifier) bool { return m.Kind == rules.ModifierAura })
	if len(mods) == 0 {
		return nil
	}
	body, _ := json.Marshal(mods) // plain values: they cannot fail to encode
	return body
}
