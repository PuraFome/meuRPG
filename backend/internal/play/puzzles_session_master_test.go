package play

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The master's puzzle calls (puzzles_master.go, puzzles_session.go) and the hint try
// (puzzles_hints.go): who may call, what a bad or unknown ID answers, the idempotent
// create, the archive that changes nothing, the reseed of a run not shown and the
// try's refusals. Each refusal is frozen with its code and its message.

// puzzleMasterCall is one call of the master's puzzle service, made with the ID it is given.
type puzzleMasterCall struct {
	name string
	do   func(ctx context.Context, c playv1connect.PuzzleServiceClient, campaignID, id string) error
}

func puzzleMasterCalls() []puzzleMasterCall {
	lock := lockConfig(4, playv1.PuzzleAlphabet_PUZZLE_ALPHABET_DIGITS)
	return []puzzleMasterCall{
		{"CreatePuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, _ string) error {
			_, err := c.CreatePuzzle(ctx, connect.NewRequest(&playv1.CreatePuzzleRequest{CampaignId: cid, Name: "X", Config: lock, Solution: lockSolution(1, 2, 3, 4), IdempotencyKey: newKey()}))
			return err
		}},
		{"UpdatePuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.UpdatePuzzle(ctx, connect.NewRequest(&playv1.UpdatePuzzleRequest{CampaignId: cid, PuzzleId: id, Name: "X", Config: lock, Solution: lockSolution(1, 2, 3, 4)}))
			return err
		}},
		{"ArchivePuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.ArchivePuzzle(ctx, connect.NewRequest(&playv1.ArchivePuzzleRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"UnarchivePuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.UnarchivePuzzle(ctx, connect.NewRequest(&playv1.UnarchivePuzzleRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"ListPuzzles", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, _ string) error {
			_, err := c.ListPuzzles(ctx, connect.NewRequest(&playv1.ListPuzzlesRequest{CampaignId: cid}))
			return err
		}},
		{"GetPuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.GetPuzzle(ctx, connect.NewRequest(&playv1.GetPuzzleRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"PreviewPuzzleStart", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, _ string) error {
			_, err := c.PreviewPuzzleStart(ctx, connect.NewRequest(&playv1.PreviewPuzzleStartRequest{CampaignId: cid, Config: lightsConfig(4)}))
			return err
		}},
		{"PreviewPuzzleCipher", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, _ string) error {
			_, err := c.PreviewPuzzleCipher(ctx, connect.NewRequest(&playv1.PreviewPuzzleCipherRequest{CampaignId: cid, Solution: cipherSolution("abc", 1).GetCipher()}))
			return err
		}},
		{"ShowPuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.ShowPuzzle(ctx, connect.NewRequest(&playv1.ShowPuzzleRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"ResetPuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.ResetPuzzle(ctx, connect.NewRequest(&playv1.ResetPuzzleRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"ReseedPuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.ReseedPuzzle(ctx, connect.NewRequest(&playv1.ReseedPuzzleRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"ClosePuzzle", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.ClosePuzzle(ctx, connect.NewRequest(&playv1.ClosePuzzleRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"ReleaseNextPuzzleHint", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.ReleaseNextPuzzleHint(ctx, connect.NewRequest(&playv1.ReleaseNextPuzzleHintRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"PlayPuzzleSequence", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.PlayPuzzleSequence(ctx, connect.NewRequest(&playv1.PlayPuzzleSequenceRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"GetMasterPuzzleRun", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, id string) error {
			_, err := c.GetMasterPuzzleRun(ctx, connect.NewRequest(&playv1.GetMasterPuzzleRunRequest{CampaignId: cid, PuzzleId: id}))
			return err
		}},
		{"ListSessionPuzzles", func(ctx context.Context, c playv1connect.PuzzleServiceClient, cid, _ string) error {
			_, err := c.ListSessionPuzzles(ctx, connect.NewRequest(&playv1.ListSessionPuzzlesRequest{CampaignId: cid}))
			return err
		}},
	}
}

// Every master call is the master's: a player, a stranger to the campaign and an anonymous caller
// are refused before the call reads anything.
func TestPuzzleMasterCallsAreTheMastersOnly(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	stranger := p.h.newUser("Estranho")
	callers := []struct {
		name string
		c    playv1connect.PuzzleServiceClient
		code connect.Code
		msg  string
	}{
		{"a player", p.pc(p.caio), connect.CodePermissionDenied, "only the campaign's master can do this"},
		{"a stranger", p.pc(stranger), connect.CodeNotFound, "campaign not found"},
		{"nobody", playv1connect.NewPuzzleServiceClient(p.h.server.Client(), p.h.server.URL), connect.CodeUnauthenticated, "sign in to continue"},
	}
	for _, caller := range callers {
		for _, call := range puzzleMasterCalls() {
			err := call.do(t.Context(), caller.c, p.campaignID, puz.GetId())
			wantRefusal(t, call.name+" by "+caller.name, err, caller.code, caller.msg)
		}
	}
}

// A bad ID and an ID of no puzzle are refused with the same words by every call that takes one;
// the master check comes first, the open session next, the ID after.
func TestPuzzleMasterCallsRefuseABadOrUnknownPuzzleID(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	withID := map[string]bool{
		"UpdatePuzzle": true, "ArchivePuzzle": true, "UnarchivePuzzle": true, "GetPuzzle": true, "ShowPuzzle": true, "ResetPuzzle": true,
		"ReseedPuzzle": true, "ClosePuzzle": true, "ReleaseNextPuzzleHint": true, "PlayPuzzleSequence": true, "GetMasterPuzzleRun": true,
	}
	for _, call := range puzzleMasterCalls() {
		if !withID[call.name] {
			continue
		}
		err := call.do(t.Context(), p.pc(p.master), p.campaignID, "nope")
		wantRefusal(t, call.name+" with a bad ID", err, connect.CodeNotFound, "puzzle not found")
		err = call.do(t.Context(), p.pc(p.master), p.campaignID, newKey())
		wantRefusal(t, call.name+" with an unknown ID", err, connect.CodeNotFound, "puzzle not found")
	}
}

// A create with the same key and the same request gives the first puzzle back and makes no other;
// the same key with another request is refused; a key that is not a UUID is taken as it is.
// (the UNSOLVABLE refusal of fromBuilt is out of reach: a kind draws its start from its answer, or
// is solvable from any start it is given).
func TestPuzzleMasterCreateReplayAndKey(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	req := func(name string) *playv1.CreatePuzzleRequest {
		return &playv1.CreatePuzzleRequest{CampaignId: p.campaignID, Name: name, Config: lightsConfig(4), IdempotencyKey: "00000000-0000-4000-8000-000000000001"}
	}
	first, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req("Luzes")))
	if err != nil {
		t.Fatal(err)
	}
	again, err := p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req("Luzes")))
	if err != nil || again.Msg.GetPuzzle().GetId() != first.Msg.GetPuzzle().GetId() || jsonOf(again.Msg) != jsonOf(first.Msg) {
		t.Fatalf("the retry = %v, %v; want the first puzzle", again, err)
	}
	listed, err := p.pc(p.master).ListPuzzles(t.Context(), connect.NewRequest(&playv1.ListPuzzlesRequest{CampaignId: p.campaignID}))
	if err != nil || len(listed.Msg.GetPuzzles()) != 1 {
		t.Fatalf("ListPuzzles() = %v, %v; want the one puzzle", listed, err)
	}
	_, err = p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(req("Outro nome")))
	wantRefusal(t, "a create with a key spent on another request", err, connect.CodeInvalidArgument, "idempotency_key was used for another change")
	// Finding: a create takes any text as its key (idem.Clean does not demand a UUID, unlike the
	// other calls), and the key makes a puzzle of its own.
	loose := req("Solto")
	loose.IdempotencyKey = "não é uuid"
	if _, err = p.pc(p.master).CreatePuzzle(t.Context(), connect.NewRequest(loose)); err != nil {
		t.Errorf("a create with a key that is not a UUID error = %v, want it made", err)
	}
}

// Archiving a puzzle that is archived already, or bringing back one that is not, changes nothing
// (the row keeps its update time) and answers the puzzle as it is.
func TestPuzzleMasterArchiveChangesNothingTwice(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lock(t, "Cofre")
	archive := func(on bool) *playv1.Puzzle {
		t.Helper()
		var (
			res *playv1.Puzzle
			err error
		)
		if on {
			var r *connect.Response[playv1.ArchivePuzzleResponse]
			r, err = p.pc(p.master).ArchivePuzzle(t.Context(), connect.NewRequest(&playv1.ArchivePuzzleRequest{CampaignId: p.campaignID, PuzzleId: puz.GetId()}))
			res = r.Msg.GetPuzzle()
		} else {
			var r *connect.Response[playv1.UnarchivePuzzleResponse]
			r, err = p.pc(p.master).UnarchivePuzzle(t.Context(), connect.NewRequest(&playv1.UnarchivePuzzleRequest{CampaignId: p.campaignID, PuzzleId: puz.GetId()}))
			res = r.Msg.GetPuzzle()
		}
		if err != nil {
			t.Fatalf("archive(%v) error = %v", on, err)
		}
		return res
	}
	if got := archive(false); got.GetArchived() || got.GetId() != puz.GetId() {
		t.Fatalf("bringing back a puzzle that is not archived = %v", got)
	}
	first := archive(true)
	if !first.GetArchived() {
		t.Fatalf("an archived puzzle = %v", first)
	}
	if second := archive(true); !second.GetArchived() || second.GetUpdatedAt().AsTime() != first.GetUpdatedAt().AsTime() {
		t.Errorf("archiving twice = %v then %v; want the same puzzle", first, second)
	}
	if got := archive(false); got.GetArchived() {
		t.Errorf("a puzzle brought back = %v", got)
	}
}

// An edit of a puzzle that does not exist, or that was shown, is refused; a generated start is
// kept when the seed is 0 and what makes it stays.
func TestPuzzleMasterUpdateRefusals(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.lights(t, "Luzes", 5)
	update := func(id string) error {
		_, err := p.pc(p.master).UpdatePuzzle(t.Context(), connect.NewRequest(&playv1.UpdatePuzzleRequest{CampaignId: p.campaignID, PuzzleId: id, Name: "Luzes 2", Config: lightsConfig(5)}))
		return err
	}
	wantRefusal(t, "an edit of no puzzle", update(newKey()), connect.CodeNotFound, "puzzle not found")
	p.show(t, puz.GetId())
	err := update(puz.GetId())
	wantPuzzleBlocked(t, "an edit of a shown puzzle", err, playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_ALREADY_SHOWN)
	wantRefusal(t, "an edit of a shown puzzle", err, connect.CodeFailedPrecondition, "the puzzle was shown in a session: it cannot be edited any more")
}

// A reseed of a kind with no generated start is refused; a reseed of a run prepared and not shown
// draws another start and shows nothing; the run is still the master's alone.
func TestPuzzleSessionReseedPreparedAndNotGenerated(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	reseed := func(id string) (*playv1.MasterPuzzleRun, error) {
		res, err := p.pc(p.master).ReseedPuzzle(t.Context(), connect.NewRequest(&playv1.ReseedPuzzleRequest{CampaignId: p.campaignID, PuzzleId: id}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetRun(), nil
	}
	riddle := p.riddle(t, "Enigma")
	_, err := reseed(riddle.GetId())
	wantPuzzleBlocked(t, "a reseed of a riddle", err, playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_GENERATED_START)
	wantRefusal(t, "a reseed of a riddle", err, connect.CodeFailedPrecondition, "this kind has no generated start")

	lights := p.lights(t, "Luzes", 5)
	one, err := reseed(lights.GetId())
	if err != nil || one.GetStatus() != playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_NOT_SHOWN || one.GetRun().GetRevision() != 0 {
		t.Fatalf("a reseed before showing = %v, %v; want a run prepared and not shown", one, err)
	}
	two, err := reseed(lights.GetId())
	if err != nil || two.GetStatus() != playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_NOT_SHOWN {
		t.Fatalf("a second reseed before showing = %v, %v", two, err)
	}
	var runs, revision int
	if err := p.h.pool.QueryRow(t.Context(), `SELECT count(*), COALESCE(max(revision), -1) FROM puzzle_runs WHERE puzzle_id = $1`, lights.GetId()).Scan(&runs, &revision); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || revision != 2 {
		t.Errorf("the lights puzzle has %d runs at revision %d, want 1 run at revision 2 (made, then saved by the second reseed)", runs, revision)
	}
	if got := p.eventCount(t, eventPuzzleReset); got != 0 {
		t.Errorf("a reseed of a run not shown wrote %d reset events, want 0", got)
	}
}

// The master's list of the session reads every puzzle that is not archived with its run (none for
// a puzzle not shown), and the single read of a puzzle with no run answers the puzzle alone.
func TestPuzzleSessionListAndReadOfTheMasterView(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	idle := p.lock(t, "Parado")
	shown := p.lock(t, "Mostrado")
	gone := p.lock(t, "Arquivado")
	p.show(t, shown.GetId())
	p.mustMove(t, p.caio, shown.GetId(), lockMove(0, 1))
	if _, err := p.pc(p.master).ArchivePuzzle(t.Context(), connect.NewRequest(&playv1.ArchivePuzzleRequest{CampaignId: p.campaignID, PuzzleId: gone.GetId()})); err != nil {
		t.Fatal(err)
	}
	res, err := p.pc(p.master).ListSessionPuzzles(t.Context(), connect.NewRequest(&playv1.ListSessionPuzzlesRequest{CampaignId: p.campaignID}))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, run := range res.Msg.GetPuzzles() {
		got = append(got, fmt.Sprintf("%s status=%v shown=%t moves=%d", run.GetPuzzle().GetName(), run.GetStatus(), run.GetPuzzle().GetShown(), run.GetMovesMade()))
	}
	want := []string{"Parado status=PUZZLE_RUN_STATUS_NOT_SHOWN shown=false moves=0", "Mostrado status=PUZZLE_RUN_STATUS_SHOWN shown=true moves=1"}
	if strings.Join(got, "|") != strings.Join(want, "|") && strings.Join(got, "|") != want[1]+"|"+want[0] {
		t.Errorf("ListSessionPuzzles = %q, want %q (in any order)", got, want)
	}
	read := p.masterRun(t, idle.GetId())
	if read.GetStatus() != playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_NOT_SHOWN || read.GetRun() != nil || read.GetPuzzle().GetName() != "Parado" || read.GetPuzzle().GetShown() {
		t.Errorf("the read of a puzzle with no run = %v", read)
	}
	if run := p.masterRun(t, shown.GetId()); run.GetStatus() != playv1.PuzzleRunStatus_PUZZLE_RUN_STATUS_SHOWN || run.GetMovesMade() != 1 {
		t.Errorf("the read of a shown puzzle = %v", run)
	}
}

// The try's own checks, in the order they are made: the caller, the key, the ID, the roll.
func TestPuzzleHintTryRefusesBeforeRolling(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.riddle(t, "Enigma", hintCheck("skill:investigation", 15))
	p.show(t, puz.GetId())
	try := func(u *user, id, key string, roll func(*playv1.TryPuzzleHintRequest)) error {
		req := &playv1.TryPuzzleHintRequest{CampaignId: p.campaignID, PuzzleId: id, IdempotencyKey: key}
		roll(req)
		_, err := p.pc(u).TryPuzzleHint(t.Context(), connect.NewRequest(req))
		return err
	}
	inApp := func(r *playv1.TryPuzzleHintRequest) { r.Roll = &playv1.TryPuzzleHintRequest_RollInApp{RollInApp: true} }
	wantRefusal(t, "a try by a stranger", try(p.h.newUser("Estranho"), puz.GetId(), newKey(), inApp), connect.CodeNotFound, "campaign not found")
	wantRefusal(t, "a try by the master", try(p.master, puz.GetId(), newKey(), inApp), connect.CodePermissionDenied, "only the campaign's player can do this")
	wantRefusal(t, "a try with a bad key", try(p.caio, puz.GetId(), "x", inApp), connect.CodeInvalidArgument, "idempotency_key must be a UUID")
	wantRefusal(t, "a try with a bad ID", try(p.caio, "nope", newKey(), inApp), connect.CodeNotFound, "puzzle not found")
	wantRefusal(t, "a try with roll_in_app false", try(p.caio, puz.GetId(), newKey(), func(r *playv1.TryPuzzleHintRequest) {
		r.Roll = &playv1.TryPuzzleHintRequest_RollInApp{}
	}), connect.CodeInvalidArgument, "roll_in_app must be true")
	wantRefusal(t, "a try with three typed dice", try(p.caio, puz.GetId(), newKey(), func(r *playv1.TryPuzzleHintRequest) { r.D20Faces = []int32{1, 2, 3} }),
		connect.CodeInvalidArgument, "set roll_in_app, d20_face or d20_faces")
	wantRefusal(t, "a try with a typed die out of range", try(p.caio, puz.GetId(), newKey(), func(r *playv1.TryPuzzleHintRequest) { r.D20Faces = []int32{21} }),
		connect.CodeInvalidArgument, "d20_faces must be 1 to 20 each")
	wantRefusal(t, "a try with two typed dice and no advantage", try(p.caio, puz.GetId(), newKey(), func(r *playv1.TryPuzzleHintRequest) { r.D20Faces = []int32{3, 4} }),
		connect.CodeInvalidArgument, "the roll takes 1 d20: type 1 face(s) in d20_faces")
	if m := p.masterRun(t, puz.GetId()); len(m.GetHintTries()) != 0 {
		t.Errorf("a refused try left %d tries", len(m.GetHintTries()))
	}
}

// A member who has no living character cannot try; the refusal is the same whatever the roll.
func TestPuzzleHintTryNeedsACharacter(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.riddle(t, "Enigma", hintCheck("skill:investigation", 15))
	p.show(t, puz.GetId())
	late := p.h.newUser("Lia")
	p.h.join(p.master, p.campaignID, late)
	_, err := p.tryHint(t, late, puz.GetId(), 12)
	wantPuzzleBlocked(t, "a try with no character", err, playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_NO_CHARACTER)
	wantRefusal(t, "a try with no character", err, connect.CodeFailedPrecondition, "you have no living character to roll for")
}

// A try that was stopped by a limit, or on a closed puzzle, says so before it rolls anything.
func TestPuzzleHintTryOnAStoppedPuzzle(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	puz := p.riddle(t, "Enigma", func(r *playv1.CreatePuzzleRequest) {
		hintCheck("skill:investigation", 15)(r)
		onWrong(func(o *playv1.PuzzleOnWrong) { o.MaxMoves = 1 })(r)
	})
	p.show(t, puz.GetId())
	p.mustMove(t, p.caio, puz.GetId(), riddleMove("luz"))
	_, err := p.tryHint(t, p.ana, puz.GetId(), 12)
	wantPuzzleBlocked(t, "a try at a stopped puzzle", err, playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_STOPPED)
	wantRefusal(t, "a try at a stopped puzzle", err, connect.CodeFailedPrecondition, "a limit stopped the puzzle")
}

// sameTypedFaces and hintRollProto read a try row without a database.
func TestPuzzleHintTryRowReads(t *testing.T) {
	t.Parallel()
	one := playdb.PuzzleHintTry{D20: 9, Modifier: 3, Total: 12, Physical: true}
	pair := playdb.PuzzleHintTry{D20: 4, D20B: 17, Counted: 1, Modifier: 3, Total: 20, Physical: true}
	lucky := playdb.PuzzleHintTry{D20: 6, D20B: 15, Counted: 1, Modifier: 3, Total: 18, Physical: true, LuckyIndex: new(int32(0))}
	tests := []struct {
		name string
		done playdb.PuzzleHintTry
		in   rollInput
		want bool
	}{
		{"one die, the same", one, rollInput{typed: 9}, true},
		{"one die, another", one, rollInput{typed: 8}, false},
		{"one die, typed as a list", one, rollInput{typedFaces: []int{9}}, true},
		{"one die, typed as two", one, rollInput{typedFaces: []int{9, 9}}, false},
		{"a pair, in the order rolled", pair, rollInput{typedFaces: []int{17, 4}}, true},
		{"a pair, in the order typed first", pair, rollInput{typedFaces: []int{4, 17}}, false},
		{"a pair, typed as one", pair, rollInput{typed: 4}, false},
		{"a pair, one face off", pair, rollInput{typedFaces: []int{4, 16}}, false},
		{"a pair with Lucky, the 1 that was typed", lucky, rollInput{typedFaces: []int{1, 6}}, true},
		{"a pair with Lucky, the face it became", lucky, rollInput{typedFaces: []int{15, 6}}, false},
	}
	for _, tt := range tests {
		if got := sameTypedFaces(tt.done, tt.in); got != tt.want {
			t.Errorf("%s: sameTypedFaces = %v, want %v", tt.name, got, tt.want)
		}
	}
	if r := hintRollProto(pair); len(r.GetFaces()) != 2 || r.GetCountedIndex() != 1 || r.GetTotal() != 20 || len(r.GetRerolled()) != 0 {
		t.Errorf("the roll of a pair = %v", r)
	}
	if r := hintRollProto(lucky); len(r.GetFaces()) != 2 || len(r.GetRerolled()) != 1 || r.GetRerolled()[0].GetTo() != 15 {
		t.Errorf("the roll of a pair with Lucky = %v", r)
	}
	if r := hintRollProto(one); len(r.GetFaces()) != 1 || r.GetFaces()[0] != 9 || r.GetTotal() != 12 {
		t.Errorf("the roll of one die = %v", r)
	}
}
