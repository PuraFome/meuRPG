package rules

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/encounter"
)

// The encounter builder (MR-043, RN-29; slice 10.9c) on the real SRD: the table, the
// numbers of the artboard E10-09, and the generator's promises.

func TestEncounterBudgetsAreTheSRD521Table(t *testing.T) {
	t.Parallel()
	b := loadForTest(t).EncounterBudgets()
	if len(b) != 20 {
		t.Fatalf("%d levels, want 20", len(b))
	}
	// Rows checked against p. 201 of the SRD 5.2.1 (scratch-ms/etapa10/srd521/SOURCE.md).
	for level, want := range map[int]encounter.Budget{
		1: {Low: 50, Moderate: 75, High: 100}, 3: {Low: 150, Moderate: 225, High: 400}, 4: {Low: 250, Moderate: 375, High: 500},
		5: {Low: 500, Moderate: 750, High: 1100}, 11: {Low: 1900, Moderate: 2900, High: 4100}, 20: {Low: 6400, Moderate: 13200, High: 22000},
	} {
		if b[level-1] != want {
			t.Errorf("level %d = %v, want %v", level, b[level-1], want)
		}
	}
	b[0].Low = 1
	if loadForTest(t).EncounterBudgets()[0].Low == 1 {
		t.Error("EncounterBudgets shares its slice")
	}
}

func TestLoadEncounterBudgetRefuses(t *testing.T) {
	t.Parallel()
	row := func(l, lo, mo, hi int) string {
		return `{"level":` + itoa(l) + `,"low":` + itoa(lo) + `,"moderate":` + itoa(mo) + `,"high":` + itoa(hi) + `}`
	}
	rows := func(mut func(i int) string) string {
		var parts []string
		for i := 1; i <= 20; i++ {
			parts = append(parts, mut(i))
		}
		return `{"source":"x","levels":[` + strings.Join(parts, ",") + `]}`
	}
	good := func(i int) string { return row(i, 10*i, 20*i, 30*i) }
	load := func(body string) error {
		return (&content{}).loadEncounterBudget(fstest.MapFS{"effects/encounter_budget.json": {Data: []byte(body)}})
	}
	if err := load(rows(good)); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	bad := map[string]string{
		"a level gap": rows(func(i int) string {
			if i == 7 {
				return row(8, 10*i, 20*i, 30*i)
			}
			return good(i)
		}),
		"a row that does not grow": rows(func(i int) string {
			if i == 5 {
				return row(i, 10*i, 10*i, 30*i)
			}
			return good(i)
		}),
		"a level cheaper than the one before": rows(func(i int) string {
			if i == 9 {
				return row(i, 10*i, 20*i, 30*(i-2))
			}
			return good(i)
		}),
		"a zero": rows(func(i int) string {
			if i == 1 {
				return row(1, 0, 20, 30)
			}
			return good(i)
		}),
		"too few levels":   `{"source":"x","levels":[` + row(1, 1, 2, 3) + `]}`,
		"no source":        `{"levels":[]}`,
		"an unknown field": strings.Replace(rows(good), `"level":1,`, `"level":1,"deadly":9,`, 1),
	}
	for name, body := range bad {
		if err := load(body); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestEvaluateEncounterTheArtboardNumbers(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	mirathel := []int{4, 4, 4, 5}
	// Artboard 1: Ogro, 2 Bugbears, 4 Hobgoblins, 6 Goblins = 1.550 XP.
	entries := []encounter.Entry{{Key: "monster:ogre", Count: 1}, {Key: "monster:bugbear", Count: 2}, {Key: "monster:hobgoblin", Count: 4}, {Key: "monster:goblin", Count: 6}}
	ev, err := c.EvaluateEncounter(mirathel, entries)
	if err != nil {
		t.Fatal(err)
	}
	if ev.TotalXP != 1550 || ev.Creatures != 13 || ev.Band != encounter.BandModerate || ev.OverXP != 0 ||
		ev.Budget != (encounter.Budget{Low: 1250, Moderate: 1875, High: 2600}) || ev.MaxCR != "7" || ev.LowestLevel != 4 {
		t.Errorf("Mirathel: %+v", ev)
	}
	if l := ev.Lines[0]; l.Creature.Key != "monster:ogre" || l.Subtotal != 450 || l.AboveCap {
		t.Errorf("first line = %+v", l)
	}

	// Artboard 2: Orin, level 3, lowers the cap to 6 and raises the budget; 1.550 stays moderate.
	ev, _ = c.EvaluateEncounter([]int{4, 4, 4, 5, 3}, entries)
	if ev.Budget != (encounter.Budget{Low: 1400, Moderate: 2100, High: 3000}) || ev.MaxCR != "6" || ev.Band != encounter.BandModerate {
		t.Errorf("with Orin: %+v", ev)
	}

	// Artboard 3: 4 Ogros make 2.900, 300 above the high budget.
	entries[0].Count = 4
	ev, _ = c.EvaluateEncounter(mirathel, entries)
	if ev.TotalXP != 2900 || ev.Band != encounter.BandAbove || ev.OverXP != 300 || ev.Creatures != 16 {
		t.Errorf("above high: %+v", ev)
	}
}

func TestEvaluateEncounterWarnsAboveTheCap(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	// A level 1 party: the cap is ND 4. The Ogro (2) is under it, the Troll (5) is over.
	ev, err := c.EvaluateEncounter([]int{1, 1}, []encounter.Entry{{Key: "monster:ogre", Count: 1}, {Key: "monster:troll", Count: 1}})
	if err != nil || ev.MaxCR != "4" || ev.Lines[0].AboveCap || !ev.Lines[1].AboveCap {
		t.Errorf("cap 4: %+v, %v", ev, err)
	}
	// A party of nobody: every cost is above the (zero) budget, and nothing is "above the cap".
	ev, err = c.EvaluateEncounter(nil, []encounter.Entry{{Key: "monster:ogre", Count: 1}})
	if err != nil || ev.Band != encounter.BandAbove || ev.OverXP != 450 || ev.Lines[0].AboveCap {
		t.Errorf("no party: %+v, %v", ev, err)
	}
}

func TestEvaluateEncounterRefusals(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	if _, err := c.EvaluateEncounter([]int{4}, []encounter.Entry{{Key: "monster:dragonet", Count: 1}}); !errors.Is(err, ErrEncounterCreature) {
		t.Errorf("unknown creature: %v", err)
	}
	if _, err := c.EvaluateEncounter([]int{4}, []encounter.Entry{{Key: "monster:ogre", Count: 0}}); !errors.Is(err, ErrEncounterCount) {
		t.Errorf("zero count: %v", err)
	}
	if _, err := c.EvaluateEncounter([]int{4, 21}, nil); !errors.Is(err, encounter.ErrLevel) {
		t.Errorf("level 21: %v", err)
	}
}

func TestGenerateEncounterKeepsItsPromisesOnTheSRD(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	parties := [][]int{{4, 4, 4, 5}, {4, 4, 4, 5, 3}, {1}, {1, 1, 2}, {8, 8, 9, 9, 10}, {20, 20, 20, 20}, {3, 12}}
	kinds := []string{"", "humanoid", "beast", "undead", "giant", "dragon", "fiend"}
	for _, party := range parties {
		for _, band := range []encounter.Band{encounter.BandLow, encounter.BandModerate, encounter.BandHigh} {
			for _, kind := range kinds {
				for seed := range uint32(40) {
					ev, err := c.GenerateEncounter(party, band, kind, seed, 0)
					if errors.Is(err, ErrEncounterNothingFits) {
						continue
					}
					if err != nil {
						t.Fatalf("party %v, band %v, type %q, seed %d: %v", party, band, kind, seed, err)
					}
					if ev.TotalXP > ev.Budget.For(band) || ev.Band > band {
						t.Fatalf("party %v, band %v, type %q, seed %d: costs %d of %d (band %v)", party, band, kind, seed, ev.TotalXP, ev.Budget.For(band), ev.Band)
					}
					for _, l := range ev.Lines {
						if l.AboveCap || (kind != "" && l.Creature.Type != kind) {
							t.Fatalf("party %v, band %v, type %q, seed %d: %s breaks the cap (%s) or the type", party, band, kind, seed, l.Creature.Key, ev.MaxCR)
						}
					}
				}
			}
		}
	}
}

// TestGenerateEncounterTheArtboardRequest: Moderada, Humanoide, for Mirathel: a leader
// and a group of humanoids within 1.875 XP and ND 7, and the seed gives it back.
func TestGenerateEncounterTheArtboardRequest(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	mirathel := []int{4, 4, 4, 5}
	first, err := c.GenerateEncounter(mirathel, encounter.BandModerate, "humanoid", 7731, 0)
	if err != nil {
		t.Fatal(err)
	}
	if first.TotalXP > 1875 || first.TotalXP < 1700 || len(first.Lines) < 2 || len(first.Lines) > 3 {
		t.Errorf("Moderada: %d XP in %d kinds, want a leader and a group of one or two kinds that spend nearly all of 1.875", first.TotalXP, len(first.Lines))
	}
	again, _ := c.GenerateEncounter(mirathel, encounter.BandModerate, "humanoid", 7731, 0)
	if !slices.EqualFunc(first.Lines, again.Lines, func(a, b EncounterLine) bool { return a.Creature.Key == b.Creature.Key && a.Count == b.Count }) {
		t.Error("the same seed gave another encounter")
	}
	other, _ := c.GenerateEncounter(mirathel, encounter.BandModerate, "humanoid", 7732, 0)
	if slices.EqualFunc(first.Lines, other.Lines, func(a, b EncounterLine) bool { return a.Creature.Key == b.Creature.Key && a.Count == b.Count }) {
		t.Log("seeds 7731 and 7732 made the same encounter (possible, but unlikely)")
	}
}

func TestGenerateEncounterRefusals(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	if _, err := c.GenerateEncounter(nil, encounter.BandLow, "", 1, 0); !errors.Is(err, ErrEncounterNoParty) {
		t.Errorf("no party: %v", err)
	}
	if _, err := c.GenerateEncounter([]int{4}, encounter.BandAbove, "", 1, 0); !errors.Is(err, ErrEncounterBand) {
		t.Errorf("band above: %v", err)
	}
	if _, err := c.GenerateEncounter([]int{4}, encounter.BandLow, "robot", 1, 0); !errors.Is(err, ErrEncounterType) {
		t.Errorf("type robot: %v", err)
	}
	// A level 1 party and giants: no giant costs 50 XP or less.
	if _, err := c.GenerateEncounter([]int{1}, encounter.BandLow, "giant", 1, 0); !errors.Is(err, ErrEncounterNothingFits) {
		t.Errorf("giants for level 1: %v", err)
	}
}

func TestEncounterSwapsAreTheSameXP(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	got, err := c.EncounterSwaps("monster:thug", "humanoid")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, e := range got {
		if e.XP != 100 || e.Type != "humanoid" || e.Key == "monster:thug" {
			t.Errorf("%s (XP %d, %s) is not a swap of the Capanga", e.Key, e.XP, e.Type)
		}
		keys = append(keys, strings.TrimPrefix(e.Key, "monster:"))
	}
	// Artboard 5: Gnoll, Hobgoblin, Orc, Povo-lagarto and Sahuagin (the app leaves out the
	// ones the encounter already has, such as the Batedor), sorted by Portuguese name.
	for _, want := range []string{"gnoll", "hobgoblin", "orc", "lizardfolk", "sahuagin", "scout"} {
		if !slices.Contains(keys, want) {
			t.Errorf("%v lacks %s", keys, want)
		}
	}
	if !slices.IsSortedFunc(got, func(a, b CreatureEntry) int { return strings.Compare(foldPT(a.NamePT), foldPT(b.NamePT)) }) {
		t.Errorf("not sorted by Portuguese name: %v", keys)
	}
	if untyped, _ := c.EncounterSwaps("monster:thug", ""); len(untyped) < len(got) {
		t.Error("without a type there are fewer swaps than with one")
	}
	if _, err := c.EncounterSwaps("monster:nope", ""); !errors.Is(err, ErrEncounterCreature) {
		t.Errorf("unknown creature: %v", err)
	}
	if _, err := c.EncounterSwaps("monster:thug", "robot"); !errors.Is(err, ErrEncounterType) {
		t.Errorf("unknown type: %v", err)
	}
}

func BenchmarkGenerateEncounter(b *testing.B) {
	c, err := LoadSRD()
	if err != nil {
		b.Fatal(err)
	}
	party := []int{4, 4, 4, 5}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, err := c.GenerateEncounter(party, encounter.BandModerate, "", uint32(i), 0); err != nil {
			b.Fatal(err)
		}
	}
}

// TestGenerateEncounterSpendsTheBandOnTheSRD: the generator spends the budget and reaches the band asked
// whenever a shape can (fix round 1). Numbers measured over 100 seeds: every standard party spends 98 % or
// more of the budget of every band.
func TestGenerateEncounterSpendsTheBandOnTheSRD(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	standard := map[string][]int{"Mirathel": {4, 4, 4, 5}, "4 × L1": {1, 1, 1, 1}, "4 × L10": {10, 10, 10, 10}, "4 × L20": {20, 20, 20, 20}, "5 × L3": {3, 3, 3, 3, 3}}
	for name, party := range standard {
		for _, band := range []encounter.Band{encounter.BandLow, encounter.BandModerate, encounter.BandHigh} {
			for seed := uint32(1); seed <= 60; seed++ {
				ev, err := c.GenerateEncounter(party, band, "", seed, 0)
				if err != nil {
					t.Fatalf("%s, band %v, seed %d: %v", name, band, seed, err)
				}
				budget := ev.Budget.For(band)
				if ev.TotalXP > budget || ev.TotalXP*100 < budget*75 || ev.Band != band {
					t.Fatalf("%s, band %v, seed %d: %d XP of %d (band %v), want 75 %% or more, in the band", name, band, seed, ev.TotalXP, budget, ev.Band)
				}
				for _, l := range ev.Lines {
					if l.AboveCap {
						t.Fatalf("%s, band %v, seed %d: %s is above the cap", name, band, seed, l.Creature.Key)
					}
				}
				if again, _ := c.GenerateEncounter(party, band, "", seed, 0); again.TotalXP != ev.TotalXP || len(again.Lines) != len(ev.Lines) {
					t.Fatalf("%s, band %v, seed %d: not deterministic", name, band, seed)
				}
			}
		}
	}
	// Four level 1 characters, Moderada, fiends: the Imp (200) and ten Lemures (10 each) make 300, which is
	// the band; the Imp is more than 6/10 of the budget, so the leader is drawn from every creature too.
	for seed := uint32(1); seed <= 60; seed++ {
		ev, err := c.GenerateEncounter([]int{1, 1, 1, 1}, encounter.BandModerate, "fiend", seed, 0)
		if err != nil || ev.Band != encounter.BandModerate || ev.TotalXP > 300 {
			t.Fatalf("fiends for 4 × L1, seed %d: %v, %v; want Moderada", seed, ev.TotalXP, err)
		}
	}
	// A level 1 and a level 20 together, Alta: the cap is ND 4, so only a big group reaches the band,
	// and most seeds find it.
	reached := 0
	for seed := uint32(1); seed <= 100; seed++ {
		ev, err := c.GenerateEncounter([]int{1, 20}, encounter.BandHigh, "", seed, 0)
		if err != nil || ev.TotalXP > ev.Budget.High {
			t.Fatalf("1 and 20, seed %d: %v, %v", seed, ev.TotalXP, err)
		}
		if ev.Band == encounter.BandHigh {
			reached++
		}
	}
	if reached < 70 {
		t.Errorf("levels 1 and 20 reached Alta in %d of 100 seeds, want most", reached)
	}
	// The party's size bounds the encounter: with room for 6, no more than 6 creatures.
	for seed := uint32(1); seed <= 40; seed++ {
		ev, err := c.GenerateEncounter([]int{5, 5, 5, 5}, encounter.BandHigh, "", seed, 6)
		if err != nil || ev.Creatures > 6 {
			t.Fatalf("room for 6, seed %d: %d creatures, %v", seed, ev.Creatures, err)
		}
	}
}

// TestEncounterBudgetsMatchTheSourceTable compares all 20 rows with the table transcribed from p. 201 of the
// SRD 5.2.1 (scratch-ms/etapa10/srd521/SOURCE.md): low / moderate / high per character.
func TestEncounterBudgetsMatchTheSourceTable(t *testing.T) {
	t.Parallel()
	want := [20][3]int{
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
	got := loadForTest(t).EncounterBudgets()
	for i, w := range want {
		if got[i].Low != w[0] || got[i].Moderate != w[1] || got[i].High != w[2] {
			t.Errorf("level %d = %v, want %v", i+1, got[i], w)
		}
	}
}
