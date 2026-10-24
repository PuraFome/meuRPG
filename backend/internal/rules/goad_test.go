package rules

import (
	"errors"
	"slices"
	"testing"
)

// The goad maneuver (superiority_die, applies "goad"): a die added to the damage of a weapon hit, and a
// saving throw of the target against the maneuver's DC; on a failure the target has disadvantage on its
// attack rolls against creatures other than the attacker until the end of the attacker's next turn.
// The names are invented; the engine is generic.

const maneuverGoad = "feature:golpe-provocador" + tableSuffix

func goadOverlay(effect Effect) Overlay {
	return maneuverOverlay(postureOption(maneuverGoad, "Golpe Provocador", effect))
}

func goadEffect() Effect {
	return Effect{Type: "superiority_die", Applies: ManeuverGoad, Ability: "str", Resource: maneuverDice, Value: maneuverSides, TextPT: "O alvo é provocado."}
}

func TestDeriveExposesTheGoadManeuverWithItsSaveDC(t *testing.T) {
	c := withOverlay(t, goadOverlay(goadEffect()))
	d := Derive(maneuverBuild(3, maneuverGoad), c)
	i := slices.IndexFunc(d.Maneuvers, func(m Maneuver) bool { return m.Key == maneuverGoad })
	if i < 0 {
		t.Fatalf("maneuvers = %+v, want the goad", d.Maneuvers)
	}
	m := d.Maneuvers[i]
	var strMod int
	for _, a := range d.Abilities {
		if a.Ability == STR {
			strMod = a.Modifier
		}
	}
	if m.Applies != ManeuverGoad || m.Sides != 8 || m.Ability != "str" || m.SaveDC != 8+d.ProficiencyBonus+strMod {
		t.Errorf("goad = %+v, want a d8 whose DC is 8 + proficiency (%d) + Strength (%d)", m, d.ProficiencyBonus, strMod)
	}
	if other := Derive(maneuverBuild(3, maneuverHeron), withOverlay(t, maneuverOverlay())).Maneuvers; len(other) != 1 || other[0].SaveDC != 0 {
		t.Errorf("a damage maneuver = %+v, want no DC", other)
	}
}

func TestGoadRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].options[0].effects[0]"
	cases := []struct {
		name          string
		edit          func(*Effect)
		field, reason string
	}{
		{"no ability for the DC", func(e *Effect) { e.Ability = "" }, at + ".ability", ReasonValue},
		{"an unknown ability", func(e *Effect) { e.Ability = "luck" }, at + ".ability", ReasonValue},
		{"an economy", func(e *Effect) { e.Economy = EconomyReaction }, at + ".economy", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := goadEffect()
			tc.edit(&e)
			_, err := loadForTest(t).With(goadOverlay(e))
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != tc.field || oe.Reason != tc.reason {
				t.Errorf("error = %v, want field %s reason %s", err, tc.field, tc.reason)
			}
		})
	}
}

func TestGoadIsOnTheMenuAndItsEffectIsInTheCatalog(t *testing.T) {
	c := withOverlay(t, goadOverlay(goadEffect()))
	menu := c.EffectMenu()
	j := slices.IndexFunc(menu.Lists, func(l MenuList) bool { return l.Name == ListManeuverApplies })
	if j < 0 || !slices.ContainsFunc(menu.Lists[j].Values, func(v MenuValue) bool { return v.Key == ManeuverGoad && v.NamePT != "" }) {
		t.Errorf("the applies list = %+v, want the goad with a name", menu.Lists)
	}
	def, ok := c.CombatEffect(EffectGoaded)
	if !ok || def.Duration.Kind != EffectDurationUntilEndOfTurnOf {
		t.Fatalf("effect %s = %+v, %v; want it until the end of the attacker's next turn", EffectGoaded, def, ok)
	}
	if len(def.Modifiers) != 1 || def.Modifiers[0].Kind != ModifierAttackDisadvantage || !def.Modifiers[0].ExceptSource || def.Modifiers[0].Once {
		t.Errorf("modifiers = %+v, want an attack disadvantage that spares the source and is not spent by one roll", def.Modifiers)
	}
}
