package progression

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/progression/progressiondb"
)

// emptyParty and emptyProfiles answer the name reads with no names.
type partyAPI = Party

type emptyParty struct{ partyAPI }

type emptyProfiles struct{ Profiles }

func (emptyParty) Names(context.Context, string, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (emptyProfiles) DisplayNames(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

// failingCombats fails every name read and counts the calls and their IDs.
type failingCombats struct {
	Combats
	calls int
	ids   []string
}

func (c *failingCombats) EncounterNames(_ context.Context, _ string, ids []string) (map[string]string, error) {
	c.calls++
	c.ids = ids
	return nil, errors.New("combats are down")
}

func TestReadHistoryNamesAsksCombatsOnlyForCombatAwards(t *testing.T) {
	enc1, enc2 := "enc-1", "enc-2"
	noCombat := []progressiondb.XpAward{{ID: "a"}, {ID: "b"}}
	withCombat := []progressiondb.XpAward{{ID: "a"}, {ID: "b", EncounterID: &enc1}, {ID: "c", EncounterID: &enc2}}

	t.Run("no award has a combat", func(t *testing.T) {
		combats := &failingCombats{}
		s := &Service{party: emptyParty{}, profiles: emptyProfiles{}, combats: combats}
		if _, err := s.readHistoryNames(t.Context(), "camp", noCombat, nil); err != nil {
			t.Fatalf("readHistoryNames: %v", err)
		}
		if combats.calls != 0 {
			t.Fatalf("combats called %d times, want 0", combats.calls)
		}
	})

	t.Run("an award has a combat", func(t *testing.T) {
		combats := &failingCombats{}
		s := &Service{party: emptyParty{}, profiles: emptyProfiles{}, combats: combats}
		if _, err := s.readHistoryNames(t.Context(), "camp", withCombat, nil); err == nil {
			t.Fatal("readHistoryNames: want the combats' error")
		}
		if combats.calls != 1 || !slices.Equal(combats.ids, []string{enc1, enc2}) {
			t.Fatalf("combats called %d times with %v, want once with [enc-1 enc-2]", combats.calls, combats.ids)
		}
	})
}
