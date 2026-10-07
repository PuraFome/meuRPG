package rules

import "testing"

// Every creature gives the XP of its challenge rating (RN-09, encounter budgets): the table is the SRD's,
// and a value copied from upstream that disagrees with it understates the encounter.
func TestCreatureXPMatchesChallengeRatingTable(t *testing.T) {
	c := loadForTest(t)
	table := map[string]int{"0": 10, "1/8": 25, "1/4": 50, "1/2": 100, "1": 200, "2": 450, "3": 700, "4": 1100, "5": 1800, "6": 2300, "7": 2900,
		"8": 3900, "9": 5000, "10": 5900, "11": 7200, "12": 8400, "13": 10000, "14": 11500, "15": 13000, "16": 15000, "17": 18000, "18": 20000,
		"19": 22000, "20": 25000, "21": 33000, "22": 41000, "23": 50000, "24": 62000, "25": 75000, "26": 90000, "27": 105000, "28": 120000,
		"29": 135000, "30": 155000}
	list, _ := c.ListCreatures(CreatureFilter{})
	for _, e := range list {
		want, ok := table[e.ChallengeRating]
		if !ok {
			t.Errorf("%s: unknown challenge rating %q", e.Key, e.ChallengeRating)
			continue
		}
		// A few CR 0 creatures (frog, sea horse) legitimately give 0.
		if e.ChallengeRating == "0" && e.XP == 0 {
			continue
		}
		if e.XP != want {
			t.Errorf("%s: challenge rating %s gives %d XP, the table says %d", e.Key, e.ChallengeRating, e.XP, want)
		}
	}
}
