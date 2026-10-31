package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// TestMR013_EqualRollsMakeAJointTurnOfAnAddedCreature shows, without a
// database, what a test meets when the dice give two creatures the same d20:
// the app rolls each one's initiative (RN-19), the totals tie, and the
// creature added after the combat began is in the same group as the one
// already on the track, so they act together and the one added does not
// "become the current one". The dice a test hands the service decide this.
func TestMR013_EqualRollsMakeAJointTurnOfAnAddedCreature(t *testing.T) {
	t.Parallel()
	s := &Service{roller: &dice.Fixed{Faces: []int{12, 12}}}
	const bonus = 2
	var track []playdb.Combatant
	for _, id := range []string{"goblin-1", "goblin-2-added-later"} {
		_, total, err := s.rollInitiative(bonus)
		if err != nil {
			t.Fatalf("rollInitiative() error = %v", err)
		}
		track = append(track, playdb.Combatant{ID: id, Initiative: new(conv.Int32(total))})
	}
	runs := groupRuns(track)
	if len(runs) != 1 || len(runs[0]) != 2 {
		t.Fatalf("groups = %d (first of %d members), want one joint turn of both creatures", len(runs), len(runs[0]))
	}
}
