package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A creature's save with advantage from Magic Resistance (RN-20, RN-29): the master reads the mode and
// both dice; a player who does not own the creature reads the outcome only, never the mode that would
// give the trait away.
func TestMagicResistanceSaveModeIsTheMastersAlone(t *testing.T) {
	t.Parallel()
	owner := "user-owner"
	creature := playdb.Combatant{ID: "imp", UserID: &owner}
	sr := &saveRoll{Saved: true, D20: 17, D20B: 4, Counted: 0, Bonus: 2, Total: 19, DC: 15, RollMode: modeAdvantageKey}
	master := saveView(sr, combatViewer{master: true}, playdb.Combatant{}, creature)
	if master.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || len(master.GetRoll().GetFaces()) != 2 { // positive control
		t.Fatalf("the master reads %v, want advantage with two dice", master)
	}
	stranger := saveView(sr, combatViewer{userID: "user-other"}, playdb.Combatant{}, creature)
	if stranger.GetMode() != playv1.RollMode_ROLL_MODE_UNSPECIFIED || stranger.GetRoll() != nil || stranger.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED {
		t.Errorf("another player reads %v, want the outcome alone", stranger)
	}
}
