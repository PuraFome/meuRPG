package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// --- RN-10: the contests and the Hide judge ADDS_NOTHING on what the roller reads ---

// hideOnToren puts an effect on Toren in the combat of the path: one only the master reads (visible off), or one everybody reads, with
// the modifiers (a JSON list).
func (e *checkEnv) hideOnToren(t *testing.T, visible bool, mods string) {
	t.Helper()
	e.execSQL(t, `INSERT INTO combatant_states (encounter_id, combatant_id, kind, started_round, created_at, group_id, source_key, source_kind, duration_kind, modifiers, player_visible)
		SELECT encounter_id, id, 'effect', 1, now(), gen_random_uuid(), 'spell:bless', 'master', 'until_dismissed', $3::jsonb, $4 FROM combatants WHERE encounter_id = $1 AND label = $2`,
		e.fight.e.GetId(), "Toren", mods, visible)
}

// checkOutcome is what the spend on a check of the combat answered, as the player reads it, for the comparison with the roll that had no
// hidden source: the refusal or the spend, and what was written.
type checkOutcome struct {
	err      string
	spent    bool
	stored   bool
	hasFavor bool
	steps    int64
}

// The contest start, the contest answer and the Hide answer the spend with a hidden advantage, or a hidden advantage and disadvantage that
// cancel, as they answer it when there is no hidden source: the spend goes, and the player is not told by a refusal that something they
// cannot read is there. The same advantage the roller reads is still refused.
func TestFavorAddsNothingIsJudgedOnWhatTheRollerReadsOnACombatCheck(t *testing.T) {
	t.Parallel()
	for _, p := range []struct {
		name string
		mk   func(*testing.T) *checkEnv
	}{{"a contest start", newContestStartEnv}, {"a contest answer", newContestAnswerEnv}, {"a hide", newHideEnv}} {
		for name, hc := range hiddenCases(checkAdvAll, checkDisAll) {
			t.Run(p.name+", "+name, func(t *testing.T) {
				t.Parallel()
				run := func(hidden bool) (checkOutcome, int) {
					e := p.mk(t)
					e.give(t, e.who)
					if hidden {
						e.hideOnToren(t, false, hc.mods)
					}
					e.h.roller.queue(4, 17)
					seq := e.seq(t)
					got, err := e.send(t, newKey(), true)
					out := checkOutcome{spent: !e.favorHolds(t), hasFavor: got.hasFavor(), steps: e.seq(t) - seq}
					if err != nil {
						out.err = err.Error()
					} else {
						out.stored = e.stored(t)
					}
					return out, len(got.faces)
				}
				base, baseFaces := run(false)
				hidden, faces := run(true)
				if base.err != "" || !base.spent || !base.stored || baseFaces != 2 {
					t.Fatalf("the check with no hidden source = %+v with %d dice, want the spend to go and the pair of dice", base, baseFaces)
				}
				if hidden != base || faces != hc.faces {
					t.Errorf("the check with %s = %+v with %d dice, want the answer of the check with none (%+v) and %d d20", name, hidden, faces, base, hc.faces)
				}
			})
		}
		t.Run(p.name+", a visible advantage", func(t *testing.T) {
			t.Parallel()
			e := p.mk(t)
			e.give(t, e.who)
			e.hideOnToren(t, true, "["+checkAdvAll+"]")
			_, err := e.send(t, newKey(), true)
			wantFavorBlocked(t, "the spend with an advantage the roller reads", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING)
			if !e.favorHolds(t) {
				t.Error("the refusal took the Inspiração")
			}
		})
	}
}

// hideConditionOnToren gives Toren a condition through an effect only the master reads (player_visible off), as the master's hiding of it
// is made: the combatant carries it (as the effect set it) and the effect's state names it, which hiddenConditionsOf reads.
func (e *checkEnv) hideConditionOnToren(t *testing.T, condition string) {
	t.Helper()
	e.execSQL(t, `INSERT INTO combatant_states (encounter_id, combatant_id, kind, started_round, created_at, group_id, source_key, source_kind, duration_kind, condition_keys, modifiers, player_visible)
		SELECT encounter_id, id, 'effect', 1, now(), gen_random_uuid(), 'spell:hidden', 'master', 'until_dismissed', ARRAY[$3]::text[], '[]'::jsonb, false FROM combatants WHERE encounter_id = $1 AND label = $2`,
		e.fight.e.GetId(), "Toren", condition)
	e.execSQL(t, `UPDATE combatants SET conditions = ARRAY[$3]::text[], effect_conditions = ARRAY[$3]::text[] WHERE encounter_id = $1 AND label = $2`,
		e.fight.e.GetId(), "Toren", condition)
}

// A condition the master hides (condition:poisoned, Envenenado: a disadvantage on ability checks) is a source the roller may not read, as a hidden effect is: the
// contest and the Hide answer the spend as they do with no hidden source, and the spend goes (the roll is straight, one d20).
func TestFavorAddsNothingIsJudgedOnAHiddenConditionOnACombatCheck(t *testing.T) {
	t.Parallel()
	for _, p := range []struct {
		name string
		mk   func(*testing.T) *checkEnv
	}{{"a contest start", newContestStartEnv}, {"a contest answer", newContestAnswerEnv}, {"a hide", newHideEnv}} {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			run := func(hidden bool) (checkOutcome, int) {
				e := p.mk(t)
				e.give(t, e.who)
				if hidden {
					e.hideConditionOnToren(t, "condition:poisoned")
				}
				e.h.roller.queue(4, 17)
				seq := e.seq(t)
				got, err := e.send(t, newKey(), true)
				out := checkOutcome{spent: !e.favorHolds(t), hasFavor: got.hasFavor(), steps: e.seq(t) - seq}
				if err != nil {
					out.err = err.Error()
				} else {
					out.stored = e.stored(t)
				}
				return out, len(got.faces)
			}
			base, baseFaces := run(false)
			hidden, faces := run(true)
			if base.err != "" || !base.spent || !base.stored || baseFaces != 2 {
				t.Fatalf("the check with no hidden source = %+v with %d dice, want the spend to go and the pair of dice", base, baseFaces)
			}
			if hidden != base || faces != 1 {
				t.Errorf("the check with a hidden condition = %+v with %d dice, want the answer of the check with none (%+v) and 1 d20", hidden, faces, base)
			}
		})
	}
}
