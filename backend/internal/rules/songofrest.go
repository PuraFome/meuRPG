package rules

import "slices"

// Song of Rest (SRD 5.1, Bard, level 2): during a short rest, if the bard, or
// friendly creatures who can hear the bard's performance, regain hit points at
// the end of the rest by spending Hit Dice, each of them regains an extra die of
// hit points: a d6, a d8 from bard level 9, a d10 from 13 and a d12 from 17.
//
// The app has no performance to hear, so the table's rule is this (the SRD
// leaves "can hear" to the master): every character of the party that rests
// together hears the song while a bard who has the feature is in the party and
// has at least 1 hit point; the bard hears itself. The extra die is rolled once
// per character per short rest, with the first hit die it spends in that rest
// (the app has no "end of the rest" moment, and the total is the same).

// songOfRestDice are the sides the feature's tiers have, from level 2 to 17.
var songOfRestDice = []int{6, 8, 10, 12}

// SongOfRestFor is the die the party hears: the largest Song of Rest die
// (Derived.SongOfRestSides) among the singers, the bards of the party with at
// least 1 hit point. It is 0 when nobody sings.
func SongOfRestFor(singers []int) int {
	best := 0
	for _, s := range singers {
		if slices.Contains(songOfRestDice, s) {
			best = max(best, s)
		}
	}
	return best
}
