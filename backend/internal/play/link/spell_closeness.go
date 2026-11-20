package link

// How close the targets of a spell must stand and how far from the caster they may (rules.SpellTarget): the
// pure part of the target check of a player's cast.

// HoldsToRange says the target at index i of a cast is held to the spell's range from the caster.
// A spell whose others hang off the first holds only the first.
func (sp Spell) HoldsToRange(i int) bool { return !sp.WithinFirst || i == 0 }

// ListedSlackFt is how far past its range a creature may be and still be a legal pick, for the list of who the caster may pick:
// the creatures of a spell that stretches past its range, or, for a spell whose others hang off the first, any creature
// within WithinFt of the first. The cast itself is checked exactly.
func (sp Spell) ListedSlackFt() int {
	if sp.WithinFirst {
		return sp.WithinFt
	}
	return sp.RangeExtraFt
}

// ClosePairs are the pairs of target indexes of a cast of n targets whose distance WithinFt limits: every pair, or
// the first with each other one.
func (sp Spell) ClosePairs(n int) [][2]int {
	if sp.WithinFt == 0 {
		return nil
	}
	var out [][2]int
	for i := range n {
		for j := i + 1; j < n; j++ {
			out = append(out, [2]int{i, j})
		}
		if sp.WithinFirst {
			break
		}
	}
	return out
}
