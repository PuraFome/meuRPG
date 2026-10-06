package characters

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// TestTableContentRefusalListsEveryFieldAtItsPath (MR-025, slice 10.11 fix round 1):
// a spell with several fields wrong comes back with every one of them, each at the
// exact path of the editor's input, and nothing is written. The paths and reasons
// of every row are pinned by rules.TestEntryViolationPaths; this proves the API
// sends them as the body's paths.
func TestTableContentRefusalListsEveryFieldAtItsPath(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")

	spell := testSpell("Feitiço torto", "class:wizard")
	spell.Range.DistanceFt = 7
	spell.Damage[0].Dice = "3d6x"
	spell.DescPt = []string{strings.Repeat("x", 5000)}
	spell.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURES, Count: 1}
	_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, spell)))
	var fields []string
	for _, v := range violationsOfErr(t, err) {
		fields = append(fields, v.GetField()+":"+v.GetReason())
	}
	want := []string{
		"table_spell.range.distance_ft:limit", "table_spell.desc_pt[0]:bad_text", "table_spell.target.count:limit", "table_spell.damage[0].dice:bad_value",
	}
	if strings.Join(fields, " ") != strings.Join(want, " ") {
		t.Errorf("violations = %v, want %v", fields, want)
	}
	// A race and a background: the same, in the body's paths.
	race := testRace("Raça torta")
	race.SpeedFt, race.AbilityBonuses.Wisdom, race.Languages = 7, 9, []string{"language:klingon"}
	_, err = master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, race)))
	fields = nil
	for _, v := range violationsOfErr(t, err) {
		fields = append(fields, v.GetField())
	}
	if want := "table_race.speed_ft table_race.ability_bonuses.wisdom table_race.languages[0]"; strings.Join(fields, " ") != want {
		t.Errorf("race violations = %v, want %q", fields, want)
	}
	bg := testBackground("Antecedente torto")
	bg.Skills, bg.Tools = []string{"skill:insight", "skill:cooking"}, []string{"proficiency:light-armor"}
	_, err = master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, bg)))
	fields = nil
	for _, v := range violationsOfErr(t, err) {
		fields = append(fields, v.GetField())
	}
	if want := "table_background.skills[1] table_background.tools[0]"; strings.Join(fields, " ") != want {
		t.Errorf("background violations = %v, want %q", fields, want)
	}
	if n := len(master.entries(t, campaign)); n != 0 {
		t.Errorf("%d entries after refusals, want none", n)
	}
}

// TestTableContentWritesCarryHowManySheetsUseTheEntry: the master's answers to an
// update, an archive and an unarchive say how many characters use the entry, as
// ListTableEntries does (TableEntry.characters_using).
func TestTableContentWritesCarryHowManySheetsUseTheEntry(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", owner)
	race := master.addEntry(t, campaign, testRace("Anão das Brumas"))
	bg := master.addEntry(t, campaign, testBackground("Guarda de farol"))
	spell := master.addEntry(t, campaign, testSpell("Raio de teste", "class:wizard"))
	if race.GetCharactersUsing() != 0 {
		t.Errorf("a new entry is used by %d sheets, want 0", race.GetCharactersUsing())
	}
	owner.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheetWithTable(race.GetKey(), bg.GetKey(), spell.GetKey()))

	next := cloneWithKeys(testRace("Anão das Brumas"), bodyMessage(race))
	updated, err := master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(campaign, race.GetKey(), race.GetRevision(), next)))
	if err != nil || updated.Msg.GetEntry().GetCharactersUsing() != 1 {
		t.Fatalf("UpdateTableEntry() = %v, %v; want characters_using 1", updated, err)
	}
	archived, err := master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: campaign, Key: race.GetKey()}))
	if err != nil || archived.Msg.GetEntry().GetCharactersUsing() != 1 {
		t.Fatalf("ArchiveTableEntry() = %v, %v; want characters_using 1", archived, err)
	}
	back, err := master.table.UnarchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.UnarchiveTableEntryRequest{CampaignId: campaign, Key: race.GetKey()}))
	if err != nil || back.Msg.GetEntry().GetCharactersUsing() != 1 {
		t.Fatalf("UnarchiveTableEntry() = %v, %v; want characters_using 1", back, err)
	}
	// An entry no sheet uses says 0.
	other := master.addEntry(t, campaign, testSpell("Faísca", "class:wizard"))
	archivedOther, err := master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: campaign, Key: other.GetKey()}))
	if err != nil || archivedOther.Msg.GetEntry().GetCharactersUsing() != 0 {
		t.Errorf("ArchiveTableEntry(unused) = %v, %v; want 0", archivedOther, err)
	}
}

// TestListContentNamesLanguagesProficienciesAndDamageTypes: a player reads
// "Comum" and "Ferramentas de ladrão", never a key (the catalog names what a table
// entry points at; the master's effect menu carries the same names).
func TestListContentNamesLanguagesProficienciesAndDamageTypes(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", player)
	res, err := player.content.ListContent(t.Context(), connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign}))
	if err != nil {
		t.Fatalf("ListContent() error = %v", err)
	}
	c := res.Msg.GetContent()
	names := func(list []*rulesv1.NamedKey) map[string]string {
		out := map[string]string{}
		for _, n := range list {
			out[n.GetKey()] = n.GetNamePt()
		}
		return out
	}
	lang, prof, dmg := names(c.GetLanguages()), names(c.GetProficiencies()), names(c.GetDamageTypes())
	if lang["language:common"] != "Comum" || lang["language:deep-speech"] != "Dialeto das Profundezas" || len(lang) != 16 {
		t.Errorf("languages = %v, want the 16 of the SRD with their Portuguese names", lang)
	}
	for _, k := range []string{"proficiency:thieves-tools", "proficiency:light-armor", "proficiency:martial-weapons"} {
		if prof[k] == "" || strings.HasPrefix(prof[k], "proficiency:") || prof[k] == k {
			t.Errorf("proficiency %q has the name %q, want a Portuguese one", k, prof[k])
		}
	}
	if dmg["damage-type:fire"] != "fogo" || len(dmg) != 13 {
		t.Errorf("damage types = %v, want the 13 of the SRD, in Portuguese", dmg)
	}
}

// TestTableContentADuplicateNameIsListedWithTheOtherViolations: a create with a name another entry has, bad dice and a bad
// area size comes back with all three, each at its own path (the duplicate name does not return early).
func TestTableContentADuplicateNameIsListedWithTheOtherViolations(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")
	master.addEntry(t, campaign, testSpell("Lâmina de Nanquim", "class:wizard"))

	spell := testSpell("Lâmina de Nanquim", "class:wizard")
	spell.Damage[0].Dice = "2d8x"
	spell.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_AREA, Shape: rulesv1.TableAreaShape_TABLE_AREA_SHAPE_CONE, SizeFt: 7}
	spell.Attack = ""
	_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, spell)))
	var got []string
	for _, v := range violationsOfErr(t, err) {
		got = append(got, v.GetField()+":"+v.GetReason())
	}
	want := "table_spell.name_pt:duplicate_name table_spell.target.size_ft:limit table_spell.damage[0].dice:bad_value"
	if strings.Join(got, " ") != want {
		t.Errorf("violations = %v, want %q", got, want)
	}
	// An update lists them the same way.
	other := master.addEntry(t, campaign, testSpell("Outra Lâmina", "class:wizard"))
	renamed := cloneWithKeys(spell, bodyMessage(other))
	_, err = master.table.UpdateTableEntry(t.Context(), connect.NewRequest(updateReq(campaign, other.GetKey(), other.GetRevision(), renamed)))
	got = nil
	for _, v := range violationsOfErr(t, err) {
		got = append(got, v.GetField())
	}
	if len(got) != 3 || got[0] != "table_spell.name_pt" {
		t.Errorf("update violations = %v, want the duplicate name and the two others", got)
	}
}
