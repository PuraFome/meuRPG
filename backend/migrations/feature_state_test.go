package migrations

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestFeatureStateMigrationCopiesOldMarks: 00329 turns the once-per-turn marks of the three old columns into entries of
// scope turn in feature_state, touches only the rows that have a mark, and runs again without changing anything.
func TestFeatureStateMigrationCopiesOldMarks(t *testing.T) {
	t.Parallel()
	db := freshDatabase(t)
	ctx := t.Context()
	provider, err := NewTestProvider(ctx, db)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, err := provider.UpTo(ctx, 325); err != nil {
		t.Fatalf("UpTo(325) error = %v", err)
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
		VALUES ('%s', 'minion', 'Goblin', '{}', '%s', now(), now()) RETURNING id`, campaign, user), &character)
	scan(fmt.Sprintf(`INSERT INTO encounters (game_session_id, name, status, grid_columns, grid_rows, created_at, mode)
		VALUES ('%s', 'No mapa', 'ended', 20, 10, now(), 'grid') RETURNING id`, session), &encounter)
	combatant := func(label, sneak, colossus, table string) string {
		var id string
		scan(fmt.Sprintf(`INSERT INTO combatants (encounter_id, character_id, label, kind, hidden, initiative_bonus, order_index, speed_ft, created_at, hp_current, hp_max, hp_temp,
			sneak_attack_turn, colossus_slayer_turn, table_once_turns)
			VALUES ('%s', '%s', '%s', 'npc', false, 0, 0, 30, now(), 7, 7, 0, NULLIF('%s', ''), NULLIF('%s', ''), '%s') RETURNING id`, encounter, character, label, sneak, colossus, table), &id)
		return id
	}
	all := combatant("Todas", "2:aaa", "2:bbb", `{"feature:giant": "2:ccc", "feature:ember": "1:ddd"}`)
	sneakOnly := combatant("Furtivo", "3:aaa", "", `{}`)
	tableOnly := combatant("Mesa", "", "", `{"feature:giant": "4:ccc"}`)
	none := combatant("Nada", "", "", `{}`)

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	turn := func(at string) map[string]any { return map[string]any{"s": "turn", "at": at, "n": float64(1)} }
	want := map[string]map[string]any{
		all:       {"sneak_attack": turn("2:aaa"), "colossus_slayer": turn("2:bbb"), "feature:giant": turn("2:ccc"), "feature:ember": turn("1:ddd")},
		sneakOnly: {"sneak_attack": turn("3:aaa")},
		tableOnly: {"feature:giant": turn("4:ccc")},
		none:      {},
	}
	check := func(when string) {
		t.Helper()
		for id, w := range want {
			var raw string
			scan(fmt.Sprintf(`SELECT feature_state::text FROM combatants WHERE id = '%s'`, id), &raw)
			var got map[string]any
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatalf("%s: feature_state %q: %v", when, raw, err)
			}
			if fmt.Sprint(got) != fmt.Sprint(w) {
				t.Errorf("%s: feature_state of %s = %v, want %v", when, id, got, w)
			}
		}
	}
	check("after Up")

	// A row that already has a state is left alone when the migration runs again.
	exec(t, db, fmt.Sprintf(`UPDATE combatants SET feature_state = '{"x": {"s": "combat", "at": "", "n": 2}}' WHERE id = '%s'`, none))
	want[none] = map[string]any{"x": map[string]any{"s": "combat", "at": "", "n": float64(2)}}
	exec(t, db, "DELETE FROM goose_db_version WHERE version_id >= 329")
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("second Up() error = %v", err)
	}
	check("after the second Up")

	// The check refuses a state that is not an object or is too big (over 16384 bytes of the jsonb text; the server's own limit
	// is 8192 compact bytes, which is never over this).
	for _, bad := range []string{`'[]'`, `'"x"'`, `jsonb_build_object('k', repeat('x', 17000))`} {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`UPDATE combatants SET feature_state = %s WHERE id = '%s'`, bad, none)); err == nil {
			t.Errorf("feature_state = %s was accepted", bad)
		}
	}
}
