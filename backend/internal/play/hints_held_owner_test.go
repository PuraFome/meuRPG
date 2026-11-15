package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A roll the master makes for a player's character out of a combat (RollForPlayer in a group check) waits for the player's question
// as any held roll does, so the character's player hears HeldRollsChanged, as the master does; another player hears nothing.
func TestAHeldRollTheMasterMakesForACharacterOutsideACombatTellsItsPlayer(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, dalilaID := joinDalila(t, a.h, a.master, a.campaignID)
	view := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc = 20 })
	if _, err := a.giveOutside(t, a.bia, dalilaID); err != nil {
		t.Fatalf("GiveBardicInspirationOutside() error = %v", err)
	}
	g := newHintRig(t, a.h, a.campaignID, a.master, map[string]*user{"Dalila": dalila, "Ana": a.ana})
	before := g.seq()
	if _, err := a.rollForPlayer(t, a.master, view.GetId(), dalilaID, 5); err != nil {
		t.Fatalf("RollForPlayer() error = %v", err)
	}
	if n := countRows(t, a.h, countInspirationHold, a.campaignID); n != 1 {
		t.Fatalf("%d rows in inspiration_holds, want the roll held for the die's question", n)
	}
	stored := g.seq()
	if stored != before+1 {
		t.Errorf("hint_seq = %d, want %d (once)", stored, before+1)
	}
	heard := g.drain()
	wantHeld(t, heard, stored, "Dalila")
	for _, ev := range heard["Ana"] {
		if ev.GetHeldRollsChanged() != nil {
			t.Errorf("Ana heard HeldRollsChanged for a roll that is not her character's: %v", eventKinds(heard["Ana"]))
		}
	}
}
