package migrations

import (
	"fmt"
	"testing"
)

// TestMigrationFindSteedCreature: a creature of the Find Steed spell has its own source, a type the player picked and a bond; a
// character keeps one bonded creature; a steed can be dismissed for the two new reasons. Down takes the steeds with it and
// leaves the creatures of the other sources, and Up brings the columns back.
func TestMigrationFindSteedCreature(t *testing.T) {
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
	scan := func(query string, dest ...any) {
		t.Helper()
		if err := db.QueryRowContext(ctx, query).Scan(dest...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	var user, campaign, character string
	scan(`INSERT INTO users DEFAULT VALUES RETURNING id`, &user)
	scan(fmt.Sprintf(`INSERT INTO campaigns (name, xp_mode, created_by) VALUES ('Mirathel', 'enemies', '%s') RETURNING id`, user), &campaign)
	scan(fmt.Sprintf(`INSERT INTO characters (campaign_id, kind, name, sheet, master_user_id, created_at, updated_at)
		VALUES ('%s', 'minion', 'Aurélio', '{}', '%s', now(), now()) RETURNING id`, campaign, user), &character)
	// insert makes a creature, then sets the columns of extra ("a=1, b=2") on it; the creature is gone again when the
	// table refuses either step.
	insert := func(source, extra string) error {
		var id string
		err := db.QueryRowContext(ctx, fmt.Sprintf(`INSERT INTO character_creatures
			(campaign_id, character_id, monster_key, name, source, attack, summon_group_id, hp_current, hp_max, created_at)
			VALUES ('%s', '%s', 'monster:warhorse', 'Trovão', '%s', 'full', gen_random_uuid(), 19, 19, now()) RETURNING id`,
			campaign, character, source)).Scan(&id)
		if err != nil || extra == "" {
			return err
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`UPDATE character_creatures SET %s WHERE id = '%s'`, extra, id)); err != nil {
			_, _ = db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM character_creatures WHERE id = '%s'`, id))
			return err
		}
		return nil
	}
	if err := insert("find_steed", `creature_type='fey', bonded=true`); err != nil {
		t.Fatalf("a bonded steed of the fey: %v", err)
	}
	for name, c := range map[string]struct{ source, extra string }{
		"a type that is no steed's":   {"find_steed", `creature_type='beast'`},
		"a second bonded creature":    {"find_steed", `creature_type='fiend', bonded=true`},
		"a source the table lacks":    {"find_mount", ``},
		"a reason the table lacks":    {"find_steed", `dismissed_at=now(), dismissed_reason='lost'`},
		"a reason without its moment": {"find_steed", `dismissed_reason='disappeared'`},
	} {
		if err := insert(c.source, c.extra); err == nil {
			t.Errorf("%s: the table accepted it", name)
		}
	}
	for _, reason := range []string{"disappeared", "dismissed"} {
		if err := insert("find_steed", fmt.Sprintf(`creature_type='celestial', dismissed_at=now(), dismissed_reason='%s'`, reason)); err != nil {
			t.Errorf("a steed dismissed as %q: %v", reason, err)
		}
	}
	if err := insert("familiar", ``); err != nil {
		t.Fatalf("a familiar: %v", err)
	}
	var bonded, typed int
	scan(`SELECT count(*) FROM character_creatures WHERE bonded`, &bonded)
	scan(`SELECT count(*) FROM character_creatures WHERE creature_type = ''`, &typed)
	if bonded != 1 || typed != 1 {
		t.Errorf("bonded = %d and untyped = %d, want 1 and 1 (the familiar)", bonded, typed)
	}

	if _, err := provider.DownTo(ctx, 337); err != nil {
		t.Fatalf("DownTo(337) error = %v", err)
	}
	var left int
	scan(`SELECT count(*) FROM character_creatures`, &left)
	if left != 1 {
		t.Errorf("after Down: %d creatures, want the familiar only", left)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() after Down error = %v", err)
	}
	scan(`SELECT count(*) FROM information_schema.columns WHERE table_name = 'character_creatures' AND column_name IN ('creature_type', 'bonded')`, &left)
	if left != 2 {
		t.Errorf("after Down then Up: %d of the 2 columns", left)
	}
}
