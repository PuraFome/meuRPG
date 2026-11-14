package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A damage_type entry that covers ONE counted dice part is not a group for removal: the attack sheet sends an entry for
// every typed field, even a part alone in its type, and the master must still be able to take that part out. The sum
// stays on the part, as a part_key sum does, with no group marked on it.

func TestADamageTypeThatCoversOnePartIsNotAGroup(t *testing.T) {
	t.Parallel()
	parts := []partRecord{
		{Key: partWeapon, Kind: partWeapon, Count: 1, Sides: 8, DamageType: groupPiercing, Available: true},
		{Key: "radiant-edge", Kind: partExtra, Choosable: true, Count: 1, Sides: 4, DamageType: groupRadiant, Available: true},
		{Key: "fire-one", Kind: partExtra, Choosable: true, Count: 1, Sides: 6, DamageType: groupFire, Available: true},
		{Key: "fire-two", Kind: partExtra, Choosable: true, Count: 1, Sides: 6, DamageType: groupFire, Available: true},
	}
	got, err := groupCase{
		parts: parts, marks: []string{"radiant-edge", "fire-one", "fire-two"},
		typed: typedSums{byType: map[string]int{groupPiercing: 5, groupRadiant: 3, groupFire: 7}},
	}.roll()
	if err != nil {
		t.Fatalf("rollParts() error = %v", err)
	}
	want := map[string]struct {
		group string
		sum   int32
	}{partWeapon: {"", 5}, "radiant-edge": {"", 3}, "fire-one": {groupFire, 7}, "fire-two": {groupFire, 0}}
	for _, r := range got.rolls {
		if w := want[r.Key]; r.Group != w.group || r.Sum != w.sum || r.GroupHidden {
			t.Errorf("%s: group %q sum %d hidden %v, want group %q sum %d", r.Key, r.Group, r.Sum, r.GroupHidden, w.group, w.sum)
		}
	}
	if got.byType[groupPiercing] != 5 || got.byType[groupRadiant] != 3 || got.byType[groupFire] != 7 {
		t.Errorf("byType = %v, want the typed sums", got.byType)
	}
	if !takeOut(got.parts, got.rolls, "radiant-edge") {
		t.Error("takeOut(a part alone in its typed type) = false, want it taken out")
	}
	if takeOut(got.parts, got.rolls, "fire-two") {
		t.Error("takeOut(a part of a two-part group) = true, want it refused")
	}
}

// The master takes out an extra typed through a damage_type entry of its own: it comes out and the damage falls by its sum.
func TestTheMasterTakesOutAPartTypedThroughAOnePartDamageType(t *testing.T) {
	t.Parallel()
	g := newGiantTableWith(t, &rulesv1.TableEffect{Type: "extra_damage", Applies: rules.ExtraDamageAnyTarget, Dice: "1", DiceSides: 6, DamageType: "damage-type:fire", Weapon: rules.ExtraDamageMeleeWeapon})
	e := g.passTo(t, g.castersFight(t, 1), "Pensantus")
	key := giantFeatureKey(t, g)
	hit := g.hit(t, e)
	rolled := g.mustDamage(t, g.ana, e, hit.GetId(), extrasMarked([]*playv1.SelectedExtra{{Key: key}},
		&playv1.TypedPart{DamageType: "damage-type:slashing", Sum: 5}, &playv1.TypedPart{DamageType: "damage-type:fire", Sum: 4})).GetPendingDamage()
	res, err := g.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(&playv1.RemoveDamagePartRequest{
		CampaignId: g.campaignID, EncounterId: e.GetId(), PendingDamageId: rolled.GetId(), PartKey: key, Reason: "motivo", IdempotencyKey: newKey(),
	}))
	if err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if got, want := res.Msg.GetPendingDamage().GetAmount(), rolled.GetAmount()-4; got != want {
		t.Errorf("the damage after the removal = %d, want %d (it fell by the 4 of the part)", got, want)
	}
}
