package play

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// TestCastingOutside_TheReachFollowsTheSpellsTargetData: outside a combat a player's cast and the
// options' reach hint follow the same target data as a cast in one: Mass Cure Wounds (60 ft, each
// creature up to 30 ft farther) reaches 90 ft and not 95, and Vampiric Touch (range Self, reach
// 5 ft) hints only the creature within 5 ft (SRD 5.1, "Range"; the spell's text).
func TestCastingOutside_TheReachFollowsTheSpellsTargetData(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.castingHero(t, a.campaignID, "Pensantus", classLevel("class:wizard", 5, ""),
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, []string{fireBolt},
			[]string{vampiricTouch}, []string{vampiricTouch}, "")
		a.bri = a.bia.castingHero(t, a.campaignID, "Brisa", classLevel("class:cleric", 17, "subclass:life"),
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 14, Intelligence: 10, Wisdom: 20, Charisma: 10}, nil, nil,
			[]string{massCureWounds}, "")
	})
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE game_sessions SET current_map_id = $2 WHERE campaign_id = $1 AND ended_at IS NULL`, a.campaignID, a.mapID); err != nil {
		t.Fatal(err)
	}
	// Brisa on the first column, Toren 90 ft away, Pensantus 95 ft away and 5 ft from Toren.
	step := 10000 / gridColumns
	at := func(col int) int { return col*step + step/2 }
	a.h.placeToken(a.mapID, a.bri.GetId(), at(0), 5000)
	a.h.placeToken(a.mapID, a.toren.GetId(), at(18), 5000)
	a.h.placeToken(a.mapID, a.pens.GetId(), at(19), 5000)

	reach := func(caster string, who *playv1.GetCastOptionsResponse, spell string) map[string]*playv1.CastingReach {
		out := map[string]*playv1.CastingReach{}
		for _, s := range who.GetSpells() {
			if s.GetSpell().GetKey() == spell {
				for _, r := range s.GetReach() {
					out[r.GetCharacterId()] = r
				}
			}
		}
		if len(out) == 0 {
			t.Fatalf("%s: no reach listed for %s", caster, spell)
		}
		return out
	}
	o, err := a.casting(t, a.bia, a.bri)
	if err != nil {
		t.Fatal(err)
	}
	mass := reach("Brisa", o, massCureWounds)
	if r := mass[a.toren.GetId()]; !r.GetInRange() {
		t.Errorf("Mass Cure Wounds hints Toren at 90 ft out of range: %v", r)
	}
	if r := mass[a.pens.GetId()]; r.GetInRange() || r.GetReasonPt() != "Fora do alcance da magia (27 m)." {
		t.Errorf("Mass Cure Wounds hint for Pensantus at 95 ft = %v, want out of range at 27 m", r)
	}
	a.mustCastOut(t, a.bia, a.bri, massCureWounds, slotOfLevel(5), false, []*charactersv1.Character{a.toren})
	_, err = a.castOut(t, a.bia, a.bri, massCureWounds, slotOfLevel(5), false, []*charactersv1.Character{a.pens})
	b := wantCastingBlocked(t, "Mass Cure Wounds at 95 ft", err, playv1.CastingBlockedReason_CASTING_BLOCKED_REASON_TARGET_OUT_OF_REACH)
	if b.GetMissingFt() != 5 {
		t.Errorf("missing = %d ft, want 5", b.GetMissingFt())
	}

	o, err = a.casting(t, a.ana, a.pens)
	if err != nil {
		t.Fatal(err)
	}
	vamp := reach("Pensantus", o, vampiricTouch)
	if r := vamp[a.toren.GetId()]; !r.GetInRange() {
		t.Errorf("Vampiric Touch hints Toren at 5 ft out of range: %v", r)
	}
	if r := vamp[a.bri.GetId()]; r.GetInRange() || r.GetReasonPt() != "Fora do alcance da magia (1,5 m)." {
		t.Errorf("Vampiric Touch hint for Brisa at 95 ft = %v, want out of range at 1,5 m", r)
	}
}
