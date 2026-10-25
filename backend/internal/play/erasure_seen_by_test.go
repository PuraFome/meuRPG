package play

import (
	"bytes"
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
	session, otherSession := h.insertGameSession(campaignID), h.insertGameSession(otherCampaignID)

	a, c, m := `"`+ana.id+`"`, `"`+caio.id+`"`, `"`+master.id+`"`
	cases := []struct{ name, before, after string }{
		{"seen_by with Ana first", `{"fogged":true,"seen_by":[` + a + `,` + c + `]}`, `{"fogged":true,"seen_by":[` + c + `]}`},
		{"seen_by keeps the order", `{"seen_by":[` + c + `,` + a + `,` + m + `]}`, `{"seen_by":[` + c + `,` + m + `]}`},
		{"Ana alone leaves an empty list", `{"fogged":true,"seen_by":[` + a + `]}`, `{"fogged":true,"seen_by":[]}`},
		{"an empty list stays", `{"fogged":true,"seen_by":[]}`, `{"fogged":true,"seen_by":[]}`},
		{"a null list stays", `{"fogged":true,"seen_by":null}`, `{"fogged":true,"seen_by":null}`},
		{"no list", `{"round":2}`, `{"round":2}`},
		// A scene open to only some players (MR-015): the players it was opened to, and the players in it when a check was rolled.
		{"players of a scene", `{"point_id":"p","actions":1,"players":[` + a + `,` + c + `]}`, `{"point_id":"p","actions":1,"players":[` + c + `]}`},
		{"audience of a roll", `{"point_id":"p","audience":[` + c + `,` + a + `],"dc_shown":true}`, `{"point_id":"p","audience":[` + c + `],"dc_shown":true}`},
		{"Ana alone in a scene", `{"point_id":"p","players":[` + a + `]}`, `{"point_id":"p","players":[]}`},
		{"cover_seen_by", `{"cover_restricted":true,"cover_seen_by":[` + a + `,` + c + `]}`, `{"cover_restricted":true,"cover_seen_by":[` + c + `]}`},
		{
			// The masks of the hits count the players by their place in cover_users: the
			// place of Ana is emptied, not removed.
			"a cast",
			`{"cast_id":"x","cover_users":[` + a + `,` + c + `],"hits":[` +
				`{"target_id":"t1","fogged":true,"seen_mask":3,"cover_seen_by":[` + c + `,` + a + `]},` +
				`{"target_id":"t2","cover_seen_by":[` + a + `]},{"target_id":"t3"}]}`,
			`{"cast_id":"x","cover_users":["",` + c + `],"hits":[` +
				`{"target_id":"t1","fogged":true,"seen_mask":3,"cover_seen_by":[` + c + `]},` +
				`{"target_id":"t2","cover_seen_by":[]},{"target_id":"t3"}]}`,
		},
		{
			"a trap",
			`{"trap":{"caught":[{"target_id":"t1","fogged":true,"seen_by":[` + a + `,` + c + `]},{"target_id":"t2"}]}}`,
			`{"trap":{"caught":[{"target_id":"t1","fogged":true,"seen_by":[` + c + `]},{"target_id":"t2"}]}}`,
		},
	}
	for i, tc := range cases {
		h.insertSessionEvent(session, i+1, tc.before)
	}
	const untouched = `{"fogged":true,"seen_by":[]}`
	h.insertSessionEvent(otherSession, 1, strings.Replace(untouched, "[]", "["+c+"]", 1))

	if _, err := h.pool.Exec(t.Context(), `DELETE FROM users WHERE id = $1`, ana.id); err != nil {
		t.Fatalf("erase the account: %v", err)
	}

	got := h.sessionPayloads(session)
	if len(got) != len(cases) {
		t.Fatalf("events after the erasure = %d, want %d: the history stays", len(got), len(cases))
	}
	for i, tc := range cases {
		if strings.Contains(got[i], ana.id) {
			t.Errorf("%s: the payload still holds the erased account: %s", tc.name, got[i])
		}
		if !sameJSON(t, got[i], tc.after) {
			t.Errorf("%s: payload = %s, want %s", tc.name, got[i], tc.after)
		}
	}
	if other := h.sessionPayloads(otherSession); len(other) != 1 || !sameJSON(t, other[0], `{"fogged":true,"seen_by":[`+c+`]}`) {
		t.Errorf("another campaign's events = %v, want them untouched", other)
	}
}

func (h *harness) insertGameSession(campaignID string) string {
	h.t.Helper()
	var id string
	if err := h.pool.QueryRow(h.t.Context(), `INSERT INTO game_sessions (campaign_id, session_number, started_at) VALUES ($1, 1, now()) RETURNING id`, campaignID).Scan(&id); err != nil {
		h.t.Fatalf("insert game session: %v", err)
	}
	return id
}

func (h *harness) insertSessionEvent(sessionID string, seq int, payload string) {
	h.t.Helper()
	if _, err := h.pool.Exec(h.t.Context(), `INSERT INTO session_events (game_session_id, seq, kind, payload, created_at) VALUES ($1, $2, 'attack_rolled', $3::jsonb, now())`, sessionID, seq, payload); err != nil {
		h.t.Fatalf("insert event %d: %v", seq, err)
	}
}

// sessionPayloads are the payloads of a game session's events, in order.
func (h *harness) sessionPayloads(sessionID string) (out []string) {
	h.t.Helper()
	rows, err := h.pool.Query(h.t.Context(), `SELECT payload::text FROM session_events WHERE game_session_id = $1 ORDER BY seq`, sessionID)
	if err != nil {
		h.t.Fatalf("read events: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			h.t.Fatalf("scan: %v", err)
		}
		out = append(out, s)
	}
	return out
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
	return bytes.Equal(xa, ya)
}
