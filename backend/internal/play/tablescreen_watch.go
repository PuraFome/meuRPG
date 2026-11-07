package play

import (
	"context"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/live"
)

// The TV tab's stream. It is a hub subscription of the master (it hears what the master hears) that
// forwards no event: each one, however many, becomes a `changed` that carries nothing, and the tab reads
// GetTableScreen again. What the TV shows is therefore only that read, never an event's content.

// screenUserPrefix makes the TV stream's hub user id its own: its streams are capped apart from the
// master's session streams, and a newer TV stream of the same master replaces the oldest.
const screenUserPrefix = "tv:"

// WatchTableScreen implements playv1connect.TableScreenServiceHandler.
func (s *Service) WatchTableScreen(
	ctx context.Context,
	req *connect.Request[playv1.WatchTableScreenRequest],
	stream *connect.ServerStream[playv1.WatchTableScreenResponse],
) error {
	m, err := authz.RequireCampaignRole(ctx, req.Msg.GetCampaignId(), authz.RoleMaster)
	if err != nil {
		return err
	}
	// Subscribe before reading the session: an end after the read is then always delivered here.
	sub, err := s.hub.Subscribe(m.CampaignID, live.Subscriber{UserID: screenUserPrefix + m.UserID, Master: true, Screen: true})
	if err != nil {
		return nil // the server is shutting down: the tab reconnects to the next one
	}
	defer sub.Close()
	session, err := s.openSession(ctx, m.CampaignID)
	if err != nil {
		return err
	}
	defer s.screenLeft(m.CampaignID, session.TvChoiceRevision)
	s.screenJoined(m.CampaignID, session.TvChoiceRevision)
	if stream.Send(&playv1.WatchTableScreenResponse{Event: &playv1.WatchTableScreenResponse_Ready_{Ready: &playv1.WatchTableScreenResponse_Ready{}}}) != nil {
		return nil // the tab went away
	}
	return s.screenLoop(ctx, stream, m.CampaignID, sub, session.ID)
}

// screenLoop sends the TV stream's `changed`, heartbeat and checks until the stream is over.
func (s *Service) screenLoop(ctx context.Context, stream *connect.ServerStream[playv1.WatchTableScreenResponse], campaignID string, sub *live.Subscription, sessionID string) error {
	heartbeat := time.NewTicker(s.live.Heartbeat)
	defer heartbeat.Stop()
	recheck := time.NewTicker(s.live.Recheck)
	defer recheck.Stop()
	lifetime := time.NewTimer(s.live.MaxLifetime)
	defer lifetime.Stop()
	gate := changedGate{after: s.live.ScreenAfter, every: s.live.ScreenInterval}
	// emit sends an event of the stream; false means the tab went away.
	emit := func(ev *playv1.WatchTableScreenResponse) bool { return stream.Send(ev) == nil }
	sendEnded := func() error {
		emit(&playv1.WatchTableScreenResponse{Event: &playv1.WatchTableScreenResponse_SessionEnded_{SessionEnded: &playv1.WatchTableScreenResponse_SessionEnded{}}})
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return nil // the tab closed the stream
		case <-lifetime.C:
			return nil // the tab opens a new one
		case ev, ok := <-sub.Events():
			if done, err := screenHubEvent(sub, ev, ok, sessionID, &gate, sendEnded); done {
				return err
			}
		case <-gate.due:
			gate.sent()
			if !emit(&playv1.WatchTableScreenResponse{Event: &playv1.WatchTableScreenResponse_Changed_{Changed: &playv1.WatchTableScreenResponse_Changed{}}}) {
				return nil
			}
		case <-heartbeat.C:
			if !emit(&playv1.WatchTableScreenResponse{Event: &playv1.WatchTableScreenResponse_Heartbeat_{Heartbeat: &playv1.WatchTableScreenResponse_Heartbeat{}}}) {
				return nil
			}
		case <-recheck.C:
			if done, err := s.recheckTick(ctx, campaignID, sessionID, true, sendEnded); done {
				return err
			}
		}
	}
}

// screenHubEvent takes one event of the hub: it ends the stream with the session, and otherwise only
// starts a `changed`. It says whether the stream is over, and why.
func screenHubEvent(sub *live.Subscription, ev live.Event, ok bool, sessionID string, gate *changedGate, sendEnded func() error) (bool, error) {
	if !ok {
		return true, endOfStream(sub)
	}
	sub.Taken(ev)
	switch {
	case endsSession(ev, sessionID):
		return true, sendEnded()
	case ev.Message.GetSessionEnded() == nil: // the end of another session of the campaign is not this stream's
		gate.hit()
	}
	return false, nil
}

// endsSession says whether ev is the end of the session the stream watches.
func endsSession(ev live.Event, sessionID string) bool {
	return ev.Message.GetSessionEnded().GetGameSession().GetId() == sessionID && ev.Message.GetSessionEnded() != nil
}

// changedGate coalesces the events of a TV stream into at most one `changed` per interval. The first
// event after a `changed` starts the wait; every event during the wait is covered by the `changed` that
// ends it, so the last event is always followed by one.
type changedGate struct {
	after func(time.Duration) <-chan time.Time
	every time.Duration
	// due delivers when the wait ends; nil while no event waits.
	due <-chan time.Time
}

// hit records an event.
func (g *changedGate) hit() {
	if g.due == nil {
		g.due = g.after(g.every)
	}
}

// sent records that the `changed` was sent, so the next event starts a new wait.
func (g *changedGate) sent() { g.due = nil }

// tvPresence counts the TV streams of each campaign on this instance. A connection is not data: the
// count is in memory, like the hub, so with more than one instance a campaign's count is the share
// that this instance holds.
type tvPresence struct {
	// mu orders the count and the message it causes: both happen under it, so the messages of two
	// changes reach the hub in the order the changes were made.
	mu sync.Mutex
	by map[string]*tvCount
	// afterLeft is a test hook: it runs after a TV stream was uncounted and its message sent.
	afterLeft func()
}

// tvCount is one campaign's TV streams, and when the last one left.
type tvCount struct {
	open   int
	lostAt time.Time // zero until a TV was connected and no longer is
}

// snapshot is the presence the master's panel loads with GetTableScreen.
func (p *tvPresence) snapshot(campaignID string) *playv1.TableScreenPresence {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.by[campaignID]
	if c == nil {
		return &playv1.TableScreenPresence{}
	}
	res := &playv1.TableScreenPresence{Open: c.open > 0}
	if c.open == 0 && !c.lostAt.IsZero() {
		res.LostAt = timestamppb.New(c.lostAt)
	}
	return res
}

// screenJoined counts a TV stream; the first one tells the master's streams.
func (s *Service) screenJoined(campaignID string, revision int32) {
	p := &s.screens
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.count(campaignID)
	c.open++
	if c.open == 1 {
		s.publishPresence(campaignID, revision, &playv1.WatchGameSessionResponse_TableScreenChanged{Open: true})
	}
}

// screenLeft uncounts a TV stream; the last one tells the master's streams when it was lost. The
// revision is the stored one, read again (the stream's own ctx may be over); known is the one the stream
// started with, for when the row cannot be read.
func (s *Service) screenLeft(campaignID string, known int32) {
	revision := known
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if session, err := s.queries.GetOpenGameSession(ctx, campaignID); err == nil {
		revision = session.TvChoiceRevision
	}
	p := &s.screens
	p.mu.Lock()
	c := p.count(campaignID)
	c.open--
	if c.open == 0 {
		c.lostAt = s.now()
		s.publishPresence(campaignID, revision, &playv1.WatchGameSessionResponse_TableScreenChanged{LostAt: timestamppb.New(c.lostAt)})
	}
	p.mu.Unlock()
	if p.afterLeft != nil {
		p.afterLeft()
	}
}

func (p *tvPresence) count(campaignID string) *tvCount {
	if p.by == nil {
		p.by = map[string]*tvCount{}
	}
	c := p.by[campaignID]
	if c == nil {
		c = &tvCount{}
		p.by[campaignID] = c
	}
	return c
}

// publishPresence tells the master's session streams that a TV tab connected or left. The TV streams
// are left out: they would answer with a `changed`, and their own coming and going would start a loop.
// The caller holds the presence lock; Publish never waits.
func (s *Service) publishPresence(campaignID string, revision int32, ev *playv1.WatchGameSessionResponse_TableScreenChanged) {
	ev.PresenceKnown, ev.ChoiceRevision = true, revision
	s.hub.Publish(campaignID, live.Event{
		Audience: live.Audience{Master: true, NoScreens: true},
		Message:  &playv1.WatchGameSessionResponse{Event: &playv1.WatchGameSessionResponse_TableScreenChanged_{TableScreenChanged: ev}},
	})
}
