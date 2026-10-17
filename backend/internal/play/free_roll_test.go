package play

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
)

// The free dice roller (MR-052): the rules of a roll that need no database. The service's own tests
// (free_roll_rpc_test.go) check who may roll, who is told and the key.

const (
	modeNormal       = playv1.FreeRollMode_FREE_ROLL_MODE_UNSPECIFIED
	modeAdvantage    = playv1.FreeRollMode_FREE_ROLL_MODE_ADVANTAGE
	modeDisadvantage = playv1.FreeRollMode_FREE_ROLL_MODE_DISADVANTAGE
)

func TestParseFreeRollAcceptsTheBoardsDiceAndLimits(t *testing.T) {
	t.Parallel()
	for _, sides := range []int32{4, 6, 8, 10, 12, 20, 100} {
		if _, err := parseFreeRoll(sides, 1, 0, modeNormal); err != nil {
			t.Errorf("a d%d: %v, want it accepted", sides, err)
		}
	}
	for _, c := range []struct {
		name            string
		sides, cnt, mod int32
		mode            playv1.FreeRollMode
	}{
		{"20 dice", 6, 20, 0, modeNormal},
		{"the highest modifier", 6, 1, 20, modeNormal},
		{"the lowest modifier", 6, 1, -20, modeNormal},
		{"advantage on one d20", 20, 1, 5, modeAdvantage},
		{"disadvantage on one d20", 20, 1, -2, modeDisadvantage},
	} {
		if _, err := parseFreeRoll(c.sides, c.cnt, c.mod, c.mode); err != nil {
			t.Errorf("%s: %v, want it accepted", c.name, err)
		}
	}
}

func TestParseFreeRollRefusesWhatTheBoardDoesNotOffer(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name            string
		sides, cnt, mod int32
		mode            playv1.FreeRollMode
	}{
		{"a d3", 3, 1, 0, modeNormal},
		{"a d7", 7, 1, 0, modeNormal},
		{"no sides", 0, 1, 0, modeNormal},
		{"a negative die", -6, 1, 0, modeNormal},
		{"a d101", 101, 1, 0, modeNormal},
		{"no dice", 6, 0, 0, modeNormal},
		{"21 dice", 6, 21, 0, modeNormal},
		{"negative count", 6, -1, 0, modeNormal},
		{"modifier +21", 6, 1, 21, modeNormal},
		{"modifier -21", 6, 1, -21, modeNormal},
		{"advantage on two d20", 20, 2, 0, modeAdvantage},
		{"disadvantage on two d20", 20, 2, 0, modeDisadvantage},
		{"advantage on a d6", 6, 1, 0, modeAdvantage},
		{"disadvantage on a d100", 100, 1, 0, modeDisadvantage},
		{"a mode nobody defined", 20, 1, 0, playv1.FreeRollMode(9)},
	} {
		_, err := parseFreeRoll(c.sides, c.cnt, c.mod, c.mode)
		var ce *connect.Error
		if !errors.As(err, &ce) || ce.Code() != connect.CodeInvalidArgument {
			t.Errorf("%s: error = %v, want invalid_argument", c.name, err)
		}
	}
}

func rollOf(t *testing.T, faces []int, sides, count, modifier int32, mode playv1.FreeRollMode) freeRollEvent {
	t.Helper()
	ask, err := parseFreeRoll(sides, count, modifier, mode)
	if err != nil {
		t.Fatalf("parseFreeRoll() error = %v", err)
	}
	ev, err := rollFree(&dice.Fixed{Faces: faces}, ask)
	if err != nil {
		t.Fatalf("rollFree() error = %v", err)
	}
	return ev
}

func facesOf(ev freeRollEvent) []int32 {
	var out []int32
	for _, d := range ev.Dice {
		out = append(out, d.Face)
	}
	return out
}

func TestRollFreeAddsTheModifierAndKeepsTheOrderTheDiceCameUp(t *testing.T) {
	t.Parallel()
	ev := rollOf(t, []int{4, 4}, 6, 2, 3, modeNormal)
	if ev.Total != 11 || !slices.Equal(facesOf(ev), []int32{4, 4}) || ev.Sides != 6 || ev.Count != 2 || ev.Modifier != 3 {
		t.Errorf("2d6+3 with 4, 4 = %+v, want total 11", ev)
	}
	ev = rollOf(t, []int{1, 6, 3}, 6, 3, 0, modeNormal)
	if ev.Total != 10 || !slices.Equal(facesOf(ev), []int32{1, 6, 3}) {
		t.Errorf("3d6 with 1, 6, 3 = %+v, want total 10 in that order", ev)
	}
	for _, d := range ev.Dice {
		if d.Discarded {
			t.Errorf("a normal roll discarded a die: %+v", ev)
		}
	}
}

func TestRollFreeTotalCanBeBelowOne(t *testing.T) {
	t.Parallel()
	if ev := rollOf(t, []int{1}, 4, 1, -20, modeNormal); ev.Total != -19 {
		t.Errorf("1d4-20 with 1 = %d, want -19: the app shows what the dice say", ev.Total)
	}
}

func TestRollFreeTwentyDiceRollTwentyFaces(t *testing.T) {
	t.Parallel()
	faces := make([]int, 20)
	for i := range faces {
		faces[i] = i%6 + 1
	}
	ev := rollOf(t, faces, 6, 20, 0, modeNormal)
	want := 0
	for _, f := range faces {
		want += f
	}
	if len(ev.Dice) != 20 || int(ev.Total) != want {
		t.Errorf("20d6: %d dice, total %d, want 20 dice and %d", len(ev.Dice), ev.Total, want)
	}
}

func TestRollFreeAdvantageCountsTheHigherAndMarksTheOtherDiscarded(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		faces     []int
		mode      playv1.FreeRollMode
		total     int32
		discarded []bool
	}{
		{"advantage, first higher", []int{14, 9}, modeAdvantage, 19, []bool{false, true}},
		{"advantage, second higher", []int{9, 14}, modeAdvantage, 19, []bool{true, false}},
		{"advantage, a tie keeps the first", []int{7, 7}, modeAdvantage, 12, []bool{false, true}},
		{"disadvantage, first lower", []int{3, 18}, modeDisadvantage, 8, []bool{false, true}},
		{"disadvantage, second lower", []int{18, 3}, modeDisadvantage, 8, []bool{true, false}},
		{"disadvantage, a tie keeps the first", []int{7, 7}, modeDisadvantage, 12, []bool{false, true}},
	}
	for _, c := range cases {
		ev := rollOf(t, c.faces, 20, 1, 5, c.mode)
		var got []bool
		for _, d := range ev.Dice {
			got = append(got, d.Discarded)
		}
		if ev.Total != c.total || !slices.Equal(got, c.discarded) || !slices.Equal(facesOf(ev), toInt32s(c.faces)) {
			t.Errorf("%s: %+v, want total %d, discarded %v, faces %v as rolled", c.name, ev, c.total, c.discarded, c.faces)
		}
		if ev.Count != 1 {
			t.Errorf("%s: count = %d, want 1 (the roll asked for one d20)", c.name, ev.Count)
		}
	}
}

func toInt32s(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}

func TestRollFreeNeverRollsMoreDiceThanItNeeds(t *testing.T) {
	t.Parallel()
	src := &dice.Fixed{Faces: []int{5, 9, 2}}
	ask, _ := parseFreeRoll(20, 1, 0, modeNormal)
	if _, err := rollFree(src, ask); err != nil {
		t.Fatal(err)
	}
	if next, _ := src.Roll(20); next != 9 {
		t.Errorf("after one d20 the source gives %d next, want 9: a normal roll takes one face", next)
	}
	adv := &dice.Fixed{Faces: []int{5, 9, 2}}
	ask, _ = parseFreeRoll(20, 1, 0, modeAdvantage)
	if _, err := rollFree(adv, ask); err != nil {
		t.Fatal(err)
	}
	if next, _ := adv.Roll(20); next != 2 {
		t.Errorf("after advantage the source gives %d next, want 2: it takes two faces", next)
	}
}

func TestRollFreeFailsWithTheSourceNotWithAGuess(t *testing.T) {
	t.Parallel()
	ask, _ := parseFreeRoll(6, 3, 0, modeNormal)
	if _, err := rollFree(&dice.Fixed{Faces: []int{1, 2}}, ask); err == nil {
		t.Error("rollFree with a source that runs dry: no error, want one")
	}
}

func TestFreeRollFormulaAndLogLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		ev      freeRollEvent
		formula string
		line    string
	}{
		{"the board's player line", rollOf(t, []int{4, 4}, 6, 2, 3, modeNormal), "2d6 + 3", "Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)"},
		{"a negative modifier", rollOf(t, []int{12}, 20, 1, -2, modeNormal), "1d20 - 2", "Rolagem livre · Brisa: 1d20 - 2 = 10 (12)"},
		{"no modifier", rollOf(t, []int{14}, 20, 1, 0, modeNormal), "1d20", "Rolagem livre · Brisa: 1d20 = 14 (14)"},
		{"advantage", rollOf(t, []int{14, 9}, 20, 1, 5, modeAdvantage), "1d20 + 5 com vantagem", "Rolagem livre · Brisa: 1d20 + 5 com vantagem = 19 (14, 9 descartado)"},
		{"disadvantage", rollOf(t, []int{9, 14}, 20, 1, 0, modeDisadvantage), "1d20 com desvantagem", "Rolagem livre · Brisa: 1d20 com desvantagem = 9 (9, 14 descartado)"},
		{"the discarded die first", rollOf(t, []int{9, 14}, 20, 1, 0, modeAdvantage), "1d20 com vantagem", "Rolagem livre · Brisa: 1d20 com vantagem = 14 (9 descartado, 14)"},
		{"a total below zero", rollOf(t, []int{1}, 4, 1, -20, modeNormal), "1d4 - 20", "Rolagem livre · Brisa: 1d4 - 20 = -19 (1)"},
		{"a d100", rollOf(t, []int{73}, 100, 1, 0, modeNormal), "1d100", "Rolagem livre · Brisa: 1d100 = 73 (73)"},
	}
	for _, c := range cases {
		if got := c.ev.formula(); got != c.formula {
			t.Errorf("%s: formula = %q, want %q", c.name, got, c.formula)
		}
		if got := c.ev.logText("Brisa"); got != c.line {
			t.Errorf("%s: log line = %q, want %q", c.name, got, c.line)
		}
	}
}

func TestFreeRollAudience(t *testing.T) {
	t.Parallel()
	table := rollOf(t, []int{4, 4}, 6, 2, 3, modeNormal)
	secret := table
	secret.Secret, secret.ByMaster = true, true

	if aud, ok := table.audience(); !ok || !aud.Everyone {
		t.Errorf("a table roll: audience %+v, %v, want everyone", aud, ok)
	}
	if aud, ok := secret.audience(); ok || aud.Everyone || aud.Players || aud.UserID != "" {
		t.Errorf("a secret roll: audience %+v, %v, want the stream told nothing at all (RN-10)", aud, ok)
	}

	master, player := viewer{master: true, userID: "m"}, viewer{userID: "p"}
	for _, c := range []struct {
		name string
		ev   freeRollEvent
		v    viewer
		want bool
	}{
		{"a player reads a table roll", table, player, true},
		{"the master reads a table roll", table, master, true},
		{"the master reads their secret roll", secret, master, true},
		{"a player never reads a secret roll", secret, player, false},
	} {
		if got := c.ev.visibleTo(c.v); got != c.want {
			t.Errorf("%s: visibleTo = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFreeRollPayloadRoundTripsThroughJSON(t *testing.T) {
	t.Parallel()
	ev := rollOf(t, []int{14, 9}, 20, 1, 5, modeAdvantage)
	ev.Secret, ev.ByMaster = true, true
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var back freeRollEvent
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(facesOf(back), facesOf(ev)) || back.Total != ev.Total || back.Mode != ev.Mode || !back.Secret || !back.ByMaster {
		t.Errorf("round trip = %+v, want %+v", back, ev)
	}
	// secret is always written, so that a reader that filters on it never meets an event without it
	var keys map[string]any
	_ = json.Unmarshal(body, &keys)
	plain, _ := json.Marshal(rollOf(t, []int{3}, 6, 1, 0, modeNormal))
	var plainKeys map[string]any
	_ = json.Unmarshal(plain, &plainKeys)
	if _, ok := plainKeys["secret"]; !ok {
		t.Errorf("a table roll's payload has no secret key: %s", plain)
	}
}

func TestFreeRollToProtoCarriesTheNameAndTheDice(t *testing.T) {
	t.Parallel()
	ev := rollOf(t, []int{14, 9}, 20, 1, 5, modeAdvantage)
	at := time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC)
	got := freeRollToProto("0b6c7f0e-0000-4000-8000-000000000001", "Brisa", at, ev)
	if got.GetRollerName() != "Brisa" || got.GetTotal() != 19 || got.GetMode() != modeAdvantage || got.GetSides() != 20 || got.GetCount() != 1 ||
		got.GetFormula() != "1d20 + 5 com vantagem" || got.GetLogText() != ev.logText("Brisa") || got.GetSecret() || got.GetByMaster() {
		t.Errorf("proto = %v", got)
	}
	if len(got.GetDice()) != 2 || got.GetDice()[0].GetFace() != 14 || got.GetDice()[0].GetDiscarded() || got.GetDice()[1].GetFace() != 9 || !got.GetDice()[1].GetDiscarded() {
		t.Errorf("dice = %v, want 14 counted and 9 discarded", got.GetDice())
	}
	if !got.GetCreatedAt().AsTime().Equal(at) {
		t.Errorf("created_at = %v, want %v", got.GetCreatedAt().AsTime(), at)
	}
}

func TestFreeRollNamesTheRollerWhoseAccountIsGone(t *testing.T) {
	t.Parallel()
	if got := freeRollRollerName("", true); got != "Mestre" {
		t.Errorf("a master with no name = %q, want Mestre", got)
	}
	if got := freeRollRollerName("", false); got != "Jogador" {
		t.Errorf("a player with no name = %q, want Jogador", got)
	}
	if got := freeRollRollerName("Brisa", false); got != "Brisa" {
		t.Errorf("a named player = %q, want Brisa", got)
	}
}
