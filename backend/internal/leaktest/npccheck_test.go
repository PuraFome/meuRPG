package leaktest

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// loreBard makes a player's College of Lore bard of level 3 with Cutting Words asked on every check.
func loreBard(w *world, p *person, name string, charisma int32) *charactersv1.Character {
	return must(p.characters.CreateCharacter(w.t.Context(), rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name,
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: charisma},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:bard", Level: 3, Subclass: &charactersv1.ClassLevel_SubclassKey{SubclassKey: "subclass:lore"}}},
			CuttingWordsAsk: charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL,
		}}},
	}))).GetCharacter()
}

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
	loreBard(w, ana, "Brisa", 16)
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
	// The check settled (Ana was the only bard asked): the master reads it again, with its numbers; no player does, and the refusal holds
	// none of them.
	if settled := must(m.play.GetNpcCheck(ctx, rq(&playv1.GetNpcCheckRequest{CampaignId: w.campaign}))).GetCheck(); !settled.GetSettled() || settled.GetTotal() != secret.total || settled.GetD20().GetFaces()[0] != 17 {
		t.Errorf("the master's latest check = %v, want the settled check with its numbers (the positive control)", settled)
	}
	for _, p := range []*person{ana, caio} {
		res, err := p.play.GetNpcCheck(ctx, rq(&playv1.GetNpcCheckRequest{CampaignId: w.campaign}))
		if connect.CodeOf(err) != connect.CodePermissionDenied || res != nil {
			t.Errorf("%s reads the latest check: %v, %v, want permission_denied and no answer", p.name, res, err)
		} else if msg := err.Error(); strings.Contains(msg, secret.name) || strings.Contains(msg, "17") || strings.Contains(msg, hold) {
			t.Errorf("the refusal of %s = %q, want no word of the check", p.name, msg)
		}
	}
	// The fence: every hint the check sent has reached the streams before the session ends, so what the walk below reads is all of it.
	fenced := w.fence(t, m, watchers, "fim da conversa")
	asked := 0
	for _, ev := range watchers[ana].since(0)[:fenced[ana]] {
		if eventCase(ev) == "held_rolls_changed" {
			asked++
		}
	}
	if asked == 0 {
		t.Errorf("Ana's stream heard no held rolls change before the fence, want the hint of her question (the positive control)")
	}
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		<-sw.done
		for _, ev := range sw.events {
			switch c := eventCase(ev); c {
			case "ready", "heartbeat", "session_ended", "session_message_added":
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

// The words and the ids that are not a number of the check: a UUID and an RFC 3339 timestamp are strings and numbers of their own kind,
// and both are taken out of a string before its numbers are read.
var (
	uuidText = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	timeText = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[Tt ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[Zz]|[+-]\d{2}:\d{2})?`)
	wordText = regexp.MustCompile(`(?i)\btotal\b`)
)

// leaks lists where the message carries one of the secrets in a field of its own, at any depth and in the values and keys of a map: a
// number of any kind (signed, unsigned, fixed, float or double) equal to the d20, the bonus or the total, an enum value no enum declares
// that is one of them, or a string that holds the creature's name in any case, one of the numbers as a whole token (not as the digits of a
// longer number or of a word) or the word total, once the ids and the timestamps are taken out of it. The counters of the stream and of the
// session are named and skipped, so nothing matches by chance, as a text search of the JSON would.
func (r rollNumbers) leaks(m proto.Message) []string {
	w := &leakWalk{r: r}
	w.message("", m.ProtoReflect())
	return w.found
}

// leakWalk is a walk of a message that collects where the secrets are.
type leakWalk struct {
	r     rollNumbers
	found []string
}

func (w *leakWalk) message(path string, m protoreflect.Message) {
	counters := []protoreflect.Name{"session_seq", "session_number"}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		at := path + "." + string(fd.Name())
		switch {
		case slices.Contains(counters, fd.Name()):
		case fd.IsMap():
			v.Map().Range(func(k protoreflect.MapKey, mv protoreflect.Value) bool {
				w.value(at+"{key}", fd.MapKey(), k.Value())
				w.value(at+"["+k.String()+"]", fd.MapValue(), mv)
				return true
			})
		case fd.IsList():
			for i := range v.List().Len() {
				w.value(fmt.Sprintf("%s[%d]", at, i), fd, v.List().Get(i))
			}
		default:
			w.value(at, fd, v)
		}
		return true
	})
}

// value checks one value of a field by its kind.
func (w *leakWalk) value(path string, fd protoreflect.FieldDescriptor, v protoreflect.Value) {
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		w.message(path, v.Message())
	case protoreflect.StringKind:
		w.text(path, v.String())
	case protoreflect.BytesKind:
		w.text(path, string(v.Bytes()))
	case protoreflect.EnumKind:
		if fd.Enum().Values().ByNumber(v.Enum()) == nil { // a declared value is a name; any other is a number
			w.number(path, float64(v.Enum()))
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		w.number(path, float64(v.Int()))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		w.number(path, float64(v.Uint()))
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		w.number(path, v.Float())
	case protoreflect.BoolKind:
	}
}

func (w *leakWalk) number(path string, n float64) {
	if slices.ContainsFunc(w.r.numbers(), func(s int32) bool { return n == float64(s) }) {
		w.found = append(w.found, fmt.Sprintf("%s = %v is the d20, the bonus or the total of the check", path, n))
	}
}

// numbers are the secret numbers of the check.
func (r rollNumbers) numbers() []int32 { return []int32{r.d20, r.modifier, r.total} }

// text checks a string: the creature's name in any case, the word total and each secret number as a token of its own, once the ids and
// the timestamps are out of it.
func (w *leakWalk) text(path, s string) {
	if w.r.name != "" && strings.Contains(strings.ToLower(s), strings.ToLower(w.r.name)) {
		w.found = append(w.found, fmt.Sprintf("%s = %q carries the creature's name", path, s))
	}
	bare := timeText.ReplaceAllString(uuidText.ReplaceAllString(s, " "), " ")
	if wordText.MatchString(bare) {
		w.found = append(w.found, fmt.Sprintf("%s = %q carries the word total", path, s))
	}
	for _, n := range slices.Compact(slices.Sorted(slices.Values(w.r.numbers()))) {
		if tokenOf(bare, strconv.Itoa(int(max(n, -n)))) {
			w.found = append(w.found, fmt.Sprintf("%s = %q carries %d as a number of its own", path, s, n))
		}
	}
}

// tokenOf says the text holds the number as a token of its own: not next to another digit or a letter ("170", "d20", "x3" are not 17,
// 20 or 3).
func tokenOf(text, number string) bool {
	for i := 0; ; {
		at := strings.Index(text[i:], number)
		if at < 0 {
			return false
		}
		from, to := i+at, i+at+len(number)
		touchesBefore := from > 0 && isWordByte(text[from-1])
		touchesAfter := to < len(text) && isWordByte(text[to])
		if !touchesBefore && !touchesAfter {
			return true
		}
		i = from + 1
	}
}

func isWordByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}

// TestNpcCheckLeakSearchFindsTheSecrets: the search of the streams sees a d20, a bonus, a total and a name in a field of any numeric kind,
// in a number or a name written in a string (any case), in the values and keys of a map and in an enum, and passes the counters, the
// ids, the timestamps and the numbers that only contain the secrets' digits.
func TestNpcCheckLeakSearchFindsTheSecrets(t *testing.T) {
	r := rollNumbers{d20: 17, modifier: 3, total: 20, name: "Lobo"}
	const id = "00170003-0020-4017-8017-317320173173"
	roll := &playv1.NpcCheckRead{HoldId: id, CreatureLabel: "um Lobo", D20: &playv1.DiceRoll{Faces: []int32{17}, Modifier: 3, Total: 20}}
	if got := r.leaks(roll); len(got) != 4 {
		t.Errorf("the leaks of a read with the d20, the bonus, the total and the name = %v, want 4 (the id with the same digits is not one)", got)
	}
	hint := &playv1.WatchGameSessionResponse{SessionSeq: 17, Event: &playv1.WatchGameSessionResponse_Heartbeat_{Heartbeat: &playv1.WatchGameSessionResponse_Heartbeat{SessionSeq: 3}}}
	if got := r.leaks(hint); len(got) != 0 {
		t.Errorf("the leaks of a heartbeat = %v, want none: the counters are not the check", got)
	}
	t.Run("every numeric kind", func(t *testing.T) {
		for kind, v := range map[descriptorpb.FieldDescriptorProto_Type]protoreflect.Value{
			descriptorpb.FieldDescriptorProto_TYPE_INT32:    protoreflect.ValueOfInt32(17),
			descriptorpb.FieldDescriptorProto_TYPE_SINT32:   protoreflect.ValueOfInt32(17),
			descriptorpb.FieldDescriptorProto_TYPE_SFIXED32: protoreflect.ValueOfInt32(17),
			descriptorpb.FieldDescriptorProto_TYPE_INT64:    protoreflect.ValueOfInt64(20),
			descriptorpb.FieldDescriptorProto_TYPE_SINT64:   protoreflect.ValueOfInt64(20),
			descriptorpb.FieldDescriptorProto_TYPE_SFIXED64: protoreflect.ValueOfInt64(20),
			descriptorpb.FieldDescriptorProto_TYPE_UINT32:   protoreflect.ValueOfUint32(3),
			descriptorpb.FieldDescriptorProto_TYPE_FIXED32:  protoreflect.ValueOfUint32(3),
			descriptorpb.FieldDescriptorProto_TYPE_UINT64:   protoreflect.ValueOfUint64(17),
			descriptorpb.FieldDescriptorProto_TYPE_FIXED64:  protoreflect.ValueOfUint64(17),
			descriptorpb.FieldDescriptorProto_TYPE_FLOAT:    protoreflect.ValueOfFloat32(20),
			descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:   protoreflect.ValueOfFloat64(3),
		} {
			if got := r.leaks(oneNumber(t, kind, v)); len(got) != 1 {
				t.Errorf("the leaks of a %v holding %v = %v, want 1", kind, v, got)
			}
		}
		if got := r.leaks(oneNumber(t, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, protoreflect.ValueOfFloat64(17.5))); len(got) != 0 {
			t.Errorf("the leaks of a double of 17.5 = %v, want none", got)
		}
	})
	t.Run("a string", func(t *testing.T) {
		for text, want := range map[string]int{
			"o teste deu 17":                   1, // the d20
			"+3 e 20":                          2, // the bonus and the total
			"Total do teste":                   1, // the word
			"totalmente":                       0,
			"um LOBO":                          1, // the name, in any case
			"um lobo":                          1,
			"170 e 1717 e d20 e x3":            0, // numbers that only contain the digits
			"2026-11-16T17:03:20Z":             0, // a timestamp
			"2026-11-16T17:03:20.320173-03:00": 0,
			"hold 00170003-0020-4017-8017-317320173173":        0, // an id
			"hold 00170003-0020-4017-8017-317320173173 deu 17": 1,
		} {
			if got := r.leaks(&playv1.NpcCheckRead{CheckName: text}); len(got) != want {
				t.Errorf("the leaks of the string %q = %v, want %d", text, got, want)
			}
		}
	})
	t.Run("a map", func(t *testing.T) {
		s, err := structpb.NewStruct(map[string]any{"nota": "o lobo", "faces": 17.0, "sem": "nada", "aninhado": map[string]any{"bonus": 3.0}})
		if err != nil {
			t.Fatalf("NewStruct() error = %v", err)
		}
		if got := r.leaks(s); len(got) != 3 {
			t.Errorf("the leaks of a struct with the name, the d20 and the bonus in its map = %v, want 3", got)
		}
		vitals := &playv1.CharacterVitals{HitDiceUsedByDie: map[int32]int32{6: 20, 17: 0, 8: 2}}
		if got := r.leaks(vitals); len(got) != 2 {
			t.Errorf("the leaks of a map with the total as a value and the d20 as a key = %v, want 2", got)
		}
	})
	t.Run("an enum", func(t *testing.T) {
		if got := r.leaks(&playv1.NpcCheckAsk{State: playv1.NpcCheckAskState(17)}); len(got) != 1 {
			t.Errorf("the leaks of an enum field holding the d20 as a value no enum declares = %v, want 1", got)
		}
		if got := r.leaks(&playv1.NpcCheckAsk{State: playv1.NpcCheckAskState_NPC_CHECK_ASK_STATE_CUT}); len(got) != 0 {
			t.Errorf("the leaks of a declared enum value = %v, want none: its number is a name, not a roll", got)
		}
	})
}

// oneNumber is a message with one field of the numeric kind, set to v.
func oneNumber(t *testing.T, kind descriptorpb.FieldDescriptorProto_Type, v protoreflect.Value) proto.Message {
	t.Helper()
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("kinds.proto"), Package: proto.String("kinds"), Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("One"), Field: []*descriptorpb.FieldDescriptorProto{
			{Name: proto.String("n"), Number: proto.Int32(1), Type: kind.Enum(), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()},
		}}},
	}, nil)
	if err != nil {
		t.Fatalf("NewFile() error = %v", err)
	}
	md := file.Messages().ByName("One")
	m := dynamicpb.NewMessage(md)
	m.Set(md.Fields().ByName("n"), v)
	return m
}

// fence has the master say a line to all and waits until every stream has heard it, once the hint gate sent what it held: what a stream
// heard of a phase is what is before the line in its events. It returns the place of the line in each stream.
func (w *world) fence(t *testing.T, m *person, watchers map[*person]*streamWatcher, text string) map[*person]int {
	t.Helper()
	w.hints.Settle(t)
	must(m.play.SendSessionMessage(t.Context(), rq(&playv1.SendSessionMessageRequest{
		CampaignId: w.campaign, SpeakerKind: playv1.SessionMessageSpeakerKind_SESSION_MESSAGE_SPEAKER_KIND_NARRATOR, Text: text, IdempotencyKey: newKey(),
	})))
	at := map[*person]int{}
	for p, sw := range watchers {
		for deadline := time.After(20 * time.Second); ; {
			if i := markerAt(sw, text); i >= 0 {
				at[p] = i
				break
			}
			select {
			case <-sw.more:
			case <-deadline:
				t.Fatalf("%s's stream never heard the line %q", p.name, text)
			}
		}
	}
	return at
}

// markerAt is the place of the master's narrator line with this text in the events a stream heard, -1 when it did not hear it yet.
func markerAt(sw *streamWatcher, text string) int {
	return slices.IndexFunc(sw.since(0), func(ev *playv1.WatchGameSessionResponse) bool {
		return ev.GetSessionMessageAdded().GetMessage().GetText() == text
	})
}

// TestNpcCheckHintsReachOnlyWhoMayHearThem: two bards are asked in order, a third player is asked nothing, and the first bard cuts. The
// hint of the held rolls goes to the master and to the bard asked and to nobody else, so the bard not asked yet hears nothing until
// their own turn; the hint of the vitals the cut spends goes to the master and to the bard's player alone. A marker line the master
// says to all ends each phase on every stream, so what a stream did not hear is told by the order of its own events.
func TestNpcCheckHintsReachOnlyWhoMayHearThem(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: nil, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m, ana, caio := w.master, w.ana, w.caio
	dani := w.newPerson("Dani")
	w.join(false, dani)
	brisa, cassio := loreBard(w, ana, "Brisa", 16), loreBard(w, caio, "Cássio", 14)
	// The creature is an NPC with Charisma 20, so its check is a d20 of 11 plus 5 for 16: none of the three is a number the bards' own
	// answers and vitals carry (the uses left, 3 and 2 before the cut and 2 and 1 after, the die of 6 and the face of 4 the bard types), so
	// a number of the check in them is the check's.
	baron := must(m.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, Name: "Barão da Serra",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 12, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: 20},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 1}},
		}}},
	}))).GetCharacter()
	must(dani.characters.CreateCharacter(ctx, rq(&charactersv1.CreateCharacterRequest{
		CampaignId: w.campaign, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Dora",
		Sheet: &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
			BaseScores: &rulesv1.AbilityScores{Strength: 16, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 10},
			RaceKey:    "race:human", Classes: []*charactersv1.ClassLevel{{ClassKey: "class:fighter", Level: 2}},
		}}},
	})))
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	w.hints.Settle(t)
	people := []*person{m, ana, caio, dani}
	watchers := map[*person]*streamWatcher{}
	for _, p := range people {
		watchers[p] = w.watchStream(p)
	}
	fence := func(text string) map[*person]int { return w.fence(t, m, watchers, text) }
	// hints lists the held rolls and the vitals hints a stream heard in events [from, to).
	hints := func(p *person, from, to int) (held int, vitals []string) {
		for _, ev := range watchers[p].since(0)[from:to] {
			switch c := eventCase(ev); c {
			case "held_rolls_changed":
				held++
			case "vitals_changed":
				vitals = append(vitals, ev.GetVitalsChanged().GetVitals().GetCharacterId())
			}
		}
		return held, vitals
	}

	res := must(m.play.RollNpcCheck(ctx, rq(&playv1.RollNpcCheckRequest{
		CampaignId: w.campaign, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: baron.GetId()}, CheckKey: "ability:cha",
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 11}, BardSees: true, IdempotencyKey: newKey(),
	})))
	asks := res.GetCheck().GetAsks()
	if d20 := res.GetCheck().GetD20(); !res.GetHeld() || len(asks) != 2 || d20.GetFaces()[0] != 11 || d20.GetModifier() != 5 || d20.GetTotal() != 16 {
		t.Fatalf("the master's roll = %v, want a held check of 11 plus 5 with both bards asked (the positive control)", res)
	}
	byChar := map[string]*person{brisa.GetId(): ana, cassio.GetId(): caio}
	first, second := byChar[asks[0].GetCharacterId()], byChar[asks[1].GetCharacterId()]
	if first == nil || second == nil || first == second {
		t.Fatalf("the asks = %v, want Brisa and Cássio", asks)
	}
	firstChar := asks[0].GetCharacterId()
	hold := res.GetCheck().GetHoldId()
	roll := fence("fim da rolagem")
	for _, p := range people {
		held, vitals := hints(p, 0, roll[p])
		switch p {
		case m, first:
			if held == 0 {
				t.Errorf("%s heard no held rolls change on the roll, want one (the positive control)", p.name)
			}
		default:
			if held != 0 {
				t.Errorf("%s heard %d held rolls changes on the roll, want none: not asked yet", p.name, held)
			}
		}
		if len(vitals) != 0 {
			t.Errorf("%s heard vitals %v on the roll, want none: the roll spends nothing", p.name, vitals)
		}
	}

	// The first bard cuts, the second lets it pass. What the cut answers is exactly the answer, the die of the bard's own and the uses left
	// (Brisa has three uses, Cássio two); what the second bard then reads is exactly their own question.
	uses := map[*person]int32{ana: 3, caio: 2}
	cut := must(first.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: hold, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: true, Roll: &playv1.CuttingWordsAnswer_TypedFace{TypedFace: 4}}},
	})))
	wantCut := &playv1.AnswerHeldRollResponse{Result: &playv1.AnswerHeldRollResponse_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswered{
		Cut: true, Die: &playv1.DiceRoll{DiceCount: 1, DiceSides: 6, Faces: []int32{4}, Total: 4, Physical: true}, UsesLeft: uses[first] - 1,
	}}}
	if !proto.Equal(cut, wantCut) {
		t.Errorf("the cut's answer = %s, want only %s", protojson.Format(cut), protojson.Format(wantCut))
	}
	offered := must(second.resources.ListHeldRolls(ctx, rq(&playv1.ListHeldRollsRequest{CampaignId: w.campaign}))).GetOffers()
	if len(offered) != 1 {
		t.Fatalf("%s lists %v, want their own question", second.name, offered)
	}
	wantOffer := &playv1.HeldRollOffer{
		HoldId: hold, Kind: playv1.HeldRollKind_HELD_ROLL_KIND_NPC_CHECK, CreatedAt: offered[0].GetCreatedAt(), RollerLabel: "uma criatura",
		Question: &playv1.HeldRollOffer_CuttingWords{CuttingWords: &playv1.CuttingWordsQuestion{Sides: 6, UsesLeft: uses[second]}},
	}
	if !proto.Equal(offered[0], wantOffer) {
		t.Errorf("%s's question = %s, want only %s", second.name, protojson.Format(offered[0]), protojson.Format(wantOffer))
	}
	must(second.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: hold, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	})))
	answered := fence("fim das respostas")
	for _, p := range people {
		held, vitals := hints(p, roll[p], answered[p])
		switch p {
		case m, first:
			if len(vitals) != 1 || vitals[0] != firstChar {
				t.Errorf("%s heard vitals of %v, want the cut bard's alone (the positive control)", p.name, vitals)
			}
		default:
			if len(vitals) != 0 {
				t.Errorf("%s heard vitals of %v, want none: the use the cut spent is the master's and the bard's player's", p.name, vitals)
			}
		}
		if p == dani && held != 0 {
			t.Errorf("Dani heard %d held rolls changes, want none: never asked", held)
		}
		if p == second && held == 0 {
			t.Errorf("%s heard no held rolls change at their turn, want one (the positive control)", p.name)
		}
	}

	secret := rollNumbers{d20: 11, modifier: 5, total: 16, name: res.GetCheck().GetCreatureLabel()}
	for what, msg := range map[string]proto.Message{"the cut's answer": cut, "the second bard's question": offered[0]} {
		for _, l := range secret.leaks(msg) {
			t.Errorf("%s: %s", what, l)
		}
	}
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for p, sw := range watchers {
		<-sw.done
		if p == m {
			continue
		}
		for _, ev := range sw.events {
			switch c := eventCase(ev); c {
			case "vitals_changed": // the bard's own use, read above
			case "ready", "heartbeat", "session_ended", "session_message_added":
			case "held_rolls_changed":
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

// A check the master's box says the bards see, with the scene open to Ana only: Caio's bard is outside the scene's audience, so it is not
// asked about the check, hears nothing of it on the stream and reads no offer; Ana's bard is (the positive control).
func TestNpcCheckOutsideTheScenesAudienceAsksNoBard(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	w.buildGallery()
	w.fogMap = must(w.master.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: "Caverna", ImageId: w.imgMap}))).GetMap().GetId()
	must(w.master.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	ps := w.buildPartialScene()
	m, ana, caio := w.master, w.ana, w.caio
	loreBard(w, ana, "Brisa", 16)
	loreBard(w, caio, "Lia", 14)
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	w.openSceneTo(ps.point.GetId(), ana)
	watchers := map[*person]*streamWatcher{ana: w.watchStream(ana), caio: w.watchStream(caio)}
	res := must(m.play.RollNpcCheck(ctx, rq(&playv1.RollNpcCheckRequest{
		CampaignId: w.campaign, Npc: &playv1.RollNpcCheckRequest_MonsterKey{MonsterKey: "monster:wolf"}, CheckKey: "skill:perception",
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 17}, BardSees: true, IdempotencyKey: newKey(),
	})))
	if !res.GetHeld() || len(res.GetCheck().GetAsks()) != 1 || res.GetCheck().GetAsks()[0].GetCharacterName() != "Brisa" {
		t.Fatalf("the master's roll = %v, want one bard asked: Ana's", res)
	}
	read := res.GetCheck()
	secret := rollNumbers{d20: 17, modifier: read.GetD20().GetModifier(), total: read.GetD20().GetTotal(), name: read.GetCreatureLabel()}
	offers := func(p *person) []*playv1.HeldRollOffer {
		return must(p.resources.ListHeldRolls(ctx, rq(&playv1.ListHeldRollsRequest{CampaignId: w.campaign}))).GetOffers()
	}
	if got := offers(ana); len(got) != 1 || got[0].GetRollerLabel() != "uma criatura" {
		t.Errorf("Ana lists %v, want the bard's question (the positive control)", got)
	}
	if got := offers(caio); len(got) != 0 {
		t.Errorf("Caio lists %v, want nothing", got)
	}
	fenced := w.fence(t, m, watchers, "fim da conversa")
	heard := func(p *person) int {
		n := 0
		for _, ev := range watchers[p].since(0)[:fenced[p]] {
			if eventCase(ev) == "held_rolls_changed" {
				n++
			}
		}
		return n
	}
	if heard(ana) == 0 {
		t.Error("Ana's stream heard no held rolls change, want the hint of her question (the positive control)")
	}
	if got := heard(caio); got != 0 {
		t.Errorf("Caio's stream heard %d held rolls changes, want none", got)
	}
	for p, sw := range watchers {
		for _, ev := range sw.since(0) {
			for _, l := range secret.leaks(ev) {
				t.Errorf("%s's stream: %s", p.name, l)
			}
		}
	}
}

// A check of a creature both bards stand within 60 feet of on a measured map, in sight, with the scene open to Ana only: the map measures
// both bards, yet Caio's bard is outside the scene's audience, so it is not asked, reads no offer and hears nothing on the stream; Ana's
// bard is (the positive control).
func TestNpcCheckOnAMeasuredMapOutsideTheScenesAudienceAsksNoBard(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: map[string]*mapsv1.MapPoint{}, puzzles: map[string]*playv1.Puzzle{}, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	w.buildGallery()
	w.fogMap = must(w.master.maps.CreateMap(ctx, rq(&mapsv1.CreateMapRequest{CampaignId: w.campaign, Name: "Caverna", ImageId: w.imgMap}))).GetMap().GetId()
	must(w.master.maps.SetMapRevealed(ctx, rq(&mapsv1.SetMapRevealedRequest{CampaignId: w.campaign, MapId: w.fogMap, Revealed: true})))
	must(w.master.maps.SetMapGrid(ctx, rq(&mapsv1.SetMapGridRequest{CampaignId: w.campaign, MapId: w.fogMap, Columns: caveColumns})))
	ps := w.buildPartialScene()
	m, ana, caio := w.master, w.ana, w.caio
	brisa, lia := loreBard(w, ana, "Brisa", 16), loreBard(w, caio, "Lia", 14)
	baron, _ := w.npc("measured-npc", &charactersv1.BasicSheet{HitPointsMax: 9, ArmorClass: 11, SpeedFt: 30})
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	must(m.play.SetCurrentMap(ctx, rq(&playv1.SetCurrentMapRequest{CampaignId: w.campaign, MapId: w.fogMap})))
	w.place(w.fogMap, brisa.GetId(), 2, 2)
	w.place(w.fogMap, lia.GetId(), 2, 4)
	w.place(w.fogMap, baron.GetId(), 6, 3)
	must(m.maps.SetMapTokenHidden(ctx, rq(&mapsv1.SetMapTokenHiddenRequest{CampaignId: w.campaign, MapId: w.fogMap, CharacterId: baron.GetId(), Hidden: false})))
	w.openSceneTo(ps.point.GetId(), ana)
	watchers := map[*person]*streamWatcher{ana: w.watchStream(ana), caio: w.watchStream(caio)}
	res := must(m.play.RollNpcCheck(ctx, rq(&playv1.RollNpcCheckRequest{
		CampaignId: w.campaign, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: baron.GetId()}, CheckKey: "ability:cha",
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 17}, IdempotencyKey: newKey(),
	})))
	if !res.GetHeld() || len(res.GetCheck().GetAsks()) != 1 || res.GetCheck().GetAsks()[0].GetCharacterId() != brisa.GetId() {
		t.Fatalf("the master's roll = %v, want one bard asked: Ana's, though the map measures both", res)
	}
	read := res.GetCheck()
	secret := rollNumbers{d20: 17, modifier: read.GetD20().GetModifier(), total: read.GetD20().GetTotal(), name: read.GetCreatureLabel()}
	offers := func(p *person) []*playv1.HeldRollOffer {
		return must(p.resources.ListHeldRolls(ctx, rq(&playv1.ListHeldRollsRequest{CampaignId: w.campaign}))).GetOffers()
	}
	if got := offers(ana); len(got) != 1 {
		t.Errorf("Ana lists %v, want the bard's question (the positive control)", got)
	}
	if got := offers(caio); len(got) != 0 {
		t.Errorf("Caio lists %v, want nothing", got)
	}
	fenced := w.fence(t, m, watchers, "fim da conversa")
	heard := func(p *person) int {
		n := 0
		for _, ev := range watchers[p].since(0)[:fenced[p]] {
			if eventCase(ev) == "held_rolls_changed" {
				n++
			}
		}
		return n
	}
	if heard(ana) == 0 {
		t.Error("Ana's stream heard no held rolls change, want the hint of her question (the positive control)")
	}
	if got := heard(caio); got != 0 {
		t.Errorf("Caio's stream heard %d held rolls changes, want none", got)
	}
	for p, sw := range watchers {
		for _, ev := range sw.since(0) {
			for _, l := range secret.leaks(ev) {
				t.Errorf("%s's stream: %s", p.name, l)
			}
		}
	}
}

// A player reads the summary of a session that ended after an NPC's check a bard cut: it holds nothing of the check (the creature, its
// roll, who cut), the bard included: a check of the master's side is nobody's roll in the summary.
func TestNpcCheckLeavesNothingInThePlayersSummaryOfTheEndedSession(t *testing.T) {
	w := &world{stack: newStack(t), secrets: newSecrets(), pts: nil, notes: map[string]string{}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("the table: %v", r)
		}
	}()
	ctx := t.Context()
	w.buildPeople()
	m, ana, caio := w.master, w.ana, w.caio
	loreBard(w, ana, "Brisa", 16)
	w.pc(caio, "Toren", "race:human")
	w.session = must(m.play.StartGameSession(ctx, rq(&playv1.StartGameSessionRequest{CampaignId: w.campaign}))).GetGameSession().GetId()
	res := must(m.play.RollNpcCheck(ctx, rq(&playv1.RollNpcCheckRequest{
		CampaignId: w.campaign, Npc: &playv1.RollNpcCheckRequest_MonsterKey{MonsterKey: "monster:wolf"}, CheckKey: "skill:perception",
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 17}, BardSees: true, IdempotencyKey: newKey(),
	})))
	read := res.GetCheck()
	if !res.GetHeld() || read.GetCreatureLabel() == "" {
		t.Fatalf("the master's roll = %v, want a held check of a named creature", res)
	}
	must(ana.resources.AnswerHeldRoll(ctx, rq(&playv1.AnswerHeldRollRequest{
		CampaignId: w.campaign, HoldId: read.GetHoldId(), IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: true, Roll: &playv1.CuttingWordsAnswer_TypedFace{TypedFace: 4}}},
	})))
	if got := must(m.play.GetNpcCheck(ctx, rq(&playv1.GetNpcCheckRequest{CampaignId: w.campaign}))).GetCheck(); !got.GetSettled() || got.GetCut() != 4 {
		t.Fatalf("the master's latest check = %v, want it settled with the 4 cut (the positive control)", got)
	}
	must(m.play.EndGameSession(ctx, rq(&playv1.EndGameSessionRequest{CampaignId: w.campaign, GameSessionId: w.session})))
	for _, p := range []*person{ana, caio} {
		s := must(p.play.GetSessionSummary(ctx, rq(&playv1.GetSessionSummaryRequest{CampaignId: w.campaign, GameSessionId: w.session}))).GetSummary()
		if s.GetMine() != nil || len(s.GetCategories()) != 0 || len(s.GetPlayers()) != 0 || s.GetCombats() != 0 || s.GetScenesOpened() != 0 ||
			s.GetChecksTried() != 0 || s.GetChecksPassed() != 0 || len(s.GetConversation()) != 0 {
			t.Errorf("%s's summary = %s, want only the times of the session", p.name, protojson.Format(s))
		}
		text := strings.ToLower(protojson.Format(s))
		for _, word := range []string{strings.ToLower(read.GetCreatureLabel()), "wolf", "npc_check", "perception", "cutting", read.GetHoldId()} {
			if strings.Contains(text, word) {
				t.Errorf("%s's summary holds %q: %s", p.name, word, text)
			}
		}
	}
}
