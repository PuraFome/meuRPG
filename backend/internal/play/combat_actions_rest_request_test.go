package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The requests of the master's hand on a hit and on the goblin's hit points that are refused for a field of
// the request, one table for each call on one fight: the exact code and message of each, and that none writes
// anything.

// actionsRestRequestFight is the usual fight with the goblin's hit on Toren rolled and waiting for the master.
func actionsRestRequestFight(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a, e, id := actionsRestOnToren(false)(t)
	actionsRestRolled(t, a, e, id, 4)
	return a, e, id
}

func TestAdjustCombatantHitPointsRefusesTheRequest(t *testing.T) {
	t.Parallel()
	a, e, _ := actionsRestRequestFight(t)
	missing := "00000000-0000-4000-8000-000000000001"
	for _, c := range []struct {
		name string
		edit func(*playv1.AdjustCombatantHitPointsRequest)
		code connect.Code
		msg  string
	}{
		{"an idempotency key that is not a UUID", actionsRestAdjusting(damageHP(1), func(r *playv1.AdjustCombatantHitPointsRequest) { r.IdempotencyKey = "not-a-key" }), connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", actionsRestAdjusting(damageHP(1), func(r *playv1.AdjustCombatantHitPointsRequest) { r.EncounterId = "nope" }), connect.CodeNotFound, "encounter not found"},
		{"a combatant that is not a UUID", actionsRestAdjusting(damageHP(1), func(r *playv1.AdjustCombatantHitPointsRequest) { r.CombatantId = "nope" }), connect.CodeNotFound, "combatant not found"},
		{"a campaign that is not a UUID", actionsRestAdjusting(damageHP(1), func(r *playv1.AdjustCombatantHitPointsRequest) { r.CampaignId = "nope" }), connect.CodeNotFound, "campaign not found"},
		{"a request that changes nothing", nil, connect.CodeInvalidArgument, "set damage, heal, hit_points or hit_points_temporary"},
		{"a negative damage", damageHP(-1), connect.CodeInvalidArgument, "damage must be 0 to 9999"},
		{"a damage over the limit", damageHP(10000), connect.CodeInvalidArgument, "damage must be 0 to 9999"},
		{"a negative heal", actionsRestHealBy(-1), connect.CodeInvalidArgument, "heal must be 0 to 9999"},
		{"a heal over the limit", actionsRestHealBy(10000), connect.CodeInvalidArgument, "heal must be 0 to 9999"},
		{"a negative hit point count", actionsRestSetHP(-1), connect.CodeInvalidArgument, "set must be 0 to 9999"},
		{"a hit point count over the limit", actionsRestSetHP(10000), connect.CodeInvalidArgument, "set must be 0 to 9999"},
		{"a hit point count over the maximum", actionsRestSetHP(8), connect.CodeInvalidArgument, "hit_points must be 0 to 7"},
		{"negative temporary hit points", actionsRestTempHP(-1), connect.CodeInvalidArgument, "hit_points_temporary must be 0 to 999"},
		{"temporary hit points over the limit", actionsRestTempHP(1000), connect.CodeInvalidArgument, "hit_points_temporary must be 0 to 999"},
		{"temporary hit points next to a damage over the limit", actionsRestAdjusting(damageHP(10000), actionsRestTempHP(1000)), connect.CodeInvalidArgument, "damage must be 0 to 9999"},
		{"a combatant nobody knows", actionsRestAdjusting(damageHP(1), func(r *playv1.AdjustCombatantHitPointsRequest) { r.CombatantId = missing }), connect.CodeNotFound, "combatant not found"},
		{"an encounter nobody knows", actionsRestAdjusting(damageHP(1), func(r *playv1.AdjustCombatantHitPointsRequest) {
			r.EncounterId = "00000000-0000-4000-8000-000000000002"
		}), connect.CodeNotFound, "encounter not found"},
	} {
		before := a.stateSnapshot(t)
		req := &playv1.AdjustCombatantHitPointsRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey()}
		if c.edit != nil {
			c.edit(req)
		}
		_, err := a.master.combat.AdjustCombatantHitPoints(t.Context(), connect.NewRequest(req))
		wantRefusal(t, "adjust "+c.name, err, c.code, c.msg)
		if a.stateSnapshot(t) != before {
			t.Errorf("adjust %s changed the fight", c.name)
		}
	}
}

func TestApplyPendingDamageRefusesTheRequest(t *testing.T) {
	t.Parallel()
	a, e, id := actionsRestRequestFight(t)
	missing := "00000000-0000-4000-8000-000000000001"
	for _, c := range []struct {
		name string
		edit func(*playv1.ApplyPendingDamageRequest)
		code connect.Code
		msg  string
	}{
		{"an idempotency key that is not a UUID", func(r *playv1.ApplyPendingDamageRequest) { r.IdempotencyKey = "not-a-key" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", func(r *playv1.ApplyPendingDamageRequest) { r.EncounterId = "nope" }, connect.CodeNotFound, "encounter not found"},
		{"a pending damage that is not a UUID", func(r *playv1.ApplyPendingDamageRequest) { r.PendingDamageId = "nope" }, connect.CodeNotFound, "pending damage not found"},
		{"a campaign that is not a UUID", func(r *playv1.ApplyPendingDamageRequest) { r.CampaignId = "nope" }, connect.CodeNotFound, "campaign not found"},
		{"a negative amount", actionsRestAmount(-1), connect.CodeInvalidArgument, "amount must be 0 to 9999"},
		{"an amount over the limit", actionsRestAmount(10000), connect.CodeInvalidArgument, "amount must be 0 to 9999"},
		{"21 sources to ignore", actionsRestIgnoring(strings.Split(strings.Repeat("feature:rage,", 21), ",")[:21]...), connect.CodeInvalidArgument, "ignore_modifiers must have at most 20 sources"},
		{"a pending damage nobody knows", func(r *playv1.ApplyPendingDamageRequest) { r.PendingDamageId = missing }, connect.CodeNotFound, "pending damage not found"},
		{"an encounter nobody knows", func(r *playv1.ApplyPendingDamageRequest) { r.EncounterId = "00000000-0000-4000-8000-000000000002" }, connect.CodeNotFound, "encounter not found"},
	} {
		before := a.stateSnapshot(t)
		_, err := a.actionsRestApply(t, a.master, e, id, c.edit)
		wantRefusal(t, "apply "+c.name, err, c.code, c.msg)
		if a.stateSnapshot(t) != before {
			t.Errorf("apply %s changed the fight", c.name)
		}
	}
}

func TestDiscardPendingDamageRefusesTheRequest(t *testing.T) {
	t.Parallel()
	a, e, id := actionsRestRequestFight(t)
	missing := "00000000-0000-4000-8000-000000000001"
	for _, c := range []struct {
		name string
		edit func(*playv1.DiscardPendingDamageRequest)
		code connect.Code
		msg  string
	}{
		{"an idempotency key that is not a UUID", func(r *playv1.DiscardPendingDamageRequest) { r.IdempotencyKey = "not-a-key" }, connect.CodeInvalidArgument, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", func(r *playv1.DiscardPendingDamageRequest) { r.EncounterId = "nope" }, connect.CodeNotFound, "encounter not found"},
		{"a pending damage that is not a UUID", func(r *playv1.DiscardPendingDamageRequest) { r.PendingDamageId = "nope" }, connect.CodeNotFound, "pending damage not found"},
		{"a pending damage nobody knows", func(r *playv1.DiscardPendingDamageRequest) { r.PendingDamageId = missing }, connect.CodeNotFound, "pending damage not found"},
	} {
		before := a.stateSnapshot(t)
		_, err := a.actionsRestDiscard(t, a.master, e, id, c.edit)
		wantRefusal(t, "discard "+c.name, err, c.code, c.msg)
		if a.stateSnapshot(t) != before {
			t.Errorf("discard %s changed the fight", c.name)
		}
	}
}
