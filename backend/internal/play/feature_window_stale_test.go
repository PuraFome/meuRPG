package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A FEATURE window is checked again while it waits (docs/architecture/play/reactions.md "The feature window"): it closes, quietly,
// when the feature can no longer be used on the creature that triggered it, so the turn is not held by a stale prompt. These tests
// need the database.

// requireTheWindowClosedAndTheTurnFree checks no FEATURE window stands for the master and Ana and the goblin's turn can end.
func (m *featureTable) requireTheWindowClosedAndTheTurnFree(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	for name, u := range map[string]*user{"the master": m.master, "Ana": m.ana} {
		if w := m.windowOf(t, u, kindFeature); w != nil {
			t.Errorf("%s reads the FEATURE window %v, want it closed", name, w)
		}
	}
	if _, err := m.endTurn(t, m.master, e, true); err != nil {
		t.Errorf("the goblin's turn is held after the window should have closed: %v", err)
	}
}

// The goblin is brought to 0 hit points while the window waits: the window closes.
func TestTheFeatureWindowClosesWhenTheCreatureIsDefeated(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e, _ := m.haloWindow(t)
	if _, err := m.master.combat.AdjustCombatantHitPoints(t.Context(), connect.NewRequest(&playv1.AdjustCombatantHitPointsRequest{
		CampaignId: m.campaignID, EncounterId: e.GetId(), CombatantId: m.id(t, "Goblin"), IdempotencyKey: newKey(),
		Change: &playv1.AdjustCombatantHitPointsRequest_HitPoints{HitPoints: 0},
	})); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	m.requireTheWindowClosedAndTheTurnFree(t, e)
}

// The goblin joins Pensantus's side while the window waits: it is no enemy now, the window closes.
func TestTheFeatureWindowClosesWhenTheCreatureChangesSide(t *testing.T) {
	t.Parallel()
	m := haloTable(t)
	e, _ := m.haloWindow(t)
	if _, err := m.master.combat.SetCombatantSide(t.Context(), connect.NewRequest(&playv1.SetCombatantSideRequest{
		CampaignId: m.campaignID, EncounterId: e.GetId(), CombatantId: m.id(t, "Goblin"), IdempotencyKey: newKey(), Side: playv1.CombatantSide_COMBATANT_SIDE_PARTY,
	})); err != nil {
		t.Fatalf("SetCombatantSide() error = %v", err)
	}
	m.requireTheWindowClosedAndTheTurnFree(t, e)
}

// The only use of the feature is spent (by the master, who uses it for Pensantus) while the window waits: the window closes.
func TestTheFeatureWindowClosesWhenTheUsesAreSpent(t *testing.T) {
	t.Parallel()
	m := haloTableOf(t, "1", nil, nil)
	e, _ := m.haloWindow(t)
	m.mustUse(t, m.master, e, "Pensantus", "Halo", m.aim(t, "Goblin"))
	m.requireTheWindowClosedAndTheTurnFree(t, e)
}
