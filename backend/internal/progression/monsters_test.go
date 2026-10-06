package progression

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestMR042_ThreeBanditsGiveTheirXPToTheParty: three Bandidos put in the combat with
// "Pôr no combate" (ND 1/8, 25 XP each) and defeated give 75 XP at the end, which
// the enemies award splits among the four players: 18 each and 3 left over
// (question 46; artboard E10-08, state 9).
func TestMR042_ThreeBanditsGiveTheirXPToTheParty(t *testing.T) {
	t.Parallel()
	tb := newTable(t, enemies, 4)
	filler := tb.master.minion(t, tb.campaign, "Vigia", 0) // gives no XP
	enc := tb.fight(t, filler, 1, 1, false)
	added, err := tb.master.combat.AddMonsters(t.Context(), connect.NewRequest(&playv1.AddMonstersRequest{
		CampaignId: tb.campaign, EncounterId: enc.GetId(), IdempotencyKey: newKey(), CreatureKey: "monster:bandit", Count: 3,
	}))
	if err != nil {
		t.Fatalf("AddMonsters() error = %v", err)
	}
	for _, id := range added.Msg.GetCombatantIds() {
		if _, err := tb.master.combat.AdjustCombatantHitPoints(t.Context(), connect.NewRequest(&playv1.AdjustCombatantHitPointsRequest{
			CampaignId: tb.campaign, EncounterId: enc.GetId(), CombatantId: id, IdempotencyKey: newKey(),
			Change: &playv1.AdjustCombatantHitPointsRequest_HitPoints{HitPoints: 0},
		})); err != nil {
			t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
		}
	}
	if _, err := tb.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: tb.campaign, EncounterId: enc.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	res, err := tb.enemies(enc.GetId(), tb.ids(4)...)
	if err != nil {
		t.Fatalf("AwardXP(enemies) error = %v", err)
	}
	if res.GetAward().GetTotalXp() != 75 || res.GetXpEach() != 18 || res.GetLostXp() != 3 {
		t.Errorf("award = total %d, each %d, lost %d; want 75, 18, 3", res.GetAward().GetTotalXp(), res.GetXpEach(), res.GetLostXp())
	}
}
