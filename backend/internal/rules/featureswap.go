package rules

// FeatureSwapLevels says at which class levels the picks of a table feature's option pool may be swapped, one pick
// for another option of the pool, as part of the level-up. The zero value is never. Levels and EveryLevel are not set
// together.
type FeatureSwapLevels struct {
	// Levels are the class levels (1 to MaxLevel, each once) at which a swap is allowed.
	Levels []int
	// EveryLevel allows a swap at every class level from the feature's own level on.
	EveryLevel bool
}

// IsZero says the feature never allows a swap.
func (s FeatureSwapLevels) IsZero() bool { return len(s.Levels) == 0 && !s.EveryLevel }

// AllowsFeatureSwaps says the offer lets every swap be made: at most one for each feature group, the given-up options
// all different and the taken ones all different, each given up from the group's picks and taken from its pool.
func (o LevelUpOffer) AllowsFeatureSwaps(swaps []PickSwap) bool {
	return false
}
