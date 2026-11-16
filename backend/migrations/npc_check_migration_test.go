package migrations

import (
	"fmt"
	"testing"
)

// TestNpcCheckMigration: 00354 widens the kind CHECK of the inspiration holds ('npc_check'), creates npc_check_asks (one row for each
// bard asked about a check of the master's side, in order) and adds the two event kinds of the check; before it none of them exists.
// The way down removes the rows of the kind, the table and the kinds, and refuses the kind again; up, down and up again leave the same.
func TestNpcCheckMigration(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx := t.Context()
	provider, err := NewTestProvider(ctx, db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.UpTo(ctx, 352); err != nil {
		t.Fatalf("UpTo(352) error = %v", err)
	}
	scan := func(query string, dest ...any) {
		t.Helper()
		if err := db.QueryRowContext(ctx, query).Scan(dest...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	var user, campaign, npc, bard, bard2 string
	scan(`INSERT INTO users DEFAULT VALUES RETURNING id`, &user)
	scan(fmt.Sprintf(`INSERT INTO campaigns (name, xp_mode, created_by) VALUES ('Mirathel', 'enemies', '%s') RETURNING id`, user), &campaign)
	character := func(name string, dest *string) {
		scan(fmt.Sprintf(`INSERT INTO characters (campaign_id, kind, name, sheet, master_user_id, created_at, updated_at)
			VALUES ('%s', 'minion', '%s', '{}', '%s', now(), now()) RETURNING id`, campaign, name, user), dest)
	}
	character("Guarda", &npc)
	character("Bardo", &bard)
	character("Outra", &bard2)

	hold := func(kind string) (string, error) {
		var id string
		err := db.QueryRowContext(ctx, fmt.Sprintf(`INSERT INTO inspiration_holds (campaign_id, character_id, user_id, kind, idempotency_key, request, faces, modifier, total, counted, physical)
			VALUES ('%s', '%s', '%s', '%s', gen_random_uuid(), '\x00', '{12}', 3, 15, 0, false) RETURNING id`, campaign, npc, user, kind)).Scan(&id)
		return id, err
	}
	ask := func(holdID, bardID string, position int, state string) error {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO npc_check_asks (hold_id, bard_character_id, position, state, sides, creature_seen)
			VALUES ('%s', '%s', %d, '%s', 8, true)`, holdID, bardID, position, state))
		return err
	}
	kinds := func() int {
		var n int
		scan(`SELECT count(*) FROM session_event_kinds WHERE kind IN ('npc_check_held', 'npc_check_resolved')`, &n)
		return n
	}
	applied := func() int {
		var n int
		scan(`SELECT count(*) FROM goose_db_version WHERE version_id = 354 AND is_applied`, &n)
		return n
	}
	tableExists := func() bool {
		var n int
		scan(`SELECT count(*) FROM information_schema.tables WHERE table_name = 'npc_check_asks'`, &n)
		return n == 1
	}
	if _, err := hold("npc_check"); err == nil {
		t.Error("a hold of kind npc_check was accepted before 00354")
	}
	if kinds() != 0 || applied() != 0 || tableExists() {
		t.Errorf("before 00354: %d event kinds, applied %d, table %v; want 0, 0, false", kinds(), applied(), tableExists())
	}

	for round := range 2 { // up, down, up again: the same each time
		if _, err := provider.Up(ctx); err != nil {
			t.Fatalf("Up() round %d error = %v", round, err)
		}
		if applied() != 1 || !tableExists() || kinds() != 2 {
			t.Fatalf("round %d after Up: applied %d, table %v, kinds %d; want 1, true, 2", round, applied(), tableExists(), kinds())
		}
		id, err := hold("npc_check")
		if err != nil {
			t.Fatalf("round %d: hold of kind npc_check after Up = %v", round, err)
		}
		if _, err := hold("mount_save"); err != nil { // the widening keeps the kinds before it
			t.Errorf("round %d: hold of kind mount_save after Up = %v", round, err)
		}
		if _, err := hold("bogus"); err == nil {
			t.Errorf("round %d: an unknown kind was accepted", round)
		}
		if err := ask(id, bard, 0, "asked"); err != nil {
			t.Errorf("round %d: first ask = %v", round, err)
		}
		if err := ask(id, bard, 1, "asked"); err == nil {
			t.Errorf("round %d: the same bard was asked twice about one check", round)
		}
		if err := ask(id, bard2, 0, "asked"); err == nil {
			t.Errorf("round %d: two bards took the same place", round)
		}
		if err := ask(id, bard2, 1, "bogus"); err == nil {
			t.Errorf("round %d: an unknown state was accepted", round)
		}
		if err := ask(id, bard2, 1, "cut"); err != nil {
			t.Errorf("round %d: second ask = %v", round, err)
		}
		// A bard whose character is deleted keeps its question, with no character: more than one may, for the same check.
		var gone1, gone2 string
		character("Sai", &gone1)
		character("Foi", &gone2)
		other, err := hold("npc_check")
		if err != nil {
			t.Fatalf("round %d: second hold of kind npc_check = %v", round, err)
		}
		if err := ask(other, gone1, 0, "asked"); err != nil {
			t.Errorf("round %d: ask of a bard to be deleted = %v", round, err)
		}
		if err := ask(other, gone2, 1, "asked"); err != nil {
			t.Errorf("round %d: ask of a second bard to be deleted = %v", round, err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM characters WHERE id IN ($1, $2)`, gone1, gone2); err != nil {
			t.Fatalf("round %d: delete the bards' characters: %v", round, err)
		}
		var kept, nobody int
		scan(fmt.Sprintf(`SELECT count(*), count(*) FILTER (WHERE bard_character_id IS NULL) FROM npc_check_asks WHERE hold_id = '%s'`, other), &kept, &nobody)
		if kept != 2 || nobody != 2 {
			t.Errorf("round %d: %d asks, %d with no character after the bards' characters were deleted; want 2 and 2", round, kept, nobody)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM inspiration_holds WHERE id IN ('%s', '%s')`, id, other)); err != nil {
			t.Fatalf("round %d: forget the holds: %v", round, err)
		}
		var left int
		scan(`SELECT count(*) FROM npc_check_asks`, &left)
		if left != 0 {
			t.Errorf("round %d: %d asks stayed after their hold went", round, left)
		}
		if _, err := hold("npc_check"); err != nil {
			t.Fatalf("round %d: hold of kind npc_check = %v", round, err)
		}
		if _, err := provider.DownTo(ctx, 352); err != nil {
			t.Fatalf("DownTo(352) round %d error = %v", round, err)
		}
		var rows int
		scan(`SELECT count(*) FROM inspiration_holds WHERE kind = 'npc_check'`, &rows)
		if rows != 0 || kinds() != 0 || tableExists() {
			t.Errorf("round %d after Down: %d holds of the kind, %d event kinds, table %v; want none", round, rows, kinds(), tableExists())
		}
		if _, err := hold("npc_check"); err == nil {
			t.Errorf("round %d: the kind was accepted after Down", round)
		}
		if _, err := hold("mount_save"); err != nil {
			t.Errorf("round %d: the kinds before it are refused after Down: %v", round, err)
		}
	}
}
