package play

import (
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
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
