package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// takeActionRepeat takes an action n times first, as the master, who is not stopped by the economy.
func takeActionRepeat(n int, who, key string, edit func(*playv1.TakeActionRequest)) func(t *testing.T, a *armed, e *playv1.Encounter) {
	return func(t *testing.T, a *armed, e *playv1.Encounter) {
		t.Helper()
		for range n {
			if _, err := a.takeActionRaw(t, a.master, e, who, key, edit); err != nil {
				t.Fatalf("TakeAction(%s %s) as the master before the scenario error = %v", who, key, err)
			}
		}
	}
}

// takeActionKeys takes the actions one after the other, as Toren's player.
func takeActionKeys(keys ...string) func(t *testing.T, a *armed, e *playv1.Encounter) {
	return func(t *testing.T, a *armed, e *playv1.Encounter) {
		t.Helper()
		for _, k := range keys {
			takeActionFirst("Toren", k, nil)(t, a, e)
		}
	}
}

// takeActionTimes runs a step n times.
func takeActionTimes(n int, step func(t *testing.T, a *armed, e *playv1.Encounter)) func(t *testing.T, a *armed, e *playv1.Encounter) {
	return func(t *testing.T, a *armed, e *playv1.Encounter) {
		t.Helper()
		for range n {
			step(t, a, e)
		}
	}
}

// takeActionForceDice sets the campaign's dice mode, as the master.
func takeActionForceDice(mode campaignsv1.DiceMode) func(t *testing.T, a *armed, e *playv1.Encounter) {
	return func(t *testing.T, a *armed, _ *playv1.Encounter) {
		t.Helper()
		a.forceDice(t, mode)
	}
}

// takeActionConcentrating makes Toren concentrate on a spell.
func takeActionConcentrating(t *testing.T, a *armed, _ *playv1.Encounter) {
	t.Helper()
	a.execSQL(t, `UPDATE combatants SET concentration_spell = 'spell:bless' WHERE label = 'Toren'`)
}

// takeActionSecondWinds are the fighter's Second Wind and Action Surge.
func takeActionSecondWinds() []takeActionScenario {
	hurt := takeActionHeroHP("Toren", 8)
	cases := []takeActionCase{
		{name: "Second Wind rolled in the app", key: "feature:second-wind", edit: takeActionInApp, prep: takeActionSteps(hurt, takeActionDice(6))},
		{name: "Second Wind rolled in the app at full hit points", key: "feature:second-wind", edit: takeActionInApp, prep: takeActionDice(4)},
		{name: "Second Wind with a typed sum", key: "feature:second-wind", edit: takeActionTyped(7), prep: hurt},
		{name: "Second Wind with the highest typed sum", key: "feature:second-wind", edit: takeActionTyped(10), prep: hurt},
		{name: "Second Wind with the lowest typed sum", key: "feature:second-wind", edit: takeActionTyped(1), prep: hurt},
		{name: "Second Wind with a typed sum of 0", key: "feature:second-wind", edit: takeActionTyped(0), prep: hurt},
		{name: "Second Wind with a typed sum of 11", key: "feature:second-wind", edit: takeActionTyped(11), prep: hurt},
		{name: "Second Wind with no roll", key: "feature:second-wind", prep: hurt},
		{name: "Second Wind rolled in the app when the master forces real dice", key: "feature:second-wind", edit: takeActionInApp, prep: takeActionForceDice(campaignsv1.DiceMode_DICE_MODE_PHYSICAL)},
		{name: "Second Wind typed when the master forces the app's dice", key: "feature:second-wind", edit: takeActionTyped(5), prep: takeActionForceDice(campaignsv1.DiceMode_DICE_MODE_APP)},
		{name: "Second Wind typed when the player prefers real dice", key: "feature:second-wind", edit: takeActionTyped(5), prep: takeActionSteps(takeActionPhysical, hurt)},
		{name: "Second Wind rolled in the app when the player prefers real dice", key: "feature:second-wind", edit: takeActionInApp, prep: takeActionSteps(takeActionPhysical, hurt, takeActionDice(3))},
		{name: "Second Wind as the master with the app's dice", key: "feature:second-wind", edit: takeActionInApp, as: takeActionAsMaster, prep: takeActionSteps(hurt, takeActionDice(8))},
		{name: "Second Wind as the master with a typed sum", key: "feature:second-wind", edit: takeActionTyped(9), as: takeActionAsMaster, prep: hurt},
		{name: "Second Wind as the master with no roll", key: "feature:second-wind", as: takeActionAsMaster},
		{name: "Second Wind as the master against forced real dice", key: "feature:second-wind", edit: takeActionInApp, as: takeActionAsMaster, prep: takeActionSteps(takeActionForceDice(campaignsv1.DiceMode_DICE_MODE_PHYSICAL), hurt, takeActionDice(2))},
		{name: "Second Wind in a combat without a map", key: "feature:second-wind", edit: takeActionTyped(6), fight: takeActionTheatre, prep: hurt},
		{name: "Second Wind with temporary hit points", key: "feature:second-wind", edit: takeActionTyped(6), prep: takeActionSteps(takeActionHeroHP("Toren", 8), takeActionHeroTemp)},
		{name: "Second Wind after the action is used", key: "feature:second-wind", edit: takeActionTyped(6), prep: takeActionSteps(hurt, takeActionFirst("Toren", "standard:dodge", nil))},
		{name: "a second Second Wind in the same turn", key: "feature:second-wind", edit: takeActionTyped(6), prep: takeActionSteps(hurt, takeActionFirst("Toren", "feature:second-wind", takeActionTyped(3)))},
		{name: "a second Second Wind as the master", key: "feature:second-wind", edit: takeActionTyped(6), as: takeActionAsMaster, prep: takeActionSteps(hurt, takeActionFirst("Toren", "feature:second-wind", takeActionTyped(3)))},
		{name: "Second Wind with no uses left in a later turn", key: "feature:second-wind", edit: takeActionTyped(6), prep: takeActionSteps(hurt, takeActionFirst("Toren", "feature:second-wind", takeActionTyped(3)), takeActionPass("Pensantus"), takeActionPass("Toren"))},
		{name: "Second Wind of an NPC is not a feature it has", key: "feature:second-wind", edit: takeActionTyped(6), who: "Goblin", as: takeActionAsMaster, prep: takeActionPass("Goblin")},
		{name: "Action Surge", key: "feature:action-surge-1-use"},
		{name: "Action Surge after the Attack action", key: "feature:action-surge-1-use", prep: takeActionAttackFirst},
		{name: "Action Surge after a Dodge", key: "feature:action-surge-1-use", prep: takeActionFirst("Toren", "standard:dodge", nil)},
		{name: "Action Surge as the master", key: "feature:action-surge-1-use", as: takeActionAsMaster},
		{name: "Action Surge in a combat without a map", key: "feature:action-surge-1-use", fight: takeActionTheatre},
		{name: "a second Action Surge in the same turn", key: "feature:action-surge-1-use", prep: takeActionFirst("Toren", "feature:action-surge-1-use", nil)},
		{name: "a second Action Surge as the master", key: "feature:action-surge-1-use", as: takeActionAsMaster, prep: takeActionFirst("Toren", "feature:action-surge-1-use", nil)},
		{name: "a Dodge after Action Surge", key: "standard:dodge", prep: takeActionSteps(takeActionFirst("Toren", "standard:dodge", nil), takeActionFirst("Toren", "feature:action-surge-1-use", nil))},
		{name: "a second Dodge after Action Surge and a Dodge", key: "standard:dodge", prep: takeActionKeys("standard:dodge", "feature:action-surge-1-use", "standard:dodge")},
		{name: "Action Surge twice in a turn with two uses", key: "feature:action-surge-1-use", fight: takeActionFightOf(takeActionFighter17, takeActionFighter2, false), prep: takeActionFirst("Toren", "feature:action-surge-1-use", nil)},
		{name: "Action Surge twice in a turn with two uses as the master", key: "feature:action-surge-1-use", fight: takeActionFightOf(takeActionFighter17, takeActionFighter2, false), as: takeActionAsMaster, prep: takeActionFirst("Toren", "feature:action-surge-1-use", nil)},
		{name: "Action Surge in the next turn with two uses", key: "feature:action-surge-1-use", fight: takeActionFightOf(takeActionFighter17, takeActionFighter2, false), prep: takeActionSteps(takeActionFirst("Toren", "feature:action-surge-1-use", nil), takeActionPass("Pensantus"), takeActionPass("Toren"))},
		{name: "Action Surge with the extra action", key: "feature:action-surge-1-use", edit: takeActionExtra},
	}
	out := make([]takeActionScenario, 0, len(cases))
	for _, c := range cases {
		out = append(out, c.scenario())
	}
	return out
}

// takeActionRogueAndMonk are the bonus-action features of the rogue and the monk.
func takeActionRogueAndMonk() []takeActionScenario {
	rogue := takeActionFightOf(takeActionRogue3, takeActionFighter2, false)
	rogueTheatre := takeActionFightOf(takeActionRogue3, takeActionFighter2, true)
	monk := takeActionFightOf(takeActionMonk4, takeActionFighter2, false)
	monkTheatre := takeActionFightOf(takeActionMonk4, takeActionFighter2, true)
	cases := []takeActionCase{
		{name: "Cunning Action: Dash", key: "feature:cunning-action:dash", fight: rogue},
		{name: "Cunning Action: Disengage", key: "feature:cunning-action:disengage", fight: rogue},
		{name: "Cunning Action: Hide, which has its own call", key: "feature:cunning-action:hide", fight: rogue},
		{name: "Cunning Action with no choice", key: "feature:cunning-action", fight: rogue},
		{name: "Cunning Action: Dash as the master", key: "feature:cunning-action:dash", fight: rogue, as: takeActionAsMaster},
		{name: "Cunning Action: Disengage in a combat without a map", key: "feature:cunning-action:disengage", fight: rogueTheatre},
		{name: "Cunning Action: Dash after the action is used", key: "feature:cunning-action:dash", fight: rogue, prep: takeActionFirst("Toren", "standard:dodge", nil)},
		{name: "Cunning Action: Disengage after a Dash", key: "feature:cunning-action:disengage", fight: rogue, prep: takeActionFirst("Toren", "standard:dash", nil)},
		{name: "Cunning Action twice in the same turn", key: "feature:cunning-action:disengage", fight: rogue, prep: takeActionFirst("Toren", "feature:cunning-action:dash", nil)},
		{name: "Cunning Action twice as the master", key: "feature:cunning-action:disengage", fight: rogue, as: takeActionAsMaster, prep: takeActionFirst("Toren", "feature:cunning-action:dash", nil)},
		{name: "a Dash after Cunning Action: Dash", key: "standard:dash", fight: rogue, prep: takeActionFirst("Toren", "feature:cunning-action:dash", nil)},
		{name: "Cunning Action with the extra action", key: "feature:cunning-action:dash", fight: rogue, edit: takeActionExtra},
		{name: "Patient Defense", key: "feature:patient-defense", fight: monk},
		{name: "Patient Defense as the master", key: "feature:patient-defense", fight: monk, as: takeActionAsMaster},
		{name: "Patient Defense in a combat without a map", key: "feature:patient-defense", fight: monkTheatre},
		{name: "Patient Defense after the action is used", key: "feature:patient-defense", fight: monk, prep: takeActionFirst("Toren", "standard:dash", nil)},
		{name: "Patient Defense twice", key: "feature:patient-defense", fight: monk, prep: takeActionFirst("Toren", "feature:patient-defense", nil)},
		{name: "Patient Defense with the ki spent", key: "feature:patient-defense", fight: monk, prep: takeActionRepeat(4, "Toren", "feature:patient-defense", nil)},
		{name: "Patient Defense with the ki spent as the master", key: "feature:patient-defense", fight: monk, as: takeActionAsMaster, prep: takeActionRepeat(4, "Toren", "feature:patient-defense", nil)},
		{name: "Step of the Wind: Dash", key: "feature:step-of-the-wind:dash", fight: monk},
		{name: "Step of the Wind: Disengage", key: "feature:step-of-the-wind:disengage", fight: monk},
		{name: "Step of the Wind: Dash in a combat without a map", key: "feature:step-of-the-wind:dash", fight: monkTheatre},
		{name: "Step of the Wind: Disengage as the master", key: "feature:step-of-the-wind:disengage", fight: monk, as: takeActionAsMaster},
		{name: "Step of the Wind twice", key: "feature:step-of-the-wind:dash", fight: monk, prep: takeActionFirst("Toren", "feature:step-of-the-wind:disengage", nil)},
		{name: "Flurry of Blows before the Attack action", key: "feature:flurry-of-blows", fight: monk},
		{name: "Flurry of Blows before the Attack action as the master", key: "feature:flurry-of-blows", fight: monk, as: takeActionAsMaster},
		{name: "Flurry of Blows after the Attack action", key: "feature:flurry-of-blows", fight: monk, prep: takeActionMonkStrike},
		{name: "Flurry of Blows after the Attack action as the master", key: "feature:flurry-of-blows", fight: monk, as: takeActionAsMaster, prep: takeActionMonkStrike},
		{name: "Flurry of Blows after a Dodge", key: "feature:flurry-of-blows", fight: monk, prep: takeActionFirst("Toren", "standard:dodge", nil)},
		{name: "Flurry of Blows after a Dash", key: "feature:flurry-of-blows", fight: monk, prep: takeActionFirst("Toren", "standard:dash", nil)},
		{name: "Flurry of Blows after the bonus action is used", key: "feature:flurry-of-blows", fight: monk, prep: takeActionSteps(takeActionMonkStrike, takeActionFirst("Toren", "feature:patient-defense", nil))},
		{name: "Flurry of Blows after two strikes", key: "feature:flurry-of-blows", fight: monk, prep: takeActionTimes(2, takeActionMonkStrike)},
		{name: "Flurry of Blows in a combat without a map", key: "feature:flurry-of-blows", fight: monkTheatre, prep: takeActionFirst("Toren", "standard:dodge", nil)},
		{name: "Flurry of Blows with the ki spent", key: "feature:flurry-of-blows", fight: monk, prep: takeActionSteps(takeActionMonkStrike, takeActionRepeat(4, "Toren", "feature:patient-defense", nil))},
		{name: "Slow Fall with no fall", key: "feature:slow-fall", fight: monk},
		{name: "Slow Fall with no fall in a combat without a map", key: "feature:slow-fall", fight: monkTheatre},
		{name: "Slow Fall with no fall, out of turn", key: "feature:slow-fall", who: "Brisa", fight: takeActionFightOf(takeActionFighter2, takeActionMonk4, false)},
		{name: "Slow Fall with no fall, out of turn, as the master", key: "feature:slow-fall", who: "Brisa", as: takeActionAsMaster, fight: takeActionFightOf(takeActionFighter2, takeActionMonk4, false)},
		{name: "Slow Fall by a monk at 0 hit points", key: "feature:slow-fall", who: "Brisa", fight: takeActionFightOf(takeActionFighter2, takeActionMonk4, false), prep: takeActionHeroHP("Brisa", 0)},
		{name: "Slow Fall by a monk after the reaction is used", key: "feature:slow-fall", who: "Brisa", fight: takeActionFightOf(takeActionFighter2, takeActionMonk4, false), prep: takeActionSQL(`UPDATE combatants SET reaction_used = true WHERE label = 'Brisa'`)},
		{name: "Slow Fall by a monk after the reaction is used as the master", key: "feature:slow-fall", who: "Brisa", as: takeActionAsMaster, fight: takeActionFightOf(takeActionFighter2, takeActionMonk4, false), prep: takeActionSQL(`UPDATE combatants SET reaction_used = true WHERE label = 'Brisa'`)},
		{name: "Slow Fall by a surprised monk", key: "feature:slow-fall", who: "Brisa", fight: takeActionFightWith(takeActionFighter2, takeActionMonk4, takeActionStart{surprised: "Brisa"})},
		{name: "Slow Fall by a monk in a combat that did not begin", key: "feature:slow-fall", who: "Brisa", fight: takeActionFightWith(takeActionFighter2, takeActionMonk4, takeActionStart{setup: true})},
	}
	out := make([]takeActionScenario, 0, len(cases))
	for _, c := range cases {
		out = append(out, c.scenario())
	}
	return out
}

// takeActionMonkStrike is the monk's Attack action: an unarmed strike on the goblin.
func takeActionMonkStrike(t *testing.T, a *armed, e *playv1.Encounter) {
	t.Helper()
	a.mustAttack(t, a.caio, e, "Toren", unarmed, "Goblin", d20(15))
}

// takeActionBarbarians are Rage and Reckless Attack.
func takeActionBarbarians() []takeActionScenario {
	barb := takeActionFightOf(takeActionBarb3, takeActionFighter2, false)
	barbTheatre := takeActionFightOf(takeActionBarb3, takeActionFighter2, true)
	berserk := takeActionFightOf(takeActionBerserk3, takeActionFighter2, false)
	cases := []takeActionCase{
		{name: "Rage", key: "feature:rage", fight: barb},
		{name: "Rage as the master", key: "feature:rage", fight: barb, as: takeActionAsMaster},
		{name: "Rage in a combat without a map", key: "feature:rage", fight: barbTheatre},
		{name: "Rage after the action is used", key: "feature:rage", fight: barb, prep: takeActionFirst("Toren", "standard:dodge", nil)},
		{name: "Rage after the Attack action", key: "feature:rage", fight: barb, prep: takeActionAttackFirst},
		{name: "Rage while concentrating", key: "feature:rage", fight: barb, prep: takeActionConcentrating},
		{name: "Rage with Frenzy by a hero that has none", key: "feature:rage", fight: barb, edit: takeActionFrenzy},
		{name: "Rage with Frenzy by a Berserker", key: "feature:rage", fight: berserk, edit: takeActionFrenzy},
		{name: "Rage without Frenzy by a Berserker", key: "feature:rage", fight: berserk},
		{name: "Rage with Frenzy by a Berserker as the master", key: "feature:rage", fight: berserk, edit: takeActionFrenzy, as: takeActionAsMaster},
		{name: "Rage twice in the same turn", key: "feature:rage", fight: barb, prep: takeActionFirst("Toren", "feature:rage", nil)},
		{name: "Rage twice as the master", key: "feature:rage", fight: barb, as: takeActionAsMaster, prep: takeActionFirst("Toren", "feature:rage", nil)},
		{name: "Rage with the uses spent", key: "feature:rage", fight: barb, prep: takeActionRepeat(3, "Toren", "feature:rage", nil)},
		{name: "Rage with the uses spent as the master", key: "feature:rage", fight: barb, as: takeActionAsMaster, prep: takeActionRepeat(3, "Toren", "feature:rage", nil)},
		{name: "Rage with the extra action", key: "feature:rage", fight: barb, edit: takeActionExtra},
		{name: "Reckless Attack", key: "feature:reckless-attack", fight: barb},
		{name: "Reckless Attack as the master", key: "feature:reckless-attack", fight: barb, as: takeActionAsMaster},
		{name: "Reckless Attack in a combat without a map", key: "feature:reckless-attack", fight: barbTheatre},
		{name: "Reckless Attack after the first attack", key: "feature:reckless-attack", fight: barb, prep: takeActionBarbarianAttack},
		{name: "Reckless Attack after the first attack as the master", key: "feature:reckless-attack", fight: barb, as: takeActionAsMaster, prep: takeActionBarbarianAttack},
		{name: "Reckless Attack twice", key: "feature:reckless-attack", fight: barb, prep: takeActionFirst("Toren", "feature:reckless-attack", nil)},
		{name: "Reckless Attack after Rage", key: "feature:reckless-attack", fight: barb, prep: takeActionFirst("Toren", "feature:rage", nil)},
	}
	out := make([]takeActionScenario, 0, len(cases))
	for _, c := range cases {
		out = append(out, c.scenario())
	}
	return out
}

// takeActionBarbarianAttack is the barbarian's first attack of the turn.
func takeActionBarbarianAttack(t *testing.T, a *armed, e *playv1.Encounter) {
	t.Helper()
	a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(15))
}

// takeActionOtherClasses are the flows other classes have and the features of a race.
func takeActionOtherClasses() []takeActionScenario {
	druid := takeActionFightOf(takeActionDruid2, takeActionFighter2, false)
	paladin := takeActionFightOf(takeActionPaladin2, takeActionFighter2, false)
	bard := takeActionFightOf(takeActionBard2, takeActionFighter2, false)
	cases := []takeActionCase{
		{name: "a druid's Wild Shape", key: "feature:wild-shape", fight: druid},
		{name: "a paladin's Lay on Hands", key: "feature:lay-on-hands", fight: paladin},
		{name: "a paladin's Lay on Hands as the master", key: "feature:lay-on-hands", fight: paladin, as: takeActionAsMaster},
		{name: "a bard's Bardic Inspiration", key: "feature:bardic-inspiration-d6", fight: bard},
		{name: "a paladin's Divine Sense", key: "feature:divine-sense", fight: paladin},
		{name: "a paladin's Divine Sense as the master", key: "feature:divine-sense", fight: paladin, as: takeActionAsMaster},
		{name: "a paladin's Divine Sense twice", key: "feature:divine-sense", fight: paladin, prep: takeActionFirst("Toren", "feature:divine-sense", nil)},
		{name: "a bard's Bardic Inspiration key on a fighter's sheet", key: "feature:bardic-inspiration-d6"},
	}
	out := make([]takeActionScenario, 0, len(cases))
	for _, c := range cases {
		out = append(out, c.scenario())
	}
	return out
}

// takeActionNimble are the actions of a race trait: the goblin's Fuga Ágil.
func takeActionNimble() []takeActionScenario {
	nimble := func(suffix string, master, twice bool) takeActionScenario {
		name := "Nimble Escape " + suffix
		if master {
			name += " as the master"
		}
		if twice {
			name += " twice"
		}
		return takeActionScenario{name: name, run: func(t *testing.T) string {
			t.Helper()
			a, e := nimbleGoblin(t)
			var key string
			for _, f := range a.mustOptions(t, a.caio, e, "Ragna").GetOptions().GetFeatureActions() {
				if k := f.GetAction().GetKey(); strings.HasPrefix(k, "trait:") && strings.HasSuffix(k, suffix) {
					key = k
				}
			}
			if key == "" {
				t.Fatalf("no Fuga Ágil action ending in %q", suffix)
			}
			if twice {
				if _, err := a.takeActionRaw(t, a.caio, e, "Ragna", key, nil); err != nil {
					t.Fatalf("TakeAction(%s) before the scenario error = %v", key, err)
				}
			}
			u := a.caio
			if master {
				u = a.master
			}
			before := a.events(t)
			res, err := a.takeActionRaw(t, u, e, "Ragna", key, nil)
			return a.takeActionReport(t, e, before, res, err)
		}}
	}
	return []takeActionScenario{
		nimble(":disengage", false, false),
		nimble(":hide", false, false),
		nimble(":disengage", true, false),
		nimble(":disengage", false, true),
		nimble(":disengage", true, true),
	}
}

// takeActionFeatureScenarios are the scenarios of the feature actions of the classes and the races.
func takeActionFeatureScenarios() []takeActionScenario {
	var all []takeActionScenario
	all = append(all, takeActionSecondWinds()...)
	all = append(all, takeActionRogueAndMonk()...)
	all = append(all, takeActionBarbarians()...)
	all = append(all, takeActionOtherClasses()...)
	all = append(all, takeActionNimble()...)
	return all
}

// takeActionWorldScenarios are the scenarios that need more of the world than a fight: a window of reaction
// that holds the turn, an opportunity attack that waits, a fall that waits for a monk's reaction.
func takeActionWorldScenarios() []takeActionScenario {
	return []takeActionScenario{
		takeActionOnce("refused while an NPC may react to the player's attack", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			a, e := mageFight(t)
			a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Mago", d20(9))
			before := a.events(t)
			res, err := a.takeActionRaw(t, a.caio, e, "Toren", "standard:dodge", nil)
			return a, e, before, res, err
		}),
		takeActionOnce("the master is not stopped while an NPC may react", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			a, e := mageFight(t)
			a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Mago", d20(9))
			before := a.events(t)
			res, err := a.takeActionRaw(t, a.master, e, "Toren", "standard:dodge", nil)
			return a, e, before, res, err
		}),
		takeActionOnce("refused while an opportunity attack waits", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			c := newCave(t)
			e := c.oppFight(t)
			c.leaveGoblin(t)
			before := c.events(t)
			res, err := c.takeActionRaw(t, c.caio, e, "Toren", "standard:dodge", nil)
			return c.armed, e, before, res, err
		}),
		takeActionOnce("the master is not stopped while an opportunity attack waits", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			c := newCave(t)
			e := c.oppFight(t)
			c.leaveGoblin(t)
			before := c.events(t)
			res, err := c.takeActionRaw(t, c.master, e, "Toren", "standard:dodge", nil)
			return c.armed, e, before, res, err
		}),
		takeActionOnce("Slow Fall on a fall that waits", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			r, e := monkFall(t)
			before := r.events(t)
			res, err := r.takeActionRaw(t, r.caio, e, "Toren", "feature:slow-fall", nil)
			return r.armed, e, before, res, err
		}),
		takeActionOnce("Slow Fall twice on a fall that waits", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			r, e := monkFall(t)
			if _, err := r.takeActionRaw(t, r.caio, e, "Toren", "feature:slow-fall", nil); err != nil {
				t.Fatalf("TakeAction(Slow Fall) error = %v", err)
			}
			before := r.events(t)
			res, err := r.takeActionRaw(t, r.caio, e, "Toren", "feature:slow-fall", nil)
			return r.armed, e, before, res, err
		}),
		takeActionOnce("Slow Fall twice on a fall that waits as the master", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			r, e := monkFall(t)
			if _, err := r.takeActionRaw(t, r.caio, e, "Toren", "feature:slow-fall", nil); err != nil {
				t.Fatalf("TakeAction(Slow Fall) error = %v", err)
			}
			before := r.events(t)
			res, err := r.takeActionRaw(t, r.master, e, "Toren", "feature:slow-fall", nil)
			return r.armed, e, before, res, err
		}),
		takeActionOnce("a Dodge in a combat that ended", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			a, e := takeActionFighters(t)
			takeActionEnd(t, a, e)
			before := a.events(t)
			res, err := a.takeActionRaw(t, a.caio, e, "Toren", "standard:dodge", nil)
			return a, e, before, res, err
		}),
		takeActionOnce("Slow Fall in a combat that ended", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			a, e := takeActionFightOf(takeActionFighter2, takeActionMonk4, false)(t)
			takeActionEnd(t, a, e)
			before := a.events(t)
			res, err := a.takeActionRaw(t, a.bia, e, "Brisa", "feature:slow-fall", nil)
			return a, e, before, res, err
		}),
		takeActionOnce("Slow Fall by a monk out of the fight", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			a, e := takeActionFightOf(takeActionFighter2, takeActionMonk4, false)(t)
			a.execSQL(t, `UPDATE combatants SET defeated = true WHERE label = 'Brisa'`)
			before := a.events(t)
			res, err := a.takeActionRaw(t, a.bia, e, "Brisa", "feature:slow-fall", nil)
			return a, e, before, res, err
		}),
		takeActionOnce("a Dodge while a fall waits for the monk's reaction", func(t *testing.T) (*armed, *playv1.Encounter, int, *playv1.TakeActionResponse, error) {
			t.Helper()
			r, e := monkFall(t)
			before := r.events(t)
			res, err := r.takeActionRaw(t, r.caio, e, "Toren", "standard:dodge", nil)
			return r.armed, e, before, res, err
		}),
	}
}

// takeActionEnd ends the combat, as the master.
func takeActionEnd(t *testing.T, a *armed, e *playv1.Encounter) {
	t.Helper()
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
}
