package leaktest

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A rider on a mount the master hid reads as a plain combatant to the players: no id, no name of the mount in any copy they get,
// and the mark the master put on it stays the master's. The master's copy and a player's copy after the reveal are the positive controls.
func TestHiddenMountLeavesItsRiderPlain(t *testing.T) {
	w := newWorld(t)
	ctx := t.Context()
	e := w.encounter
	mount, rider := w.combatant(w.hiddenNPC), w.combatant(w.seenNPC)
	if !mount.GetHidden() {
		t.Fatalf("the fixture's hidden NPC combatant is not hidden")
	}
	must(w.master.combat.SetCombatantMount(ctx, rq(&playv1.SetCombatantMountRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: mount.GetId(), IdempotencyKey: newKey(), Kind: "controlled"})))
	if _, err := w.pool.Exec(ctx, `UPDATE combatants SET mounted_on_id = $1 WHERE id = $2`, mount.GetId(), rider.GetId()); err != nil {
		t.Fatalf("put the rider on the mount: %v", err)
	}
	read := func(p *person) string {
		return jsonOf(must(p.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))))
	}
	// The positive control: the master reads the link, the mark and the mount's name.
	masterCopy := read(w.master)
	for _, needle := range []string{mount.GetId(), mount.GetLabel(), "mountKind"} {
		if !strings.Contains(masterCopy, needle) {
			t.Fatalf("the positive control failed: the master's copy has no %s", needle)
		}
	}
	for _, p := range []*person{w.ana, w.caio} {
		got := read(p)
		for _, needle := range []string{mount.GetId(), mount.GetLabel(), "mountKind", "mountedOnId", "riderId"} {
			if strings.Contains(got, needle) {
				t.Errorf("%s reads %q while the mount is hidden", p.name, needle)
			}
		}
	}
	// The reveal is the second positive control: a player who sees both rows reads the link and the mount, and still not the mark.
	must(w.master.combat.SetCombatantHidden(ctx, rq(&playv1.SetCombatantHiddenRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: mount.GetId(), IdempotencyKey: newKey(), Hidden: false})))
	got := read(w.ana)
	for _, needle := range []string{mount.GetId(), mount.GetLabel(), "mountedOnId", "riderId"} {
		if !strings.Contains(got, needle) {
			t.Errorf("the positive control failed: Ana reads no %q after the mount is revealed", needle)
		}
	}
	if strings.Contains(got, "mountKind") {
		t.Errorf("Ana reads the master's mark of a mount that is not hers")
	}
}
