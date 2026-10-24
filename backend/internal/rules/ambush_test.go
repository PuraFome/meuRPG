package rules

import (
	"errors"
	"slices"
	"testing"
)

// The ambush effect: an attacker's edge against a creature that has not had its turn yet
// (advantage) or that is surprised (a hit is critical). The names are invented.

const (
	ambushSubclass = "subclass:emboscador" + tableSuffix
	ambushEdge     = "feature:golpe-de-emboscada" + tableSuffix
	ambushKill     = "feature:golpe-mortal" + tableSuffix
)

func ambushOverlay(edge, kill []Effect) Overlay {
	return Overlay{Revision: 1, Subclasses: []TableSubclass{{
		Key: ambushSubclass, NamePT: "Emboscador", Class: "class:fighter",
		Levels: []TableSubclassLevel{{Level: 3, Features: []TableFeature{
			{Key: ambushEdge, NamePT: "Golpe de Emboscada", DescPT: []string{"Vantagem contra quem não agiu."}, Effects: edge},
			{Key: ambushKill, NamePT: "Golpe Mortal", DescPT: []string{"Crítico contra surpreso."}, Effects: kill},
		}}},
	}}}
}

func ambushBuild(level int) Build {
	b := postureBuild(level)
	b.Classes[0].Subclass = ambushSubclass
	return b
}

func bothAmbushes() Overlay {
	return ambushOverlay([]Effect{{Type: "ambush", Applies: AmbushUnactedAdvantage}}, []Effect{{Type: "ambush", Applies: AmbushSurprisedCritical}})
}

func TestDeriveExposesTheAmbushesByTheFeatureThatGivesThem(t *testing.T) {
	c := withOverlay(t, bothAmbushes())
	d := Derive(ambushBuild(3), c)
	if d.UnactedAdvantage != ambushEdge || d.SurprisedCritical != ambushKill {
		t.Errorf("ambushes = %q, %q; want the keys of the two features", d.UnactedAdvantage, d.SurprisedCritical)
	}
	if got := Derive(ambushBuild(2), c); got.UnactedAdvantage != "" || got.SurprisedCritical != "" {
		t.Errorf("ambushes at level 2 = %q, %q; want none before the features", got.UnactedAdvantage, got.SurprisedCritical)
	}
}

func TestAmbushRefusalsNameTheirField(t *testing.T) {
	t.Parallel()
	const at = "subclasses[0].levels[0].features[0].effects[0]"
	cases := []struct {
		name          string
		effect        Effect
		field, reason string
	}{
		{"no applies", Effect{Type: "ambush"}, at + ".applies", ReasonValue},
		{"an unknown applies", Effect{Type: "ambush", Applies: "behind_target"}, at + ".applies", ReasonValue},
		{"a value", Effect{Type: "ambush", Applies: AmbushUnactedAdvantage, Value: "2"}, at + ".value", ReasonValue},
		{"a resource", Effect{Type: "ambush", Applies: AmbushUnactedAdvantage, Resource: "x"}, at + ".resource", ReasonValue},
		{"an economy", Effect{Type: "ambush", Applies: AmbushUnactedAdvantage, Economy: "bonus_action"}, at + ".economy", ReasonValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := ambushOverlay([]Effect{tc.effect}, nil)
			o.Strict = []string{ambushSubclass}
			_, err := loadForTest(t).With(o)
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != tc.field || oe.Reason != tc.reason {
				t.Errorf("error = %v, want field %s reason %s", err, tc.field, tc.reason)
			}
		})
	}
}

func TestAmbushIsOnTheMenu(t *testing.T) {
	menu := withOverlay(t, bothAmbushes()).EffectMenu()
	i := slices.IndexFunc(menu.Types, func(ty MenuType) bool { return ty.Type == "ambush" })
	if i < 0 {
		t.Fatal("no ambush on the menu")
	}
	j := slices.IndexFunc(menu.Lists, func(l MenuList) bool { return l.Name == ListAmbushApplies })
	if j < 0 || len(menu.Lists[j].Values) != 2 {
		t.Errorf("the applies list = %+v, want two values", menu.Lists)
	}
}
