package play

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of what each viewer reads of a puzzle run (MR-038, RN-10, RN-27) and of
// the move's "Ao resolver" when its target is gone. Every viewer's answer is a golden file
// (testdata/puzzle_view_*.golden.json, ids replaced by names, times by a mark). Run with
// -update-puzzle-view to write them again after a change of behavior that was meant.

var updatePuzzleView = flag.Bool("update-puzzle-view", false, "write the golden files of the puzzle run's viewers again")

var (
	puzzleViewUUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	puzzleViewTime = regexp.MustCompile(`\d{4}-\d\d-\d\dT[0-9:.]+Z`)
)

// puzzleViewGolden compares the message, as compact JSON with its ids named, with the golden file.
func puzzleViewGolden(t *testing.T, name string, labels map[string]string, m proto.Message) {
	t.Helper()
	raw, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	// protojson adds a random space after some colons, so the text is parsed and written again.
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	stable, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := string(stable)
	for id, label := range labels {
		got = strings.ReplaceAll(got, id, "<"+label+">")
	}
	got = puzzleViewUUID.ReplaceAllString(got, "<uuid>")
	got = puzzleViewTime.ReplaceAllString(got, "<time>") + "\n"
	path := filepath.Join("testdata", "puzzle_view_"+name+".golden.json")
	if *updatePuzzleView {
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file: %v (write it with -update-puzzle-view)", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from %s:\n%s", name, path, got)
	}
}

// The riddle with everything a viewer may read of it: hints won by a check, split
// information, attempts per player, a limit of moves, a trap that a wrong move fired.
func TestPuzzleView_EachViewerReadsTheirOwnRun(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	trap := p.trapPoint(t, "Dardos envenenados", 12, 4)
	puz := p.riddle(t, "A porta da Cripta pergunta", func(r *playv1.CreatePuzzleRequest) {
		hintCheck("skill:investigation", 15)(r)
		r.Parts = []*playv1.PuzzlePart{
			{CharacterId: p.toren.GetId(), Text: "A porta ouve o que o chão esconde…"},
			{CharacterId: p.pens.GetId(), Text: "…os tambores ecoam três vezes."},
		}
		onWrong(func(o *playv1.PuzzleOnWrong) {
			o.AttemptsPerPlayer, o.MaxMoves = 2, 9
			o.Trap = &playv1.PuzzleTrapTarget{MapId: p.mapID, PointId: trap.GetId()}
		})(r)
	})
	labels := map[string]string{
		puz.GetId(): "puzzle", p.mapID: "map", trap.GetId(): "trap",
		p.toren.GetId(): "toren", p.pens.GetId(): "pensantus", p.bri.GetId(): "brisa",
	}
	p.show(t, puz.GetId())
	if _, err := p.tryHint(t, p.caio, puz.GetId(), 20); err != nil { // Toren wins the first hint
		t.Fatal(err)
	}
	if _, err := p.tryHint(t, p.ana, puz.GetId(), 1); err != nil { // Pensantus fails: tried, won nothing
		t.Fatal(err)
	}
	p.release(t, puz.GetId())
	p.mustMove(t, p.caio, puz.GetId(), riddleMove("luz")) // wrong: fires the trap
	p.mustMove(t, p.caio, puz.GetId(), riddleMove("fogo"))
	p.mustMove(t, p.ana, puz.GetId(), riddleMove("vento"))

	// A second riddle whose only wrong move is the one that fired its trap: the last move names it
	// to a player that sees the point, and the master reads it too.
	trap2 := p.trapPoint(t, "Fosso", 14, 4)
	rid := p.riddle(t, "Enigma", onWrong(onWrongTrap(&playv1.PuzzleTrapTarget{MapId: p.mapID, PointId: trap2.GetId()})))
	labels[rid.GetId()], labels[trap2.GetId()] = "riddle", "trap2"
	p.show(t, rid.GetId())
	p.mustMove(t, p.caio, rid.GetId(), riddleMove("luz"))
	puzzleViewGolden(t, "trap_master", labels, p.masterRun(t, rid.GetId()))
	puzzleViewGolden(t, "trap_toren", labels, p.read(t, p.caio, rid.GetId()))
	puzzleViewGolden(t, "trap_brisa", labels, p.read(t, p.bia, rid.GetId()))

	puzzleViewGolden(t, "master", labels, p.masterRun(t, puz.GetId()))
	puzzleViewGolden(t, "master_as_player", labels, p.read(t, p.master, puz.GetId()))
	puzzleViewGolden(t, "toren", labels, p.read(t, p.caio, puz.GetId()))
	puzzleViewGolden(t, "pensantus", labels, p.read(t, p.ana, puz.GetId()))
	puzzleViewGolden(t, "brisa", labels, p.read(t, p.bia, puz.GetId()))
	// Toren has no attempt left in the round: a third wrong answer is refused, with the exact words.
	_, err := p.move(t, p.caio, puz.GetId(), riddleMove("terra"))
	wantPuzzleBlocked(t, "a move with no attempts left", err, playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_ATTEMPTS_LEFT)
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "you have no attempts left in this round") {
		t.Errorf("no attempts left: %v", err)
	}
	// A stranger finds nothing of the campaign.
	stranger := p.h.newUser("Intruso")
	_, err = p.pc(stranger).GetPuzzleRun(t.Context(), connect.NewRequest(&playv1.GetPuzzleRunRequest{CampaignId: p.campaignID, PuzzleId: puz.GetId()}))
	wantCode(t, "a stranger reads the run", err, connect.CodeNotFound)
	_, err = p.pc(stranger).MakePuzzleMove(t.Context(), connect.NewRequest(&playv1.MakePuzzleMoveRequest{CampaignId: p.campaignID, PuzzleId: puz.GetId(), Move: riddleMove("a"), IdempotencyKey: newKey()}))
	wantCode(t, "a stranger moves", err, connect.CodeNotFound)
}

// The cipher: a player that found the key clue reads where it is, the others only that it exists;
// the sequence reads its playback for everyone.
func TestPuzzleView_TheCipherKeyAndTheSequencePlayback(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	x, y := sq(6, 6)
	pt, err := p.mc(p.master).CreateMapPoint(t.Context(), connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: p.campaignID, MapId: p.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "A biblioteca", XBp: x, YBp: y,
	}))
	if err != nil {
		t.Fatal(err)
	}
	clue, err := p.mc(p.master).AddSceneClue(t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{
		CampaignId: p.campaignID, MapId: p.mapID, PointId: pt.Msg.GetPoint().GetId(), Text: "Cada letra anda três para trás.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	clueID := clue.Msg.GetClue().GetId()
	cip := p.cipher(t, "A carta do Capitão", func(r *playv1.CreatePuzzleRequest) { r.Config = cipherConfig(clueID) })
	seq := p.sequence(t, "Os sinos")
	labels := map[string]string{cip.GetId(): "cipher", seq.GetId(): "sequence", clueID: "clue", p.mapID: "map"}
	p.show(t, cip.GetId())
	p.show(t, seq.GetId())
	if _, err := p.mc(p.master).RevealSceneClue(t.Context(), connect.NewRequest(&mapsv1.RevealSceneClueRequest{CampaignId: p.campaignID, ClueId: clueID, CharacterIds: []string{p.pens.GetId()}})); err != nil {
		t.Fatal(err)
	}
	p.mustMove(t, p.bia, cip.GetId(), cipherMove("nada"))
	puzzleViewGolden(t, "cipher_pensantus", labels, p.read(t, p.ana, cip.GetId()))
	puzzleViewGolden(t, "cipher_toren", labels, p.read(t, p.caio, cip.GetId()))
	puzzleViewGolden(t, "cipher_master", labels, p.masterRun(t, cip.GetId()))
	p.playSequence(t, seq.GetId())
	p.movableClock().advance(time.Minute)
	p.mustMove(t, p.caio, seq.GetId(), bellMove(1))
	got := p.read(t, p.ana, seq.GetId())
	if got.GetSequence() == nil || got.GetSequence().GetTotalSteps() != 4 || got.GetLastMove().GetMove() != nil {
		t.Errorf("a player reads the sequence as %v", got)
	}
	if j := jsonOf(p.masterRun(t, seq.GetId())); !strings.Contains(j, "\"steps\"") {
		t.Errorf("the master reads no solution: %s", j)
	}
}

// A target of "Ao resolver" that is gone does not undo the solve: the master reads TARGET_GONE.
func TestPuzzleView_SolvingATargetThatIsGoneIsTargetGone(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	other := p.h.newMap(p.campaignID, gridColumns)
	x, y := sq(5, 5)
	point, err := p.mc(p.master).CreateMapPoint(t.Context(), connect.NewRequest(&mapsv1.CreateMapPointRequest{
		CampaignId: p.campaignID, MapId: p.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "A câmara", XBp: x, YBp: y,
	}))
	if err != nil {
		t.Fatal(err)
	}
	clue, err := p.mc(p.master).AddSceneClue(t.Context(), connect.NewRequest(&mapsv1.AddSceneClueRequest{
		CampaignId: p.campaignID, MapId: p.mapID, PointId: point.Msg.GetPoint().GetId(), Text: "Uma pista.",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.mc(p.master).PaintMapCells(t.Context(), connect.NewRequest(&mapsv1.PaintMapCellsRequest{
		CampaignId: p.campaignID, MapId: other, Layer: mapsv1.MapLayer_MAP_LAYER_DOORS, Value: int32(mapsv1.DoorState_DOOR_STATE_CLOSED), Squares: []*mapsv1.MapSquare{{Col: 7, Row: 4}},
	})); err != nil {
		t.Fatalf("PaintMapCells(door) error = %v", err)
	}
	door := p.oneMoveLock(t, doorTarget(other, 7, 4))
	reveal := p.oneMoveLock(t, &playv1.PuzzleOnSolve{
		Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_POINT,
		Target: &playv1.PuzzleOnSolve_Point{Point: &playv1.PuzzlePointTarget{MapId: p.mapID, PointId: point.Msg.GetPoint().GetId()}},
	})
	pick := p.oneMoveLock(t, &playv1.PuzzleOnSolve{
		Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE,
		Target: &playv1.PuzzleOnSolve_Clue{Clue: &playv1.PuzzleClueTarget{ClueId: clue.Msg.GetClue().GetId()}},
	})
	if _, err := p.mc(p.master).DeleteMapPoint(t.Context(), connect.NewRequest(&mapsv1.DeleteMapPointRequest{CampaignId: p.campaignID, MapId: p.mapID, PointId: point.Msg.GetPoint().GetId()})); err != nil {
		t.Fatal(err)
	}
	if _, err := p.mc(p.master).DeleteMap(t.Context(), connect.NewRequest(&mapsv1.DeleteMapRequest{CampaignId: p.campaignID, MapId: other})); err != nil {
		t.Fatalf("DeleteMap() error = %v", err)
	}
	for name, id := range map[string]string{"door": door.GetId(), "point": reveal.GetId(), "clue": pick.GetId()} {
		if res := p.mustMove(t, p.caio, id, lockMove(0, 1)); !res.GetSolvedByThisMove() {
			t.Fatalf("%s: not solved: %v", name, res)
		}
		if got := p.masterRun(t, id).GetOutcome(); got != playv1.PuzzleSolveOutcome_PUZZLE_SOLVE_OUTCOME_TARGET_GONE {
			t.Errorf("%s: outcome = %v, want TARGET_GONE", name, got)
		}
		if msg := p.read(t, p.ana, id).GetSolvedMessage(); msg != "" {
			t.Errorf("%s: the players read %q, want nothing", name, msg)
		}
	}
}
