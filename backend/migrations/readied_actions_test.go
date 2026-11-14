package migrations

import (
	"strings"
	"testing"
)

// readiedMigration is the version of the migration that creates readied_actions (00334).
const readiedMigration = 334

// TestReadiedActionsMigrationRunsTwice: 00334 runs again over itself (a migration that failed halfway runs again) and
// after its own Down, and leaves the same schema each time: the table with its one waiting row per combatant, the
// reaction window kind 'readied' and the three event kinds.
func TestReadiedActionsMigrationRunsTwice(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx := t.Context()
	provider, err := NewTestProvider(ctx, db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	before := schema(t, db)

	// 00334 and every later migration run again: goose runs none lower than the database version.
	exec(t, db, "DELETE FROM goose_db_version WHERE version_id >= 334")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if after := schema(t, db); after != before {
		t.Errorf("schema changed when 00334 ran again.\nbefore:\n%s\nafter:\n%s", before, after)
	}

	if _, err := provider.DownTo(ctx, readiedMigration-1); err != nil {
		t.Fatalf("DownTo(%d) error = %v", readiedMigration-1, err)
	}
	if strings.Contains(schema(t, db), "readied_actions") {
		t.Error("the Down of 00334 left readied_actions behind")
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() after the Down error = %v", err)
	}
	after := schema(t, db)
	if after != before {
		t.Errorf("schema changed after the Down and Up of 00334.\nbefore:\n%s\nafter:\n%s", before, after)
	}
	for _, want := range []string{"readied_actions_one_waiting_idx", "readied_actions_slot_valid", "'readied'"} {
		if !strings.Contains(after, want) {
			t.Errorf("the schema has no %s", want)
		}
	}
	var kinds int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM session_event_kinds WHERE kind IN ('action_readied', 'readied_fired', 'readied_lost')`).Scan(&kinds); err != nil {
		t.Fatal(err)
	}
	if kinds != 3 {
		t.Errorf("session_event_kinds has %d of the readied kinds, want 3", kinds)
	}
}
