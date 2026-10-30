package leaktest

import (
	"bytes"
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
	crabDC   = 11 // the escape DC of the claw of monster:giant-crab
	spiderDC = 11 // the bite of monster:giant-spider
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

// statBlockNeedles registers what a player never reads of a creature's action: the key of the stat block
// and of its action (they name the creature's type) and the numbers of the stat block (the DC and the escape
// DC are searched in the fields named like them).
func (w *world) statBlockNeedles(monster, action string) {
	w.secrets.add(&canary{needle: monster + "#" + action, kind: "creature-action-key"})
	w.secrets.add(&canary{needle: monster, kind: "creature-key"})
}

// checkNoStatBlock fails when a player's reads (and the answers they hold) have a finding of the matrix,
// and fails when the master's do not hold every needle given (the needle is unreachable).
func (w *world) checkNoStatBlock(t *testing.T, master []reply, held []reply, positive ...*canary) {
	t.Helper()
	for _, p := range []*person{w.ana, w.caio} {
		for _, r := range w.readsOf(p, held...) {
			for _, f := range w.inspect(p, r, nil) {
				t.Errorf("%s: %s", p.name, f)
			}
		}
	}
	all := append(w.reads(w.master), master...)
	for _, c := range positive {
		found := false
		for _, r := range all {
			if c.numeric {
				found = found || holdsNumber(r, c)
			} else {
				found = found || bytes.Contains(r.body, []byte(c.needle))
			}
		}
		if !found {
			t.Errorf("the master's answers do not hold %s: the needle is unreachable", c)
		}
	}
}

// logOf is the entries of the master's log, to read what a line holds.
func (w *world) logOf() []*playv1.CombatLogEntry {
	var out []*playv1.CombatLogEntry
	for _, r := range must(w.master.combat.ListCombatLog(w.t.Context(), rq(&playv1.ListCombatLogRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId()}))).GetRounds() {
		out = append(out, r.GetEntries()...)
	}
	return out
}

// settleAll rolls (in the app) and applies every pending damage the master's turn options list.
func (w *world) settleAll(from *playv1.Combatant) {
	w.t.Helper()
	ctx := w.t.Context()
	opts := must(w.master.combat.GetTurnOptions(ctx, rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: from.GetId()})))
	for _, p := range opts.GetPendingDamages() {
		if p.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL {
			must(w.master.combat.RollDamage(ctx, rq(&playv1.RollDamageRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: p.GetId(), IdempotencyKey: newKey(),
				Roll: &playv1.RollDamageRequest_RollInApp{RollInApp: true},
			})))
			break // one roll settles the whole cast's damage of the type
		}
	}
	opts = must(w.master.combat.GetTurnOptions(ctx, rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: from.GetId()})))
	for _, p := range opts.GetPendingDamages() {
		if p.GetStatus() == playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_ROLLED {
			must(w.master.combat.ApplyPendingDamage(ctx, rq(&playv1.ApplyPendingDamageRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: p.GetId(), IdempotencyKey: newKey(),
			})))
		}
	}
}

// The master breathes the hell hound's fire on Toren and settles the damage. Toren's player reads the save of
// his character and the damage, never the DC, the stat block's key (not in the line of the action, nor in the
// lines of the damage that follow), nor the text of the action; the other player reads nothing of it. A hidden
// hound names no one.
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
			w.statBlockNeedles("monster:hell-hound", "fire-breath")
			hound := w.monsterOnTurn("monster:hell-hound", name, hidden)

			res := w.master.call(playv1connect.CombatServiceUseCreatureActionProcedure, &playv1.UseCreatureActionRequest{
				CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: hound.GetId(), ActionKey: "monster:hell-hound#fire-breath",
				TargetIds: []string{w.combatant(w.toren).GetId()}, IdempotencyKey: newKey(),
				Area: &playv1.UseCreatureActionRequest_Direction{Direction: &playv1.SpellDirection{Dx: -1}},
			})
			if !res.ok() {
				t.Fatalf("the breath: status %d %s", res.status, res.body)
			}
			// The text of the action is the master's: its words are a needle once read from the master's log.
			text := ""
			for _, e := range w.logOf() {
				if ca := e.GetCreatureAction(); ca != nil && ca.GetText() != "" {
					text = ca.GetText()
				}
			}
			if len(text) < 20 {
				t.Fatalf("the master's line of the breath has no text of the action (%q)", text)
			}
			words := w.secrets.add(&canary{needle: text[:20], kind: "creature-action-text"})
			w.settleAll(hound)
			w.checkNoStatBlock(t, []reply{res}, nil, dc, words, w.canaryNamed("creature-action-key"), w.canaryNamed("creature-key"))
		})
	}
}

// canaryNamed finds the first needle (not a number) of a kind.
func (w *world) canaryNamed(kind string) *canary {
	w.t.Helper()
	for _, c := range w.secrets.list {
		if c.kind == kind {
			return c
		}
	}
	w.t.Fatalf("no needle of the kind %q", kind)
	return nil
}

// firstAttack is the key of the first attack on the creature's turn options.
func (w *world) firstAttack(who *playv1.Combatant) string {
	w.t.Helper()
	opts := must(w.master.combat.GetTurnOptions(w.t.Context(), rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: who.GetId()})))
	for _, a := range opts.GetOptions().GetAttacks() {
		return a.GetAttack().GetKey()
	}
	w.t.Fatal("the creature has no attack")
	return ""
}

// hitToren makes the creature hit Toren (a natural 20 typed by the master), rolls and applies the damage, and
// returns the rider the hit offered.
func (w *world) hitToren(who *playv1.Combatant, attackKey string) string {
	w.t.Helper()
	ctx := w.t.Context()
	// Temporary hit points, so that the hit does not down Toren: a downed target is offered no rider.
	must(w.master.play.AdjustCharacterVitals(ctx, rq(&playv1.AdjustCharacterVitalsRequest{
		CampaignId: w.campaign, CharacterId: w.toren.GetId(), IdempotencyKey: newKey(), HitPointsTemporary: new(int32(500)),
	})))
	hit := must(w.master.combat.RollAttack(ctx, rq(&playv1.RollAttackRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), AttackerId: who.GetId(), AttackKey: attackKey, TargetId: w.combatant(w.toren).GetId(),
		IdempotencyKey: newKey(), Roll: &playv1.RollAttackRequest_D20Face{D20Face: 20},
	})))
	if hit.GetPendingDamage().GetId() == "" {
		w.t.Fatal("the attack left no damage to roll")
	}
	must(w.master.combat.RollDamage(ctx, rq(&playv1.RollDamageRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: hit.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
		Roll: &playv1.RollDamageRequest_RollInApp{RollInApp: true},
	})))
	must(w.master.combat.ApplyPendingDamage(ctx, rq(&playv1.ApplyPendingDamageRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), PendingDamageId: hit.GetPendingDamage().GetId(), IdempotencyKey: newKey(),
	})))
	enc := must(w.master.combat.GetEncounter(ctx, rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter()
	for _, o := range enc.GetHitRiders() {
		if o.GetKind() == playv1.HitRiderKind_HIT_RIDER_KIND_CREATURE {
			return o.GetId()
		}
	}
	w.t.Fatal("the hit offered no rider to the master")
	return ""
}

// answerRider applies a rider as the master.
func (w *world) applyRider(rider string) reply {
	w.t.Helper()
	ans := w.master.call(playv1connect.CombatServiceUseHitRiderProcedure, &playv1.UseHitRiderRequest{
		CampaignId: w.campaign, EncounterId: w.encounter.GetId(), RiderId: rider, Choice: playv1.HitRiderChoice_HIT_RIDER_CHOICE_APPLY, IdempotencyKey: newKey(),
	})
	if !ans.ok() {
		w.t.Fatalf("the rider: status %d %s", ans.status, ans.body)
	}
	return ans
}

// A ghoul's claws hit Toren and the master applies the paralysis: the offer, the answer and the log hold the
// DC of the claws, the key of the stat block and, for a save that held, the condition, for the master alone.
// The save is rolled by the server, so what a player may read of the condition depends on how it ended: when
// the save held, the condition, its duration and its repeating are the master's.
func TestACreatureRiderShowsThePlayersTheirSaveNeverTheStatBlock(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run("hidden "+strconv.FormatBool(hidden), func(t *testing.T) {
			w := newWorld(t)
			name := w.secrets.public("ghoul-name")
			if hidden {
				name = w.secrets.marker("ghoul-name")
			}
			w.secrets.number(creature, ghoulDC, "dc")
			dc := w.canaryOf(creature)
			w.statBlockNeedles("monster:ghoul", "claws")
			ghoul := w.monsterOnTurn("monster:ghoul", name, hidden)
			key := ""
			for _, a := range must(w.master.combat.GetTurnOptions(t.Context(), rq(&playv1.GetTurnOptionsRequest{CampaignId: w.campaign, EncounterId: w.encounter.GetId(), CombatantId: ghoul.GetId()}))).GetOptions().GetAttacks() {
				if key == "" || a.GetAttack().GetName() == "Garras" {
					key = a.GetAttack().GetKey()
				}
			}
			rider := w.hitToren(ghoul, key)
			// The offer waits: the players read nothing of it, and the master's has the condition it would give.
			cond := ""
			for _, o := range must(w.master.combat.GetEncounter(t.Context(), rq(&playv1.GetEncounterRequest{CampaignId: w.campaign}))).GetEncounter().GetHitRiders() {
				cond = o.GetConditionKey()
			}
			if cond == "" {
				t.Fatal("the master's offer names no condition")
			}
			condition := w.secrets.add(&canary{needle: cond, kind: "creature-condition-key"})
			w.checkNoStatBlock(t, nil, nil, dc, condition)

			ans := w.applyRider(rider)
			var line *playv1.CombatLogHitRider
			for _, e := range w.logOf() {
				if r := e.GetHitRider(); r != nil {
					line = r
				}
			}
			if line == nil {
				t.Fatal("the master's log has no line of the rider")
			}
			if line.GetResult() == playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED {
				// A save that failed gave the condition, which Toren's player and the others see on him.
				w.secrets.allowNeedle(cond, w.ana, w.caio)
			}
			w.checkNoStatBlock(t, []reply{ans}, nil, dc, w.canaryNamed("creature-action-key"))
		})
	}
}

// A giant crab's claw grapples Toren with no save: the master applies the rider, and the escape DC of the
// grapple, the key of the stat block and the offer's fields are the master's alone.
func TestACreatureGrappleShowsThePlayersNeitherTheEscapeDCNorTheStatBlock(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run("hidden "+strconv.FormatBool(hidden), func(t *testing.T) {
			w := newWorld(t)
			name := w.secrets.public("crab-name")
			if hidden {
				name = w.secrets.marker("crab-name")
			}
			w.secrets.number("creature-escape-dc", crabDC, "escape_dc")
			escape := w.canaryOf("creature-escape-dc")
			w.statBlockNeedles("monster:giant-crab", "claw")
			crab := w.monsterOnTurn("monster:giant-crab", name, hidden)
			rider := w.hitToren(crab, w.firstAttack(crab))
			w.checkNoStatBlock(t, nil, nil, escape, w.canaryNamed("creature-action-key"))

			ans := w.applyRider(rider)
			w.checkNoStatBlock(t, []reply{ans}, nil, escape)
		})
	}
}

// A giant spider's bite hits Toren and the master applies the rider: the save is rolled by the server and the
// poison it governs is opened as pending damage, then rolled and applied. The damage lines are on the rider's
// line of the log (the master reads them all, with the stat block's key and DC); Toren's player and the other
// player read the damage but never the DC, the key of the bite, nor the text of the action.
func TestACreatureRiderDamageShowsThePlayersNoStatBlock(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run("hidden "+strconv.FormatBool(hidden), func(t *testing.T) {
			w := newWorld(t)
			name := w.secrets.public("spider-name")
			if hidden {
				name = w.secrets.marker("spider-name")
			}
			w.secrets.number(creature, spiderDC, "dc")
			dc := w.canaryOf(creature)
			w.statBlockNeedles("monster:giant-spider", "bite")
			spider := w.monsterOnTurn("monster:giant-spider", name, hidden)
			rider := w.hitToren(spider, w.firstAttack(spider))
			ans := w.applyRider(rider)
			w.settleAll(spider)

			// Positive control: the rider's line of the master's log holds the damage, rolled and applied.
			var damage *playv1.CombatLogDamage
			for _, e := range w.logOf() {
				if r := e.GetHitRider(); r != nil {
					damage = r.GetDamage()
				}
			}
			if damage == nil || damage.GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_APPLIED || damage.GetAmount() <= 0 {
				t.Fatalf("the master's line of the rider holds the damage %v, want it rolled and applied", damage)
			}
			w.checkNoStatBlock(t, []reply{ans}, nil, dc, w.canaryNamed("creature-action-key"), w.canaryNamed("creature-key"))
		})
	}
}
