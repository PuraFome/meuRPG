package characters

import (
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TestStoredEffectFieldsReachTheEngine: the stored fields of a costed action and of a
// hook become the engine's effect, and the engine's action reaches the sheet with its
// cost range and its heal.
func TestStoredEffectFieldsReachTheEngine(t *testing.T) {
	t.Parallel()
	got := tableEffectsOf([]*rulesv1.TableEffect{
		{Type: "grant_action", Economy: "action", Resource: "reserva_de_teste", Cost: "1", CostMax: "3", Heal: "1d6", ValuePerPoint: "1d6", TempHp: "level()", Ability: "wis"},
		{Type: "on_event", Event: "death_save_failed", Resource: "reserva_de_teste", Cost: "1", HealPercentMax: 50, Restore: "outra", Value: "2"},
	})
	want := []rules.Effect{
		{Type: "grant_action", Economy: "action", Resource: "reserva_de_teste", Cost: "1", CostMax: "3", Heal: "1d6", ValuePerPoint: "1d6", TempHP: "level()", Ability: "wis"},
		{Type: "on_event", Event: "death_save_failed", Resource: "reserva_de_teste", Cost: "1", HealPercentMax: 50, Restore: "outra", Value: "2"},
	}
	for i := range want {
		if got[i].Type != want[i].Type || got[i].Cost != want[i].Cost || got[i].CostMax != want[i].CostMax || got[i].Heal != want[i].Heal ||
			got[i].ValuePerPoint != want[i].ValuePerPoint || got[i].TempHP != want[i].TempHP || got[i].Event != want[i].Event ||
			got[i].Restore != want[i].Restore || got[i].HealPercentMax != want[i].HealPercentMax || got[i].Value != want[i].Value || got[i].Ability != want[i].Ability {
			t.Errorf("effect %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	a := actionToProto(rules.Action{
		Key: "feat:x", Resource: "reserva_de_teste", Cost: 1, CostMax: 3,
		Result: rules.ActionResult{Heal: &rules.DiceFormula{Count: 1, Sides: 6, Bonus: 1}, HealPerPoint: rules.DiceFormula{Count: 1, Sides: 6}, TempHP: 2, TempHPPerPoint: 3},
	})
	if a.GetCost() != 1 || a.GetCostMax() != 3 || a.GetHeal().GetSides() != 6 || a.GetHeal().GetBonus() != 1 || a.GetHealPerPoint().GetCount() != 1 || a.GetTempHp() != 2 || a.GetTempHpPerPoint() != 3 {
		t.Errorf("action proto = %v", a)
	}
	if plain := actionToProto(rules.Action{Key: "standard:dash"}); plain.GetHeal() != nil || plain.GetHealPerPoint() != nil || plain.GetCost() != 0 {
		t.Errorf("a plain action has a cost or a heal: %v", plain)
	}
	for recharge, want := range map[string]rulesv1.Recharge{
		rules.RechargeInitiative:      rulesv1.Recharge_RECHARGE_INITIATIVE,
		rules.RechargeRestoreOneShort: rulesv1.Recharge_RECHARGE_RESTORE_ONE_ON_SHORT_REST,
		rules.RechargeRestoreOneLong:  rulesv1.Recharge_RECHARGE_RESTORE_ONE_ON_LONG_REST,
	} {
		if rechargeToProto[recharge] != want {
			t.Errorf("recharge %s = %v, want %v", recharge, rechargeToProto[recharge], want)
		}
	}
}
