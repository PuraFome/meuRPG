package leaktest

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The aura of a table feature (#579, RN-10) is the owner's: the bearer's player and the master read its label, its source
// and its radius; another player, in whose reach it stands, and the TV read none of them in the combat, the log, the lists
// of effects or the screen. The bearer is Caio's Toren (the aura is written when he joins, owner-only), Ana's Pensantus
// stands inside its radius.
func TestAnOwnerOnlyAuraOfAFeatureReachesNoOtherReader(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m := w.master
	w.imgMap = w.image("aura-map-image", 240, 160)
	w.fogMap = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: w.secrets.public("aura-map-name"), ImageId: w.imgMap}))).GetMap().GetId()
	must(m.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	must(m.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: 24})))
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))

	label := w.secrets.marker("aura-tag", w.caio)
	const radiusFt = 35
	w.secrets.number("feature-aura-radius", radiusFt, "radius")
	sub := must(m.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: &rulesv1.TableSubclass{
			NamePt: "Guardião do Posto", ClassKey: "class:fighter", DescPt: []string{"Um posto de vigia."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{{NamePt: "Sinal do Posto", DescPt: []string{"Fique de olho."}, Effects: []*rulesv1.TableEffect{{Type: "aura", Aura: &rulesv1.TableSpellEffect{
				Aura:          &rulesv1.TableEffectAura{RadiusFt: radiusFt, Affects: rulesv1.TableAuraAffects_TABLE_AURA_AFFECTS_ALLIES},
				Visibility:    rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER,
				PlayerLabelPt: label,
				Modifiers: []*rulesv1.TableEffectModifier{{
					Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 2,
					AppliesTo: []rulesv1.TableEffectRollKind{rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK},
				}},
			}}}}}}},
		}},
	}))).GetEntry()
	w.secrets.id("feature-aura-source", sub.GetTableSubclass().GetLevels()[0].GetFeatures()[0].GetKey(), w.caio)
	w.toren = must(w.caio.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Toren",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: sub.GetKey()}}},
			WeaponKeys: []string{"equipment:longsword"},
		}}},
	}))).GetCharacter()
	pens := w.pc(w.ana, "Pensantus", "race:gnome")
	goblin, _ := w.npc("aura-goblin", &charactersv1.BasicSheet{HitPointsMax: 20, ArmorClass: 12, SpeedFt: 30})
	w.allow("aura-goblin-name", w.ana, w.caio)
	revealed := false
	e := must(m.combat.StartEncounter(ctx, rq(&playv1.StartEncounterRequest{
		CampaignId: w.campaign, IdempotencyKey: newKey(), Name: w.secrets.public("feature-aura-encounter"),
		Participants: []*playv1.Participant{{CharacterId: goblin.GetId(), Hidden: &revealed}},
	}))).GetEncounter()
	for _, c := range e.GetCombatants() {
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_PLAYER {
			must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
				CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(),
				Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 12},
			})))
		}
	}
	for char, sq := range map[string][2]int32{w.toren.GetId(): {3, 3}, pens.GetId(): {4, 3}, goblin.GetId(): {10, 10}} {
		for _, c := range e.GetCombatants() {
			if c.GetCharacterId() == char {
				must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(), Col: sq[0], Row: sq[1]})))
			}
		}
	}
	w.encounter = must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()}))).GetEncounter()

	type readOf struct {
		procedure string
		req       proto.Message
	}
	reads := []readOf{
		{playv1connect.CombatServiceGetEncounterProcedure, &playv1.GetEncounterRequest{CampaignId: w.campaign}},
		{playv1connect.CombatServiceListCombatLogProcedure, &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()}},
		{playv1connect.LastingEffectServiceListCharacterEffectsProcedure, &playv1.ListCharacterEffectsRequest{CampaignId: w.campaign}},
	}
	// Ana, in whose reach the aura stands, reads none of it.
	for _, r := range reads {
		reply := w.ana.call(r.procedure, r.req)
		if !reply.ok() {
			t.Fatalf("Ana's %s: HTTP %d %s", r.procedure, reply.status, reply.body)
		}
		for _, f := range w.inspect(w.ana, reply, nil) {
			t.Errorf("Ana, inside the aura: %s", f)
		}
	}
	// The TV reads what the master's screen holds, as the table reads it.
	screen := m.call(playv1connect.TableScreenServiceGetTableScreenProcedure, &playv1.GetTableScreenRequest{CampaignId: w.campaign})
	if !screen.ok() {
		t.Fatalf("the master's table screen: HTTP %d %s", screen.status, screen.body)
	}
	for _, f := range w.inspect(w.tvReader(), screen, nil) {
		t.Errorf("the TV: %s", f)
	}
	// The positive controls: the bearer's player and the master read the label, so the needle is reachable.
	for who, p := range map[string]*person{"the master": m, "Caio, the bearer's player": w.caio} {
		found := false
		for _, r := range reads {
			if bytes.Contains(p.call(r.procedure, r.req).body, []byte(label)) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not read the label of the aura: the needle is unreachable", who)
		}
	}
}
