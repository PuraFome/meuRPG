package characters

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A table class feature (MR-025): an invented ranger feature at level 1, "Olhar de Rastreador", that takes
// the place of Favored Enemy. The class and the feature it replaces are the SRD's; these tests need the
// database (MEURPG_TEST_DATABASE_URL).

func trackerGaze() *rulesv1.TableClassFeature {
	return &rulesv1.TableClassFeature{
		NamePt: "Olhar de Rastreador", DescPt: []string{"Você lê rastros como outros leem cartas."},
		ClassKey: "class:ranger", Level: 1, ReplacesFeatureKey: "feature:favored-enemy-1-type",
		Effects: []*rulesv1.TableEffect{{Type: "proficiency", Proficiency: "skill:survival"}},
	}
}

func asksForFavoredEnemy(res *charactersv1.PreviewChoicesResponse) bool {
	for _, g := range res.GetGroups() {
		for _, ch := range g.GetChoices() {
			if strings.Contains(ch.GetKey(), "favored-enemy") {
				return true
			}
		}
	}
	return false
}

func TestMR025_ATableClassFeatureReplacesAFeatureOfTheClass(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	ranger := choiceSheet("race:human", "", "class:ranger", "", 1)
	if res, err := tb.preview(tb.ana, "", ranger); err != nil || !asksForFavoredEnemy(res) {
		t.Fatalf("the plain ranger: err = %v, asks for a favored enemy = %v, want it asked", err, res != nil && asksForFavoredEnemy(res))
	}

	feature := tb.master.addEntry(t, tb.campaign, trackerGaze())
	key := feature.GetKey()
	if key != "classfeature:olhar-de-rastreador@mesa" {
		t.Fatalf("key = %q", key)
	}
	res, err := tb.preview(tb.ana, "", ranger)
	if err != nil {
		t.Fatalf("PreviewChoices() error = %v", err)
	}
	if asksForFavoredEnemy(res) {
		t.Error("the replaced feature still asks its choice")
	}
	// The master turns it off: the class reads as before.
	tb.master.setOff(t, tb.campaign, true, key)
	if res, err := tb.preview(tb.ana, "", ranger); err != nil || !asksForFavoredEnemy(res) {
		t.Errorf("with the feature off: err = %v, favored enemy asked = %v, want the class as before", err, res != nil && asksForFavoredEnemy(res))
	}
	tb.master.setOff(t, tb.campaign, false, key)

	// The sheet shows the feature and its effect.
	sheet := choiceSheet("race:human", "", "class:ranger", "", 1, "feature:natural-explorer-1-terrain-type=terrain:forest")
	sheet.GetFull().SkillProficiencyKeys = []string{"skill:stealth", "skill:perception", "skill:athletics"}
	c := tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Rastreadora", sheet)
	d := tb.master.get(t, tb.campaign, c.GetId()).GetDerived()
	found := false
	for _, f := range d.GetFeatures() {
		if f.GetKey() == key && f.GetNamePt() == "Olhar de Rastreador" {
			found = true
		}
		if f.GetKey() == "feature:favored-enemy-1-type" {
			t.Error("the sheet lists the replaced feature")
		}
	}
	if !found {
		t.Errorf("features = %v, want the table's feature", d.GetFeatures())
	}
	for _, s := range d.GetSkills() {
		if s.GetKey() == "skill:survival" && s.GetProficiency() == rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_NONE {
			t.Error("survival has no proficiency: the feature's effect is not on the sheet")
		}
	}
}

func TestMR025_ATableClassFeatureHasItsOwnSwitchAndCount(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	key := tb.master.addEntry(t, tb.campaign, trackerGaze()).GetKey()
	if _, err := tb.create(tb.ana, choiceSheet("race:human", "", "class:fighter", "", 1, "feature:fighter-fighting-style-defense")); err != nil {
		t.Fatalf("create() error = %v", err)
	}
	tb.master.create(t, tb.campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Rastreadora", choiceSheet("race:human", "", "class:ranger", "", 1))
	list, err := tb.master.table.ListOptionSwitches(t.Context(), connect.NewRequest(&rulesv1.ListOptionSwitchesRequest{CampaignId: tb.campaign}))
	if err != nil {
		t.Fatalf("ListOptionSwitches() error = %v", err)
	}
	for _, o := range list.Msg.GetOptions() {
		if o.GetKey() != key {
			continue
		}
		if o.GetKind() != rulesv1.TableContentKind_TABLE_CONTENT_KIND_CLASS_FEATURE || o.GetParentKey() != "class:ranger" || o.GetLevel() != 1 || o.GetCharactersUsing() != 1 || o.GetOff() {
			t.Errorf("entry = %v, want the class feature of the ranger at level 1, on, used by the one ranger", o)
		}
		return
	}
	t.Fatalf("the switch list has no %s", key)
}
