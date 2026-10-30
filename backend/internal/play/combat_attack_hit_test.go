package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// attackHitsOf are the attack_hit events of what a stream has now.
func attackHitsOf(evs []*playv1.WatchGameSessionResponse) []*playv1.WatchGameSessionResponse_AttackHit {
	var out []*playv1.WatchGameSessionResponse_AttackHit
	for _, ev := range evs {
		if h := ev.GetAttackHit(); h != nil {
			out = append(out, h)
		}
	}
	return out
}

// TestApplyingAnNPCsHitTellsOnlyTheTargetsPlayerWhoHit: the damage the master applies on an NPC's attack reaches
// the target's player as an attack hit (who, how much, what type), so the sheet does not call it an adjustment of the
// master's; a hidden attacker has no name in it, and another player hears nothing.
func TestApplyingAnNPCsHitTellsOnlyTheTargetsPlayerWhoHit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		hidden bool
		actor  string
	}{
		{"a visible attacker is named", false, "Goblin"},
		{"a hidden attacker has no name", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e := rollsTable(t)
			e = a.passTo(t, e, "Goblin")
			if tc.hidden {
				hideAgain(t, a, e, "Goblin")
			}
			target, other := a.bia.watch(t, a.campaignID), a.caio.watch(t, a.campaignID)
			a.h.roller.queue(4)
			hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Brisa", d20(18))
			id := hit.GetPendingDamage().GetId()
			dmg := a.mustDamage(t, a.master, e, id, inAppDamage).GetPendingDamage()
			if got := attackHitsOf(drain(target)); len(got) != 0 {
				t.Fatalf("an attack hit before the master applied the damage: %v", got)
			}
			if _, err := a.settle(t, a.master, e, id, true); err != nil {
				t.Fatalf("ApplyPendingDamage() error = %v", err)
			}
			got := attackHitsOf(drain(target))
			if len(got) != 1 || got[0].GetId() != id || got[0].GetActorLabel() != tc.actor || got[0].GetAmount() != dmg.GetAmount() || got[0].GetDamageTypePt() != "cortante" {
				t.Fatalf("Brisa's player heard %v, want one hit %q by %q of %d cortante", got, id, tc.actor, dmg.GetAmount())
			}
			if heard := attackHitsOf(drain(other)); len(heard) != 0 {
				t.Errorf("another player heard the hit: %v", heard)
			}
		})
	}
}
