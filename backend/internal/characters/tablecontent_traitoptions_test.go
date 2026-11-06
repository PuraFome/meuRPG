package characters

import (
	"slices"
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// TestATraitOptionPickIsStoredOnTheSheetAndOneOutsideTheOptionsIsRefused: a table race whose trait
// offers two options. The sheet that picks one is saved with the pick; a pick that is not one of
// the options is CHOICE_NOT_OFFERED.
func TestATraitOptionPickIsStoredOnTheSheetAndOneOutsideTheOptionsIsRefused(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	race := testRace("Anão das Brumas")
	race.ChoiceBonuses = nil
	race.Traits[0].Options = []*rulesv1.TableFeature{{NamePt: "Cor A", DescPt: []string{"A."}}, {NamePt: "Cor B", DescPt: []string{"B."}}}
	entry := tb.master.addEntry(t, tb.campaign, race)
	options := entry.GetTableRace().GetTraits()[0].GetOptions()
	if len(options) != 2 {
		t.Fatalf("the trait's options = %v, want two", options)
	}
	const style = "feature:fighter-fighting-style-defense"
	sheet := func(picks ...string) *charactersv1.CharacterSheet {
		return choiceSheet(entry.GetKey(), "", "class:fighter", "", 1, append([]string{style}, picks...)...)
	}

	c, err := tb.create(tb.ana, sheet(options[1].GetKey()))
	if err != nil {
		t.Fatalf("a sheet that picks one option of the trait: %v", err)
	}
	if got := c.GetSheet().GetFull().GetFeatureChoiceKeys(); !slices.Contains(got, options[1].GetKey()) {
		t.Errorf("stored picks = %v, want the option %s kept", got, options[1].GetKey())
	}

	_, err = tb.create(tb.ana, sheet("feature:fighter-fighting-style-archery"))
	if r := choiceRefusal(t, "a pick outside the trait's options", err); r.GetReason() != charactersv1.ChoiceRefusalReason_CHOICE_REFUSAL_REASON_CHOICE_NOT_OFFERED {
		t.Errorf("refusal = %v, want CHOICE_NOT_OFFERED", r)
	}
}

// TestAPackWithAWrongReplacedTraitIsRefusedAtItsIndexNamingTheKey: the preview of a pack whose
// subrace replaces a key its race does not have.
func TestAPackWithAWrongReplacedTraitIsRefusedAtItsIndexNamingTheKey(t *testing.T) {
	t.Parallel()
	pt := newPackTable(t)
	race := pt.master.addEntry(t, pt.to, testRace("Anão das Brumas"))
	have := race.GetTableRace().GetTraits()[0].GetKey()
	sub := testSubrace("Da Colina", race.GetKey())
	sub.ReplacesTraitKeys = []string{have, "trait:nao-existe@mesa"}
	pack := &rulesv1.TableContentPack{Format: "meurpg.table-content", Version: 1, Entries: []*rulesv1.TableEntry{
		{Key: "subrace:da-colina@mesa", Kind: rulesv1.TableContentKind_TABLE_CONTENT_KIND_SUBRACE, NamePt: "Da Colina", Body: &rulesv1.TableEntry_TableSubrace{TableSubrace: sub}},
	}}
	res := pt.preview(t, pt.to, pack)
	if res.GetTotals().GetRefused() != 1 {
		t.Fatalf("preview totals = %v, want the subrace refused", res.GetTotals())
	}
	vs := res.GetEntries()[0].GetViolations()
	if len(vs) != 1 || vs[0].GetField() != "table_subrace.replaces_trait_keys[1]" || !strings.Contains(vs[0].GetMessage(), "trait:nao-existe@mesa") {
		t.Errorf("violations = %v, want one at table_subrace.replaces_trait_keys[1] naming the key", vs)
	}
}
