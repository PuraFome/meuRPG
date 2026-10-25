package characters

import (
	"slices"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// What table content can say beyond the first menu: a class's own list naming SRD
// spells (MR-025). Each test starts its own database.

// issueCodesOf are the codes of the issues of a character's sheet.
func issueCodesOf(c *charactersv1.Character) []string {
	var out []string
	for _, is := range c.GetDerived().GetIssues() {
		out = append(out, is.GetCode())
	}
	return out
}

// artificerLike is a table class whose own list is made of SRD spells only.
func artificerLike(name string, spells ...string) *rulesv1.TableClass {
	c := testGuardian(name)
	c.Casting.ListFrom = ""
	c.Casting.SrdSpellKeys = spells
	return c
}

// gapsSheet is a level 1 sheet of the class with the given cantrips and
// prepared spells.
func gapsSheet(class string, cantrips, prepared []string) *charactersv1.CharacterSheet {
	return &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 13, Constitution: 14, Intelligence: 12, Wisdom: 16, Charisma: 8},
		RaceKey:    "race:human",
		Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
		Languages:  testLanguages,
		Classes:    []*charactersv1.ClassLevel{{ClassKey: class, Level: 1}},
		// The class's table gives the survival; two more come from its list.
		SkillProficiencyKeys: []string{"skill:arcana", "skill:history"},
		CantripKeys:          cantrips, PreparedSpellKeys: prepared,
		HitPoints: hitPointsAverage(),
	}}}
}

// TestTableClassListNamesSRDSpells: a class's own list names SRD spells; the
// Magias page lists them for the class, a character of the class prepares them
// without a spell_not_on_list issue, and one off the list still gets the issue.
func TestTableClassListNamesSRDSpells(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, rafa, lia := h.newUser("Samuel"), h.newUser("Rafa"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", rafa, lia)

	class := master.addEntry(t, campaign, artificerLike("Artífice",
		"spell:guidance", "spell:mending", "spell:light", "spell:cure-wounds", "spell:healing-word", "spell:aid", "spell:revivify"))
	selo := master.addEntry(t, campaign, testSpell("Selo do Artífice", "class:wizard", class.GetKey()))

	list := spellKeysOf(rafa.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, ClassKey: class.GetKey(), PageSize: 400}))
	for _, want := range []string{"spell:guidance", "spell:cure-wounds", "spell:aid", "spell:revivify", selo.GetKey()} {
		if !slices.Contains(list, want) {
			t.Errorf("the class's list has no %s", want)
		}
	}
	if len(list) != 8 || slices.Contains(list, "spell:fireball") || slices.Contains(list, "spell:bless") {
		t.Errorf("the class's list = %v, want its seven SRD spells and the table's", list)
	}
	// The SRD's lists are the same for everyone else.
	if wizard := spellKeysOf(rafa.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, ClassKey: "class:wizard", PageSize: 400})); slices.Contains(wizard, "spell:cure-wounds") {
		t.Error("Cure Wounds is on the wizard's list")
	}

	c := rafa.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Tinker",
		gapsSheet(class.GetKey(), []string{"spell:guidance", "spell:mending", "spell:light"}, []string{"spell:cure-wounds", "spell:healing-word"}))
	if codes := issueCodesOf(c); len(codes) != 0 {
		t.Errorf("a character that prepares spells of the class's list has issues %v", codes)
	}
	if !slices.ContainsFunc(c.GetDerived().GetSpells(), func(s *rulesv1.CharacterSpell) bool {
		return s.GetSpell().GetKey() == "spell:cure-wounds" && s.GetPrepared()
	}) {
		t.Errorf("Cure Wounds is not prepared on the sheet: %v", c.GetDerived().GetSpells())
	}

	// What the learnable filter of the page reads is the same list.
	learn := spellKeysOf(rafa.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, CharacterId: c.GetId(), PageSize: 400}))
	if !slices.Contains(learn, "spell:cure-wounds") || slices.Contains(learn, "spell:aid") || slices.Contains(learn, "spell:fireball") {
		t.Errorf("a level 1 character of the class can learn %v: want Cure Wounds, not Aid (2nd circle) nor Fireball", learn)
	}

	bad := lia.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Fora da lista",
		gapsSheet(class.GetKey(), []string{"spell:guidance", "spell:mending", "spell:light"}, []string{"spell:cure-wounds", "spell:magic-missile"}))
	if !slices.Contains(issueCodesOf(bad), "spell_not_on_list") {
		t.Errorf("Magic Missile is not on the class's list: issues %v", issueCodesOf(bad))
	}
}

// TestTableClassSRDSpellRefusalsPointAtTheirField: the entries casting.srd_spell_keys
// refuses come back at the exact field with a stable reason, and a refused write
// changes nothing.
func TestTableClassSRDSpellRefusalsPointAtTheirField(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")
	revision := h.contentRevision(campaign)

	cases := []struct {
		name          string
		edit          func(c *rulesv1.TableClass)
		field, reason string
	}{
		{"an unknown spell", func(c *rulesv1.TableClass) { c.Casting.SrdSpellKeys[1] = "spell:fantasma" }, "table_class.casting.srd_spell_keys[1]", "dangling_reference"},
		{"a table spell", func(c *rulesv1.TableClass) { c.Casting.SrdSpellKeys[2] = "spell:selo@mesa" }, "table_class.casting.srd_spell_keys[2]", "dangling_reference"},
		{"a spell twice", func(c *rulesv1.TableClass) { c.Casting.SrdSpellKeys[2] = c.Casting.SrdSpellKeys[0] }, "table_class.casting.srd_spell_keys[2]", "bad_value"},
		{"a reused list", func(c *rulesv1.TableClass) { c.Casting.ListFrom = "class:cleric" }, "table_class.casting.srd_spell_keys", "bad_casting"},
		{"a class that does not cast", func(c *rulesv1.TableClass) { *c = *testClass("Sem magia"); c.Casting = &rulesv1.TableCasting{SrdSpellKeys: []string{"spell:aid"}} }, "table_class.casting.srd_spell_keys", "bad_casting"},
	}
	for _, tc := range cases {
		c := artificerLike("Artífice", "spell:guidance", "spell:cure-wounds", "spell:aid")
		tc.edit(c)
		vs := master.refused(t, campaign, c)
		if len(vs) == 0 || vs[0].GetField() != tc.field || vs[0].GetReason() != tc.reason {
			t.Errorf("%s: violations %v, want %s (%s)", tc.name, vs, tc.field, tc.reason)
		}
	}
	if got := h.contentRevision(campaign); got != revision {
		t.Errorf("refused writes moved the content revision from %d to %d", revision, got)
	}

	// The stored class keeps its SRD spells as written.
	entry := master.addEntry(t, campaign, artificerLike("Artífice", "spell:aid", "spell:guidance"))
	got := master.entries(t, campaign)[entry.GetKey()].GetTableClass().GetCasting().GetSrdSpellKeys()
	if !slices.Equal(got, []string{"spell:aid", "spell:guidance"}) {
		t.Errorf("stored srd_spell_keys = %v", got)
	}
}
