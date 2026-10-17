package characters

import (
	"fmt"
	"testing"
	"time"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
)

// A package is staged under the rule the editors apply: no two entries of a kind
// share a name, whatever their keys and the case of the name.
func TestStageEntriesRefusesTwoEntriesOfOneKindWithOneName(t *testing.T) {
	t.Parallel()
	background := func(key, name string) *rulesv1.TableEntry {
		body := testBackground(name)
		body.Feature.Key = ""
		return &rulesv1.TableEntry{Key: key, Kind: rulesv1.TableContentKind_TABLE_CONTENT_KIND_BACKGROUND, NamePt: name,
			Body: &rulesv1.TableEntry_TableBackground{TableBackground: body}}
	}
	race := func(key, name string) *rulesv1.TableEntry {
		return &rulesv1.TableEntry{Key: key, Kind: rulesv1.TableContentKind_TABLE_CONTENT_KIND_RACE, NamePt: name,
			Body: &rulesv1.TableEntry_TableRace{TableRace: testRace(name)}}
	}
	cases := []struct {
		name    string
		entries []*rulesv1.TableEntry
		wantBad int
	}{
		{"the same name under two keys", []*rulesv1.TableEntry{background("background:farol@mesa", "Farol"), background("background:farol-2@mesa", "Farol")}, 1},
		{"the same name in another case", []*rulesv1.TableEntry{background("background:farol@mesa", "Farol"), background("background:farol-2@mesa", "FAROL")}, 1},
		{"the same name in another kind", []*rulesv1.TableEntry{background("background:farol@mesa", "Farol"), race("race:farol@mesa", "Farol")}, 0},
		{"two names", []*rulesv1.TableEntry{background("background:farol@mesa", "Farol"), background("background:torre@mesa", "Torre")}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var files []packagePartEntry
			for i, e := range tc.entries {
				files = append(files, packagePartContent(t, fmt.Sprintf("content/%04d.json", i), e))
			}
			in := campaignpackage.NewImport(packagePartOpen(t, files), "00000000-0000-4000-8000-0000000000c1", "u", false, nil)
			part := &packagePart{s: &Service{now: time.Now}}
			sc, good := part.stageEntries(in, in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_CONTENT))
			if good != (tc.wantBad == 0) || len(in.Problems.List) != tc.wantBad {
				t.Fatalf("stageEntries() good = %v, problems = %v, want %d problem(s)", good, in.Problems.List, tc.wantBad)
			}
			if tc.wantBad == 0 {
				return
			}
			p := in.Problems.List[0]
			if p.GetReason() != pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_DUPLICATE || p.GetName() != "Farol" {
				t.Errorf("problem = %v, want a DUPLICATE named Farol", p)
			}
			if len(sc.entries) != 1 {
				t.Errorf("staged %d entries, want only the first", len(sc.entries))
			}
		})
	}
}
