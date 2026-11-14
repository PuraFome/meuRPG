package migrations

import (
	"database/sql"
	"strings"
	"testing"
)

// featureWindowMigration is the version of the migration that widens reaction_windows.kind with 'feature' (00340).
const featureWindowMigration = 340

// TestFeatureWindowMigrationRunsTwice: 00340 runs again over itself and after its own Down, and leaves the same schema each
// time: the reaction window kinds of 00334 and the new 'feature', and no kind lost.
func TestFeatureWindowMigrationRunsTwice(t *testing.T) {
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
	kinds := windowKindCheck(t, db)
	allKinds := []string{
		"shield", "uncanny_dodge", "hellish_rebuke", "counterspell", "cutting_words", "deflect_missiles", "feather_fall",
		"concentration_save", "master_check", "effect_save", "contest", "maneuver_reduce", "protection", "legendary_resistance",
		"parry", "slow_fall", "readied", "feature",
	}
	for _, kind := range allKinds {
		if !strings.Contains(kinds, "'"+kind+"'") {
			t.Errorf("the reaction window kinds (%s) have no %s", kinds, kind)
		}
	}

	// 00340 and every later migration run again: goose runs none lower than the database version.
	exec(t, db, "DELETE FROM goose_db_version WHERE version_id >= 340")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if after := schema(t, db); after != before {
		t.Errorf("schema changed when 00340 ran again.\nbefore:\n%s\nafter:\n%s", before, after)
	}

	if _, err := provider.DownTo(ctx, featureWindowMigration-1); err != nil {
		t.Fatalf("DownTo(%d) error = %v", featureWindowMigration-1, err)
	}
	down := windowKindCheck(t, db)
	if strings.Contains(down, "'feature'") {
		t.Errorf("the Down of 00340 left the kind 'feature' behind: %s", down)
	}
	if !strings.Contains(down, "'readied'") {
		t.Errorf("the Down of 00340 dropped the kind 'readied' of 00334: %s", down)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() after the Down error = %v", err)
	}
	if after := schema(t, db); after != before {
		t.Errorf("schema changed after the Down and Up of 00340.\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// windowKindCheck is what reaction_windows.kind is checked against.
func windowKindCheck(t *testing.T, db *sql.DB) string {
	t.Helper()
	var clause string
	if err := db.QueryRowContext(t.Context(), `SELECT check_clause FROM information_schema.check_constraints WHERE constraint_name = 'reaction_windows_kind_valid'`).Scan(&clause); err != nil {
		t.Fatalf("read the reaction window kinds: %v", err)
	}
	return clause
}
