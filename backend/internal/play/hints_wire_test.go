package play

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
)

// The stream carries the session's hint number: on Ready, on every heartbeat (read from the database), and on the events a
// counted write sends.
func TestWatchStampsReadyHeartbeatAndEventsWithTheHintNumber(t *testing.T) {
	t.Parallel()
	h := newHarness(t, LiveConfig{Heartbeat: 20 * time.Millisecond})
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master, "Mirathel")
	session := master.start(t, campaign).GetGameSession()
	setSeq := func(n int) {
		t.Helper()
		if _, err := h.pool.Exec(t.Context(), `UPDATE game_sessions SET hint_seq = $2 WHERE id = $1`, session.GetId(), n); err != nil {
			t.Fatal(err)
		}
	}
	setSeq(7)

	stream := master.watch(t, campaign)
	ev := stream.next(t)
	if ev.GetReady() == nil || ev.GetSessionSeq() != 7 || ev.GetReady().GetSessionSeq() != 7 {
		t.Fatalf("first event = %v, want ready with the number 7 on both", ev)
	}

	// The number moves in the database; a heartbeat carries the new one (no sleep: the next heartbeats are read until it does).
	setSeq(9)
	for {
		ev := stream.next(t)
		hb := ev.GetHeartbeat()
		if hb == nil {
			t.Fatalf("event = %v, want only heartbeats", ev)
		}
		if hb.GetSessionSeq() != ev.GetSessionSeq() {
			t.Fatalf("heartbeat number %d, stamp %d: want the same", hb.GetSessionSeq(), ev.GetSessionSeq())
		}
		if hb.GetSessionSeq() == 9 {
			break
		}
		if hb.GetSessionSeq() != 7 {
			t.Fatalf("heartbeat number = %d, want 7 or 9", hb.GetSessionSeq())
		}
	}

	// A counted write stamps its event with the number it left; an event sent without a write has none.
	err := h.svc.tx(t.Context(), session.GetId(), func(_ pgx.Tx, hints *hints) error {
		hints.tell(campaign, live.Event{
			Audience: live.Audience{Master: true},
			Message:  &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_ContentChanged_{ContentChanged: &playv1.WatchGameSessionResponse_ContentChanged{}}},
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h.svc.PublishContentChanged(campaign)
	var stamped, plain []int64
	for len(stamped)+len(plain) < 2 {
		ev := stream.next(t)
		switch {
		case ev.GetHeartbeat() != nil:
		case len(stamped) == 0:
			stamped = append(stamped, ev.GetSessionSeq())
		default:
			plain = append(plain, ev.GetSessionSeq())
		}
	}
	if stamped[0] != 10 || plain[0] != 0 {
		t.Errorf("the counted event carried %d (want 10) and the uncounted one %d (want 0)", stamped[0], plain[0])
	}
}

// ListCombatLog with since_seq: the new and changed lines only, the highest number to ask from next, and the whole log again
// after an undo.
func TestListCombatLogSinceSeq(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	a.h.roller.queue(15, 3)
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", inAppRoll)

	first := a.log(t, a.master, e)
	entries := func(l *playv1.ListCombatLogResponse) (out []*playv1.CombatLogEntry) {
		for _, r := range l.GetRounds() {
			out = append(out, r.GetEntries()...)
		}
		return out
	}
	if !first.GetFull() || first.GetLastSeq() == 0 || len(entries(first)) == 0 {
		t.Fatalf("the whole log: full = %v, last_seq = %d, %d entries; want full, a number and entries", first.GetFull(), first.GetLastSeq(), len(entries(first)))
	}
	for _, en := range entries(first) {
		if en.GetSeq() == 0 || en.GetSeq() > first.GetLastSeq() {
			t.Errorf("entry %s has seq %d, want 1 to %d", en.GetId(), en.GetSeq(), first.GetLastSeq())
		}
	}
	since := func(u *user, n int64) *playv1.ListCombatLogResponse {
		t.Helper()
		res, err := u.combat.ListCombatLog(t.Context(), connect.NewRequest(&playv1.ListCombatLogRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), SinceSeq: n}))
		if err != nil {
			t.Fatalf("ListCombatLog(since %d) error = %v", n, err)
		}
		return res.Msg
	}

	// Nothing new: no entries, the same number, not full. (The entry the master can undo is always sent.)
	again := since(a.caio, first.GetLastSeq())
	if len(entries(again)) != 0 || again.GetFull() || again.GetLastSeq() != first.GetLastSeq() {
		t.Errorf("nothing new, as a player: %d entries, full = %v, last_seq = %d; want 0, false, %d", len(entries(again)), again.GetFull(), again.GetLastSeq(), first.GetLastSeq())
	}

	// The damage joins the attack's entry: it comes again with a higher number, alone.
	a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), inAppDamage)
	delta := since(a.caio, first.GetLastSeq())
	got := entries(delta)
	if delta.GetFull() || len(got) != 1 || got[0].GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK || got[0].GetSeq() <= first.GetLastSeq() || delta.GetLastSeq() < got[0].GetSeq() {
		t.Fatalf("after the damage: full = %v, entries = %v, last_seq = %d; want only the attack's entry, with a higher seq", delta.GetFull(), got, delta.GetLastSeq())
	}
	if all := a.log(t, a.caio, e); len(entries(all)) < 2 {
		t.Errorf("the whole log has %d entries, want the delta to be a part of more", len(entries(all)))
	}

	// An undo takes lines back: the answer is the whole log again.
	l := a.log(t, a.master, e)
	if err := a.undo(t, a.master, e, l.GetUndoableEventId()); err != nil {
		t.Fatal(err)
	}
	after := since(a.master, l.GetLastSeq())
	if !after.GetFull() || after.GetLastSeq() <= l.GetLastSeq() {
		t.Errorf("after an undo: full = %v, last_seq = %d (was %d); want full and a higher number", after.GetFull(), after.GetLastSeq(), l.GetLastSeq())
	}
	if len(entries(after)) != len(entries(a.log(t, a.master, e))) {
		t.Errorf("the full answer has %d entries, the whole log %d", len(entries(after)), len(entries(a.log(t, a.master, e))))
	}
}

// An undo tells the streams the log was rewritten (full); an ordinary change does not.
func TestCombatLogChangedIsFullAfterAnUndo(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.threeAndAGoblin(t)
	stream := a.master.watch(t, a.campaignID)
	stream.ready(t)
	a.h.roller.queue(15, 3)
	a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", inAppRoll)
	logChange := func() *playv1.WatchGameSessionResponse_CombatLogChanged {
		t.Helper()
		for {
			if c := stream.next(t).GetCombatLogChanged(); c != nil {
				return c
			}
		}
	}
	if c := logChange(); c.GetFull() {
		t.Error("an attack told the streams the log was rewritten")
	}
	if err := a.undo(t, a.master, e, a.log(t, a.master, e).GetUndoableEventId()); err != nil {
		t.Fatal(err)
	}
	if c := logChange(); !c.GetFull() {
		t.Error("an undo did not tell the streams the log was rewritten")
	}
}
