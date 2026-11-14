package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// openReadiedWindows counts the READIED windows open in the database.
func (a *armed) openReadiedWindows(t *testing.T) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM reaction_windows WHERE kind = 'readied' AND status = 'open'`).Scan(&n); err != nil {
		t.Fatalf("count the READIED windows: %v", err)
	}
	return n
}

// A held spell that breaks while its READIED window is open closes the window in the same call, so the table moves on:
// the master ending its concentration, and a correction that takes the player readier to 0 hit points.
func TestABrokenHeldSpellClosesItsOpenWindow(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		brk  func(t *testing.T, a *armed, e *playv1.Encounter)
	}{
		{"the master ends its concentration", func(t *testing.T, a *armed, e *playv1.Encounter) {
			if _, err := a.conditions(t, a.master, e, "Pensantus", nil, false, true); err != nil {
				t.Fatalf("SetCombatantConditions(end_concentration) error = %v", err)
			}
		}},
		{"a correction to 0 hit points", func(t *testing.T, a *armed, _ *playv1.Encounter) { a.correct(t, a.pens, hpIs(0)) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := newCasters(t)
			e := a.readyHoldPerson(t)
			a.mustFire(t, e, "Pensantus")
			c.brk(t, a, e)
			if got := a.readiedStatus(t, "Pensantus"); got != readiedBroken {
				t.Fatalf("rows = %q, want broken", got)
			}
			if n := a.openReadiedWindows(t); n != 0 {
				t.Errorf("%d READIED windows still open after the held spell broke", n)
			}
			if _, err := a.endTurn(t, a.master, e, false); err != nil {
				t.Errorf("EndTurn after the held spell broke: %v", err)
			}
		})
	}
}
