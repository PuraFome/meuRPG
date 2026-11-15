package play

import (
	"encoding/json"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The undo does not take back an action whose change threw a rider off its mount with no window, whatever the change wrote between the
// fall and its own event.
func TestUndo_AForcedFallClosesTheChainWhateverFollowsItInTheChange(t *testing.T) {
	t.Parallel()
	row := func(id, kind string, ev actionEvent) playdb.ListRecentSessionEventsRow {
		p, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		return playdb.ListRecentSessionEventsRow{ID: id, Kind: kind, Payload: p}
	}
	fall := row("fall", eventRiderThrown, actionEvent{Reaction: &reactionEvent{}})
	answered := row("answered", eventRiderThrown, actionEvent{Reaction: &reactionEvent{Window: "w1"}})
	own := row("own", eventHitPointsAdjusted, actionEvent{})
	earlier := row("earlier", eventHitPointsAdjusted, actionEvent{})
	other := row("other", eventMountSaveRolled, actionEvent{})
	for _, tt := range []struct {
		name   string
		recent []playdb.ListRecentSessionEventsRow
		want   chainVerdict
	}{
		{"the fall just before", []playdb.ListRecentSessionEventsRow{own, fall}, chainClose},
		{"another event of the change between", []playdb.ListRecentSessionEventsRow{own, other, fall}, chainClose},
		{"a fall of an earlier action", []playdb.ListRecentSessionEventsRow{own, earlier, fall}, chainGoOn},
		{"a fall that was answered", []playdb.ListRecentSessionEventsRow{own, other, answered}, chainGoOn},
		{"no fall", []playdb.ListRecentSessionEventsRow{own, other}, chainGoOn},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := chainForcedFall(chainEvent{recent: tt.recent, row: own}); got != tt.want {
				t.Errorf("chainForcedFall() = %v, want %v", got, tt.want)
			}
		})
	}
}
