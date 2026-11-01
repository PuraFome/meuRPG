package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A stat block's innate spells with uses a day reach its reaction stats, with the resource that holds the
// uses (the Couatl's Shield), and a creature without them has none.
func TestCreatureReactionStatsHoldsTheInnateUses(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	couatl := creatureReactionStats(srd, "monster:couatl")
	want := link.InnateUse{Spell: "spell:shield", Resource: "innate_3_day_shield", Max: 3}
	if got := couatl.Innate; !containsUse(got, want) {
		t.Errorf("couatl Innate = %v, want %+v among them", got, want)
	}
	if got := couatl.ResourceMax[want.Resource]; got != want.Max {
		t.Errorf("couatl ResourceMax[%s] = %d, want %d", want.Resource, got, want.Max)
	}
	if got := couatl.SlotsTotal; got != [9]int{} {
		t.Errorf("couatl SlotsTotal = %v, want no slots", got)
	}
	for _, key := range []string{"monster:goblin", "monster:mage"} {
		if got := creatureReactionStats(srd, key).Innate; len(got) != 0 {
			t.Errorf("%s Innate = %v, want none", key, got)
		}
	}
}

func containsUse(uses []link.InnateUse, want link.InnateUse) bool {
	for _, u := range uses {
		if u == want {
			return true
		}
	}
	return false
}
