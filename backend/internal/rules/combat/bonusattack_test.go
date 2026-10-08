package combat

import "testing"

func TestBonusAttack(t *testing.T) {
	t.Parallel()
	shortsword := AttackTraits{Melee: true, Light: true}
	longsword := AttackTraits{Melee: true}
	unarmed := AttackTraits{Melee: true, Unarmed: true, MartialArts: true}
	staff := AttackTraits{Melee: true, MartialArts: true}
	cantrip := AttackTraits{Spell: true}
	after := func(last AttackTraits) BonusAttackTurn { return BonusAttackTurn{AttackAction: true, Last: last} }

	for _, tc := range []struct {
		name string
		turn BonusAttackTurn
		next AttackTraits
		want BonusKind
	}{
		{"light weapon after a light weapon", after(shortsword), shortsword, BonusTwoWeapon},
		{"a heavier weapon off hand", after(shortsword), longsword, BonusNone},
		{"light weapon after a heavier one", after(longsword), shortsword, BonusNone},
		{"no Attack action yet", BonusAttackTurn{Last: shortsword}, shortsword, BonusNone},
		{"the bonus action is spent", BonusAttackTurn{AttackAction: true, BonusUsed: true, Last: shortsword}, shortsword, BonusNone},
		{"unarmed after an unarmed strike", after(unarmed), unarmed, BonusMartialArts},
		{"unarmed after a monk weapon", after(staff), unarmed, BonusMartialArts},
		{"unarmed after another weapon", after(longsword), unarmed, BonusNone},
		{"a monk weapon is no Martial Arts strike", after(unarmed), staff, BonusNone},
		{"flurry strike with the bonus action spent", BonusAttackTurn{AttackAction: true, BonusUsed: true, FlurryLeft: 2, Last: longsword}, unarmed, BonusFlurry},
		{"flurry allows only unarmed strikes", BonusAttackTurn{AttackAction: true, BonusUsed: true, FlurryLeft: 1, Last: longsword}, longsword, BonusNone},
		{"a cantrip is no bonus action attack", after(shortsword), cantrip, BonusNone},
		{"after a cantrip", after(cantrip), shortsword, BonusNone},
	} {
		if got := BonusAttack(tc.turn, tc.next); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestOffHandBonus(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		bonus, mod int
		style      bool
		want       int
	}{
		{"positive modifier is dropped", 5, 3, false, 2},
		{"the style keeps it", 5, 3, true, 5},
		{"a negative modifier stays", 0, -1, false, 0},
		{"a zero modifier", 4, 0, false, 4},
	} {
		if got := OffHandBonus(tc.bonus, tc.mod, tc.style); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}
