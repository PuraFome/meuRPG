package maps

import (
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

type namedRules struct{ noRules }

func (namedRules) NamePT(string) string { return "Perfurante" }

// The SRD's severity table gives a deadly trap at levels 17 to 20 24d10, so
// the master can save the suggested damage; a 25th die is still refused.
func TestTrapDamageAcceptsUpTo24Dice(t *testing.T) {
	t.Parallel()
	s := &Service{rules: namedRules{}}
	for dice, ok := range map[string]bool{"24d10": true, "25d10": false} {
		_, err := s.cleanDamage("damage[0]", &rulesv1.TrapDamage{Dice: dice, DamageTypeKey: "damage-type:piercing"})
		if (err == nil) != ok {
			t.Errorf("cleanDamage(%s) error = %v, accepted should be %v", dice, err, ok)
		}
	}
}
