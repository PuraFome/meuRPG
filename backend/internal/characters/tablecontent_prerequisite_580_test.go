package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// #580: a feat asks for a class level, a Pact Boon, a spell and options; an option of a feat's list asks for
// what a table option asks for. Written by the master, stored as sent, refused at its own field.

func gatedFeatBody(p *rulesv1.FeatPrerequisite) *rulesv1.TableFeat {
	return &rulesv1.TableFeat{NamePt: "Pacto firme", DescPt: []string{"Texto."}, Prerequisite: p}
}

func TestAFeatPrerequisiteOfClassLevelPactSpellAndOptionsIsStoredAndChecked(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel: a Costa")

	good := master.addEntry(t, campaign, gatedFeatBody(&rulesv1.FeatPrerequisite{
		ClassLevel: &rulesv1.FeatClassLevel{ClassKey: "class:warlock", Level: 5}, RequiresPact: "feature:pact-of-the-tome",
		RequiresSpellKey: "spell:magic-missile", RequiresOptionKeys: []string{"feature:fighting-style-defense"},
	}))
	p := good.GetTableFeat().GetPrerequisite()
	if p.GetClassLevel().GetClassKey() != "class:warlock" || p.GetClassLevel().GetLevel() != 5 || p.GetRequiresPact() != "feature:pact-of-the-tome" ||
		p.GetRequiresSpellKey() != "spell:magic-missile" || !slices.Equal(p.GetRequiresOptionKeys(), []string{"feature:fighting-style-defense"}) {
		t.Fatalf("stored prerequisite = %v, want all four conditions", p)
	}

	cases := []struct {
		name  string
		p     *rulesv1.FeatPrerequisite
		field string
	}{
		{"a class that does not exist", &rulesv1.FeatPrerequisite{ClassLevel: &rulesv1.FeatClassLevel{ClassKey: "class:nada", Level: 3}}, "table_feat.prerequisite.class_level.class_key"},
		{"a class without a level", &rulesv1.FeatPrerequisite{ClassLevel: &rulesv1.FeatClassLevel{ClassKey: "class:warlock"}}, "table_feat.prerequisite.class_level"},
		{"a pact that is not one", &rulesv1.FeatPrerequisite{RequiresPact: "feature:action-surge"}, "table_feat.prerequisite.requires_pact"},
		{"a spell that does not exist", &rulesv1.FeatPrerequisite{RequiresSpellKey: "spell:nada"}, "table_feat.prerequisite.requires_spell_key"},
		{"an option that does not exist", &rulesv1.FeatPrerequisite{RequiresOptionKeys: []string{"feature:nada"}}, "table_feat.prerequisite.requires_option_keys[0]"},
	}
	for _, tc := range cases {
		_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, gatedFeatBody(tc.p))))
		if got := violationFields(violationsOfErr(t, err)); !slices.Contains(got, tc.field) {
			t.Errorf("%s: violations at %v, want one at %s", tc.name, got, tc.field)
		}
	}
}

func TestAnOptionOfAFeatsListAsksForAPrerequisite(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel: a Costa")

	body := func(p *rulesv1.OptionPrerequisite) *rulesv1.TableFeat {
		return &rulesv1.TableFeat{
			NamePt: "Posturas", DescPt: []string{"Texto."},
			Effects: []*rulesv1.TableEffect{{Type: "choice", Choice: "feature", Count: 1}},
			Options: []*rulesv1.TableFeature{{NamePt: "Postura firme", DescPt: []string{"Texto."}, Prerequisite: p}},
		}
	}
	good := master.addEntry(t, campaign, body(&rulesv1.OptionPrerequisite{ClassLevel: &rulesv1.OptionClassLevel{ClassKey: "class:fighter", Level: 5}}))
	if got := good.GetTableFeat().GetOptions()[0].GetPrerequisite().GetClassLevel().GetLevel(); got != 5 {
		t.Fatalf("stored option prerequisite level = %d, want 5", got)
	}
	_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, body(&rulesv1.OptionPrerequisite{RequiresPact: "feature:action-surge"}))))
	if got := violationFields(violationsOfErr(t, err)); !slices.Contains(got, "table_feat.options[0].prerequisite.requires_pact") {
		t.Errorf("violations at %v, want one at table_feat.options[0].prerequisite.requires_pact", got)
	}
}

// A player's read of a feat or of an option of its list does not name a key the players do not see.
func TestAPlayerDoesNotReadTheHiddenKeysOfANewPrerequisite(t *testing.T) {
	t.Parallel()
	v := playerView{hidden: func(k string) bool { return k == "feature:oculto@mesa" || k == "class:oculta@mesa" }}
	feat := gatedFeatBody(&rulesv1.FeatPrerequisite{
		ClassLevel: &rulesv1.FeatClassLevel{ClassKey: "class:oculta@mesa", Level: 3}, RequiresPact: "feature:oculto@mesa",
		RequiresOptionKeys: []string{"feature:oculto@mesa", "feature:fighting-style-defense"},
	})
	feat.Options = []*rulesv1.TableFeature{{Prerequisite: &rulesv1.OptionPrerequisite{RequiresOptionKeys: []string{"feature:oculto@mesa", "feature:a@mesa"}}}}
	v.stripFeat(feat)
	p := feat.GetPrerequisite()
	if p.GetClassLevel() != nil || p.GetRequiresPact() != "" || !slices.Equal(p.GetRequiresOptionKeys(), []string{"feature:fighting-style-defense"}) {
		t.Errorf("feat prerequisite = %v, want the hidden keys gone", p)
	}
	v.dropEffects(feat.GetOptions())
	if got := feat.GetOptions()[0].GetPrerequisite().GetRequiresOptionKeys(); !slices.Equal(got, []string{"feature:a@mesa"}) {
		t.Errorf("option prerequisite = %v, want only the visible key", got)
	}
}
