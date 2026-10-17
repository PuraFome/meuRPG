package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The scenarios of the golden that need a table of their own: a mage that counters, a sorcerer, a bard, a
// summoner, a ranger and the rest of the casts the usual fight does not hold.

// castTable starts a fight for a scenario that plays its own casts.
type castTable func(t *testing.T) (*armed, *playv1.Encounter)

// castSteps is a scenario of a table: steps cast and answer, and return the answer of the last cast.
func castSteps(name string, table castTable, steps func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error)) goldenScenario {
	return castOn(name, func(t *testing.T) (*armed, int, *playv1.CastSpellResponse, error) {
		t.Helper()
		a, e := table(t)
		before := a.markCall(t)
		res, err := steps(t, a, e)
		return a, before, res, err
	})
}

func castersTable(goblins int32) castTable {
	return func(t *testing.T) (*armed, *playv1.Encounter) {
		t.Helper()
		a := newCasters(t)
		enc := a.castersFight(t, goblins)
		return a, enc
	}
}

func theatreTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCasters(t)
	enc := a.theatreCasters(t, 1)
	return a, enc
}

func hiddenTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCasters(t)
	enc := a.hiddenCapitaoFight(t)
	return a, enc
}

func mageTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a, e := mageFight(t)
	a.pensantusTurn(t, e)
	enc := a.get(t, a.master)
	return a, enc
}

func sorcererTable(metamagic ...string) castTable {
	return func(t *testing.T) (*armed, *playv1.Encounter) {
		t.Helper()
		a := newResourceTableWith(t, metamagic)
		enc := a.sorcererFight(t, nil)
		return a, enc
	}
}

func summonersTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newSummoners(t)
	enc := a.summonersFight(t)
	return a, enc
}

func salviaTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a, e := summonersTable(t)
	enc := a.toSalvia(t, e)
	return a, enc
}

func rangersTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newRangers(t)
	e := a.castersFight(t, 1)
	enc := a.passTo(t, e, "Brisa")
	return a, enc
}

// castWindowScenarios are the casts held by a reaction window: a Counterspell of the Mage, or of a player.
func castWindowScenarios() []goldenScenario {
	return []goldenScenario{
		castSteps("a cast held by the mage's counterspell", mageTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.castMissile(t, e)
		}),
		castSteps("a held cast countered by the master", mageTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := a.castMissile(t, e)
			w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.master, e, w.GetId(), useAnswer(3))
			return held, err
		}),
		castSteps("a held cast let through by the master", mageTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := a.castMissile(t, e)
			w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
			return held, err
		}),
		castSteps("a held concentration cast let through", mageTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(6)
			held, err := a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), noCastRoll)
			w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.master, e, w.GetId(), passAnswer)
			return held, err
		}),
		castSteps("a held cast countered by the master, concentration", mageTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), noCastRoll)
			w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.master, e, w.GetId(), useAnswer(3))
			return held, err
		}),
		castSteps("a second cast while the first waits", mageTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			if _, err := a.castMissile(t, e); err != nil {
				return nil, err
			}
			return a.castMissile(t, e)
		}),
		castSteps("a non-attack cantrip beside the mage", mageTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Brisa")
			a.h.roller.queue(8)
			return a.cast(t, a.bia, e, "Brisa", sacredFlame, nil, a.at(t, "Mago"), noCastRoll)
		}),
		castSteps("the master casts for a boss and a player counters", counterFight, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := a.cast(t, a.master, e, "Mago Sombrio", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
			w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(3))
			return held, err
		}),
		castSteps("the master casts for a boss and the player lets it through", counterFight, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := a.cast(t, a.master, e, "Mago Sombrio", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
			w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
			return held, err
		}),
		castSteps("a boss's fireball that a player counters", counterFight, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := a.castCall(t, a.master, e, "Mago Sombrio", fireballSpell, slotOfLevel(3), a.at(t, "Toren"), aimAt(6, 5))
			w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.ana, e, w.GetId(), useAnswer(3))
			return held, err
		}),
		castSteps("a boss's fireball that goes through", counterFight, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(4, 11, 7)
			held, err := a.castCall(t, a.master, e, "Mago Sombrio", fireballSpell, slotOfLevel(3), a.at(t, "Toren"), aimAt(6, 5))
			w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_COUNTERSPELL)
			a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
			return held, err
		}),
		castSteps("a boss's cantrip that is an attack roll", counterFight, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.master, e, "Mago Sombrio", fireBolt, nil, a.at(t, "Toren"), d20Face(12))
		}),
		castSteps("a hidden boss's spell", counterFight, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.castMasterHides(t, e, "Mago Sombrio")
			return a.cast(t, a.master, e, "Mago Sombrio", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Toren", 3)}, noCastRoll)
		}),
	}
}

// castSorcererScenarios are the casts with Metamagic.
func castSorcererScenarios() []goldenScenario {
	allMeta := []string{rules.MetamagicTwinned, rules.MetamagicCareful}
	goblinAndCaptain := func(t *testing.T, a *armed) []*playv1.SpellTarget {
		t.Helper()
		return a.at(t, "Goblin", "Capitão Goblin")
	}
	return []goldenScenario{
		castSteps("a careful fireball", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(1, 1, 1, 1, 1)
			return a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), goblinAndCaptain(t, a),
				&playv1.MetamagicChoice{Key: rules.MetamagicCareful, CarefulIds: []string{a.id(t, "Goblin")}})
		}),
		castSteps("a twinned hold person", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(3, 4)
			return a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"),
				&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Capitão Goblin")}})
		}),
		castSteps("a twinned spell on the same target", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"),
				&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Goblin")}})
		}),
		castSteps("a twinned area spell", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.castWith(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"),
				&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Capitão Goblin")}})
		}),
		castSteps("a careful spell for a creature not hit", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.castStand(t, map[string][2]int32{"Nael": {4, 9}, "Tavo": {9, 3}, "Orla": {10, 3}, "Goblin": {4, 4}, "Capitão Goblin": {9, 9}})
			return a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"),
				&playv1.MetamagicChoice{Key: rules.MetamagicCareful, CarefulIds: []string{a.id(t, "Tavo")}})
		}),
		castSteps("a careful spell for nobody", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicCareful))
		}),
		castSteps("an option the sorcerer does not know", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicSubtle))
		}),
		castSteps("two options that do not join", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin", "Capitão Goblin"),
				&playv1.MetamagicChoice{Key: rules.MetamagicCareful, CarefulIds: []string{a.id(t, "Goblin")}},
				&playv1.MetamagicChoice{Key: rules.MetamagicTwinned, TargetIds: []string{a.id(t, "Capitão Goblin")}})
		}),
		castSteps("a cast with no sorcery points left", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.correct(t, a.pens, func(r *playv1.AdjustCharacterVitalsRequest) {
				r.ResourcesUsed = []*playv1.ResourceUsed{{Key: rules.SorceryPointsKey, Used: 5}}
			})
			return a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"),
				&playv1.MetamagicChoice{Key: rules.MetamagicCareful, CarefulIds: []string{a.id(t, "Goblin")}})
		}),
		castSteps("metamagic for a paladin", sorcererTable(allMeta...), func(t *testing.T, a *armed, _ *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e := a.turnOf(t, "Tavo")
			return a.castWith(t, a.caio, e, "Tavo", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicCareful))
		}),
		castSteps("a distant spell", sorcererTable(rules.MetamagicDistant, rules.MetamagicSubtle), func(t *testing.T, a *armed, _ *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.castStand(t, map[string][2]int32{"Nael": {1, 1}, "Tavo": {5, 3}, "Orla": {6, 3}, "Goblin": {17, 1}, "Capitão Goblin": {18, 1}})
			return a.castWith(t, a.ana, a.get(t, a.master), "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), choice(rules.MetamagicDistant))
		}),
		castSteps("a spell out of range with no distant option", sorcererTable(rules.MetamagicDistant, rules.MetamagicSubtle), func(t *testing.T, a *armed, _ *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.castStand(t, map[string][2]int32{"Nael": {1, 1}, "Tavo": {5, 3}, "Orla": {6, 3}, "Goblin": {17, 1}, "Capitão Goblin": {18, 1}})
			return a.castWith(t, a.ana, a.get(t, a.master), "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"))
		}),
		castSteps("a quickened spell after the action", sorcererTable(rules.MetamagicQuickened, rules.MetamagicSubtle), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			if _, err := a.caioActs(t, a.ana, e, "Nael", "standard:dodge"); err != nil {
				t.Fatalf("TakeAction(dodge) error = %v", err)
			}
			return a.castWith(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"), choice(rules.MetamagicQuickened))
		}),
		castSteps("an action spell after the action with no quickening", sorcererTable(rules.MetamagicQuickened, rules.MetamagicSubtle), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			if _, err := a.caioActs(t, a.ana, e, "Nael", "standard:dodge"); err != nil {
				t.Fatalf("TakeAction(dodge) error = %v", err)
			}
			return a.castWith(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"))
		}),
		castSteps("a quickened spell after a bonus action", sorcererTable(rules.MetamagicQuickened, rules.MetamagicSubtle), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			if _, err := a.caioActs(t, a.ana, e, "Nael", "standard:dodge"); err != nil {
				t.Fatalf("TakeAction(dodge) error = %v", err)
			}
			if _, err := a.castWith(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"), choice(rules.MetamagicQuickened)); err != nil {
				t.Fatalf("CastSpell(quickened) error = %v", err)
			}
			return a.castWith(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), choice(rules.MetamagicQuickened))
		}),
		castSteps("a heightened spell", sorcererTable(rules.MetamagicHeightened, rules.MetamagicSubtle), func(t *testing.T, a *armed, _ *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.castStand(t, map[string][2]int32{"Nael": {4, 9}, "Tavo": {9, 3}, "Orla": {10, 3}, "Goblin": {4, 4}, "Capitão Goblin": {9, 9}})
			a.h.roller.queue(18, 2)
			return a.castWith(t, a.ana, a.get(t, a.master), "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"),
				&playv1.MetamagicChoice{Key: rules.MetamagicHeightened, HeightenedId: a.id(t, "Goblin")})
		}),
		castSteps("a subtle and empowered spell", sorcererTable(rules.MetamagicSubtle, rules.MetamagicEmpowered), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.castWith(t, a.ana, e, "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"), choice(rules.MetamagicSubtle), choice(rules.MetamagicEmpowered))
		}),
		castSteps("a cast retried with the same key", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			key := newKey()
			first, err := a.castKey(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), poolInApp, key)
			if err != nil || first == nil {
				t.Fatalf("CastSpell() = %v, %v", first, err)
			}
			return a.castKey(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), poolInApp, key)
		}),
		castSteps("a key used again for another cast", sorcererTable(allMeta...), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			key := newKey()
			if _, err := a.castKey(t, a.ana, e, "Nael", holdPerson, slotOfLevel(2), a.at(t, "Goblin"), poolInApp, key); err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			return a.castKey(t, a.ana, e, "Nael", burningHands, slotOfLevel(1), a.at(t, "Goblin"), aimToward(1, 0), key)
		}),
	}
}

// castBardTable is the party of the held spell attack: Toren (Caio), Brisa the cleric (Ana, of the race
// given) and Orla the bard (Bia), with the goblin.
func castBardTable(race string, inspired bool) castTable {
	return func(t *testing.T) (*armed, *playv1.Encounter) {
		t.Helper()
		a := newArmedWith(t, func(a *armed) {
			a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
				&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
			a.pens = a.ana.caster(t, a.campaignID, "Brisa", "class:cleric", race, 3,
				&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 16, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
				[]string{cureWounds, healingWord, guidingBolt})
			a.bri = a.bia.hero(t, a.campaignID, "Orla", "class:bard", "race:human", 5,
				&rulesv1.AbilityScores{Strength: 8, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 16}, nil, nil)
		})
		e := a.start(t, plan{
			npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}, {CharacterId: a.capitao.GetId()}}, npcRolls: []int{3, 2},
			players: map[string]int32{"Orla": 20, "Brisa": 14, "Toren": 10}, reveal: []string{"Goblin", "Capitão Goblin"},
			at: map[string][2]int32{"Orla": {3, 3}, "Brisa": {4, 3}, "Toren": {5, 3}, "Goblin": {4, 8}, "Capitão Goblin": {8, 4}},
		})
		if inspired {
			if _, err := a.giveInspiration(t, a.bia, e, "Orla", "Brisa"); err != nil {
				t.Fatalf("GiveBardicInspiration() error = %v", err)
			}
		}
		enc := a.turnOf(t, "Brisa")
		return a, enc
	}
}

// castHeldScenarios are the spell attacks that wait for the player's answer.
func castHeldScenarios() []goldenScenario {
	bolt := func(t *testing.T, a *armed, e *playv1.Encounter, u *user, roll func(*playv1.CastSpellRequest), targets ...string) (*playv1.CastSpellResponse, error) {
		t.Helper()
		return a.cast(t, u, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, targets...), roll)
	}
	return []goldenScenario{
		castSteps("a bardic die holds a typed spell attack", castBardTable("race:human", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return bolt(t, a, e, a.ana, d20Face(5), "Goblin")
		}),
		castSteps("a bardic die holds a spell attack rolled in the app", castBardTable("race:human", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(5)
			return bolt(t, a, e, a.ana, poolInApp, "Goblin")
		}),
		castSteps("a bardic die used on a spell attack", castBardTable("race:human", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := bolt(t, a, e, a.ana, d20Face(5), "Goblin")
			a.h.roller.queue(7)
			if _, aerr := a.answerOutside(t, a.ana, held.GetInspirationOffer().GetHoldId(), true, true, 0); aerr != nil {
				t.Fatalf("AnswerOutsideInspiration() error = %v", aerr)
			}
			return held, err
		}),
		castSteps("a bardic die kept on a spell attack", castBardTable("race:human", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := bolt(t, a, e, a.ana, d20Face(5), "Goblin")
			if _, aerr := a.answerOutside(t, a.ana, held.GetInspirationOffer().GetHoldId(), false, false, 0); aerr != nil {
				t.Fatalf("AnswerOutsideInspiration() error = %v", aerr)
			}
			return held, err
		}),
		castSteps("a bardic die on a spell attack at two targets", castBardTable("race:human", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(5, 6)
			return bolt(t, a, e, a.master, poolInApp, "Goblin", "Capitão Goblin")
		}),
		castSteps("a bardic die and the master casting", castBardTable("race:human", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return bolt(t, a, e, a.master, d20Face(5), "Goblin")
		}),
		castSteps("a halfling's one asks about Lucky", castBardTable("race:halfling", false), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return bolt(t, a, e, a.ana, d20Face(1), "Goblin")
		}),
		castSteps("a halfling's one rolled in the app asks about Lucky", castBardTable("race:halfling", false), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(1)
			return bolt(t, a, e, a.ana, poolInApp, "Goblin")
		}),
		castSteps("a halfling's reroll", castBardTable("race:halfling", false), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := bolt(t, a, e, a.ana, d20Face(1), "Goblin")
			if _, aerr := a.answerOutsideLucky(t, a.ana, held.GetLuckyOffer().GetHoldId(), true, 15); aerr != nil {
				t.Fatalf("AnswerOutsideLucky() error = %v", aerr)
			}
			return held, err
		}),
		castSteps("a halfling keeps the one", castBardTable("race:halfling", false), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			held, err := bolt(t, a, e, a.ana, d20Face(1), "Goblin")
			if _, aerr := a.answerOutsideLucky(t, a.ana, held.GetLuckyOffer().GetHoldId(), false, 0); aerr != nil {
				t.Fatalf("AnswerOutsideLucky() error = %v", aerr)
			}
			return held, err
		}),
		castSteps("a halfling's other face", castBardTable("race:halfling", false), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return bolt(t, a, e, a.ana, d20Face(14), "Goblin")
		}),
		castSteps("a halfling with a bardic die and a one", castBardTable("race:halfling", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return bolt(t, a, e, a.ana, d20Face(1), "Goblin")
		}),
		castSteps("a bardic die asked for a spell attack with disadvantage", castBardTable("race:human", true), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return bolt(t, a, e, a.ana, together(d20Faces(5, 9), rollMode(playv1.RollMode_ROLL_MODE_DISADVANTAGE, "Brisa está cansada")), "Goblin")
		}),
	}
}

// answerOutsideLucky answers the Lucky question of a held roll that is not a weapon attack.
func (a *armed) answerOutsideLucky(t *testing.T, u *user, hold string, reroll bool, typed int32) (*playv1.AnswerOutsideLuckyResponse, error) {
	t.Helper()
	req := &playv1.AnswerOutsideLuckyRequest{CampaignId: a.campaignID, HoldId: hold, Reroll: reroll, IdempotencyKey: newKey()}
	if reroll {
		req.Roll = &playv1.AnswerOutsideLuckyRequest_TypedFace{TypedFace: typed}
	}
	res, err := u.resource.AnswerOutsideLucky(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// castMasterHides hides a combatant as the master.
func (a *armed) castMasterHides(t *testing.T, e *playv1.Encounter, label string) {
	t.Helper()
	if _, err := a.master.combat.SetCombatantHidden(t.Context(), connect.NewRequest(&playv1.SetCombatantHiddenRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Hidden: true,
	})); err != nil {
		t.Fatalf("SetCombatantHidden(%s) error = %v", label, err)
	}
}

// castSummonScenarios are the summoning spells and the concentration that ends them.
func castSummonScenarios() []goldenScenario {
	wolves := func(face int32) func(*playv1.CastSpellRequest) {
		return func(r *playv1.CastSpellRequest) {
			r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: face}
			r.Summon = &playv1.SummonChoice{Option: 1, CreatureKeys: []string{direWolf, direWolf}}
		}
	}
	return []goldenScenario{
		castSteps("conjure animals", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.summonWolves(t, a.bia, e, 11)
		}),
		castSteps("conjure animals as the master", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.summonWolves(t, a.master, e, 13)
		}),
		castSteps("conjure animals twice", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			if _, err := a.summonWolves(t, a.bia, e, 11); err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			e = a.turnOf(t, "Sálvia")
			return a.summonWolves(t, a.bia, e, 12)
		}),
		castSteps("conjure animals with no summon", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Sálvia", conjureAnimals, slotOfLevel(3), nil, d20Face(11))
		}),
		castSteps("conjure animals with a target", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Sálvia", conjureAnimals, slotOfLevel(3), a.at(t, "Goblin"), wolves(11))
		}),
		castSteps("conjure animals with no d20", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Sálvia", conjureAnimals, slotOfLevel(3), nil, func(r *playv1.CastSpellRequest) {
				r.Summon = &playv1.SummonChoice{Option: 1, CreatureKeys: []string{direWolf, direWolf}}
			})
		}),
		castSteps("conjure animals with a creature that does not fit", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Sálvia", conjureAnimals, slotOfLevel(3), nil, func(r *playv1.CastSpellRequest) {
				r.Roll = &playv1.CastSpellRequest_D20Face{D20Face: 11}
				r.Summon = &playv1.SummonChoice{Option: 1, CreatureKeys: []string{"monster:owl"}}
			})
		}),
		castSteps("conjure animals with a pool", salviaTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Sálvia", conjureAnimals, slotOfLevel(3), nil, func(r *playv1.CastSpellRequest) {
				r.Roll = &playv1.CastSpellRequest_PoolSum{PoolSum: 12}
				r.Summon = &playv1.SummonChoice{Option: 1, CreatureKeys: []string{direWolf, direWolf}}
			})
		}),
		castSteps("a familiar cast in a fight", summonersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", findFamiliar, nil, nil, func(r *playv1.CastSpellRequest) {
				r.Summon = &playv1.SummonChoice{CreatureKeys: []string{"monster:owl"}}
			})
		}),
		castSteps("animate dead cast in a fight", summonersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", animateDead, slotOfLevel(3), nil, func(r *playv1.CastSpellRequest) {
				r.Summon = &playv1.SummonChoice{CreatureKeys: []string{"monster:skeleton"}}
			})
		}),
	}
}

// castOtherScenarios are the casts of the other tables: a ranger's mark, the dead raised, the table's
// spells, the hit point spells, a combat without a map and the hidden.
func castOtherScenarios() []goldenScenario {
	return []goldenScenario{
		castSteps("hunter's mark", rangersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", huntersMark, slotOfLevel(1), a.at(t, "Goblin"), noCastRoll)
		}),
		castSteps("hunter's mark moved to another target", rangersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.mustCast(t, a.bia, e, "Brisa", huntersMark, slotOfLevel(1), a.at(t, "Goblin"), noCastRoll)
			e = a.passTo(t, a.mustEndTurnDiscarding(t, e), "Brisa")
			return a.cast(t, a.bia, e, "Brisa", huntersMark, slotOfLevel(1), a.at(t, "Capitão Goblin"), noCastRoll)
		}),
		castSteps("hunter's mark with two targets", rangersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.master, e, "Brisa", huntersMark, slotOfLevel(1), a.at(t, "Goblin", "Capitão Goblin"), noCastRoll)
		}),
		castSteps("a spell cast in a combat without a map", theatreTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(5, 7)
			return a.cast(t, a.ana, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Goblin", "Capitão Goblin"), noCastRoll)
		}),
		castSteps("a spell cast in a combat without a map, too many targets", theatreTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Goblin", "Capitão Goblin"), noCastRoll)
		}),
		castSteps("magic missile in a combat without a map", theatreTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Goblin", 3)}, noCastRoll)
		}),
		castSteps("an area on a combat without a map", theatreTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Goblin"), aimToward(1, 0))
		}),
		castSteps("a spell attack in a combat without a map", theatreTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Brisa")
			return a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), d20Face(16))
		}),
		castSteps("a cone that holds a hidden creature, revealed", hiddenTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Pensantus")
			a.castStand(t, map[string][2]int32{"Toren": {5, 3}, "Capitão Goblin": {7, 4}, "Goblin": {7, 5}})
			a.h.roller.queue(5, 6)
			return a.cast(t, a.master, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Capitão Goblin", "Goblin"), func(r *playv1.CastSpellRequest) { r.RevealHidden = new(true) })
		}),
		castSteps("a cone that holds a hidden creature, kept hidden", hiddenTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Pensantus")
			a.castStand(t, map[string][2]int32{"Toren": {5, 3}, "Capitão Goblin": {7, 4}, "Goblin": {7, 5}})
			a.h.roller.queue(5, 6)
			return a.cast(t, a.master, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Capitão Goblin", "Goblin"), func(r *playv1.CastSpellRequest) { r.RevealHidden = new(false) })
		}),
		castSteps("a cone that holds a hidden creature, left to ask", hiddenTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Pensantus")
			a.castStand(t, map[string][2]int32{"Toren": {5, 3}, "Capitão Goblin": {7, 4}, "Goblin": {7, 5}})
			a.h.roller.queue(5, 6)
			return a.cast(t, a.master, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Capitão Goblin", "Goblin"), noCastRoll)
		}),
		castSteps("a cone of a player that holds a hidden creature", hiddenTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Pensantus")
			a.castStand(t, map[string][2]int32{"Toren": {5, 3}, "Capitão Goblin": {7, 4}, "Goblin": {7, 5}})
			a.h.roller.queue(5, 6)
			return a.cast(t, a.ana, e, "Pensantus", burningHands, slotOfLevel(1), a.at(t, "Goblin"), aimToward(1, 0))
		}),
		castSteps("a spell at a hidden creature as the master", hiddenTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Pensantus")
			a.h.roller.queue(8)
			return a.cast(t, a.master, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Capitão Goblin"), noCastRoll)
		}),
		castSteps("a spell at a hidden creature as the player", hiddenTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Pensantus")
			return a.cast(t, a.ana, e, "Pensantus", holdPerson, slotOfLevel(2), a.at(t, "Capitão Goblin"), noCastRoll)
		}),
		castSteps("a raging caster", castersTable(1), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.execSQL(t, `INSERT INTO combatant_states (encounter_id, combatant_id, kind, started_round, created_at) SELECT encounter_id, id, 'rage', 1, now() FROM combatants WHERE label = 'Pensantus'`)
			return a.cast(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Goblin", 3)}, noCastRoll)
		}),
		castSteps("a raging caster and the master", castersTable(1), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.execSQL(t, `INSERT INTO combatant_states (encounter_id, combatant_id, kind, started_round, created_at) SELECT encounter_id, id, 'rage', 1, now() FROM combatants WHERE label = 'Pensantus'`)
			return a.cast(t, a.master, e, "Pensantus", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, "Goblin", 3)}, noCastRoll)
		}),
		castSteps("a cast in a beast form", newBeastFormCasters,
			func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
				t.Helper()
				e = a.passTo(t, e, "Sálvia")
				return a.summonWolves(t, a.bia, e, 11)
			}),
		castSteps("scorching ray at three creatures", tableTarget4, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(10, 3, 18)
			return a.cast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(2), a.at(t, "Goblin 1", "Goblin 2", "Capitão Goblin"), poolInApp)
		}),
		castSteps("scorching ray at four creatures from a third level slot", tableTarget4, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(10, 10, 10, 10)
			return a.cast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(3), a.at(t, "Goblin 1", "Goblin 2", "Goblin 3", "Goblin 4"), poolInApp)
		}),
		castSteps("scorching ray at too many creatures", tableTarget4, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", scorchingRay, slotOfLevel(3), a.at(t, "Goblin 1", "Goblin 2", "Goblin 3", "Goblin 4", "Capitão Goblin"), poolInApp)
		}),
		castSteps("revivify on the dead", mortuaryTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.deadTarget(t, "Toren"), withDiamonds)
		}),
		castSteps("revivify with no diamonds", mortuaryTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.deadTarget(t, "Toren"), noCastRoll)
		}),
		castSteps("revivify on the living", mortuaryTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.at(t, "Toren"), withDiamonds)
		}),
		castSteps("revivify retried with the same key", mortuaryTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			key := newKey()
			if _, err := a.castKey(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.deadTarget(t, "Toren"), withDiamonds, key); err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			return a.castKey(t, a.ana, e, "Ilaria", revivify, slotOfLevel(3), a.deadTarget(t, "Toren"), withDiamonds, key)
		}),
		castSteps("word power kill on a creature", hpTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.castStand(t, map[string][2]int32{"Toren": {5, 3}, "Capitão Goblin": {7, 4}})
			return a.cast(t, a.ana, e, "Pensantus", wordKill, slotOfLevel(9), a.at(t, "Goblin"), noCastRoll)
		}),
		castSteps("word power kill on a player's character", hpTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.castStand(t, map[string][2]int32{"Toren": {5, 3}, "Capitão Goblin": {7, 4}})
			return a.cast(t, a.ana, e, "Pensantus", wordKill, slotOfLevel(9), a.at(t, "Toren"), noCastRoll)
		}),
		castSteps("color spray", hpTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", colorSpray, slotOfLevel(1), a.at(t, "Capitão Goblin", "Goblin"), together(poolInApp, aimToward(1, 0)))
		}),
		castSteps("a healing pool spell", hpTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			e = a.passTo(t, e, "Brisa")
			a.correct(t, a.toren, hpIs(3))
			return a.cast(t, a.bia, e, "Brisa", healSpell, slotOfLevel(6), a.at(t, "Toren"), noCastRoll)
		}),
		castSteps("false life", tempHPTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", falseLife, slotOfLevel(1), nil, poolInApp)
		}),
		castSteps("false life with no pool", tempHPTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", falseLife, slotOfLevel(1), nil, noCastRoll)
		}),
	}
}

func tableTarget4(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newTableCasters(t)
	enc := a.castersFight(t, 4)
	return a, enc
}

func mortuaryTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a, e := newMortuary(t)
	a.dies(t, e, "Toren", a.toren)
	enc := a.passTo(t, a.get(t, a.master), "Ilaria")
	return a, enc
}

func hpTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newHPCasters(t)
	enc := a.castersFight(t, 1)
	return a, enc
}

func tempHPTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newTempHPCasters(t)
	enc := a.castersFight(t, 1)
	return a, enc
}

func shieldOfFaithTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCastersWithShieldOfFaith(t)
	a.castersFight(t, 1)
	enc := a.turnOf(t, "Brisa")
	return a, enc
}

func mageArmorTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newCastersWithMageArmor(t)
	enc := a.castersFight(t, 1)
	return a, enc
}

func mockersTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newMockers(t)
	enc := a.mockFight(t, false)
	return a, enc
}

func effectsTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newEffectsParty(t)
	a.castersFight(t, 1)
	enc := a.turnOf(t, "Brisa")
	return a, enc
}

func blessersTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newBlessersParty(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Brisa": 20, "Toren": 10}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Brisa": {5, 5}, "Toren": {6, 5}, "Goblin": {9, 5}},
	})
	return a, e
}

func abilityOf(key string) func(*playv1.CastSpellRequest) {
	return func(r *playv1.CastSpellRequest) { r.AbilityKey = key }
}

// castEffectScenarios are the spells that leave an effect on the targets.
func castEffectScenarios() []goldenScenario {
	return []goldenScenario{
		castSteps("shield of faith", shieldOfFaithTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", shieldOfFaithKey, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
		}),
		castSteps("mage armor", mageArmorTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.ana, e, "Pensantus", mageArmorKey, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
		}),
		castSteps("vicious mockery failed", mockersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(2)
			return a.cast(t, a.bia, e, "Orla", viciousMockeryKey, nil, a.at(t, "Goblin"), noCastRoll)
		}),
		castSteps("vicious mockery saved", mockersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.h.roller.queue(19)
			return a.cast(t, a.bia, e, "Orla", viciousMockeryKey, nil, a.at(t, "Goblin"), noCastRoll)
		}),
		castSteps("bless at three creatures", effectsTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", blessKey, slotOfLevel(1), a.at(t, "Toren", "Pensantus", "Brisa"), noCastRoll)
		}),
		castSteps("bless after another concentration", effectsTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			a.mustCast(t, a.bia, e, "Brisa", blessKey, slotOfLevel(1), a.at(t, "Toren"), noCastRoll)
			e = a.turnOf(t, "Brisa")
			return a.cast(t, a.bia, e, "Brisa", blessKey, slotOfLevel(1), a.at(t, "Pensantus", "Brisa"), noCastRoll)
		}),
		castSteps("enhance ability", blessersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", enhanceAbilityKey, slotOfLevel(2), a.at(t, "Toren"), abilityOf("dex"))
		}),
		castSteps("enhance ability with no ability", blessersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", enhanceAbilityKey, slotOfLevel(2), a.at(t, "Toren"), noCastRoll)
		}),
		castSteps("enhance ability with a wrong ability", blessersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", enhanceAbilityKey, slotOfLevel(2), a.at(t, "Toren"), abilityOf("ability:strength"))
		}),
		castSteps("guidance", blessersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", guidanceKey, nil, a.at(t, "Toren"), noCastRoll)
		}),
		castSteps("an ability sent to a spell that does not choose one", blessersTable, func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			return a.cast(t, a.bia, e, "Brisa", guidanceKey, nil, a.at(t, "Toren"), abilityOf("ability:strength"))
		}),
	}
}
