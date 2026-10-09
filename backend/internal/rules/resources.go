package rules

import (
	"encoding/json"
	"errors"
	"fmt"
)

// The class resources that have a flow of their own: Lay on Hands, Flexible Casting
// and Bardic Inspiration. Metamagic is in metamagic.go.

// LayOnHandsKey is the resource of the paladin's pool of healing.
const LayOnHandsKey = "lay_on_hands"

// LayOnHandsCureCost is what curing one disease or neutralizing one poison takes
// from the pool (SRD 5.1, Paladin, Lay on Hands: "expend 5 hit points from your
// pool of healing").
const LayOnHandsCureCost = 5

// ErrLayOnHands is the base of the Lay on Hands refusals.
var ErrLayOnHands = errors.New("lay on hands")

// LayOnHandsAmount checks the points a touch spends: 1 up to what is left in the
// pool ("up to the maximum amount remaining in your pool").
func LayOnHandsAmount(amount, poolLeft int) error {
	if amount < 1 || amount > poolLeft {
		return fmt.Errorf("%w: the amount must be 1 to %d", ErrLayOnHands, poolLeft)
	}
	return nil
}

// LayOnHandsNoEffect says whether the touch does nothing to a creature of the type:
// "This feature has no effect on undead and constructs" (SRD 5.1, Paladin).
func LayOnHandsNoEffect(creatureType string) bool {
	return creatureType == "undead" || creatureType == "construct"
}

// SorceryPointsKey is the resource of the sorcerer's points.
const SorceryPointsKey = "sorcery_points"

// ErrFlexibleCasting is the base of the Flexible Casting refusals.
var ErrFlexibleCasting = errors.New("flexible casting")

// SlotCreationCosts is the SRD's "Creating Spell Slots" table: the sorcery points
// that make a spell slot of each level 1 to 5 (2, 3, 5, 6 and 7). It is read from the
// sorcerer's class table (the snapshot's creating_spell_slots column), where the SRD
// has it.
func (c *Content) SlotCreationCosts() map[int]int {
	out := map[int]int{}
	for _, row := range c.c.classLevels["class:sorcerer"] {
		if row == nil || len(row.ClassSpecific) == 0 {
			continue
		}
		var cs struct {
			Slots []struct {
				Level int `json:"spell_slot_level"`
				Cost  int `json:"sorcery_point_cost"`
			} `json:"creating_spell_slots"`
		}
		if json.Unmarshal(row.ClassSpecific, &cs) != nil {
			continue
		}
		for _, s := range cs.Slots {
			out[s.Level] = s.Cost
		}
	}
	return out
}

// SlotCreationCost is the sorcery points a created slot of the level costs, and
// whether Flexible Casting can create one that high ("no higher in level than 5th").
func (c *Content) SlotCreationCost(level int) (int, bool) {
	cost, ok := c.SlotCreationCosts()[level]
	return cost, ok
}

// CreateSlotCheck checks the creation of a spell slot of the level with the points
// the sorcerer has now. It returns the cost.
func (c *Content) CreateSlotCheck(level, points int) (int, error) {
	cost, ok := c.SlotCreationCost(level)
	if !ok {
		return 0, fmt.Errorf("%w: a spell slot of level %d cannot be created", ErrFlexibleCasting, level)
	}
	if points < cost {
		return 0, fmt.Errorf("%w: a slot of level %d costs %d points and there are %d", ErrFlexibleCasting, level, cost, points)
	}
	return cost, nil
}

// ConvertSlotCheck checks the conversion of one free spell slot of the level into
// sorcery points. The slot gives its level in points (SRD 5.1, Flexible Casting:
// "a number of sorcery points equal to the slot's level"), and the sorcerer "can
// never have more sorcery points than shown on the table for your level" (Font of
// Magic): a conversion that would pass the maximum is refused, with the reason
// ErrSorceryPointsFull when the sorcerer is at the maximum already.
func ConvertSlotCheck(level, points, maxPoints int) (gain int, err error) {
	if level < 1 || level > maxSpellLevel {
		return 0, fmt.Errorf("%w: no spell slot of level %d", ErrFlexibleCasting, level)
	}
	if points >= maxPoints {
		return 0, ErrSorceryPointsFull
	}
	if points+level > maxPoints {
		return 0, ErrSorceryPointsOver
	}
	return level, nil
}

// The two ways a conversion is refused for the maximum of points.
var (
	ErrSorceryPointsFull = fmt.Errorf("%w: already at the maximum of sorcery points", ErrFlexibleCasting)
	ErrSorceryPointsOver = fmt.Errorf("%w: the slot would pass the maximum of sorcery points", ErrFlexibleCasting)
)

// maxSpellLevel is the highest spell level.
const maxSpellLevel = 9

// BardicInspirationKey is the resource of the bard's uses.
const BardicInspirationKey = "bardic_inspiration"

// BardicInspirationRangeFt is how far the bard's words reach: "within 60 feet of
// you who can hear you" (SRD 5.1, Bard, Bardic Inspiration).
const BardicInspirationRangeFt = 60

// BardicInspirationRounds is how long the die lasts, in combat rounds: 10 minutes
// of 6 seconds a round.
const BardicInspirationRounds = 100

// BardicInspirationDie is the size of the die the bard gives at its level: the
// class table's column (d6, d8 at level 5, d10 at 10 and d12 at 15).
func (c *Content) BardicInspirationDie(bardLevel int) int {
	rows := c.c.classLevels["class:bard"]
	if bardLevel < 1 || bardLevel > len(rows) || rows[bardLevel-1] == nil {
		return 0
	}
	var cs struct {
		Die int `json:"bardic_inspiration_die"`
	}
	if json.Unmarshal(rows[bardLevel-1].ClassSpecific, &cs) != nil {
		return 0
	}
	return cs.Die
}

// ErrBardicInspiration is the base of the Bardic Inspiration refusals.
var ErrBardicInspiration = errors.New("bardic inspiration")

// BardicInspirationTarget is what the bard knows of the creature it chooses: it is
// not the bard, it is within 60 feet, it can hear, and it has no die yet (SRD 5.1:
// "one creature other than yourself within 60 feet of you who can hear you"; "a
// creature can have only one Bardic Inspiration die at a time"). The SRD asks no
// line of sight.
type BardicInspirationTarget struct {
	IsBard   bool
	Distance int  // in feet; meaningful when OnMap
	OnMap    bool // false: theatre of the mind, where the master judges the range
	CanHear  bool
	HasDie   bool
}

// BardicInspirationRefusal is the reason the target cannot be given a die, "" when it
// can. The reasons are the ones the choice list writes next to the creature.
func BardicInspirationRefusal(t BardicInspirationTarget) string {
	switch {
	case t.IsBard:
		return "É você"
	case t.OnMap && t.Distance > BardicInspirationRangeFt:
		return "Além de 18 m"
	case !t.CanHear:
		return "Não ouve você"
	case t.HasDie:
		return "Já tem um dado"
	}
	return ""
}

// BardicInspirationExpiry is the round the die given in round has run out: it
// lasts 10 minutes.
func BardicInspirationExpiry(round int) int { return round + BardicInspirationRounds }
