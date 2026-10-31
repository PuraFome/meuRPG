package leaktest

import (
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A cloud on a fog map hides what stands in it (RN-10, RN-37). Fog Cloud lands on a creature the players saw a moment
// before: from then on nothing a player receives of the combat names, counts or places it, as for any creature the
// players do not see. The master's own copies are the positive control: the scan finds the needles there. The cast
// is the one the wizard's player makes, and it uses no call of its own, so the matrix (reads_test.go) has nothing to add.
func TestAFogCloudOnACreatureTakesItOutOfWhatThePlayersRead(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m := w.master
	w.imgMap = w.image("cloud-map-image", 240, 160)
	w.fogMap = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: w.secrets.public("cloud-map-name"), ImageId: w.imgMap}))).GetMap().GetId()
	must(m.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	must(m.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: 24})))
	must(m.maps.SetMapFog(ctx, rq(&mapsv1.SetMapFogRequest{CampaignId: w.campaign, MapId: w.fogMap, FogEnabled: new(true), BaseLight: mapsv1.LightLevel_LIGHT_LEVEL_BRIGHT.Enum()})))
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))

	spells := []string{"spell:fog-cloud"}
	wizard := must(w.ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8},
			RaceKey:    "race:gnome", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:wizard", Level: 5}},
			KnownSpellKeys: spells, PreparedSpellKeys: spells,
		}}},
	}))).GetCharacter()
	toren := w.pc(w.caio, "Toren", "race:human")
	foe, foeName := w.npc("cloud-foe", &charactersv1.BasicSheet{HitPointsMax: 40, ArmorClass: 12, SpeedFt: 30})
	w.allow("cloud-foe-name", w.ana, w.caio)

	shown := false
	e := must(m.combat.StartEncounter(ctx, rq(&playv1.StartEncounterRequest{
		CampaignId: w.campaign, IdempotencyKey: newKey(), Name: w.secrets.public("cloud-encounter"),
		Participants: []*playv1.Participant{{CharacterId: foe.GetId(), Hidden: &shown}},
	}))).GetEncounter()
	var foeCombatant, casterCombatant string
	for _, c := range e.GetCombatants() {
		switch c.GetCharacterId() {
		case foe.GetId():
			foeCombatant = c.GetId()
		case wizard.GetId():
			casterCombatant = c.GetId()
		}
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_PLAYER {
			must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
				CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(),
				Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: 12},
			})))
		}
	}
	place := func(characterID string, col, row int32) {
		for _, c := range e.GetCombatants() {
			if c.GetCharacterId() == characterID {
				must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(), Col: col, Row: row})))
			}
		}
	}
	place(wizard.GetId(), 3, 5)
	place(toren.GetId(), 2, 6)
	place(foe.GetId(), 10, 5)
	e = must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()}))).GetEncounter()
	for range 20 { // until the wizard's turn
		cur := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == casterCombatant {
			break
		}
		must(m.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound()})))
	}
	needles := []string{foeName, foe.GetId(), foeCombatant}
	players := []*person{w.ana, w.caio}
	readAll := func(p *person) string {
		var b strings.Builder
		b.WriteString(jsonOf(must(p.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign})))))
		b.WriteString(jsonOf(must(p.combat.ListCombatLog(ctx, rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: e.GetId()})))))
		return b.String()
	}

	// The positive controls: before the cloud both players read the creature, and so does the master.
	for _, p := range players {
		if found := namesAny(readAll(p), needles); len(found) == 0 {
			t.Fatalf("%s does not read the creature before the cloud, so the scan proves nothing", p.name)
		}
	}

	cast := must(w.ana.combat.CastSpell(ctx, rq(&playv1.CastSpellRequest{
		CampaignId: w.campaign, EncounterId: e.GetId(), CasterId: casterCombatant, SpellKey: "spell:fog-cloud", Slot: &playv1.SpellSlot{Level: 1}, IdempotencyKey: newKey(),
		Area: &playv1.CastSpellRequest_Origin{Origin: &playv1.SpellOrigin{Col: 10, Row: 5}},
	})))
	if found := namesAny(jsonOf(cast), needles); len(found) > 0 {
		t.Errorf("the cast's answer to the player names %v: %s", found, jsonOf(cast))
	}
	for _, p := range players {
		if found := namesAny(readAll(p), needles); len(found) > 0 {
			t.Errorf("%s reads %v of a creature in the cloud", p.name, found)
		}
	}
	if !strings.Contains(jsonOf(must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign})))), foeCombatant) {
		t.Error("the master's combat does not hold the creature in the cloud, so the scan proves nothing")
	}
}
