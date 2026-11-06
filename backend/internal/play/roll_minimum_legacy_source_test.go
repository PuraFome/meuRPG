package play

import (
	"encoding/json"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A roll stored before the table minimum has counted_as and no counted_by: only Reliable Talent could have raised it, so
// every reader of a stored countedRoll names that feature as the source.
func TestStoredCountedRollsWithoutASourceNameReliableTalent(t *testing.T) {
	t.Parallel()
	const stored = `{"counted_as":10}`
	decode := func(v any) {
		t.Helper()
		if err := json.Unmarshal([]byte(stored), v); err != nil {
			t.Fatalf("decode %T: %v", v, err)
		}
	}
	var contest contestRoll
	decode(&contest)
	var ev actionEvent
	decode(&ev)
	var rider riderEvent
	decode(&rider)
	var channel channelEvent
	if err := json.Unmarshal([]byte(`{"turns":[{"counted_as":10}]}`), &channel); err != nil {
		t.Fatalf("decode a channel: %v", err)
	}
	var save playv1.CombatLogHitRider
	riderNumbersProto(&save, &rider, false)
	checks := map[string]string{
		"contest side":     contest.proto().GetTreatedAsSource(),
		"attack or save":   ev.mark(&playv1.DiceRoll{}).GetTreatedAsSource(),
		"hit rider":        save.GetTreatedAsSource(),
		"channel divinity": channelProto(&channel).GetTurns()[0].GetTreatedAsSource(),
	}
	for what, got := range checks {
		if got != reliableTalentSource {
			t.Errorf("%s names %q as the source of a stored counted_as with no counted_by, want %q", what, got, reliableTalentSource)
		}
	}
}
