package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The dice a spell's hit effect adds per slot level are worked out at the cast (hitModifiersAtSlot) into the row, so the
// damage part and the typed group read them: a level-1 spell of 2d8 and one more d8 for each level above, cast in a
// level-3 slot, rolls 4d8 (4 to 32, doubled to 8 to 64 on a critical hit).
func TestAHitSpellCastInAHigherSlotRollsItsPerLevelDiceInTheTypedGroup(t *testing.T) {
	t.Parallel()
	const fire = "damage-type:fire"
	for _, tc := range []struct {
		name     string
		roll     func(*playv1.RollAttackRequest)
		dice     int32
		lo, hi   int32
		accepted int32
	}{
		{"a hit", d20(19), 4, 4, 32, 20},
		{"a critical hit", d20(20), 8, 8, 64, 33},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) { m.Die, m.Dice, m.PerSlotDice, m.MaxDice, m.DamageType = 8, 2, 1, 10, fire }))
			e := a.hitFight(t)
			a.castMark(t, e, 3)
			hit := a.strike(t, e, battleaxe, "Goblin", tc.roll)
			part := wantHitPart(t, tc.name, hit, true)
			if got := part.GetDiceCount(); got != tc.dice || part.GetDiceSides() != 8 {
				t.Errorf("the part rolls %dd%d, want %d dice of d8 (2 and 2 levels above, doubled on a critical hit)", got, part.GetDiceSides(), tc.dice)
			}
			typed := func(sum int32) func(*playv1.RollDamageRequest) {
				return extrasMarked(nil, &playv1.TypedPart{DamageType: "damage-type:slashing", Sum: 5}, &playv1.TypedPart{DamageType: fire, Sum: sum})
			}
			for _, n := range []int32{tc.lo - 1, tc.hi + 1} {
				if _, err := a.damage(t, a.ana, e, hit.GetId(), typed(n)); connect.CodeOf(err) != connect.CodeInvalidArgument || err == nil || !strings.Contains(err.Error(), "must be") {
					t.Errorf("the fire sum %d: error = %v, want invalid_argument with the range", n, err)
				}
			}
			if _, err := a.damage(t, a.ana, e, hit.GetId(), typed(tc.accepted)); err != nil {
				t.Errorf("the fire sum %d: error = %v, want it accepted", tc.accepted, err)
			}
		})
	}
}
