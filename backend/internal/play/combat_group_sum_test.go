package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master takes an extra out of a group of parts that share a damage type (#835): the damage is taken back as the
// undo takes it back, the part is marked out and the damage waits for the sum of that damage type alone.

// groupHit is a rogue's hit on a restrained shrub, rolled with one piercing sum that covers the rapier and Sneak
// Attack: the usual group of two.
type groupHit struct {
	a      *armed
	e      *playv1.Encounter
	shrub  string
	id     string
	before int32
	rolled *playv1.PendingDamage
}

func newGroupHit(t *testing.T, sum int32) *groupHit {
	t.Helper()
	a, _, shrub := partsFight(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, abilities(16, 16, 14, 12, 10), []string{rapier}, nil)
	}, "monster:awakened-shrub")
	e := a.passTo(t, a.get(t, a.master), "Toren")
	a.setConditions(t, e, shrub, "condition:restrained")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, shrub, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	g := &groupHit{a: a, e: e, shrub: shrub, id: hit.GetPendingDamage().GetId()}
	g.before, _, _ = a.hp(t, shrub)
	g.rolled = a.mustDamage(t, a.caio, e, g.id, extrasMarked([]*playv1.SelectedExtra{{Key: "sneak-attack"}}, &playv1.TypedPart{DamageType: groupPiercing, Sum: sum})).GetPendingDamage()
	return g
}

func (g *groupHit) takeOut(t *testing.T, key, reason string) (*playv1.RemoveDamagePartResponse, error) {
	t.Helper()
	res, err := g.a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(&playv1.RemoveDamagePartRequest{
		CampaignId: g.a.campaignID, EncounterId: g.e.GetId(), PendingDamageId: g.id, PartKey: key, Reason: reason, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (g *groupHit) hp(t *testing.T) int32 {
	t.Helper()
	hp, _, _ := g.a.hp(t, g.shrub)
	return hp
}

func damagePartOf(p *playv1.PendingDamage, key string) *playv1.DamagePart {
	for _, d := range p.GetParts() {
		if d.GetKey() == key {
			return d
		}
	}
	return nil
}

// A damage already applied to an NPC: the hit points come back, the part is out, and the damage waits for the sum of
// the type alone; the new sum covers what is left of the group and the damage lands again.
func TestTheMasterTakesAPartOutOfAGroupAndTheDamageAwaitsItsSum(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if g.hp(t) >= g.before {
		t.Fatalf("hit points = %d after the roll, want fewer than %d", g.hp(t), g.before)
	}
	res, err := g.takeOut(t, "sneak-attack", "sem aliado perto")
	if err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	p := res.GetPendingDamage()
	if p.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_GROUP_SUM || p.GetAwaitedDamageTypeKey() != groupPiercing {
		t.Fatalf("pending damage = status %v awaiting %q, want AWAITING_GROUP_SUM awaiting %q", p.GetStatus(), p.GetAwaitedDamageTypeKey(), groupPiercing)
	}
	if p.GetRoll() != nil || p.GetAmount() != 0 {
		t.Errorf("the damage kept a roll (%v) or an amount (%d), want none", p.GetRoll(), p.GetAmount())
	}
	if sneak := damagePartOf(p, "sneak-attack"); sneak == nil || sneak.GetSelected() || sneak.GetAvailable() {
		t.Errorf("Sneak Attack = %v, want it listed and out", sneak)
	}
	if got := g.hp(t); got != g.before {
		t.Errorf("hit points = %d after the take-out, want %d: the damage was undone", got, g.before)
	}

	// The sum is the rapier alone now: 1 to 8, with no modifier.
	_, err = g.a.damage(t, g.a.caio, g.e, g.id, typedDamage(9))
	wantCode(t, "RollDamage(a sum out of range)", err, connect.CodeInvalidArgument)
	again, err := g.a.damage(t, g.a.caio, g.e, g.id, typedDamage(5))
	if err != nil {
		t.Fatalf("RollDamage() error = %v", err)
	}
	got := again.GetPendingDamage()
	if got.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED || got.GetAwaitedDamageTypeKey() != "" {
		t.Errorf("pending damage = status %v awaiting %q, want APPLIED and nothing awaited", got.GetStatus(), got.GetAwaitedDamageTypeKey())
	}
	if sneak := damagePartOf(got, "sneak-attack"); sneak == nil || sneak.GetSelected() {
		t.Errorf("Sneak Attack after the second roll = %v, want it still out", sneak)
	}
	if hp := g.hp(t); hp >= g.before || got.GetAmount() < 5 {
		t.Errorf("hit points = %d (were %d), amount %d: want the rapier's damage landed", hp, g.before, got.GetAmount())
	}
}

func TestDebugLog(t *testing.T) {
	g := newGroupHit(t, 12)
	if _, err := g.takeOut(t, "sneak-attack", "sem aliado perto"); err != nil {
		t.Fatal(err)
	}
	for _, u := range []*user{g.a.master, g.a.caio, g.a.ana} {
		l := g.a.log(t, u, g.e)
		for _, r := range l.GetRounds() {
			for _, en := range r.GetEntries() {
				t.Logf("%p kind=%v reason=%q key=%q damage=%v", u, en.GetKind(), en.GetReason(), en.GetKeyNamePt(), en.GetDamage() != nil)
			}
		}
	}
}
