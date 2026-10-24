package play

import (
	"fmt"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A monk's unarmed strike is magical from level 6 (Ki-Empowered Strikes): the grick, which
// resists nonmagical bludgeoning, piercing and slashing, takes all of it from a monk 6 and
// half from a monk 5, and only the master reads the step.
func TestKiEmpoweredStrikesOvercomeNonmagicalResistance(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		level int32
		hp    int32
		steps int
	}{{5, 27 - 4, 1}, {6, 27 - 9, 0}} {
		t.Run(fmt.Sprintf("monk %d", tc.level), func(t *testing.T) {
			t.Parallel()
			a, e, grick := partsFight(t, func(a *armed) {
				a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:monk", "race:human", tc.level, abilities(10, 16, 14, 10, 14), nil, nil)
			}, "monster:grick")
			hit := a.mustAttack(t, a.caio, e, "Toren", unarmed, grick, d20(15))
			a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), typedDamage(6))
			var seen bool
			for _, r := range a.log(t, a.master, e).GetRounds() {
				for _, en := range r.GetEntries() {
					if en.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK {
						continue
					}
					seen = true
					if got := len(en.GetDamage().GetSteps()); got != tc.steps {
						t.Errorf("the master reads %v, want %d step(s)", en.GetDamage().GetSteps(), tc.steps)
					}
				}
			}
			if !seen {
				t.Error("the master reads no attack line")
			}
			if hp, _, _ := a.hp(t, grick); hp != tc.hp {
				t.Errorf("the grick has %d hit points, want %d", hp, tc.hp)
			}
		})
	}
}
