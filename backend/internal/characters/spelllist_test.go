package characters

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The players' "Magias" page and the spell's target (MR-045, MR-025, RN-23), and the
// free-text "Outro" background (MR-025, question 82). Each test starts its own
// database.

// listSpells calls ListSpells as u, or fails the test.
func (u *user) listSpells(t *testing.T, req *rulesv1.ListSpellsRequest) *rulesv1.ListSpellsResponse {
	t.Helper()
	res, err := u.content.ListSpells(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("ListSpells(%v) error = %v", req, err)
	}
	return res.Msg
}

func spellKeysOf(res *rulesv1.ListSpellsResponse) []string {
	var out []string
	for _, s := range res.GetSpells() {
		out = append(out, s.GetKey())
	}
	return out
}

// testGuardian is a table class that casts from the cleric's list (a full caster's
// table, as the editors fill it).
func testGuardian(name string) *rulesv1.TableClass {
	c := testClass(name)
	c.Casting = &rulesv1.TableCasting{Kind: "full", Ability: rulesv1.Ability_ABILITY_WISDOM, Preparation: "prepared", ListFrom: "class:cleric"}
	slots := [][]int32{
		{2},
		{3},
		{4, 2},
		{4, 3},
		{4, 3, 2},
		{4, 3, 3},
		{4, 3, 3, 1},
		{4, 3, 3, 2},
		{4, 3, 3, 3, 1},
		{4, 3, 3, 3, 2},
		{4, 3, 3, 3, 2, 1},
		{4, 3, 3, 3, 2, 1},
		{4, 3, 3, 3, 2, 1, 1},
		{4, 3, 3, 3, 2, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1},
		{4, 3, 3, 3, 2, 1, 1, 1, 1},
		{4, 3, 3, 3, 3, 1, 1, 1, 1},
		{4, 3, 3, 3, 3, 2, 1, 1, 1},
		{4, 3, 3, 3, 3, 2, 2, 1, 1},
	}
	for i, row := range c.Levels {
		row.CantripsKnown = 3
		row.Slots = append(slices.Clone(slots[i]), make([]int32, 9-len(slots[i]))...)
	}
	return c
}

// TestMR045_TheSpellsPage: a member finds every spell of the table, the SRD's and the
// master's together, by name and by class, circle and school; a player never reads an
// archived table spell, nor an archived class's key; a pending member gets not_found.
func TestMR045_TheSpellsPage(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, dona, outra, pendente := h.newUser("Samuel"), h.newUser("Dona"), h.newUser("Outra"), h.newUser("Pendente")
	campaign := h.newCampaign(master, "Mirathel", dona, outra)
	h.joinPending(master, campaign, pendente)

	nanquim := master.addEntry(t, campaign, testSpell("Lâmina de Nanquim", "class:wizard", "class:warlock"))
	sopro := testSpell("Sopro de Nanquim", "class:wizard")
	sopro.Range = &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF}
	sopro.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_AREA, Shape: rulesv1.TableAreaShape_TABLE_AREA_SHAPE_CONE, SizeFt: 15}
	sopro.Attack = ""
	sopro.Save = &rulesv1.SpellSave{Ability: rulesv1.Ability_ABILITY_DEXTERITY, OnSuccess: rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_HALF}
	sopro.Damage = []*rulesv1.TableSpellDamage{{DamageTypeKey: "damage-type:necrotic", Dice: "3d6", PerSlotLevel: "1d6"}}
	soproEntry := master.addEntry(t, campaign, sopro)
	guardian := master.addEntry(t, campaign, testGuardian("Guardião do Vale"))
	selo := master.addEntry(t, campaign, testSpell("Selo do Vale", "class:wizard", guardian.GetKey()))
	oculta := master.addEntry(t, campaign, testSpell("Raio Oculto", "class:wizard"))
	if _, err := master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: campaign, Key: oculta.GetKey()})); err != nil {
		t.Fatalf("ArchiveTableEntry() error = %v", err)
	}

	// The search finds the SRD's and the table's, by the Portuguese or the English name,
	// without accents or case.
	for query, want := range map[string]string{
		"nanquim": nanquim.GetKey(), "LÂMINA": nanquim.GetKey(), "maos flamejantes": "spell:burning-hands", "Burning Hands": "spell:burning-hands",
		"escudo arcano": "spell:shield",
	} {
		res := dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, Query: query})
		if !slices.Contains(spellKeysOf(res), want) || int(res.GetTotal()) != len(res.GetSpells()) {
			t.Errorf("search %q = %v (total %d), want %s", query, spellKeysOf(res), res.GetTotal(), want)
		}
		if !strings.Contains(res.GetContentVersion(), "+mesa.") {
			t.Errorf("content version = %q, want the table's", res.GetContentVersion())
		}
	}
	if res := dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, Query: "zzz"}); len(res.GetSpells()) != 0 || res.GetTotal() != 0 || res.GetNextPageToken() != "" {
		t.Errorf("a search with no result = %v", res)
	}

	// By class: the wizard's list has the table's spells that name it; the table class
	// (a cleric's list) has the cleric's and what names it.
	wizard := spellKeysOf(dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, ClassKey: "class:wizard", PageSize: 400}))
	for _, key := range []string{"spell:fireball", nanquim.GetKey(), soproEntry.GetKey(), selo.GetKey()} {
		if !slices.Contains(wizard, key) {
			t.Errorf("the wizard's list has no %s", key)
		}
	}
	if slices.Contains(wizard, "spell:cure-wounds") || slices.Contains(wizard, oculta.GetKey()) {
		t.Error("the wizard's list has a cleric spell or the archived one")
	}
	guard := spellKeysOf(dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, ClassKey: guardian.GetKey(), PageSize: 400}))
	if !slices.Contains(guard, "spell:cure-wounds") || !slices.Contains(guard, selo.GetKey()) || slices.Contains(guard, "spell:fireball") {
		t.Errorf("the table class's list = %d spells, want the cleric's and the Selo, not Fireball", len(guard))
	}
	if got := dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, ClassKey: "class:fighter"}); len(got.GetSpells()) != 0 {
		t.Errorf("a class that casts nothing has %d spells", len(got.GetSpells()))
	}

	// By circle and school, added up.
	got := dona.listSpells(t, &rulesv1.ListSpellsRequest{
		CampaignId: campaign, ClassKey: "class:wizard", Levels: []int32{1}, SchoolKeys: []string{"school:evocation"}, Query: "nanquim",
	})
	if keys := spellKeysOf(got); len(keys) != 2 || !slices.Contains(keys, nanquim.GetKey()) || !slices.Contains(keys, soproEntry.GetKey()) {
		t.Errorf("wizard, 1st circle, evocation, nanquim = %v", keys)
	}
	for _, s := range dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, Levels: []int32{0}, PageSize: 400}).GetSpells() {
		if s.GetLevel() != 0 {
			t.Errorf("%s is circle %d in a list of cantrips", s.GetKey(), s.GetLevel())
		}
	}

	// Pages: the pieces add up to the whole list, and a token is good for its own filters.
	whole := spellKeysOf(dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, PageSize: 400}))
	var paged []string
	token := ""
	for range 100 {
		page := dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, PageSize: 50, PageToken: token})
		paged = append(paged, spellKeysOf(page)...)
		if token = page.GetNextPageToken(); token == "" {
			break
		}
	}
	if !slices.Equal(whole, paged) || len(whole) != 319+3 { // the SRD's 319 and the 3 table spells a player may read (the fourth is archived)
		t.Errorf("%d spells in one page, %d in pages, want %d", len(whole), len(paged), 319+3)
	}
	first := dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, PageSize: 5})
	_, err := dona.content.ListSpells(t.Context(), connect.NewRequest(&rulesv1.ListSpellsRequest{CampaignId: campaign, PageSize: 5, Query: "a", PageToken: first.GetNextPageToken()}))
	wantCode(t, "ListSpells() with a token of other filters", err, connect.CodeInvalidArgument)

	// A player never reads an archived table spell, the master does (marked).
	if dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, Query: "raio oculto"}).GetTotal() != 0 {
		t.Error("a player finds the archived spell")
	}
	m := master.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, Query: "raio oculto"})
	if len(m.GetSpells()) != 1 || !m.GetSpells()[0].GetArchived() {
		t.Errorf("the master's search = %v, want the archived spell, marked", m.GetSpells())
	}
	if int(master.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, PageSize: 400}).GetTotal()) != len(whole)+1 {
		t.Error("the master's list is not the players' plus the archived spell")
	}
	// ... and an archived class's key is never named to a player on a spell's list.
	if _, err := master.table.ArchiveTableEntry(t.Context(), connect.NewRequest(&rulesv1.ArchiveTableEntryRequest{CampaignId: campaign, Key: guardian.GetKey()})); err != nil {
		t.Fatalf("ArchiveTableEntry(class) error = %v", err)
	}
	classKeysOf := func(u *user) []string {
		for _, s := range u.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, Query: "selo do vale"}).GetSpells() {
			return s.GetClassKeys()
		}
		return nil
	}
	if slices.Contains(classKeysOf(dona), guardian.GetKey()) || !slices.Contains(classKeysOf(master), guardian.GetKey()) {
		t.Errorf("class keys of the Selo: player %v, master %v", classKeysOf(dona), classKeysOf(master))
	}

	// Only an active member asks.
	for name, u := range map[string]*user{"pending member": pendente, "non-member": h.newUser("Estranho")} {
		_, err := u.content.ListSpells(t.Context(), connect.NewRequest(&rulesv1.ListSpellsRequest{CampaignId: campaign}))
		wantCode(t, "ListSpells() as a "+name, err, connect.CodeNotFound)
	}

	// What the call refuses.
	for name, req := range map[string]*rulesv1.ListSpellsRequest{
		"a long query":      {CampaignId: campaign, Query: strings.Repeat("a", 101)},
		"circle 10":         {CampaignId: campaign, Levels: []int32{10}},
		"circle -1":         {CampaignId: campaign, Levels: []int32{-1}},
		"page size 401":     {CampaignId: campaign, PageSize: 401},
		"a negative page":   {CampaignId: campaign, PageSize: -1},
		"a bad token":       {CampaignId: campaign, PageToken: "x"},
		"21 school filters": {CampaignId: campaign, SchoolKeys: make([]string, 21)},
		"21 circles":        {CampaignId: campaign, Levels: make([]int32, 21)},
	} {
		_, err := dona.content.ListSpells(t.Context(), connect.NewRequest(req))
		wantCode(t, "ListSpells() with "+name, err, connect.CodeInvalidArgument)
		if name == "21 school filters" && !strings.Contains(err.Error(), "school_keys") {
			t.Errorf("%s: %v, want the field school_keys", name, err)
		}
	}
}

// TestMR045_OnlyTheSpellsACharacterCanLearn: "Só as que posso aprender", the list of
// each casting class up to the circle it casts, a multiclass sheet included.
func TestMR045_OnlyTheSpellsACharacterCanLearn(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, dona, outra := h.newUser("Samuel"), h.newUser("Dona"), h.newUser("Outra")
	campaign := h.newCampaign(master, "Mirathel", dona, outra)
	nanquim := master.addEntry(t, campaign, testSpell("Lâmina de Nanquim", "class:wizard"))
	third := testSpell("Chama do Vale", "class:wizard")
	third.Level = 3
	third.Attack = "ranged"
	master.addEntry(t, campaign, third)

	// Pensantus is a Wizard 3: up to the 2nd circle.
	pens := dona.createPensantus(t, campaign)
	learn := func(u *user, character string) []string {
		return spellKeysOf(u.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, CharacterId: character, PageSize: 400}))
	}
	got := learn(dona, pens.GetId())
	for _, want := range []string{"spell:fire-bolt", "spell:shield", "spell:web", nanquim.GetKey()} {
		if !slices.Contains(got, want) {
			t.Errorf("Pensantus can learn no %s", want)
		}
	}
	for _, not := range []string{"spell:fireball", "spell:bless", "spell:cure-wounds", "spell:chama-do-vale@mesa"} {
		if slices.Contains(got, not) {
			t.Errorf("Pensantus can learn %s", not)
		}
	}
	// ... and the filters still add up: only the 3rd circle of the wizard is empty for him.
	if got := dona.listSpells(t, &rulesv1.ListSpellsRequest{CampaignId: campaign, CharacterId: pens.GetId(), Levels: []int32{3}}); got.GetTotal() != 0 {
		t.Errorf("Pensantus can learn %d spells of the 3rd circle", got.GetTotal())
	}
	// The master may ask for a player's character; another player may not.
	if !slices.Equal(learn(master, pens.GetId()), got) {
		t.Error("the master reads another list for Pensantus")
	}
	_, err := outra.content.ListSpells(t.Context(), connect.NewRequest(&rulesv1.ListSpellsRequest{CampaignId: campaign, CharacterId: pens.GetId()}))
	wantCode(t, "ListSpells() with another player's character", err, connect.CodeNotFound)
	_, err = dona.content.ListSpells(t.Context(), connect.NewRequest(&rulesv1.ListSpellsRequest{CampaignId: campaign, CharacterId: "not-an-id"}))
	wantCode(t, "ListSpells() with a bad character", err, connect.CodeNotFound)

	// Maga 3 / Clériga 1: each class's list by its own level.
	multi := pensantusSheet()
	multi.GetFull().Classes = append(multi.GetFull().Classes, &charactersv1.ClassLevel{ClassKey: "class:cleric", Level: 1})
	multi.GetFull().BaseScores.Wisdom = 14
	corvina := outra.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Corvina", multi)
	got = learn(outra, corvina.GetId())
	for _, want := range []string{"spell:shield", "spell:bless", "spell:cure-wounds", "spell:sacred-flame", nanquim.GetKey()} {
		if !slices.Contains(got, want) {
			t.Errorf("Corvina can learn no %s", want)
		}
	}
	for _, not := range []string{"spell:fireball", "spell:spiritual-weapon", "spell:chama-do-vale@mesa"} { // a cleric's 2nd circle: she is a Cleric 1
		if slices.Contains(got, not) {
			t.Errorf("Corvina can learn %s", not)
		}
	}

	// A fighter casts nothing and learns nothing; an NPC's basic sheet has no classes.
	fighter := pensantusSheet()
	fighter.GetFull().Classes = []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 1}}
	fighter.GetFull().CantripKeys, fighter.GetFull().KnownSpellKeys, fighter.GetFull().PreparedSpellKeys = nil, nil, nil
	if _, err := dona.update(t, pens, "Guerreiro", fighter); err != nil {
		t.Fatalf("UpdateCharacter(a fighter) error = %v", err)
	}
	if got := learn(dona, pens.GetId()); len(got) != 0 {
		t.Errorf("a fighter can learn %d spells", len(got))
	}
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_STORY, "Taverneiro", basicSheet())
	_, err = master.content.ListSpells(t.Context(), connect.NewRequest(&rulesv1.ListSpellsRequest{CampaignId: campaign, CharacterId: npc.GetId()}))
	wantCode(t, "ListSpells() with an NPC", err, connect.CodeFailedPrecondition)
}

// TestMR025_TheSpellDetailsSayWhomItReaches: GetSpellDetails carries the target of an SRD
// spell (the structured area) and of a table spell (the master's), in the table's units.
func TestMR025_TheSpellDetailsSayWhomItReaches(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, dona := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", dona)

	cone := testSpell("Sopro de Nanquim", "class:wizard")
	cone.Range = &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF}
	cone.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_AREA, Shape: rulesv1.TableAreaShape_TABLE_AREA_SHAPE_CONE, SizeFt: 15}
	cone.Attack = ""
	cone.Save = &rulesv1.SpellSave{Ability: rulesv1.Ability_ABILITY_DEXTERITY, OnSuccess: rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_HALF}
	toque := testSpell("Mão de Nanquim", "class:wizard")
	toque.Range = &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_TOUCH}
	toque.Attack = "melee"
	pessoal := testSpell("Vigia de Nanquim", "class:wizard")
	pessoal.Range = &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF}
	pessoal.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF}
	pessoal.Attack, pessoal.Damage = "", nil
	hold := testSpell("Prisão de Nanquim", "class:wizard")
	hold.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURE, PerSlotLevel: 1}
	several := testSpell("Par de Nanquim", "class:wizard")
	several.Target = &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_CREATURES, Count: 3, PerSlotLevel: 2}
	keys := map[string]*rulesv1.TableEntry{}
	for _, s := range []*rulesv1.TableSpell{cone, toque, pessoal, several, hold} {
		keys[s.GetNamePt()] = master.addEntry(t, campaign, s)
	}
	// A self spell that picks creatures is refused: Pessoal reaches the caster or an area.
	bad := testSpell("Mau Alvo", "class:wizard")
	bad.Range = &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF}
	_, err := master.table.CreateTableEntry(t.Context(), connect.NewRequest(createReq(campaign, bad)))
	if vs := violationsOfErr(t, err); len(vs) != 1 || vs[0].GetField() != "table_spell.target" {
		t.Errorf("a self spell that picks a creature: violations = %v", vs)
	}

	for _, tc := range []struct {
		key         string
		kind        rulesv1.SpellTargetKind
		shape       rulesv1.SpellAreaShape
		size, count int32
		perLevel    int32
		label       string
		rangeKind   rulesv1.SpellRangeKind
	}{
		{"spell:burning-hands", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_AREA, rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_CONE, 15, 0, 0, "Cone de 4,5 m", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF},
		{"spell:fireball", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_AREA, rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_SPHERE, 20, 0, 0, "Esfera de 6 m", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED},
		{"spell:shield", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_SELF, 0, 0, 0, 0, "Só quem conjura", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF},
		{"spell:fire-bolt", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_CREATURE, 0, 0, 0, 0, "Uma criatura", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED},
		{"spell:bless", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_CREATURES, 0, 0, 3, 1, "Várias criaturas", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED},
		{"spell:detect-magic", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_SELF, 0, 0, 0, 0, "Só quem conjura", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF},
		{"spell:arcane-eye", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_NONE, 0, 0, 0, 0, "Nenhuma criatura", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED},
		{"spell:flame-strike", rulesv1.SpellTargetKind_SPELL_TARGET_KIND_AREA, rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_CYLINDER, 10, 0, 0, "Cilindro de 3 m de raio", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED},
		{keys["Sopro de Nanquim"].GetKey(), rulesv1.SpellTargetKind_SPELL_TARGET_KIND_AREA, rulesv1.SpellAreaShape_SPELL_AREA_SHAPE_CONE, 15, 0, 0, "Cone de 4,5 m", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF},
		{keys["Mão de Nanquim"].GetKey(), rulesv1.SpellTargetKind_SPELL_TARGET_KIND_CREATURE, 0, 0, 0, 0, "Uma criatura", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_TOUCH},
		{keys["Vigia de Nanquim"].GetKey(), rulesv1.SpellTargetKind_SPELL_TARGET_KIND_SELF, 0, 0, 0, 0, "Só quem conjura", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF},
		{keys["Prisão de Nanquim"].GetKey(), rulesv1.SpellTargetKind_SPELL_TARGET_KIND_CREATURE, 0, 0, 0, 1, "Uma criatura", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED},
		{keys["Par de Nanquim"].GetKey(), rulesv1.SpellTargetKind_SPELL_TARGET_KIND_CREATURES, 0, 0, 3, 2, "Várias criaturas", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED},
	} {
		res, err := dona.content.GetSpellDetails(t.Context(), connect.NewRequest(&rulesv1.GetSpellDetailsRequest{CampaignId: campaign, SpellKey: tc.key}))
		if err != nil {
			t.Fatalf("GetSpellDetails(%s) error = %v", tc.key, err)
		}
		d := res.Msg.GetSpell()
		tg := d.GetTarget()
		if tg.GetKind() != tc.kind || tg.GetShape() != tc.shape || tg.GetSizeFt() != tc.size || tg.GetCount() != tc.count || tg.GetPerSlotLevel() != tc.perLevel || tg.GetLabelPt() != tc.label ||
			d.GetRange().GetKind() != tc.rangeKind {
			t.Errorf("%s: target %v, range %v; want %v %v %d ft, count %d (+%d), %q, range %v", tc.key, tg, d.GetRange().GetKind(), tc.kind, tc.shape, tc.size, tc.count, tc.perLevel, tc.label, tc.rangeKind)
		}
	}
}

// TestMR025_TheOutroBackground: the free-text background follows SRD 5.1 "Customizing a
// Background" (question 82): two skills, two tools or languages, a feature and the
// equipment; the sheet calculates with it; what is missing is an issue, and what cannot
// be is an error naming the field.
func TestMR025_TheOutroBackground(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, dona := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", dona)

	pc := dona.createPensantus(t, campaign)
	d := pc.GetDerived()
	if len(d.GetIssues()) != 0 {
		t.Fatalf("Pensantus's issues = %v, want none", d.GetIssues())
	}
	if d.GetBackgroundNamePt() != "Sábio" || d.GetBackgroundEquipmentPt() != "Um tinteiro, uma pena e roupas comuns." {
		t.Errorf("background %q, equipment %q", d.GetBackgroundNamePt(), d.GetBackgroundEquipmentPt())
	}
	for _, want := range []string{"Comum", "Gnômico", "Dracônico", "Élfico"} { // the gnome's and the background's two
		if !slices.Contains(d.GetLanguages(), want) {
			t.Errorf("languages = %v, want %s", d.GetLanguages(), want)
		}
	}
	var feature *rulesv1.Feature
	for _, f := range d.GetFeatures() {
		if f.GetKey() == rules.CustomBackgroundFeatureKey {
			feature = f
		}
	}
	if feature == nil || feature.GetNamePt() != "Pesquisador" || feature.GetSourcePt() != "Sábio" || feature.GetDescription() != "Quando você não sabe uma informação, sabe a quem perguntar." {
		t.Errorf("feature = %v", feature)
	}

	// An update: a tool in place of a language, the equipment rewritten.
	next := pensantusSheet()
	cb := next.GetFull().GetCustomBackground()
	cb.ProficiencyKeys = []string{"proficiency:thieves-tools", "language:elvish"}
	cb.Equipment = "  Um mapa  \n\n"
	pc, err := dona.update(t, pc, "Pensantus", next)
	if err != nil {
		t.Fatalf("UpdateCharacter() error = %v", err)
	}
	if pc.GetDerived().GetBackgroundEquipmentPt() != "Um mapa" {
		t.Errorf("equipment = %q, want it trimmed", pc.GetDerived().GetBackgroundEquipmentPt())
	}
	if tools := pc.GetDerived().GetProficiencies().GetTools(); !slices.ContainsFunc(tools, func(n string) bool { return strings.Contains(n, "ladrão") }) {
		t.Errorf("tool proficiencies = %v, want the thieves' tools", tools)
	}

	// A draft: the parts the player has not written yet are issues, not errors.
	draft := pensantusSheet()
	draft.GetFull().GetCustomBackground().ProficiencyKeys = []string{"language:elvish"}
	draft.GetFull().GetCustomBackground().FeatureText = ""
	draft.GetFull().GetCustomBackground().Equipment = ""
	pc, err = dona.update(t, pc, "Pensantus", draft)
	if err != nil {
		t.Fatalf("UpdateCharacter(a draft) error = %v", err)
	}
	var fields []string
	for _, is := range pc.GetDerived().GetIssues() {
		fields = append(fields, is.GetField())
	}
	slices.Sort(fields)
	if want := []string{"full.custom_background.equipment", "full.custom_background.feature_name", "full.custom_background.proficiency_keys"}; !slices.Equal(fields, want) {
		t.Errorf("a draft's issues = %v, want %v", fields, want)
	}

	// What is refused names the field and never repeats what was typed.
	const typed = "Segredo"
	for name, tc := range map[string]struct {
		edit  func(*charactersv1.CustomBackground)
		field string
	}{
		"three tools or languages": {func(c *charactersv1.CustomBackground) {
			c.ProficiencyKeys = []string{"language:elvish", "language:dwarvish", "proficiency:thieves-tools"}
		}, "sheet.full.custom_background.proficiency_keys"},
		"an armor":                         {func(c *charactersv1.CustomBackground) { c.ProficiencyKeys = []string{"proficiency:light-armor"} }, "sheet.full.custom_background.proficiency_keys[0]"},
		"a long feature name":              {func(c *charactersv1.CustomBackground) { c.FeatureName = typed + strings.Repeat("x", 40) }, "sheet.full.custom_background.feature_name"},
		"a feature name with a line break": {func(c *charactersv1.CustomBackground) { c.FeatureName = typed + "\nx" }, "sheet.full.custom_background.feature_name"},
		"a long feature text":              {func(c *charactersv1.CustomBackground) { c.FeatureText = typed + strings.Repeat("x", 1000) }, "sheet.full.custom_background.feature_text"},
		"long equipment":                   {func(c *charactersv1.CustomBackground) { c.Equipment = typed + strings.Repeat("x", 500) }, "sheet.full.custom_background.equipment"},
		"a control character":              {func(c *charactersv1.CustomBackground) { c.Equipment = typed + "\x07" }, "sheet.full.custom_background.equipment"},
	} {
		s := pensantusSheet()
		tc.edit(s.GetFull().GetCustomBackground())
		_, err := dona.update(t, pc, "Pensantus", s)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid_argument", name, err)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, tc.field+" ") || strings.Contains(msg, typed) {
			t.Errorf("%s: message %q should name %s and never repeat what was typed", name, msg, tc.field)
		}
	}
}
