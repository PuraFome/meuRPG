package leaktest

import (
	"strconv"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// What the master does with a creature's full actions (UseCreatureAction, and the riders of its attacks) is
// seen by the players as what happens to them: the save THEY make and the damage they take. The numbers of
// the stat block (the DC of the breath, the DC of the claws) are the master's; the line of a hidden creature
// names no one. The DC is a number no other number of these answers matches, searched in the fields named
// "dc"; the positive controls show it in the master's answers.

const (
	houndDC  = 12 // the Fire Breath of monster:hell-hound
	ghoulDC  = 10 // the claws of monster:ghoul
	creature = "creature-dc"
)

// monsterOnTurn adds the creature next to Toren (it at 13,14, he at 12,14) and passes the turns until it is
// the one on turn.
func (w *world) monsterOnTurn(key, name string, hidden bool) *playv1.Combatant {
	w.t.Helper()
	ctx := w.t.Context()
	m := w.master
	must(m.combat.AddMonsters(ctx, rq(&playv1.AddMonstersRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), IdempotencyKey: newKey(), CreatureKey: key, Count: 1, Name: name, Hidden: &hidden,
	})))
	e := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
	var mon *playv1.Combatant
	for _, c := range e.GetCombatants() {
		if c.GetBestiaryCreatureKey() == key {
			mon = c
		}
	}
	if mon == nil {
		w.t.Fatalf("%s is not in the combat", key)
	}
	for id, sq := range map[string][2]int32{w.combatant(w.toren).GetId(): {12, 14}, mon.GetId(): {13, 14}} {
		must(m.combat.MoveCombatant(ctx, rq(&playv1.MoveCombatantRequest{CampaignId: w.campaign, EncounterId: e.GetId(), CombatantId: id, IdempotencyKey: newKey(), Col: sq[0], Row: sq[1], Forced: true})))
	}
	for range 40 {
		cur := must(m.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
		if cur.GetCurrentCombatantId() == mon.GetId() {
			w.encounter = cur
			return mon
		}
		must(m.combat.EndTurn(ctx, rq(&playv1.EndTurnRequest{CampaignId: w.campaign, EncounterId: e.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: cur.GetCurrentCombatantId(), ExpectedRound: cur.GetRound(), DiscardPendingDamage: true})))
	}
	w.t.Fatalf("the turn of %s never came", key)
	return nil
}

// readsOf are the reads that show the combat and its log, plus the answers the test holds.
func (w *world) readsOf(p *person, held ...reply) []reply {
	return append(w.reads(p), held...)
}

// checkNoStatBlock fails when a player's reads hold a number of the stat block or a finding of the matrix,
// and fails when the master's do not hold the number (the needle is unreachable).
func (w *world) checkNoStatBlock(t *testing.T, dc *canary, master []reply, held map[*person][]reply) {
	t.Helper()
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.readsOf(p, held[p]...) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
			if holdsNumber(r, dc) {
				t.Errorf("%s reads the DC of the stat block", p.name)
			}
		}
	}
	found := false
	for _, r := range append(w.reads(w.master), master...) {
		found = found || holdsNumber(r, dc)
	}
	if !found {
		t.Error("the master's answers do not hold the DC: the needle is unreachable")
	}
}

// The master breathes the hell hound's fire on Toren. Toren's player reads the save of his character and
// the damage, never the DC; the other player reads nothing of it. A hidden hound names no one.
func TestACreatureActionShowsThePlayersTheirSaveNeverTheStatBlock(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run("hidden "+strconv.FormatBool(hidden), func(t *testing.T) {
			w := newWorld(t)
			name := w.secrets.public("hound-name")
			if hidden {
				name = w.secrets.marker("hound-name")
			}
			w.secrets.number(creature, houndDC, "dc")
			dc := w.canaryOf(creature)
			hound := w.monsterOnTurn("monster:hell-hound", name, hidden)

			res := w.master.call(playv1connect.CombatServiceUseCreatureActionProcedure, &playv1.UseCreatureActionRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: hound.GetId(), ActionKey: "monster:hell-hound#fire-breath",
				TargetIds: []string{w.combatant(w.toren).GetId()}, IdempotencyKey: newKey(),
				Area: &playv1.UseCreatureActionRequest_Direction{Direction: &playv1.SpellDirection{Dx: -1}},
			})
			if !res.ok() {
				t.Fatalf("the breath: status %d %s", res.status, res.body)
			}
			w.checkNoStatBlock(t, dc, []reply{res}, nil)
		})
	}
}

// A ghoul's claws hit Toren and the master applies the paralysis: the offer, the answer and the log hold the
// DC of the claws for the master alone.
func TestACreatureRiderShowsThePlayersTheirSaveNeverTheStatBlock(t *testing.T) {
	w := newWorld(t)
	w.secrets.number(creature, ghoulDC, "dc")
	dc := w.canaryOf(creature)
	ghoul := w.monsterOnTurn("monster:ghoul", w.secrets.public("ghoul-name"), false)
	ctx := t.Context()

	opts := must(w.master.combat.GetTurnOptions(ctx, rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: ghoul.GetId()})))
	key := ""
	for _, a := range opts.GetOptions().GetAttacks() {
		if key == "" || a.GetAttack().GetName() == "Garras" {
			key = a.GetAttack().GetKey()
		}
	}
	if key == "" {
		t.Fatal("the ghoul has no attack")
	}
	hit := must(w.master.combat.RollAttack(ctx, rq(&playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: ghoul.GetId(), AttackKey: key, TargetId: w.combatant(w.toren).GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 20},
	})))
	if hit.GetPendingDamage().GetId() == "" {
		t.Fatal("the claws left no damage to roll")
	}
	must(w.master.combat.RollDamage(ctx, rq(&playv1.RollDamageRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: hit.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
		Roll: &playv1.RollDamageRequest_RollInApp{RollInApp: true},
	})))
	must(w.master.combat.ApplyPendingDamage(ctx, rq(&playv1.ApplyPendingDamageRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: hit.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
	})))
	enc := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
	rider := ""
	for _, o := range enc.GetHitRiders() {
		if o.GetKind() == playv1.HitRiderKind_HIT_RIDER_KIND_CREATURE {
			rider = o.GetId()
		}
	}
	if rider == "" {
		t.Fatal("the claws offered no rider to the master")
	}
	// The offer waits: the players read nothing of it.
	w.checkNoStatBlock(t, dc, nil, nil)

	ans := w.master.call(playv1connect.CombatServiceUseHitRiderProcedure, &playv1.UseHitRiderRequest{
		CampaignId: w.campaign, EncounterId: enc.GetId(), RiderId: rider, Choice: playv1.HitRiderChoice_HIT_RIDER_CHOICE_APPLY, IdempotencyKey: newKey(),
	})
	if !ans.ok() {
		t.Fatalf("the rider: status %d %s", ans.status, ans.body)
	}
	w.checkNoStatBlock(t, dc, []reply{ans}, nil)
}
