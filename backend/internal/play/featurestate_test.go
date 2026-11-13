package play

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFeatureStateReadsBadDocumentsAsEmpty(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "null", "not json", "[]", `"x"`, `{"a":1}`} {
		if got := readFeatureState([]byte(raw)); len(got) != 0 {
			t.Errorf("readFeatureState(%q) = %v, want empty", raw, got)
		}
	}
	if got := readFeatureState(nil); got == nil {
		t.Error("readFeatureState(nil) = nil, want a map that can be written")
	}
}

func TestFeatureStateRoundTrip(t *testing.T) {
	t.Parallel()
	now := stateNow{Turn: "2:x", Round: 2}
	fs := featureState{}
	fs.set("a", now.turnEntry())
	fs.set("b", featureEntry{S: scopeRound, At: "2", N: 2, C: "fire"})
	again := readFeatureState(fs.bytes())
	if len(again) != 2 || again["a"] != now.turnEntry() || again["b"] != (featureEntry{S: scopeRound, At: "2", N: 2, C: "fire"}) {
		t.Errorf("round trip = %v", again)
	}
	if got := string(featureState{}.bytes()); got != "{}" {
		t.Errorf("empty state = %s, want {}", got)
	}
	if got, _ := json.Marshal(now.turnEntry()); string(got) != `{"s":"turn","at":"2:x","n":1}` {
		t.Errorf("entry = %s, want no c when there is no choice", got)
	}
}

func TestFeatureStateEntriesExpireByTheirMark(t *testing.T) {
	t.Parallel()
	now := stateNow{Turn: "3:abc", Round: 3}
	fs := featureState{
		"turn":       {S: scopeTurn, At: "3:abc", N: 1},
		"turnOld":    {S: scopeTurn, At: "3:def", N: 1},
		"round":      {S: scopeRound, At: "3", N: 1},
		"roundOld":   {S: scopeRound, At: "2", N: 1},
		"combat":     {S: scopeCombat, N: 1},
		"noUses":     {S: scopeCombat, N: 0},
		"strangeKey": {S: "rest", At: "", N: 1},
	}
	for key, want := range map[string]bool{"turn": true, "turnOld": false, "round": true, "roundOld": false, "combat": true, "noUses": false, "strangeKey": false, "absent": false} {
		if _, got := fs.live(key, now); got != want {
			t.Errorf("live(%q) = %v, want %v", key, got, want)
		}
	}
	// The next turn of the same round: the turn's entry is stale, the round's is not.
	next := stateNow{Turn: "3:def", Round: 3}
	if _, ok := fs.live("turn", next); ok {
		t.Error("an entry of the turn is live in the next turn")
	}
	if _, ok := fs.live("round", next); !ok {
		t.Error("an entry of the round is stale in the same round")
	}
	fs.prune(now)
	if len(fs) != 3 || !fs.used("turn", now) || !fs.used("round", now) || !fs.used("combat", now) {
		t.Errorf("after prune = %v, want the three live ones", fs)
	}
}

func TestFeatureStateLimits(t *testing.T) {
	t.Parallel()
	fs := featureState{}
	for i := range featureStateMaxKeys {
		fs.set(strings.Repeat("k", 10)+string(rune('A'+i/26))+string(rune('a'+i%26)), featureEntry{S: scopeCombat, N: 1})
	}
	if err := fs.validate(); err != nil {
		t.Fatalf("a document of %d keys is refused: %v", featureStateMaxKeys, err)
	}
	fs.set("one-too-many", featureEntry{S: scopeCombat, N: 1})
	if fs.validate() == nil {
		t.Error("a document of 65 keys is accepted")
	}
	if (featureState{strings.Repeat("k", featureStateMaxKey): {S: scopeCombat, N: 1}}).validate() != nil {
		t.Error("a key of the longest length is refused")
	}
	if (featureState{strings.Repeat("k", featureStateMaxKey+1): {S: scopeCombat, N: 1}}).validate() == nil {
		t.Error("a key of 81 characters is accepted")
	}
	if (featureState{"": {S: scopeCombat, N: 1}}).validate() == nil {
		t.Error("an empty key is accepted")
	}
	big := featureState{"a": {S: scopeCombat, N: 1, C: strings.Repeat("x", featureStateMaxBytes)}}
	if big.validate() == nil {
		t.Error("a document over the size is accepted")
	}
}
