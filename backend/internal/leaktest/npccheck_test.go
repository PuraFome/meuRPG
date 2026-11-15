package leaktest

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// An NPC's check that a Lore bard may cut (RN-10, RN-20): the master's roll is the master's alone. The bard asked reads the question
// of the bard's own turn, with the creature as "uma criatura" when the bard does not know its name, and no d20, bonus or total; the other players
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
	must(ana.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Brisa",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: 16},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:bard", Level: 3, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: "subclass:lore"}}},
			CuttingWordsAsk: charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL,
		}}},
	})))
	must(caio.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Toren",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}},
		}}},
	})))
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	watchers := map[*person]*streamWatcher{ana: w.watchStream(ana), caio: w.watchStream(caio)}

	// Players cannot roll, skip or list the picker.
	roll := func(key string) *playv1.RollNpcCheckRequest {
		return &playv1.RollNpcCheckRequest{
			CampaignId: w.campaign, Npc: &playv1.RollNpcCheckRequest_MonsterKey{MonsterKey: "monster:wolf"}, CheckKey: "skill:perception",
			Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 17}, BardSees: true, IdempotencyKey: key,
		}
	}
	for _, p := range []*person{ana, caio} {
		_, err := p.play.RollNpcCheck(ctx, rq(roll(newKey())))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s rolls an NPC's check: %v, want permission_denied", p.name, err)
		}
		_, err = p.play.ListNpcCheckPicker(ctx, rq(&playv1.ListNpcCheckPickerRequest{CampaignId: w.campaign}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s lists the picker: %v, want permission_denied", p.name, err)
		}
	}
	res := must(m.play.RollNpcCheck(ctx, rq(roll(newKey()))))
	read := res.GetCheck()
	hold, name := read.GetHoldId(), read.GetCreatureLabel()
	d20 := read.GetD20()
	if !res.GetHeld() || hold == "" || name == "" || len(read.GetAsks()) != 1 || d20.GetFaces()[0] != 17 || d20.GetModifier() == 0 || d20.GetTotal() != 17+d20.GetModifier() {
		t.Fatalf("the master's roll = %v, want a held check of 17 plus a bonus, with the bard asked (the positive control)", res)
	}
	for _, p := range []*person{ana, caio} {
		_, err := p.play.SkipNpcCheckAsks(ctx, rq(&playv1.SkipNpcCheckAsksRequest{CampaignId: w.campaign, HoldId: hold, IdempotencyKey: newKey()}))
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("%s skips the asks: %v, want permission_denied", p.name, err)
		}
	}
	secret := rollNumbers{d20: 17, modifier: d20.GetModifier(), total: d20.GetTotal(), name: name}

	offers := func(p *person) []*playv1.HeldRollOffer {
		return must(p.resources.ListHeldRolls(ctx, rq(&playv1.ListHeldRollsRequest{CampaignId: w.campaign}))).GetOffers()
	}
	if got := offers(m); len(got) != 1 || got[0].GetD20().GetFaces()[0] != 17 || got[0].GetRollerLabel() != name {
		t.Errorf("the master lists %v, want the roll with its d20 (the positive control)", got)
	}
	if got := offers(caio); len(got) != 0 {
		t.Errorf("Caio lists %v, want nothing", got)
	}
	got := offers(ana)
	if len(got) != 1 {
		t.Fatalf("Ana lists %v, want the bard's question", got)
	}
	// Ana's question is exactly the hold, the kind, the time, "uma criatura" and the size of the bard's die with the uses left: no d20,
	// no check, no asks, no creature.
	wantOffer := &playv1.HeldRollOffer{
		HoldId: hold, Kind: playv1.HeldRollKind_HELD_ROLL_KIND_NPC_CHECK, CreatedAt: got[0].GetCreatedAt(), RollerLabel: "uma criatura",
		Question: &playv1.HeldRollOffer_CuttingWords{CuttingWords: &playv1.CuttingWordsQuestion{Sides: 6, UsesLeft: 3}},
	}
	if !proto.Equal(got[0], wantOffer) {
		t.Errorf("Ana's question = %s, want only %s", protojson.Format(got[0]), protojson.Format(wantOffer))
	}
	_, err := caio.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: hold, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	}))
	if connect.CodeOf(err) != connect.CodeNotFound || strings.Contains(err.Error(), hold) {
		t.Errorf("Caio's answer = %v, want not_found without the id", err)
	}
	answered := must(ana.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: hold, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	})))
	if want := (&playv1.AnswerHeldRollResponse{Result: &playv1.AnswerHeldRollResponse_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswered{UsesLeft: 3}}}); !proto.Equal(answered, want) {
		t.Errorf("Ana's answer = %s, want only %s", protojson.Format(answered), protojson.Format(want))
	}
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		<-sw.done
		for _, ev := range sw.events {
			switch c := eventCase(ev); c {
			case "ready", "heartbeat", "session_ended":
			case "held_rolls_changed":
				if p == caio {
					t.Errorf("Caio's stream heard a hint of the held rolls, want none: none was Caio's")
				}
				if proto.Size(ev.GetHeldRollsChanged()) != 0 {
					t.Errorf("%s's hint of the held rolls = %v, want an empty hint", p.name, ev.GetHeldRollsChanged())
				}
			default:
				t.Errorf("%s's stream carries a %q event, want none of the check", p.name, c)
			}
			for _, l := range secret.leaks(ev) {
				t.Errorf("%s's stream: %s", p.name, l)
			}
		}
	}
}

// rollNumbers are the master's secrets of a check: its d20, bonus and total, and the creature's name.
type rollNumbers struct {
	d20, modifier, total int32
	name                 string
}

// leaks lists where the message carries one of the secrets in a field of its own: a whole number equal to the d20, the bonus or the
// total, or a string with the creature's name. The ids and the timestamps are strings and numbers of their own kind, and the counters of
// the stream and of the session are named, so nothing matches by chance, as a text search of the JSON would.
func (r rollNumbers) leaks(m proto.Message) []string {
	var found []string
	counters := []protoreflect.Name{"session_seq", "session_number"}
	var walk func(path string, m protoreflect.Message)
	check := func(path string, fd protoreflect.FieldDescriptor, v protoreflect.Value) {
		if slices.Contains(counters, fd.Name()) {
			return
		}
		switch fd.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			walk(path, v.Message())
		case protoreflect.StringKind:
			if strings.Contains(v.String(), r.name) {
				found = append(found, fmt.Sprintf("%s = %q carries the creature's name", path, v.String()))
			}
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			if n := v.Int(); n == int64(r.d20) || n == int64(r.modifier) || n == int64(r.total) {
				found = append(found, fmt.Sprintf("%s = %d is the d20, the bonus or the total of the check", path, n))
			}
		}
	}
	walk = func(path string, m protoreflect.Message) {
		m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			switch {
			case fd.IsList():
				for i := range v.List().Len() {
					check(path+"."+string(fd.Name()), fd, v.List().Get(i))
				}
			case fd.IsMap():
			default:
				check(path+"."+string(fd.Name()), fd, v)
			}
			return true
		})
	}
	walk("", m.ProtoReflect())
	return found
}

// TestNpcCheckLeakSearchFindsTheSecrets: the search of the streams sees a d20, a bonus, a total and a name in a field, and passes the
// counters, the ids and the timestamps of an event that carries none of them.
func TestNpcCheckLeakSearchFindsTheSecrets(t *testing.T) {
	r := rollNumbers{d20: 17, modifier: 3, total: 20, name: "Lobo"}
	roll := &playv1.NpcCheckRead{HoldId: "1717-3-20", CreatureLabel: "um Lobo", D20: &playv1.DiceRoll{Faces: []int32{17}, Modifier: 3, Total: 20}}
	if got := r.leaks(roll); len(got) != 4 {
		t.Errorf("the leaks of a read with the d20, the bonus, the total and the name = %v, want 4 (the id with the same digits is not one)", got)
	}
	hint := &playv1.WatchGameSessionResponse{SessionSeq: 17, Event: &playv1.WatchGameSessionResponse_Heartbeat_{Heartbeat: &playv1.WatchGameSessionResponse_Heartbeat{SessionSeq: 3}}}
	if got := r.leaks(hint); len(got) != 0 {
		t.Errorf("the leaks of a heartbeat = %v, want none: the counters are not the check", got)
	}
}
