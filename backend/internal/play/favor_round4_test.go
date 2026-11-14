package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// --- RN-10: the refusal of a worse roll_mode is judged on what the roller reads ---

// askedOutcome is the answer to a spend that asks a roll_mode worse than the one the Inspiration gives: the error code and whether the
// Inspiration went.
type askedOutcome struct {
	code  connect.Code
	err   bool
	spent bool
}

// A request that spends and asks a mode worse than the one the roller reads is refused (the roll would waste the Inspiration); a source
// the roller may not read, advantage or disadvantage, never changes the answer, so the refusal cannot tell it apart.
func TestFavorAgainstIsJudgedOnWhatTheRollerReads(t *testing.T) {
	t.Parallel()
	for _, p := range favorPaths {
		for _, asked := range []playv1.RollMode{playv1.RollMode_ROLL_MODE_NORMAL, playv1.RollMode_ROLL_MODE_DISADVANTAGE} {
			run := func(t *testing.T, hidden string) askedOutcome {
				t.Helper()
				f := newFavorFight(t)
				p.ready(t, f)
				u, who := p.roller(f)
				f.give(t, who)
				if hidden != "" {
					f.hiddenEffect(t, p.actor, false, "["+hidden+"]")
				}
				f.h.roller.queue(4, 17)
				_, err := p.send(t, f, u, newKey(), true, func(r any) {
					switch r := r.(type) {
					case *playv1.RollAttackRequest:
						r.RollMode = asked
					case *playv1.CastSpellRequest:
						r.RollMode = asked
					}
				})
				out := askedOutcome{err: err != nil, spent: !f.favorHolds(t)}
				if err != nil {
					out.code = connect.CodeOf(err)
				}
				return out
			}
			for name, hidden := range map[string]string{"a hidden advantage": attackAdv, "a hidden disadvantage": attackDis} {
				t.Run(p.name+", "+asked.String()+", "+name, func(t *testing.T) {
					t.Parallel()
					base, got := run(t, ""), run(t, hidden)
					if base.code != connect.CodeInvalidArgument || base.spent {
						t.Fatalf("the request with no hidden source = %+v, want INVALID_ARGUMENT and the Inspiration kept", base)
					}
					if got != base {
						t.Errorf("the request with %s = %+v, want the answer of the request with none (%+v)", name, got, base)
					}
				})
			}
		}
	}
}

// The control: a worse mode than the one the roller reads is still refused, and the Inspiration stays.
func TestFavorAgainstStillRefusesAWorseModeThanAVisibleDisadvantage(t *testing.T) {
	t.Parallel()
	for _, p := range favorPaths {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			f := newFavorFight(t)
			p.ready(t, f)
			u, who := p.roller(f)
			f.give(t, who)
			f.hiddenEffect(t, p.actor, true, "["+attackDis+"]") // read by the roller: the Inspiration makes it a straight roll
			_, err := p.send(t, f, u, newKey(), true, func(r any) {
				switch r := r.(type) {
				case *playv1.RollAttackRequest:
					r.RollMode = playv1.RollMode_ROLL_MODE_DISADVANTAGE
				case *playv1.CastSpellRequest:
					r.RollMode = playv1.RollMode_ROLL_MODE_DISADVANTAGE
				}
			})
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("the request that asks a disadvantage over a visible one = %v, want INVALID_ARGUMENT", err)
			}
			if !f.favorHolds(t) {
				t.Error("the refusal took the Inspiração")
			}
		})
	}
}
