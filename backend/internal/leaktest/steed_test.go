package leaktest

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1/charactersv1connect"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1/rulesv1connect"
)

// What another player reads of a paladin's steed (Find Steed, RN-20): nothing. The steed, its name, the type the player picked
// and the bond are the owner's and the master's; the list of the creatures and the summon options of the character are not
// found for Caio, and neither is the page of the creature itself. The positive controls show them in Ana's and the master's
// answers.
func TestASteedAndItsTypeAndBondAreTheOwnersAndTheMastersOnly(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("building the fixture: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	w.session = must(w.master.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	spell := []string{"spell:find-steed"}
	paladin := must(w.ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Aurélio",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 10, Constitution: 14, Intelligence: 10, Wisdom: 12, Charisma: 16},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:paladin", Level: 5}},
			KnownSpellKeys: spell, PreparedSpellKeys: spell,
		}}},
	}))).GetCharacter()
	name := w.secrets.marker("steed-name", w.ana)
	cast := must(w.ana.play.CastSummon(ctx, rq(&playv1.CastSummonRequest{
		CampaignId: w.campaign, CharacterId: paladin.GetId(), SpellKey: spell[0], Slot: &playv1.SpellSlot{Level: 2}, IdempotencyKey: newKey(),
		Summon: &playv1.SummonChoice{CreatureKeys: []string{"monster:warhorse"}, CreatureType: "fey", Names: []string{name}},
	})))
	steedID := cast.GetCreatureIds()[0]

	reads := []struct {
		what, procedure string
		req             proto.Message
	}{
		{"the creatures of the character", charactersv1connect.CharacterServiceListCharacterCreaturesProcedure, &charactersv1.ListCharacterCreaturesRequest{CampaignId: w.campaign, CharacterId: paladin.GetId()}},
		{"the summon options", charactersv1connect.CharacterServiceGetSummonOptionsProcedure, &charactersv1.GetSummonOptionsRequest{CampaignId: w.campaign, CharacterId: paladin.GetId()}},
		{"the page of the steed", rulesv1connect.ContentServiceGetCreatureProcedure, &rulesv1.GetCreatureRequest{CampaignId: w.campaign, Key: "monster:warhorse", CreatureId: steedID}},
	}
	for _, r := range reads {
		for _, owner := range []*person{w.ana, w.master} {
			got := owner.call(r.procedure, r.req)
			if !got.ok() {
				t.Fatalf("%s: the positive control failed for %s: status %d", r.what, owner.name, got.status)
			}
		}
		other := w.caio.call(r.procedure, r.req)
		if other.ok() {
			t.Errorf("%s: Caio reads it: %s", r.what, other.body)
		}
		for _, f := range w.inspect(w.caio, other, nil) {
			t.Errorf("%s: Caio: %s", r.what, f)
		}
		for _, secret := range []string{"fey", "bonded", name} {
			if bytes.Contains(other.body, []byte(secret)) {
				t.Errorf("%s: Caio's refusal carries %q", r.what, secret)
			}
		}
	}
	// The positive controls: the owner and the master read the name, the type and the bond.
	list := w.master.call(charactersv1connect.CharacterServiceListCharacterCreaturesProcedure, &charactersv1.ListCharacterCreaturesRequest{CampaignId: w.campaign, CharacterId: paladin.GetId()})
	for _, want := range []string{name, `"creatureType":"fey"`, `"bonded":true`} {
		if !bytes.Contains(list.body, []byte(want)) {
			t.Errorf("the master's list lacks %s (the positive control): %s", want, list.body)
		}
	}
	options := w.ana.call(charactersv1connect.CharacterServiceGetSummonOptionsProcedure, &charactersv1.GetSummonOptionsRequest{CampaignId: w.campaign, CharacterId: paladin.GetId()})
	for _, want := range []string{`"typeChoices"`, `"bondedSteed"`, `"creatureType":"fey"`} {
		if !bytes.Contains(options.body, []byte(want)) {
			t.Errorf("Ana's summon options lack %s (the positive control): %s", want, options.body)
		}
	}
}
