package play

import (
	"fmt"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The flat bonus a table spell's roll_bonus effect adds to attack rolls, saving throws and ability
// checks counts wherever a die of Bênção would, and shows as a source of the roll like the bonus
// of a skill.

// rollBonusesOf lists the flat bonuses the effects on a combatant add to a roll of the kind
// (rules.RollAppliesAttack, ...Save or ...Check), one for each effect.
func rollBonusesOf(states map[string][]playdb.CombatantState, id, applies string) []effectBonus {
	var out []effectBonus
	for _, st := range effectsOn(states, id) {
		if n := combat.RollBonus(effectModifiers(st), applies); n != 0 {
			out = append(out, effectBonus{Key: conv.OrZero(st.SourceKey), Value: n, Hidden: !st.PlayerVisible})
		}
	}
	return out
}

// bonusTotal is what the bonuses add up to.
func bonusTotal(bonuses []effectBonus) int {
	n := 0
	for _, b := range bonuses {
		n += b.Value
	}
	return n
}

// withRollBonus is the saving throw with the flat bonus the effects on the combatant add to it.
func withRollBonus(save link.Save, states map[string][]playdb.CombatantState, id, applies string) link.Save {
	save.Bonus += bonusTotal(rollBonusesOf(states, id, applies))
	return save
}

// bonusSources writes the bonuses as sources of the roll ("Bênção +1"). What an effect the master
// hides gives reads "Outra fonte" to a player; the master reads the name.
func bonusSources(bonuses []effectBonus, names func(string) string, master bool) []*playv1.AdvantageSource {
	var out []*playv1.AdvantageSource
	for _, b := range bonuses {
		name := names(b.Key)
		if b.Hidden && !master {
			name = "Outra fonte"
		}
		out = append(out, &playv1.AdvantageSource{
			Kind: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_DIE, TextPt: fmt.Sprintf("%s %+d", name, b.Value),
		})
	}
	return out
}
