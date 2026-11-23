package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

const (
	goblinGoneMaster  = "Reator usou Palavras de Interrupção: d6 (5) subtraído do ataque de Goblin."
	goblinGonePlayers = goblinPendingPlayers
	// a removed combatant leaves its lines anonymous
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
