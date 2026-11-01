package play

import (
	"testing"
	"time"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A surprised creature takes no legendary action until its first turn ends (SRD 5.1, "Surprise"; legendarySpend):
// the text option, the attack and the move are refused with SURPRISED and spend nothing. These tests need the
// database (MEURPG_TEST_DATABASE_URL).

// surprisedFight is a combat with the creature last in the order, marked surprised before the combat begins, so
// that Toren has the first turn and the creature has not had its own yet. It answers the creature's label.
func surprisedFight(t *testing.T, theatre bool, key, name string, surprised bool) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a := newArmed(t)
	e := a.monsterSetup(t, theatre)
	a.h.roller.queue(1) // the creature's initiative: after the party
	res := a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = key, 1, name, new(false)
	})
	id := res.GetCombatantIds()[0]
	if surprised {
		if err := playdb.New(a.h.pool).UpsertSurprised(t.Context(), playdb.UpsertSurprisedParams{CombatantID: id, EncounterID: e.GetId(), CreatedAt: time.Now()}); err != nil {
			t.Fatalf("mark %s surprised: %v", name, err)
		}
	}
	e = a.begin(t, a.get(t, a.master))
	label := byLabelID(t, e, id)
	if !theatre {
		a.castStand(t, map[string][2]int32{label: {10, 5}, "Toren": {8, 5}, "Goblin": {7, 5}, "Pensantus": {10, 9}, "Brisa": {1, 1}})
	}
	if current(e) != "Toren" {
		t.Fatalf("on turn = %s, want Toren (the creature is last)", current(e))
	}
	return a, e, label
}

// The three ways to spend are refused while the creature is surprised, and nothing is spent; the same calls
// pass once its surprise is gone (the positive control).
func TestLegendarySpend_ASurprisedCreatureCannotSpend(t *testing.T) {
	t.Parallel()
	a, e, dragon := surprisedFight(t, true, adultRedDragon, "Draco", true)
	_, err := a.spendLegendary(t, a.master, e, dragon, "detect")
	wantContestBlocked(t, "a text option while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	_, err = a.legendaryAttack(t, e, dragon, "tail-attack", dragonTail, "Toren", 20)
	wantContestBlocked(t, "a legendary attack while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	if got := a.spentOn(t, dragon); got != 0 {
		t.Errorf("the refusals left spent = %d, want 0", got)
	}

	b, f, free := surprisedFight(t, true, adultRedDragon, "Draco", false)
	if _, err := b.spendLegendary(t, b.master, f, free, "detect"); err != nil {
		t.Errorf("a creature that is not surprised was refused: %v", err)
	}
}

// A legendary move is refused while the creature is surprised too, and spends nothing.
func TestLegendarySpend_ASurprisedCreatureCannotMove(t *testing.T) {
	t.Parallel()
	a, e, vamp := surprisedFight(t, false, vampireKey, "Drácula", true)
	_, err := a.legendaryMove(t, a.master, e, vamp, "move", 10, 6)
	wantContestBlocked(t, "a legendary move while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	if got := a.spentOn(t, vamp); got != 0 {
		t.Errorf("the refusal left spent = %d, want 0", got)
	}
	b, f, free := surprisedFight(t, false, vampireKey, "Drácula", false)
	if _, err := b.legendaryMove(t, b.master, f, free, "move", 10, 6); err != nil {
		t.Errorf("a creature that is not surprised was refused: %v", err)
	}
}
