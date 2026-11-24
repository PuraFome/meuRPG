package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/autopass"
)

// A READIED window the master passes with the window's auto-pass key is a pass that came alone (#1091): the server records it
// in the window's outcome and tells it on the closed event to the master and the readier's player, nobody else. The master's
// own pass, with another key, is not.
func TestAReadiedPassWithTheAutoPassKeyIsRecordedAsAlone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		alone bool
	}{{"auto-pass key", true}, {"the master's own tap", false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newArmed(t)
			e := a.readyThenPass(t, nil)
			w := a.mustFire(t, e, "Toren")
			master := listen(t, a.h, a.campaignID, a.master.id, true)
			caio := listen(t, a.h, a.campaignID, a.caio.id, false)
			ana := listen(t, a.h, a.campaignID, a.ana.id, false)
			key := newKey()
			if tc.alone {
				key = autopass.Key(w.GetId())
			}
			a.mustAnswer(t, a.master, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
				passAnswer(r)
				r.IdempotencyKey = key
			})
			var recorded int
			if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM reaction_windows WHERE id = $1 AND (outcome->>'alone') IS NOT DISTINCT FROM $2`, w.GetId(), map[bool]*string{true: new("true"), false: nil}[tc.alone]).Scan(&recorded); err != nil || recorded != 1 {
				t.Errorf("the window's outcome alone = %v: rows %d, err %v", tc.alone, recorded, err)
			}
			for who, s := range map[string]heardBy{"the master": master, "the readier's player": caio} {
				var closed []*playv1.WatchGameSessionResponse_ReactionWindowClosed
				for _, ev := range s.heard(t, a.h) {
					if c := ev.GetReactionWindowClosed(); c != nil {
						closed = append(closed, c)
					}
				}
				if len(closed) != 1 || closed[0].GetWindowId() != w.GetId() || closed[0].GetPassedAlone() != tc.alone {
					t.Errorf("%s heard %v, want one closed event of the window with passed_alone=%v", who, closed, tc.alone)
				}
			}
			for _, ev := range ana.heard(t, a.h) {
				if ev.GetReactionWindowClosed() != nil {
					t.Errorf("a third player heard a window closed: %v", ev)
				}
			}
		})
	}
}

// The reactor's own player sending the window's auto-pass key is not the master's screen passing it by itself: the pass is
// recorded and told as not alone.
func TestAReadiedPassWithTheAutoPassKeyFromThePlayerIsNotAlone(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.readyThenPass(t, nil)
	w := a.mustFire(t, e, "Toren")
	caio := listen(t, a.h, a.campaignID, a.caio.id, false)
	a.mustAnswer(t, a.caio, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		passAnswer(r)
		r.IdempotencyKey = autopass.Key(w.GetId())
	})
	var alone int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM reaction_windows WHERE id = $1 AND (outcome->>'alone') IS NOT NULL`, w.GetId()).Scan(&alone); err != nil || alone != 0 {
		t.Errorf("the window's outcome has alone set: rows %d, err %v", alone, err)
	}
	var closed int
	for _, ev := range caio.heard(t, a.h) {
		if c := ev.GetReactionWindowClosed(); c != nil {
			closed++
			if c.GetPassedAlone() {
				t.Errorf("the player's pass was told as passed alone: %v", c)
			}
		}
	}
	if closed != 1 {
		t.Errorf("the player heard %d closed events, want 1", closed)
	}
}
