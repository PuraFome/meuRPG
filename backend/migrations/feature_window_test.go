package migrations

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// featureWindowMigration is the version of the migration that widens reaction_windows.kind with 'feature' (00346).
const featureWindowMigration = 346

// TestFeatureWindowMigrationRunsTwice: 00346 runs again over itself and after its own Down, and leaves the same schema each
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

	// 00346 and every later migration run again: goose runs none lower than the database version.
	exec(t, db, "DELETE FROM goose_db_version WHERE version_id >= 346")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	if after := schema(t, db); after != before {
		t.Errorf("schema changed when 00346 ran again.\nbefore:\n%s\nafter:\n%s", before, after)
	}

	if _, err := provider.DownTo(ctx, featureWindowMigration-1); err != nil {
		t.Fatalf("DownTo(%d) error = %v", featureWindowMigration-1, err)
	}
	down := windowKindCheck(t, db)
	if strings.Contains(down, "'feature'") {
		t.Errorf("the Down of 00346 left the kind 'feature' behind: %s", down)
	}
	if !strings.Contains(down, "'readied'") {
		t.Errorf("the Down of 00346 dropped the kind 'readied' of 00334: %s", down)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() after the Down error = %v", err)
	}
	if after := schema(t, db); after != before {
		t.Errorf("schema changed after the Down and Up of 00346.\nbefore:\n%s\nafter:\n%s", before, after)
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

// TestFeatureWindowMigrationKeepsTheMountWindows: a database at 00339, which adds the kinds 'mount_save' and 'dismount',
// with a window of each keeps both through the Up and the Down of 00346: the Up lists them, and the Down puts back the list of
// 00339 (without 'feature'), so no kind is dropped and no stored window breaks the CHECK.
func TestFeatureWindowMigrationKeepsTheMountWindows(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx := t.Context()
	provider, err := NewTestProvider(ctx, db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.UpTo(ctx, featureWindowMigration-1); err != nil {
		t.Fatalf("UpTo(%d) error = %v", featureWindowMigration-1, err)
	}
	scan := func(query string, dest ...any) {
		t.Helper()
		if err := db.QueryRowContext(ctx, query).Scan(dest...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	var user, campaign, session, character, encounter, reactor string
	scan(`INSERT INTO users DEFAULT VALUES RETURNING id`, &user)
	scan(fmt.Sprintf(`INSERT INTO campaigns (name, xp_mode, created_by) VALUES ('Mirathel', 'enemies', '%s') RETURNING id`, user), &campaign)
	scan(fmt.Sprintf(`INSERT INTO game_sessions (campaign_id, session_number, started_at) VALUES ('%s', 1, now()) RETURNING id`, campaign), &session)
	scan(fmt.Sprintf(`INSERT INTO characters (campaign_id, kind, name, sheet, master_user_id, created_at, updated_at)
		VALUES ('%s', 'minion', 'Cavaleiro', '{}', '%s', now(), now()) RETURNING id`, campaign, user), &character)
	scan(fmt.Sprintf(`INSERT INTO encounters (game_session_id, name, status, grid_columns, grid_rows, created_at, mode)
		VALUES ('%s', 'No mapa', 'ended', 20, 10, now(), 'grid') RETURNING id`, session), &encounter)
	scan(fmt.Sprintf(`INSERT INTO combatants (encounter_id, character_id, label, kind, hidden, initiative_bonus, order_index, speed_ft, created_at, hp_current, hp_max, hp_temp)
		VALUES ('%s', '%s', 'Cavaleiro', 'npc', false, 0, 0, 30, now(), 7, 7, 0) RETURNING id`, encounter, character), &reactor)
	for _, kind := range []string{"mount_save", "dismount"} {
		exec(t, db, fmt.Sprintf(`INSERT INTO reaction_windows (encounter_id, group_id, kind, reactor_id, created_at)
			VALUES ('%s', gen_random_uuid(), '%s', '%s', now())`, encounter, kind, reactor))
	}
	count := func(when string) {
		t.Helper()
		var n int
		scan(`SELECT count(*) FROM reaction_windows WHERE kind IN ('mount_save', 'dismount')`, &n)
		if n != 2 {
			t.Errorf("%s: %d mount windows, want 2", when, n)
		}
	}

	if _, err := provider.UpTo(ctx, featureWindowMigration); err != nil {
		t.Fatalf("Up of 00346 over the mount windows error = %v", err)
	}
	count("after the Up of 00346")
	up := windowKindCheck(t, db)
	for _, kind := range []string{"mount_save", "dismount", "feature", "readied"} {
		if !strings.Contains(up, "'"+kind+"'") {
			t.Errorf("the kinds after the Up of 00346 (%s) have no %s", up, kind)
		}
	}
	if _, err := provider.DownTo(ctx, featureWindowMigration-1); err != nil {
		t.Fatalf("Down of 00346 over the mount windows error = %v", err)
	}
	count("after the Down of 00346")
	down := windowKindCheck(t, db)
	if strings.Contains(down, "'feature'") || !strings.Contains(down, "'mount_save'") || !strings.Contains(down, "'dismount'") {
		t.Errorf("the kinds after the Down of 00346 (%s) must be the list of 00339: mount_save and dismount, no feature", down)
	}
}
