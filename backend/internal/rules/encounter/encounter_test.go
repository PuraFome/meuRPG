package encounter

import (
	"errors"
	"slices"
	"strconv"
	"testing"
)

// The SRD 5.2.1's first rows and the ones the artboard E10-09 uses (levels 3, 4 and 5).
var table = []Budget{
	{50, 75, 100},
	{100, 150, 200},
	{150, 225, 400},
	{250, 375, 500},
	{500, 750, 1100},
	{600, 1000, 1400},
	{750, 1300, 1700},
	{1000, 1700, 2100},
	{1300, 2000, 2600},
	{1600, 2300, 3100},
	{1900, 2900, 4100},
	{2200, 3700, 4700},
	{2600, 4200, 5400},
	{2900, 4900, 6200},
	{3300, 5400, 7800},
	{3800, 6100, 9800},
	{4500, 7200, 11700},
	{5000, 8700, 14200},
	{5500, 10700, 17200},
	{6400, 13200, 22000},
}

func TestPartyBudget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		levels []int
		want   Budget
	}{
		{"Mirathel: three of level 4 and one of 5", []int{4, 4, 4, 5}, Budget{1250, 1875, 2600}},
		{"with Orin, level 3", []int{4, 4, 4, 5, 3}, Budget{1400, 2100, 3000}},
		{"one character", []int{1}, Budget{50, 75, 100}},
		{"level 20", []int{20}, Budget{6400, 13200, 22000}},
		{"nobody", nil, Budget{}},
	}
	for _, tt := range tests {
		got, err := PartyBudget(table, tt.levels)
		if err != nil || got != tt.want {
			t.Errorf("%s: PartyBudget() = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	for _, bad := range []int{0, -1, 21} {
		if _, err := PartyBudget(table, []int{4, bad}); !errors.Is(err, ErrLevel) {
			t.Errorf("level %d: error = %v, want ErrLevel", bad, err)
		}
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()
	b := Budget{1250, 1875, 2600}
	tests := []struct {
		total    int
		wantBand Band
		wantOver int
	}{
		{0, BandLow, 0},
		{300, BandLow, 0}, // under "Baixa" still reads "Baixa"
		{1250, BandLow, 0},
		{1251, BandModerate, 0},
		{1550, BandModerate, 0}, // the artboard's encounter
		{1875, BandModerate, 0},
		{1876, BandHigh, 0},
		{2600, BandHigh, 0},
		{2601, BandAbove, 1},
		{2900, BandAbove, 300}, // artboard 3
	}
	for _, tt := range tests {
		band, over := Classify(b, tt.total)
		if band != tt.wantBand || over != tt.wantOver {
			t.Errorf("Classify(%d) = %v, %d; want %v, %d", tt.total, band, over, tt.wantBand, tt.wantOver)
		}
	}
	// A party of nobody has no budget: any cost is above it.
	if band, over := Classify(Budget{}, 25); band != BandAbove || over != 25 {
		t.Errorf("Classify(empty party, 25) = %v, %d", band, over)
	}
}

func TestMaxCR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		levels []int
		want   int
	}{
		{[]int{4, 4, 4, 5}, 7 * 8},
		{[]int{4, 4, 4, 5, 3}, 6 * 8}, // an NPC of level 3 lowers the cap
		{[]int{1}, 4 * 8},
		{[]int{20, 20}, 23 * 8},
		{[]int{20}, 23 * 8},
		{nil, 0},
	}
	for _, tt := range tests {
		if got := MaxCR(tt.levels, 240); got != tt.want {
			t.Errorf("MaxCR(%v) = %d, want %d", tt.levels, got, tt.want)
		}
	}
	if got := MaxCR([]int{20}, 16); got != 16 {
		t.Errorf("MaxCR with a ceiling of 2 = %d, want 16", got)
	}
	if LowestLevel([]int{5, 3, 4}) != 3 || LowestLevel(nil) != 0 {
		t.Error("LowestLevel is the smallest, 0 for none")
	}
}

// pool is a small pool in the shape of the SRD's: ratings in eighths, the XP of the table.
var pool = []Creature{
	{"monster:bandit", 1, 25, "humanoid"},
	{"monster:goblin", 2, 50, "humanoid"},
	{"monster:scout", 4, 100, "humanoid"},
	{"monster:thug", 4, 100, "humanoid"},
	{"monster:orc", 4, 100, "humanoid"},
	{"monster:bugbear", 8, 200, "humanoid"},
	{"monster:ogre", 16, 450, "giant"},
	{"monster:bandit-captain", 16, 450, "humanoid"},
	{"monster:wolf", 2, 50, "beast"},
	{"monster:troll", 40, 1800, "giant"},
	{"monster:young-dragon", 80, 5900, "dragon"},
	{"monster:frog", 0, 0, "beast"},
}

func TestTotal(t *testing.T) {
	t.Parallel()
	byKey := map[string]Creature{}
	for _, c := range pool {
		byKey[c.Key] = c
	}
	// Artboard 1: 450 + 2 × 200 + 4 × 100 + 6 × 50 = 1550.
	got, ok := Total(byKey, []Entry{{"monster:ogre", 1}, {"monster:bugbear", 2}, {"monster:orc", 4}, {"monster:goblin", 6}})
	if !ok || got != 1550 {
		t.Errorf("Total() = %d, %v; want 1550 (no multiplier for the number of creatures)", got, ok)
	}
	if _, ok := Total(byKey, []Entry{{"monster:nope", 1}}); ok {
		t.Error("Total() accepted a creature that is not in the pool")
	}
	if got, ok := Total(byKey, nil); !ok || got != 0 {
		t.Errorf("Total(nothing) = %d, %v", got, ok)
	}
}

func costOf(t *testing.T, r Result) int {
	t.Helper()
	byKey := map[string]Creature{}
	for _, c := range pool {
		byKey[c.Key] = c
	}
	xp, ok := Total(byKey, r.Entries)
	if !ok {
		t.Fatalf("an entry is not in the pool: %v", r.Entries)
	}
	return xp
}

func TestGenerateKeepsItsPromises(t *testing.T) {
	t.Parallel()
	byKey := map[string]Creature{}
	for _, c := range pool {
		byKey[c.Key] = c
	}
	for _, budget := range []int{50, 100, 400, 1250, 1875, 2600, 6000, 20000} {
		for _, kind := range []string{"", "humanoid", "beast"} {
			for _, maxCR := range []int{4, 16, 56, 240} {
				for seed := range uint32(200) {
					r, err := Generate(pool, Options{Budget: budget, MaxCR: maxCR, Type: kind, Seed: seed})
					if err != nil {
						if !errors.Is(err, ErrNoCreature) {
							t.Fatalf("Generate() error = %v", err)
						}
						continue
					}
					if got := costOf(t, r); got != r.XP || got > budget {
						t.Fatalf("budget %d, type %q, cap %d, seed %d: costs %d (reported %d), over the budget", budget, kind, maxCR, seed, got, r.XP)
					}
					n := 0
					for i, e := range r.Entries {
						c := byKey[e.Key]
						if c.CR > maxCR || (kind != "" && c.Type != kind) || c.XP <= 0 {
							t.Fatalf("budget %d, type %q, cap %d, seed %d: %s breaks the cap, the type or has no XP", budget, kind, maxCR, seed, e.Key)
						}
						if e.Count < 1 || e.Count > DefaultMaxPerKind {
							t.Fatalf("seed %d: %s has count %d", seed, e.Key, e.Count)
						}
						if i > 0 && c.CR > byKey[r.Entries[0].Key].CR {
							t.Fatalf("seed %d: the group's %s is stronger than the leader", seed, e.Key)
						}
						n += e.Count
					}
					if n > DefaultMaxTotal || len(r.Entries) > 3 {
						t.Fatalf("seed %d: %d creatures in %d kinds, want one leader and a group of one or two kinds", seed, n, len(r.Entries))
					}
				}
			}
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	t.Parallel()
	o := Options{Budget: 1875, MaxCR: 56, Type: "humanoid", Seed: 7731}
	first, err := Generate(pool, o)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		again, _ := Generate(pool, o)
		if !slices.Equal(first.Entries, again.Entries) || first.XP != again.XP {
			t.Fatalf("the same seed gave %v then %v", first, again)
		}
	}
	// The order of the pool never changes the answer.
	rev := slices.Clone(pool)
	slices.Reverse(rev)
	if other, _ := Generate(rev, o); !slices.Equal(first.Entries, other.Entries) {
		t.Errorf("a reversed pool gave %v, want %v", other.Entries, first.Entries)
	}
	// Seeds differ: a hundred of them make more than one encounter.
	seen := map[string]bool{}
	for seed := range uint32(100) {
		o.Seed = seed
		r, _ := Generate(pool, o)
		seen[keyOf(r)] = true
	}
	if len(seen) < 5 {
		t.Errorf("100 seeds gave only %d different encounters", len(seen))
	}
}

func keyOf(r Result) string {
	s := ""
	for _, e := range r.Entries {
		s += e.Key + "×" + strconv.Itoa(e.Count) + ";"
	}
	return s
}

// TestGenerateSpendsAsMuchAsItCan: the generator keeps the try that spent the most,
// so on a pool with a 25 XP creature (the cheapest) the leftover is below 25 for the
// types that have it, and the budget is nearly all spent.
func TestGenerateSpendsAsMuchAsItCan(t *testing.T) {
	t.Parallel()
	for seed := range uint32(300) {
		r, err := Generate(pool, Options{Budget: 1875, MaxCR: 56, Type: "humanoid", Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		if left := 1875 - r.XP; left >= 50 {
			t.Errorf("seed %d: %d XP of 1875 left unspent in %v", seed, left, r.Entries)
		}
		if r.Entries[0].Count != 1 && len(r.Entries) > 1 {
			t.Errorf("seed %d: the leader has %d copies beside a group", seed, r.Entries[0].Count)
		}
	}
}

func TestGenerateRefusesWhenNothingFits(t *testing.T) {
	t.Parallel()
	tests := map[string]Options{
		"no creature of the type":    {Budget: 1000, MaxCR: 240, Type: "undead"},
		"the cap is below the pool":  {Budget: 1000, MaxCR: 0},
		"the budget is below a cost": {Budget: 10, MaxCR: 240},
		"no budget":                  {Budget: 0, MaxCR: 240},
	}
	for name, o := range tests {
		if _, err := Generate(pool, o); !errors.Is(err, ErrNoCreature) {
			t.Errorf("%s: error = %v, want ErrNoCreature", name, err)
		}
	}
	// Only the leader's own kind fits: the group is of that kind.
	r, err := Generate([]Creature{{"monster:orc", 4, 100, "humanoid"}}, Options{Budget: 450, MaxCR: 56, Seed: 1})
	if err != nil || r.XP != 400 || r.Entries[0].Count != 4 {
		t.Errorf("one kind only: %v, %v; want 4 orcs for 400", r, err)
	}
}

func TestGenerateRespectsTheLimits(t *testing.T) {
	t.Parallel()
	r, err := Generate([]Creature{{"monster:bandit", 1, 25, "humanoid"}}, Options{Budget: 10000, MaxCR: 8, Seed: 3, MaxPerKind: 4, MaxTotal: 3})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range r.Entries {
		n += e.Count
	}
	if n > 3 || r.XP != 75 {
		t.Errorf("got %v (%d creatures, %d XP), want 3 bandits for 75", r.Entries, n, r.XP)
	}
}

func FuzzGenerate(f *testing.F) {
	f.Add(1875, 56, "humanoid", uint32(7731), 0, 0)
	f.Add(50, 4, "", uint32(0), 3, 5)
	f.Add(100000, 240, "beast", uint32(4294967295), 1, 1)
	f.Add(-5, -1, "dragon", uint32(9), -1, -1)
	f.Fuzz(func(t *testing.T, budget, maxCR int, kind string, seed uint32, perKind, total int) {
		o := Options{Budget: budget, MaxCR: maxCR, Type: kind, Seed: seed, MaxPerKind: perKind, MaxTotal: total}
		r, err := Generate(pool, o)
		if err != nil {
			if !errors.Is(err, ErrNoCreature) {
				t.Fatalf("Generate() error = %v", err)
			}
			return
		}
		if r.XP > budget || r.XP != costOf(t, r) || len(r.Entries) == 0 {
			t.Fatalf("budget %d: %+v is over it, misreported or empty", budget, r)
		}
		again, _ := Generate(pool, o)
		if !slices.Equal(r.Entries, again.Entries) {
			t.Fatalf("not deterministic: %v then %v", r.Entries, again.Entries)
		}
	})
}

func BenchmarkGenerate(b *testing.B) {
	big := make([]Creature, 0, 334)
	for i := range 334 {
		cr := []int{0, 1, 2, 4, 8, 16, 24, 32, 48, 64, 96, 160}[i%12]
		big = append(big, Creature{Key: "monster:" + string(rune('a'+i%26)) + string(rune('a'+i/26)), CR: cr, XP: 10 + 25*(cr+1)*(1+i%3), Type: []string{"humanoid", "beast", "undead", "giant"}[i%4]})
	}
	o := Options{Budget: 2600, MaxCR: 56, Seed: 1}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		o.Seed = uint32(i)
		if _, err := Generate(big, o); err != nil {
			b.Fatal(err)
		}
	}
}
