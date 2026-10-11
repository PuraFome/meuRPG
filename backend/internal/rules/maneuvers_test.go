package rules

import (
	"errors"
	"slices"
	"testing"
)

// The superiority_die effect of the table's options (a die a maneuver adds to a roll,
// spent from a resource). The names are invented; the engine is generic.

const (
	maneuverSubclass = "subclass:mestre-das-garcas" + tableSuffix
	maneuverFeature  = "feature:golpes" + tableSuffix
	maneuverHeron    = "feature:golpe-da-garca" + tableSuffix
	maneuverGrip     = "feature:golpe-de-pegada" + tableSuffix
	maneuverGuard    = "feature:golpe-de-guarda" + tableSuffix
	maneuverPlain    = "feature:golpe-simples" + tableSuffix
	maneuverDice     = "dados_de_golpe"
	maneuverSides    = `classLevel("fighter") >= 10 ? 10 : 8`
)

func maneuverOverlay(options ...TableFeature) Overlay {
	if options == nil {
		options = []TableFeature{
			postureOption(maneuverHeron, "Golpe da Garça", Effect{Type: "superiority_die", Applies: ManeuverDamage, Resource: maneuverDice, Value: maneuverSides, TextPT: "O alvo faz um teste de Sabedoria."}),
			postureOption(maneuverGrip, "Golpe de Pegada", Effect{Type: "superiority_die", Applies: ManeuverGrapple, Economy: EconomyBonusAction, Resource: maneuverDice, Value: maneuverSides}),
			postureOption(maneuverGuard, "Golpe de Guarda", Effect{Type: "superiority_die", Applies: ManeuverReduceMelee, Economy: EconomyReaction, Ability: "dex", Resource: maneuverDice, Value: maneuverSides}),
			postureOption(maneuverPlain, "Golpe Simples", Effect{Type: "grant_action", Economy: "bonus_action", Resource: maneuverDice}),
		}
	}
	golpes := TableFeature{
		Key: maneuverFeature, NamePT: "Golpes", DescPT: []string{"Escolha golpes."},
		Effects: []Effect{
			{Type: "resource", Resource: maneuverDice, Max: "4", Recharge: "short_rest"},
			// Up to three picks, never more than the options the test gives.
			{Type: "choice", Choice: "feature", Count: min(3, len(options))},
		},
		Options: options,
	}
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: maneuverSubclass, NamePT: "Mestre das Garças", Class: "class:fighter",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{golpes}}},
	}}}
}

func maneuverBuild(level int, picks ...string) Build {
	b := postureBuild(level, picks...)
	b.Classes[0].Subclass = maneuverSubclass
	return b
}

func TestDeriveExposesThePickedManeuversWithTheirDie(t *testing.T) {
	c := withOverlay(t, maneuverOverlay())
	d := Derive(maneuverBuild(3, maneuverHeron, maneuverGrip, maneuverGuard), c)
	if len(d.Maneuvers) != 3 {
		t.Fatalf("maneuvers = %+v, want the three picked", d.Maneuvers)
	}
	i := slices.IndexFunc(d.Maneuvers, func(m Maneuver) bool { return m.Key == maneuverHeron })
	if i < 0 {
		t.Fatalf("no heron maneuver in %+v", d.Maneuvers)
	}
	m := d.Maneuvers[i]
	if m.NamePT != "Golpe da Garça" || m.Resource != maneuverDice || m.Sides != 8 || m.Applies != ManeuverDamage || m.TextPT != "O alvo faz um teste de Sabedoria." {
		t.Errorf("heron = %+v", m)
	}
	guard := d.Maneuvers[slices.IndexFunc(d.Maneuvers, func(m Maneuver) bool { return m.Key == maneuverGuard })]
	if guard.Economy != EconomyReaction || guard.Ability != "dex" {
		t.Errorf("guard = %+v, want a reaction adding Dexterity", guard)
	}
	// The resource is there with the uses the formula gives.
	if !slices.ContainsFunc(d.Resources, func(r Resource) bool { return r.Key == maneuverDice && r.Max == 4 }) {
		t.Errorf("resources = %+v, want %s with 4 uses", d.Resources, maneuverDice)
	}
}

func TestManeuverDieGrowsWithTheLevel(t *testing.T) {
	c := withOverlay(t, maneuverOverlay())
	for _, tc := range []struct{ level, sides int }{{3, 8}, {9, 8}, {10, 10}, {17, 10}} {
		// The level 7 and 15 picks do not exist in this table: three picks are enough.
		d := Derive(maneuverBuild(tc.level, maneuverHeron), c)
		if len(d.Maneuvers) != 1 || d.Maneuvers[0].Sides != tc.sides {
			t.Errorf("level %d: maneuvers = %+v, want a d%d", tc.level, d.Maneuvers, tc.sides)
		}
	}
}

func TestAnUnpickedManeuverAndAPlainActionAreNotManeuvers(t *testing.T) {
	c := withOverlay(t, maneuverOverlay())
	d := Derive(maneuverBuild(3, maneuverPlain), c)
	if len(d.Maneuvers) != 0 {
		t.Errorf("maneuvers = %+v, want none: a plain grant_action is an action, not a die", d.Maneuvers)
	}
	if !slices.ContainsFunc(d.Actions, func(a Action) bool { return a.Key == maneuverPlain && a.Resource == maneuverDice }) {
		t.Errorf("actions = %+v, want the plain action still spending the resource", d.Actions)
	}
	if got := Derive(maneuverBuild(3), c).Maneuvers; len(got) != 0 {
		t.Errorf("maneuvers without picks = %+v", got)
	}
}

func TestSuperiorityDieRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].options[0].effects[0]"
	cases := []struct {
		name          string
		effect        Effect
		field, reason string
	}{
		{"no applies", Effect{Type: "superiority_die", Resource: maneuverDice, Value: "8"}, at + ".applies", ReasonValue},
		{"an unknown applies", Effect{Type: "superiority_die", Applies: "heal", Resource: maneuverDice, Value: "8"}, at + ".applies", ReasonValue},
		{"a resource nobody defines", Effect{Type: "superiority_die", Applies: ManeuverDamage, Resource: "fantasma", Value: "8"}, at + ".resource", ReasonReference},
		{"no value", Effect{Type: "superiority_die", Applies: ManeuverDamage, Resource: maneuverDice}, at + ".value", ReasonValue},
		{"a value that does not compile", Effect{Type: "superiority_die", Applies: ManeuverDamage, Resource: maneuverDice, Value: "prof("}, at + ".value", ReasonFormula},
		{"a reduction without an ability", Effect{Type: "superiority_die", Applies: ManeuverReduceMelee, Economy: EconomyReaction, Resource: maneuverDice, Value: "8"}, at + ".ability", ReasonValue},
		{"a reduction without a reaction", Effect{Type: "superiority_die", Applies: ManeuverReduceMelee, Ability: "dex", Resource: maneuverDice, Value: "8"}, at + ".economy", ReasonValue},
		{"a damage die with an economy", Effect{Type: "superiority_die", Applies: ManeuverDamage, Economy: EconomyReaction, Resource: maneuverDice, Value: "8"}, at + ".economy", ReasonValue},
		{"a grapple that costs an action", Effect{Type: "superiority_die", Applies: ManeuverGrapple, Economy: "action", Resource: maneuverDice, Value: "8"}, at + ".economy", ReasonValue},
		{"an ability on a damage die", Effect{Type: "superiority_die", Applies: ManeuverDamage, Ability: "dex", Resource: maneuverDice, Value: "8"}, at + ".ability", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := maneuverOverlay(postureOption(maneuverHeron, "Golpe da Garça", tc.effect))
			_, err := loadForTest(t).With(o)
			if err == nil {
				t.Fatal("the effect is accepted")
			}
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != tc.field || oe.Reason != tc.reason {
				t.Errorf("error = %v, want field %s reason %s", err, tc.field, tc.reason)
			}
		})
	}
}

func TestSuperiorityDieIsOnTheMenu(t *testing.T) {
	menu := withOverlay(t, maneuverOverlay()).EffectMenu()
	i := slices.IndexFunc(menu.Types, func(ty MenuType) bool { return ty.Type == "superiority_die" })
	if i < 0 {
		t.Fatal("no superiority_die on the menu")
	}
	ty := menu.Types[i]
	if ty.NamePT != "Dado de manobra" {
		t.Errorf("name = %q", ty.NamePT)
	}
	for _, name := range []string{"applies", "resource", "value", "ability", "economy", "text_pt"} {
		if !slices.ContainsFunc(ty.Fields, func(f MenuField) bool { return f.Name == name }) {
			t.Errorf("the menu type has no %s field", name)
		}
	}
	j := slices.IndexFunc(menu.Lists, func(l MenuList) bool { return l.Name == ListManeuverApplies })
	if j < 0 || len(menu.Lists[j].Values) != 4 {
		t.Errorf("the applies list = %+v, want four values", menu.Lists)
	}
}

// An option that is a die on a roll and also carries the action that only spends the resource
// does not list the action: the resource would be spent twice.
func TestAManeuverOptionWithAnActionDoesNotListTheAction(t *testing.T) {
	both := postureOption(maneuverHeron, "Golpe da Garça",
		Effect{Type: "superiority_die", Applies: ManeuverDamage, Resource: maneuverDice, Value: "8"},
		Effect{Type: "grant_action", Economy: "bonus_action", Resource: maneuverDice})
	c := withOverlay(t, maneuverOverlay(both, postureOption(maneuverPlain, "Golpe Simples", Effect{Type: "grant_action", Economy: "bonus_action", Resource: maneuverDice})))
	d := Derive(maneuverBuild(3, maneuverHeron, maneuverPlain), c)
	if slices.ContainsFunc(d.Actions, func(a Action) bool { return a.Key == maneuverHeron }) {
		t.Errorf("actions = %+v, want no action for the option that is a die", d.Actions)
	}
	if !slices.ContainsFunc(d.Actions, func(a Action) bool { return a.Key == maneuverPlain }) {
		t.Errorf("actions = %+v, want the plain option's action kept", d.Actions)
	}
	if len(d.Maneuvers) != 1 || d.Maneuvers[0].Key != maneuverHeron {
		t.Errorf("maneuvers = %+v, want the die", d.Maneuvers)
	}
}
