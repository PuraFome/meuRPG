package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The pure parts of the held-roll gate: how a row of either table becomes a heldRoll, which questions
// a roll asks and in what order, when a roll is stale, and the order of the checks an answer meets.
// The paths that use them are pinned by the tests of the held rolls, which need a database.

// pinnedAttackRequest is the request a held attack keeps, as the app sent it with the two dice rolled.
func pinnedAttackRequest(t *testing.T, faces []int32, typed bool) []byte {
	t.Helper()
	req := &playv1.RollAttackRequest{
		CampaignId: "campaign-1", IdempotencyKey: "key-1", RollMode: playv1.RollMode_ROLL_MODE_ADVANTAGE, D20Faces: faces,
	}
	if !typed {
		req.Roll = &playv1.RollAttackRequest_RollInApp{RollInApp: true}
	}
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal the request: %v", err)
	}
	return body
}

// A roll_holds row converts on read: the faces come from the pinned request, the die that counts is the
// pinned mode's pick, the total adds the bonus to that die, and the "#window" leaves the key for Window.
func TestHeldRollOfARollHoldsRow(t *testing.T) {
	t.Parallel()
	row := playdb.RollHold{
		ID: "hold-1", EncounterID: "enc-1", CombatantID: "cmb-1", IdempotencyKey: "key-1" + windowHoldSuffix,
		Request: pinnedAttackRequest(t, []int32{4, 13}, false), Face: 13, Modifier: 5, Round: 2,
	}
	h := heldOfRollHold(row)
	if h.table != rollHolds || h.Kind != heldWeaponAttack || h.CampaignID != "campaign-1" || h.EncounterID != "enc-1" || h.CombatantID != "cmb-1" {
		t.Errorf("heldOfRollHold() = %+v, want a weapon attack of the combatant in the campaign", h)
	}
	if h.Key != "key-1" || !h.Window {
		t.Errorf("key = %q, window = %v, want the key without the suffix and the window set", h.Key, h.Window)
	}
	if len(h.Faces) != 2 || h.Faces[0] != 4 || h.Faces[1] != 13 || h.Counted != 1 || h.Modifier != 5 || h.Total != 18 || h.Physical {
		t.Errorf("faces = %v counted %d modifier %d total %d physical %v, want 4 and 13, the second counting, 5 and 18, rolled in the app",
			h.Faces, h.Counted, h.Modifier, h.Total, h.Physical)
	}
	if h.Round == nil || *h.Round != 2 || h.UserID != "" || h.CharacterID != "" {
		t.Errorf("round = %v, user = %q, character = %q, want round 2 and neither owner (the row has none)", h.Round, h.UserID, h.CharacterID)
	}
	typed := row
	typed.Request, typed.IdempotencyKey = pinnedAttackRequest(t, []int32{4, 13}, true), "key-2"
	if got := heldOfRollHold(typed); !got.Physical || got.Window {
		t.Errorf("a typed pair, no suffix: physical = %v, window = %v, want physical and no window", got.Physical, got.Window)
	}
	single := playdb.RollHold{Face: 9, Modifier: 3, Request: pinnedAttackRequest(t, nil, false)}
	if got := heldOfRollHold(single); len(got.Faces) != 1 || got.Faces[0] != 9 || got.Counted != 0 || got.Total != 12 {
		t.Errorf("a single d20: faces = %v counted %d total %d, want the face, 0 and 12", got.Faces, got.Counted, got.Total)
	}
}

// The same rule of "stale" reads the two tables as they have always been read: a roll_holds row only by
// its round; an inspiration_holds row also when its die is gone and no Lucky question needs one.
func TestAHeldRollIsStaleByItsTable(t *testing.T) {
	t.Parallel()
	round := int32(3)
	st := func(round int32, holds bool) rollerState {
		return rollerState{Die: heldDie{Combatant: &playdb.Combatant{}, Round: round}, Holds: holds}
	}
	attack := heldRoll{table: rollHolds, Round: &round}
	spell := heldRoll{table: inspirationHolds, Round: &round}
	cases := []struct {
		name string
		h    heldRoll
		st   rollerState
		want bool
	}{
		{"an attack in its round with its die", attack, st(3, true), false},
		{"an attack in its round whose die ran out", attack, st(3, false), false},
		{"an attack of an earlier round", attack, st(4, true), true},
		{"a spell attack in its round with its die", spell, st(3, true), false},
		{"a spell attack in its round whose die ran out", spell, st(3, false), true},
		{"a spell attack of an earlier round", spell, st(4, true), true},
		{"a roll out of a combat", heldRoll{table: inspirationHolds}, rollerState{}, false},
		{"an attack whose combat is over", attack, rollerState{}, true},
	}
	for _, tc := range cases {
		if got := tc.h.stale(tc.st); got != tc.want {
			t.Errorf("%s: stale = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The chain asks Lucky first and the die after it; a roll that waits for neither still gets the die's
// question, which the path decides what to say about.
func TestTheChainAsksLuckyBeforeTheDie(t *testing.T) {
	t.Parallel()
	holds := rollerState{Holds: true}
	asking := heldRoll{LuckyIndex: new(int32(0))}
	answered := heldRoll{LuckyIndex: new(int32(0)), LuckyAnswerKey: new("k")}
	closed := heldRoll{LuckyIndex: new(int32(0)), LuckyAnswerKey: new("k"), AnswerKey: new("k")}
	cases := []struct {
		name string
		h    heldRoll
		st   rollerState
		want questionKind
	}{
		{"a 1 and a die", asking, holds, askLucky},
		{"a 1 and no die", asking, rollerState{}, askLucky},
		{"Lucky answered, the die waits", answered, holds, askDie},
		{"no Lucky question, the die waits", heldRoll{}, holds, askDie},
		{"the roll is answered", closed, holds, askDie},
		{"nothing waits", heldRoll{}, rollerState{}, askDie},
	}
	for _, tc := range cases {
		if got := askedBy(tc.h, tc.st).Asks; got != tc.want {
			t.Errorf("%s: asks %d, want %d", tc.name, got, tc.want)
		}
	}
	if (dieQuestion{}).pending(closed, holds) || !(dieQuestion{}).pending(answered, holds) || (dieQuestion{}).pending(answered, rollerState{}) {
		t.Error("dieQuestion.pending: want it only for a roller that holds a die and a roll nobody answered")
	}
}

// The checks an answer meets, in order: an answer to the Lucky question of a roll that asked none, an
// answer already given by another key, the same answer again (never rolled twice), and the die's answer
// before Lucky's.
func TestTheChecksOfAnAnswerKeepTheirOrder(t *testing.T) {
	t.Parallel()
	lucky := heldRoll{ID: "h", LuckyIndex: new(int32(0))}
	luckyDone := heldRoll{ID: "h", LuckyIndex: new(int32(0)), LuckyAnswerKey: new("a"), LuckyTo: new(int32(15))}
	closed := heldRoll{ID: "h", AnswerKey: new("z")}
	closedByLucky := heldRoll{ID: "h", LuckyIndex: new(int32(0)), LuckyAnswerKey: new("a"), AnswerKey: new("a")}
	cases := []struct {
		name     string
		h        heldRoll
		req      answerRequest
		state    answerState
		wantCode connect.Code
		wantKey  string
	}{
		{"Lucky answered for a roll that asked none", heldRoll{ID: "h"}, answerRequest{key: "a", question: askLucky}, answerOpen, connect.CodeFailedPrecondition, ""},
		{"Lucky, the first answer", lucky, answerRequest{key: "a", question: askLucky}, answerOpen, 0, "a"},
		{"Lucky again with the same key", luckyDone, answerRequest{key: "a", question: askLucky}, answerRepeated, 0, "a"},
		{"Lucky again with another key", luckyDone, answerRequest{key: "b", question: askLucky}, answerOpen, connect.CodeFailedPrecondition, ""},
		{"Lucky after the roll was settled with its key", closedByLucky, answerRequest{key: "a", question: askLucky}, answerSettled, 0, "a"},
		{"Lucky after the roll was settled by another", closed, answerRequest{key: "a", question: askLucky}, answerOpen, connect.CodeFailedPrecondition, ""},
		{"the die, the first answer", heldRoll{ID: "h"}, answerRequest{key: "a", question: askDie}, answerOpen, 0, "a"},
		{"the die before Lucky", lucky, answerRequest{key: "a", question: askDie}, answerOpen, connect.CodeFailedPrecondition, ""},
		{"the die again with the same key", closed, answerRequest{key: "z", question: askDie}, answerSettled, 0, "z"},
		{"the die with another key", closed, answerRequest{key: "a", question: askDie}, answerOpen, connect.CodeFailedPrecondition, ""},
	}
	for _, tc := range cases {
		a, state, err := tc.h.replyTo(tc.req)
		if tc.wantCode != 0 {
			if connect.CodeOf(err) != tc.wantCode {
				t.Errorf("%s: error = %v, want %v", tc.name, err, tc.wantCode)
			}
			continue
		}
		if err != nil || state != tc.state || a.key != tc.wantKey || a.holdID != "h" {
			t.Errorf("%s: reply = %+v, state = %d, error = %v; want key %q and state %d", tc.name, a, state, err, tc.wantKey, tc.state)
		}
	}
	// The same Lucky answer again hands the path the die the first one rolled.
	if a, _, _ := luckyDone.replyTo(answerRequest{key: "a", question: askLucky}); a.reroll == nil || a.reroll.To != 15 || a.reroll.From != 1 {
		t.Errorf("the same Lucky answer again: reroll = %+v, want the 1 rolled again to 15", a.reroll)
	}
	// Lucky first, by the reason the player reads.
	_, _, err := lucky.replyTo(answerRequest{key: "a", question: askDie})
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Message() != "answer the Lucky question of the roll first" {
		t.Errorf("the die before Lucky: error = %v, want the message of LUCKY_PENDING", err)
	}
}

// An answer to a roll another answer settled is refused; one to a roll of an earlier round is too, in the
// table that checks its answers only.
func TestAnAnswerIsRefusedAfterAnotherOrAfterTheRound(t *testing.T) {
	t.Parallel()
	round := int32(2)
	late := rollerState{Die: heldDie{Combatant: &playdb.Combatant{}, Round: 3}, Holds: true}
	cases := []struct {
		name string
		h    heldRoll
		st   rollerState
		want bool
	}{
		{"open", heldRoll{table: inspirationHolds}, rollerState{}, false},
		{"answered by the same key", heldRoll{table: inspirationHolds, AnswerKey: new("a")}, rollerState{}, false},
		{"answered by another key", heldRoll{table: inspirationHolds, AnswerKey: new("b")}, rollerState{}, true},
		{"a spell attack of an earlier round", heldRoll{table: inspirationHolds, Round: &round}, late, true},
		{"a weapon attack of an earlier round", heldRoll{table: rollHolds, Round: &round}, late, false},
	}
	for _, tc := range cases {
		err := tc.h.answerable(tc.st, "a")
		if (err != nil) != tc.want || (err != nil && connect.CodeOf(err) != connect.CodeFailedPrecondition) {
			t.Errorf("%s: error = %v, want refused = %v with failed_precondition", tc.name, err, tc.want)
		}
	}
}

// How the new die of an answer comes: the Lucky one is a d20, the Bardic Inspiration one is checked against the
// die later, and every refusal says what the request has to set.
func TestRollOfAnswer(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	face, zero, big := int32(7), int32(0), int32(21)
	cases := []struct {
		name    string
		pick    rollPick
		q       questionKind
		wantIn  rollInput
		wantMsg string
	}{
		{"in the app", rollPick{app: &yes}, askLucky, rollInput{inApp: true}, ""},
		{"in the app, false", rollPick{app: &no}, askDie, rollInput{}, "roll_in_app must be true"},
		{"a typed d20", rollPick{face: &face}, askLucky, rollInput{typed: 7}, ""},
		{"a typed d20 of 0", rollPick{face: &zero}, askLucky, rollInput{}, "typed_face must be 1 to 20"},
		{"a typed d20 of 21", rollPick{face: &big}, askLucky, rollInput{}, "typed_face must be 1 to 20"},
		{"a typed face of the die, checked later", rollPick{face: &big}, askDie, rollInput{typed: 21}, ""},
		{"nothing, for Lucky", rollPick{}, askLucky, rollInput{}, "set roll_in_app or typed_face to roll again"},
		{"nothing, for the die", rollPick{}, askDie, rollInput{}, "set roll_in_app or typed_face to use the die"},
	}
	for _, tc := range cases {
		in, err := rollOfAnswer(tc.pick, tc.q)
		if tc.wantMsg != "" {
			var ce *connect.Error
			if !errors.As(err, &ce) || ce.Code() != connect.CodeInvalidArgument || ce.Message() != tc.wantMsg {
				t.Errorf("%s: error = %v, want invalid_argument %q", tc.name, err, tc.wantMsg)
			}
			continue
		}
		if err != nil || in.inApp != tc.wantIn.inApp || in.typed != tc.wantIn.typed {
			t.Errorf("%s: rollInput = %+v, error = %v, want %+v", tc.name, in, err, tc.wantIn)
		}
	}
}

// A retry of the request that made a roll wait is the request with the same key; a replay of a reaction window
// held the roll under a key of its own, so the original key does not find it.
func TestARollIsFromTheSameRequestOnlyWithoutAWindow(t *testing.T) {
	t.Parallel()
	if !(heldRoll{Key: "k"}).fromSameRequest("k") || (heldRoll{Key: "k"}).fromSameRequest("other") || (heldRoll{Key: "k", Window: true}).fromSameRequest("k") {
		t.Error("fromSameRequest: want the key to match only for a roll no window replayed")
	}
}

// The d20 of a question shows both dice of a roll with advantage, the one that counts marked, and the die Lucky
// rolled again, whichever table the roll came from.
func TestTheD20OfAQuestionMarksTheDieThatCounts(t *testing.T) {
	t.Parallel()
	h := heldRoll{Faces: []int{15, 13}, Counted: 0, Modifier: 5, Total: 20, LuckyIndex: new(int32(0)), LuckyTo: new(int32(15)), Physical: true}
	d20 := heldD20Of(h)
	if d20.GetDiceCount() != 2 || len(d20.GetFaces()) != 2 || d20.GetCountedIndex() != 0 || d20.GetModifier() != 5 || d20.GetTotal() != 20 || !d20.GetPhysical() {
		t.Errorf("heldD20Of(pair) = %v, want both dice, the first counting, 5 and 20, physical", d20)
	}
	if r := d20.GetRerolled(); len(r) != 1 || r[0].GetFrom() != 1 || r[0].GetTo() != 15 {
		t.Errorf("rerolled = %v, want the 1 rolled again to 15", r)
	}
	one := heldD20Of(heldRoll{Faces: []int{9}, Modifier: 2, Total: 11})
	if one.GetDiceCount() != 1 || len(one.GetFaces()) != 1 || one.GetTotal() != 11 || len(one.GetRerolled()) != 0 {
		t.Errorf("heldD20Of(single) = %v, want one die and 11, nothing rolled again", one)
	}
}
