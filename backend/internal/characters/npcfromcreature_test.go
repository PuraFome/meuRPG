package characters

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TestNpcSheetFromCreature: the ogre of the "Criar NPC" design (E10-08) and a
// creature that does not walk and one whose attacks the sheet cannot say.
func TestNpcSheetFromCreature(t *testing.T) {
	t.Parallel()
	content := loadRules(t)
	ogre, err := npcSheetFromCreature(content, "monster:ogre")
	if err != nil {
		t.Fatal(err)
	}
	if ogre.GetArmorClass() != 11 || ogre.GetHitPointsMax() != 59 || ogre.GetSpeedFt() != 40 || ogre.GetChallengeRating() != "2" ||
		ogre.GetXpValue() != 450 || ogre.GetSize() != rulesv1.CreatureSize_CREATURE_SIZE_LARGE || ogre.GetMonsterKey() != "monster:ogre" {
		t.Errorf("ogre = %v", ogre)
	}
	if a := ogre.GetAbilityScores(); a.GetStrength() != 19 || a.GetDexterity() != 8 || a.GetConstitution() != 16 || a.GetIntelligence() != 5 ||
		a.GetWisdom() != 7 || a.GetCharisma() != 7 || ogre.GetInitiativeBonus() != -1 {
		t.Errorf("ogre abilities = %v, initiative %d", a, ogre.GetInitiativeBonus())
	}
	if len(ogre.GetAttacks()) != 2 {
		t.Fatalf("ogre attacks = %v, want the greatclub and the javelin", ogre.GetAttacks())
	}
	if g := ogre.GetAttacks()[0]; g.GetAttackBonus() != 6 || g.GetDamageDiceCount() != 2 || g.GetDamageDiceSides() != 8 || g.GetDamageBonus() != 4 ||
		g.GetDamageType() != charactersv1.DamageType_DAMAGE_TYPE_BLUDGEONING || g.GetRangeFt() != 0 {
		t.Errorf("ogre melee attack = %v", g)
	}
	if j := ogre.GetAttacks()[1]; j.GetRangeFt() != 30 {
		t.Errorf("the javelin's range = %v", j)
	}

	// The creature's own name is in the description, in Portuguese.
	if ogre.GetDescription() != "Baseado em Ogro (SRD 5.1)." {
		t.Errorf("description = %q", ogre.GetDescription())
	}

	// A creature that does not walk takes its best other speed.
	if shark, err := npcSheetFromCreature(content, "monster:reef-shark"); err != nil || shark.GetSpeedFt() != 40 {
		t.Errorf("reef shark = %v, %v", shark, err)
	}
	if _, err := npcSheetFromCreature(content, "spell:fireball"); err == nil {
		t.Error("a spell is not a creature")
	}
}

// TestNpcSheetAttacks: the dragon's extra fire is written in the description
// instead of being cut, spell attacks come along, and every attack has a
// Portuguese name.
func TestNpcSheetAttacks(t *testing.T) {
	t.Parallel()
	content := loadRules(t)
	dragon, err := npcSheetFromCreature(content, "monster:adult-red-dragon")
	if err != nil {
		t.Fatal(err)
	}
	if len(dragon.GetAttacks()) != 3 || dragon.GetAttacks()[0].GetName() != "Mordida" ||
		dragon.GetAttacks()[0].GetDamageType() != charactersv1.DamageType_DAMAGE_TYPE_PIERCING ||
		!strings.Contains(dragon.GetDescription(), "Mordida: +2d6 fogo.") {
		t.Errorf("dragon attacks = %v, description %q", dragon.GetAttacks(), dragon.GetDescription())
	}
	for _, key := range []string{"monster:lich", "monster:specter", "monster:will-o-wisp"} {
		b, err := npcSheetFromCreature(content, key)
		if err != nil || len(b.GetAttacks()) == 0 {
			t.Errorf("%s: attacks = %v, %v; want its spell attack", key, b.GetAttacks(), err)
		}
	}
	for _, key := range []string{"monster:ogre", "monster:goblin", "monster:bandit-captain", "monster:wolf"} {
		b, _ := npcSheetFromCreature(content, key)
		for _, a := range b.GetAttacks() {
			if a.GetName() == "" || strings.ContainsAny(a.GetName()[:1], "abcdefghijklmnopqrstuvwxyz") {
				t.Errorf("%s: attack name %q", key, a.GetName())
			}
		}
	}
}

// TestEveryCreatureMakesAnNpc: all 334 stat blocks pass the checks of a basic
// sheet, so "Criar NPC" never fails on a creature of the bestiary.
func TestEveryCreatureMakesAnNpc(t *testing.T) {
	t.Parallel()
	content := loadRules(t)
	list, err := content.ListCreatures(rules.CreatureFilter{})
	if err != nil || len(list) != 334 {
		t.Fatalf("creatures = %d, %v", len(list), err)
	}
	for _, e := range list {
		b, err := npcSheetFromCreature(content, e.Key)
		if err != nil {
			t.Errorf("%s: %v", e.Key, err)
			continue
		}
		if err := checkChallenge(content, "sheet.basic", b.GetChallengeRating(), b.GetXpValue()); err != nil {
			t.Errorf("%s: %v", e.Key, err)
		}
		if b.GetHitPointsMax() != i32(e.HitPoints) || b.GetArmorClass() != i32(e.ArmorClass) || len(b.GetAttacks()) > maxBasicAttacks {
			t.Errorf("%s: sheet %v", e.Key, b)
		}
	}
}

func TestCreatureSizeWord(t *testing.T) {
	t.Parallel()
	for size, want := range map[rulesv1.CreatureSize]string{
		rulesv1.CreatureSize_CREATURE_SIZE_UNSPECIFIED: "", rulesv1.CreatureSize_CREATURE_SIZE_TINY: "Tiny",
		rulesv1.CreatureSize_CREATURE_SIZE_GARGANTUAN: "Gargantuan",
	} {
		if got, ok := creatureSizeWord(size); !ok || got != want {
			t.Errorf("creatureSizeWord(%v) = %q, %v; want %q", size, got, ok, want)
		}
	}
	if _, ok := creatureSizeWord(99); ok {
		t.Error("99 is not a size")
	}
}

// TestListCreaturesBestiaryRequests: the size and the range filters, the rows of
// the list ("Lobo · Wolf · SRD", CA, PV), and the bad requests; and that the
// Etapa 9 requests (no size, no min_cr) answer what the rules module lists.
func TestListCreaturesBestiaryRequests(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	list := func(req *rulesv1.ListCreaturesRequest) (*rulesv1.ListCreaturesResponse, error) {
		req.CampaignId = campaign
		res, err := player.content.ListCreatures(t.Context(), connect.NewRequest(req))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}

	lobo, err := list(&rulesv1.ListCreaturesRequest{Query: "lobo"})
	if err != nil || lobo.GetTotal() < 4 {
		t.Fatalf("lobo = %v, %v", lobo, err)
	}
	var spider *rulesv1.CreatureSummary
	for _, c := range lobo.GetCreatures() {
		if c.GetKey() == "monster:giant-wolf-spider" {
			spider = c
		}
	}
	if spider == nil || spider.GetName() != "Giant Wolf Spider" || spider.GetArmorClass() != 13 || spider.GetHitPoints() != 11 {
		t.Errorf("the wolf spider's row = %v", spider)
	}
	if byEnglish, err := list(&rulesv1.ListCreaturesRequest{Query: "wolf"}); err != nil || byEnglish.GetTotal() < lobo.GetTotal() {
		t.Errorf("\"wolf\" finds %v, \"lobo\" %d: %v", byEnglish.GetTotal(), lobo.GetTotal(), err)
	}

	large, err := list(&rulesv1.ListCreaturesRequest{Size: rulesv1.CreatureSize_CREATURE_SIZE_LARGE, MinCr: "1", MaxCr: "3", PageSize: 100})
	if err != nil || large.GetTotal() == 0 {
		t.Fatalf("large = %v, %v", large, err)
	}
	for _, c := range large.GetCreatures() {
		if c.GetSize() != "Large" || !slices.Contains([]string{"1", "2", "3"}, c.GetChallengeRating()) {
			t.Errorf("%s is %s, CR %s", c.GetKey(), c.GetSize(), c.GetChallengeRating())
		}
	}
	// A page token is good for the same size and range only.
	first, err := list(&rulesv1.ListCreaturesRequest{Size: rulesv1.CreatureSize_CREATURE_SIZE_LARGE, PageSize: 1})
	if err != nil || first.GetNextPageToken() == "" {
		t.Fatalf("first page = %v, %v", first, err)
	}
	_, err = list(&rulesv1.ListCreaturesRequest{Size: rulesv1.CreatureSize_CREATURE_SIZE_HUGE, PageSize: 1, PageToken: first.GetNextPageToken()})
	wantCode(t, "a token of another size", err, connect.CodeInvalidArgument)
	_, err = list(&rulesv1.ListCreaturesRequest{Size: rulesv1.CreatureSize_CREATURE_SIZE_LARGE, MinCr: "1", PageSize: 1, PageToken: first.GetNextPageToken()})
	wantCode(t, "a token of another range", err, connect.CodeInvalidArgument)

	for name, req := range map[string]*rulesv1.ListCreaturesRequest{
		"min above max":  {MinCr: "5", MaxCr: "4"},
		"a bad min_cr":   {MinCr: "1/3"},
		"a bad size":     {Size: 99},
		"a bad max_cr":   {MaxCr: "40"},
		"a min over 30":  {MinCr: "31"},
		"a fraction too": {MinCr: "1/2", MaxCr: "1/4"},
	} {
		_, err := list(req)
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}

	// The Etapa 9 callers: the same rows as the rules module's list for the same
	// filters (the Wild Shape list and the summon choices are such requests).
	content := loadRules(t)
	for _, f := range []rules.CreatureFilter{
		{Type: "beast", MaxCR: "1/4", NoFly: true, NoSwim: true},
		{Type: "beast", MaxCR: "2"},
		{Query: "cobra"},
		{},
	} {
		want, err := content.ListCreatures(f)
		if err != nil {
			t.Fatal(err)
		}
		got, err := list(&rulesv1.ListCreaturesRequest{
			Type: f.Type, MaxCr: f.MaxCR, Query: f.Query, NoFly: f.NoFly, NoSwim: f.NoSwim, PageSize: 100,
		})
		if err != nil || int(got.GetTotal()) != len(want) {
			t.Fatalf("%+v: total = %v, %v; want %d", f, got.GetTotal(), err, len(want))
		}
		for i, c := range got.GetCreatures() {
			if c.GetKey() != want[i].Key || c.GetNamePt() != want[i].NamePT || c.GetXp() != i32(want[i].XP) {
				t.Errorf("%+v: row %d = %v, want %s", f, i, c, want[i].Key)
			}
		}
	}
}

func npcFromCreature(t *testing.T, u *user, campaign, key, name string, kind charactersv1.CharacterKind, idem string) (*charactersv1.Character, error) {
	res, err := u.api.CreateNpcFromCreature(t.Context(), connect.NewRequest(&charactersv1.CreateNpcFromCreatureRequest{
		CampaignId: campaign, CreatureKey: key, Name: name, Kind: kind, IdempotencyKey: idem,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

// TestCreateNpcFromCreature: the master makes "Grak, o ogro" from the bestiary:
// the numbers come from the stat block, only the master may, the same key
// answers the same NPC, and an edit keeps the link to the creature.
func TestCreateNpcFromCreature(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, "Mirathel", player)
	minion := charactersv1.CharacterKind_CHARACTER_KIND_MINION

	key := uuid.New().String()
	grak, err := npcFromCreature(t, master, campaign, "monster:ogre", "  Grak, o ogro ", minion, key)
	if err != nil {
		t.Fatal(err)
	}
	b := grak.GetSheet().GetBasic()
	if grak.GetName() != "Grak, o ogro" || grak.GetKind() != minion || b.GetMonsterKey() != "monster:ogre" || b.GetArmorClass() != 11 ||
		b.GetHitPointsMax() != 59 || b.GetSpeedFt() != 40 || b.GetAbilityScores().GetStrength() != 19 || len(b.GetAttacks()) != 2 ||
		b.GetChallengeRating() != "2" || b.GetXpValue() != 450 || grak.GetRevision() != 1 {
		t.Errorf("Grak = %v", grak)
	}
	if grak.GetState() != charactersv1.CharacterState_CHARACTER_STATE_DRAFT {
		t.Errorf("an NPC never locks: state = %v", grak.GetState())
	}

	// Hidden by default (RN-10): the player does not list it and cannot read it.
	for _, c := range player.list(t, campaign) {
		if c.GetId() == grak.GetId() {
			t.Error("the player lists the master's NPC")
		}
	}
	_, err = player.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: campaign, CharacterId: grak.GetId()}))
	wantCode(t, "a player reads the NPC", err, connect.CodeNotFound)
	var listed bool
	for _, c := range master.list(t, campaign) {
		listed = listed || c.GetId() == grak.GetId()
	}
	if !listed {
		t.Error("the master does not list the NPC")
	}

	// Idempotent: the same key and request answers the same NPC; a different
	// request with that key is refused; a new key makes a second NPC.
	again, err := npcFromCreature(t, master, campaign, "monster:ogre", "Grak, o ogro", minion, key)
	if err != nil || again.GetId() != grak.GetId() {
		t.Errorf("the same key: %v, %v; want NPC %s", again.GetId(), err, grak.GetId())
	}
	var n int
	if err := h.pool.QueryRow(t.Context(), "SELECT count(*) FROM characters WHERE campaign_id = $1", campaign).Scan(&n); err != nil || n != 1 {
		t.Errorf("characters after a retry = %d, %v; want 1", n, err)
	}
	_, err = npcFromCreature(t, master, campaign, "monster:goblin", "Grak, o ogro", minion, key)
	wantCode(t, "the key of another creature", err, connect.CodeInvalidArgument)
	_, err = npcFromCreature(t, master, campaign, "monster:ogre", "Outro nome", minion, key)
	wantCode(t, "the key of another name", err, connect.CodeInvalidArgument)
	second, err := npcFromCreature(t, master, campaign, "monster:ogre", "Grak, o ogro", minion, uuid.New().String())
	if err != nil || second.GetId() == grak.GetId() {
		t.Errorf("a new key makes a new NPC: %v, %v", second.GetId(), err)
	}

	// The bad requests.
	for name, tt := range map[string]struct {
		creature, name, key string
		kind                charactersv1.CharacterKind
	}{
		"not a creature":   {"spell:fireball", "X", uuid.New().String(), minion},
		"no creature":      {"", "X", uuid.New().String(), minion},
		"no name":          {"monster:ogre", "  ", uuid.New().String(), minion},
		"a long name":      {"monster:ogre", strings.Repeat("a", 200), uuid.New().String(), minion},
		"no key":           {"monster:ogre", "X", "", minion},
		"a key not a UUID": {"monster:ogre", "X", "abc", minion},
		"no kind":          {"monster:ogre", "X", uuid.New().String(), charactersv1.CharacterKind_CHARACTER_KIND_UNSPECIFIED},
		"a boss":           {"monster:ogre", "X", uuid.New().String(), charactersv1.CharacterKind_CHARACTER_KIND_BOSS},
		"a player":         {"monster:ogre", "X", uuid.New().String(), charactersv1.CharacterKind_CHARACTER_KIND_PLAYER},
	} {
		_, err := npcFromCreature(t, master, campaign, tt.creature, tt.name, tt.kind, tt.key)
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
	_, err = npcFromCreature(t, player, campaign, "monster:ogre", "Grak", minion, uuid.New().String())
	wantCode(t, "a player", err, connect.CodePermissionDenied)
	_, err = h.newUser("De fora").api.CreateNpcFromCreature(t.Context(), connect.NewRequest(&charactersv1.CreateNpcFromCreatureRequest{
		CampaignId: campaign, CreatureKey: "monster:ogre", Name: "Grak", Kind: minion, IdempotencyKey: uuid.New().String(),
	}))
	wantCode(t, "a non-member", err, connect.CodeNotFound)

	// A story NPC works too, and the sheet can be edited without losing the link
	// (the client never sends monster_key or the ability scores).
	story, err := npcFromCreature(t, master, campaign, "monster:commoner", "Dona Berta", charactersv1.CharacterKind_CHARACTER_KIND_STORY, uuid.New().String())
	if err != nil {
		t.Fatal(err)
	}
	edited := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{
		HitPointsMax: 20, ArmorClass: 12, SpeedFt: 30, MonsterKey: "monster:dragon", Description: "Mais forte.",
	}}}
	got, err := master.update(t, story, "Dona Berta, a forte", edited)
	if err != nil {
		t.Fatal(err)
	}
	eb := got.GetSheet().GetBasic()
	if eb.GetHitPointsMax() != 20 || eb.GetMonsterKey() != "monster:commoner" || eb.GetAbilityScores().GetStrength() != 10 {
		t.Errorf("after the edit: monster_key %q, abilities %v, hp %d", eb.GetMonsterKey(), eb.GetAbilityScores(), eb.GetHitPointsMax())
	}

	// An NPC the master typed cannot claim a creature through the API.
	typed := master.create(t, campaign, minion, "Goblin do mestre", &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{
		HitPointsMax: 7, ArmorClass: 15, SpeedFt: 30, MonsterKey: "monster:goblin", AbilityScores: &rulesv1.AbilityScores{Strength: 30},
	}}})
	if tb := typed.GetSheet().GetBasic(); tb.GetMonsterKey() != "" || tb.GetAbilityScores() != nil {
		t.Errorf("a typed NPC kept %q and %v", tb.GetMonsterKey(), tb.GetAbilityScores())
	}
}

// TestCreateNpcFromCreatureRace: six calls with the same key at once, five times
// over, all answer the same NPC and leave one row per key.
func TestCreateNpcFromCreatureRace(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 8)
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")
	minion := charactersv1.CharacterKind_CHARACTER_KIND_MINION
	const callers = 6
	for round := range 5 {
		key := uuid.New().String()
		ids := make([]string, callers)
		errs := make([]error, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := npcFromCreature(t, master, campaign, "monster:ogre", "Grak", minion, key)
				errs[i] = err
				if err == nil {
					ids[i] = c.GetId()
				}
			}()
		}
		wg.Wait()
		for i := range callers {
			if errs[i] != nil {
				t.Fatalf("round %d, call %d: %v", round, i, errs[i])
			}
			if ids[i] != ids[0] {
				t.Errorf("round %d: call %d made NPC %s, call 0 made %s", round, i, ids[i], ids[0])
			}
		}
		var n int
		if err := h.pool.QueryRow(t.Context(), "SELECT count(*) FROM characters WHERE campaign_id = $1 AND create_key = $2", campaign, key).Scan(&n); err != nil || n != 1 {
			t.Errorf("round %d: %d rows for the key, %v; want 1", round, n, err)
		}
	}
}
