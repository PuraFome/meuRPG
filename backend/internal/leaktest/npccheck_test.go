package leaktest

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// An NPC's check that a Lore bard may cut (RN-10, RN-20): the master's roll is the master's alone. The bard asked reads the question
// of her own turn, with the creature as "uma criatura" when she does not know its name, and no d20, bonus or total; the other players
// read nothing of it, in the list of the questions, in the answer to a try at answering it or in the stream; a player cannot roll,
// skip or list the picker.
func TestNpcCheckReachesOnlyTheBardAsked(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: nil, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m, ana, caio := w.master, w.ana, w.caio
	bard := must(ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Brisa",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: 16},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:bard", Level: 3, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: "subclass:lore"}}},
			CuttingWordsAsk: charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL,
		}}},
	}))).GetCharacter()
	must(caio.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Toren",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}},
		}}},
	})))
	npc, name := w.npc("npc-check", &charactersv1.BasicSheet{HitPointsMax: 9, ArmorClass: 11, SpeedFt: 30})
	w.secrets.id("npc-check-id", npc.GetId())
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	watchers := map[*person]*streamWatcher{ana: w.watchStream(ana), caio: w.watchStream(caio)}

	// Players cannot roll, skip or list the picker.
	for _, p := range []*person{ana, caio} {
		_, err := p.play.RollNpcCheck(ctx, rq(&playv1.RollNpcCheckRequest{
			CampaignId: w.campaign, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: npc.GetId()}, CheckKey: "skill:deception",
			Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 17}, BardSees: true, IdempotencyKey: newKey(),
		}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s rolls an NPC's check: %v, want permission_denied", p.name, err)
		}
	}
	res := must(m.play.RollNpcCheck(ctx, rq(&playv1.RollNpcCheckRequest{
		CampaignId: w.campaign, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: npc.GetId()}, CheckKey: "skill:deception",
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 17}, BardSees: true, IdempotencyKey: newKey(),
	})))
	hold := res.GetCheck().GetHoldId()
	if !res.GetHeld() || hold == "" || len(res.GetCheck().GetAsks()) != 1 || res.GetCheck().GetD20().GetFaces()[0] != 17 {
		t.Fatalf("the master's roll = %v, want a held check of 17 with the bard asked (the positive control)", res)
	}
	_ = bard

	offers := func(p *person) []*playv1.HeldRollOffer {
		return must(p.resources.ListHeldRolls(ctx, rq(&playv1.ListHeldRollsRequest{CampaignId: w.campaign}))).GetOffers()
	}
	if got := offers(m); len(got) != 1 || got[0].GetD20() == nil {
		t.Errorf("the master lists %v, want the roll with its d20 (the positive control)", got)
	}
	if got := offers(caio); len(got) != 0 {
		t.Errorf("Caio lists %v, want nothing", got)
	}
	got := offers(ana)
	if len(got) != 1 {
		t.Fatalf("Ana lists %v, want her question", got)
	}
	body := protojson.Format(got[0])
	if got[0].GetD20() != nil || got[0].GetRollerLabel() != "uma criatura" || strings.Contains(body, name) || strings.Contains(body, "17") {
		t.Errorf("Ana's question = %s, want no d20, no total and the creature as \"uma criatura\"", body)
	}
	_, err := caio.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: hold, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	}))
	if connect.CodeOf(err) != connect.CodeNotFound || strings.Contains(err.Error(), hold) {
		t.Errorf("Caio's answer = %v, want not_found without the id", err)
	}
	must(ana.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: hold, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	})))
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		<-sw.done
		for _, ev := range sw.events {
			b, err := protojson.Marshal(ev)
			if err != nil {
				t.Fatalf("marshal an event: %v", err)
			}
			if p == caio && eventCase(ev) == "held_rolls_changed" {
				t.Errorf("Caio's stream heard a hint of the held rolls, want none: none was his")
			}
			if strings.Contains(string(b), name) {
				t.Errorf("%s's stream carries the NPC's name: %s", p.name, shorten(b))
			}
		}
	}
}
