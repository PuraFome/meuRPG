package characters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	goflag "flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// These tests freeze what the combat and the fog read from a sheet: CombatSpell (every
// kind of spell: attack, save, area, heal, hit point effect, summon, a caster that casts
// nothing), CombatSheet (every class the sheet names and both kinds of sheet, the dead
// included), the full sheet's reactions and PartyVision (each player of the party as the
// maps module reads them). They take no caller: each output is written to a golden file,
// one entry per character and call.

var updateCombatReadsGolden = goflag.Bool("update-combat-reads-golden", false, "write the golden files of the combat reads and of the campaign package part again")

// checkCombatReadsGolden compares got, as indented JSON, with testdata/<name>, or writes it
// with -update-combat-reads-golden.
func checkCombatReadsGolden(t *testing.T, name string, got any) {
	t.Helper()
	body, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	body = append(body, '\n')
	path := filepath.Join("testdata", name)
	if *updateCombatReadsGolden {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file (run with -update-combat-reads-golden to write it): %v", err)
	}
	if !bytes.Equal(body, want) {
		t.Errorf("%s differs from the golden file; diff:\n%s", path, vitalsGoldenDiff(string(want), string(body)))
	}
}

// combatReadRefusal is a refused read as the golden keeps it: the code and the message.
func combatReadRefusal(err error) map[string]string {
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		return map[string]string{"code": ce.Code().String(), "message": ce.Message()}
	}
	return map[string]string{"error": err.Error()}
}

// combatReadTx runs read in a transaction that is rolled back, as the combat's reads run
// inside play's transaction.
func combatReadTx(t *testing.T, h *harness, read func(tx pgx.Tx)) {
	t.Helper()
	tx, err := h.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(t.Context())) }()
	read(tx)
}

// combatReadMissingID is a well-formed character id that names nobody.
const combatReadMissingID = "00000000-0000-4000-8000-000000000001"

// TestCombatSpellGoldenOfEveryKind casts, as each kind of caster, the spells that reach
// every branch of CombatSpell, and the refusals: an unknown spell, a character that is not
// there or is dead.
func TestCombatSpellGoldenOfEveryKind(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogador")
	campaign := h.newCampaign(master, "Mirathel", player)
	enemy := charactersv1.CharacterKind_CHARACTER_KIND_ENEMY
	wizard := master.create(t, campaign, enemy, "Maga", casterSheet("class:wizard", "subclass:evocation", 9, 10, 14, 14, 18, 12, 10, nil, nil))
	cleric := master.create(t, campaign, enemy, "Clériga", casterSheet("class:cleric", "subclass:life", 9, 10, 10, 14, 10, 18, 12, nil, nil))
	fighter := master.create(t, campaign, enemy, "Guerreiro", enemySheet())
	goblin := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin", basicSheet())
	dead := player.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Morto", casterSheet("class:wizard", "", 1, 10, 14, 14, 16, 12, 10, nil, nil))
	master.markDead(t, dead)

	type cast struct {
		who        *charactersv1.Character
		id, spell  string
		slot       int
		damageType string
	}
	casts := map[string]cast{}
	for _, c := range []struct {
		spell string
		slot  int
	}{
		{"spell:fire-bolt", 0},
		{"spell:fireball", 3},
		{"spell:fireball", 5},
		{"spell:magic-missile", 1},
		{"spell:magic-missile", 3},
		{"spell:shield", 1},
		{"spell:sleep", 1},
		{"spell:false-life", 1},
		{"spell:find-familiar", 1},
		{"spell:contact-other-plane", 5},
		{"spell:cone-of-cold", 5},
		{"spell:hold-person", 3},
		{"spell:animate-dead", 3},
		{"spell:ray-of-frost", 0},
	} {
		casts[fmt.Sprintf("wizard %s@%d", c.spell, c.slot)] = cast{who: wizard, spell: c.spell, slot: c.slot}
	}
	for _, c := range []struct {
		spell string
		slot  int
	}{
		{"spell:cure-wounds", 1},
		{"spell:cure-wounds", 3},
		{"spell:healing-word", 1},
		{"spell:sacred-flame", 0},
		{"spell:spiritual-weapon", 2},
		{"spell:hold-person", 2},
		{"spell:spirit-guardians", 3},
		{"spell:flame-strike", 5},
		{"spell:mass-cure-wounds", 5},
		{"spell:guiding-bolt", 1},
	} {
		casts[fmt.Sprintf("cleric %s@%d", c.spell, c.slot)] = cast{who: cleric, spell: c.spell, slot: c.slot}
	}
	casts["cleric spell:flame-strike@6 radiant"] = cast{who: cleric, spell: "spell:flame-strike", slot: 6, damageType: "damage-type:radiant"}
	casts["fighter casting spell:fire-bolt (no spellcasting)"] = cast{who: fighter, spell: "spell:fire-bolt"}
	casts["fighter casting spell:cure-wounds (no spellcasting)"] = cast{who: fighter, spell: "spell:cure-wounds", slot: 1}
	// Finding: a character that casts nothing (a basic sheet, a fighter) casting a spell with a
	// saving throw keeps the save's ability with a DC of 0, so the target saves against 0.
	// Frozen as it is.
	casts["basic sheet casting spell:sacred-flame"] = cast{who: goblin, spell: "spell:sacred-flame"}
	casts["refused: an unknown spell"] = cast{who: wizard, spell: "spell:nao-existe", slot: 1}
	casts["refused: a dead character"] = cast{who: dead, spell: "spell:fire-bolt"}
	casts["refused: a character that is not there"] = cast{id: combatReadMissingID, spell: "spell:fire-bolt"}
	casts["refused: an id that is no UUID"] = cast{id: "nao-e-um-id", spell: "spell:fire-bolt"}

	got := map[string]any{}
	combatReadTx(t, h, func(tx pgx.Tx) {
		for name, c := range casts {
			id := c.id
			if c.who != nil {
				id = c.who.GetId()
			}
			sp, err := h.svc.CombatSpell(t.Context(), tx, campaign, id, c.spell, c.slot, c.damageType)
			if err != nil {
				got[name] = combatReadRefusal(err)
				continue
			}
			got[name] = sp
		}
	})
	checkCombatReadsGolden(t, "combat_spell.golden.json", got)
}

// TestCombatSheetGoldenOfEveryKind reads the combat sheet of each class the sheet names
// (the bard's die and Charisma, the fighter's and monk's levels, the paladin's pool), a
// basic sheet, one made from a creature and a dead character (the log names the attacks of
// the dead), and the refusals.
func TestCombatSheetGoldenOfEveryKind(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	player := h.newUser("Jogador")
	campaign := h.newCampaign(master, "Mirathel", player)
	enemy := charactersv1.CharacterKind_CHARACTER_KIND_ENEMY
	who := map[string]string{}
	add := func(name string, sheet *charactersv1.CharacterSheet) {
		who[name] = master.create(t, campaign, enemy, name, sheet).GetId()
	}
	add("bardo do conhecimento 3", casterSheet("class:bard", "subclass:lore", 3, 10, 14, 12, 10, 10, 18, nil, nil))
	add("guerreiro campeão 5", casterSheet("class:fighter", "subclass:champion", 5, 18, 12, 14, 10, 10, 8, nil, nil))
	add("monge da mão aberta 5", casterSheet("class:monk", "subclass:open-hand", 5, 10, 18, 12, 10, 16, 8, nil, nil))
	add("paladino 2", casterSheet("class:paladin", "", 2, 16, 10, 14, 8, 10, 16, nil, nil))
	add("bárbaro berserker 9", casterSheet("class:barbarian", "subclass:berserker", 9, 18, 14, 16, 8, 10, 8, nil, nil))
	add("guerreiro de cota de malha", enemySheet())
	who["goblin de ficha básica"] = master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin", basicSheet()).GetId()
	ogre, err := npcFromCreature(t, master, campaign, "monster:ogre", "Grak", charactersv1.CharacterKind_CHARACTER_KIND_MINION, "00000000-0000-4000-8000-0000000000a1")
	if err != nil {
		t.Fatalf("npcFromCreature() error = %v", err)
	}
	who["ogro do bestiário"] = ogre.GetId()
	pc := player.createPensantus(t, campaign)
	master.markDead(t, pc)
	who["Pensantus, morto"] = pc.GetId()
	who["refused: a character that is not there"] = combatReadMissingID
	who["refused: an id that is no UUID"] = "nao-e-um-id"

	got := map[string]any{}
	combatReadTx(t, h, func(tx pgx.Tx) {
		for name, id := range who {
			sheet, err := h.svc.CombatSheet(t.Context(), tx, campaign, id)
			if err != nil {
				got[name] = combatReadRefusal(err)
				continue
			}
			got[name] = sheet
		}
	})
	checkCombatReadsGolden(t, "combat_sheet.golden.json", got)
}

// TestReactionStatsGoldenOfEveryKind reads the reactions of each kind of combatant through
// ReactionStats: full sheets (a wizard's prepared spells, a warlock's pact, a rogue, a monk,
// a lore bard with each "Perguntar" setting, a tiefling before and after the 3rd level, a
// fighter that casts nothing), a basic sheet and one made from a creature.
func TestReactionStatsGoldenOfEveryKind(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")
	enemy := charactersv1.CharacterKind_CHARACTER_KIND_ENEMY
	who := map[string]string{}
	add := func(name string, sheet *charactersv1.CharacterSheet) {
		who[name] = master.create(t, campaign, enemy, name, sheet).GetId()
	}
	tiefling := func(class, sub string, level int32) *charactersv1.CharacterSheet {
		s := casterSheet(class, sub, level, 10, 14, 12, 10, 10, 16, nil, nil)
		s.GetFull().RaceKey = "race:tiefling"
		return s
	}
	bard := func(ask charactersv1.CuttingWordsAsk) *charactersv1.CharacterSheet {
		s := casterSheet("class:bard", "subclass:lore", 3, 10, 14, 12, 10, 10, 18, nil, nil)
		s.GetFull().CuttingWordsAsk = ask
		return s
	}
	add("mago com magias preparadas", pensantusSheet())
	add("bruxo tiefling 3", tiefling("class:warlock", "subclass:fiend", 3))
	add("feiticeiro tiefling 1", tiefling("class:sorcerer", "subclass:draconic", 1))
	add("ladino 5", casterSheet("class:rogue", "subclass:thief", 5, 10, 18, 12, 10, 12, 10, nil, nil))
	add("monge 3", casterSheet("class:monk", "subclass:open-hand", 3, 10, 18, 12, 10, 16, 8, nil, nil))
	add("bardo, pergunta padrão", bard(charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_UNSPECIFIED))
	add("bardo, pergunta sempre", bard(charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL))
	add("bardo, pergunta nunca", bard(charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_NEVER))
	add("guerreiro que não conjura", enemySheet())
	who["goblin de ficha básica"] = master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_MINION, "Goblin", basicSheet()).GetId()
	mage, err := npcFromCreature(t, master, campaign, "monster:mage", "Arquimago", charactersv1.CharacterKind_CHARACTER_KIND_MINION, "00000000-0000-4000-8000-0000000000a2")
	if err != nil {
		t.Fatalf("npcFromCreature(mage) error = %v", err)
	}
	who["mago do bestiário"] = mage.GetId()
	who["refused: a character that is not there"] = combatReadMissingID

	got := map[string]any{}
	combatReadTx(t, h, func(tx pgx.Tx) {
		for name, id := range who {
			st, err := h.svc.ReactionStats(t.Context(), tx, campaign, id)
			if err != nil {
				got[name] = combatReadRefusal(err)
				continue
			}
			got[name] = st
		}
	})
	checkCombatReadsGolden(t, "reaction_stats.golden.json", got)
}

// TestFullReactionStatsOfHandMadeSheets feeds fullReactionStats derived sheets the SRD
// cannot make (a table maneuver that reduces a melee hit, every feature at once, nothing at
// all), so each branch is frozen without a database.
func TestFullReactionStatsOfHandMadeSheets(t *testing.T) {
	t.Parallel()
	everything := rules.Derived{
		TotalLevel: 3, ProficiencyBonus: 2,
		Abilities: []rules.AbilityScore{{Ability: rules.DEX, Modifier: 3}, {Ability: rules.CHA, Modifier: 4}, {Ability: rules.STR, Modifier: 1}},
		Spells: []rules.CharacterSpell{
			{Spell: rules.SpellEntry{Key: "spell:shield"}, Prepared: true},
			{Spell: rules.SpellEntry{Key: "spell:fireball"}},
		},
		SpellSlots: []int{4, 2},
		Resources:  []rules.Resource{{Key: "ki", Max: 3}, {Key: "superiority", Max: 4}},
		PactMagic:  &rules.PactMagic{SlotLevel: 2, Slots: 2},
		Spellcasting: []rules.Spellcasting{
			{Class: "class:wizard", Ability: rules.INT, SaveDC: 13},
			{Class: "class:bard", Ability: rules.CHA, SaveDC: 14},
		},
		Features: []rules.Feature{
			{Key: "feature:uncanny-dodge"}, {Key: "feature:deflect-missiles"}, {Key: "feature:cutting-words"}, {Key: "trait:infernal-legacy"}, {Key: "feature:other"},
		},
		Classes: []rules.DerivedClass{{ClassKey: "class:monk", Level: 2}, {ClassKey: "class:bard", Level: 1}, {ClassKey: "class:fighter", Level: 1}},
		Maneuvers: []rules.Maneuver{
			{Key: "feature:aparar@mesa", NamePT: "Aparar", Resource: "superiority", Sides: 8, Applies: rules.ManeuverReduceMelee, Ability: "str"},
			{Key: "feature:golpe@mesa", NamePT: "Golpe", Resource: "superiority", Sides: 8, Applies: rules.ManeuverDamage},
		},
	}
	youngTiefling := rules.Derived{TotalLevel: 2, Features: []rules.Feature{{Key: "trait:infernal-legacy"}}}
	got := map[string]any{
		"every branch, asking for every attack": fullReactionStats(everything, &charactersv1.FullSheet{CuttingWordsAsk: charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL}),
		"a tiefling below the 3rd level":        fullReactionStats(youngTiefling, &charactersv1.FullSheet{}),
		"nothing derived, no full sheet":        fullReactionStats(rules.Derived{}, nil),
	}
	checkCombatReadsGolden(t, "full_reaction_stats.golden.json", got)
}

// TestPartyVisionGoldenOfEveryViewer reads the party as the maps module does: a gnome with
// darkvision, a druid in a wolf's shape, a wizard looking through an owl, one looking
// through a creature the bestiary does not have, one whose familiar was dismissed, a dead
// character, and one whose player deleted the account (left out), next to an NPC (never
// part of the party). The ids are replaced by the characters' names.
func TestPartyVisionGoldenOfEveryViewer(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	players := make([]*user, 7)
	for i := range players {
		players[i] = h.newUser(fmt.Sprintf("Jogador %d", i+1))
	}
	campaign := h.newCampaign(master, "Mirathel", players...)
	names := map[string]string{}
	create := func(p *user, name string, sheet *charactersv1.CharacterSheet) *charactersv1.Character {
		c := p.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, name, sheet)
		names[c.GetId()] = name
		names[p.id] = "player of " + name
		return c
	}
	create(players[0], "Gnomo", pensantusSheet())
	druid := create(players[1], "Druida", casterSheet("class:druid", "", 2, 10, 12, 14, 10, 16, 10, nil, nil))
	owl := create(players[2], "Coruja", wizardAt(1))
	odd := create(players[3], "Estranho", wizardAt(1))
	gone := create(players[4], "Dispensado", wizardAt(1))
	dead := create(players[5], "Morto", wizardAt(1))
	deleted := create(players[6], "Sem conta", wizardAt(1))
	master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Inimigo", enemySheet())
	master.markDead(t, dead)
	h.deleteUser(players[6].id)
	_ = deleted

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := h.pool.Exec(t.Context(), sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`INSERT INTO character_wild_shapes (character_id, beast, hp, updated_at) VALUES ($1, 'monster:wolf', 11, now())`, druid.GetId())
	familiar := func(c *charactersv1.Character, monsterKey string, dismissed bool) {
		t.Helper()
		var id string
		if err := h.pool.QueryRow(t.Context(), `INSERT INTO character_creatures
			(campaign_id, character_id, monster_key, name, source, attack, summon_group_id, hp_current, hp_max, created_at)
			VALUES ($1, $2, $3, 'Familiar', 'familiar', 'none', gen_random_uuid(), 1, 1, now()) RETURNING id`,
			campaign, c.GetId(), monsterKey).Scan(&id); err != nil {
			t.Fatalf("insert a familiar: %v", err)
		}
		names[id] = "familiar of " + names[c.GetId()]
		if err := db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
			_, err := h.svc.SetFamiliarSight(t.Context(), tx, campaign, c.GetId(), id, false, nil)
			return err
		}); err != nil {
			t.Fatalf("SetFamiliarSight() error = %v", err)
		}
		if dismissed {
			exec(`UPDATE character_creatures SET dismissed_at = now(), dismissed_reason = 'owner' WHERE id = $1`, id)
		}
	}
	familiar(owl, "monster:owl", false)
	familiar(odd, "monster:nao-existe", false)
	familiar(gone, "monster:owl", true)

	var got []byte
	combatReadTx(t, h, func(tx pgx.Tx) {
		members, err := h.svc.PartyVision(t.Context(), tx, campaign)
		if err != nil {
			t.Fatalf("PartyVision() error = %v", err)
		}
		if got, err = json.Marshal(members); err != nil {
			t.Fatal(err)
		}
	})
	text := string(got)
	for id, name := range names {
		text = strings.ReplaceAll(text, id, name)
	}
	var out any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	checkCombatReadsGolden(t, "party_vision.golden.json", out)
}
