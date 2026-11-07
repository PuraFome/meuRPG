package play

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
)

// send is the one place in this package that hands an event to the live hub:
// every other file tells the streams through it (the archtest in
// internal/archtest/publish_test.go fails for a call to Publish anywhere else).
// It takes the hub's arguments and passes them on unchanged, and, like the hub,
// it takes no context and waits for nobody; it reports how many subscriptions
// received the event.
func (s *Service) send(campaignID string, ev live.Event) int {
	return s.hub.Publish(campaignID, ev)
}

// tellMembers sends ev to the master's streams, always, and to the players'
// only when players is true. It is the body of the exported Publish (the
// maps module's seam) and the call of the play files that tell the same two
// audiences.
func (s *Service) tellMembers(campaignID string, players bool, ev *playv1.WatchGameSessionResponse) {
	s.send(campaignID, live.Event{Audience: live.Audience{Master: true, Everyone: players}, Message: ev})
}

// hints collects, during one attempt of a transaction, what the write will tell the live streams once it is saved.
// Service.tx makes it and hands it to the closure; nothing in it is sent before the commit.
type hints struct {
	events  []told
	touched bool
	// seq is the session's hint_seq as this transaction left it: set by Service.tx after it bumped the number, 0 while it did not.
	// txOn stamps the events with it (WatchGameSessionResponse.session_seq) just before it sends them.
	seq int64
}

// told is one collected event and the campaign whose streams receive it.
type told struct {
	campaignID string
	event      live.Event
}

// tell collects ev for the streams of campaignID. It is sent, through send, after the transaction commits, and never if the
// transaction fails. The closure may call it as often as it likes: the session's number moves by one however many it tells.
func (h *hints) tell(campaignID string, ev live.Event) {
	h.events = append(h.events, told{campaignID: campaignID, event: ev})
}

// touch marks a write that sends no event but still changes what the screens show, so the session's number moves and a
// stream that missed the change knows it is behind.
func (h *hints) touch() { h.touched = true }

// reset empties the collector: the start of every attempt calls it, so a retried attempt keeps nothing of the one the database rolled back.
func (h *hints) reset() { *h = hints{} }

// moves reports whether the write must move the session's number.
func (h *hints) moves() bool { return h.touched || len(h.events) > 0 }

// tx runs fn in a transaction (db.InTx, so with its retries and its rules for fn) on the session sessionID and tells the
// streams what fn collected, after the commit.
//
// The first statement of every attempt takes the session's row lock (FOR UPDATE), before fn reads anything: all the writes of
// one session take turns in that order, so they do not collide at the bump, and none can deadlock on it. fn must not lock
// the row again in another order; a closure that locks it itself (GetOpenGameSessionForUpdate) finds it already held.
// fn calls h.tell for each hint its write sends and h.touch for a write that sends none but must still move the screens.
// The collector is empty at the start of every attempt, whatever the last attempt collected.
// When fn returns nil having told or touched, tx adds 1 to game_sessions.hint_seq of sessionID in the same transaction
// (once, however many events were told), so the number is saved with the write or not at all; the session's row must exist.
// A transaction that told nothing and touched nothing takes the lock and writes nothing: the number stays as it was.
// When InTx returns nil, tx stamps each collected event with the new number and sends them, in the order they were told. On any error, nothing is sent.
//
// A hint that goes with no write (typing, presence, the table screen's "changed") does not go through tx: it calls send.
func (s *Service) tx(ctx context.Context, sessionID string, fn func(pgx.Tx, *hints) error) error {
	return s.txOn(ctx, s.pool, sessionID, fn)
}

// txOn is tx on any transaction starter: a test passes one whose COMMIT fails.
func (s *Service) txOn(ctx context.Context, starter db.TxStarter, sessionID string, fn func(pgx.Tx, *hints) error) error {
	var h hints
	err := db.InTx(ctx, starter, func(tx pgx.Tx) error {
		h.reset()
		q := s.queries.WithTx(tx)
		if _, err := q.LockSessionForHints(ctx, sessionID); err != nil {
			return fmt.Errorf("lock the session: %w", err)
		}
		if err := fn(tx, &h); err != nil {
			return err
		}
		if !h.moves() {
			return nil
		}
		seq, err := q.BumpHintSeq(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("bump the session's hint number: %w", err)
		}
		h.seq = seq
		return nil
	})
	if err != nil {
		return err
	}
	for _, t := range h.events {
		if h.seq > 0 && t.event.Message != nil {
			t.event.Message.SessionSeq = h.seq // not shared with a stream yet: send is next
		}
		s.send(t.campaignID, t.event)
	}
	return nil
}
