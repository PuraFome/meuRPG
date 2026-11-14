package rules

import (
	"slices"
	"strings"
	"testing"
)

// #580: a table feat may also ask for a level in one class, a Pact Boon, a spell and options the character picked.

const (
	featKeyWarlock5   = "feat:bruxo-5" + tableSuffix
	featKeyTomePact   = "feat:livro" + tableSuffix
	featKeyMagicMissl = "feat:misseis" + tableSuffix
	featKeyNeedsStyle = "feat:estilo" + tableSuffix
)

func gatedFeats(t *testing.T) *Content {
	t.Helper()
	feat := func(key, name string, p FeatPrerequisite) TableFeat {
		return TableFeat{TableEntry: TableEntry{Key: key, NamePT: name}, DescPT: []string{"Texto."}, Prerequisite: p}
	}
	return withOverlay(t, Overlay{Revision: 1, Feats: []TableFeat{
		feat(featKeyWarlock5, "Bruxo 5", FeatPrerequisite{ClassKey: "class:warlock", ClassLevel: 5}),
		feat(featKeyTomePact, "Livro", FeatPrerequisite{RequiresPact: tome}),
		feat(featKeyMagicMissl, "Mísseis", FeatPrerequisite{RequiresSpell: "spell:magic-missile"}),
		feat(featKeyNeedsStyle, "Estilo", FeatPrerequisite{RequiresOptions: []string{"feature:fighting-style-defense", "feature:fighting-style-dueling"}}),
	}})
}

func TestAFeatAsksForAClassLevelAPactASpellAndOptions(t *testing.T) {
	t.Parallel()
	c := gatedFeats(t)
	known := warlock(5, tome)
	known.Cantrips = nil
	known.SpellsKnown = []string{"spell:magic-missile"}
	cases := []struct {
		name string
		b    Build
		feat string
		want []FeatUnmet
	}{
		{"class level met", warlock(5), featKeyWarlock5, nil},
		{"class level too low", warlock(4), featKeyWarlock5, []FeatUnmet{{Kind: FeatUnmetClassLevel, Key: "class:warlock", Value: 5}}},
		{"another class does not count", choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 9}), featKeyWarlock5, []FeatUnmet{{Kind: FeatUnmetClassLevel, Key: "class:warlock", Value: 5}}},
		{"pact met", warlock(3, tome), featKeyTomePact, nil},
		{"another pact", warlock(3, chain), featKeyTomePact, []FeatUnmet{{Kind: FeatUnmetPact, Key: tome}}},
		{"no pact", warlock(3), featKeyTomePact, []FeatUnmet{{Kind: FeatUnmetPact, Key: tome}}},
		{"spell known", known, featKeyMagicMissl, nil},
		{"spell unknown", warlock(3), featKeyMagicMissl, []FeatUnmet{{Kind: FeatUnmetSpell, Key: "spell:magic-missile"}}},
		{"both options missing", choiceBuild("race:human", "", ClassLevel{Class: "class:fighter", Level: 1}), featKeyNeedsStyle, []FeatUnmet{
			{Kind: FeatUnmetOption, Key: "feature:fighting-style-defense"}, {Kind: FeatUnmetOption, Key: "feature:fighting-style-dueling"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := unmetOf(t, c, tc.b, tc.feat)
			if !slices.EqualFunc(got, tc.want, func(a, b FeatUnmet) bool { return a.Kind == b.Kind && a.Key == b.Key && a.Value == b.Value }) {
				t.Errorf("unmet = %+v, want %+v", got, tc.want)
			}
		})
	}
	if got := c.c.featAsksPT(FeatPrerequisite{ClassKey: "class:warlock", ClassLevel: 5, RequiresPact: tome}, Build{}); !strings.Contains(got, "nível 5 de") || !strings.Contains(got, "Dádiva") && !strings.Contains(got, c.c.namePT(tome)) {
		t.Errorf("asks = %q", got)
	}
}

// A feat the character took whose class level is gone stays on the sheet and shows the existing problem.
func TestAFeatWhoseClassLevelIsLostStaysWithTheProblem(t *testing.T) {
	t.Parallel()
	c := gatedFeats(t)
	b := warlock(3)
	b.Feats = []string{featKeyWarlock5}
	d := Derive(b, c)
	if !slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == featKeyWarlock5 }) {
		t.Fatalf("the feat left the sheet: %v", d.Features)
	}
	if !slices.ContainsFunc(d.Issues, func(i Issue) bool { return i.Code == IssueFeatPrerequisite }) {
		t.Errorf("issues = %+v, want %q", d.Issues, IssueFeatPrerequisite)
	}
}
