package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The play module's share of the campaign package (MR-050): the puzzles the
// master made and the encounters saved on battle points. What a session did
// (the runs, the moves, the hint tries, the history) is play and stays out.
//
// A puzzle goes through the checks the master's own CreatePuzzle runs: the
// kind builds its start from the config and the answer and proves the puzzle
// can be solved, and the texts, "Ao resolver", "Ao errar", the hint check and
// the split information are normalised the same way. The part of those checks
// that asks the database whether a door, a point, a clue or a character is the
// campaign's is answered from the package itself, since the campaign does not
// exist yet: what a puzzle names must be in the package.

// PackagePart returns the play module's part of the package. srd is the SRD
// content, which knows the creatures of an encounter.
func (s *Service) PackagePart(srd *rules.Content) campaignpackage.Part {
	return &packagePart{s: s, srd: srd}
}

type packagePart struct {
	s   *Service
	srd *rules.Content
}

const (
	stashPuzzles    = "play.puzzles"
	stashEncounters = "play.encounters"
	encountersEntry = "encounters.json"
	nsPuzzle        = "puzzle"
	orderStep       = time.Millisecond
)

// Export implements campaignpackage.Part.
func (p *packagePart) Export(ctx context.Context, tx pgx.Tx, campaignID string, snap *campaignpackage.Snapshot) error {
	q := p.s.queriesIn(tx)
	rows, err := q.ListPuzzles(ctx, playdb.ListPuzzlesParams{CampaignID: campaignID, IncludeArchived: true})
	if err != nil {
		return fmt.Errorf("list the puzzles: %w", err)
	}
	// The list is newest first; the package keeps the order they were made in.
	for _, row := range slices.Backward(rows) {
		d, err := decodePuzzle(row)
		if err != nil {
			return err
		}
		out := &pkgv1.PackagePuzzle{
			Id: row.ID, Name: row.Name, Config: d.config, Solution: d.solution, Start: d.start, Seed: row.Seed, Clue: row.Clue,
			Hints: d.hints, OnSolve: d.onSolve, HintCheck: d.hintCheck, Parts: d.parts, OnWrong: d.onWrong, Archived: row.ArchivedAt != nil,
		}
		name := campaignpackage.EntryName("puzzles", snap.Next("puzzles"), ".json")
		if err := snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_PUZZLE, name, out); err != nil {
			return err
		}
	}
	encs, err := q.ListBattleEncountersOfCampaign(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list the encounters: %w", err)
	}
	if len(encs) == 0 {
		return nil
	}
	list := &pkgv1.PackageEncounters{}
	for _, e := range encs {
		be, err := battleEncounterOf(e.Encounter)
		if err != nil {
			return err
		}
		list.Encounters = append(list.Encounters, &pkgv1.PackageEncounter{PointId: e.MapPointID, Encounter: be})
	}
	return snap.AddMessage(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_ENCOUNTERS, encountersEntry, list)
}

// stagedPuzzle is a puzzle checked and turned into its columns.
type stagedPuzzle struct {
	cols     puzzleColumns
	d        puzzleDef
	action   string
	archived bool
	at       time.Time
}

type stagedEncounter struct {
	pointOld string
	json     []byte
	at       time.Time
}

// Stage implements campaignpackage.Part.
func (p *packagePart) Stage(_ context.Context, in *campaignpackage.Import) error {
	var puzzles []*stagedPuzzle
	base := p.s.now()
	for i, f := range in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_PUZZLE) {
		var pp pkgv1.PackagePuzzle
		if !in.Read(f.GetPath(), &pp) {
			continue
		}
		if sp := p.stagePuzzle(in, &pp, base.Add(time.Duration(i)*orderStep)); sp != nil {
			puzzles = append(puzzles, sp)
			in.Counts.Puzzles++
		}
	}
	in.Set(stashPuzzles, puzzles)

	var encounters []*stagedEncounter
	files := in.Pkg.Entries(pkgv1.PackageEntryKind_PACKAGE_ENTRY_KIND_ENCOUNTERS)
	switch {
	case len(files) > 1 || (len(files) == 1 && files[0].GetPath() != encountersEntry):
		in.Problem(pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_ENCOUNTER, "", pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNEXPECTED_ENTRY, 0)
	case len(files) == 1:
		var list pkgv1.PackageEncounters
		if in.Read(encountersEntry, &list) {
			encounters = p.stageEncounters(in, &list, base)
		}
	}
	in.Set(stashEncounters, encounters)
	return nil
}

func (p *packagePart) stageEncounters(in *campaignpackage.Import, list *pkgv1.PackageEncounters, at time.Time) []*stagedEncounter {
	kind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_ENCOUNTER
	var out []*stagedEncounter
	seen := map[string]bool{}
	for i, e := range list.GetEncounters() {
		pointName := e.GetPointId()
		in.IDs.Ref("point", e.GetPointId(), kind, pointName)
		if in.Facts.PointKind[e.GetPointId()] != "battle" || seen[e.GetPointId()] {
			in.Problem(kind, pointName, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID, 0)
			continue
		}
		seen[e.GetPointId()] = true
		be, _, _, err := normalBattleEncounter(p.srd, e.GetEncounter())
		if err != nil {
			reason := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
			if ce, ok := errors.AsType[*connect.Error](err); ok && strings.Contains(ce.Message(), "not an SRD creature") {
				reason = pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT
			}
			in.Problem(kind, pointName, reason, 0)
			continue
		}
		out = append(out, &stagedEncounter{pointOld: e.GetPointId(), json: toJSON(be), at: at.Add(time.Duration(i) * orderStep)})
	}
	return out
}

// stagePuzzle checks one puzzle; nil when it has a problem (recorded).
func (p *packagePart) stagePuzzle(in *campaignpackage.Import, pp *pkgv1.PackagePuzzle, at time.Time) *stagedPuzzle {
	s := p.s
	kindOfProblem := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PUZZLE
	name := strings.TrimSpace(pp.GetName())
	bad := func(reason pkgv1.PackageProblemReason) *stagedPuzzle {
		in.Problem(kindOfProblem, name, reason, 0)
		return nil
	}
	if _, ok := in.IDs.Define(nsPuzzle, pp.GetId()); !ok {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	input := puzzleInput{
		name: pp.GetName(), config: proto.Clone(pp.GetConfig()).(*playv1.PuzzleConfig), solution: pp.GetSolution(), start: pp.GetStart(), //nolint:forcetypeassert // Clone returns the same type
		seed: pp.GetSeed(), clue: pp.GetClue(), hints: pp.GetHints(),
		onSolve: pp.GetOnSolve(), hintCheck: pp.GetHintCheck(), parts: pp.GetParts(), onWrong: pp.GetOnWrong(),
	}
	if err := input.checkTexts(); err != nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	name = input.name
	kind := kindOf(input.config)
	if kind == nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_UNKNOWN_CONTENT)
	}
	action, target, reason := p.stageOnSolve(in, name, input.onSolve)
	if reason != 0 {
		return bad(reason)
	}
	if clue := input.config.GetCipher().GetKeyClueId(); clue != "" {
		input.config.GetCipher().KeyClueId = in.IDs.Ref("clue", clue, kindOfProblem, name)
	}
	hintCheck, err := s.checkHintCheck(input.hintCheck, input.hints)
	if err != nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	parts, reason := p.stageParts(in, name, input.parts)
	if reason != 0 {
		return bad(reason)
	}
	onWrong, reason := p.stageOnWrong(in, name, kind, input.onWrong)
	if reason != 0 {
		return bad(reason)
	}
	seed := input.seed
	if kind.generates() && seed <= 0 {
		seed = s.newSeed()
	}
	built, err := kind.build(input, seed)
	if err != nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	built.hintCheck, built.parts, built.onWrong = hintCheck, parts, onWrong
	d, err := fromBuilt(kind, input, built, action, target)
	if err != nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	cols, err := columnsOf(d)
	if err != nil {
		return bad(pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID)
	}
	return &stagedPuzzle{cols: cols, d: d, action: action, archived: pp.GetArchived(), at: at}
}

// stageOnSolve normalises "Ao resolver" as checkOnSolve does, and checks its
// target against the package: a door that exists, a point of the map that is not
// a light, a clue of a scene. It rewrites the ids to the new ones.
func (p *packagePart) stageOnSolve(in *campaignpackage.Import, name string, on *playv1.PuzzleOnSolve) (string, *playv1.PuzzleOnSolve, pkgv1.PackageProblemReason) {
	invalid := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	kind := pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PUZZLE
	action := on.GetAction()
	if action == playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_UNSPECIFIED {
		action = playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_NOTIFY
	}
	message, err := checkedText(on.GetMessage(), 0, maxSolveMessage, playv1.PuzzleInvalidReason_PUZZLE_INVALID_REASON_TEXT, "on_solve.message")
	if err != nil {
		return "", nil, invalid
	}
	out := &playv1.PuzzleOnSolve{Action: action, Message: message}
	switch action {
	case playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_NOTIFY:
		if on.GetTarget() != nil {
			return "", nil, invalid
		}
		if message == "" {
			return solveNotify, nil, 0
		}
		return solveNotify, out, 0
	case playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_OPEN_DOOR:
		t := on.GetDoor()
		if t == nil || in.Facts.DoorAt == nil || !in.Facts.DoorAt(t.GetMapId(), int(t.GetCol()), int(t.GetRow())) {
			return "", nil, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE
		}
		out.Target = &playv1.PuzzleOnSolve_Door{Door: &playv1.PuzzleDoorTarget{
			MapId: in.IDs.Ref("map", t.GetMapId(), kind, name), Col: t.GetCol(), Row: t.GetRow(),
		}}
	case playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_POINT:
		t := on.GetPoint()
		if t == nil || in.Facts.PointMap[t.GetPointId()] != t.GetMapId() || in.Facts.PointKind[t.GetPointId()] == "light" {
			return "", nil, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE
		}
		out.Target = &playv1.PuzzleOnSolve_Point{Point: &playv1.PuzzlePointTarget{
			MapId: in.IDs.Ref("map", t.GetMapId(), kind, name), PointId: in.IDs.Ref("point", t.GetPointId(), kind, name),
		}}
	case playv1.PuzzleSolveAction_PUZZLE_SOLVE_ACTION_REVEAL_CLUE:
		t := on.GetClue()
		if t == nil {
			return "", nil, invalid
		}
		out.Target = &playv1.PuzzleOnSolve_Clue{Clue: &playv1.PuzzleClueTarget{ClueId: in.IDs.Ref("clue", t.GetClueId(), kind, name)}}
	default:
		return "", nil, invalid
	}
	return solveActionName(action), out, 0
}

// stageParts normalises the split information as checkParts does: up to 8
// parts of 1 to 300 characters, each for a player's character of the package or
// for nobody yet, and no character twice.
func (p *packagePart) stageParts(in *campaignpackage.Import, name string, parts []*playv1.PuzzlePart) ([]*playv1.PuzzlePart, pkgv1.PackageProblemReason) {
	invalid := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	if len(parts) > maxParts {
		return nil, invalid
	}
	var out []*playv1.PuzzlePart
	owners := map[string]bool{}
	for _, pt := range parts {
		text := strings.TrimSpace(pt.GetText())
		if n := len([]rune(text)); n < 1 || n > maxPartText {
			return nil, invalid
		}
		id := strings.TrimSpace(pt.GetCharacterId())
		if id != "" {
			if !in.Facts.Players[id] {
				return nil, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE
			}
			if owners[id] {
				return nil, invalid
			}
			owners[id] = true
			id = in.IDs.Ref("character", id, pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PUZZLE, name)
		}
		out = append(out, &playv1.PuzzlePart{CharacterId: id, Text: text})
	}
	return out, 0
}

// stageOnWrong normalises "Ao errar" as checkOnWrong does, and checks its trap
// against the package.
func (p *packagePart) stageOnWrong(in *campaignpackage.Import, name string, kind puzzleKind, on *playv1.PuzzleOnWrong) (*playv1.PuzzleOnWrong, pkgv1.PackageProblemReason) {
	invalid := pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_INVALID
	if on == nil {
		return nil, 0
	}
	out := &playv1.PuzzleOnWrong{}
	if n := on.GetAttemptsPerPlayer(); n != 0 {
		if n < 1 || n > maxAttempts || !kind.judges() {
			return nil, invalid
		}
		out.AttemptsPerPlayer = n
	}
	if n := on.GetMaxMoves(); n != 0 {
		if n < 1 || n > maxMovesLimit {
			return nil, invalid
		}
		out.MaxMoves = n
	}
	if n := on.GetTimeLimitSeconds(); n != 0 {
		if n < minTimeLimit || n > maxTimeLimit {
			return nil, invalid
		}
		out.TimeLimitSeconds = n
	}
	if t := on.GetTrap(); t != nil {
		if !kind.judges() {
			return nil, invalid
		}
		if in.Facts.PointMap[t.GetPointId()] != t.GetMapId() || in.Facts.PointKind[t.GetPointId()] != "trap" {
			return nil, pkgv1.PackageProblemReason_PACKAGE_PROBLEM_REASON_NOT_IN_PACKAGE
		}
		out.Trap = &playv1.PuzzleTrapTarget{
			MapId:   in.IDs.Ref("map", t.GetMapId(), pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PUZZLE, name),
			PointId: in.IDs.Ref("point", t.GetPointId(), pkgv1.PackageProblemKind_PACKAGE_PROBLEM_KIND_PUZZLE, name),
		}
	}
	if out.GetTrap() == nil && out.GetAttemptsPerPlayer() == 0 && out.GetMaxMoves() == 0 && out.GetTimeLimitSeconds() == 0 {
		return nil, 0
	}
	return out, 0
}

// Apply implements campaignpackage.Part.
func (p *packagePart) Apply(ctx context.Context, tx pgx.Tx, in *campaignpackage.Import) error {
	q := p.s.queriesIn(tx)
	puzzles, _ := in.Get(stashPuzzles).([]*stagedPuzzle)
	for _, sp := range puzzles {
		d := sp.d
		row, err := q.InsertPuzzle(ctx, playdb.InsertPuzzleParams{
			CampaignID: in.CampaignID, Kind: d.row.Kind, Name: d.row.Name, Config: sp.cols.config, Solution: sp.cols.solution,
			Seed: d.row.Seed, Start: sp.cols.start, MinimumMoves: d.row.MinimumMoves, Clue: d.row.Clue, Hints: sp.cols.hints,
			SolveAction: sp.action, SolveTarget: sp.cols.target, HintSkill: sp.cols.hintSkill, HintDc: sp.cols.hintDC, Parts: sp.cols.parts,
			OnWrong: sp.cols.onWrong, CreatedAt: sp.at,
		})
		if err != nil {
			return fmt.Errorf("insert an imported puzzle: %w", err)
		}
		if sp.archived {
			if _, err := q.SetPuzzleArchived(ctx, playdb.SetPuzzleArchivedParams{CampaignID: in.CampaignID, ID: row.ID, ArchivedAt: &sp.at, UpdatedAt: sp.at}); err != nil {
				return fmt.Errorf("archive an imported puzzle: %w", err)
			}
		}
	}
	encounters, _ := in.Get(stashEncounters).([]*stagedEncounter)
	for _, e := range encounters {
		pointID, ok := in.IDs.Get("point", e.pointOld)
		mapOld := in.Facts.PointMap[e.pointOld]
		mapID, ok2 := in.IDs.Get("map", mapOld)
		if !ok || !ok2 {
			return errors.New("play: an encounter names a point the import did not create")
		}
		if _, err := q.UpsertBattleEncounter(ctx, playdb.UpsertBattleEncounterParams{
			MapPointID: pointID, CampaignID: in.CampaignID, MapID: mapID, Encounter: e.json, CreatedAt: e.at,
		}); err != nil {
			return fmt.Errorf("insert an imported encounter: %w", err)
		}
	}
	return nil
}
