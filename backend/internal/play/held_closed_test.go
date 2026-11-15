package play

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestAHeldRollWhoseGroupCheckClosedIsForgottenWithItsRefusal: a question kept for a group check the master closes before the answer
// cannot be answered any more. The answer is refused with the closed reason, the hold is gone (not kept to refuse the character's
// every later roll) and the character's next held roll is accepted.
func TestAHeldRollWhoseGroupCheckClosedIsForgottenWithItsRefusal(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, dalilaID := joinDalila(t, a.h, a.master, a.campaignID)
	check := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc = 20 })
	if _, err := a.giveOutside(t, a.bia, dalilaID); err != nil {
		t.Fatalf("GiveBardicInspirationOutside() error = %v", err)
	}
	rollFor := func(id string) (*playv1.RollGroupCheckResponse, error) {
		res, err := dalila.contests.RollGroupCheck(t.Context(), connect.NewRequest(&playv1.RollGroupCheckRequest{
			CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: id, Roll: rollIn(),
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	held, err := rollFor(check.GetId())
	if err != nil || held.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("Dalila's roll = %v, %v; want the die's question", held, err)
	}
	if _, err := a.master.contests.CloseGroupCheck(t.Context(), connect.NewRequest(&playv1.CloseGroupCheckRequest{
		CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: check.GetId(),
	})); err != nil {
		t.Fatalf("CloseGroupCheck() error = %v", err)
	}

	_, err = a.answerHeldRoll(t, dalila, held.GetHeldRollOffer().GetHoldId(), dieAnswer(false))
	wantContestBlocked(t, "the answer of a closed group check", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED)
	if n := countRows(t, a.h, countInspirationHold, a.campaignID); n != 0 {
		t.Errorf("held rolls after the refusal = %d, want it forgotten", n)
	}
	if list := a.listHeldRolls(t, dalila); len(list) != 0 {
		t.Errorf("Dalila lists %v, want nothing", list)
	}

	next := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc = 20 })
	again, err := rollFor(next.GetId())
	if err != nil || again.GetHeldRollOffer().GetDie() == nil {
		t.Fatalf("Dalila's next roll = %v, %v; want it accepted, with the die's question", again, err)
	}
}

// TestHeldClosedIsOnlyForWhatTheRollWasForGoingAway: the rerun's failures that say the thing the roll was for is closed or gone
// make the hold go; any other failure keeps it, so the player can answer again.
func TestHeldClosedIsOnlyForWhatTheRollWasForGoingAway(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"the session ended", errNoOpenSession(), true},
		{"the group check closed", errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED, "closed"), true},
		{"the puzzle has no open session", errNoOpenSessionForPuzzle(), true},
		{"the puzzle was solved", puzzleBlocked(playv1.PuzzleBlockedReason_PUZZLE_BLOCKED_REASON_SOLVED, "solved"), true},
		{"the scene is not open", errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_NO_OPEN_SCENE, "no scene"), true},
		{"the thing is not found", connect.NewError(connect.CodeNotFound, errors.New("group check not found")), true},
		{"the character answered the check already", errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_ALREADY_ANSWERED, "answered"), false},
		{"the character is not in the check", errContest(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_IN_GROUP_CHECK, "not in"), false},
		{"no attempt is left", errScene(playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_ALREADY_ROLLED, "no attempt"), false},
		{"another question is pending", resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING, "pending"), false},
		{"a failed precondition with no reason", connect.NewError(connect.CodeFailedPrecondition, errors.New("no")), false},
		{"a bad request", connect.NewError(connect.CodeInvalidArgument, errors.New("bad")), false},
		{"a database error", connect.NewError(connect.CodeInternal, errors.New("db")), false},
		{"not a connect error", errors.New("plain"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := heldClosed(tc.err); got != tc.want {
				t.Errorf("heldClosed(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
