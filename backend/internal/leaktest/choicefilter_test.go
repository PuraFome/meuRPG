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

// The pool of a choice that offers the options of another feature is the same for every reader, and holds no option of an
// entry the table switched off or archived; a pick naming one of those gets the answer of a key that does not exist.
func TestAChoiceOfTheOptionsOfAFeatureOffersNoOptionOfAnEntryTheTableKeepsOut(t *testing.T) {
	for _, how := range []string{"switched off", "archived"} {
		t.Run(how, func(t *testing.T) {
			p := newChoicesProbe(t)
			w := p.w
			source := tableClass("Escola das Marés", &rulesv1.TableFeature{
				NamePt: "Marés", DescPt: []string{"Texto de teste."},
				Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feature", Count: 1}},
				Options: []*rulesv1.TableFeature{{NamePt: "Maré Baixa", DescPt: []string{"Texto de teste."}}, {NamePt: "Maré Alta", DescPt: []string{"Texto de teste."}}},
			})
			made := p.createEntry(t, source)
			feature := made.GetTableClass().GetLevels()[0].GetFeatures()[0]
			var options []string
			for _, o := range feature.GetOptions() {
				options = append(options, o.GetKey())
			}
			slices.Sort(options)
			if len(options) != 2 {
				t.Fatalf("the source feature has %d options, want 2", len(options))
			}
			asking := p.createClassShowing(t, tableClass("Cantor das Marés", &rulesv1.TableFeature{
				NamePt: "Eco das marés", DescPt: []string{"Texto de teste."},
				Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feature", Count: 1, FromFeature: feature.GetKey()}},
			}))

			// Before the table keeps the entry out, every member reads the two options.
			var key string
			for _, who := range p.readers() {
				if !who.member {
					continue
				}
				res := choicesOf(t, p.preview(who, sheet(asking, 1)))
				key = firstChoiceKey(res, "--eco-das-mares@mesa")
				if got := optionsOf(res, "--eco-das-mares@mesa"); !slices.Equal(got, options) {
					t.Fatalf("%s: the pool is %v, want %v\n%s", who.name, got, options, shorten(p.preview(who, sheet(asking, 1)).body))
				}
			}
			if how == "archived" {
				must(w.master.table.ArchiveTableEntry(t.Context(), rq(&rulesv1.ArchiveTableEntryRequest{CampaignId: w.campaign, Key: made.GetKey()})))
			} else {
				must(w.master.table.SetOptionSwitches(t.Context(), rq(&rulesv1.SetOptionSwitchesRequest{CampaignId: w.campaign, Switches: []*rulesv1.OptionSwitch{{Key: made.GetKey(), Off: true}}})))
			}

			const missing = "feature:nenhuma--opcao@mesa"
			for _, who := range p.readers() {
				t.Run(who.name, func(t *testing.T) {
					plain := p.preview(who, sheet(asking, 1))
					if who.member != plain.ok() {
						t.Fatalf("HTTP %d for the plain sheet, want a read only for a member", plain.status)
					}
					if !who.member {
						return
					}
					for _, o := range options {
						if strings.Contains(string(plain.body), o) {
							t.Errorf("the answer names the option %s of an entry the table keeps out", o)
						}
					}
					for _, o := range options {
						named := p.preview(who, sheet(asking, 1, key+"="+o))
						absent := p.preview(who, sheet(asking, 1, key+"="+missing))
						if !sameAnswer(named, o, absent, missing) {
							t.Errorf("the pick of %s: HTTP %d %s\nthe pick of a key that does not exist: HTTP %d %s", o, named.status, shorten(named.body), absent.status, shorten(absent.body))
						}
					}
				})
			}
		})
	}
}
