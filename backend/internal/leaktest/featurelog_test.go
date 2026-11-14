package leaktest

import (
	"bytes"
	"strings"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The line of a use of a feature (#743 part 2): what the use cost of the turn, the resource it spent and what is left of it are
// read by the master and the user's player. Toren (Caio's) uses a bonus action feature that spends one of the uses of a
// resource, with a goblin everybody sees on the table. Ana, who plays another character, reads the plain line: the feature,
// and none of the cost or the resource's name. The master and Caio are the positive control.

const chamadoKey = "chamados_leak"

// chamadoFight builds the combat of Toren (a feature "Chamado" of the bonus action that spends the resource), Pensantus and a goblin
// in view, with Toren on turn. It returns the marker that names the resource.
func (w *world) chamadoFight() (resourceName string) {
	w.t.Helper()
	ctx := w.t.Context()
	m := w.master
	resourceName = w.secrets.marker("feature-resource-name", w.caio)
	w.imgMap = w.image("chamado-map-image", 240, 160)
	w.fogMap = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: w.secrets.public("chamado-map-name"), ImageId: w.imgMap}))).GetMap().GetId()
	must(m.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	must(m.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: 24})))
	w.map2 = must(m.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: w.secrets.marker("chamado-map2-name"), ImageId: w.image("chamado-map2-image", 60, 40)}))).GetMap().GetId()
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))

	sub := must(m.table.CreateTableEntry(ctx, rq(&rulesv1.CreateTableEntryRequest{
		CampaignId: w.campaign, Body: &rulesv1.CreateTableEntryRequest_TableSubclass{TableSubclass: &rulesv1.TableSubclass{
			NamePt: "Guardião do Chamado", ClassKey: "class:fighter", DescPt: []string{"Um guardião."},
			Levels: []*rulesv1.TableSubclassLevel{{Level: 3, Features: []*rulesv1.TableFeature{
				{NamePt: "Chamado Curto", DescPt: []string{"Um grito."}, Effects: []*rulesv1.TableEffect{{
					Type: "grant_action", Economy: "bonus_action", Resource: chamadoKey, TextPt: "Um grito.",
					Use: &rulesv1.FeatureUse{Applies: &rulesv1.FeatureApplies{TempHp: "1"}, Targets: &rulesv1.FeatureTargets{Who: rulesv1.FeatureTargetWho_FEATURE_TARGET_WHO_SELF}},
				}}},
				{NamePt: resourceName, DescPt: []string{"Uma reserva."}, Effects: []*rulesv1.TableEffect{{Type: "resource", Resource: chamadoKey, Max: "2", Recharge: "short_rest"}}},
			}}},
		}},
	}))).GetEntry()
	must(m.characters.UpdateCharacter(ctx, rq(&charactersv1.UpdateCharacterRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), Revision: w.toren.GetRevision(), Name: w.toren.GetName(),
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 5, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: sub.GetKey()}}},
			WeaponKeys: []string{"equipment:longsword"},
		}}},
	})))
	goblin := must(m.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_MINION, Name: w.secrets.public("chamado-goblin-name"),
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{HitPointsMax: 20, ArmorClass: 12, SpeedFt: 30}}},
	}))).GetCharacter()
	shown := false
	e := must(m.combat.StartEncounter(ctx, rq(&playv1.StartEncounterRequest{
		CampaignId: w.campaign, IdempotencyKey: newKey(), Name: w.secrets.public("chamado-encounter"),
		Participants: []*playv1.Participant{{CharacterId: goblin.GetId(), Hidden: &shown}},
	}))).GetEncounter()
	for _, c := range e.GetCombatants() {
		face := int32(1)
		if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_PLAYER && c.GetCharacterId() == w.toren.GetId() {
			face = 20
		} else if c.GetKind() == playv1.CombatantKind_COMBATANT_KIND_PLAYER {
			face = 10
		}
		must(m.combat.SubmitInitiative(ctx, rq(&playv1.SubmitInitiativeRequest{
			CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: c.GetId(), IdempotencyKey: newKey(),
			Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: face},
		})))
	}
	w.placeForced(m, e.GetId(), placement{w.combatantOf(e, w.toren.GetId()).GetId(), 3, 3}, placement{w.combatantOf(e, w.pens.GetId()).GetId(), 3, 6}, placement{w.combatantOf(e, goblin.GetId()).GetId(), 9, 3})
	w.encounter = must(m.combat.BeginCombat(ctx, rq(&playv1.BeginCombatRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey()}))).GetEncounter()
	return resourceName
}

// featureKeyOf is the key of the feature action the combatant has under the name, as the master's turn options read it.
func (w *world) featureKeyOf(c *playv1.Combatant, name string) string {
	w.t.Helper()
	opts := must(w.master.combat.GetTurnOptions(w.t.Context(), rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: c.GetId()})))
	for _, a := range opts.GetOptions().GetFeatureActions() {
		if a.GetAction().GetNamePt() == name {
			return a.GetAction().GetKey()
		}
	}
	w.t.Fatalf("%s has no feature action %q", c.GetLabel(), name)
	return ""
}

// Toren's use of Chamado Curto: Caio and the master read the economy and the resource with what is left of it, Ana reads the plain
// line, and nothing she reads of the combat or its log holds the resource's name or the words of the cost.
func TestAFeatureUsesCostIsReadByTheMasterAndTheUserAlone(t *testing.T) {
	w := haloWorld(t)
	resource := w.chamadoFight()
	w.secrets.add(&canary{needle: "FEATURE_ECONOMY_", kind: "feature-use-economy", readers: names([]*person{w.caio})})
	w.secrets.add(&canary{needle: "resourceNamePt", kind: "feature-use-resource-field", readers: names([]*person{w.caio})})

	toren := w.combatant(w.toren)
	if cur := w.encounter.GetCurrentCombatantId(); cur != toren.GetId() {
		t.Fatalf("the turn is %s's, want Toren's", cur)
	}
	must(w.caio.feature.UseFeature(t.Context(), rq(&playv1.UseFeatureRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: toren.GetId(), IdempotencyKey: newKey(),
		ActionKey: w.featureKeyOf(toren, "Chamado Curto"),
	})))

	// The positive control: the master's and Caio's logs hold the needles, so they are reachable.
	for _, p := range []*person{w.master, w.caio} {
		body := p.call(playv1connect.CombatServiceListCombatLogProcedure, &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()}).body
		for _, needle := range []string{resource, "FEATURE_ECONOMY_BONUS_ACTION", "resourceNamePt"} {
			if !bytes.Contains(body, []byte(needle)) {
				t.Errorf("%s's log does not hold %q: the needle is unreachable (%s)", p.name, needle, body)
			}
		}
	}
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.reads(p) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	plain := w.ana.call(playv1connect.CombatServiceListCombatLogProcedure, &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()}).body
	if !strings.Contains(string(plain), "Chamado Curto") {
		t.Errorf("Ana's log does not hold the feature's name: the plain line is missing (%s)", plain)
	}
}
