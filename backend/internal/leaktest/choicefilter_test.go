package leaktest

import (
	"slices"
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The pool of a choice of tools with a filter of proficiency kinds is the same for every reader: the proficiencies of
// those kinds the content has, and nothing the table wrote; the answer says nothing of the proficiencies it leaves out.
func TestAFilterOfProficiencyKindsOffersTheSamePoolToEveryone(t *testing.T) {
	p := newChoicesProbe(t)
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	var want []string
	for _, e := range srd.Catalog().Proficiencies {
		if e.Kind == "weapon" || e.Kind == "other" {
			want = append(want, e.Key)
		}
	}
	slices.Sort(want)
	if len(want) == 0 {
		t.Fatal("the content has no weapon or other proficiency")
	}
	class := p.createClass(tableClass("Contrabandista do Cais", &rulesv1.TableFeature{
		NamePt: "Rede do cais", DescPt: []string{"Texto de teste."},
		Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "tool", Count: 1, Filter: &rulesv1.TableChoiceFilter{ProficiencyKinds: []string{"weapon", "other"}}}},
	}))

	const outside, inside = "proficiency:smiths-tools", "proficiency:glaives"
	for _, who := range p.readers() {
		t.Run(who.name, func(t *testing.T) {
			plain := p.preview(who, sheet(class, 1))
			if who.member != plain.ok() {
				t.Fatalf("HTTP %d for the plain sheet, want a read only for a member", plain.status)
			}
			if !who.member {
				return
			}
			res := choicesOf(t, plain)
			if got := optionsOf(res, "#tool"); !slices.Equal(got, want) {
				t.Errorf("the pool has %d options, want the %d proficiencies of the filter's kinds, none of the table's", len(got), len(want))
			}
			if strings.Contains(string(plain.body), "@mesa\"") && strings.Contains(strings.Join(optionsOf(res, "#tool"), " "), "@mesa") {
				t.Error("the pool holds an entry of the table")
			}
			// What the filter leaves out is named nowhere in the answer, and a pick inside it is taken.
			if strings.Contains(string(plain.body), outside) {
				t.Errorf("the answer names %s, a proficiency of a kind the filter leaves out", outside)
			}
			key := firstChoiceKey(res, "#tool")
			taken := p.preview(who, sheet(class, 1, key+"="+inside))
			if !taken.ok() || len(choicesOf(t, taken).GetNotOffered()) != 0 {
				t.Errorf("the pick of %s: HTTP %d %s; want it taken", inside, taken.status, shorten(taken.body))
			}
		})
	}
}

func firstChoiceKey(res *charactersv1.PreviewChoicesResponse, suffix string) string {
	for _, g := range res.GetGroups() {
		for _, ch := range g.GetChoices() {
			if strings.HasSuffix(ch.GetKey(), suffix) {
				return ch.GetKey()
			}
		}
	}
	return ""
}
