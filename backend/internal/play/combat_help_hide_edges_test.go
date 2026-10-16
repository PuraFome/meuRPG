package play

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Edges of Help, ClearHelp, Hide and ResolveHide that the tests of combat_help_test.go and
// combat_hide_test.go leave out: the exact refusal messages, the feature form of Hide, the
// master's bypasses. They freeze the behaviour before the functions are split into phases.

// wantHelpHideMessage fails unless err is a connect error of this code whose message is exactly msg.
func wantHelpHideMessage(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s error = %v, want a connect error", call, err)
	}
	if ce.Code() != code || ce.Message() != msg {
		t.Errorf("%s = %v %q, want %v %q", call, ce.Code(), ce.Message(), code, msg)
	}
}

// TestHelpAndClearHelpRefusalMessagesAreFrozen freezes the message of each refusal of the checks
// before the transaction and of the ones inside it.
func TestHelpAndClearHelpRefusalMessagesAreFrozen(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, "Brisa")

	long := "skill:" + strings.Repeat("x", 95)  // 101 bytes
	exact := "skill:" + strings.Repeat("x", 94) // 100 bytes
	for name, tc := range map[string]struct {
		edit func(*playv1.HelpRequest)
		code connect.Code
		msg  string
	}{
		"no kind": {func(r *playv1.HelpRequest) { r.AllyId = c.id(t, "Toren") }, connect.CodeInvalidArgument, "kind must be CHECK or ATTACK"},
		"a check with no task": {func(r *playv1.HelpRequest) { r.Kind, r.AllyId = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Toren") },
			connect.CodeInvalidArgument, "task_key must name the check of the task"},
		"a check with a task of 101 bytes": {func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Toren"), long
		}, connect.CodeInvalidArgument, "task_key must name the check of the task"},
		"a check with a target": {func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey, r.TargetId = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Toren"), "skill:stealth", c.id(t, c.hob)
		}, connect.CodeInvalidArgument, "target_id is for a Help with an attack"},
		"an attack with a task": {func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey, r.TargetId = playv1.HelpKind_HELP_KIND_ATTACK, c.id(t, "Toren"), "skill:stealth", c.id(t, c.hob)
		}, connect.CodeInvalidArgument, "task_key is for a Help with a check"},
		"a task of 100 bytes that is no check of the SRD": {func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Toren"), exact
		}, connect.CodeFailedPrecondition, "the task is not a check of the SRD"},
		"a saving throw": {func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Toren"), "save:dex"
		}, connect.CodeFailedPrecondition, "the task is not a skill or an ability check"},
		"oneself": {func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Brisa"), "skill:stealth"
		}, connect.CodeFailedPrecondition, "that is not an ally the combatant can help"},
		"an attack on an ally": {func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TargetId = playv1.HelpKind_HELP_KIND_ATTACK, c.id(t, "Toren"), c.id(t, "Pensantus")
		}, connect.CodeFailedPrecondition, "the target is not a creature the ally can attack"},
	} {
		_, err := c.help(t, a.bia, "Brisa", tc.edit)
		wantHelpHideMessage(t, name, err, tc.code, tc.msg)
	}

	// An unknown Help and a Help already cleared.
	wantHelpHideMessage(t, "clearing an unknown Help", c.clearHelp(t, a.master, newKey()), connect.CodeNotFound, "help not found")
	given := helped(t)(c.helpCheck(t, a.bia, "Brisa", "Toren", "skill:stealth"))
	if err := c.clearHelp(t, a.master, given.GetId()); err != nil {
		t.Fatalf("ClearHelp() error = %v", err)
	}
	wantContestBlocked(t, "clearing it twice", c.clearHelp(t, a.master, given.GetId()), playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AWAITING)
	wantHelpHideMessage(t, "clearing a Help that is no UUID", c.clearHelp(t, a.master, "nope"), connect.CodeNotFound, "help not found")
}

// TestHelpOfAHelperWithNoNumberForTheTaskIsRefused: the helper has to be able to attempt the task
// alone (the Goblin's basic sheet has no number for it), and the master helps with any combatant
// whose action is spent.
func TestHelpOfAHelperWithNoNumberForTheTaskIsRefused(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, c.hob)
	c.advance(t, "Goblin")

	_, err := c.helpCheck(t, a.master, "Goblin", c.hob, "skill:stealth")
	wantContestBlocked(t, "a helper with no number", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_TASK_NOT_AVAILABLE)
	wantHelpHideMessage(t, "a helper with no number", err, connect.CodeFailedPrecondition, "the helper could not attempt the task alone")

	// The master's Help spends the action but, as the master, is not stopped by a spent action.
	helped(t)(c.helpAttack(t, a.master, "Goblin", c.hob, "Toren"))
	helped(t)(c.helpAttack(t, a.master, "Goblin", c.hob, "Brisa"))
	if n := len(c.state(t, a.master).GetHelps()); n != 2 {
		t.Errorf("the master reads %d helps, want 2", n)
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		// Finding: the players read an enemy's Help aimed at the party (a Help is a source of advantage every
		// player reads) even though the helper is the master's; frozen as it is.
		if n, want := len(c.state(t, u).GetHelps()), 2; n != want {
			t.Errorf("%s reads %d helps of the enemies, want %d", who, n, want)
		}
	}
}

// TestHideFeatureFormRefusalsAreFrozen: a Hide by a feature (Cunning Action) spends the bonus
// action; the key of a feature the combatant lacks or of another action of the feature is refused,
// and the master hides again with a spent economy.
func TestHideFeatureFormRefusalsAreFrozen(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})

	c.mustHide(t, a.caio, "Toren", cunningHide, 12)
	_, err := c.hide(t, a.caio, "Toren", cunningHide, 12)
	wantBlockedBy(t, "a second Cunning Action", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_BONUS_ACTION_USED)
	wantHelpHideMessage(t, "a second Cunning Action", err, connect.CodeFailedPrecondition, "the bonus action of this turn is used")

	_, err = c.hide(t, a.caio, "Toren", "feature:cunning-action:dash", 12)
	wantContestBlocked(t, "another action of the feature", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AVAILABLE)
	wantHelpHideMessage(t, "another action of the feature", err, connect.CodeFailedPrecondition, "the action is not a Hide")

	_, err = c.hide(t, a.caio, "Toren", "standard:dash", 12)
	wantHelpHideMessage(t, "the key of the Dash", err, connect.CodeFailedPrecondition, "the combatant has no such action")

	c.advance(t, "Brisa")
	_, err = c.hide(t, a.bia, "Brisa", cunningHide, 12)
	wantHelpHideMessage(t, "Cunning Action of a fighter", err, connect.CodeFailedPrecondition, "the combatant has no such action")
	if a.mustOptions(t, a.bia, c.refresh(t), "Brisa").GetOptions().GetEconomy().GetAction().GetUsed() {
		t.Error("a refused Hide spent the action")
	}

	// The master hides the Hobgoblin twice in one turn: the economy does not stop him.
	c.advance(t, "Pensantus")
	c.advance(t, c.hob)
	c.mustHide(t, a.master, c.hob, "", 12)
	c.mustHide(t, a.master, c.hob, "", 12)
	if n := len(c.state(t, a.master).GetHideAttempts()); n != 3 {
		t.Errorf("the master reads %d attempts, want 3", n)
	}
}

// TestResolveHideRefusalMessagesAreFrozen: the checks of the decision before and inside the
// transaction, with their messages.
func TestResolveHideRefusalMessagesAreFrozen(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	tried := c.mustHide(t, a.caio, "Toren", "", 12)

	_, err := c.resolveHide(t, a.master, tried.GetId(), func(r *playv1.ResolveHideRequest) { r.Refuse, r.Refusal = true, strings.Repeat("x", 121) })
	wantHelpHideMessage(t, "a refusal of 121 characters", err, connect.CodeInvalidArgument, "the reason must be 1 to 120 characters on one line")
	_, err = c.resolveHide(t, a.master, tried.GetId(), func(r *playv1.ResolveHideRequest) { r.Refuse, r.Refusal = true, "a\nb" })
	wantHelpHideMessage(t, "a refusal of two lines", err, connect.CodeInvalidArgument, "the reason must be 1 to 120 characters on one line")
	_, err = c.resolveHide(t, a.master, tried.GetId(), func(r *playv1.ResolveHideRequest) { r.SeesClearlyIds = []string{"nope"} })
	wantCode(t, "a creature id that is no UUID", err, connect.CodeNotFound)
	_, err = c.resolveHide(t, a.master, newKey(), nil)
	wantHelpHideMessage(t, "an unknown attempt", err, connect.CodeNotFound, "hide attempt not found")
	_, err = c.resolveHide(t, a.master, "nope", nil)
	wantCode(t, "an attempt id that is no UUID", err, connect.CodeNotFound)
	_, err = c.resolveHide(t, a.caio, tried.GetId(), nil)
	wantCode(t, "a player deciding", err, connect.CodePermissionDenied)

	// A refusal of 120 characters is kept, with the creatures named twice counted once.
	hob := c.id(t, c.hob)
	got := c.mustResolveHide(t, tried.GetId(), func(r *playv1.ResolveHideRequest) {
		r.SeesClearlyIds = []string{hob, hob}
		r.Refuse, r.Refusal = true, "  "+strings.Repeat("y", 120)+"  "
	})
	if got.GetStatus() != playv1.HideAttemptStatus_HIDE_ATTEMPT_STATUS_REFUSED || got.GetRefusal() != strings.Repeat("y", 120) {
		t.Errorf("the refused attempt = %v, want REFUSED with the trimmed reason", got)
	}
	_, err = c.resolveHide(t, a.master, tried.GetId(), nil)
	wantContestBlocked(t, "deciding twice", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_HIDE_NOT_PENDING)
	wantHelpHideMessage(t, "deciding twice", err, connect.CodeFailedPrecondition, "the hide attempt is answered already")
}
