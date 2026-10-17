package play

import (
	"testing"
	"uuid"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The CONTEST window follows its contest row (syncContests, run after every change of a
// combat) and RevealHider's refusals. These freeze what the refactor of both must keep.

// contestWindowOf is the CONTEST window an encounter reads, nil when there is none.
func contestWindowOf(e *playv1.Encounter) *playv1.ReactionWindow {
	for _, w := range e.GetReactionWindows() {
		if w.GetKind() == playv1.ReactionKind_REACTION_KIND_CONTEST {
			return w
		}
	}
	return nil
}

// TestSyncContestsKeepsTheWindowWhileTheReactorStaysAndReplacesItWhenItChanges: the Hobgoblin
// shoves Brisa. Her window survives an unrelated change with the same id (the reactor did not
// change); once she answers and the Hobgoblin wins, the shover is an NPC, so the window is
// replaced by one the master holds; the master's choice closes it.
func TestSyncContestsKeepsTheWindowWhileTheReactorStaysAndReplacesItWhenItChanges(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.advance(t, c.hob)
	c.h.roller.queue(17)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), purposeShoveP
	})
	contestID := started.GetContest().GetId()

	first := contestWindowOf(c.get(t, a.master))
	if first == nil || first.GetReactorId() != c.id(t, "Brisa") || first.GetGroupId() != contestID {
		t.Fatalf("the window = %v, want Brisa's, grouped by the contest", first)
	}
	if got := c.get(t, a.caio).GetReactionWait().GetTitlePt(); got != "Esperando Brisa" {
		t.Errorf("another player reads %q, want %q", got, "Esperando Brisa")
	}

	// An unrelated change settles the combat again: the window is the same row.
	if _, err := c.moveTo(t, a.master, "Goblin", 9, 4); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	kept := contestWindowOf(c.get(t, a.master))
	if kept == nil || kept.GetId() != first.GetId() {
		t.Fatalf("the window after an unrelated change = %v, want the same %s", kept, first.GetId())
	}

	// Brisa loses the contest: the Hobgoblin (an NPC) now owes the choice, so the master holds it.
	c.mustRespond(t, a.bia, contestID, playv1.ContestSkill_CONTEST_SKILL_ACROBATICS, 1)
	next := contestWindowOf(c.get(t, a.master))
	if next == nil || next.GetId() == first.GetId() || next.GetReactorId() != "" || next.GetGroupId() != contestID || !next.GetForYou() {
		t.Fatalf("the window after the answer = %v, want a new one with no reactor, the master's", next)
	}
	if got := c.get(t, a.caio).GetReactionWait().GetTitlePt(); got != "Esperando o mestre" {
		t.Errorf("another player reads %q, want %q", got, "Esperando o mestre")
	}
	if w := contestWindowOf(c.get(t, a.bia)); w != nil {
		t.Errorf("Brisa still reads a window to answer: %v", w)
	}

	c.mustShove(t, a.master, contestID, playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE)
	if w := contestWindowOf(c.get(t, a.master)); w != nil || c.get(t, a.caio).GetReactionWait() != nil {
		t.Errorf("the window outlives the shove: %v", w)
	}
}

// TestRevealHiderRefusalsAreFrozen: every refusal of RevealHider, with its code and message.
func TestRevealHiderRefusalsAreFrozen(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	toren := c.id(t, "Toren")
	reveal := func(edit func(*playv1.RevealHiderRequest)) error {
		req := &playv1.RevealHiderRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: toren}
		edit(req)
		_, err := a.master.contests.RevealHider(t.Context(), connect.NewRequest(req))
		return err
	}
	tests := []struct {
		name string
		edit func(*playv1.RevealHiderRequest)
		code connect.Code
		msg  string
	}{
		{"an idempotency key that is not a UUID", func(r *playv1.RevealHiderRequest) { r.IdempotencyKey = "x" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", func(r *playv1.RevealHiderRequest) { r.EncounterId = "x" }, connect.CodeNotFound, "encounter not found"},
		{"a combatant that is not a UUID", func(r *playv1.RevealHiderRequest) { r.CombatantId = "x" }, connect.CodeNotFound, "combatant not found"},
		{"an encounter that does not exist", func(r *playv1.RevealHiderRequest) { r.EncounterId = uuid.New().String() }, connect.CodeNotFound, "encounter not found"},
		{"a combatant that is not in the combat", func(r *playv1.RevealHiderRequest) { r.CombatantId = uuid.New().String() }, connect.CodeNotFound, "combatant not found"},
	}
	for _, tt := range tests {
		wantRefusal(t, tt.name, reveal(tt.edit), tt.code, tt.msg)
	}

	// A combatant that is not hidden is the ContestBlocked refusal, with its message.
	err := reveal(func(*playv1.RevealHiderRequest) {})
	wantRefusal(t, "a combatant that is not hidden", err, connect.CodeFailedPrecondition, "the combatant is not hidden")
	wantContestBlocked(t, "a combatant that is not hidden", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_HIDDEN)

	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	wantRefusal(t, "a combat that ended", reveal(func(*playv1.RevealHiderRequest) {}), connect.CodeFailedPrecondition, "the combat has ended")
}
