package characters

import (
	"regexp"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TestCastingTimePTLeavesNoEnglish: every SRD spell's casting time reads in
// Portuguese ("1 reação", not "1 reaction").
func TestCastingTimePTLeavesNoEnglish(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	english := regexp.MustCompile(`(?i)\b(action|bonus|reaction|minutes?|hours?)\b`)
	spells := srd.ListSpells(rules.SpellFilter{})
	if len(spells) != 319 {
		t.Fatalf("spells = %d, want 319", len(spells))
	}
	for _, e := range spells {
		d, ok := srd.SpellDetails(e.Key)
		if !ok {
			t.Fatalf("no details for %s", e.Key)
		}
		if got := castingTimePT(d.CastingTime); got == "" || english.MatchString(got) {
			t.Errorf("%s: casting time %q (raw %q) is not Portuguese", e.Key, got, d.CastingTime.Raw)
		}
	}
}
