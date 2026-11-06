package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// appliedDamageOnToren plays a goblin's hit on Toren up to the applied damage, the last action.
func appliedDamageOnToren(t *testing.T, a *armed) (*playv1.Encounter, string) {
	t.Helper()
	e := a.threeAndAGoblin(t)
	a.mustEndTurn(t, a.caio, e)
	a.mustEndTurn(t, a.ana, e)
	a.mustEndTurn(t, a.bia, e) // the goblin
	a.h.roller.queue(15, 3)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Toren", inAppRoll)
	id := hit.GetPendingDamage().GetId()
	a.mustDamage(t, a.master, e, id, inAppDamage)
	if _, err := a.settle(t, a.master, e, id, true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	return e, a.log(t, a.master, e).GetUndoableEventId()
}

// blockedOf reads the EncounterBlocked detail of a refusal.
func blockedOf(t *testing.T, err error) *playv1.EncounterBlocked {
	t.Helper()
	var ce *connect.Error
	if !asConnectError(err, &ce) {
		t.Fatalf("error = %v, want a connect error", err)
	}
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if b, ok := v.(*playv1.EncounterBlocked); ok {
				return b
			}
		}
	}
	t.Fatalf("error %v has no EncounterBlocked detail", err)
	return nil
}

// TestUndoIsRefusedWhenACharactersHitPointsChangedAfterTheAction: a write that leaves no event to close the
// chain (here, the vitals row itself) changed Toren after the damage was applied. The undo would put the
// "before" over it, so the whole undo is refused and nothing is written.
func TestUndoIsRefusedWhenACharactersHitPointsChangedAfterTheAction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e, last := appliedDamageOnToren(t, a)
	afterHit := a.vitals(t, a.toren).GetHitPointsCurrent()
	healed := afterHit + 2
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE character_vitals SET hit_points_current = $2 WHERE character_id = $1`, a.toren.GetId(), healed); err != nil {
		t.Fatalf("change the hit points: %v", err)
	}
	err := a.undo(t, a.master, e, last)
	wantCode(t, "UndoLastAction after the change", err, connect.CodeFailedPrecondition)
	b := blockedOf(t, err)
	if b.GetReason() != playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNDO_TARGET_CHANGED {
		t.Errorf("reason = %v, want UNDO_TARGET_CHANGED", b.GetReason())
	}
	if want := a.id(t, "Toren"); len(b.GetCombatantIds()) != 1 || b.GetCombatantIds()[0] != want {
		t.Errorf("combatant_ids = %v, want [%s]", b.GetCombatantIds(), want)
	}
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != healed {
		t.Errorf("hit points = %d after the refused undo, want %d: nothing is written", got, healed)
	}
	if got := a.log(t, a.master, e).GetUndoableEventId(); got != last {
		t.Errorf("the damage is no longer the last action after a refused undo")
	}
}

// TestUndoIsRefusedWhenANpcsHitPointsChangedAfterTheAction is the same for a combatant that holds its own hit points.
func TestUndoIsRefusedWhenANpcsHitPointsChangedAfterTheAction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(2)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	last := a.log(t, a.master, e).GetUndoableEventId()
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET hp_current = hp_current + 1 WHERE id = $1`, a.id(t, "Goblin")); err != nil {
		t.Fatalf("change the hit points: %v", err)
	}
	err := a.undo(t, a.master, e, last)
	wantCode(t, "UndoLastAction after the change", err, connect.CodeFailedPrecondition)
	if got := blockedOf(t, err).GetReason(); got != playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNDO_TARGET_CHANGED {
		t.Errorf("reason = %v, want UNDO_TARGET_CHANGED", got)
	}
}

// TestUndoWithNothingInBetweenStillWorks: the hit points are the ones the action left, so the undo goes through.
func TestUndoWithNothingInBetweenStillWorks(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e, last := appliedDamageOnToren(t, a)
	before := a.vitals(t, a.toren).GetHitPointsMax()
	if err := a.undo(t, a.master, e, last); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != before {
		t.Errorf("Toren = %d after the undo, want the full %d", got, before)
	}
}

// TestUndoOfAnEventWithoutHoldsIsNotChecked: an event from before the hit points were kept has nothing to compare.
func TestUndoOfAnEventWithoutHoldsIsNotChecked(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e, last := appliedDamageOnToren(t, a)
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE session_events SET payload = payload - 'holds' WHERE id = $1`, last); err != nil {
		t.Fatalf("drop the holds: %v", err)
	}
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE character_vitals SET hit_points_current = hit_points_current + 1 WHERE character_id = $1`, a.toren.GetId()); err != nil {
		t.Fatalf("change the hit points: %v", err)
	}
	if err := a.undo(t, a.master, e, last); err != nil {
		t.Fatalf("UndoLastAction() of an event without holds error = %v, want it to go through", err)
	}
}

// TestUndoRefusalNamesWhoChangedAndNoNumbers: the refusal carries the combatant, never a hit point number, and a
// player cannot ask for the undo at all (RN-10, RN-20): the master is the positive control.
func TestUndoRefusalNamesWhoChangedAndNoNumbers(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(2)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	last := a.log(t, a.master, e).GetUndoableEventId()
	const secretHP = 4217
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET hp_temp = $2 WHERE id = $1`, a.id(t, "Goblin"), secretHP); err != nil {
		t.Fatalf("change the hit points: %v", err)
	}
	// A player is refused before anything is read.
	err := a.undo(t, a.caio, e, last)
	wantCode(t, "UndoLastAction by a player", err, connect.CodePermissionDenied)
	if strings.Contains(err.Error(), "4217") {
		t.Errorf("the player's refusal leaks the hit points: %v", err)
	}
	// The master's refusal names who and says no number.
	err = a.undo(t, a.master, e, last)
	wantCode(t, "UndoLastAction by the master", err, connect.CodeFailedPrecondition)
	b := blockedOf(t, err)
	if len(b.GetCombatantIds()) != 1 {
		t.Errorf("combatant_ids = %v, want the goblin", b.GetCombatantIds())
	}
	if raw := b.String() + err.Error(); strings.Contains(raw, "4217") {
		t.Errorf("the master's refusal carries the hit points: %s", raw)
	}
}
