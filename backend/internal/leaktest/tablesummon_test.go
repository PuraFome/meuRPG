package leaktest

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1/rulesv1connect"
)

// A table spell that summons a table creature (#578). The spell is public (every member reads that it exists), but
// what it summons is not: the creature's key and the count formula are the master's, and the caster's. A member who
// holds none of the creature reads neither in the table's entries; the caster reads the key and the name of the creature
// in the options of the spell (she must pick it) and, once she holds it, in the entries too. Caio, who holds none,
// reads neither at any step. The count formula is the master's alone.
func TestATableSpellThatSummonsKeepsItsCreatureAndCountFromThoseWhoHoldNone(t *testing.T) {
	const countFormula = "1 + 5 * slot()"
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m := w.master
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()

	creature := must(m.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableCreature{TableCreature: &rulesv1.TableCreature{
			NamePt: w.secrets.marker("summon-creature-name", w.ana), BaseKey: "monster:giant-crab", HitPoints: 31,
		}},
	}))).GetEntry()
	creatureKey := creature.GetKey()
	w.secrets.id("summon-creature-key", creatureKey, w.ana)
	w.secrets.add(&canary{needle: countFormula, kind: "summon-count-formula"})
	spell := must(m.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: &rulesv1.TableSpell{
			NamePt: w.secrets.public("summon-spell-name"), Level: 1, SchoolKey: "school:conjuration",
			CastingTime: &rulesv1.TableSpellCastingTime{Unit: rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_ACTION, Amount: 1},
			Range:       &rulesv1.TableSpellRange{Kind: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF},
			Duration:    &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_INSTANTANEOUS},
			Components:  &rulesv1.TableSpellComponents{Verbal: true, Somatic: true},
			ClassKeys:   []string{"class:wizard"}, DescPt: []string{w.secrets.public("summon-spell-text")},
			Target: &rulesv1.TableSpellTarget{Kind: rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF},
			Summon: &rulesv1.TableSummon{CreatureKeys: []string{creatureKey}, Count: countFormula, MaxLive: 6},
		}},
	}))).GetEntry()
	spells := []string{spell.GetKey()}
	w.pens = must(w.ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8},
			RaceKey:    "race:gnome", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 5}},
			KnownSpellKeys: spells, PreparedSpellKeys: spells,
		}}},
	}))).GetCharacter()
	w.toren = w.pc(w.caio, "Toren", "race:human")

	entries := read{
		procedure: rulesv1connect.TableContentServiceListTableEntriesProcedure, allow: members,
		why: "the creature's own entry is the holder's (Ana, once she holds one); the spell's summon names none the reader does not hold",
		req: func(w *world) proto.Message { return &rulesv1.ListTableEntriesRequest{CampaignId: w.campaign} },
	}
	options := read{
		procedure: charactersv1connect.CharacterServiceGetSummonOptionsProcedure, label: "Ana's character", allow: onlyAna,
		req: func(w *world) proto.Message {
			return &charactersv1.GetSummonOptionsRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()}
		},
	}
	got := newAnswers()

	// Before the cast Ana holds no creature: the options name the creature to her (she picks it), the entries do not.
	// Ana is a reader of the creature's canaries (she reads it in the options, then holds it), so the matrix cannot
	// refuse her the key in the entries before the cast: the explicit checks below hold that line.
	t.Run("before the cast", func(t *testing.T) {
		w.runRead(t, options, got)
		w.runRead(t, entries, got)
	})
	for _, p := range []*person{w.ana, w.caio} {
		body := string(p.call(rulesv1connect.TableContentServiceListTableEntriesProcedure, &rulesv1.ListTableEntriesRequest{CampaignId: w.campaign}).body)
		if strings.Contains(body, creatureKey) || strings.Contains(body, countFormula) {
			t.Errorf("%s, who holds no creature, reads the summon's creature or count in the entries: %s", p.name, shorten([]byte(body)))
		}
		if !strings.Contains(body, spell.GetKey()) {
			t.Errorf("%s does not read the spell in the entries: the control is missing", p.name)
		}
	}
	masterBody := string(m.call(rulesv1connect.TableContentServiceListTableEntriesProcedure, &rulesv1.ListTableEntriesRequest{CampaignId: w.campaign}).body)
	if !strings.Contains(masterBody, creatureKey) || !strings.Contains(masterBody, countFormula) {
		t.Errorf("the master does not read the creature and the count in the entries (the positive control): %s", shorten([]byte(masterBody)))
	}
	opts := w.ana.call(charactersv1connect.CharacterServiceGetSummonOptionsProcedure, &charactersv1.GetSummonOptionsRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()})
	if !strings.Contains(string(opts.body), creatureKey) {
		t.Errorf("Ana's options do not name the creature (the positive control): %s", shorten(opts.body))
	}

	// The cast: Ana now holds the creatures. Her entries carry the creature (it is hers), Caio's still do not.
	must(w.ana.play.CastSummon(ctx, rq(&playv1.CastSummonRequest{
		CampaignId: w.campaign, CharacterId: w.pens.GetId(), SpellKey: spell.GetKey(), Slot: &playv1.SpellSlot{Level: 1}, IdempotencyKey: newKey(),
		Summon: &playv1.SummonChoice{CreatureKeys: []string{creatureKey, creatureKey, creatureKey, creatureKey, creatureKey, creatureKey}},
	})))
	rows := []read{
		options, entries,
		{
			procedure: charactersv1connect.CharacterServiceListCharacterCreaturesProcedure, allow: onlyAna,
			req: func(w *world) proto.Message {
				return &charactersv1.ListCharacterCreaturesRequest{CampaignId: w.campaign, CharacterId: w.pens.GetId()}
			},
		},
		{
			procedure: rulesv1connect.ContentServiceGetCreatureProcedure, label: "the summoned creature", allow: onlyAna, why: "a table creature is not found by a player who holds none",
			req: func(w *world) proto.Message {
				return &rulesv1.GetCreatureRequest{CampaignId: w.campaign, Key: creatureKey}
			},
		},
	}
	for _, r := range rows {
		t.Run("after the cast "+r.name(), func(t *testing.T) { w.runRead(t, r, got) })
	}
	t.Run("canaries are reachable", func(t *testing.T) { checkCanariesAreReachable(t, w, got) })
}
