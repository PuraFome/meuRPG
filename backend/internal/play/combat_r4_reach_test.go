package play

import (
	"connectrpc.com/connect"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// TestR4_AMasterNPCMeleeAttackOutOfReachIsRefused: rehearsal 4, a ghoul's Garras (5 ft) at a target
// 6 m away was allowed and spent the action. SRD 5.1, "Making an Attack": a melee attack reaches 5 ft
// unless the creature says more. The reach limits the master's NPCs on a map as it does a player,
// and the refusal comes before anything is rolled or spent.
func TestR4_AMasterNPCMeleeAttackOutOfReachIsRefused(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	setup := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{30},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Goblin"},
		at:    map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
		setup: true,
	})
	if _, err := a.master.combat.BeginCombat(t.Context(), connect.NewRequest(&playv1.BeginCombatRequest{CampaignId: a.campaignID, EncounterId: setup.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("BeginCombat() error = %v", err)
	}
	e := a.get(t, a.master)
	// Pensantus is 35 ft from the goblin (7 squares): out of the 5 ft reach.
	_, err := a.attack(t, a.master, e, "Goblin", sword, "Pensantus", d20(15))
	if b := wantBlockedBy(t, "the master's goblin attacking from 30 ft", err, blockedOutOfReach); b.GetMissingFt() != 25 {
		t.Errorf("missing_ft = %d, want 25", b.GetMissingFt())
	}
	// The refusal spent nothing: the goblin still attacks Toren, one square away.
	if _, err := a.attack(t, a.master, e, "Goblin", sword, "Toren", d20(15)); err != nil {
		t.Errorf("the master's goblin attacking an adjacent target: %v, want it allowed (the refusal spent nothing)", err)
	}
}

// closeTo puts the attacker on a free square next to the target, when it is not within 5 ft already: the
// tests that only need a melee hit to land, now that the reach binds the master's NPCs too (SRD 5.1).
func (a *armed) closeTo(t *testing.T, attacker, target string) {
	t.Helper()
	e := a.get(t, a.master)
	var from, to *playv1.Combatant
	taken := map[[2]int32]bool{}
	for _, c := range e.GetCombatants() {
		taken[[2]int32{c.GetCol(), c.GetRow()}] = true
		switch c.GetId() {
		case a.id(t, attacker):
			from = c
		case a.id(t, target):
			to = c
		}
	}
	if from == nil || to == nil {
		t.Fatalf("closeTo(%s, %s): not in the combat", attacker, target)
	}
	abs := func(n int32) int32 { return max(n, -n) }
	if max(abs(from.GetCol()-to.GetCol()), abs(from.GetRow()-to.GetRow())) <= 1 {
		return
	}
	for _, d := range [][2]int32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, 1}, {-1, 1}, {1, -1}} {
		col, row := to.GetCol()+d[0], to.GetRow()+d[1]
		if col >= 0 && row >= 0 && !taken[[2]int32{col, row}] {
			a.put(t, attacker, col, row)
			return
		}
	}
	t.Fatalf("closeTo(%s, %s): no free square", attacker, target)
}
