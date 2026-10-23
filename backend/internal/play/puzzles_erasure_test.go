package play

import (
	"strings"
	"testing"
)

// Erasing an account clears the ID from its puzzle moves and leaves the typed text,
// anonymous, with the session (docs/privacy.md, "Delete the account"). The other
// player's move and the run's move count keep working.
func TestPuzzleMoves_ErasedAccountKeepsTypedTextWithoutTheID(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	ctx := t.Context()
	pz := p.riddle(t, "Apagar")
	p.show(t, pz.GetId())
	if _, err := p.move(t, p.ana, pz.GetId(), riddleMove("texto-da-ana")); err != nil {
		t.Fatalf("MakePuzzleMove(ana) error = %v", err)
	}
	if _, err := p.move(t, p.bia, pz.GetId(), riddleMove("texto-da-bia")); err != nil {
		t.Fatalf("MakePuzzleMove(bia) error = %v", err)
	}
	if _, err := p.h.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, p.ana.id); err != nil {
		t.Fatalf("erasing the account: %v", err)
	}
	rows, err := p.h.pool.Query(ctx, `SELECT user_id IS NULL, move::text FROM puzzle_moves ORDER BY seq`)
	if err != nil {
		t.Fatalf("reading the moves: %v", err)
	}
	defer rows.Close()
	var anonymous []bool
	var texts []string
	for rows.Next() {
		var gone bool
		var mv string
		if err := rows.Scan(&gone, &mv); err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		anonymous = append(anonymous, gone)
		texts = append(texts, mv)
	}
	if len(texts) != 2 || !anonymous[0] || anonymous[1] ||
		!strings.Contains(texts[0], "texto-da-ana") || !strings.Contains(texts[1], "texto-da-bia") {
		t.Errorf("moves after the erasure = %v with anonymous %v, want both texts kept and only the first one without the account", texts, anonymous)
	}
	if m := p.masterRun(t, pz.GetId()); m.GetMovesMade() != 2 {
		t.Errorf("the master reads %d moves, want 2", m.GetMovesMade())
	}
}
