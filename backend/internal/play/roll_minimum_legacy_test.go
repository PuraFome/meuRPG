package play

import (
	"encoding/json"
	"testing"
	"time"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// Rolls stored before the table minimum (#757) kept no counted_by.
// Any total that differed from face plus modifier then was Reliable Talent's, and the roll still says so.

func wantLegacyTreated(t *testing.T, what string, r *playv1.DiceRoll, counted int32, source string) {
	t.Helper()
	if r.TreatedAs == nil || r.GetTreatedAs() != counted || r.GetTreatedAsSource() != source {
		t.Errorf("%s = %v, want treated as %d by %q", what, r, counted, source)
	}
}

func TestALegacyStoredSceneRollNamesReliableTalent(t *testing.T) {
	t.Parallel()
	var ev sceneRollEvent
	if err := json.Unmarshal([]byte(`{"d20":3,"modifier":4,"total":14}`), &ev); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	out := sceneRollToProto("id", "c", "Toren", time.Time{}, ev, true)
	wantLegacyTreated(t, "the scene roll", out.GetRoll(), 10, reliableTalentSource)
	if err := json.Unmarshal([]byte(`{"d20":13,"modifier":4,"total":17}`), &ev); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := sceneRollToProto("id", "c", "Toren", time.Time{}, ev, true).GetRoll(); got.TreatedAs != nil {
		t.Errorf("a roll that counted as it came up = %v, want no mention of a feature", got)
	}
}

func TestALegacyStoredHintTryNamesReliableTalent(t *testing.T) {
	t.Parallel()
	single := hintRollProto(playdb.PuzzleHintTry{D20: 3, Modifier: 4, Total: 14})
	wantLegacyTreated(t, "the hint roll", single, 10, reliableTalentSource)
	pair := hintRollProto(playdb.PuzzleHintTry{D20: 3, D20B: 6, Counted: 1, Modifier: 4, Total: 14})
	wantLegacyTreated(t, "the hint pair", pair, 10, reliableTalentSource)
}

func TestALegacyStoredPairSearchNamesReliableTalent(t *testing.T) {
	t.Parallel()
	var ev actionEvent
	if err := json.Unmarshal([]byte(`{"d20":3,"d20_b":6,"counted":1,"modifier":4,"total":14,"roll_mode":"advantage"}`), &ev); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	out := trapSearchAnswer(&trapSearch{ev: ev})
	wantLegacyTreated(t, "the pair with no counted_by", out.GetRoll(), 10, reliableTalentSource)
}

// A search stored with reliable_talent and a roll mode replays the source through floor(), not through floor_by.
func TestAStoredReliableTalentPairSearchReplaysTreatedAs(t *testing.T) {
	t.Parallel()
	var ev actionEvent
	if err := json.Unmarshal([]byte(`{"d20":3,"d20_b":6,"counted":1,"modifier":4,"total":14,"roll_mode":"advantage","reliable_talent":true}`), &ev); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	out := trapSearchAnswer(&trapSearch{ev: ev})
	wantLegacyTreated(t, "the pair", out.GetRoll(), 10, reliableTalentSource)
}
