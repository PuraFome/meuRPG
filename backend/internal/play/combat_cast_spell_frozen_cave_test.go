package play

import (
	"testing"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The scenarios of the golden that play in the cave: the areas the map places, the creatures an evocation
// wizard spares and the hidden creatures an area hits.

// caveCast is a scenario of the cave: build starts the fight, cast casts.
func caveCast(name string, build func(t *testing.T) *cave, cast func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error)) castScenario {
	return castOn(name, func(t *testing.T) (*armed, int, *playv1.CastSpellResponse, error) {
		c := build(t)
		before := c.castEvents(t)
		res, err := cast(t, c)
		return c.armed, before, res, err
	})
}

func coneCave(t *testing.T) *cave {
	c := newSculptCave(t)
	c.coneFight(t)
	return c
}

func areaCave(t *testing.T) *cave {
	c := newAreaCave(t)
	c.areaFight(t)
	return c
}

func askCave(t *testing.T) *cave {
	c := newAreaCave(t)
	c.askFight(t)
	return c
}

func revealCave(t *testing.T) *cave {
	c := newAreaCave(t)
	c.areaFight(t)
	c.setRules(t, func(r *campaignsv1.TableRules) {
		r.HiddenAreaHits = campaignsv1.HiddenAreaHitRule_HIDDEN_AREA_HIT_RULE_REVEAL
	})
	c.hide(t, "Goblin 2")
	c.hide(t, "Goblin 3")
	return c
}

func castCaveScenarios() []castScenario {
	return []castScenario{
		caveCast("a sculpted cone", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10)
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(c.id(t, "Toren")))
		}),
		caveCast("a cone sculpted for three from a second level slot", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10)
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(2), nil, toward(1, 0), sculpt(c.id(t, "Toren"), c.id(t, "Goblin 1"), c.id(t, "Goblin 2")))
		}),
		caveCast("a cone sculpted for three from a first level slot", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(c.id(t, "Toren"), c.id(t, "Goblin 1"), c.id(t, "Goblin 2")))
		}),
		caveCast("a cone sculpted for a creature outside it", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(c.id(t, "Goblin 3")))
		}),
		caveCast("a cone sculpted for the caster", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(c.id(t, "Pensantus")))
		}),
		caveCast("a cone sculpted for a creature twice", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(c.id(t, "Toren"), c.id(t, "Toren")))
		}),
		caveCast("a sleep sculpted", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", sleepSpell, slotOfLevel(1), at(17, 7), nil, sculpt(c.id(t, "Toren")), poolInApp)
		}),
		caveCast("a sculpt id that is not an id", coneCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt("nope"))
		}),
		caveCast("a sculpt by a wizard without the feature", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0), sculpt(c.id(t, "Toren")))
		}),
		caveCast("a fireball in the cave", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10, 10)
			return c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
		}),
		caveCast("a fireball in the cave as the master", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10, 10)
			return c.castArea(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
		}),
		caveCast("a fireball out of range", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(1, 1), nil)
		}),
		caveCast("a fireball with a direction", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), nil, toward(1, 0))
		}),
		caveCast("a cone with an origin", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), at(14, 7), nil)
		}),
		caveCast("a lightning bolt through the cave", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 3, 18)
			return c.castArea(t, c.ana, "Pensantus", lightningBolt, slotOfLevel(3), nil, toward(1, 0))
		}),
		caveCast("a caster off the map", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.execSQL(t, `UPDATE combatants SET grid_col = NULL, grid_row = NULL WHERE label = 'Pensantus'`)
			return c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
		}),
		caveCast("an area that hits hidden creatures when the table asks", askCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10, 10)
			return c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
		}),
		caveCast("an area that hits hidden creatures when the table asks, as the master", askCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10, 10)
			return c.castArea(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
		}),
		caveCast("an area that hits no hidden creature when the table asks", askCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10)
			return c.castArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(0, 1))
		}),
		caveCast("an area that hits no hidden creature when the table asks, as the master", askCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10)
			return c.castArea(t, c.master, "Pensantus", burningHands, slotOfLevel(1), nil, toward(0, 1))
		}),
		caveCast("an area that reveals the hidden creatures it hits", revealCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10, 10)
			return c.castArea(t, c.ana, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil)
		}),
		caveCast("an area that the master keeps hidden", revealCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10, 10)
			return c.castArea(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil, func(r *playv1.CastSpellRequest) { r.RevealHidden = new(false) })
		}),
		caveCast("an area that the master reveals", revealCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10, 10)
			return c.castArea(t, c.master, "Pensantus", fireball, slotOfLevel(3), at(20, 7), nil, func(r *playv1.CastSpellRequest) { r.RevealHidden = new(true) })
		}),
		caveCast("a spell centered on the caster", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			c.h.roller.queue(10, 10, 10)
			return c.castArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, nil)
		}),
		caveCast("a spell centered on the caster with an origin", areaCave, func(t *testing.T, c *cave) (*playv1.CastSpellResponse, error) {
			return c.castArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), at(14, 7), nil)
		}),
	}
}
