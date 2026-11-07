package characters

import (
	"testing"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The summon of a table class feature (#578), as the table content RPCs take and show it. These tests need the
// database (MEURPG_TEST_DATABASE_URL).

func classFeatureSummonBody() *rulesv1.TableClassFeature {
	return &rulesv1.TableClassFeature{
		NamePt: "Chamado da Matilha", DescPt: []string{"O druida chama a matilha."}, ClassKey: "class:druid", Level: 2,
		Effects: []*rulesv1.TableEffect{{Type: "resource", Resource: "chamados", Max: "2", Recharge: "long_rest"}},
		Summon:  &rulesv1.TableSummon{CreatureKeys: []string{"monster:wolf"}, Count: "1 + prof()", MaxLive: 4, ResourceKey: "chamados"},
	}
}

func TestATableClassFeatureWithASummonIsAcceptedAndAPlayerReadsOnlyTheCreatures(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	key := tb.master.addEntry(t, tb.campaign, classFeatureSummonBody()).GetKey()
	if s := tb.master.entries(t, tb.campaign)[key].GetTableClassFeature().GetSummon(); s.GetCount() != "1 + prof()" || s.GetResourceKey() != "chamados" || s.GetMaxLive() != 4 {
		t.Errorf("the master reads summon = %v, want it whole", s)
	}
	e := tb.ana.entries(t, tb.campaign)[key]
	if e == nil {
		t.Fatal("a player does not read the class feature")
	}
	s := e.GetTableClassFeature().GetSummon()
	if len(s.GetCreatureKeys()) != 1 || s.GetCreatureKeys()[0] != "monster:wolf" {
		t.Errorf("a player reads summon = %v, want the SRD creature key", s)
	}
	s.CreatureKeys = nil
	if s.GetCount() != "" || s.GetResourceKey() != "" || s.GetMaxLive() != 0 {
		t.Errorf("a player reads more than creature_keys: %v", s)
	}
}

func TestATableClassFeatureSummonIsRefusedAtTheFieldThatIsWrong(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	for _, tc := range []struct {
		name  string
		edit  func(*rulesv1.TableClassFeature)
		field string
	}{
		{"a resource the class does not define", func(f *rulesv1.TableClassFeature) { f.Summon.ResourceKey = "pontos" }, "table_class_feature.summon.resource_key"},
		{"a creature that does not exist", func(f *rulesv1.TableClassFeature) { f.Summon.CreatureKeys = []string{"creature:fantasma@mesa"} }, "table_class_feature.summon.creature_keys[0]"},
		{"a count nobody can read", func(f *rulesv1.TableClassFeature) { f.Summon.Count = "1 +" }, "table_class_feature.summon.count"},
	} {
		f := classFeatureSummonBody()
		tc.edit(f)
		_, err := tb.master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(tb.campaign, f)))
		found := false
		for _, v := range violationsOfErr(t, err) {
			found = found || v.GetField() == tc.field
		}
		if !found {
			t.Errorf("%s: no violation at %s in %v", tc.name, tc.field, err)
		}
	}
}

func TestADuplicateClassFeatureNameIsRefusedAtTheBodyFieldOfTheRequest(t *testing.T) {
	t.Parallel()
	tb := newChoiceTable(t)
	tb.master.addEntry(t, tb.campaign, classFeatureSummonBody())
	_, err := tb.master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(tb.campaign, classFeatureSummonBody())))
	for _, v := range violationsOfErr(t, err) {
		if v.GetField() == "table_class_feature.name_pt" && v.GetReason() == reasonDuplicateName {
			return
		}
	}
	t.Errorf("no duplicate_name violation at table_class_feature.name_pt in %v", err)
}

func TestAPlayerViewOfAClassFeatureSummonDropsTheCountAndTheUnheldCreatures(t *testing.T) {
	t.Parallel()
	body := &rulesv1.TableClassFeature{Summon: &rulesv1.TableSummon{CreatureKeys: []string{"monster:wolf", "creature:lobo@mesa"}, Count: "2", ResourceKey: "x"}}
	row := entryRow{kind: rulesv1.TableContentKind_TABLE_CONTENT_KIND_CLASS_FEATURE, body: body}
	got := forPlayer(row, nil, map[string]bool{}).body.(*rulesv1.TableClassFeature).GetSummon()
	if len(got.GetCreatureKeys()) != 1 || got.GetCount() != "" || got.GetResourceKey() != "" {
		t.Errorf("summon = %v, want only the SRD creature key", got)
	}
	if body.GetSummon().GetCount() != "2" {
		t.Error("forPlayer changed the stored body")
	}
}
