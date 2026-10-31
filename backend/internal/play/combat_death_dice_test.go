package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestRN18_ADeathSaveFollowsTheCampaignsDice: a death save takes the d20 in the app or the face of a
// real die as the campaign's dice setting allows, 1 to 20; the master is exempt. The server counts: the
// request never carries a total (RN-18, SRD 5.1 "Death Saving Throws").
func TestRN18_ADeathSaveFollowsTheCampaignsDice(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e) // Toren's turn

	// The campaign has everyone roll real dice.
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	_, err := a.deathSave(t, a.caio, e, "Toren", rollApp)
	wantBlockedBy(t, "an app roll in a campaign that rolls physical dice", err, blockedWrongDice)
	for face := range map[int32]bool{0: true, 21: true} {
		if _, err := a.deathSave(t, a.caio, e, "Toren", deathFace(face)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("face %d = %v, want invalid_argument", face, err)
		}
	}
	res, err := a.deathSave(t, a.caio, e, "Toren", deathFace(10))
	if err != nil {
		t.Fatalf("RollDeathSave(10) error = %v", err)
	}
	if ds := res.GetDeathSave(); ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS || ds.GetSuccesses() != 1 || ds.GetFailures() != 0 || !ds.GetRoll().GetPhysical() {
		t.Errorf("a typed 10 = %v, want a success, physical", ds)
	}

	// A typed 1 is two failures.
	e = a.passTo(t, e, "Pensantus")
	a.mustEndTurn(t, a.ana, e)
	res, err = a.deathSave(t, a.caio, e, "Toren", deathFace(1))
	if err != nil {
		t.Fatalf("RollDeathSave(1) error = %v", err)
	}
	if ds := res.GetDeathSave(); ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_CRITICAL_FAILURE || ds.GetFailures() != 2 {
		t.Errorf("a typed 1 = %v, want two failures", ds)
	}

	// The campaign makes everyone roll in the app.
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
	e = a.passTo(t, e, "Pensantus")
	a.mustEndTurn(t, a.ana, e)
	_, err = a.deathSave(t, a.caio, e, "Toren", deathFace(10))
	wantBlockedBy(t, "typed dice in a campaign that rolls in the app", err, blockedWrongDice)
	a.h.roller.queue(12)
	res, err = a.deathSave(t, a.caio, e, "Toren", rollApp)
	if err != nil {
		t.Fatalf("RollDeathSave(app) error = %v", err)
	}
	if ds := res.GetDeathSave(); ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS || ds.GetRoll().GetPhysical() {
		t.Errorf("an app roll = %v, want a success, not physical", ds)
	}

	// The master rolls the player's save with a typed face while the table rolls in the app: a natural 20
	// brings the character back with 1 hit point.
	e = a.passTo(t, e, "Pensantus")
	a.mustEndTurn(t, a.ana, e)
	res, err = a.deathSave(t, a.master, e, "Toren", deathFace(20))
	if err != nil {
		t.Fatalf("RollDeathSave(20) as the master error = %v", err)
	}
	if ds := res.GetDeathSave(); ds.GetOutcome() != playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_REVIVED {
		t.Errorf("a typed 20 = %v, want revived", ds)
	}
	if got := a.vitals(t, a.toren).GetHitPointsCurrent(); got != 1 {
		t.Errorf("Toren after a 20 = %d PV, want 1", got)
	}
}
