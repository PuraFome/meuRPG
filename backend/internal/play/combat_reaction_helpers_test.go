package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The helpers of the reaction window tests (PM-04).

// windows are the reaction windows u reads in the combat now.
func (a *armed) windows(t *testing.T, u *user) []*playv1.ReactionWindow {
	t.Helper()
	return a.get(t, u).GetReactionWindows()
}

// windowOf is the first open window of the kind u reads, or nil.
func (a *armed) windowOf(t *testing.T, u *user, kind playv1.ReactionKind) *playv1.ReactionWindow {
	t.Helper()
	for _, w := range a.windows(t, u) {
		if w.GetKind() == kind {
			return w
		}
	}
	return nil
}

// answerReaction calls AnswerReaction as u; set fills the answer.
func (a *armed) answerReaction(t *testing.T, u *user, e *playv1.Encounter, windowID string, set func(*playv1.AnswerReactionRequest)) (*playv1.AnswerReactionResponse, error) {
	t.Helper()
	req := &playv1.AnswerReactionRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: windowID, IdempotencyKey: newKey()}
	if set != nil {
		set(req)
	}
	res, err := u.combat.AnswerReaction(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func passAnswer(r *playv1.AnswerReactionRequest) {
	r.Answer = playv1.ReactionChoice_REACTION_CHOICE_PASS
}

func useAnswer(slotLevel int32) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE
		if slotLevel > 0 {
			r.Slot = slotOfLevel(slotLevel)
		}
	}
}

// mustAnswer is answerReaction that fails the test on an error.
func (a *armed) mustAnswer(t *testing.T, u *user, e *playv1.Encounter, windowID string, set func(*playv1.AnswerReactionRequest)) *playv1.AnswerReactionResponse {
	t.Helper()
	res, err := a.answerReaction(t, u, e, windowID, set)
	if err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	return res
}

// resolveConcentration calls ResolveConcentrationSave as u.
func (a *armed) resolveConcentration(t *testing.T, u *user, e *playv1.Encounter, windowID string, set func(*playv1.ResolveConcentrationSaveRequest)) (*playv1.ResolveConcentrationSaveResponse, error) {
	t.Helper()
	req := &playv1.ResolveConcentrationSaveRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), WindowId: windowID, IdempotencyKey: newKey()}
	set(req)
	res, err := u.combat.ResolveConcentrationSave(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func keepConcentration(r *playv1.ResolveConcentrationSaveRequest) {
	r.Roll = &playv1.ResolveConcentrationSaveRequest_Keep{Keep: true}
}

func concentrationFace(face int32) func(*playv1.ResolveConcentrationSaveRequest) {
	return func(r *playv1.ResolveConcentrationSaveRequest) {
		r.Roll = &playv1.ResolveConcentrationSaveRequest_D20Face{D20Face: face}
	}
}

// keepAllConcentrations has the master keep every concentration an open window asks a
// save for: the tests of other things do not need the save.
func (a *armed) keepAllConcentrations(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	for range 4 {
		w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
		if w == nil {
			return
		}
		if _, err := a.resolveConcentration(t, a.master, e, w.GetId(), keepConcentration); err != nil {
			t.Fatalf("ResolveConcentrationSave(keep) error = %v", err)
		}
	}
}

// reactionRPCs are the methods of the reaction window: its authorization matrix is
// TestReactionWindowAnswersAreTheReactorsAndTheMasters (combat_reaction_window_test.go).
var reactionRPCs = []string{"AnswerReaction", "ResolveConcentrationSave"}
