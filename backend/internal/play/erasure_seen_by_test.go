package play

import (
	"encoding/json"
	"strings"
	"testing"
)

// Erasing an account (DELETE FROM users, which is how the operator does it) must also
// take the account's ID out of the lists of who saw an NPC act that the history keeps
// inside its payloads (docs/privacy.md). The event itself stays.
func TestErasingAnAccountScrubsItsIDFromTheWhoSawListsOfTheHistory(t *testing.T) {
	h := newHarness(t)
	master, ana, caio := h.newUser("Mestre"), h.newUser("Ana"), h.newUser("Caio")
	campaignID := h.newCampaign(master, "Mesa", ana, caio)
	otherCampaignID := h.newCampaign(caio, "Outra mesa") // Ana was never a member of it
	ctx := t.Context()

	sessionOf := func(campaign string) string {
		t.Helper()
		var id string
		if err := h.pool.QueryRow(ctx, `INSERT INTO game_sessions (campaign_id, session_number, started_at) VALUES ($1, 1, now()) RETURNING id`, campaign).Scan(&id); err != nil {
			t.Fatalf("insert game session: %v", err)
		}
		return id
	}
	session, otherSession := sessionOf(campaignID), sessionOf(otherCampaignID)

	a, c := ana.id, caio.id
	payloads := []string{
		`{"fogged":true,"seen_by":["` + a + `","` + c + `"]}`,
		`{"fogged":true,"seen_by":["` + c + `","` + a + `","` + master.id + `"]}`,
		`{"fogged":true,"seen_by":["` + a + `"]}`,
		`{"fogged":true,"seen_by":[]}`,
		`{"fogged":true,"seen_by":null}`,
		`{"round":2}`,
		`{"cover_restricted":true,"cover_seen_by":["` + a + `","` + c + `"]}`,
		`{"cast_id":"x","cover_users":["` + a + `","` + c + `"],"hits":[` +
			`{"target_id":"t1","fogged":true,"seen_mask":3,"cover_seen_by":["` + c + `","` + a + `"]},` +
			`{"target_id":"t2","cover_seen_by":["` + a + `"]},{"target_id":"t3"}]}`,
		`{"trap":{"caught":[{"target_id":"t1","fogged":true,"seen_by":["` + a + `","` + c + `"]},{"target_id":"t2"}]}}`,
	}
	insert := func(sessionID string, seq int, payload string) {
		t.Helper()
		if _, err := h.pool.Exec(ctx, `INSERT INTO session_events (game_session_id, seq, kind, payload, created_at) VALUES ($1, $2, 'attack_rolled', $3::jsonb, now())`, sessionID, seq, payload); err != nil {
			t.Fatalf("insert event %d: %v", seq, err)
		}
	}
	for i, p := range payloads {
		insert(session, i+1, p)
	}
	insert(otherSession, 1, `{"fogged":true,"seen_by":["`+c+`"]}`)

	read := func() (out []string) {
		t.Helper()
		rows, err := h.pool.Query(ctx, `SELECT payload::text FROM session_events WHERE game_session_id = $1 ORDER BY seq`, session)
		if err != nil {
			t.Fatalf("read events: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatalf("scan: %v", err)
			}
			out = append(out, s)
		}
		return out
	}

	if _, err := h.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatalf("erase the account: %v", err)
	}

	got := read()
	if len(got) != len(payloads) {
		t.Fatalf("events after the erasure = %d, want %d: the history stays", len(got), len(payloads))
	}
	for i, p := range got {
		if strings.Contains(p, a) {
			t.Errorf("event %d still holds the erased account: %s", i+1, p)
		}
	}
	// What is left reads the same for everyone else, in the same order.
	want := map[int]string{
		1: `{"fogged":true,"seen_by":["` + c + `"]}`,
		2: `{"fogged":true,"seen_by":["` + c + `","` + master.id + `"]}`,
		3: `{"fogged":true,"seen_by":[]}`,
		4: `{"fogged":true,"seen_by":[]}`,
		5: `{"fogged":true,"seen_by":null}`,
		6: `{"round":2}`,
		7: `{"cover_restricted":true,"cover_seen_by":["` + c + `"]}`,
		9: `{"trap":{"caught":[{"seen_by":["` + c + `"],"fogged":true,"target_id":"t1"},{"target_id":"t2"}]}}`,
	}
	for n, w := range want {
		if !sameJSON(t, got[n-1], w) {
			t.Errorf("event %d = %s, want %s", n, got[n-1], w)
		}
	}
	// A cast keeps the order of CoverUsers: the masks of its hits are bits of it, so the
	// erased account's place is emptied, not removed.
	var cast struct {
		CoverUsers []string `json:"cover_users"`
		Hits       []struct {
			SeenMask    uint64   `json:"seen_mask"`
			CoverSeenBy []string `json:"cover_seen_by"`
		} `json:"hits"`
	}
	if err := json.Unmarshal([]byte(got[7]), &cast); err != nil {
		t.Fatal(err)
	}
	if len(cast.CoverUsers) != 2 || cast.CoverUsers[0] != "" || cast.CoverUsers[1] != c {
		t.Errorf("cover_users = %q, want [\"\", Caio]", cast.CoverUsers)
	}
	if len(cast.Hits) != 3 || cast.Hits[0].SeenMask != 3 || len(cast.Hits[0].CoverSeenBy) != 1 || cast.Hits[0].CoverSeenBy[0] != c || len(cast.Hits[1].CoverSeenBy) != 0 {
		t.Errorf("hits = %+v, want the mask kept and Caio alone in the lists", cast.Hits)
	}

	var other string
	if err := h.pool.QueryRow(ctx, `SELECT payload::text FROM session_events WHERE game_session_id = $1`, otherSession).Scan(&other); err != nil || !sameJSON(t, other, `{"fogged":true,"seen_by":["`+c+`"]}`) {
		t.Errorf("another campaign's event = %s, %v; want it untouched", other, err)
	}
}

// sameJSON says whether two documents hold the same values (key order aside).
func sameJSON(t *testing.T, a, b string) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal([]byte(a), &x); err != nil {
		t.Fatalf("not JSON: %s", a)
	}
	if err := json.Unmarshal([]byte(b), &y); err != nil {
		t.Fatalf("not JSON: %s", b)
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return string(xa) == string(ya)
}
