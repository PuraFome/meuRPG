package migrations

import (
	"fmt"
	"testing"
)

// TestMountSaveMigration: 00339 widens the kind CHECKs of the reaction windows ('mount_save', 'dismount') and of the inspiration holds
// ('mount_save') and adds the event kinds of the rider's save; before it none of them is accepted (00333 only has the mounted
// combat). The way down removes the rows of the new kinds and the kinds, and refuses them again; up, down and up again leave the same.
func TestMountSaveMigration(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx := t.Context()
	provider, err := NewTestProvider(ctx, db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.UpTo(ctx, 333); err != nil {
		t.Fatalf("UpTo(333) error = %v", err)
	}
	scan := func(query string, dest ...any) {
		t.Helper()
		if err := db.QueryRowContext(ctx, query).Scan(dest...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	var user, campaign, session, character, encounter string
	scan(`INSERT INTO users DEFAULT VALUES RETURNING id`, &user)
	scan(fmt.Sprintf(`INSERT INTO campaigns (name, xp_mode, created_by) VALUES ('Mirathel', 'enemies', '%s') RETURNING id`, user), &campaign)
	scan(fmt.Sprintf(`INSERT INTO game_sessions (campaign_id, session_number, started_at) VALUES ('%s', 1, now()) RETURNING id`, campaign), &session)
	scan(fmt.Sprintf(`INSERT INTO characters (campaign_id, kind, name, sheet, master_user_id, created_at, updated_at)
		VALUES ('%s', 'minion', 'Cavalo', '{}', '%s', now(), now()) RETURNING id`, campaign, user), &character)
	scan(fmt.Sprintf(`INSERT INTO encounters (game_session_id, name, status, grid_columns, grid_rows, created_at, mode)
		VALUES ('%s', 'No mapa', 'ended', 20, 10, now(), 'grid') RETURNING id`, session), &encounter)

	window := func(kind string) error {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO reaction_windows (encounter_id, group_id, kind, created_at) VALUES ('%s', gen_random_uuid(), '%s', now())`, encounter, kind))
		return err
	}
	hold := func(kind string) error {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO inspiration_holds (campaign_id, character_id, user_id, kind, idempotency_key, request, faces, modifier, total, counted, physical)
			VALUES ('%s', '%s', '%s', '%s', gen_random_uuid(), '\x00', '{12}', 0, 12, 0, false)`, campaign, character, user, kind))
		return err
	}
	kinds := func() int {
		var n int
		scan(`SELECT count(*) FROM session_event_kinds WHERE kind IN ('mount_save_rolled', 'rider_thrown')`, &n)
		return n
	}
	version := func() int {
		var n int
		scan(`SELECT count(*) FROM goose_db_version WHERE version_id = 339 AND is_applied`, &n)
		return n
	}
	for _, k := range []string{"mount_save", "dismount"} {
		if window(k) == nil {
			t.Errorf("a window of kind %s was accepted before 00339", k)
		}
	}
	if hold("mount_save") == nil {
		t.Error("a hold of kind mount_save was accepted before 00339")
	}
	if kinds() != 0 || version() != 0 {
		t.Errorf("before 00339: %d event kinds, version applied %d; want 0, 0", kinds(), version())
	}

	for round := range 2 { // up, down, up again: the same each time
		if _, err := provider.Up(ctx); err != nil {
			t.Fatalf("Up() round %d error = %v", round, err)
		}
		if version() != 1 {
			t.Fatalf("round %d: 00339 is not applied", round)
		}
		for _, k := range []string{"mount_save", "dismount"} {
			if err := window(k); err != nil {
				t.Errorf("round %d: window of kind %s after Up = %v", round, k, err)
			}
		}
		if err := hold("mount_save"); err != nil {
			t.Errorf("round %d: hold of kind mount_save after Up = %v", round, err)
		}
		if window("bogus") == nil || hold("bogus") == nil {
			t.Errorf("round %d: an unknown kind was accepted", round)
		}
		if kinds() != 2 {
			t.Errorf("round %d: event kinds = %d, want 2", round, kinds())
		}
		if _, err := provider.DownTo(ctx, 333); err != nil {
			t.Fatalf("DownTo(333) round %d error = %v", round, err)
		}
		var rows int
		scan(`SELECT (SELECT count(*) FROM reaction_windows WHERE kind IN ('mount_save', 'dismount')) + (SELECT count(*) FROM inspiration_holds WHERE kind = 'mount_save')`, &rows)
		if rows != 0 || kinds() != 0 {
			t.Errorf("round %d after Down: %d rows of the new kinds, %d event kinds; want none", round, rows, kinds())
		}
		if window("mount_save") == nil || window("dismount") == nil || hold("mount_save") == nil {
			t.Errorf("round %d: a new kind was accepted after Down", round)
		}
	}
}
