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
	return newGroupHitWith(t, sum, nil)
}

// newGroupHitWith is newGroupHit with a step before the roll (a state of the target the damage depends on).
func newGroupHitWith(t *testing.T, sum int32, before func(g *groupHit)) *groupHit {
	t.Helper()
	a, _, shrub := partsFight(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, abilities(16, 16, 14, 12, 10), []string{rapier}, nil)
	}, "monster:awakened-shrub")
	e := a.passTo(t, a.get(t, a.master), "Toren")
	a.setConditions(t, e, shrub, "condition:restrained")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, shrub, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{5, 18} })
	g := &groupHit{a: a, e: e, shrub: shrub, id: hit.GetPendingDamage().GetId()}
	g.before, _, _ = a.hp(t, shrub)
	if before != nil {
		before(g)
	}
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

// pending is the damage as the master reads it in Toren's turn options.
func (g *groupHit) pending(t *testing.T) *playv1.PendingDamage {
	t.Helper()
	opts, err := g.a.options(t, g.a.master, g.e, "Toren")
	if err != nil {
		t.Fatalf("GetTurnOptions(Toren) error = %v", err)
	}
	for _, p := range opts.GetPendingDamages() {
		if p.GetId() == g.id {
			return p
		}
	}
	t.Fatalf("pending damage %s is not in the turn options", g.id)
	return nil
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

// The limit is the undo's: another event after the damage, and the damage can no longer be taken back. Nothing changes.
func TestTakingAPartOutIsRefusedWhenSomethingWasDoneAfterTheDamage(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	hurt := g.hp(t)
	if _, err := g.a.adjustHP(t, g.e, g.shrub, damageHP(1)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	after := g.hp(t)
	_, err := g.takeOut(t, "sneak-attack", "depois")
	wantBlockedBy(t, "RemoveDamagePart(after another action)", err, blockedNothingToUndo)
	if got := g.hp(t); got != after || after >= hurt {
		t.Errorf("hit points = %d, want %d: nothing written", got, after)
	}
	if status, _ := g.a.pendingOf(t, g.id); status != pendingApplied {
		t.Errorf("status = %q, want it untouched", status)
	}
}

// The target's hit points changed with no event to close the chain: UNDO_TARGET_CHANGED names the target, nothing is written.
func TestTakingAPartOutIsRefusedWhenTheTargetsHitPointsChanged(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.a.h.pool.Exec(t.Context(), `UPDATE combatants SET hp_current = hp_current + 1 WHERE id = $1`, g.a.id(t, g.shrub)); err != nil {
		t.Fatalf("change the hit points: %v", err)
	}
	changed := g.hp(t)
	_, err := g.takeOut(t, "sneak-attack", "mudou")
	b := wantBlockedBy(t, "RemoveDamagePart(hit points changed)", err, blockedUndoTargetChanged)
	if want := g.a.id(t, g.shrub); len(b.GetCombatantIds()) != 1 || b.GetCombatantIds()[0] != want {
		t.Errorf("combatant_ids = %v, want [%s]", b.GetCombatantIds(), want)
	}
	if got := g.hp(t); got != changed {
		t.Errorf("hit points = %d, want %d: nothing written", got, changed)
	}
	if status, _ := g.a.pendingOf(t, g.id); status != pendingApplied {
		t.Errorf("status = %q, want it untouched", status)
	}
}

// A retry with the same key answers the same and writes nothing more.
func TestTakingAPartOutIsIdempotent(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	req := &playv1.RemoveDamagePartRequest{CampaignId: g.a.campaignID, EncounterId: g.e.GetId(), PendingDamageId: g.id, PartKey: "sneak-attack", Reason: "igual", IdempotencyKey: newKey()}
	first, err := g.a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	second, err := g.a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("RemoveDamagePart(retry) error = %v", err)
	}
	if first.Msg.GetPendingDamage().GetStatus() != second.Msg.GetPendingDamage().GetStatus() || g.hp(t) != g.before {
		t.Errorf("retry = %v, hit points %d (want %d)", second.Msg.GetPendingDamage().GetStatus(), g.hp(t), g.before)
	}
}

// The weapon, and a part the damage waits on, stay as they are: only an extra comes out. The waiting damage cannot be applied.
func TestAGroupPartThatIsNotAnExtraStaysAndTheWaitingDamageIsNotApplied(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	_, err := g.takeOut(t, partWeapon, "arma")
	wantBlockedBy(t, "RemoveDamagePart(the weapon)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
	if _, err := g.takeOut(t, "sneak-attack", "ok"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	_, err = g.takeOut(t, "sneak-attack", "de novo")
	wantBlockedBy(t, "RemoveDamagePart(while it waits for the sum)", err, blockedNotRolled)
	_, err = g.a.settle(t, g.a.master, g.e, g.id, true)
	wantBlockedBy(t, "ApplyPendingDamage(waiting for the sum)", err, blockedNotRolled)
}

// The master discards the waiting damage; a player cannot end the turn over it.
func TestTheDamageWaitingForASumCanBeDiscardedAndBlocksTheTurnLikeARoll(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	_, err := g.a.endTurn(t, g.a.caio, g.e, false)
	wantBlockedBy(t, "EndTurn(waiting for the sum)", err, blockedPendingDamage)
	got, err := g.a.settle(t, g.a.master, g.e, g.id, false)
	if err != nil {
		t.Fatalf("DiscardPendingDamage() error = %v", err)
	}
	if got.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_DISCARDED {
		t.Errorf("status = %v, want DISCARDED", got.GetStatus())
	}
	if hp := g.hp(t); hp != g.before {
		t.Errorf("hit points = %d, want %d", hp, g.before)
	}
}

// The undo of the second roll goes back to the wait for the sum, with the part still out.
func TestTheSecondRollOfAGroupSumCanBeUndone(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.takeOut(t, "sneak-attack", "x"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	if _, err := g.a.damage(t, g.a.caio, g.e, g.id, typedDamage(5)); err != nil {
		t.Fatalf("RollDamage() error = %v", err)
	}
	if err := g.a.undo(t, g.a.master, g.e, g.a.log(t, g.a.master, g.e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction() error = %v", err)
	}
	if g.hp(t) != g.before {
		t.Errorf("hit points = %d, want %d", g.hp(t), g.before)
	}
	p := g.pending(t)
	if p.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED || damagePartOf(p, "sneak-attack").GetSelected() {
		t.Errorf("pending damage = status %v, want it open with Sneak Attack still out", p.GetStatus())
	}
}

// RN-10: the master and the attacker's player read why the extra is gone; another player reads only that it is gone.
func TestTheReasonOfAGroupTakeOutIsReadByTheMasterAndTheAttackerOnly(t *testing.T) {
	t.Parallel()
	g := newGroupHit(t, 12)
	if _, err := g.takeOut(t, "sneak-attack", "segredo do mestre"); err != nil {
		t.Fatalf("RemoveDamagePart() error = %v", err)
	}
	for _, c := range []struct {
		name string
		u    *user
		want bool
	}{{"master", g.a.master, true}, {"attacker", g.a.caio, true}, {"other player", g.a.ana, false}} {
		var removed, reason bool
		for _, r := range g.a.log(t, c.u, g.e).GetRounds() {
			for _, en := range r.GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_DAMAGE_PART_REMOVED {
					removed = true
					reason = reason || en.GetReason() != ""
				}
				if en.GetReason() == "segredo do mestre" && !c.want {
					t.Errorf("%s reads the reason", c.name)
				}
				if d := en.GetDamage(); d != nil && d.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_GROUP_SUM && d.GetAmount() != 0 {
					t.Errorf("%s reads an amount (%d) on a damage that waits for a sum", c.name, d.GetAmount())
				}
			}
		}
		if reason != c.want || (c.want && !removed) {
			t.Errorf("%s: removed %v, reason %v, want reason %v", c.name, removed, reason, c.want)
		}
	}
}

var blockedUndoTargetChanged = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_UNDO_TARGET_CHANGED

// groupSumDamages are the rolls of a damage that waits for the sum of its group (the golden of the damage roll): in the app, with
// a typed sum, and the requests the status refuses.
func groupSumDamages() []goldenScenario {
	waiting := func(name string, edit func(*playv1.RollDamageRequest)) goldenScenario {
		return damageOn("a damage waiting for a group sum: "+name, func(t *testing.T) damagePlay {
			t.Helper()
			g := newGroupHit(t, 12)
			if _, err := g.takeOut(t, "sneak-attack", "motivo"); err != nil {
				t.Fatalf("RemoveDamagePart() error = %v", err)
			}
			g.a.h.roller.queue(6)
			return g.a.rolled(t, g.a.caio, g.e, g.id, edit)
		})
	}
	return []goldenScenario{
		waiting("rolled in the app", inAppDamage),
		waiting("a typed sum", typedSum(5)),
		waiting("a typed sum out of range", typedSum(9)),
		waiting("a typed part of another damage type", typedParts(&playv1.TypedPart{DamageType: groupRadiant, Sum: 3})),
	}
}
