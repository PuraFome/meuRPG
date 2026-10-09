package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// SRD 5.1, "Resting": the master's rest ends the spells that last when their whole duration
// fits it. A short rest (an hour) ends Bless (1 minute) and leaves Mage Armor (8 hours); a long
// rest (8 hours) ends Mage Armor. The panel promises it, and TakeRest does it in its transaction.
func TestARestEndsTheSpellsThatLastLessThanIt(t *testing.T) {
	t.Parallel()
	a := newCastingParty(t)
	mage := a.mustCastOut(t, a.ana, a.pens, mageArmorKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	bless := a.mustCastOut(t, a.bia, a.bri, blessKey, slotOfLevel(1), false, []*charactersv1.Character{a.toren}).GetCast()
	rest := func(kind playv1.RestKind) {
		t.Helper()
		if _, err := a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{
			CampaignId: a.campaignID, Kind: kind, IdempotencyKey: newKey(),
		})); err != nil {
			t.Fatalf("TakeRest(%v) error = %v", kind, err)
		}
	}
	rest(playv1.RestKind_REST_KIND_SHORT)
	if c := a.castByID(t, a.master, bless.GetId()); c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_REST {
		t.Errorf("Bless after a short rest = %v, want ended by the rest", c)
	}
	if c := a.castByID(t, a.master, mage.GetId()); c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_UNSPECIFIED {
		t.Errorf("Mage Armor after a short rest = %v, want it still on", c)
	}
	rest(playv1.RestKind_REST_KIND_LONG)
	if c := a.castByID(t, a.master, mage.GetId()); c.GetEndReason() != playv1.OutsideCastEnd_OUTSIDE_CAST_END_REST {
		t.Errorf("Mage Armor after a long rest = %v, want ended by the rest", c)
	}
}
