package characters

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The richer prerequisites of a feat (races, size, a feature the character must not have) at
// the three places a feat is judged: when the master writes it, at the guided level-up and in
// a pack that is imported.

func violationFields(vs []*rulesv1.TableContentViolation) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.GetField())
	}
	return out
}

func richFeat(name string, p *rulesv1.FeatPrerequisite) *rulesv1.TableFeat {
	return &rulesv1.TableFeat{NamePt: name, DescPt: []string{"Texto."}, Prerequisite: p}
}

// TestFeatRichPrerequisiteIsCheckedWhenTheMasterWritesTheFeat: a valid feat is stored with its
// new fields, and each wrong one is refused at its own field.
func TestFeatRichPrerequisiteIsCheckedWhenTheMasterWritesTheFeat(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel: a Costa")

	good := master.addEntry(t, campaign, richFeat("Sangue antigo", &rulesv1.FeatPrerequisite{
		Races: []string{"race:elf", "subrace:high-elf"}, Size: "Medium", LacksFeatureKey: "feat:grappler",
	}))
	p := good.GetTableFeat().GetPrerequisite()
	if !slices.Equal(p.GetRaces(), []string{"race:elf", "subrace:high-elf"}) || p.GetSize() != "Medium" || p.GetLacksFeatureKey() != "feat:grappler" {
		t.Fatalf("the stored prerequisite = %v, want the races, the size and the lacked feature", p)
	}

	cases := []struct {
		name  string
		p     *rulesv1.FeatPrerequisite
		field string
	}{
		{"race_key and races together", &rulesv1.FeatPrerequisite{RaceKey: "race:elf", Races: []string{"race:dwarf"}}, "table_feat.prerequisite.races"},
		{"a race that does not exist", &rulesv1.FeatPrerequisite{Races: []string{"race:elf", "race:nada"}}, "table_feat.prerequisite.races[1]"},
		{"a race twice", &rulesv1.FeatPrerequisite{Races: []string{"race:elf", "race:elf"}}, "table_feat.prerequisite.races[1]"},
		{"a size that does not exist", &rulesv1.FeatPrerequisite{Size: "Huge"}, "table_feat.prerequisite.size"},
		{"a feature that does not exist", &rulesv1.FeatPrerequisite{LacksFeatureKey: "trait:nada"}, "table_feat.prerequisite.lacks_feature_key"},
	}
	for _, tc := range cases {
		_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, richFeat("Recusado", tc.p))))
		if got := violationFields(violationsOfErr(t, err)); !slices.Contains(got, tc.field) {
			t.Errorf("%s: violations at %v, want one at %s", tc.name, got, tc.field)
		}
	}
}

// TestFeatRichPrerequisiteIsCheckedAtTheLevelUp: the options say what Pensantus lacks (the races
// he is none of, a size that is not his gnome's, the feature he has), the level-up refuses those feats and takes the one
// he qualifies for.
func TestFeatRichPrerequisiteIsCheckedAtTheLevelUp(t *testing.T) {
	t.Parallel()
	ft := newFeatsTable(t)
	ft.allowFeats(t, true)
	own := ft.pc.GetSheet().GetFull().GetRaceKey()
	if own == "race:elf" || own == "race:dwarf" || own == "race:halfling" {
		t.Fatalf("Pensantus is %s: the test needs a race that is none of elf, dwarf or halfling", own)
	}
	const owned = "trait:darkvision" // a gnome has it
	if !slices.ContainsFunc(ft.pc.GetDerived().GetFeatures(), func(f *rulesv1.Feature) bool { return f.GetKey() == owned }) {
		t.Fatalf("the sheet does not list %s: %v", owned, ft.pc.GetDerived().GetFeatures())
	}
	add := func(name string, p *rulesv1.FeatPrerequisite) *rulesv1.TableEntry {
		return ft.master.addEntry(t, ft.campaign, richFeat(name, p))
	}
	otherRaces := add("De outras raças", &rulesv1.FeatPrerequisite{Races: []string{"race:elf", "race:dwarf"}})
	sameRace := add("Da minha raça", &rulesv1.FeatPrerequisite{Races: []string{"race:dwarf", own}})
	medium := add("Médio", &rulesv1.FeatPrerequisite{Size: "Medium"})
	small := add("Pequeno", &rulesv1.FeatPrerequisite{Size: "Small"})
	lacksOwned := add("Sem o que tenho", &rulesv1.FeatPrerequisite{LacksFeatureKey: owned})
	lacksOther := add("Sem agarrar", &rulesv1.FeatPrerequisite{LacksFeatureKey: "feat:grappler"})

	o, err := ft.options(ft.owner, ft.pc)
	if err != nil {
		t.Fatal(err)
	}
	want := map[*rulesv1.TableEntry]rulesv1.FeatUnmetKind{
		otherRaces: rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_RACE, medium: rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_SIZE,
		lacksOwned: rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_LACKS_FEATURE,
		sameRace:   rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_UNSPECIFIED, small: rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_UNSPECIFIED, lacksOther: rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_UNSPECIFIED,
	}
	for e, kind := range want {
		opt := optionOf(o, e.GetKey())
		if opt == nil {
			t.Fatalf("the options lack %s", e.GetNamePt())
		}
		if kind == rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_UNSPECIFIED {
			if !opt.GetQualifies() || len(opt.GetUnmet()) != 0 {
				t.Errorf("%s: qualifies %v, unmet %v; want it met", e.GetNamePt(), opt.GetQualifies(), opt.GetUnmet())
			}
			continue
		}
		if opt.GetQualifies() || len(opt.GetUnmet()) != 1 || opt.GetUnmet()[0].GetKind() != kind {
			t.Errorf("%s: qualifies %v, unmet %v; want only %v", e.GetNamePt(), opt.GetQualifies(), opt.GetUnmet(), kind)
		}
	}
	if u := optionOf(o, otherRaces.GetKey()).GetUnmet()[0]; !slices.Equal(u.GetKeys(), []string{"race:elf", "race:dwarf"}) || u.GetKey() != "" {
		t.Errorf("the races unmet = key %q keys %v, want the two races", u.GetKey(), u.GetKeys())
	}
	if u := optionOf(o, medium.GetKey()).GetUnmet()[0]; u.GetKey() != "Medium" {
		t.Errorf("the size unmet = %q, want Medium", u.GetKey())
	}
	if u := optionOf(o, lacksOwned.GetKey()).GetUnmet()[0]; u.GetKey() != owned {
		t.Errorf("the lacked feature unmet = %q, want %s", u.GetKey(), owned)
	}
	if p := optionOf(o, sameRace.GetKey()).GetPrerequisite(); !slices.Equal(p.GetRaces(), []string{"race:dwarf", own}) {
		t.Errorf("the prerequisite shown = %v, want the races kept", p)
	}

	for _, e := range []*rulesv1.TableEntry{otherRaces, medium, lacksOwned} {
		_, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(e.GetKey(), nil))
		if r := refusal(t, "LevelUpCharacter", err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEAT_PREREQUISITE {
			t.Errorf("%s: refusal = %v, want feat_prerequisite", e.GetNamePt(), r)
		}
	}
	up, err := ft.levelUp(ft.owner, ft.pc, ft.featChoice(sameRace.GetKey(), nil))
	if err != nil {
		t.Fatalf("LevelUpCharacter() with the feat of his race: %v", err)
	}
	if !slices.Equal(up.GetSheet().GetFull().GetFeatKeys(), []string{sameRace.GetKey()}) {
		t.Errorf("feats after = %v, want the feat of his race", up.GetSheet().GetFull().GetFeatKeys())
	}
}

// TestFeatRichPrerequisiteGoesThroughAPack: the new fields are in the exported file, come back
// whole on import, and a pack with a wrong one is refused at the entry's field in the preview.
func TestFeatRichPrerequisiteGoesThroughAPack(t *testing.T) {
	t.Parallel()
	pt := newPackTable(t)
	pt.master.addEntry(t, pt.from, richFeat("Sangue antigo", &rulesv1.FeatPrerequisite{
		Races: []string{"race:elf", "race:dwarf"}, Size: "Medium", LacksFeatureKey: "feat:grappler",
	}))
	pack := pt.export(t, pt.from)
	file, err := storeJSON.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"races"`, `"size"`, `"lacks_feature_key"`} {
		if !strings.Contains(string(file), field) {
			t.Errorf("the pack file lacks %s: %s", field, file)
		}
	}
	loaded := &rulesv1.TableContentPack{}
	if err := loadJSON.Unmarshal(file, loaded); err != nil {
		t.Fatal(err)
	}
	if res := pt.apply(t, pt.to, loaded, "rich-1"); res.GetTotals().GetNew() != 1 {
		t.Fatalf("import totals = %v, want one new entry", res.GetTotals())
	}
	var got *rulesv1.FeatPrerequisite
	for _, e := range pt.entries(t, pt.to) {
		if e.GetTableFeat() != nil {
			got = e.GetTableFeat().GetPrerequisite()
		}
	}
	if !slices.Equal(got.GetRaces(), []string{"race:elf", "race:dwarf"}) || got.GetSize() != "Medium" || got.GetLacksFeatureKey() != "feat:grappler" {
		t.Errorf("the imported prerequisite = %v, want it whole", got)
	}

	entry := func(slug string, p *rulesv1.FeatPrerequisite) *rulesv1.TableEntry {
		return &rulesv1.TableEntry{Key: "feat:" + slug + "@mesa", Kind: rulesv1.TableContentKind_TABLE_CONTENT_KIND_FEAT, NamePt: slug, Body: &rulesv1.TableEntry_TableFeat{TableFeat: richFeat(slug, p)}}
	}
	bad := &rulesv1.TableContentPack{Format: "meurpg.table-content", Version: 1, Entries: []*rulesv1.TableEntry{
		entry("both", &rulesv1.FeatPrerequisite{RaceKey: "race:elf", Races: []string{"race:dwarf"}}),
		entry("unknown", &rulesv1.FeatPrerequisite{Races: []string{"race:elf", "race:nada"}, Size: "Huge", LacksFeatureKey: "trait:nada"}),
	}}
	res := pt.preview(t, pt.to, bad)
	if res.GetTotals().GetRefused() != 2 {
		t.Fatalf("preview totals = %v, want both entries refused", res.GetTotals())
	}
	fieldsOf := func(i int) []string {
		var out []string
		for _, v := range res.GetEntries()[i].GetViolations() {
			out = append(out, v.GetField())
		}
		return out
	}
	if got := fieldsOf(0); !slices.Contains(got, "table_feat.prerequisite.races") {
		t.Errorf("the first entry's violations = %v, want one at the races", got)
	}
	for _, want := range []string{"table_feat.prerequisite.races[1]", "table_feat.prerequisite.size", "table_feat.prerequisite.lacks_feature_key"} {
		if got := fieldsOf(1); !slices.Contains(got, want) {
			t.Errorf("the second entry's violations = %v, want one at %s", got, want)
		}
	}
}
