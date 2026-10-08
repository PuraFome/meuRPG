package characters

import (
	"context"
	"fmt"
	"slices"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// castAs casts key at the slot as a caster of the class and level (spellcasting
// scores high enough for any spell) and returns what the combat reads of the cast,
// with the damage type picked (empty takes the spell's first).
func castAs(t *testing.T, class, sub string, level int32, key string, slot int, damageType string) link.Spell {
	t.Helper()
	h := newHarness(t)
	master := h.newUser("Mestre")
	p := h.newUser("Conjurador")
	campaign := h.newCampaign(master, "Mirathel", p)
	ch := p.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Conjurador",
		casterSheet(class, sub, level, 10, 12, 14, 18, 18, 12, nil, nil))
	tx, err := h.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.WithoutCancel(t.Context())) })
	sp, err := h.svc.CombatSpell(t.Context(), tx, campaign, ch.GetId(), key, slot, damageType)
	if err != nil {
		t.Fatalf("CombatSpell(%s, %d) error = %v", key, slot, err)
	}
	return sp
}

// damagesOf prints the damage parts of a cast as "NdS+B type", in the spell's order.
func damagesOf(sp link.Spell) []string {
	var out []string
	for _, d := range sp.Damages {
		out = append(out, fmt.Sprintf("%dd%d+%d %s", d.Count, d.Sides, d.Bonus, d.DamageType))
	}
	return out
}

// TestCastKeepsEveryDamageTypeOfTheSpell: a spell that deals two damage types
// reaches the combat as two parts, each its own roll with its own type (the SRD's
// Ice Storm, Meteor Swarm and Flame Strike).
func TestCastKeepsEveryDamageTypeOfTheSpell(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		class, sub string
		level      int32
		key        string
		slot       int
		want       []string
	}{
		{"Ice Storm", "class:wizard", "subclass:evocation", 17, "spell:ice-storm", 4, []string{"2d8+0 damage-type:bludgeoning", "4d6+0 damage-type:cold"}},
		{"Meteor Swarm", "class:wizard", "subclass:evocation", 17, "spell:meteor-swarm", 9, []string{"20d6+0 damage-type:fire", "20d6+0 damage-type:bludgeoning"}},
		{"Flame Strike", "class:cleric", "subclass:life", 9, "spell:flame-strike", 5, []string{"4d6+0 damage-type:fire", "4d6+0 damage-type:radiant"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := damagesOf(castAs(t, tc.class, tc.sub, tc.level, tc.key, tc.slot, "")); !slices.Equal(got, tc.want) {
				t.Errorf("%s damage = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestFlameStrikeGrowsTheChosenType: from a 6th-level slot the caster's choice of
// fire or radiant takes the extra 1d6 for each level above the 5th, and the other
// type stays at 4d6.
func TestFlameStrikeGrowsTheChosenType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		pick string
		want []string
	}{
		{"", []string{"5d6+0 damage-type:fire", "4d6+0 damage-type:radiant"}},
		{"damage-type:radiant", []string{"4d6+0 damage-type:fire", "5d6+0 damage-type:radiant"}},
	} {
		sp := castAs(t, "class:cleric", "subclass:life", 11, "spell:flame-strike", 6, tc.pick)
		if got := damagesOf(sp); !slices.Equal(got, tc.want) {
			t.Errorf("Flame Strike at slot 6 picking %q = %v, want %v", tc.pick, got, tc.want)
		}
		if want := []string{"damage-type:fire", "damage-type:radiant"}; sp.DamageChoice != rules.DamageChoiceScale || !slices.Equal(sp.DamageTypes, want) {
			t.Errorf("Flame Strike choice = %q %v, want scale %v", sp.DamageChoice, sp.DamageTypes, want)
		}
	}
}

// TestFalseLifeAndAidAreNotHealing: False Life gives temporary hit points and Aid
// raises the maximum, so neither reaches the combat as an ordinary heal capped at
// the maximum; each is a hit point effect of its own.
func TestFalseLifeAndAidAreNotHealing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		class, sub string
		key        string
		slot       int
		kind       string
		pool       link.Dice
		amount     int
	}{
		{"False Life", "class:wizard", "subclass:evocation", "spell:false-life", 1, rules.SpellKindTempHP, link.Dice{Count: 1, Sides: 4}, 4},
		{"False Life with a 3rd-level slot", "class:wizard", "subclass:evocation", "spell:false-life", 3, rules.SpellKindTempHP, link.Dice{Count: 1, Sides: 4}, 14},
		{"Aid", "class:cleric", "subclass:life", "spell:aid", 2, rules.SpellKindMaxHP, link.Dice{}, 5},
		{"Aid with a 4th-level slot", "class:cleric", "subclass:life", "spell:aid", 4, rules.SpellKindMaxHP, link.Dice{}, 15},
	} {
		sp := castAs(t, tc.class, tc.sub, 7, tc.key, tc.slot, "")
		if sp.Heal != nil || len(sp.Damages) != 0 {
			t.Errorf("%s: reaches the combat as a heal %v / damage %v, want a hit point effect only", tc.name, sp.Heal, sp.Damages)
		}
		if sp.HP == nil || sp.HP.Kind != tc.kind || sp.HP.Pool != tc.pool || sp.HP.Amount != tc.amount {
			t.Errorf("%s: hit point effect = %+v, want %s %v + %d", tc.name, sp.HP, tc.kind, tc.pool, tc.amount)
		}
	}
}
