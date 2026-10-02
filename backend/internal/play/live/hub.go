// Package live fans the live session's events out to the members watching
// it (PlayService.WatchGameSession, ADR-0005).
//
// A Hub keeps, per campaign, the subscriptions of the members who have the
// session open. Whoever changes something at the table (package play)
// publishes an Event with its Audience: who may receive it. The hub hands
// each event only to the subscriptions in its audience, so a player never
// receives what only the master may see (RN-10, RN-11): the filter runs on
// the server, before anything is sent.
//
// Publishing never waits for a subscriber. Each subscription has a small
// buffer; one whose buffer is full (the app stopped reading, or the
// network stalled) is dropped: its channel closes, its stream ends, and the
// app reconnects and reads the snapshot again. A slow phone never holds the
// master's change back.
//
// The hub lives in the server's memory. With more than one server
// instance, a change made on one instance would never reach the streams
// open on another, so the service runs with Cloud Run max-instances = 1
// while fan-out stays in memory (docs/operacao.md). A shared channel, such
// as CockroachDB changefeeds or Pub/Sub, replaces it when one instance is no
// longer enough.
package live

import (
	"errors"
	"sync"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// DefaultBuffer is how many events a subscription holds before it is
// dropped as too slow. Changes at a table come a few per minute; 16 is
// plenty for a burst, and small enough to notice a stream nobody reads.
const DefaultBuffer = 16

// Why a subscription ended. Err returns one of these once Events is closed.
var (
	// ErrSlow: the subscriber did not keep up and was dropped.
	ErrSlow = errors.New("live: the subscriber fell behind and was dropped")
	// ErrClosed: the hub closed, because the server is shutting down.
	ErrClosed = errors.New("live: the hub is closed")
)

// Subscriber is who watches: their account, and whether they are the
// campaign's master.
type Subscriber struct {
	UserID string
	Master bool
}

// Audience says who may receive an event. The zero Audience reaches nobody.
type Audience struct {
	// Everyone watching the campaign: the master and every player.
	Everyone bool
	// The campaign's master.
	Master bool
	// Every player of the campaign, not the master: for an event the master
	// gets in another form (a combat's turn, where a hidden combatant's name
	// is the master's alone).
	Players bool
	// One more user, such as the player of the character the event is
	// about. Empty for none.
	UserID string
}

// includes reports whether s may receive an event for this audience.
func (a Audience) includes(s Subscriber) bool {
	return a.Everyone || (a.Master && s.Master) || (a.Players && !s.Master) || (a.UserID != "" && a.UserID == s.UserID)
}

// Event is one live change, and who may receive it.
type Event struct {
	Audience Audience
	// Message is what the stream sends. It must not be changed after
	// Publish: several streams send the same message.
	Message *playv1.WatchGameSessionResponse
}

// Hub is the in-memory fan-out. The zero Hub is not usable: call New.
type Hub struct {
	buffer int

	mu     sync.Mutex
	subs   map[string]map[*Subscription]struct{} // by campaign ID
	closed bool
}

// New returns a Hub whose subscriptions hold buffer events each (0 means
// DefaultBuffer).
func New(buffer int) *Hub {
	if buffer <= 0 {
		buffer = DefaultBuffer
	}
	return &Hub{buffer: buffer, subs: map[string]map[*Subscription]struct{}{}}
}

// Subscription is one member watching one campaign. Read Events until it
// closes, then Err says why; call Close when done watching.
type Subscription struct {
	hub        *Hub
	campaignID string
	who        Subscriber
	events     chan Event
	err        error // set, under hub.mu, before events closes
}

// Subscribe starts delivering the campaign's events for who. It fails with
// ErrClosed once the hub is closed.
func (h *Hub) Subscribe(campaignID string, who Subscriber) (*Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrClosed
	}
	s := &Subscription{hub: h, campaignID: campaignID, who: who, events: make(chan Event, h.buffer)}
	if h.subs[campaignID] == nil {
		h.subs[campaignID] = map[*Subscription]struct{}{}
	}
	h.subs[campaignID][s] = struct{}{}
	return s, nil
}

// Publish hands ev to every subscription of the campaign in its audience,
// without waiting: a subscription whose buffer is full is dropped
// (ErrSlow). It reports how many subscriptions received the event.
func (h *Hub) Publish(campaignID string, ev Event) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	delivered := 0
	for s := range h.subs[campaignID] {
		if !ev.Audience.includes(s.who) {
			continue
		}
		select {
		case s.events <- ev:
			delivered++
		default:
			h.remove(s, ErrSlow)
		}
	}
	return delivered
}

// Close ends every subscription (ErrClosed) and refuses new ones. The
// server calls it when its graceful shutdown starts, so open streams end
// right away instead of holding the shutdown until its deadline. Closing
// twice is fine.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for _, subs := range h.subs {
		for s := range subs {
			h.remove(s, ErrClosed)
		}
	}
}

// Count returns how many subscriptions the campaign has, for tests and
// logs.
func (h *Hub) Count(campaignID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[campaignID])
}

// remove drops s and closes its channel, with err as the reason. The
// caller holds h.mu; sending and closing both happen under it, so nothing
// ever sends on a closed channel.
func (h *Hub) remove(s *Subscription, err error) {
	subs := h.subs[s.campaignID]
	if _, ok := subs[s]; !ok {
		return // already removed
	}
	delete(subs, s)
	if len(subs) == 0 {
		delete(h.subs, s.campaignID)
	}
	s.err = err
	close(s.events)
}

// Events delivers the subscription's events. It closes when the
// subscription ends: Close, too slow, or the hub closed.
func (s *Subscription) Events() <-chan Event { return s.events }

// Err says why Events closed: ErrSlow, ErrClosed, or nil after Close. Call
// it only after Events has closed.
func (s *Subscription) Err() error {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	return s.err
}

// Close ends the subscription. Closing twice, or after the hub dropped it,
// is fine.
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	s.hub.remove(s, nil)
}
