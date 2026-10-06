package play

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PuraFome/meuRPG/backend/internal/play/live"
)

// content_changed (MR-025, RN-23, RN-10): the hint that tells the open screens the
// table's content changed. It carries nothing, goes to everyone, and is throttled per
// campaign like the puzzles' hint. No database: the hub and the gate are all it needs.
func TestRN10_ContentChangedIsAContentFreeThrottledHint(t *testing.T) {
	t.Parallel()
	hub := live.New(0)
	s := &Service{hub: hub}
	gc := &gateClock{t: time.Unix(1_000_000, 0)}
	s.puzzles.hints.now, s.puzzles.hints.after = gc.now, gc.after
	master, err := hub.Subscribe("camp-1", live.Subscriber{UserID: "m", Master: true})
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	player, err := hub.Subscribe("camp-1", live.Subscriber{UserID: "p"})
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	stranger, err := hub.Subscribe("camp-2", live.Subscriber{UserID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	defer stranger.Close()

	drain := func(sub *live.Subscription) []live.Event {
		var out []live.Event
		for {
			select {
			case ev := <-sub.Events():
				sub.Taken(ev)
				out = append(out, ev)
			default:
				return out
			}
		}
	}
	s.PublishContentChanged("camp-1")
	for who, sub := range map[string]*live.Subscription{"master": master, "player": player} {
		got := drain(sub)
		if len(got) != 1 {
			t.Fatalf("%s got %d events, want the hint at once", who, len(got))
		}
		hint := got[0].Message
		if hint.GetContentChanged() == nil || proto.Size(hint.GetContentChanged()) != 0 {
			t.Errorf("%s got %v, want an empty content_changed", who, hint)
		}
		if !got[0].Audience.Everyone {
			t.Errorf("%s: the hint is not for everyone", who)
		}
	}
	if got := drain(stranger); len(got) != 0 {
		t.Errorf("another campaign's stream got %d events", len(got))
	}
	// A burst inside the interval is held, then one hint goes out when it ends.
	for range 5 {
		gc.advance(20 * time.Millisecond)
		s.PublishContentChanged("camp-1")
	}
	if got := drain(player); len(got) != 0 {
		t.Fatalf("a burst sent %d hints, want it held", len(got))
	}
	gc.advance(defaultHintEvery)
	if got := drain(player); len(got) != 1 {
		t.Fatalf("after the interval the player got %d hints, want the burst merged into one", len(got))
	}
}
