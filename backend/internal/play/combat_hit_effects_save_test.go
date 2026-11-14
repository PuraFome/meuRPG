package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// RN-10: the save of a hit effect on an NPC. The audience of the spell decides who reads the spell; it does not decide
// whether the attacker's player learns what the attacker's own hit did. That player reads the outcome of the NPC's save
// (saved or failed, and the condition it gave), never the NPC's die or total; the master reads all; another player
// reads nothing of an effect that is for its owner alone.
func TestRN10_TheAttackersPlayerReadsTheOutcomeOfAnNPCsHitEffectSave(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, func(s *rulesv1.TableSpell) { s.Effect.Visibility = rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER },
		hitMod(trigEach, func(m *tableMod) { m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 4) // the weapon, the die of the effect, the save (a 4: it fails)
	master := a.hitRiderEntries(t, a.master, e)
	if len(master) != 1 || !master[0].GetHitRider().GetNumbers() || master[0].GetHitRider().GetD20() != 4 {
		t.Fatalf("the master reads %v, want the line with the NPC's die: the canary is unreachable", master)
	}
	lines := a.hitRiderEntries(t, a.ana, e)
	if len(lines) != 1 {
		t.Fatalf("the attacker's player reads %d save lines, want the outcome", len(lines))
	}
	r := lines[0].GetHitRider()
	if r.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED || r.GetConditionNamePt() == "" {
		t.Errorf("the attacker's player reads %v, want a failed save and the condition it gave", r)
	}
	if r.GetNumbers() || r.GetD20() != 0 || r.GetSaveTotal() != 0 || r.GetDc() != 0 || r.GetTreatedAs() != 0 {
		t.Errorf("the attacker's player reads the NPC's numbers: %v", r)
	}
	if lines[0].GetKey() != "" || lines[0].GetKeyNamePt() != otherSourceTx {
		t.Errorf("the attacker's player reads the spell %q %q of an effect its audience does not give, want Outra fonte", lines[0].GetKey(), lines[0].GetKeyNamePt())
	}
	for who, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		if got := a.hitRiderEntries(t, u, e); len(got) != 0 {
			t.Errorf("%s reads %d save lines of an effect that is for its owner", who, len(got))
		}
	}
}
