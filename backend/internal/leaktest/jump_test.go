package leaktest

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// A long jump over, or onto, a trap the character does not know says nothing of it: the
// preview, the move's answer, the combat and the log name a trap only once it is revealed
// to that character (RN-10).

const (
	jumpOver = 11 // Toren stands at (10, 13); the unrevealed trap-caio is at (11, 13)
	jumpLand = 12
	jumpRow  = 13
	// jumpReachDft is how far (in tenths of a foot) Toren's jump goes in this test, with the
	// running start he is given: more than the two squares of the jump.
	jumpReachDft = 300
)

// jumpTurn gives Toren the 10 ft he walked just before and a long jump, and brings the combat
// to his turn (Ana rolls the death save her character owes on hers).
func jumpTurn(w *world) {
	w.t.Helper()
	toren := w.combatant(w.toren).GetId()
	if _, err := w.pool.Exec(w.t.Context(), `UPDATE combatants SET jump_long_dft = $2, last_move_dft = 100 WHERE id = $1`, toren, jumpReachDft); err != nil {
		w.t.Fatalf("give Toren a jump: %v", err)
	}
	ctx := w.t.Context()
	for range 60 {
		cur := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == toren {
			return
		}
		if w.combatantOf(cur, w.pens.GetId()).GetDeathSaveDue() {
			must(w.ana.combat.RollDeathSave(ctx, rq(&playv1.RollDeathSaveRequest{
				CampaignId: w.campaign, EncounterId: cur.GetId(), CombatantId: w.combatantOf(cur, w.pens.GetId()).GetId(), IdempotencyKey: newKey(),
				Roll: &playv1.RollDeathSaveRequest_D20Face{D20Face: 15},
			})))
		}
		must(w.master.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{
			CampaignId: w.campaign, EncounterId: cur.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound(),
		})))
	}
	w.t.Fatal("Toren's turn never came")
}

// aboutTraps keeps the findings of a read that concern a trap: after the jump Toren stands
// somewhere new, and what he may now see of the rest is the fog's business (its own tests).
func (w *world) aboutTraps(p *person, r reply) []string {
	var out []string
	for _, f := range w.inspect(p, r, nil) {
		if strings.Contains(f, "trap") {
			out = append(out, f)
		}
	}
	return out
}

func (w *world) jumpPreview() reply {
	return w.caio.call(playv1connect.CombatServiceGetMoveOptionsProcedure, &playv1.GetMoveOptionsRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: w.combatant(w.toren).GetId(), Jump: playv1.JumpKind_JUMP_KIND_LONG,
	})
}

// TestAJumpOverAnUnknownTrapNamesNothing: Caio previews and makes a long jump from one side
// of a trap he does not know to the other. The squares of the preview carry no trap, the
// move's answer, the combat and its log hold nothing of it, and the players' log says
// that Toren jumped and no more. The positive control: once the master reveals the trap to
// Toren, the preview warns about it by its name, which Caio may read.
func TestAJumpOverAnUnknownTrapNamesNothing(t *testing.T) {
	w := newWorld(t)
	jumpTurn(w)
	trap := w.pts["trap-caio"]

	preview := w.jumpPreview()
	if !preview.ok() {
		t.Fatalf("the preview of the jump: status %d %s", preview.status, preview.body)
	}
	var opts playv1.GetMoveOptionsResponse
	if err := protojson.Unmarshal(preview.body, &opts); err != nil {
		t.Fatalf("read the preview: %v", err)
	}
	landing := func(col int32) *playv1.ReachableSquare {
		for _, r := range opts.GetReachable() {
			if r.GetCol() == col && r.GetRow() == jumpRow {
				return r
			}
		}
		return nil
	}
	if landing(jumpLand) == nil || landing(jumpOver) == nil {
		t.Fatalf("the preview lists %d squares and not both (%d, %d) and (%d, %d)", len(opts.GetReachable()), jumpOver, jumpRow, jumpLand, jumpRow)
	}
	for _, r := range opts.GetReachable() {
		if r.GetKnownTrapPointId() != "" || r.GetKnownTrapName() != "" {
			t.Errorf("the square (%d, %d) of the jump names the trap %q before it is known", r.GetCol(), r.GetRow(), r.GetKnownTrapName())
		}
	}
	for _, f := range w.aboutTraps(w.caio, preview) {
		t.Errorf("the preview: %s", f)
	}

	// The jump itself.
	moved := w.caio.call(playv1connect.CombatServiceMoveCombatantProcedure, &playv1.MoveCombatantRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: w.combatant(w.toren).GetId(), IdempotencyKey: newKey(),
		Col: jumpLand, Row: jumpRow, Jump: playv1.JumpKind_JUMP_KIND_LONG,
	})
	if !moved.ok() {
		t.Fatalf("the jump: status %d %s", moved.status, moved.body)
	}
	for _, f := range w.aboutTraps(w.caio, moved) {
		t.Errorf("the jump's answer: %s", f)
	}
	for name, procedure := range map[string]string{"the combat": playv1connect.CombatServiceGetEncounterProcedure, "the log": playv1connect.CombatServiceListCombatLogProcedure} {
		var req proto.Message
		if name == "the log" {
			req = &playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()}
		} else {
			req = &playv1.GetEncounterRequest{CampaignId: w.campaign}
		}
		r := w.caio.call(procedure, req)
		if !r.ok() {
			t.Fatalf("%s: status %d %s", name, r.status, r.body)
		}
		for _, f := range w.aboutTraps(w.caio, r) {
			t.Errorf("%s after the jump: %s", name, f)
		}
	}
	m := w.caio.call("/meurpg.maps.v1.MapService/GetMap", &mapsv1.GetMapRequest{CampaignId: w.campaign, MapId: w.fogMap})
	for _, f := range w.aboutTraps(w.caio, m) {
		t.Errorf("the map after the jump: %s", f)
	}

	// The control: revealed to Toren, the trap is named where the jump lands in it. Caio
	// may read what his character knows.
	must(w.master.maps.RevealTrap(w.t.Context(), rq(&mapsv1.RevealTrapRequest{
		CampaignId: w.campaign, MapId: w.fogMap, PointId: trap.GetId(), CharacterIds: []string{w.toren.GetId()},
	})))
	for _, c := range w.secrets.list {
		if c.needle == trap.GetId() || c.kind == "trap-caio-name" {
			c.readers = names([]*person{w.caio})
		}
	}
	jumpTurn(w) // back to Toren's turn, with a run to jump from
	again := w.jumpPreview()
	if !again.ok() {
		t.Fatalf("the preview after the reveal: status %d %s", again.status, again.body)
	}
	var known playv1.GetMoveOptionsResponse
	if err := protojson.Unmarshal(again.body, &known); err != nil {
		t.Fatalf("read the preview: %v", err)
	}
	named := 0
	for _, r := range known.GetReachable() {
		if r.GetKnownTrapPointId() == trap.GetId() && r.GetKnownTrapName() != "" {
			named++
			if r.GetCol() != jumpOver {
				t.Errorf("the jump warns about the trap on (%d, %d), want only the square it lands in, (%d, %d)", r.GetCol(), r.GetRow(), jumpOver, jumpRow)
			}
		}
	}
	if named == 0 {
		t.Errorf("after the reveal the jump to (%d, %d) does not name the trap: the control found nothing to read", jumpOver, jumpRow)
	}
	for _, f := range w.aboutTraps(w.caio, again) {
		t.Errorf("the preview after the reveal: %s", f)
	}
}
