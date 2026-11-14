package characters

import (
	"slices"
	"testing"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A power_attack of the table's content: the message's fields reach the engine's effect, the engine
// works it out for a character and the traits the combat reads carry it.

func powerAttackMessage() []*rulesv1.TableEffect {
	return []*rulesv1.TableEffect{
		{Type: "resource", Resource: "trocas_pesadas", Max: "2", Recharge: "short_rest"},
		{
			Type: "power_attack", AttackPenalty: 5, DamageBonus: 10, Weapon: "melee", OnlyProficient: true,
			WeaponProperties: []string{"weapon-property:heavy", "weapon-property:two-handed"},
			Resource:         "trocas_pesadas", When: "level() >= 3", TextPt: "Só com armas pesadas.",
		},
	}
}

func TestTableEffectsOfReadsThePowerAttackFields(t *testing.T) {
	t.Parallel()
	got := tableEffectsOf(powerAttackMessage())[1]
	want := rules.Effect{
		Type: "power_attack", AttackPenalty: 5, DamageBonus: 10, Weapon: "melee", OnlyProficient: true,
		WeaponProperties: []string{"weapon-property:heavy", "weapon-property:two-handed"},
		Resource:         "trocas_pesadas", When: "level() >= 3", TextPT: "Só com armas pesadas.",
	}
	if got.Type != want.Type || got.AttackPenalty != want.AttackPenalty || got.DamageBonus != want.DamageBonus || got.Weapon != want.Weapon ||
		got.OnlyProficient != want.OnlyProficient || !slices.Equal(got.WeaponProperties, want.WeaponProperties) ||
		got.Resource != want.Resource || got.When != want.When || got.TextPT != want.TextPT {
		t.Errorf("effect = %+v, want %+v", got, want)
	}
}

func TestPowerAttackOfTheTableReachesTheTraits(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	const (
		subclass = "subclass:tiro-pesado" + "@mesa"
		feature  = "feature:troca-pesada" + "@mesa"
	)
	content, err := srd.With(rules.Overlay{Revision: 1, Subclasses: []rules.TableSubclass{{
		Key: subclass, NamePT: "Tiro Pesado", Class: "class:fighter",
		Levels: []rules.TableSubclassLevel{{Level: 3, Features: []rules.TableFeature{{
			Key: feature, NamePT: "Troca Pesada", DescPT: []string{"Troca precisão por dano."}, Effects: tableEffectsOf(powerAttackMessage()),
		}}}},
	}}})
	if err != nil {
		t.Fatalf("With: %v", err)
	}
	d := rules.Derive(rules.Build{
		BaseScores: map[rules.Ability]int{rules.STR: 15, rules.DEX: 14, rules.CON: 13, rules.INT: 12, rules.WIS: 10, rules.CHA: 8},
		Race:       "race:human", Background: "background:acolyte",
		Classes: []rules.ClassLevel{{Class: "class:fighter", Level: 5, Subclass: subclass}},
		Weapons: []string{"equipment:greatsword"},
	}, content)
	got := traitsOf(content, d).PowerAttacks
	if len(got) != 1 {
		t.Fatalf("power attacks = %+v, want one", got)
	}
	p := got[0]
	if p.Key != feature || p.NamePT != "Troca Pesada" || p.AttackPenalty != 5 || p.DamageBonus != 10 || p.Weapon != "melee" || !p.OnlyProficient ||
		p.Resource != "trocas_pesadas" || p.TextPT != "Só com armas pesadas." ||
		!slices.Equal(p.Properties, []string{"weapon-property:heavy", "weapon-property:two-handed"}) ||
		!slices.Equal(p.PropertyNamesPT, []string{"Pesada", "Duas mãos"}) {
		t.Errorf("power attack = %+v", p)
	}
}
