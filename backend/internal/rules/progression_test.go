package rules

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNextLevelXP(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range []struct {
		level, want int
		ok          bool
	}{
		{1, 300, true},
		{2, 900, true},
		{3, 2700, true}, // ADR-0008's example
		{4, 6500, true},
		{19, 355000, true},
		{20, 0, false},
		{0, 0, false},
		{21, 0, false},
		{-1, 0, false},
	} {
		got, ok := c.NextLevelXP(tc.level)
		if got != tc.want || ok != tc.ok {
			t.Errorf("NextLevelXP(%d) = %d, %v; want %d, %v", tc.level, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLevelForXP(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	thresholds := []int{0, 300, 900, 2700, 6500, 14000, 23000, 34000, 48000, 64000, 85000, 100000, 120000, 140000, 165000, 195000, 225000, 265000, 305000, 355000}
	for i, xp := range thresholds {
		level := i + 1
		if got := c.LevelForXP(xp); got != level {
			t.Errorf("LevelForXP(%d) = %d, want %d", xp, got, level)
		}
		if i > 0 {
			if got := c.LevelForXP(xp - 1); got != level-1 {
				t.Errorf("LevelForXP(%d) = %d, want %d", xp-1, got, level-1)
			}
		}
		if got := c.LevelForXP(xp + 1); got != level {
			t.Errorf("LevelForXP(%d) = %d, want %d", xp+1, got, level)
		}
	}
	if got := c.LevelForXP(-5); got != 1 {
		t.Errorf("LevelForXP(-5) = %d, want 1", got)
	}
	if got := c.LevelForXP(1_000_000); got != 20 {
		t.Errorf("LevelForXP(1000000) = %d, want 20", got)
	}
}

func TestXPForChallenge(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := map[string]int{
		"0": 10, "1/8": 25, "1/4": 50, "1/2": 100, "1": 200, "2": 450, "3": 700, "4": 1100,
		"5": 1800, "6": 2300, "7": 2900, "8": 3900, "9": 5000, "10": 5900, "11": 7200,
		"12": 8400, "13": 10000, "14": 11500, "15": 13000, "16": 15000, "17": 18000,
		"18": 20000, "19": 22000, "20": 25000, "21": 33000, "22": 41000, "23": 50000,
		"24": 62000, "25": 75000, "26": 90000, "27": 105000, "28": 120000, "29": 135000,
		"30": 155000,
	}
	if len(want) != 34 {
		t.Fatalf("test table has %d ratings", len(want))
	}
	for r, xp := range want {
		if got, ok := c.XPForChallenge(r); !ok || got != xp {
			t.Errorf("XPForChallenge(%q) = %d, %v; want %d", r, got, ok, xp)
		}
	}
	for _, bad := range []string{"", "31", "-1", "1/3", "01", "ND 1", " 1"} {
		if _, ok := c.XPForChallenge(bad); ok {
			t.Errorf("XPForChallenge(%q) should be unknown", bad)
		}
	}
	list := c.ChallengeRatings()
	if len(list) != 34 || list[0].Rating != "0" || list[33].Rating != "30" {
		t.Errorf("ChallengeRatings() = %v", list)
	}
	list[0].XP = 1 // a copy: must not change the content
	if xp, _ := c.XPForChallenge("0"); xp != 10 {
		t.Error("ChallengeRatings leaked the content's slice")
	}
	if cat := c.Catalog(); len(cat.ChallengeRatings) != 34 {
		t.Errorf("Catalog has %d ratings", len(cat.ChallengeRatings))
	}
}

func TestSplitXP(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ total, n, want int }{
		{100, 4, 25}, {100, 3, 33}, {50, 0, 0}, {50, -2, 0}, {0, 3, 0}, {-10, 2, 0}, {1, 2, 0}, {7, 1, 7},
	} {
		if got := SplitXP(tc.total, tc.n); got != tc.want {
			t.Errorf("SplitXP(%d, %d) = %d, want %d", tc.total, tc.n, got, tc.want)
		}
	}
}

func TestDerivedNextLevelXP(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	if d := Derive(pensantus(), c); d.TotalLevel != 3 || d.NextLevelXP != 2700 {
		t.Errorf("Pensantus: level %d, next %d; want 3, 2700", d.TotalLevel, d.NextLevelXP)
	}
	b := pensantus()
	b.Classes[0].Level = 20
	if d := Derive(b, c); d.NextLevelXP != 0 {
		t.Errorf("level 20: next = %d, want 0", d.NextLevelXP)
	}
}

func TestSceneOptions(t *testing.T) {
	t.Parallel()
	d := Derive(pensantus(), loadForTest(t))
	got, err := SceneOptions(d, []SceneAction{
		{Key: "skill:investigation", Name: "Procurar pistas", DC: 15},
		{Key: "skill:perception"},
		{Key: "skill:insight"},
		{Key: "skill:arcana"},
		{Key: "ability:int"},
		{Key: "ability:str"},
		{Key: "save:wis"},
	})
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		name    string
		bonus   int
		passive int
		has     bool
	}
	want := []row{
		{"Investigação", 6, d.PassiveInvestigation, true},
		{"Percepção", got[1].Bonus, d.PassivePerception, true},
		{"Intuição", got[2].Bonus, d.PassiveInsight, true},
		{"Arcanismo", got[3].Bonus, 0, false},
		{"Teste de Inteligência", mod(d, INT), 0, false},
		{"Teste de Força", mod(d, STR), 0, false},
		{"Salvaguarda de Sabedoria", save(d, WIS), 0, false},
	}
	for i, w := range want {
		g := got[i]
		if g.NamePT != w.name || g.Bonus != w.bonus || g.Passive != w.passive || g.HasPassive != w.has {
			t.Errorf("option %d = %+v, want %+v", i, g, w)
		}
	}
	if got[0].Passive != 10+got[0].Bonus {
		t.Errorf("passive Investigation = %d, want 10 + %d", got[0].Passive, got[0].Bonus)
	}
	if got[0].Action.Name != "Procurar pistas" || got[0].Action.DC != 15 || got[0].Kind != SceneSkill {
		t.Errorf("the action did not travel with the option: %+v", got[0])
	}
	// A wizard is proficient in Wisdom saves.
	if got[6].Bonus != mod(d, WIS)+d.ProficiencyBonus {
		t.Errorf("Wisdom save = %d, want modifier + proficiency", got[6].Bonus)
	}
}

func mod(d Derived, a Ability) int {
	for _, s := range d.Abilities {
		if s.Ability == a {
			return s.Modifier
		}
	}
	return -99
}

func save(d Derived, a Ability) int {
	for _, s := range d.SavingThrows {
		if s.Ability == a {
			return s.Bonus
		}
	}
	return -99
}

func TestSceneOptionsRefuseWhatIsNotAScene(t *testing.T) {
	t.Parallel()
	d := Derive(pensantus(), loadForTest(t))
	for _, key := range []string{
		"", "skill:flying", "skill:", "ability:luck", "save:xyz", "attack:longsword",
		"spell:fire-bolt", "standard:attack", "feature:action-surge", "investigation", "skill:Investigation",
	} {
		_, err := SceneOptions(d, []SceneAction{{Key: "skill:arcana"}, {Key: key}})
		var se *SceneError
		if !errors.As(err, &se) || se.Index != 1 || se.Key != key {
			t.Errorf("key %q: err = %v, want a *SceneError at index 1", key, err)
		}
	}
	if got, err := SceneOptions(d, nil); err != nil || len(got) != 0 {
		t.Errorf("no actions: %v, %v", got, err)
	}
}

// TestLoadAdvancementRefusesBrokenTables proves the load check: a table
// that is not the SRD's shape stops the content from loading.
func TestLoadAdvancementRefusesBrokenTables(t *testing.T) {
	t.Parallel()
	const levels = `[0, 300, 900, 2700, 6500, 14000, 23000, 34000, 48000, 64000, 85000, 100000, 120000, 140000, 165000, 195000, 225000, 265000, 305000, 355000]`
	var ok strings.Builder
	for i, r := range ratingOrder {
		if i > 0 {
			ok.WriteString(",")
		}
		ok.WriteString(`{"rating":"` + r + `","xp":` + strconv.Itoa(10+i*5) + `}`)
	}
	ratings := ok.String()
	build := func(levels, ratings string) string {
		return `{"level_xp":` + levels + `,"challenge_ratings":[` + ratings + `]}`
	}
	if err := (&content{}).loadAdvancement(fstest.MapFS{"effects/advancement.json": {Data: []byte(build(levels, ratings))}}); err != nil {
		t.Fatalf("a good table was refused: %v", err)
	}
	for name, tc := range map[string]struct{ levels, ratings, want string }{
		"too few levels":        {`[0, 300]`, ratings, "20 entries"},
		"level 1 not zero":      {strings.Replace(levels, "[0,", "[5,", 1), ratings, "level 1"},
		"levels not increasing": {strings.Replace(levels, "900", "300", 1), ratings, "strictly increase"},
		"too few ratings":       {levels, `{"rating":"0","xp":10}`, "34 entries"},
		"ratings out of order":  {levels, strings.Replace(ratings, `"1/8"`, `"1/3"`, 1), `must be "1/8"`},
		"xp not increasing":     {levels, strings.Replace(ratings, `"xp":15`, `"xp":5`, 1), "increasing"},
	} {
		err := (&content{}).loadAdvancement(fstest.MapFS{"effects/advancement.json": {Data: []byte(build(tc.levels, tc.ratings))}})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, tc.want)
		}
	}
}

func TestSceneCheckName(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for key, want := range map[string]string{
		"skill:investigation": "Investigação", "ability:str": "Teste de Força", "save:wis": "Salvaguarda de Sabedoria",
	} {
		if got, ok := c.SceneCheckName(key); !ok || got != want {
			t.Errorf("SceneCheckName(%q) = %q, %v; want %q", key, got, ok, want)
		}
	}
	// The same names SceneOptions gives, and the same refusals.
	d := Derive(pensantus(), c)
	for _, key := range []string{"skill:arcana", "ability:dex", "save:con"} {
		opts, err := SceneOptions(d, []SceneAction{{Key: key}})
		got, ok := c.SceneCheckName(key)
		if err != nil || !ok || got != opts[0].NamePT {
			t.Errorf("SceneCheckName(%q) = %q, %v; SceneOptions says %q", key, got, ok, opts[0].NamePT)
		}
	}
	for _, key := range []string{"", "skill:flying", "skill:", "ability:luck", "save:xyz", "attack:longsword", "spell:fire-bolt", "feature:action-surge", "investigation", "skill:Investigation"} {
		if got, ok := c.SceneCheckName(key); ok {
			t.Errorf("SceneCheckName(%q) = %q, true; want false", key, got)
		}
	}
}
