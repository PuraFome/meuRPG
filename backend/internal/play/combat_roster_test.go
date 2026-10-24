package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// The roster of a combat about to start: who has a token on the fight's map and the square it gives.

func TestTheStartRosterFollowsTheTokensOfTheMap(t *testing.T) {
	t.Parallel()
	g := link.Grid{Columns: 20, Rows: 10}
	party := []link.Character{{ID: "tor", Name: "Tor"}, {ID: "brisa", Name: "Brisa"}, {ID: "jabuti", Name: "Jabuti"}}
	tokens := []link.TokenPosition{
		{CharacterID: "brisa", XBP: 2550, YBP: 3050}, // column 5, row 3
		{CharacterID: "tor", XBP: 5000, YBP: 5000},   // column 10, row 5
		{CharacterID: "someone-else", XBP: 100, YBP: 100},
	}
	got := rosterOf(party, tokens, g)
	want := []rosterEntry{
		{id: "tor", onMap: true, col: 10, row: 5},
		{id: "brisa", onMap: true, col: 5, row: 3},
		{id: "jabuti"},
	}
	if len(got) != len(want) {
		t.Fatalf("roster = %+v, want %d entries", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTwoTokensOnOneSquareLeaveTheSecondWithoutASquare(t *testing.T) {
	t.Parallel()
	g := link.Grid{Columns: 20, Rows: 10}
	party := []link.Character{{ID: "tor"}, {ID: "brisa"}}
	tokens := []link.TokenPosition{{CharacterID: "tor", XBP: 5010, YBP: 5010}, {CharacterID: "brisa", XBP: 5040, YBP: 5040}}
	got := rosterOf(party, tokens, g)
	if !got[0].onMap || got[0].sharesSquare || !got[1].onMap || !got[1].sharesSquare {
		t.Errorf("roster = %+v, want the second to share the first's square", got)
	}
}
