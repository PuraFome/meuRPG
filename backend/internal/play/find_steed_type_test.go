package play

import "testing"

// The type the player picked for a steed (celestial, fey or fiend) is the type the combat reads of it: Divine Smite, Turn Undead and the
// other type checks read Traits.CreatureType. The stat block's own type (beast) stays what everyone reads on the creature's sheet.

func TestFindSteedCombatReadsTheStoredType(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"celestial", "fey", "fiend"} {
		a := newRiders(t)
		steed := a.mustCastSteed(t, warhorse, kind, "Trovão")
		sheet, ok, err := a.h.svc.roster.CreatureSheet(t.Context(), nil, a.campaignID, steed.GetId(), warhorse, "full")
		if err != nil || !ok {
			t.Fatalf("CreatureSheet(%s) = %v, %v", kind, ok, err)
		}
		if got := sheet.Traits.CreatureType; got != kind {
			t.Errorf("the combat reads the %s steed as %q, want %q", kind, got, kind)
		}
	}
}
