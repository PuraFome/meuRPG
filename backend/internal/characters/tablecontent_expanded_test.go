package characters

import (
	"slices"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The spells of a table subclass that marks them expanded (MR-025). Each test
// starts its own database.

// patronOfThe is a subclass of the class whose spells are Magic Missile (class
// level 1) and Shield (3), two spells that are not on the cleric's list.
func patronOfThe(name, class string, expanded bool) *rulesv1.TableSubclass {
	s := testSubclass(name, class)
	s.AlwaysPrepared = []*rulesv1.TableAlwaysPrepared{{ClassLevel: 1, SpellKey: "spell:magic-missile"}, {ClassLevel: 3, SpellKey: "spell:shield"}}
	s.SpellsExpanded = expanded
	return s
}

// TestTableSubclassSpellsExpanded: the spells of a subclass that marks them
// expanded join the class's list to choose from; the ones of another subclass of
// the same class are given. A subclass without spells refuses the flag.
func TestTableSubclassSpellsExpanded(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, rafa, lia := h.newUser("Samuel"), h.newUser("Rafa"), h.newUser("Lia")
	campaign := h.newCampaign(master, "Mirathel", rafa, lia)
	class := master.addEntry(t, campaign, testGuardian("Guardião"))
	given := master.addEntry(t, campaign, patronOfThe("Caminho dado", class.GetKey(), false))
	chosen := master.addEntry(t, campaign, patronOfThe("Caminho escolhido", class.GetKey(), true))

	if got := master.entries(t, campaign)[chosen.GetKey()].GetTableSubclass().GetSpellsExpanded(); !got {
		t.Error("the stored subclass lost spells_expanded")
	}
	sheet := func(sub string, prepared ...string) *charactersv1.CharacterSheet {
		s := gapsSheet(class.GetKey(), []string{"spell:sacred-flame", "spell:guidance", "spell:light"}, prepared)
		s.GetFull().Classes = []*charactersv1.ClassLevel{ownedSubclass(&charactersv1.ClassLevel{ClassKey: class.GetKey(), Level: 3}, sub)}
		return s
	}
	hasSpell := func(c *charactersv1.Character, key string) bool {
		return slices.ContainsFunc(c.GetDerived().GetSpells(), func(s *rulesv1.CharacterSpell) bool { return s.GetSpell().GetKey() == key })
	}

	// Given: on the sheet, prepared, with no pick.
	a := rafa.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Dada", sheet(given.GetKey(), "spell:cure-wounds", "spell:bless", "spell:healing-word"))
	if codes := issueCodesOf(a); len(codes) != 0 {
		t.Errorf("a sheet with the given spells has issues %v", codes)
	}
	if !hasSpell(a, "spell:magic-missile") || !hasSpell(a, "spell:shield") {
		t.Error("the subclass's given spells are not on the sheet")
	}

	// Expanded: not given, a legal pick among the prepared ones.
	b := lia.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Escolhida", sheet(chosen.GetKey(), "spell:cure-wounds", "spell:magic-missile", "spell:shield"))
	if codes := issueCodesOf(b); len(codes) != 0 {
		t.Errorf("a sheet that prepares the expanded spells has issues %v", codes)
	}
	none := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Sem escolha", sheet(chosen.GetKey(), "spell:cure-wounds"))
	if hasSpell(none, "spell:magic-missile") || hasSpell(none, "spell:shield") {
		t.Error("an expanded spell is on the sheet without being picked")
	}
	other := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Outro caminho", sheet(given.GetKey(), "spell:cure-wounds"))
	if !hasSpell(other, "spell:magic-missile") {
		t.Error("the other subclass of the class stopped giving its spells")
	}

	// The flag needs spells.
	empty := testSubclass("Sem magias", class.GetKey())
	empty.SpellsExpanded = true
	vs := master.refused(t, campaign, empty)
	if len(vs) != 1 || vs[0].GetField() != "table_subclass.spells_expanded" || vs[0].GetReason() != "bad_value" {
		t.Errorf("violations = %v, want table_subclass.spells_expanded (bad_value)", vs)
	}
}
