package link

import "errors"

// What a class resource call refuses with: the characters module returns them, the
// play module turns them into the typed refusal the app reads (ResourceBlocked).
var (
	// ErrNoHitDiceLeft: the character has no hit die of that size left to spend.
	ErrNoHitDiceLeft = errors.New("no hit die of that size is left")
	// ErrNotEnoughPoints: the pool or the sorcery points do not cover the cost.
	ErrNotEnoughPoints = errors.New("not enough points")
	// ErrSlotLevelTooHigh: Flexible Casting makes no slot of that level.
	ErrSlotLevelTooHigh = errors.New("the slot level is too high to create")
	// ErrNoFreeSlot: no free slot of that level to expend.
	ErrNoFreeSlot = errors.New("no free slot of that level")
	// ErrPointsFull: the sorcerer has the most sorcery points already.
	ErrPointsFull = errors.New("the sorcery points are at the maximum")
	// ErrPointsOver: the slot would take the sorcery points past the maximum.
	ErrPointsOver = errors.New("the slot would pass the maximum of sorcery points")
	// ErrNoResource: the character does not have the feature.
	ErrNoResource = errors.New("the character does not have the resource")
	// ErrBadHitDiceChoice: a choice of hit dice that is not the character's.
	ErrBadHitDiceChoice = errors.New("the hit dice choice is not the character's")
)

// PointsError is a refusal that names the points it needed and the ones there were.
type PointsError struct {
	Err       error
	Needed    int
	Available int
}

func (e *PointsError) Error() string { return e.Err.Error() }

// Unwrap lets errors.Is find the refusal.
func (e *PointsError) Unwrap() error { return e.Err }
