package cutting_test

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/cutting"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

func TestAsks(t *testing.T) {
	t.Parallel()
	base := cutting.Bard{Feature: true, Ask: "all", HitPoints: 7, UsesLeft: 2}
	tests := []struct {
		name string
		edit func(*cutting.Bard)
		want bool
	}{
		{"a Lore bard that asks about everything", func(*cutting.Bard) {}, true},
		{"asks about attacks only", func(b *cutting.Bard) { b.Ask = "attacks" }, false},
		{"never asks", func(b *cutting.Bard) { b.Ask = "never" }, false},
		{"an empty setting is the default, attacks", func(b *cutting.Bard) { b.Ask = "" }, false},
		{"no Cutting Words", func(b *cutting.Bard) { b.Feature = false }, false},
		{"no use of Bardic Inspiration left", func(b *cutting.Bard) { b.UsesLeft = 0 }, false},
		{"at 0 hit points", func(b *cutting.Bard) { b.HitPoints = 0 }, false},
		{"with the last use", func(b *cutting.Bard) { b.UsesLeft = 1 }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := base
			tt.edit(&b)
			if got := b.Asks(); got != tt.want {
				t.Errorf("Asks() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMayCut: the use left is not asked at the answer (the ask already took it into account); the setting and the hit points are.
func TestMayCut(t *testing.T) {
	t.Parallel()
	base := cutting.Bard{Feature: true, Ask: "all", HitPoints: 7}
	tests := []struct {
		name string
		edit func(*cutting.Bard)
		want bool
	}{
		{"a Lore bard that asks about everything", func(*cutting.Bard) {}, true},
		{"no use left is for Asks, not for MayCut", func(b *cutting.Bard) { b.UsesLeft = 0 }, true},
		{"the setting changed to attacks only", func(b *cutting.Bard) { b.Ask = "attacks" }, false},
		{"the setting changed to never", func(b *cutting.Bard) { b.Ask = "never" }, false},
		{"fell to 0 hit points", func(b *cutting.Bard) { b.HitPoints = 0 }, false},
		{"no Cutting Words", func(b *cutting.Bard) { b.Feature = false }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := base
			tt.edit(&b)
			if got := b.MayCut(); got != tt.want {
				t.Errorf("MayCut() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDie(t *testing.T) {
	t.Parallel()
	for level, want := range map[int]int{1: 6, 4: 6, 5: 8, 9: 8, 10: 10, 14: 10, 15: 12, 20: 12} {
		if got := cutting.Die(level); got != want {
			t.Errorf("Die(%d) = %d, want %d", level, got, want)
		}
	}
}

func TestSightOnTheMap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		sight     cutting.Sight
		wantSees  bool
		wantNamed bool
	}{
		{"both tokens, 60 feet, in the open", cutting.Sight{Mapped: true, DistanceFt: 60, Seen: true}, true, true},
		{"both tokens, 65 feet", cutting.Sight{Mapped: true, DistanceFt: 65, Seen: true}, false, false},
		{"close but in the dark of the fog", cutting.Sight{Mapped: true, DistanceFt: 10, Seen: false}, false, false},
		{"close but the token is hidden", cutting.Sight{Mapped: true, DistanceFt: 10, Seen: true, TokenHidden: true}, false, false},
		{"the box says yes and the map says no: the map wins", cutting.Sight{Mapped: true, DistanceFt: 100, Seen: true, Box: true}, false, false},
		{"the box says no and the map says yes: the map wins", cutting.Sight{Mapped: true, DistanceFt: 5, Seen: true}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.sight.Sees(); got != tt.wantSees {
				t.Errorf("Sees() = %v, want %v", got, tt.wantSees)
			}
			if got := tt.sight.Named(); got != tt.wantNamed {
				t.Errorf("Named() = %v, want %v", got, tt.wantNamed)
			}
		})
	}
}

func TestSightWithoutAMap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		sight     cutting.Sight
		wantSees  bool
		wantNamed bool
	}{
		{"the master's box is off", cutting.Sight{}, false, false},
		{"the box is on, the creature is off the stage", cutting.Sight{Box: true}, true, false},
		{"the box is on, the creature is on the stage for the bard", cutting.Sight{Box: true, OnStage: true}, true, true},
		{"the box is off, the creature is on the stage", cutting.Sight{OnStage: true}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.sight.Sees(); got != tt.wantSees {
				t.Errorf("Sees() = %v, want %v", got, tt.wantSees)
			}
			if got := tt.sight.Named(); got != tt.wantNamed {
				t.Errorf("Named() = %v, want %v", got, tt.wantNamed)
			}
		})
	}
}

func TestLabel(t *testing.T) {
	t.Parallel()
	if got := cutting.Label("Capitão Brann", true); got != "Capitão Brann" {
		t.Errorf("Label(named) = %q", got)
	}
	if got := cutting.Label("Capitão Brann", false); got != "uma criatura" {
		t.Errorf("Label(unnamed) = %q, want uma criatura", got)
	}
}

func TestSettle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		total     int
		cuts      []int
		wantTotal int
		wantCut   int
	}{
		{"nobody cut", 15, nil, 15, 0},
		{"one die", 15, []int{4}, 11, 4},
		{"two dice come off one after the other", 15, []int{4, 3}, 8, 7},
		{"the total may go below zero: it is a check, not a damage", 2, []int{6}, -4, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			total, cut := cutting.Settle(tt.total, tt.cuts)
			if total != tt.wantTotal || cut != tt.wantCut {
				t.Errorf("Settle(%d, %v) = %d, %d; want %d, %d", tt.total, tt.cuts, total, cut, tt.wantTotal, tt.wantCut)
			}
		})
	}
}

func TestCurrent(t *testing.T) {
	t.Parallel()
	asks := []cutting.State{cutting.Cut, cutting.Waiting, cutting.Waiting}
	if got := cutting.Current(asks); got != 1 {
		t.Errorf("Current(%v) = %d, want 1: the first question still open, in order", asks, got)
	}
	if got := cutting.Current([]cutting.State{cutting.Cut, cutting.Passed, cutting.Closed}); got != -1 {
		t.Errorf("Current(all answered) = %d, want -1", got)
	}
	if got := cutting.Current(nil); got != -1 {
		t.Errorf("Current(nil) = %d, want -1", got)
	}
}

func TestChecks(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"skill:deception", "skill:persuasion", "skill:intimidation", "ability:cha", "ability:str", "skill:sleight-of-hand"} {
		if !cutting.ValidCheck(key) {
			t.Errorf("ValidCheck(%q) = false", key)
		}
	}
	for _, key := range []string{"", "save:wis", "skill:", "skill:bogus", "ability:luck", "monster:bandit", "initiative"} {
		if cutting.ValidCheck(key) {
			t.Errorf("ValidCheck(%q) = true", key)
		}
	}
	keys := cutting.CheckKeys()
	if len(keys) != 24 || !slices.IsSorted(keys[6:]) {
		t.Errorf("CheckKeys() = %d keys %v, want the six abilities and the eighteen skills in alphabetical order", len(keys), keys)
	}
	if keys[0] != "ability:str" || keys[5] != "ability:cha" {
		t.Errorf("CheckKeys() starts with %v, want the abilities in sheet order", keys[:6])
	}
}

func TestParseState(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]cutting.State{
		cutting.StateAsked: cutting.Waiting, cutting.StateCut: cutting.Cut, cutting.StatePassed: cutting.Passed, cutting.StateClosed: cutting.Closed,
		"": cutting.Waiting, "unknown": cutting.Waiting,
	} {
		if got := cutting.ParseState(in); got != want {
			t.Errorf("ParseState(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestPlaceSightOf(t *testing.T) {
	t.Parallel()
	yes := func(grid.Square) bool { return true }
	both := cutting.Place{NPC: "npc", Tokens: map[string]cutting.Token{
		"npc":  {Sq: grid.Square{Col: 10, Row: 0}, Hidden: true},
		"bard": {Sq: grid.Square{Col: 0, Row: 0}},
	}}
	got := both.SightOf("bard", true, yes, true)
	if !got.Mapped || got.DistanceFt != 50 || !got.Seen || !got.TokenHidden || got.Sees() {
		t.Errorf("both tokens = %+v, want the map's sight (50 ft, the hidden token unseen), whatever the box and the stage say", got)
	}
	if blind := both.SightOf("bard", true, func(grid.Square) bool { return false }, true); blind.Seen {
		t.Errorf("a bard whose player does not see the square = %+v, want Seen false", blind)
	}
	noBard := cutting.Place{NPC: "npc", Tokens: map[string]cutting.Token{"npc": {}}}
	if got := noBard.SightOf("bard", true, yes, true); got != (cutting.Sight{Box: true, OnStage: true}) {
		t.Errorf("a bard with no token = %+v, want the box and the stage", got)
	}
	if got := (cutting.Place{NPC: "npc"}).SightOf("bard", false, yes, false); got != (cutting.Sight{}) {
		t.Errorf("no map = %+v, want nothing seen", got)
	}
}
