package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A readied action against the turn in progress: a readied move's opportunity offers, and a readied attack whose roll is
// held for the windows any attack roll opens.

// rogueReadier is a fight with Reator, Ana's rogue 5, beside the goblin at (4,4), on Reator's turn.
func rogueReadier(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		a.pens = rogueFive(t, a)
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Reator": {4, 4}, "Brisa": {8, 8}},
	})
	return a, a.onTurnOf(t, e, "Reator")
}

// The rogue readies a move and then Disengages with Cunning Action: Disengage covers only the turn it is taken on, so the
// readied move on the goblin's turn still offers the goblin its attack. The offer holds the end of the goblin's turn, the
// master's too, until it is answered; then the turn ends and no offer is left.
func TestAReadiedMoveProvokesAfterADisengageAndHoldsTheTurn(t *testing.T) {
	t.Parallel()
	a, e := rogueReadier(t)
	a.mustReady(t, a.ana, &playv1.ReadyActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Reator"), IdempotencyKey: newKey(),
		TriggerText: readyTrigger, Kind: playv1.ReadiedKind_READIED_KIND_MOVE,
	})
	if _, err := a.action(t, a.ana, e, "Reator", "feature:cunning-action:disengage"); err != nil {
		t.Fatalf("Cunning Action (Disengage): %v", err)
	}
	e = a.mustEndTurn(t, a.master, e) // the goblin's turn
	a.mustFire(t, e, "Reator")
	res, err := a.frozenMove(t, a.ana, "Reator", 4, 6)
	if err != nil {
		t.Fatalf("the readied move: %v", err)
	}
	offers := a.get(t, a.master).GetOpportunityOffers()
	if !res.GetProvoked() || len(offers) != 1 || offers[0].GetReactorId() != a.id(t, "Goblin") {
		t.Fatalf("the readied move after a Disengage: provoked %v, offers %v; want the goblin's", res.GetProvoked(), offers)
	}
	_, err = a.endTurn(t, a.master, e, false)
	wantBlockedBy(t, "the master's EndTurn while the readied move's offer waits", err, reasonOpportunityPending)
	if _, err := a.master.combat.DeclineOpportunity(t.Context(), connect.NewRequest(&playv1.DeclineOpportunityRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: offers[0].GetId(), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("the master's decline: %v", err)
	}
	e = a.mustEndTurn(t, a.master, e)
	if got := a.get(t, a.master).GetOpportunityOffers(); len(got) != 0 {
		t.Errorf("offers after the turn = %v", got)
	}
}
