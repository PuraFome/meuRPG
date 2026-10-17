package play

import (
	"strings"
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// Characterization of slowFall (combat_slowfall.go) on falls the app never makes
// by itself: the parts of one fall, damage that is not a fall, another combatant's
// fall and an older fall.

// copyPending copies a pending damage with a new id and the changes given
// (column -> SQL value, with $1 the copy's id), and returns the new id.
func (a *armed) copyPending(t *testing.T, srcID string, set map[string]any) string {
	t.Helper()
	rows, err := a.h.pool.Query(t.Context(), `SELECT column_name FROM information_schema.columns WHERE table_name = 'pending_damages' AND column_name <> 'id' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatalf("list the columns: %v", err)
	}
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatalf("scan a column: %v", err)
		}
		cols = append(cols, c)
	}
	rows.Close()
	var id string
	list := strings.Join(cols, ", ")
	if err := a.h.pool.QueryRow(t.Context(), `INSERT INTO pending_damages (id, `+list+`) SELECT gen_random_uuid(), `+list+` FROM pending_damages WHERE id = $1 RETURNING id`, srcID).Scan(&id); err != nil {
		t.Fatalf("copy the pending damage: %v", err)
	}
	for col, v := range set {
		if _, err := a.h.pool.Exec(t.Context(), `UPDATE pending_damages SET `+col+` = $2 WHERE id = $1`, id, v); err != nil {
			t.Fatalf("set %s: %v", col, err)
		}
	}
	return id
}

func (a *armed) pendingAmount(t *testing.T, id string) int32 {
	t.Helper()
	var amt *int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT amount FROM pending_damages WHERE id = $1`, id).Scan(&amt); err != nil {
		t.Fatalf("read the pending damage: %v", err)
	}
	if amt == nil {
		return -1
	}
	return *amt
}

func (a *armed) torenFallID(t *testing.T) string {
	t.Helper()
	var id string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM pending_damages WHERE target_id = $1 AND trap_point_id IS NOT NULL`, a.id(t, "Toren")).Scan(&id); err != nil {
		t.Fatalf("find the fall: %v", err)
	}
	return id
}

func TestSlowFall_PartsOfOneFallShareTheReductionAndOtherDamageIsLeftAlone(t *testing.T) {
	t.Parallel()
	r, e := monkFall(t)
	first := r.torenFallID(t)
	mk := func(secs int, set map[string]any) string {
		id := r.copyPending(t, first, set)
		if _, err := r.h.pool.Exec(t.Context(), `UPDATE pending_damages SET created_at = created_at + $2::INT8 * interval '1 second' WHERE id = $1`, id, secs); err != nil {
			t.Fatalf("move the copy in time: %v", err)
		}
		return id
	}
	if _, err := r.h.pool.Exec(t.Context(), `UPDATE pending_damages SET amount = 12 WHERE id = $1`, first); err != nil {
		t.Fatalf("set the first part: %v", err)
	}
	second := mk(1, map[string]any{"amount": int32(15)})
	fire := mk(2, map[string]any{"amount": int32(50), "damage_type": "damage-type:fire"})
	other := mk(3, map[string]any{"amount": int32(9), "target_id": r.id(t, "Brisa")})
	zero := mk(4, map[string]any{"amount": int32(0)})

	if _, err := r.action(t, r.caio, e, "Toren", "feature:slow-fall"); err != nil {
		t.Fatalf("TakeAction(Slow Fall) error = %v", err)
	}
	// Level 4: 20 points. The first part takes 12, the second the 8 that are left.
	for name, want := range map[string]struct {
		id  string
		amt int32
	}{"first part": {first, 0}, "second part": {second, 7}, "fire damage": {fire, 50}, "another combatant's fall": {other, 9}, "a part of 0": {zero, 0}} {
		if got := r.pendingAmount(t, want.id); got != want.amt {
			t.Errorf("%s = %d, want %d", name, got, want.amt)
		}
	}
}

func TestSlowFall_OnlyTheLastFallIsReduced(t *testing.T) {
	t.Parallel()
	r, e := monkFall(t)
	older := r.torenFallID(t)
	second := r.trap(t, "Fosso 2", 9, 9, pit("6d6"), func(s *mapsv1.TrapSpec) { s.PresetKey = "trap:simple-pit" })
	newer := r.copyPending(t, older, map[string]any{"trap_point_id": second.GetId(), "amount": int32(30)})
	if _, err := r.h.pool.Exec(t.Context(), `UPDATE pending_damages SET created_at = created_at + interval '5 seconds' WHERE id = $1`, newer); err != nil {
		t.Fatalf("move the copy in time: %v", err)
	}
	if _, err := r.action(t, r.caio, e, "Toren", "feature:slow-fall"); err != nil {
		t.Fatalf("TakeAction(Slow Fall) error = %v", err)
	}
	if got := r.pendingAmount(t, older); got != 30 {
		t.Errorf("the older fall = %d, want 30: only the last fall happens", got)
	}
	if got := r.pendingAmount(t, newer); got != 10 {
		t.Errorf("the last fall = %d, want 10 (30 - 5 x 4)", got)
	}
}
