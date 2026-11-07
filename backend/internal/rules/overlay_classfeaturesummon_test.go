package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The summon of a table class feature (#578): the entry is a feature of an SRD class that brings the table's creatures
// to every character that has it, through the same hidden spell as a feature's summon.

const summonerFeatureKey = "classfeature:chamado-da-matilha" + tableSuffix

func classFeatureSummonOverlay(edit func(*TableClassFeature)) Overlay {
	cf := TableClassFeature{
		TableEntry: TableEntry{Key: summonerFeatureKey, NamePT: "Chamado da Matilha"}, DescPT: []string{"O druida chama a matilha."},
		Class: "class:druid", Level: 2,
		Effects: []Effect{{Type: "resource", Resource: packResource, Max: "2", Recharge: RechargeLongRest}},
		Summon: &TableSummon{
			CreatureKeys: []string{packLeader, wolfKey}, Count: "1 + floor(prof() / 2)", MaxLive: 6,
			Replaces: SummonReplacesSame, ResourceKey: packResource,
		},
	}
	if edit != nil {
		edit(&cf)
	}
	return Overlay{Revision: 1, Creatures: []TableCreature{packLeaderCreature()}, ClassFeatures: []TableClassFeature{cf}}
}

func TestATableClassFeatureSummonsThroughTheSpellChecks(t *testing.T) {
	t.Parallel()
	c := withOverlayOn(t, loadForTest(t), classFeatureSummonOverlay(nil))
	spell := FeatureSummonSpellKey(summonerFeatureKey)
	rule, ok := c.SummonRule(spell)
	if !ok || rule.Feature != summonerFeatureKey || rule.Resource != packResource || rule.MaxLive != 6 {
		t.Fatalf("SummonRule = %+v, %v; want the class feature, its resource and its rule", rule, ok)
	}
	if slices.ContainsFunc(c.ListSpells(SpellFilter{}), func(e SpellEntry) bool { return e.Key == spell }) {
		t.Error("ListSpells lists the class feature's hidden spell")
	}
	b := Build{
		BaseScores: standard("class:x", 1).BaseScores, Race: "race:human", Background: "background:acolyte", Languages: testLanguages,
		Classes: []ClassLevel{{Class: "class:druid", Level: 2}},
	}
	got, err := c.SummonOptions(spell, featureSummonLevel, b)
	if err != nil || len(got.Options) != 1 || got.Options[0].Count != 2 || len(got.Options[0].Forms) != 2 {
		t.Fatalf("SummonOptions = %+v, %v; want one option of 2 creatures and two forms", got, err)
	}
	if _, err := c.CheckSummon(spell, featureSummonLevel, b, SummonPick{Creatures: []string{packLeader, wolfKey}}); err != nil {
		t.Errorf("CheckSummon = %v, want the pick accepted", err)
	}
	d := Derive(b, c)
	if !slices.ContainsFunc(d.GrantedSpells, func(g GrantedSpell) bool {
		return g.Spell.Key == spell && g.Source == summonerFeatureKey && g.Resource == packResource && g.FeatureSummon
	}) {
		t.Errorf("GrantedSpells = %+v, want the class feature's spell, spending the resource", d.GrantedSpells)
	}
	// At level 1 the feature has not come yet.
	b.Classes = []ClassLevel{{Class: "class:druid", Level: 1}}
	if d := Derive(b, c); slices.ContainsFunc(d.GrantedSpells, func(g GrantedSpell) bool { return g.Spell.Key == spell }) {
		t.Error("a level 1 druid is granted the level 2 class feature's spell")
	}
}

func TestATableClassFeatureSummonIsRefusedAtTheFieldThatIsWrong(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		edit  func(*TableClassFeature)
		field string
		more  string
	}{
		{"a resource nobody defines", func(f *TableClassFeature) { f.Summon.ResourceKey = "pontos" }, ".summon.resource_key", ReasonReference},
		{"no resource", func(f *TableClassFeature) { f.Summon.ResourceKey = "" }, ".summon.resource_key", ReasonValue},
		{"a count that does not compile", func(f *TableClassFeature) { f.Summon.Count = "1 +" }, ".summon.count", ReasonFormula},
		{"a creature that does not exist", func(f *TableClassFeature) { f.Summon.CreatureKeys = []string{"creature:fantasma@mesa"} }, ".summon.creature_keys[0]", ReasonReference},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadForTest(t).With(classFeatureSummonOverlay(tc.edit))
			var oe *OverlayError
			if !errors.As(err, &oe) {
				t.Fatalf("With() error = %v, want a refusal", err)
			}
			for _, v := range oe.Violations() {
				if strings.HasSuffix(v.Field, tc.field) && v.Reason == tc.more && strings.HasPrefix(v.Field, "class_features[0]") {
					return
				}
			}
			t.Errorf("no %s violation at class_features[0]%s: %v", tc.more, tc.field, err)
		})
	}
}
