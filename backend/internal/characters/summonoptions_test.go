package characters

import (
	"context"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// A caster's full sheet for the summon tests: a human with the acolyte
// background, the class and its level, the spells and the feature choices.
func casterSheet(class, subclass string, level int32, str, dex, con, intel, wis, cha int32, prepared, known []string, features ...string) *charactersv1.CharacterSheet {
	cl := &charactersv1.ClassLevel{ClassKey: class, Level: level}
	if subclass != "" {
		cl.Subclass = &charactersv1.ClassLevel_SubclassKey{SubclassKey: subclass}
	}
	return &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores:        &rulesv1.AbilityScores{Strength: str, Dexterity: dex, Constitution: con, Intelligence: intel, Wisdom: wis, Charisma: cha},
		RaceKey:           "race:human",
		Classes:           []*charactersv1.ClassLevel{cl},
		Background:        &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
		KnownSpellKeys:    known,
		PreparedSpellKeys: prepared,
		FeatureChoiceKeys: features,
	}}}
}

func summonOptions(t *testing.T, u *user, campaign, character string) (*charactersv1.GetSummonOptionsResponse, error) {
	t.Helper()
	res, err := u.api.GetSummonOptions(t.Context(), connect.NewRequest(&charactersv1.GetSummonOptionsRequest{CampaignId: campaign, CharacterId: character}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func spellOf(t *testing.T, r *charactersv1.GetSummonOptionsResponse, key string) *charactersv1.SummonSpellOptions {
	t.Helper()
	for _, s := range r.GetSpells() {
		if s.GetSpellKey() == key {
			return s
		}
	}
	return nil
}

func formKeys(o *charactersv1.SummonOption) []string {
	var out []string
	for _, f := range o.GetForms() {
		out = append(out, f.GetMonsterKey())
	}
	return out
}

func counts(c *charactersv1.SummonCircle) []int32 {
	var out []int32
	for _, o := range c.GetOptions() {
		out = append(out, o.GetCount())
	}
	return out
}

// TestMR037_SummonOptions: the read the casting sheet draws from. A wizard casts
// Convocar Familiar as a ritual from the spellbook (no slot), a warlock with the
// Pact of the Chain gets four more forms that attack with their reaction, a
// cleric's Animar Mortos grows with the slot, a druid's Conjurar Animais
// multiplies by the circle; and what a casting would replace is listed.
func TestMR037_SummonOptions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	wizard, warlock, cleric, druid := h.newUser("Maga"), h.newUser("Bruxo"), h.newUser("Clerigo"), h.newUser("Druida")
	campaign := h.newCampaign(master, "Mirathel", wizard, warlock, cleric, druid)
	pensantus := wizard.createPensantus(t, campaign)
	bruxo := warlock.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Bruxo",
		casterSheet("class:warlock", "", 3, 8, 14, 14, 10, 10, 16, nil, nil, "feature:pact-of-the-chain"))
	clerigo := cleric.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Clérigo",
		casterSheet("class:cleric", "subclass:life", 9, 10, 12, 14, 10, 18, 12, []string{"spell:animate-dead", "spell:bless"}, nil))
	druida := druid.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Druida",
		casterSheet("class:druid", "", 9, 10, 12, 14, 10, 18, 12, []string{"spell:conjure-animals", "spell:cure-wounds"}, nil))

	t.Run("a wizard casts Convocar Familiar as a ritual from the spellbook, not with a slot", func(t *testing.T) {
		r, err := summonOptions(t, wizard, campaign, pensantus.GetId())
		if err != nil {
			t.Fatal(err)
		}
		if len(r.GetSpells()) != 1 {
			t.Fatalf("spells = %v, want only Convocar Familiar (Pensantus has no other summoning spell)", r.GetSpells())
		}
		s := r.GetSpells()[0]
		if s.GetSpellKey() != "spell:find-familiar" || s.GetNamePt() != "Convocar Familiar" || s.GetLevel() != 1 || s.GetCastingTimePt() != "1 hora" ||
			!s.GetRitual() || s.GetConcentration() || !s.GetCanRitual() || s.GetCanCastWithSlot() {
			t.Errorf("find familiar = %v", s)
		}
		if len(s.GetCircles()) != 1 || s.GetCircles()[0].GetCircle() != 1 {
			t.Fatalf("circles = %v, want the spell's own circle for the ritual only", s.GetCircles())
		}
		o := s.GetCircles()[0].GetOptions()
		if len(o) != 1 || o[0].GetCount() != 1 || len(o[0].GetForms()) != 15 || o[0].GetForms()[0].GetNamePt() == "" || o[0].GetForms()[0].GetAttack() != 1 {
			t.Errorf("options = %v, want one creature of 15 forms that cannot attack", o)
		}
		if slices.Contains(formKeys(o[0]), "monster:imp") {
			t.Error("a wizard gets no imp")
		}
		if len(s.GetReplaces()) != 0 {
			t.Errorf("replaces = %v, want none before a familiar exists", s.GetReplaces())
		}
		// The slots: 4 first circle and 2 second at level 3, all free.
		if len(r.GetSlots()) != 2 || r.GetSlots()[0].GetLevel() != 1 || r.GetSlots()[0].GetTotal() != 4 || r.GetSlots()[0].GetFree() != 4 || r.GetSlots()[1].GetLevel() != 2 {
			t.Errorf("slots = %v", r.GetSlots())
		}
	})

	t.Run("a familiar the character has is listed as what a new one replaces", func(t *testing.T) {
		tx, err := h.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(t.Context()) }()
		if _, err := h.svc.SummonCreatures(t.Context(), tx, link.Summon{
			CampaignID: campaign, CharacterID: pensantus.GetId(), Source: "familiar", GroupID: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0aa",
			Creatures: []link.CreatureSpec{{MonsterKey: "monster:raven", Name: "Nanquim", Attack: "none"}},
		}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
		r, err := summonOptions(t, master, campaign, pensantus.GetId())
		if err != nil {
			t.Fatal(err)
		}
		rep := spellOf(t, r, "spell:find-familiar").GetReplaces()
		if len(rep) != 1 || rep[0].GetName() != "Nanquim" || rep[0].GetMonsterKey() != "monster:raven" || rep[0].GetMonsterNamePt() != "Corvo" || rep[0].GetId() == "" {
			t.Errorf("replaces = %v", rep)
		}
	})

	t.Run("a warlock with the Pact of the Chain gets the imp, the pseudodragon, the quasit and the sprite, attacking with the reaction, and pact slots", func(t *testing.T) {
		r, err := summonOptions(t, warlock, campaign, bruxo.GetId())
		if err != nil {
			t.Fatal(err)
		}
		s := spellOf(t, r, "spell:find-familiar")
		if s == nil || !s.GetCanRitual() || s.GetCanCastWithSlot() {
			t.Fatalf("find familiar = %v, want a ritual with no slot (the warlock has no such spell)", s)
		}
		o := s.GetCircles()[0].GetOptions()[0]
		for _, k := range []string{"monster:imp", "monster:pseudodragon", "monster:quasit", "monster:sprite", "monster:raven"} {
			if !slices.Contains(formKeys(o), k) {
				t.Errorf("%s is missing from the chain familiar's forms", k)
			}
		}
		if len(o.GetForms()) != 19 {
			t.Errorf("forms = %d, want 19", len(o.GetForms()))
		}
		for _, f := range o.GetForms() {
			if f.GetAttack() != 2 {
				t.Errorf("%s attack = %d, want 2 (the reaction)", f.GetMonsterKey(), f.GetAttack())
			}
		}
		// Pact magic: two slots of the 2nd circle at warlock 3, listed with `pact`.
		if len(r.GetSlots()) != 1 || !r.GetSlots()[0].GetPact() || r.GetSlots()[0].GetLevel() != 2 || r.GetSlots()[0].GetTotal() != 2 {
			t.Errorf("slots = %v", r.GetSlots())
		}
	})

	t.Run("a cleric's Animar Mortos with a slot: one undead at the 3rd circle, five at the 5th, any mix of the two kinds", func(t *testing.T) {
		r, err := summonOptions(t, cleric, campaign, clerigo.GetId())
		if err != nil {
			t.Fatal(err)
		}
		s := spellOf(t, r, "spell:animate-dead")
		if s == nil || s.GetCanRitual() || !s.GetCanCastWithSlot() || s.GetRitual() || s.GetConcentration() || s.GetCastingTimePt() != "1 minuto" || s.GetLevel() != 3 {
			t.Fatalf("animate dead = %v", s)
		}
		var circles []int32
		var got [][]int32
		for _, c := range s.GetCircles() {
			circles = append(circles, c.GetCircle())
			got = append(got, counts(c))
		}
		if !slices.Equal(circles, []int32{3, 4, 5}) || !slices.Equal(got[0], []int32{1}) || !slices.Equal(got[1], []int32{3}) || !slices.Equal(got[2], []int32{5}) {
			t.Errorf("circles = %v with counts %v, want 3, 4, 5 with 1, 3, 5 undead", circles, got)
		}
		o := s.GetCircles()[0].GetOptions()[0]
		if !slices.Equal(formKeys(o), []string{"monster:skeleton", "monster:zombie"}) || o.GetForms()[0].GetAttack() != 3 {
			t.Errorf("forms = %v, want the skeleton and the zombie, free to attack", o.GetForms())
		}
		if spellOf(t, r, "spell:find-familiar") != nil {
			t.Error("a cleric has no Convocar Familiar")
		}
	})

	t.Run("a druid's Conjurar Animais: four options by challenge rating, multiplied by the circle, and the concentration's creatures replaced", func(t *testing.T) {
		r, err := summonOptions(t, druid, campaign, druida.GetId())
		if err != nil {
			t.Fatal(err)
		}
		s := spellOf(t, r, "spell:conjure-animals")
		if s == nil || !s.GetConcentration() || s.GetCastingTimePt() != "1 ação" || s.GetCanRitual() || !s.GetCanCastWithSlot() {
			t.Fatalf("conjure animals = %v", s)
		}
		by := map[int32]*charactersv1.SummonCircle{}
		for _, c := range s.GetCircles() {
			by[c.GetCircle()] = c
		}
		if !slices.Equal(counts(by[3]), []int32{1, 2, 4, 8}) || !slices.Equal(counts(by[5]), []int32{2, 4, 8, 16}) {
			t.Errorf("counts at 3 = %v, at 5 = %v", counts(by[3]), counts(by[5]))
		}
		o := by[3].GetOptions()
		if len(o[0].GetForms()) != 0 || o[0].GetType() != "beast" || o[0].GetMaxCr() != "2" || o[3].GetMaxCr() != "1/4" || o[0].GetAttack() != 3 {
			t.Errorf("options = %v, want beasts by challenge rating, no fixed forms", o)
		}
		// A previous casting's wolves are what a new one ends.
		tx, err := h.pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(t.Context()) }()
		if _, err := h.svc.SummonCreatures(t.Context(), tx, link.Summon{
			CampaignID: campaign, CharacterID: druida.GetId(), Source: "conjure_animals", GroupID: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0bb", Concentration: true,
			Creatures: []link.CreatureSpec{{MonsterKey: "monster:wolf", Attack: "full"}, {MonsterKey: "monster:wolf", Attack: "full"}},
		}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
		r, err = summonOptions(t, druid, campaign, druida.GetId())
		if err != nil {
			t.Fatal(err)
		}
		rep := spellOf(t, r, "spell:conjure-animals").GetReplaces()
		if len(rep) != 2 || rep[0].GetMonsterNamePt() != "Lobo" || rep[0].GetName() != "Lobo 1" || rep[1].GetName() != "Lobo 2" {
			t.Errorf("replaces = %v, want the two wolves", rep)
		}
	})

	t.Run("a master reads any character's; another player gets not_found; an NPC is refused", func(t *testing.T) {
		if _, err := summonOptions(t, master, campaign, pensantus.GetId()); err != nil {
			t.Errorf("master: %v", err)
		}
		if _, err := summonOptions(t, cleric, campaign, pensantus.GetId()); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("another player: error = %v, want not_found", err)
		}
		npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_BOSS, "Strahd", enemySheet())
		if _, err := summonOptions(t, master, campaign, npc.GetId()); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("an NPC: error = %v, want invalid_argument", err)
		}
	})
}

// TestRN20_SummonOptionsCarryNoHitPoints: what the owner's player reads, as the JSON
// the app receives, has no hit points and no creature's number.
func TestRN20_SummonOptionsCarryNoHitPoints(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, owner := h.newUser("Mestre"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", owner)
	pc := owner.createPensantus(t, campaign)
	r, err := summonOptions(t, owner, campaign, pc.GetId())
	if err != nil {
		t.Fatal(err)
	}
	b, err := protojson.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"hitPoints", "hit_points", "hp", "armorClass"} {
		if strings.Contains(string(b), banned) {
			t.Errorf("the response has %q: %s", banned, b)
		}
	}
}

// The summon options are one moment: a casting that commits while they are
// being read (it spends a slot and makes the familiar together) never shows
// the slot still free next to the familiar it just made.
func TestMR037_TheSummonOptionsAreOneSnapshotWhileTheCasterCasts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, wizard := h.newUser("Mestre"), h.newUser("Maga")
	campaign := h.newCampaign(master, "Mirathel", wizard)
	pensantus := wizard.createPensantus(t, campaign)

	h.hookAfterQuery("character_vitals", func() {
		tx, err := h.pool.Begin(context.Background())
		if err != nil {
			t.Errorf("begin: %v", err)
			return
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		// A casting: a first circle slot spent and a familiar made, in one transaction.
		if _, err := tx.Exec(context.Background(),
			`INSERT INTO character_vitals (character_id, hit_points_current, spell_slots_used, revision, updated_at) VALUES ($1, NULL, '{1}', 1, now())`,
			pensantus.GetId()); err != nil {
			t.Errorf("spend the slot from the hook: %v", err)
			return
		}
		if _, err := h.svc.SummonCreatures(context.Background(), tx, link.Summon{
			CampaignID: campaign, CharacterID: pensantus.GetId(), Source: "familiar", GroupID: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0aa",
			Creatures: []link.CreatureSpec{{MonsterKey: "monster:raven", Name: "Nanquim", Attack: "none"}},
		}); err != nil {
			t.Errorf("summon from the hook: %v", err)
			return
		}
		if err := tx.Commit(context.Background()); err != nil {
			t.Errorf("commit: %v", err)
		}
	})
	r, err := summonOptions(t, wizard, campaign, pensantus.GetId())
	if err != nil {
		t.Fatal(err)
	}
	free := r.GetSlots()[0].GetFree()
	replaced := len(spellOf(t, r, "spell:find-familiar").GetReplaces())
	if (free == 4) != (replaced == 0) {
		t.Errorf("GetSummonOptions(): %d first circle slots free with %d familiars to replace; want the options as they were (4, 0) or as they became (3, 1)", free, replaced)
	}

	// Positive control: the hook did cast.
	after, err := summonOptions(t, wizard, campaign, pensantus.GetId())
	if err != nil || after.GetSlots()[0].GetFree() != 3 || len(spellOf(t, after, "spell:find-familiar").GetReplaces()) != 1 {
		t.Errorf("options after the casting = %v, %v; want 3 free slots and one familiar", after, err)
	}
}
