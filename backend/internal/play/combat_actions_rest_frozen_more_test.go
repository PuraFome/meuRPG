package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The scenarios of the golden that settle a hit: the usual fight, a raging barbarian, a half-orc, a druid in a
// beast form, a spell's damage and an opportunity attack.

// actionsRestRageHit is the table of rollsTable: Brisa, a barbarian, rages and the goblin hits her.
func actionsRestRageHit(face int32) actionsRestHit {
	return func(t *testing.T) (*armed, *playv1.Encounter, string) {
		t.Helper()
		a, _, id := rageTarget(face)(t)
		e := a.get(t, a.master)
		return a, e, id
	}
}

// actionsRestHalfOrcWorld is raceFight with a half-orc fighter, Ragna.
func actionsRestHalfOrcWorld(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	return raceFight(t, "race:half-orc", "class:fighter", 1)
}

// actionsRestShapeWorld is the druid Sálvia as a wolf in her fight.
func actionsRestShapeWorld(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	s := newShapers(t)
	s.fight(t)
	e := s.mustAssume(t, s.bia, s.bri, wolfKey).GetEncounter()
	return s.armed, e
}

// actionsRestFlame is Brisa's Flame Strike on Toren, who fails the save: the fire damage (which 0) or the
// radiant one (which 1), after the earlier ones were rolled and applied.
func actionsRestFlame(which int, concentrates bool) actionsRestHit {
	return func(t *testing.T) (*armed, *playv1.Encounter, string) {
		t.Helper()
		a, e := flameStrikeFixture(t)
		if concentrates {
			a.concentrate(t, "Toren", webSpell)
		}
		fire, radiant := flameStrikeOn(t, a, e, "Toren")
		if which == 0 {
			return a, e, fire
		}
		actionsRestRolled(t, a, e, fire, 10)
		if _, err := a.settle(t, a.master, e, fire, true); err != nil {
			t.Fatalf("ApplyPendingDamage(fire) error = %v", err)
		}
		return a, e, radiant
	}
}

// actionsRestOpportunity is the goblin's opportunity attack on Toren, who leaves its reach; taken puts another
// combatant on the square he left.
func actionsRestOpportunity(taken bool) actionsRestHit {
	return func(t *testing.T) (*armed, *playv1.Encounter, string) {
		t.Helper()
		c := newCave(t)
		c.oppFight(t)
		c.leaveGoblin(t)
		offer := c.offersOf(t, c.master)[0]
		if taken {
			c.mustMove(t, c.master, "Goblin 2", offer.GetLeftCol(), offer.GetLeftRow())
		}
		res, err := c.offerAttack(t, c.master, "Goblin 1", sword, "Toren", offer.GetId(), d20(15))
		if err != nil || res.GetPendingDamage() == nil {
			t.Fatalf("the goblin's opportunity attack = (%v, %v), want a hit", res, err)
		}
		return c.armed, c.get(t, c.master), res.GetPendingDamage().GetId()
	}
}

// actionsRestHeldHit is the goblin's hit on the rogue Reator, whose damage waits for his Uncanny Dodge.
func actionsRestHeldHit(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a, e := reactorFight(t, rogueFive)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Reator", d20(15))
	return a, e, hit.GetPendingDamage().GetId()
}

// actionsRestHidden hides the goblin after its hit.
func actionsRestHidden(t *testing.T, a *armed, e *playv1.Encounter, _ string) {
	t.Helper()
	setHidden(t, a, e, "Goblin", true)
}

// actionsRestApplies are the hits the master applies.
func actionsRestApplies() []goldenScenario {
	toren := actionsRestToren
	druidHP := func(hp int32) func(t *testing.T, a *armed, e *playv1.Encounter, id string) {
		return func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.correct(t, a.bri, hpIs(hp))
		}
	}
	halfOrcHP := func(hp int32) func(t *testing.T, a *armed, e *playv1.Encounter, id string) {
		return func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.correct(t, a.toren, hpIs(hp))
		}
	}
	halfOrc := actionsRestGoblinHits(actionsRestHalfOrcWorld, "Ragna", 15, nil)
	wolf := actionsRestGoblinHits(actionsRestShapeWorld, "Sálvia", 15, nil)
	onPens := actionsRestGoblinHits(actionsRestUsual, "Pensantus", 15, nil)
	spent := actionsRestSQL(`UPDATE character_vitals SET resources_used = '{"relentless_endurance": 1}'::JSONB`)
	cases := []actionsRestSettle{
		{name: "apply a plain hit"},
		{name: "apply a critical hit", hit: actionsRestOnToren(true), sum: 6},
		{name: "apply a hit on the wizard", hit: onPens},
		{name: "apply a hit that Toren survives with 1 hit point", prep: toren(7, -1, 0)},
		{name: "apply a hit that drops Toren to 0", prep: toren(6, -1, 0)},
		{name: "apply a hit that drops Toren from 1 to 0", prep: toren(1, -1, 0)},
		{name: "apply a hit that temporary hit points absorb", prep: toren(-1, 10, 0)},
		{name: "apply a hit that goes through the temporary hit points", prep: toren(-1, 3, 0)},
		{name: "apply a hit that the temporary hit points and the last point take", prep: toren(5, 3, 0)},
		{name: "apply the master's lower amount", edit: actionsRestAmount(2)},
		{name: "apply the master's higher amount", edit: actionsRestAmount(11)},
		{name: "apply the master's amount equal to the roll", edit: actionsRestAmount(6)},
		{name: "apply the master's amount of 0", edit: actionsRestAmount(0)},
		{name: "apply a massive damage", edit: actionsRestAmount(9999)},
		{name: "apply a massive damage on a critical hit at 0", hit: actionsRestOnToren(true), edit: actionsRestAmount(9999), prep: toren(0, -1, 0)},
		{name: "apply a hit on Toren at 0 hit points", prep: toren(0, -1, 0)},
		{name: "apply a critical hit on Toren at 0 hit points", hit: actionsRestOnToren(true), prep: toren(0, -1, 0)},
		{name: "apply a hit on Toren at 0 hit points with 2 failures", prep: toren(0, -1, 2)},
		{name: "apply a critical hit on Toren at 0 hit points with 2 failures", hit: actionsRestOnToren(true), prep: toren(0, -1, 2)},
		{name: "apply a hit on Toren at 0 hit points with the death saves hidden", prep: actionsRestSteps(toren(0, -1, 1), func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.setRules(t, hiddenDeath)
		})},
		{name: "apply a hit on Toren at 0 hit points and the amount 0", prep: toren(0, -1, 0), edit: actionsRestAmount(0)},
		{name: "apply a hit on Toren who concentrates", prep: actionsRestConcentrating},
		{name: "apply a hit on Toren who concentrates and takes nothing", prep: actionsRestConcentrating, edit: actionsRestAmount(0)},
		{name: "apply a hit on Toren who concentrates and falls to 0", prep: actionsRestSteps(toren(6, -1, 0), actionsRestConcentrating)},
		{name: "apply a hit on Toren at 0 hit points who concentrates", prep: actionsRestSteps(toren(0, -1, 0), actionsRestConcentrating)},
		{name: "apply a hit on Toren whose temporary hit points absorb it, who concentrates", prep: actionsRestSteps(toren(-1, 10, 0), actionsRestConcentrating)},
		{name: "apply a hit of a hidden goblin", prep: actionsRestHidden},
		{name: "apply a raging barbarian's resistance", hit: actionsRestRageHit(18)},
		{name: "apply a raging barbarian's resistance on a critical hit", hit: actionsRestRageHit(20), sum: 7},
		{name: "apply a raging barbarian's hit leaving out the rage", hit: actionsRestRageHit(18), edit: actionsRestIgnoring("feature:rage")},
		{name: "apply a raging barbarian's hit leaving out another source", hit: actionsRestRageHit(18), edit: actionsRestIgnoring("feature:other")},
		{name: "apply a raging barbarian's hit with the master's amount", hit: actionsRestRageHit(18), edit: actionsRestAmount(9)},
		{name: "apply a raging barbarian's hit with the master's amount equal to the steps", hit: actionsRestRageHit(18), edit: actionsRestAmount(3)},
		{name: "apply a raging barbarian's hit leaving out the rage and the master's amount", hit: actionsRestRageHit(18), edit: func(r *playv1.ApplyPendingDamageRequest) {
			actionsRestIgnoring("feature:rage")(r)
			actionsRestAmount(6)(r)
		}},
		{name: "apply a half-orc's hit that drops it to 0", hit: halfOrc, prep: halfOrcHP(5)},
		{name: "apply a half-orc's hit that drops it to 0 with the endurance spent", hit: halfOrc, prep: actionsRestSteps(halfOrcHP(5), spent)},
		{name: "apply a half-orc's hit that leaves it standing", hit: halfOrc},
		{name: "apply a half-orc's massive damage", hit: halfOrc, prep: halfOrcHP(5), edit: actionsRestAmount(9999)},
		{name: "apply a hit on a druid in a beast form", hit: wolf},
		{name: "apply a critical hit that carries over from the beast", hit: actionsRestGoblinHits(actionsRestShapeWorld, "Sálvia", 20, nil), sum: 12, edit: actionsRestAmount(15)},
		{name: "apply a hit that drops the beast exactly", hit: wolf, edit: actionsRestAmount(11)},
		{name: "apply a hit that drops the beast and some of the druid", hit: wolf, edit: actionsRestAmount(20)},
		{name: "apply a hit that drops the beast and the druid", hit: wolf, edit: actionsRestAmount(200)},
		{name: "apply a hit on a druid in a beast form who concentrates", hit: wolf, prep: func(t *testing.T, a *armed, _ *playv1.Encounter, _ string) {
			t.Helper()
			a.concentrate(t, "Sálvia", webSpell)
		}},
		{name: "apply a hit on a druid at 1 hit point under the beast", hit: wolf, prep: druidHP(1), edit: actionsRestAmount(40)},
		{name: "apply the fire of Flame Strike on Toren", hit: actionsRestFlame(0, false)},
		{name: "apply the fire of Flame Strike on Toren who concentrates", hit: actionsRestFlame(0, true)},
		{name: "apply the radiant of Flame Strike on Toren after the fire", hit: actionsRestFlame(1, false)},
		{name: "apply the radiant of Flame Strike on Toren who concentrates after the fire", hit: actionsRestFlame(1, true)},
		{name: "apply the radiant of Flame Strike on Toren at 0 hit points after the fire", hit: actionsRestFlame(1, true), prep: toren(0, -1, 0)},
		{name: "apply the radiant of Flame Strike on Toren with no damage", hit: actionsRestFlame(1, true), edit: actionsRestAmount(0)},
		{name: "apply a damage that waits for the Uncanny Dodge", hit: actionsRestHeldHit},
		{name: "apply an opportunity attack that sends the mover back", hit: actionsRestOpportunity(false), prep: toren(6, -1, 0)},
		{name: "apply an opportunity attack that finds the square taken", hit: actionsRestOpportunity(true), prep: toren(6, -1, 0)},
		{name: "apply an opportunity attack that leaves the mover standing", hit: actionsRestOpportunity(false)},
	}
	return actionsRestSettleScenarios(cases)
}

// actionsRestDiscards are the hits the master discards.
func actionsRestDiscards() []goldenScenario {
	cases := []actionsRestSettle{
		{name: "discard a rolled hit", discard: true},
		{name: "discard a hit not rolled yet", discard: true, unrolled: true},
		{name: "discard a critical hit", discard: true, hit: actionsRestOnToren(true), sum: 6},
		{name: "discard a hit of a hidden goblin", discard: true, prep: actionsRestHidden},
		{name: "discard a hit on the wizard", discard: true, hit: actionsRestGoblinHits(actionsRestUsual, "Pensantus", 15, nil)},
		{name: "discard Toren's hit on the goblin", discard: true, hit: actionsRestHitOnGoblin, unrolled: true},
		{name: "discard a raging barbarian's hit", discard: true, hit: actionsRestRageHit(18)},
		{name: "discard the fire of Flame Strike", discard: true, hit: actionsRestFlame(0, false)},
		{name: "discard the fire of Flame Strike on Toren who concentrates", discard: true, hit: actionsRestFlame(0, true)},
		{name: "discard the radiant of Flame Strike after the fire landed", discard: true, hit: actionsRestFlame(1, true)},
		{name: "discard the radiant of Flame Strike after the fire landed on Toren who does not concentrate", discard: true, hit: actionsRestFlame(1, false)},
		{name: "discard an opportunity attack", discard: true, hit: actionsRestOpportunity(false)},
		{name: "discard a damage that waits for the Uncanny Dodge", discard: true, hit: actionsRestHeldHit},
		{name: "discard a healing", discard: true, hit: actionsRestHeal, unrolled: true},
	}
	return actionsRestSettleScenarios(cases)
}

// actionsRestHeal is Brisa's Cure Wounds on Toren, which waits for its roll.
func actionsRestHeal(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a, e, id := healFight(t, "Toren", nil)
	return a, e, id
}

// actionsRestSettlesTwice is a retry of one request: the same key is the same answer.
func actionsRestSettlesTwice() []goldenScenario {
	replay := func(name string, discard bool) goldenScenario {
		return goldenScenario{name: name, run: func(t *testing.T) string {
			t.Helper()
			a, e, id := actionsRestOnToren(false)(t)
			actionsRestRolled(t, a, e, id, 4)
			e = a.get(t, a.master)
			key := newKey()
			var err error
			if discard {
				_, err = a.actionsRestDiscard(t, a.master, e, id, func(r *playv1.DiscardPendingDamageRequest) { r.IdempotencyKey = key })
			} else {
				_, err = a.actionsRestApply(t, a.master, e, id, func(r *playv1.ApplyPendingDamageRequest) { r.IdempotencyKey = key })
			}
			if err != nil {
				t.Fatalf("settle error = %v", err)
			}
			before := a.afterSetup(t)
			names := labelsOf(e)
			if discard {
				res, err := a.actionsRestDiscard(t, a.master, e, id, func(r *playv1.DiscardPendingDamageRequest) { r.IdempotencyKey = key })
				return a.actionsRestReport(t, before, actionsRestHead(res.GetPendingDamage(), names, err))
			}
			res, err := a.actionsRestApply(t, a.master, e, id, func(r *playv1.ApplyPendingDamageRequest) { r.IdempotencyKey = key })
			return a.actionsRestReport(t, before, actionsRestHead(res.GetPendingDamage(), names, err))
		}}
	}
	return []goldenScenario{
		replay("apply the same request twice", false),
		replay("discard the same request twice", true),
	}
}

// actionsRestScenarios are the scenarios of the golden, in the order they are listed.
func actionsRestScenarios() []goldenScenario {
	var all []goldenScenario
	all = append(all, actionsRestSettleRefusals()...)
	all = append(all, actionsRestApplies()...)
	all = append(all, actionsRestDiscards()...)
	all = append(all, actionsRestSettlesTwice()...)
	all = append(all, actionsRestAdjustments()...)
	all = append(all, actionsRestOptionScenarios()...)
	return all
}

// TestActionsRestIsFrozen runs every scenario on a fight of its own and compares what each reads with the golden.
func TestActionsRestIsFrozen(t *testing.T) {
	t.Parallel()
	runGolden(t, actionsRestGolden, actionsRestScenarios())
}
