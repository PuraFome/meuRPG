package play

import (
	"strings"
	"testing"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
)

// Characterization of the staging of a puzzle's "Ao resolver", "Ao errar" and
// split information when a campaign package is imported (package.go): what is
// refused, with which reason, and the ids that come out. No database: the
// import is an in-memory campaignpackage.Import.

const (
	pkgInvalid = pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	pkgMissing = pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE
	idMapA     = "11111111-1111-4111-8111-111111111111"
	idPointA   = "22222222-2222-4222-8222-222222222222"
	idClueA    = "33333333-3333-4333-8333-333333333333"
	idPlayerA  = "44444444-4444-4444-8444-444444444444"
	idPlayerB  = "55555555-5555-4555-8555-555555555555"
	idTrapA    = "66666666-6666-4666-8666-666666666666"
	idLightA   = "77777777-7777-4777-8777-777777777777"
)

func newStageImport() *campaignpackage.Import {
	in := campaignpackage.NewImport(nil, "campaign", "user", false, nil)
	in.Facts.DoorAt = func(mapID string, col, row int) bool { return mapID == idMapA && col == 1 && row == 2 }
	for id, kind := range map[string]string{idPointA: "scene", idTrapA: "trap", idLightA: "light"} {
		in.Facts.PointMap[id], in.Facts.PointKind[id] = idMapA, kind
	}
	in.Facts.Players[idPlayerA], in.Facts.Players[idPlayerB] = true, true
	return in
}

// newID is the id the import gave to the thing of the package (the same id every time it is asked).
func newID(t *testing.T, in *campaignpackage.Import, ns, old string) string {
	t.Helper()
	id := in.IDs.Ref(ns, old, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PUZZLE, "")
	if id == "" || id == old || !campaignpackage.ValidID(id) {
		t.Fatalf("the %s %s has no new id (%q)", ns, old, id)
	}
	return id
}

func solveDoor(mapID string, col, row int32) *playv1.PuzzleOnSolve {
	return &playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_OPEN_DOOR,
		Target: &playv1.PuzzleOnSolve_Door{Door: &playv1.PuzzleDoorTarget{MapId: mapID, Col: col, Row: row}}}
}

func solvePoint(mapID, pointID string) *playv1.PuzzleOnSolve {
	return &playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_POINT,
		Target: &playv1.PuzzleOnSolve_Point{Point: &playv1.PuzzlePointTarget{MapId: mapID, PointId: pointID}}}
}

func TestPackageStage_OnSolveRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		on   *playv1.PuzzleOnSolve
		want pkgv1.PackageProblemReason
	}{
		"a message that is too long":  {&playv1.PuzzleOnSolve{Message: strings.Repeat("a", 201)}, pkgInvalid},
		"notify with a target":        {&playv1.PuzzleOnSolve{Target: &playv1.PuzzleOnSolve_Clue{Clue: &playv1.PuzzleClueTarget{ClueId: idClueA}}}, pkgInvalid},
		"an unknown action":           {&playv1.PuzzleOnSolve{Action: 99}, pkgInvalid},
		"a door with no target":       {&playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_OPEN_DOOR}, pkgMissing},
		"a door that is not there":    {solveDoor(idMapA, 3, 3), pkgMissing},
		"a door of another map":       {solveDoor(idPointA, 1, 2), pkgMissing},
		"a point with no target":      {&playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_POINT}, pkgMissing},
		"a point of another map":      {solvePoint(idClueA, idPointA), pkgMissing},
		"a point that is not a point": {solvePoint(idMapA, idClueA), pkgMissing},
		"a light":                     {solvePoint(idMapA, idLightA), pkgMissing},
		"a clue with no target":       {&playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE}, pkgInvalid},
	}
	for name, tc := range cases {
		in := newStageImport()
		action, out, reason := (&packagePart{}).stageOnSolve(in, "Enigma", tc.on)
		if reason != tc.want || action != "" || out != nil {
			t.Errorf("%s: %q, %v, %v; want only the reason %v", name, action, out, reason, tc.want)
		}
	}
}

func TestPackageStage_OnSolveAccepted(t *testing.T) {
	t.Parallel()
	p := &packagePart{}

	in := newStageImport()
	if action, out, reason := p.stageOnSolve(in, "Enigma", nil); action != solveNotify || out != nil || reason != 0 {
		t.Errorf("nothing = %q, %v, %v; want notify with no payload", action, out, reason)
	}
	if action, out, reason := p.stageOnSolve(in, "Enigma", &playv1.PuzzleOnSolve{Message: "  A porta cede.  "}); action != solveNotify || out.GetMessage() != "A porta cede." || out.GetAction() != playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_NOTIFY || reason != 0 {
		t.Errorf("a message = %q, %v, %v; want notify with the trimmed message", action, out, reason)
	}

	action, out, reason := p.stageOnSolve(in, "Enigma", solveDoor(idMapA, 1, 2))
	if action != solveOpenDoor || reason != 0 || out.GetDoor().GetMapId() != newID(t, in, "map", idMapA) || out.GetDoor().GetCol() != 1 || out.GetDoor().GetRow() != 2 {
		t.Errorf("a door = %q, %v, %v; want open_door on the new map id", action, out, reason)
	}
	action, out, reason = p.stageOnSolve(in, "Enigma", solvePoint(idMapA, idPointA))
	if action != solveRevealPoint || reason != 0 || out.GetPoint().GetMapId() != newID(t, in, "map", idMapA) || out.GetPoint().GetPointId() != newID(t, in, "point", idPointA) {
		t.Errorf("a point = %q, %v, %v; want reveal_point on the new ids", action, out, reason)
	}
	clue := &playv1.PuzzleOnSolve{Action: playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE, Message: "x",
		Target: &playv1.PuzzleOnSolve_Clue{Clue: &playv1.PuzzleClueTarget{ClueId: idClueA}}}
	action, out, reason = p.stageOnSolve(in, "Enigma", clue)
	if action != solveRevealClue || reason != 0 || out.GetClue().GetClueId() != newID(t, in, "clue", idClueA) || out.GetMessage() != "x" {
		t.Errorf("a clue = %q, %v, %v; want reveal_clue on the new id, with its message", action, out, reason)
	}
}

func TestPackageStage_Parts(t *testing.T) {
	t.Parallel()
	p := &packagePart{}
	part := func(id, text string) *playv1.PuzzlePart { return &playv1.PuzzlePart{CharacterId: id, Text: text} }

	refusals := map[string]struct {
		parts []*playv1.PuzzlePart
		want  pkgv1.PackageProblemReason
	}{
		"nine parts":                 {make([]*playv1.PuzzlePart, 9), pkgInvalid},
		"an empty text":              {[]*playv1.PuzzlePart{part("", "   ")}, pkgInvalid},
		"a text of 301 characters":   {[]*playv1.PuzzlePart{part("", strings.Repeat("é", 301))}, pkgInvalid},
		"a character not in package": {[]*playv1.PuzzlePart{part(idClueA, "x")}, pkgMissing},
		"a character twice":          {[]*playv1.PuzzlePart{part(idPlayerA, "x"), part(" "+idPlayerA+" ", "y")}, pkgInvalid},
	}
	for name, tc := range refusals {
		if tc.parts[0] == nil { // the nine parts: all empty messages
			for i := range tc.parts {
				tc.parts[i] = part("", "x")
			}
		}
		in := newStageImport()
		out, reason := p.stageParts(in, "Enigma", tc.parts)
		if reason != tc.want || out != nil {
			t.Errorf("%s: %v, %v; want only the reason %v", name, out, reason, tc.want)
		}
	}

	in := newStageImport()
	out, reason := p.stageParts(in, "Enigma", []*playv1.PuzzlePart{part(idPlayerA, " Um "), part("", "Dois"), part(idPlayerB, strings.Repeat("é", 300))})
	if reason != 0 || len(out) != 3 || out[0].GetText() != "Um" || out[0].GetCharacterId() != newID(t, in, "character", idPlayerA) ||
		out[1].GetCharacterId() != "" || out[2].GetCharacterId() != newID(t, in, "character", idPlayerB) {
		t.Errorf("parts = %v, %v; want the three in order with the owners' new ids and trimmed texts", out, reason)
	}
	if out, reason := p.stageParts(newStageImport(), "Enigma", nil); out != nil || reason != 0 {
		t.Errorf("no parts = %v, %v; want none", out, reason)
	}
	eight := make([]*playv1.PuzzlePart, 8)
	for i := range eight {
		eight[i] = part("", "x")
	}
	if out, reason := p.stageParts(newStageImport(), "Enigma", eight); len(out) != 8 || reason != 0 {
		t.Errorf("eight parts = %d, %v; want them all", len(out), reason)
	}
}

func TestPackageStage_OnWrong(t *testing.T) {
	t.Parallel()
	p := &packagePart{}
	judging, plain := riddleKind{}, lightsKind{}
	trapOf := func(mapID, pointID string) *playv1.PuzzleTrapTarget {
		return &playv1.PuzzleTrapTarget{MapId: mapID, PointId: pointID}
	}

	refusals := map[string]struct {
		kind puzzleKind
		on   *playv1.PuzzleOnWrong
		want pkgv1.PackageProblemReason
	}{
		"attempts below 1":                     {judging, &playv1.PuzzleOnWrong{AttemptsPerPlayer: -1}, pkgInvalid},
		"attempts above 10":                    {judging, &playv1.PuzzleOnWrong{AttemptsPerPlayer: 11}, pkgInvalid},
		"attempts on a kind that never judges": {plain, &playv1.PuzzleOnWrong{AttemptsPerPlayer: 3}, pkgInvalid},
		"moves below 1":                        {judging, &playv1.PuzzleOnWrong{MaxMoves: -1}, pkgInvalid},
		"moves above 200":                      {judging, &playv1.PuzzleOnWrong{MaxMoves: 201}, pkgInvalid},
		"a time below 10 seconds":              {judging, &playv1.PuzzleOnWrong{TimeLimitSeconds: 9}, pkgInvalid},
		"a time above four hours":              {judging, &playv1.PuzzleOnWrong{TimeLimitSeconds: 14401}, pkgInvalid},
		"a trap on a kind that never judges":   {plain, &playv1.PuzzleOnWrong{Trap: trapOf(idMapA, idTrapA)}, pkgInvalid},
		"a trap of another map":                {judging, &playv1.PuzzleOnWrong{Trap: trapOf(idClueA, idTrapA)}, pkgMissing},
		"a point that is not a trap":           {judging, &playv1.PuzzleOnWrong{Trap: trapOf(idMapA, idPointA)}, pkgMissing},
	}
	for name, tc := range refusals {
		out, reason := p.stageOnWrong(newStageImport(), "Enigma", tc.kind, tc.on)
		if reason != tc.want || out != nil {
			t.Errorf("%s: %v, %v; want only the reason %v", name, out, reason, tc.want)
		}
	}

	if out, reason := p.stageOnWrong(newStageImport(), "Enigma", judging, nil); out != nil || reason != 0 {
		t.Errorf("nothing = %v, %v; want nothing", out, reason)
	}
	if out, reason := p.stageOnWrong(newStageImport(), "Enigma", judging, &playv1.PuzzleOnWrong{}); out != nil || reason != 0 {
		t.Errorf("an empty rule = %v, %v; want nothing", out, reason)
	}
	in := newStageImport()
	out, reason := p.stageOnWrong(in, "Enigma", judging, &playv1.PuzzleOnWrong{AttemptsPerPlayer: 10, MaxMoves: 200, TimeLimitSeconds: 14400, Trap: trapOf(idMapA, idTrapA)})
	if reason != 0 || out.GetAttemptsPerPlayer() != 10 || out.GetMaxMoves() != 200 || out.GetTimeLimitSeconds() != 14400 ||
		out.GetTrap().GetMapId() != newID(t, in, "map", idMapA) || out.GetTrap().GetPointId() != newID(t, in, "point", idTrapA) {
		t.Errorf("every limit = %v, %v; want them kept with the trap on the new ids", out, reason)
	}
	// A kind that never judges may still limit the moves and the time.
	if out, reason := p.stageOnWrong(newStageImport(), "Enigma", plain, &playv1.PuzzleOnWrong{MaxMoves: 1, TimeLimitSeconds: 10}); reason != 0 || out.GetMaxMoves() != 1 || out.GetTimeLimitSeconds() != 10 {
		t.Errorf("moves and time on a lights puzzle = %v, %v; want them kept", out, reason)
	}
}
