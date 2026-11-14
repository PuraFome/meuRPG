package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The READIED window against the rest of the table: the windows an attack roll opens, the actions Ready refuses because a
// reaction would do nothing with them, the undo of the fire, a readied move without a map, and who may answer.

// The goblin's readied attack on Toren waits for the windows any attack roll opens before the d20: a bard's Cutting Words
// and a shield-bearer's Protection. The READIED window stays open and the row waits until they are answered; then the
// attack rolls as the goblin's reaction and the row is fired.
func TestFireReadiedAttackWaitsForThePreRollWindows(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		fight func(t *testing.T) (*armed, *playv1.Encounter)
		kind  playv1.ReactionKind
	}{
		{"Cutting Words", func(t *testing.T) (*armed, *playv1.Encounter) {
			t.Helper()
			return reactorFight(t, loreBard)
		}, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS},
		{"Protection", func(t *testing.T) (*armed, *playv1.Encounter) {
			t.Helper()
			return protectionFight(t, protector())
		}, playv1.ReactionKind_REACTION_KIND_PROTECTION},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, e := c.fight(t) // the goblin's turn
			a.readyGoblinAttack(t, e)
			e = a.mustEndTurn(t, a.master, e)
			w := a.mustFire(t, e, "Goblin")
			res := a.mustAnswer(t, a.master, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
				useTyped(15)(r)
				r.CreatureIds = []string{a.id(t, "Toren")}
			})
			if res.GetResult().GetUsed() {
				t.Errorf("the result says used while the attack waits: %v", res.GetResult())
			}
			pre := a.windowOf(t, a.ana, c.kind)
			if pre == nil {
				t.Fatalf("Reator has no %v window before the readied attack's roll", c.kind)
			}
			if got := a.readiedStatus(t, "Goblin"); got != readiedWaiting || !a.readiedOpen(t, "Goblin") || byLabel(t, a.get(t, a.master), "Goblin").GetReactionUsed() {
				t.Errorf("while the roll waits: rows %q, READIED window open %v", got, a.readiedOpen(t, "Goblin"))
			}
			a.mustAnswer(t, a.ana, e, pre.GetId(), passAnswer)
			goblin := byLabel(t, a.get(t, a.master), "Goblin")
			if got := a.readiedStatus(t, "Goblin"); got != readiedFired || a.readiedOpen(t, "Goblin") || !goblin.GetReactionUsed() || goblin.GetReactionKind() != kindReadied {
				t.Errorf("after the answer: rows %q, window open %v, reaction %v (%v)", got, a.readiedOpen(t, "Goblin"), goblin.GetReactionUsed(), goblin.GetReactionKind())
			}
			if en := a.lastAttackEntry(t, e); !en.GetAsReaction() || en.GetActorLabel() != "Goblin" {
				t.Errorf("the attack line = %v, want the goblin's reaction", en)
			}
			if n := len(a.firedLines(t, a.master, e)); n != 1 {
				t.Errorf("fired lines = %d", n)
			}
		})
	}
}

// Ready refuses the standard actions a reaction would take for nothing: Hide and Help have their own calls, and Dash and
// Disengage last only for the readier's own turn, which no reaction reaches. Search is held as any other.
func TestReadyRefusesTheActionsAReactionCannotUse(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	other := func(key string) *playv1.ReadyActionRequest {
		r := a.readyAttack(t, e)
		r.Kind, r.ActionKey, r.TargetCombatantId = playv1.ReadiedKind_READIED_KIND_OTHER, key, ""
		return r
	}
	for _, key := range []string{actionHide, actionHelp, actionDash, actionDisengage} {
		_, err := a.ready(t, a.caio, other(key))
		wantBlockedBy(t, "Ready("+key+")", err, blockedReadyAction)
	}
	a.mustReady(t, a.caio, other("standard:search"))
}

// The fire is undone while its window was not answered: the window goes, the row waits, and no line is written or taken.
// Once answered, the answer is undone first (the pass), then the fire.
func TestTheFireIsUndoneUntilItsWindowIsAnswered(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	lines := len(logEntries(a.log(t, a.master, e)))
	a.mustFire(t, e, "Toren")
	a.mustUndo(t, e)
	if n := len(a.windows(t, a.master)); n != 0 || a.readiedStatus(t, "Toren") != readiedWaiting {
		t.Fatalf("after the undo of the fire: %d windows, rows %q", n, a.readiedStatus(t, "Toren"))
	}
	if n := len(logEntries(a.log(t, a.master, e))); n != lines {
		t.Errorf("log lines after the fire and its undo = %d, want %d", n, lines)
	}
	w := a.mustFire(t, e, "Toren") // fired again
	a.mustAnswer(t, a.caio, e, w.GetId(), passAnswer)
	a.mustUndo(t, e) // the pass
	if !a.readiedOpen(t, "Toren") {
		t.Fatal("the undo of the pass did not open the window again")
	}
	a.mustUndo(t, e) // the fire
	if n := len(a.windows(t, a.master)); n != 0 || a.readiedStatus(t, "Toren") != readiedWaiting || len(a.firedLines(t, a.master, e)) != 0 {
		t.Errorf("after both undos: %d windows, rows %q", n, a.readiedStatus(t, "Toren"))
	}
	a.mustFire(t, e, "Toren")
}

// A combat without a map has no squares to walk: Ready refuses a move there.
func TestReadyRefusesAMoveWithoutAMap(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		theatre:  true,
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
	})
	r := a.readyAttack(t, e)
	r.Kind, r.ActionKey, r.TargetCombatantId = playv1.ReadiedKind_READIED_KIND_MOVE, "", ""
	_, err := a.ready(t, a.caio, r)
	wantBlockedBy(t, "a readied move without a map", err, blockedReadyAction)
}

// Who is not the reactor reads a READIED window as a window that does not exist, whatever it holds: through
// AnswerReaction, and through RollAttack's catch_window_id.
func TestAReadiedWindowIsUnknownToWhoIsNotItsReactor(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyMove(t)
	w := a.mustFire(t, e, "Toren")
	_, unknown := a.answerReaction(t, a.ana, e, newKey(), useTyped(10))
	_, got := a.answerReaction(t, a.ana, e, w.GetId(), useTyped(10))
	if u, g := refusalOf(t, unknown), refusalOf(t, got); u != g {
		t.Errorf("Ana's answer to Toren's window = %v, to no window = %v", g, u)
	}

	b := newArmed(t)
	f := b.readyThenPass(t, nil) // Pensantus acts
	bw := b.mustFire(t, f, "Toren")
	catch := func(id string) func(*playv1.RollAttackRequest) {
		return func(r *playv1.RollAttackRequest) { d20(10)(r); r.CatchWindowId = id }
	}
	_, unknown = b.attack(t, b.ana, f, "Pensantus", fireBolt, "Goblin", catch(newKey()))
	_, got = b.attack(t, b.ana, f, "Pensantus", fireBolt, "Goblin", catch(bw.GetId()))
	if u, g := refusalOf(t, unknown), refusalOf(t, got); u != g {
		t.Errorf("Pensantus's attack on Toren's window = %v, on no window = %v", g, u)
	}
}
