package grid

// LengthDFt is the straight distance between the centers of two squares in
// tenths of a foot, exact to the nearest tenth: a square straight is 50 and a
// diagonal one 71. It is what a move on open floor costs (D1), and what the
// combat log says a combatant walked.
func LengthDFt(from, to Square) int { return lengthDFt(from, to) }
