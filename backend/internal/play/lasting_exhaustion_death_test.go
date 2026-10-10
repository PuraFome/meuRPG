package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Exhaustion 6 is death (SRD 5.1, Conditions). With no combat there is no death confirmation to
// go to, so the master's confirm_death is the confirmation and the character dies at once (RN-03).
func TestExhaustionSixOutsideACombatKillsTheCharacterOnTheMastersConfirmation(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	set := func(confirm bool) error {
		_, err := a.master.lasting.SetExhaustion(t.Context(), connect.NewRequest(&playv1.SetExhaustionRequest{
			CampaignId: a.campaignID, IdempotencyKey: newKey(), Level: 6, ExpectedLevel: 0, ConfirmDeath: confirm,
			Subject: &playv1.SetExhaustionRequest_CharacterId{CharacterId: a.toren.GetId()},
		}))
		return err
	}
	if err := set(false); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("level 6 without confirmation: err = %v, want failed_precondition", err)
	}
	if got := a.master.character(t, a.toren).GetState(); got == charactersv1.CharacterState_CHARACTER_STATE_DEAD {
		t.Fatal("the character died without the confirmation")
	}
	if err := set(true); err != nil {
		t.Fatalf("level 6 confirmed: err = %v", err)
	}
	if got := a.master.character(t, a.toren).GetState(); got != charactersv1.CharacterState_CHARACTER_STATE_DEAD {
		t.Errorf("the character's state = %v, want dead", got)
	}
}

// A concentration save is a saving throw like the others (SRD 5.1): exhaustion 3 or more gives
// disadvantage on it, so it takes two d20 and the lower counts.
func TestAConcentrationSaveHasDisadvantageAtExhaustionThree(t *testing.T) {
	t.Parallel()
	a, e := concentratingHit(t, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	if _, err := a.setExhaustion(t, e, "Pensantus", 3, 0, false); err != nil {
		t.Fatalf("SetExhaustion(3) error = %v", err)
	}
	// One die is not enough any more.
	if _, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(18)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("one d20 at disadvantage: err = %v, want invalid_argument", err)
	}
	second := int32(3)
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), func(r *playv1.ResolveConcentrationSaveRequest) {
		r.Roll = &playv1.ResolveConcentrationSaveRequest_D20Face{D20Face: 18}
		r.SecondD20Face = &second
	})
	if err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	// The lower die (3) plus the +1 of the bonus is 4 against DC 10: the concentration is lost.
	if r := res.GetResult(); r.GetKept() || r.GetSave().GetTotal() != 4 || len(r.GetSave().GetFaces()) != 2 {
		t.Errorf("the result = %v, want a failed save of 4 from two dice", r)
	}
}
