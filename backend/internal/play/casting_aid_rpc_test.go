package play

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// TestCastAidOutsideCombatRaisesTheMaximumAndEndingGivesItBack: Aid cast outside a combat raises the
// maximum and the current hit points of each character it reaches by the spell's amount for the slot (5 at the
// 2nd level), as the same spell does in a combat; ending the cast takes the bonus off again and keeps the
// current hit points within the maximum. A second Aid keeps the stronger bonus; ending the stronger leaves the
// weaker. Finding: the outside path recorded Aid and never applied it (takeBackMaxHP was a stub).
func TestCastAidOutsideCombatRaisesTheMaximumAndEndingGivesItBack(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	targets := []*charactersv1.Character{a.toren, a.bri}
	max0 := map[string]int32{}
	hp0 := map[string]int32{}
	for _, c := range targets {
		v := a.vitals(t, c)
		max0[c.GetId()], hp0[c.GetId()] = v.GetHitPointsMax(), v.GetHitPointsCurrent()
	}
	weak := a.mustCastOut(t, a.bia, a.bri, aidSpell, slotOfLevel(2), false, targets).GetCast().GetId()
	for _, c := range targets {
		v := a.vitals(t, c)
		if v.GetHitPointsMax() != max0[c.GetId()]+5 || v.GetHitPointsCurrent() != hp0[c.GetId()]+5 || v.GetHitPointsMaxBonus() != 5 {
			t.Errorf("%s after Aid: max %d current %d bonus %d, want %d, %d, 5", c.GetName(), v.GetHitPointsMax(), v.GetHitPointsCurrent(), v.GetHitPointsMaxBonus(), max0[c.GetId()]+5, hp0[c.GetId()]+5)
		}
	}
	strong := a.mustCastOut(t, a.bia, a.bri, aidSpell, slotOfLevel(3), false, targets).GetCast().GetId()
	if v := a.vitals(t, a.toren); v.GetHitPointsMaxBonus() != 10 {
		t.Errorf("Toren after a stronger Aid: bonus %d, want 10 (the stronger applies, they do not add)", v.GetHitPointsMaxBonus())
	}
	if _, err := a.endSpell(t, a.bia, strong); err != nil {
		t.Fatalf("EndActiveSpell(stronger) error = %v", err)
	}
	if v := a.vitals(t, a.toren); v.GetHitPointsMaxBonus() != 5 || v.GetHitPointsMax() != max0[a.toren.GetId()]+5 {
		t.Errorf("Toren after the stronger Aid ended: bonus %d max %d, want the weaker Aid's 5 and %d", v.GetHitPointsMaxBonus(), v.GetHitPointsMax(), max0[a.toren.GetId()]+5)
	}
	if _, err := a.endSpell(t, a.bia, weak); err != nil {
		t.Fatalf("EndActiveSpell(weaker) error = %v", err)
	}
	for _, c := range targets {
		v := a.vitals(t, c)
		if v.GetHitPointsMax() != max0[c.GetId()] || v.GetHitPointsMaxBonus() != 0 || v.GetHitPointsCurrent() > v.GetHitPointsMax() {
			t.Errorf("%s after Aid ended: max %d bonus %d current %d, want max %d, no bonus, current within it", c.GetName(), v.GetHitPointsMax(), v.GetHitPointsMaxBonus(), v.GetHitPointsCurrent(), max0[c.GetId()])
		}
	}
}
