package play

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
)

// The free dice roller (MR-052): dice the app rolls on request, outside every game rule. This file
// is the roll itself, with no database: what may be asked, how it is rolled, how it reads in the
// log and who may read it. free_roll_rpc.go is the service around it.

// eventFreeRoll is the session event of a free roll (session_event_kinds).
const eventFreeRoll = "free_roll_rolled"

// What a free roll may ask: the board's dice and its steppers.
const (
	freeRollMaxCount    = 20
	freeRollMaxModifier = 20
	freeRollD20         = 20
	// freeRollPair is how many d20 advantage and disadvantage roll.
	freeRollPair = 2
	// freeRollListLimit and freeRollMineLimit are how many rolls ListFreeRolls returns for the log
	// and for "Suas últimas rolagens".
	freeRollListLimit = 50
	freeRollMineLimit = 10
)

// freeRollSides are the dice of the board's chips: d4 to d100.
var freeRollSides = []int32{4, 6, 8, 10, 12, 20, 100}

// How a single d20 was rolled, as the event stores it. The empty string is a normal roll.
const (
	freeRollAdvantage    = "advantage"
	freeRollDisadvantage = "disadvantage"
)

// freeRollDie is one die of a roll as the event stores it.
type freeRollDie struct {
	Face int32 `json:"face"`
	// Discarded is the d20 that did not count under advantage or disadvantage.
	Discarded bool `json:"discarded"`
}

// freeRollEvent is the payload of a free_roll_rolled event: numbers only, the roller is the event's
// actor. Secret is always written, so that a reader that filters on it (the list's SQL does) never
// meets an event without it.
type freeRollEvent struct {
	Sides    int32         `json:"sides"`
	Count    int32         `json:"count"`
	Modifier int32         `json:"modifier"`
	Mode     string        `json:"mode"`
	Dice     []freeRollDie `json:"dice"`
	Total    int32         `json:"total"`
	// Secret is the master's "Só eu": the roll is the master's alone (RN-10).
	Secret   bool `json:"secret"`
	ByMaster bool `json:"by_master"`
}

// freeRollAsk is a request that passed the checks.
type freeRollAsk struct {
	sides, count, modifier int32
	mode                   string
}

// parseFreeRoll checks what a roll asks for: one of the board's dice, 1 to 20 of them, a modifier
// from -20 to +20, and advantage or disadvantage only on a single d20.
func parseFreeRoll(sides, count, modifier int32, mode playv1.FreeRollMode) (freeRollAsk, error) {
	switch {
	case !slices.Contains(freeRollSides, sides):
		return freeRollAsk{}, invalidFreeRoll("sides must be 4, 6, 8, 10, 12, 20 or 100")
	case count < 1 || count > freeRollMaxCount:
		return freeRollAsk{}, invalidFreeRoll("count must be 1 to 20")
	case modifier < -freeRollMaxModifier || modifier > freeRollMaxModifier:
		return freeRollAsk{}, invalidFreeRoll("modifier must be -20 to +20")
	}
	ask := freeRollAsk{sides: sides, count: count, modifier: modifier}
	switch mode {
	case playv1.FreeRollMode_FREE_ROLL_MODE_UNSPECIFIED:
		return ask, nil
	case playv1.FreeRollMode_FREE_ROLL_MODE_ADVANTAGE:
		ask.mode = freeRollAdvantage
	case playv1.FreeRollMode_FREE_ROLL_MODE_DISADVANTAGE:
		ask.mode = freeRollDisadvantage
	default:
		return freeRollAsk{}, invalidFreeRoll("mode is not one of the known ones")
	}
	if sides != freeRollD20 || count != 1 {
		return freeRollAsk{}, invalidFreeRoll("advantage and disadvantage are for a single d20 only")
	}
	return ask, nil
}

func invalidFreeRoll(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}

// rollFree rolls what was asked with the server's dice. Under advantage or disadvantage two d20 are
// rolled, in order, and the higher (or the lower) counts: on a tie the first does, so the second
// reads "descartado".
func rollFree(r dice.Roller, ask freeRollAsk) (freeRollEvent, error) {
	asked := int(ask.count)
	if ask.mode != "" {
		asked = freeRollPair
	}
	res, err := dice.Roll(r, dice.Expr{Count: asked, Sides: int(ask.sides)})
	if err != nil {
		return freeRollEvent{}, fmt.Errorf("roll the free dice: %w", err)
	}
	ev := freeRollEvent{Sides: ask.sides, Count: ask.count, Modifier: ask.modifier, Mode: ask.mode}
	for _, f := range res.Faces {
		ev.Dice = append(ev.Dice, freeRollDie{Face: conv.Int32(f)})
	}
	if ask.mode != "" {
		discard := 1
		if first, second := ev.Dice[0].Face, ev.Dice[1].Face; (ask.mode == freeRollAdvantage && second > first) || (ask.mode == freeRollDisadvantage && second < first) {
			discard = 0
		}
		ev.Dice[discard].Discarded = true
	}
	ev.Total = ask.modifier
	for _, d := range ev.Dice {
		if !d.Discarded {
			ev.Total += d.Face
		}
	}
	return ev, nil
}

// formula is the roll as the app writes it: "2d6 + 3", "1d20 - 2", "1d20 + 5 com vantagem".
func (ev freeRollEvent) formula() string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(int(ev.Count)) + "d" + strconv.Itoa(int(ev.Sides)))
	switch {
	case ev.Modifier > 0:
		b.WriteString(" + " + strconv.Itoa(int(ev.Modifier)))
	case ev.Modifier < 0:
		b.WriteString(" - " + strconv.Itoa(int(-ev.Modifier)))
	}
	switch ev.Mode {
	case freeRollAdvantage:
		b.WriteString(" com vantagem")
	case freeRollDisadvantage:
		b.WriteString(" com desvantagem")
	}
	return b.String()
}

// logText is the line of the session log: "Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)". A die
// that did not count says so: "(14, 9 descartado)".
func (ev freeRollEvent) logText(name string) string {
	faces := make([]string, 0, len(ev.Dice))
	for _, d := range ev.Dice {
		face := strconv.Itoa(int(d.Face))
		if d.Discarded {
			face += " descartado"
		}
		faces = append(faces, face)
	}
	return fmt.Sprintf("Rolagem livre · %s: %s = %d (%s)", name, ev.formula(), ev.Total, strings.Join(faces, ", "))
}

// audience is who the stream tells of the roll: the whole table for a table roll, and for a secret
// roll nobody (false), not even the master's other screens: the master has the answer in hand
// (RN-10). The zero Audience reaches nobody, so a secret roll cannot leak by a forgotten field.
func (ev freeRollEvent) audience() (live.Audience, bool) {
	if ev.Secret {
		return live.Audience{}, false
	}
	return live.Audience{Everyone: true}, true
}

// visibleTo says whether v may read the roll: the master reads every roll, a player the table's.
func (ev freeRollEvent) visibleTo(v viewer) bool { return v.master || !ev.Secret }

// freeRollRollerName is the name a roll shows when its roller's account has none any more.
func freeRollRollerName(name string, byMaster bool) string {
	switch {
	case name != "":
		return name
	case byMaster:
		return "Mestre"
	}
	return "Jogador"
}

// freeRollToProto shapes a roll for a reader that may read it.
func freeRollToProto(id, name string, at time.Time, ev freeRollEvent) *playv1.FreeRoll {
	mode := playv1.FreeRollMode_FREE_ROLL_MODE_UNSPECIFIED
	switch ev.Mode {
	case freeRollAdvantage:
		mode = playv1.FreeRollMode_FREE_ROLL_MODE_ADVANTAGE
	case freeRollDisadvantage:
		mode = playv1.FreeRollMode_FREE_ROLL_MODE_DISADVANTAGE
	}
	out := &playv1.FreeRoll{
		Id: id, RollerName: name, ByMaster: ev.ByMaster, Secret: ev.Secret,
		Sides: ev.Sides, Count: ev.Count, Modifier: ev.Modifier, Mode: mode, Total: ev.Total,
		Formula: ev.formula(), LogText: ev.logText(name), CreatedAt: timestamppb.New(at),
	}
	for _, d := range ev.Dice {
		out.Dice = append(out.Dice, &playv1.FreeRollDie{Face: d.Face, Discarded: d.Discarded})
	}
	return out
}

// decodeFreeRoll reads the payload of a stored free roll.
func decodeFreeRoll(payload []byte, into *freeRollEvent) error {
	if err := json.Unmarshal(payload, into); err != nil {
		return fmt.Errorf("read a free roll event: %w", err)
	}
	return nil
}
