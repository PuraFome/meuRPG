package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// FireReadied and the READIED window (ready_fire.go, ready_answer.go): the master fires, the readier uses its reaction to
// run the held action through its own path, or lets it pass; what each spends, refuses, writes and undoes.

const (
	kindReadied            = playv1.ReactionKind_REACTION_KIND_READIED
	blockedReactionPending = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_PENDING
)

// fire calls FireReadied as u.
func (a *armed) fire(t *testing.T, u *user, e *playv1.Encounter, id string) (*playv1.Encounter, error) {
	t.Helper()
	res, err := u.ready.FireReadied(t.Context(), connect.NewRequest(&playv1.FireReadiedRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), ReadiedId: id, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetEncounter(), nil
}

// mustFire fires the readied action of the combatant as the master and answers its window as the master reads it.
func (a *armed) mustFire(t *testing.T, e *playv1.Encounter, label string) *playv1.ReactionWindow {
	t.Helper()
	id := byLabel(t, a.get(t, a.master), label).GetReadied().GetId()
	if _, err := a.fire(t, a.master, e, id); err != nil {
		t.Fatalf("FireReadied(%s) error = %v", label, err)
	}
	w := a.windowOf(t, a.master, kindReadied)
	if w == nil || w.GetReactorId() != a.id(t, label) {
		t.Fatalf("the master's READIED window = %v, want %s's", w, label)
	}
	return w
}

// useTyped is "Usar" with a typed d20.
func useTyped(face int32) func(*playv1.AnswerReactionRequest) {
	return func(r *playv1.AnswerReactionRequest) {
		r.Answer = playv1.ReactionChoice_REACTION_CHOICE_USE
		r.Roll = &playv1.AnswerReactionRequest_Typed{Typed: face}
	}
}

// readyThenPass has Toren ready his battleaxe on the goblin and ends his turn: Pensantus acts.
func (a *armed) readyThenPass(t *testing.T, req func(*playv1.ReadyActionRequest)) *playv1.Encounter {
	t.Helper()
	e := a.threeAndAGoblin(t)
	r := a.readyAttack(t, e)
	if req != nil {
		req(r)
	}
	a.mustReady(t, a.caio, r)
	return a.mustEndTurn(t, a.master, e)
}

// firedLines are the READIED_FIRED lines u reads.
func (a *armed) firedLines(t *testing.T, u *user, e *playv1.Encounter) []*playv1.CombatLogEntry {
	t.Helper()
	return a.readiedLines(t, u, e, playv1.CombatLogKind_COMBAT_LOG_KIND_READIED_FIRED)
}

// The master fires Toren's readied battleaxe on Pensantus's turn: Toren's player and the master answer the window, Ana
// reads only the wait, and the turn waits for it. "Usar" is the attack as a reaction: hit or miss, the reaction is spent
// with its kind, the row is fired, and the line says so (the trigger the master's and Toren's player's).
func TestFireReadiedAttackIsAReactionAttack(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		face int32
		hit  bool
	}{{"a hit", 15, true}, {"a miss", 2, false}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := newArmed(t)
			e := a.readyThenPass(t, nil)
			w := a.mustFire(t, e, "Toren")
			if mine := a.windowOf(t, a.caio, kindReadied); mine == nil || !mine.GetForYou() || mine.GetTrigger() != nil || mine.GetPrompt() != nil {
				t.Errorf("Toren's player's window = %v, want his, with no trigger and no prompt", mine)
			}
			if tr := w.GetTrigger(); tr.GetActorLabel() != "Toren" || tr.GetTargetLabel() != "Goblin" || tr.GetActionKey() != battleaxe || tr.GetActionNamePt() == "" {
				t.Errorf("the master's trigger = %v", tr)
			}
			ana := a.get(t, a.ana)
			if len(ana.GetReactionWindows()) != 0 || ana.GetReactionWait().GetTitlePt() != "Esperando a reação de Toren" {
				t.Errorf("Ana reads windows %v and the wait %q", ana.GetReactionWindows(), ana.GetReactionWait().GetTitlePt())
			}
			_, err := a.endTurn(t, a.ana, e, false)
			wantBlockedBy(t, "Pensantus's EndTurn while the window waits", err, blockedReactionPending)

			res := a.mustAnswer(t, a.caio, e, w.GetId(), useTyped(c.face))
			if r := res.GetResult(); r.GetKind() != kindReadied || !r.GetUsed() || r.GetByMaster() {
				t.Errorf("the result = %v", r)
			}
			toren := byLabel(t, a.get(t, a.caio), "Toren")
			if !toren.GetReactionUsed() || toren.GetReactionKind() != kindReadied || toren.GetReadied() != nil {
				t.Errorf("Toren after the use: reaction used %v (%v), readied %v", toren.GetReactionUsed(), toren.GetReactionKind(), toren.GetReadied())
			}
			if got := a.readiedStatus(t, "Toren"); got != readiedFired {
				t.Errorf("rows = %q, want one fired", got)
			}
			if a.windowOf(t, a.master, kindReadied) != nil {
				t.Error("the window is still open")
			}
			if n := a.pendingCount(t); (n == 1) != c.hit {
				t.Errorf("pending damages after the attack = %d, hit %v", n, c.hit)
			}
			var reaction bool
			for _, r := range a.log(t, a.master, e).GetRounds() {
				for _, en := range r.GetEntries() {
					if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetAsReaction() {
						reaction = true
					}
				}
			}
			if !reaction {
				t.Error("the log has no reaction attack")
			}
			for _, u := range []*user{a.caio, a.master} {
				lines := a.firedLines(t, u, e)
				if len(lines) != 1 || lines[0].GetReadiedFired().GetTriggerText() != readyTrigger || lines[0].GetReadiedFired().GetStatus() != playv1.ReadiedStatus_READIED_STATUS_FIRED {
					t.Errorf("%s's fired lines = %v", u.id, lines)
				}
			}
			if lines := a.firedLines(t, a.ana, e); len(lines) != 1 || lines[0].GetReadiedFired() != nil || lines[0].GetActorLabel() != "Toren" {
				t.Errorf("Ana's fired lines = %v, want one with the actor alone", lines)
			}
		})
	}
}
