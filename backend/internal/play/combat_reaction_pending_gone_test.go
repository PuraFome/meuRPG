package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

const (
	goblinGoneMaster  = "Reator usou Palavras de Interrupção: d6 (5) subtraído do ataque de Goblin."
	goblinGonePlayers = goblinPendingPlayers
	// A removed combatant leaves its lines anonymous.
	removedMaster  = "Reator usou Palavras de Interrupção: d6 (5) subtraído do ataque."
	removedPlayers = "Reator usou Palavras de Interrupção: o ataque diminuiu."
)

// A roll that never settles: its roller is removed, or the combat ends, while the question is open. The hold is forgotten with a
// combatant_removed or an encounter_ended event the log reads, so the master's line stops promising the roll and says nothing of an
// outcome, and the players keep "diminuiu".
func TestACutLineOfARollThatWillNeverSettleStopsWaiting(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		end            func(t *testing.T, a *armed, e *playv1.Encounter)
		master, player string
	}{
		"the roller is removed": {func(t *testing.T, a *armed, e *playv1.Encounter) {
			t.Helper()
			if _, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
				CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(),
			})); err != nil {
				t.Fatalf("RemoveCombatant() error = %v", err)
			}
		}, removedMaster, removedPlayers},
		"the combat ends": {func(t *testing.T, a *armed, e *playv1.Encounter) {
			t.Helper()
			a.endEncounter(t, e)
		}, goblinGoneMaster, goblinGonePlayers},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, e, _ := goblinCut(t)
			a.wantLines(t, e, a.goblinReaders(), goblinPendingMaster, goblinPendingPlayers)
			tc.end(t, a, e)
			if got := a.listHeldRolls(t, a.master); len(got) != 0 {
				t.Fatalf("the master's questions after it = %v, want none", got)
			}
			a.wantLines(t, e, a.goblinReaders(), tc.master, tc.player)
		})
	}
}

// The end of the combat is no delta-safe event: a master who holds the log from before it reads, in what the server answers after it,
// the line a fresh read gives.
func TestADeltaReadAfterTheEndGivesTheSameCutLineAsAFreshRead(t *testing.T) {
	t.Parallel()
	a, e, _ := goblinCut(t)
	held := a.log(t, a.master, e).GetLastSeq()
	a.endEncounter(t, e)
	got := reactionEntry(a.since(t, a.master, e, held)).GetReactionTextPt()
	if got != goblinGoneMaster {
		t.Errorf("the master reads %q in the delta after the end (a fresh read: %q), want %q", got, a.cutLinesOf(t, a.master, e), goblinGoneMaster)
	}
}

// Only the roller's removal stops the waiting: the removal of another combatant leaves the line waiting.
func TestRemovingAnotherCombatantLeavesTheCutLineWaiting(t *testing.T) {
	t.Parallel()
	a, e, _ := goblinCut(t)
	known := map[string]bool{}
	for _, c := range e.GetCombatants() {
		known[c.GetId()] = true
	}
	res, err := a.master.combat.AddCombatants(t.Context(), connect.NewRequest(&playv1.AddCombatantsRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), Participants: []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
	}))
	if err != nil {
		t.Fatalf("AddCombatants() error = %v", err)
	}
	other := ""
	for _, c := range res.Msg.GetEncounter().GetCombatants() {
		if !known[c.GetId()] {
			other = c.GetId()
		}
	}
	if other == "" {
		t.Fatal("no combatant was added")
	}
	if _, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: other, IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant() error = %v", err)
	}
	a.wantLines(t, e, a.goblinReaders(), goblinPendingMaster, goblinPendingPlayers)
}

// The end of the combat leaves a line that was already completed as it was.
func TestEndingTheCombatKeepsACompletedCutLine(t *testing.T) {
	t.Parallel()
	a, e, offer := goblinCut(t)
	if _, err := a.answerHeldRoll(t, a.master, offer.GetHoldId(), dieAnswer(false)); err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	a.endEncounter(t, e)
	a.wantLines(t, e, a.goblinReaders(),
		"Reator usou Palavras de Interrupção: d6 (5) subtraído do ataque de Goblin: 12 → 7 contra CA 12: errou.",
		"Reator usou Palavras de Interrupção: o ataque de Goblin errou.")
}
