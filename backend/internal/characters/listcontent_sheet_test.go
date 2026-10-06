package characters

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The catalog with a character (slice 10.12b fix round 1): a player editing or
// leveling their own sheet gets back the entries that sheet uses even when the
// master archived or switched them off, marked; never another player's.

func (u *user) catalogFor(t *testing.T, campaign, character string) (*rulesv1.ListContentResponse, error) {
	t.Helper()
	res, err := u.content.ListContent(t.Context(), connect.NewRequest(&rulesv1.ListContentRequest{CampaignId: campaign, CharacterId: character}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func catalogClass(c *rulesv1.Content, key string) *rulesv1.CharacterClass {
	for _, cl := range c.GetClasses() {
		if cl.GetKey() == key {
			return cl
		}
	}
	return nil
}

func catalogSpell(c *rulesv1.Content, key string) *rulesv1.Spell {
	for _, s := range c.GetSpells() {
		if s.GetKey() == key {
			return s
		}
	}
	return nil
}

// TestListContentGivesBackTheCallersOwnSheetsRetiredEntries: the owner's catalog with
// the sheet has the class the master switched off, its subclass and the table spell the
// master archived, each marked; without the sheet, and for another player, they are not
// there; nobody else's sheet, an NPC or a stranger gets anything but not_found.
func TestListContentGivesBackTheCallersOwnSheetsRetiredEntries(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner, other, stranger := h.newUser("Samuel"), h.newUser("Dona"), h.newUser("Outra"), h.newUser("Intruso")
	campaign := h.newCampaign(master, "Mirathel", owner, other)
	spell := master.addEntry(t, campaign, testSpell("Raio de teste", "class:wizard"))
	sheet := pensantusSheet()
	if spell.GetKey() == "" {
		t.Fatal("the table spell has no key")
	}
	mine := owner.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheet)
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Capitão", pensantusSheet())

	master.setOff(t, campaign, true, "class:wizard", "spell:fireball")
	if _, err := master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: campaign, Key: spell.GetKey()})); err != nil {
		t.Fatalf("ArchiveTableEntry() error = %v", err)
	}

	plain := owner.catalog(t, campaign).GetContent()
	if catalogClass(plain, "class:wizard") != nil || catalogSpell(plain, spell.GetKey()) != nil {
		t.Error("the plain catalog of a player has what the master retired")
	}
	res, err := owner.catalogFor(t, campaign, mine.GetId())
	if err != nil {
		t.Fatalf("ListContent(own sheet) error = %v", err)
	}
	c := res.GetContent()
	if w := catalogClass(c, "class:wizard"); w == nil || !w.GetOff() {
		t.Errorf("the owner's wizard = %v, want it there and off", w)
	}
	if !slices.ContainsFunc(c.GetSubclasses(), func(s *rulesv1.Subclass) bool { return s.GetKey() == "subclass:evocation" }) {
		t.Error("the subclass the sheet uses, under an off class, is not in the owner's catalog")
	}
	// What the sheet does not use stays hidden: the switched-off fireball, the archived table spell.
	if catalogSpell(c, "spell:fireball") != nil || catalogSpell(c, spell.GetKey()) != nil {
		t.Error("the owner's catalog has a retired spell the sheet does not use")
	}
	if mentions(jsonOf(t, res), "spell:fireball") {
		t.Error("the owner's catalog names a switched-off spell the sheet does not use")
	}

	// Another player, a stranger and an NPC's id: not_found, never a catalog.
	if _, err := other.catalogFor(t, campaign, mine.GetId()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another player's ListContent(the owner's sheet) error = %v, want not_found", err)
	}
	if _, err := stranger.catalogFor(t, campaign, mine.GetId()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a stranger's ListContent(character) error = %v, want not_found", err)
	}
	if _, err := owner.catalogFor(t, campaign, npc.GetId()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("the owner's ListContent(an NPC) error = %v, want not_found", err)
	}
	if _, err := owner.catalogFor(t, campaign, "00000000-0000-4000-8000-000000000000"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("the owner's ListContent(no such character) error = %v, want not_found", err)
	}
	// The master's catalog is the same with or without it.
	if mc, err := master.catalogFor(t, campaign, mine.GetId()); err != nil || catalogClass(mc.GetContent(), "class:wizard") == nil {
		t.Errorf("the master's ListContent(character) = %v, %v, want the full catalog", mc, err)
	}
}

// TestListContentKeepsAnArchivedTableSpellTheSheetKnows: a table spell the master
// archived stays in the owner's catalog, marked, while a spell nobody uses does not.
// A pending member edits their own character while it waits (RN-15), so their own sheet's
// retired entries come back to them too; another pending member's sheet is not_found.
func TestListContentGivesAPendingMemberTheirOwnDraftsRetiredEntries(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, waiting, also := h.newUser("Samuel"), h.newUser("Espera"), h.newUser("Tambem")
	campaign := h.newCampaign(master, "Mirathel")
	h.joinPending(master, campaign, waiting, also)
	draft := waiting.createPensantus(t, campaign)
	master.setOff(t, campaign, true, "class:wizard")

	res, err := waiting.catalogFor(t, campaign, draft.GetId())
	if err != nil {
		t.Fatalf("a pending member's ListContent(own draft) error = %v", err)
	}
	if w := catalogClass(res.GetContent(), "class:wizard"); w == nil || !w.GetOff() {
		t.Errorf("the pending member's wizard = %v, want it there and off", w)
	}
	if _, err := also.catalogFor(t, campaign, draft.GetId()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another pending member's ListContent(the draft) error = %v, want not_found", err)
	}
}

func TestListContentKeepsAnArchivedTableSpellTheSheetKnows(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", owner)
	used := master.addEntry(t, campaign, testSpell("Raio usado", "class:wizard"))
	unused := master.addEntry(t, campaign, testSpell("Raio sem uso", "class:wizard"))
	sheet := pensantusSheet()
	full := sheet.GetFull()
	full.KnownSpellKeys = append(full.KnownSpellKeys, used.GetKey())
	mine := owner.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Pensantus", sheet)
	for _, e := range []*rulesv1.TableEntry{used, unused} {
		if _, err := master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: campaign, Key: e.GetKey()})); err != nil {
			t.Fatalf("ArchiveTableEntry(%s) error = %v", e.GetKey(), err)
		}
	}
	res, err := owner.catalogFor(t, campaign, mine.GetId())
	if err != nil {
		t.Fatalf("ListContent(own sheet) error = %v", err)
	}
	if s := catalogSpell(res.GetContent(), used.GetKey()); s == nil || !s.GetArchived() {
		t.Errorf("the spell the sheet has = %v, want it there and archived", s)
	}
	if catalogSpell(res.GetContent(), unused.GetKey()) != nil {
		t.Error("an archived spell nobody uses is in the owner's catalog")
	}
}

// TestCatalogSubclassesCarryTheirAlwaysPreparedSpells: the SRD's domain and the
// table's subclass tell which spells they always prepare, from which class level, so
// the editor can show them at creation; one that depends on a feature's choice is left out.
func TestCatalogSubclassesCarryTheirAlwaysPreparedSpells(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", player)
	d, menu := master.classDefaults(t, campaign), master.effectMenu(t, campaign)
	class := master.addEntry(t, campaign, guardianClass(t, d, menu))
	path := master.addEntry(t, campaign, valleyPath(class.GetKey()))

	c := player.catalog(t, campaign).GetContent()
	got := map[string][]string{}
	for _, s := range c.GetSubclasses() {
		for _, ap := range s.GetAlwaysPrepared() {
			got[s.GetKey()] = append(got[s.GetKey()], ap.GetSpellKey())
			if ap.GetClassLevel() < 1 {
				t.Errorf("%s: %s from class level %d", s.GetKey(), ap.GetSpellKey(), ap.GetClassLevel())
			}
		}
	}
	if want := []string{"spell:cure-wounds", "spell:lesser-restoration"}; !slices.Equal(got[path.GetKey()], want) {
		t.Errorf("the table subclass's always-prepared = %v, want %v", got[path.GetKey()], want)
	}
	if len(got["subclass:life"]) == 0 {
		t.Errorf("the SRD's Life domain has no always-prepared spells in the catalog: %v", got)
	}
	for _, s := range c.GetSubclasses() {
		if s.GetKey() == "subclass:land" && len(s.GetAlwaysPrepared()) > 0 {
			t.Errorf("the Circle of the Land (a spell per terrain choice) lists %v", s.GetAlwaysPrepared())
		}
	}
}

// TestCatalogNamedKeysSayWhatTheyAre: every proficiency has a kind, a language is a
// language, and the armour and the weapons are told apart.
func TestCatalogNamedKeysSayWhatTheyAre(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", player)
	c := player.catalog(t, campaign).GetContent()
	kinds := map[string]rulesv1.NamedKeyKind{}
	for _, p := range c.GetProficiencies() {
		kinds[p.GetKey()] = p.GetKind()
		if p.GetKind() == rulesv1.NamedKeyKind_NAMED_KEY_KIND_UNSPECIFIED {
			t.Errorf("proficiency %s has no kind", p.GetKey())
		}
	}
	for _, l := range c.GetLanguages() {
		if l.GetKind() != rulesv1.NamedKeyKind_NAMED_KEY_KIND_LANGUAGE {
			t.Errorf("language %s has kind %v", l.GetKey(), l.GetKind())
		}
	}
	for key, want := range map[string]rulesv1.NamedKeyKind{
		"proficiency:smiths-tools":    rulesv1.NamedKeyKind_NAMED_KEY_KIND_TOOL,
		"proficiency:light-armor":     rulesv1.NamedKeyKind_NAMED_KEY_KIND_ARMOR,
		"proficiency:martial-weapons": rulesv1.NamedKeyKind_NAMED_KEY_KIND_WEAPON,
	} {
		if kinds[key] != want {
			t.Errorf("%s has kind %v, want %v", key, kinds[key], want)
		}
	}
	for _, d := range c.GetDamageTypes() {
		if d.GetKind() != rulesv1.NamedKeyKind_NAMED_KEY_KIND_UNSPECIFIED {
			t.Errorf("damage type %s has kind %v", d.GetKey(), d.GetKind())
		}
	}
}
