package leaktest

import (
	"slices"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A keyed replay of a write whose subject the players cannot see tells them nothing that a write under a key the server never
// saw does not tell them (#735): the replay's hints are a superset of what the first call told (play.tellReplayed), and they
// must not reach a player who heard nothing of the first call. Two worlds differ in the one thing under test: in the first the
// master sends the damage to the hidden NPC and then sends the same request again under the same key (a replay); in the
// second the second request has a key the server never saw (a second write). Ana and Caio hear the same in both: nothing.
func TestAReplayOfAWriteToAHiddenCombatantTellsPlayersWhatANewKeyDoes(t *testing.T) {
	hear := func(t *testing.T, replay bool) map[string][]string {
		t.Helper()
		w := newWorld(t)
		hid := w.combatant(w.hiddenNPC)
		if !hid.GetHidden() {
			t.Fatal("the fixture's hidden NPC combatant is not hidden")
		}
		damage := func(key string) {
			must(w.master.combat.AdjustCombatantHitPoints(t.Context(), rq(&playv1.AdjustCombatantHitPointsRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: hid.GetId(), IdempotencyKey: key,
				Change: &playv1.AdjustCombatantHitPointsRequest_Damage{Damage: 1},
			})))
		}
		return w.hiddenOnlyEvents(func() {
			first := newKey()
			damage(first)
			if replay {
				damage(first)
			} else {
				damage(newKey())
			}
		})
	}
	replayed, fresh := hear(t, true), hear(t, false)
	for _, name := range []string{"Ana", "Caio"} {
		if len(fresh[name]) != 0 {
			t.Errorf("%s heard %q of two writes to a hidden combatant: the control is gone", name, fresh[name])
		}
		if !slices.Equal(replayed[name], fresh[name]) {
			t.Errorf("%s heard %q after a replay and %q after a write under a new key: a replay tells more", name, replayed[name], fresh[name])
		}
	}
	if len(replayed["Mestre"]) == 0 {
		t.Error("the master heard nothing of the replay: the control is gone")
	}
}
