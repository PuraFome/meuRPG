package combat

import (
	"errors"
	"maps"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// ErrNoSlot is returned when the slot asked for is not free.
var ErrNoSlot = errors.New("combat: no free spell slot of that level")

// ErrNoUses is returned when a resource has no uses left.
var ErrNoUses = errors.New("combat: no uses left")

// Usage is what the character has spent since the last rest. The play
// module stores it; the totals come from rules.Derived.
type Usage struct {
	// SlotsUsed[l-1] is how many spell slots of level l were used.
	SlotsUsed [9]int
	// PactSlotsUsed is how many pact magic slots were used.
	PactSlotsUsed int
	// ResourcesUsed is the uses spent of each rules.Resource, by key.
	ResourcesUsed map[string]int
}

// clone copies the map, so a function that returns a changed Usage never
// changes the one it was given.
func (u Usage) clone() Usage {
	u.ResourcesUsed = maps.Clone(u.ResourcesUsed)
	if u.ResourcesUsed == nil {
		u.ResourcesUsed = map[string]int{}
	}
	return u
}

// SlotsFree is how many slots of a spell level (1 to 9) are free.
func SlotsFree(d rules.Derived, u Usage, level int) int {
	if level < 1 || level > 9 || level > len(d.SpellSlots) {
		return 0
	}
	return max(d.SpellSlots[level-1]-u.SlotsUsed[level-1], 0)
}

// PactSlotsFree is how many pact magic slots are free.
func PactSlotsFree(d rules.Derived, u Usage) int {
	if d.PactMagic == nil {
		return 0
	}
	return max(d.PactMagic.Slots-u.PactSlotsUsed, 0)
}

// SpendSlot uses one slot of the level (1 to 9). ErrNoSlot when none is
// free.
func SpendSlot(d rules.Derived, u Usage, level int) (Usage, error) {
	if SlotsFree(d, u, level) == 0 {
		return u, ErrNoSlot
	}
	u = u.clone()
	u.SlotsUsed[level-1]++
	return u, nil
}

// SpendPactSlot uses one pact magic slot.
func SpendPactSlot(d rules.Derived, u Usage) (Usage, error) {
	if PactSlotsFree(d, u) == 0 {
		return u, ErrNoSlot
	}
	u = u.clone()
	u.PactSlotsUsed++
	return u, nil
}

// ResourceLeft is how many uses of a resource remain, and whether the
// character has it at all.
func ResourceLeft(d rules.Derived, u Usage, key string) (left int, ok bool) {
	for _, r := range d.Resources {
		if r.Key == key {
			return max(r.Max-u.ResourcesUsed[key], 0), true
		}
	}
	return 0, false
}

// SpendResource uses one use of a resource. ErrNoUses when none is left or
// the character does not have it.
func SpendResource(d rules.Derived, u Usage, key string) (Usage, error) {
	if left, ok := ResourceLeft(d, u, key); !ok || left == 0 {
		return u, ErrNoUses
	}
	u = u.clone()
	u.ResourcesUsed[key]++
	return u, nil
}
