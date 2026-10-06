package characters

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// These tests need no database.

// TestBasicSheetFeedsTheTurnOptions: a minion's structured attacks and the
// standard actions go through the same rules engine as a hero's sheet
// (MR-014), with its own key per attack and the damage type as a content key.
func TestBasicSheetFeedsTheTurnOptions(t *testing.T) {
	t.Parallel()
	s := offlineService(t)
	d := basicDerived(s.srd, &charactersv1.BasicSheet{
		HitPointsMax: 27, ArmorClass: 18, SpeedFt: 30,
		Attacks: []*charactersv1.BasicAttack{
			{Name: "Cimitarra", AttackBonus: 4, DamageDiceCount: 1, DamageDiceSides: 6, DamageBonus: 2, DamageType: charactersv1.DamageType_DAMAGE_TYPE_SLASHING},
			{Name: "Arco curto", AttackBonus: 4, DamageDiceCount: 2, DamageDiceSides: 8, DamageBonus: -1, DamageType: charactersv1.DamageType_DAMAGE_TYPE_PIERCING, RangeFt: 80},
		},
	})
	if d.ArmorClass != 18 || len(d.Attacks) != 2 || len(d.StandardActions) != 10 {
		t.Fatalf("derived = AC %d, %d attacks, %d standard actions; want 18, 2 and 10", d.ArmorClass, len(d.Attacks), len(d.StandardActions))
	}
	sword, bow := d.Attacks[0], d.Attacks[1]
	if sword.Key != "basic:0" || sword.DamageType != "damage-type:slashing" || sword.DamageTypeNamePT != "cortante" || sword.Damage != "1d6+2" ||
		sword.DamageDice != (rules.DiceFormula{Count: 1, Sides: 6, Bonus: 2}) {
		t.Errorf("the first attack = %+v, want basic:0, 1d6+2 of damage-type:slashing", sword)
	}
	if bow.Key != "basic:1" || bow.Damage != "2d8-1" || bow.RangeFt != 80 {
		t.Errorf("the second attack = %+v, want basic:1, 2d8-1, 80 ft", bow)
	}

	// The engine's options, copied to the API's: the action spent disables the
	// attacks and the standard actions with the code, the movement is the speed.
	opts := turnOptionsToProto(combat.Options(d, combat.TurnState{ActionUsed: true, MovementUsedFt: 10}, combat.Usage{}))
	if len(opts.GetAttacks()) != 2 || opts.GetAttacks()[0].GetEnabled() ||
		opts.GetAttacks()[0].GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED {
		t.Errorf("attacks = %v, want both disabled by ACTION_USED", opts.GetAttacks())
	}
	if mv := opts.GetEconomy().GetMovement(); mv.GetSpeedFt() != 30 || mv.GetUsedFt() != 10 || mv.GetLeftFt() != 20 || !opts.GetEconomy().GetAction().GetUsed() {
		t.Errorf("economy = %v, want 20 ft left of 30 and the action used", opts.GetEconomy())
	}
	if a := opts.GetAttacks()[1].GetAttack(); a.GetKey() != "basic:1" || a.GetRangeFt() != 80 || a.GetDamageDice().GetCount() != 2 {
		t.Errorf("the bow as the API says it = %v", a)
	}
	if len(opts.GetStandardActions()) != 10 {
		t.Errorf("standard actions = %d, want the 10 everyone has", len(opts.GetStandardActions()))
	}
}

func TestDiceText(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		f    rules.DiceFormula
		want string
	}{
		{rules.DiceFormula{Count: 1, Sides: 10}, "1d10"},
		{rules.DiceFormula{Count: 2, Sides: 6, Bonus: 3}, "2d6+3"},
		{rules.DiceFormula{Count: 1, Sides: 4, Bonus: -1}, "1d4-1"},
	} {
		if got := diceText(tt.f); got != tt.want {
			t.Errorf("diceText(%+v) = %q, want %q", tt.f, got, tt.want)
		}
	}
}
