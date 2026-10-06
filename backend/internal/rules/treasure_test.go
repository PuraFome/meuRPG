package rules

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// seeds is a spread of seeds for the property tests.
func seeds() []uint64 {
	out := []uint64{0, 1, 2, 3, 42, 2209, 5148, 1<<64 - 1, 1 << 63}
	for i := range uint64(300) {
		out = append(out, i*0x9E3779B97F4A7C15+i)
	}
	return out
}

func TestTreasureBands(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := map[int][2]int{1: {1, 4}, 4: {1, 4}, 5: {5, 10}, 10: {5, 10}, 11: {11, 16}, 16: {11, 16}, 17: {17, 20}, 20: {17, 20}}
	for level, b := range want {
		from, to, ok := c.TreasureBand(level)
		if !ok || from != b[0] || to != b[1] {
			t.Errorf("TreasureBand(%d) = %d-%d, %v; want %d-%d", level, from, to, ok, b[0], b[1])
		}
	}
	for _, level := range []int{0, -1, 21} {
		if _, _, ok := c.TreasureBand(level); ok {
			t.Errorf("TreasureBand(%d) is ok", level)
		}
		if _, err := c.GenerateTreasure(TreasureHoard, level, 1); !errors.Is(err, ErrTreasureLevel) {
			t.Errorf("GenerateTreasure(level %d) = %v, want ErrTreasureLevel", level, err)
		}
	}
	if _, err := c.GenerateTreasure("cache", 4, 1); !errors.Is(err, ErrTreasureMode) {
		t.Errorf("an unknown mode: %v", err)
	}
}

// The same request and seed give the same treasure, and a seed is not ignored.
func TestTreasureIsDeterministic(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, mode := range []string{TreasureIndividual, TreasureHoard} {
		differs := false
		for _, level := range []int{1, 4, 5, 10, 11, 16, 17, 20} {
			a, err := c.GenerateTreasure(mode, level, 2209)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := c.GenerateTreasure(mode, level, 2209)
			if !treasureEqual(a, b) {
				t.Errorf("%s level %d: the same seed gave two treasures", mode, level)
			}
			other, _ := c.GenerateTreasure(mode, level, 2210)
			differs = differs || !treasureEqual(a, other)
		}
		if !differs {
			t.Errorf("%s: two seeds never differed", mode)
		}
	}
}

func treasureEqual(a, b Treasure) bool {
	return a.Mode == b.Mode && a.Level == b.Level && a.Seed == b.Seed &&
		slices.Equal(a.Coins, b.Coins) && slices.Equal(a.Gems, b.Gems) && slices.Equal(a.Art, b.Art) && slices.Equal(a.Items, b.Items) &&
		a.CoinsPO == b.CoinsPO && a.GemsPO == b.GemsPO && a.ArtPO == b.ArtPO && a.GoldPO == b.GoldPO && a.ItemsPO == b.ItemsPO
}

// describe writes a treasure as one line: the coins, the gems and the art with their
// counts, and the items' keys.
func describe(tr Treasure) string {
	var got []string
	for _, k := range tr.Coins {
		got = append(got, k.Coin+"="+itoa(k.Count))
	}
	for _, g := range tr.Gems {
		got = append(got, itoa(g.Count)+"x"+g.NamePT)
	}
	for _, a := range tr.Art {
		got = append(got, itoa(a.Count)+"x"+a.NamePT)
	}
	for _, i := range tr.Items {
		got = append(got, i.Key)
	}
	return strings.Join(got, " ")
}

// Pinned: a seed gives the same treasure within a content version. One treasure per
// band and mode, with gems and art in every hoard. If this fails after a change to the
// tables or to the items, update the lines on purpose and raise the content revision;
// if it fails without one, the generator's stream changed.
func TestTreasureGolden(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, tc := range goldenTreasures {
		tr, err := c.GenerateTreasure(tc.mode, tc.level, tc.seed)
		if err != nil {
			t.Fatal(err)
		}
		if got := describe(tr); got != tc.want {
			t.Errorf("%s, level %d, seed %d:\n got %s\nwant %s", tc.mode, tc.level, tc.seed, got, tc.want)
		}
	}
}

// What every treasure must satisfy, for many seeds, modes and levels.
func TestTreasureInvariants(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, mode := range []string{TreasureIndividual, TreasureHoard} {
		for level := MinTreasureLevel; level <= MaxTreasureLevel; level++ {
			for _, seed := range seeds() {
				tr, err := c.GenerateTreasure(mode, level, seed)
				if err != nil {
					t.Fatal(err)
				}
				checkTreasure(t, c, tr)
			}
		}
	}
}

func checkTreasure(t *testing.T, c *Content, tr Treasure) {
	t.Helper()
	where := tr.Mode + " " + itoa(tr.Level) + " seed " + itoa(int(tr.Seed&0xffff))
	coins, prev := 0, 0
	for _, k := range tr.Coins {
		if k.Count < 1 || coinCopper[k.Coin] <= prev || k.ValuePO != k.Count*coinCopper[k.Coin]/100 {
			t.Fatalf("%s: a bad coin stack %+v", where, k)
		}
		prev = coinCopper[k.Coin]
		coins += k.ValuePO
	}
	pieces := func(list []TreasurePiece, limit int, tiers []treasureTier) int {
		sum, n := 0, 0
		for _, g := range list {
			ok := slices.ContainsFunc(tiers, func(x treasureTier) bool { return x.valuePO == g.ValuePO && slices.Contains(x.names, g.NamePT) })
			if !ok || g.Count < 1 {
				t.Fatalf("%s: %+v is not a piece of the tables", where, g)
			}
			sum += g.ValuePO * g.Count
			n += g.Count
		}
		if n > limit {
			t.Fatalf("%s: %d pieces, limit %d", where, n, limit)
		}
		return sum
	}
	gems := pieces(tr.Gems, maxTreasureGems, c.c.treasure.gems)
	art := pieces(tr.Art, maxTreasureArt, c.c.treasure.art)
	items := 0
	for _, it := range tr.Items {
		m, ok := c.MagicItem(it.Key)
		if !ok {
			t.Fatalf("%s: %s is not an SRD item", where, it.Key)
		}
		if m.Rarity == RarityArtifact || m.Rarity == RarityVaries || m.IsFamily() && !m.Standalone {
			t.Fatalf("%s: %s (rarity %s) can never be in a treasure", where, it.Key, m.Rarity)
		}
		if strings.TrimSpace(it.NamePT) == "" || it.NamePT == it.Name {
			t.Fatalf("%s: %s has no Portuguese name", where, it.Key)
		}
		v, ok := c.MagicItemValue(it.Key)
		if !ok || v.Priceless || v.PO != it.ValuePO || v.PO <= 0 {
			t.Fatalf("%s: %s is worth %+v, the treasure says %d", where, it.Key, v, it.ValuePO)
		}
		items += it.ValuePO
	}
	if len(tr.Items) > maxTreasureItems {
		t.Fatalf("%s: %d items", where, len(tr.Items))
	}
	if tr.Mode == TreasureIndividual && (len(tr.Gems)+len(tr.Art)+len(tr.Items) > 0) {
		t.Fatalf("%s: an individual treasure with more than coins", where)
	}
	if tr.CoinsPO != coins || tr.GemsPO != gems || tr.ArtPO != art || tr.ItemsPO != items || tr.GoldPO != coins+gems+art {
		t.Fatalf("%s: the totals do not add up: %+v", where, tr)
	}
	if tr.GoldPO+tr.ItemsPO > maxTreasureValue {
		t.Fatalf("%s: worth %d PO", where, tr.GoldPO+tr.ItemsPO)
	}
}

// A treasure's rolls never leave what the tables allow, level by level: the
// smallest and the largest a band can give.
func TestTreasureStaysInTheTables(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, b := range c.c.treasure.bands {
		for _, m := range []treasureMode{b.individual, b.hoard} {
			for _, cr := range m.coins {
				if cr.dice.max() < cr.dice.min() {
					t.Errorf("band %d-%d: %s max below min", b.from, b.to, cr.coin)
				}
			}
		}
	}
	// An individual treasure of level 1 to 4 is worth little, a level 17 to 20 hoard a lot.
	small, big := 0, 0
	for _, seed := range seeds() {
		a, _ := c.GenerateTreasure(TreasureIndividual, 2, seed)
		b, _ := c.GenerateTreasure(TreasureHoard, 18, seed)
		small = max(small, a.GoldPO)
		big = max(big, b.GoldPO+b.ItemsPO)
	}
	if small > 100 || big < 20000 {
		t.Errorf("an individual treasure of level 2 reached %d PO, a hoard of level 18 only %d PO", small, big)
	}
}

// Over many seeds a hoard holds what the table promises: at most the limits,
// and every rarity of the band is drawn at least once.
func TestHoardRarities(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, b := range c.c.treasure.bands {
		seen := map[string]bool{}
		for _, seed := range seeds() {
			tr, _ := c.GenerateTreasure(TreasureHoard, b.from, seed)
			for _, it := range tr.Items {
				seen[it.Rarity] = true
			}
		}
		for _, w := range b.hoard.items.choices {
			if !seen[w.rarity] {
				t.Errorf("band %d-%d: never drew a %s item", b.from, b.to, w.rarity)
			}
		}
		if len(seen) != len(b.hoard.items.choices) {
			t.Errorf("band %d-%d: drew rarities %v, the table has %d", b.from, b.to, seen, len(b.hoard.items.choices))
		}
	}
}

// A family is drawn as one unit, and what comes out is a variant of the chosen
// rarity; the Potion of Healing comes out as one of its four, never as "varies".
func TestFamiliesAreDrawnAsVariants(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	sawVariant := false
	for _, seed := range seeds() {
		for _, level := range []int{1, 5, 11, 17} {
			tr, _ := c.GenerateTreasure(TreasureHoard, level, seed)
			for _, it := range tr.Items {
				if m, _ := c.MagicItem(it.Key); m.VariantOf != "" {
					sawVariant = true
				}
			}
		}
	}
	if !sawVariant {
		t.Error("no family was ever drawn")
	}
}

// The values (SRD 5.2.1, p. 205), the halving of a consumable and the scroll rule.
func TestMagicItemValues(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	cases := []struct {
		key       string
		po        int
		halved    bool
		priceless bool
		ok        bool
	}{
		{"item:potion-of-healing-common", 50, true, false, true},   // 100, halved: a potion
		{"item:potion-of-healing-greater", 200, true, false, true}, // uncommon 400, halved
		{"item:cloak-of-elvenkind", 400, false, false, true},       // uncommon, not consumed
		{"item:ring-of-protection", 4000, false, false, true},      // rare
		{"item:armor-3", 200000, false, false, true},               // legendary
		{"item:ammunition-1", 200, true, false, true},              // uncommon ammunition: consumed
		{"item:spell-scroll-cantrip", 100, false, false, true},     // common scroll: not halved (the SRD note leaves scrolls out)
		{"item:spell-scroll-3rd", 400, false, false, true},         // uncommon
		{"item:spell-scroll-9th", 200000, false, false, true},      // legendary
		{"item:potion-of-healing", 0, false, false, false},         // a family that varies: no value of its own
		{"item:nope", 0, false, false, false},
	}
	for _, tc := range cases {
		v, ok := c.MagicItemValue(tc.key)
		if ok != tc.ok || v.PO != tc.po || v.Halved != tc.halved || v.Priceless != tc.priceless {
			t.Errorf("MagicItemValue(%s) = %+v, %v; want %d PO, halved %v, ok %v", tc.key, v, ok, tc.po, tc.halved, tc.ok)
		}
	}
	// The artifacts have no price.
	artifacts := 0
	for _, e := range c.MagicItems() {
		if e.Rarity != RarityArtifact {
			continue
		}
		artifacts++
		if v, ok := c.MagicItemValue(e.Key); !ok || !v.Priceless || v.PO != 0 {
			t.Errorf("%s: an artifact must be priceless, got %+v", e.Key, v)
		}
	}
	if artifacts == 0 {
		t.Error("the SRD has artifacts")
	}
	// Every scroll variant is worth its rarity's value, whole.
	for _, e := range c.MagicItems() {
		if !e.SpellScroll || e.IsFamily() {
			continue
		}
		v, _ := c.MagicItemValue(e.Key)
		if v.Halved || v.PO != v.RarityPO {
			t.Errorf("%s: a spell scroll must not be halved: %+v", e.Key, v)
		}
	}
	if !strings.Contains(c.MagicItemValuesSource(), "5.2.1") {
		t.Errorf("the source does not name the SRD 5.2.1: %q", c.MagicItemValuesSource())
	}
}

// The loader refuses tables out of shape.
func TestLoadTreasureRefuses(t *testing.T) {
	t.Parallel()
	base := loadForTest(t).c
	read := func(name string) string {
		b, err := fs.ReadFile(srd51.Files, "effects/"+name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	values, tables := read("magic_item_values.json"), read("treasure.json")
	load := func(values, tables string) error {
		c := *base
		c.treasure = treasureTables{}
		return c.loadTreasure(fstest.MapFS{
			"effects/magic_item_values.json": {Data: []byte(values)},
			"effects/treasure.json":          {Data: []byte(tables)},
		})
	}
	if err := load(values, tables); err != nil {
		t.Fatalf("the real files: %v", err)
	}
	swap := func(s, old, now string) string {
		if !strings.Contains(s, old) {
			t.Fatalf("the fixture has no %q", old)
		}
		return strings.Replace(s, old, now, 1)
	}
	badValues := map[string]string{
		"an artifact with a value": swap(values, `"legendary": 200000`, `"legendary": 200000, "artifact": 1`),
		"a missing rarity":         swap(values, `"rare": 4000,`, ``),
		"values that do not rise":  swap(values, `"rare": 4000`, `"rare": 300`),
		"a rarity that is not one": swap(values, `"legendary": 200000`, `"legendary": 200000, "mythic": 9`),
		"no divisor":               swap(values, `"consumable_divisor": 2`, `"consumable_divisor": 0`),
		"no source":                swap(values, `SRD 5.2.1 (regras de 2024), p. 205, tabela \"Magic Item Rarities and Values\" (CC BY 4.0)`, ` `),
		"an unknown field":         swap(values, `"consumable_divisor": 2`, `"consumable_divisor": 2, "x": 1`),
	}
	for name, body := range badValues {
		if err := load(body, tables); err == nil {
			t.Errorf("values: %s was accepted", name)
		}
	}
	badTables := map[string]string{
		"a roll that does not parse": swap(tables, `"3d8*4", "chance_pct": 100`, `"four dice", "chance_pct": 100`),
		"zero dice":                  swap(tables, `"3d8*4", "chance_pct": 100`, `"0d6", "chance_pct": 100`),
		"a coin that does not exist": swap(tables, `"coin": "pc"`, `"coin": "pz"`),
		"a chance over 100":          swap(tables, `"chance_pct": 70`, `"chance_pct": 101`),
		"a chance of 0":              swap(tables, `"chance_pct": 70`, `"chance_pct": 0`),
		"a tier that does not exist": swap(tables, `{"value_po": 6, "weight": 50}`, `{"value_po": 7, "weight": 50}`),
		"a tier with no weight":      swap(tables, `{"value_po": 6, "weight": 50}`, `{"value_po": 6, "weight": 0}`),
		"an artifact rarity":         swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "artifact", "weight": 5}`),
		"the varying rarity":         swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "varies", "weight": 5}`),
		"a rarity repeated":          swap(tables, `{"rarity": "rare", "weight": 5}`, `{"rarity": "common", "weight": 5}`),
		"too many gems":              swap(tables, `"count": "2d4", "chance_pct": 70`, `"count": "5d4", "chance_pct": 70`),
		"too many items":             swap(tables, `"count": "1d3", "chance_pct": 100`, `"count": "7d3", "chance_pct": 100`),
		"a name used twice":          swap(tables, `"Quartzo-leitoso"`, `"Quartzo-fumê"`),
		"a name over 60 characters":  swap(tables, `"Quartzo-fumê"`, `"`+strings.Repeat("x", 61)+`"`),
		"an art tier over 10,000 PO": swap(tables, `{"value_po": 6600, "names_pt"`, `{"value_po": 10001, "names_pt"`),
		"gems in an individual":      swap(tables, `"individual": {"coins": [`, `"individual": {"gems": {"count": "1d4", "chance_pct": 50, "tiers": [{"value_po": 6, "weight": 1}]}, "coins": [`),
		"an individual with no coins": swap(tables, `{"coin": "pc", "dice": "2d20*3", "chance_pct": 100},
    {"coin": "pp", "dice": "3d8*4", "chance_pct": 100},
    {"coin": "po", "dice": "1d8+1", "chance_pct": 60}`, ``),
		"tiers and rarities in a pick":  swap(tables, `"art": {"count": "1d2", "chance_pct": 40, "tiers": [`, `"art": {"count": "1d2", "chance_pct": 40, "rarities": [{"rarity": "common", "weight": 1}], "tiers": [`),
		"a gap between bands":           swap(tables, `"from": 5`, `"from": 6`),
		"bands that stop short":         swap(tables, `"to": 20`, `"to": 19`),
		"bands that go past 20":         swap(tables, `"to": 20`, `"to": 21`),
		"gem tiers out of order":        swap(tables, `{"value_po": 18, "names_pt": ["Ágata-de-fogo"`, `{"value_po": 3, "names_pt": ["Ágata-de-fogo"`),
		"an unknown field":              swap(tables, `"from": 1, "to": 4,`, `"from": 1, "to": 4, "x": 1,`),
		"no source":                     swap(tables, `"source": "Tabelas nossas`, `"source": "`+" "+`", "x2": "Tabelas nossas`),
		"a treasure worth over the cap": swap(tables, `"dice": "10d20*320"`, `"dice": "10d20*9999"`),
	}
	for name, body := range badTables {
		if err := load(values, body); err == nil {
			t.Errorf("tables: %s was accepted", name)
		}
	}
	// A rarity with no items to roll.
	empty := *base
	empty.treasure = treasureTables{}
	empty.magicUnits = map[string][]MagicItemUnit{}
	if err := empty.loadTreasure(srd51.Files); err == nil {
		t.Error("a table that rolls a rarity with no items was accepted")
	}
}

func TestParseTreasureDice(t *testing.T) {
	t.Parallel()
	good := map[string]treasureDice{
		"1d6":      {1, 6, 1, 0},
		"2d6*100":  {2, 6, 100, 0},
		"1d4+1":    {1, 4, 1, 1},
		"3d6*10-2": {3, 6, 10, -2},
	}
	for in, want := range good {
		got, err := parseTreasureDice(in)
		if err != nil || got != want {
			t.Errorf("%q = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "d6", "1d", "1d1", "0d6", "2d6*0", "2d6*", "1d6+", "6", "1d6 + 1", "100d6", "1d6*100000", "-1d6", "1d6*2*3"} {
		if _, err := parseTreasureDice(in); err == nil {
			t.Errorf("%q was accepted", in)
		}
	}
	if d, _ := parseTreasureDice("1d3-5"); d.min() != 0 || d.max() != 0 {
		t.Errorf("a roll below 0 must count as 0: %d to %d", d.min(), d.max())
	}
}

// The generator's own random source is pinned, so a seed means the same for ever.
func TestTreasureRNG(t *testing.T) {
	t.Parallel()
	r := newTreasureRNG(1, phaseCoins)
	var got [4]uint64
	for i := range got {
		got[i] = r.next()
	}
	again := newTreasureRNG(1, phaseCoins)
	for i := range got {
		if v := again.next(); v != got[i] {
			t.Fatalf("not repeatable at %d", i)
		}
	}
	if got[0] == got[1] || newTreasureRNG(1, phaseGems).next() == got[0] {
		t.Error("the streams must differ")
	}
	// intn stays in range and, over many draws, reaches both ends.
	lo, hi := 100, -1
	for range 10000 {
		n := r.intn(6)
		lo, hi = min(lo, n), max(hi, n)
	}
	if lo != 0 || hi != 5 {
		t.Errorf("intn(6) covered %d to %d", lo, hi)
	}
	if r.intn(1) != 0 || r.intn(0) != 0 || r.intn(-3) != 0 {
		t.Error("intn of one or less must be 0")
	}
}

func FuzzGenerateTreasure(f *testing.F) {
	c := loadForTest(f)
	f.Add(uint64(0), 1, true)
	f.Add(uint64(2209), 4, true)
	f.Add(uint64(1<<64-1), 20, false)
	f.Add(uint64(7), 0, true)
	f.Add(uint64(7), 21, false)
	f.Fuzz(func(t *testing.T, seed uint64, level int, hoard bool) {
		mode := TreasureIndividual
		if hoard {
			mode = TreasureHoard
		}
		tr, err := c.GenerateTreasure(mode, level, seed)
		if level < MinTreasureLevel || level > MaxTreasureLevel {
			if !errors.Is(err, ErrTreasureLevel) {
				t.Fatalf("level %d: %v", level, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		checkTreasure(t, c, tr)
		again, _ := c.GenerateTreasure(mode, level, seed)
		if !treasureEqual(tr, again) {
			t.Fatal("not deterministic")
		}
	})
}

func BenchmarkGenerateTreasure(b *testing.B) {
	c := loadForTest(b)
	b.ReportAllocs()
	for i := range b.N {
		if _, err := c.GenerateTreasure(TreasureHoard, 17, uint64(i)); err != nil { //nolint:gosec // G115: a counter
			b.Fatal(err)
		}
	}
}

var goldenTreasures = []struct {
	mode  string
	level int
	seed  uint64
	want  string
}{
	{"individual", 2, 2209, "pc=39 pp=28"},
	{"hoard", 2, 2219, "pc=98 pp=279 po=220 2xCalcedônia 1xQuartzo-fumê 1xÁgata-de-fogo 1xAmetista clara 2xOlho-de-gato 1xColher de peltre com o brasão de uma estalagem 1xCandeeiro de latão com vidro verde soprado item:spell-scroll-1st item:eyes-of-charming"},
	{"individual", 7, 2209, "pp=78 po=72"},
	{"hoard", 7, 2213, "pp=392 pe=800 po=1232 1xÁgata-de-fogo 2xCitrino pálido 1xRodonita 1xÁgua-marinha clara 1xGranada almandina 1xTopázio dourado 1xTurmalina verde 1xDedal de prata com um pássaro gravado item:armor-1 item:brooch-of-shielding item:javelin-of-lightning item:boots-of-the-winterlands item:dagger-of-venom"},
	{"individual", 13, 2209, "po=300 pl=40"},
	{"hoard", 13, 2213, "pe=2400 po=6715 pl=920 2xÁgua-marinha clara 1xCitrino 2xÁgua-marinha 1xTurmalina verde 1xAlexandrita clara 1xCrisoberilo 1xTopázio imperial 1xAlexandrita 1xOpala nobre 1xRelicário de prata com a miniatura de um santo-marinheiro item:amulet-of-the-planes item:boots-of-levitation item:mace-of-terror item:boots-of-the-winterlands item:dwarven-thrower"},
	{"individual", 19, 2209, "po=1392 pl=299"},
	{"hoard", 19, 2209, "po=24640 pl=4690 1xTopázio dourado 1xAlexandrita clara 1xTurmalina rubelita 1xAlexandrita 1xOpala nobre 1xEsmeralda lapidada 1xRubi 1xColar de pérolas de rio com fecho de ouro 1xEspelho de bronze polido numa moldura de jacarandá e ouro 1xAstrolábio de latão dourado de um piloto de naus item:potion-of-gaseous-form item:cloak-of-displacement item:spellguard-shield"},
}

// The generator is SplitMix64: pinned to its reference output for seed 0, so nothing
// changes the stream by accident.
func TestTreasureRNGMatchesSplitMix64(t *testing.T) {
	t.Parallel()
	r := treasureRNG{s: 0}
	for i, want := range []uint64{0xe220a8397b1dcdaf, 0x6e789e6aa1b965f4, 0x06c45d188009454f} {
		if got := r.next(); got != want {
			t.Errorf("output %d = %#x, want %#x", i, got, want)
		}
	}
	// The streams of one seed differ, and a seed and its neighbor never share one.
	seen := map[uint64]bool{}
	for _, seed := range []uint64{0, 1, 2, 3} {
		for _, ph := range []uint64{phaseCoins, phaseGems, phaseArt, phaseItems} {
			first := newTreasureRNG(seed, ph).next()
			if seen[first] {
				t.Errorf("seed %d, phase %#x starts like another stream", seed, ph)
			}
			seen[first] = true
		}
	}
}

// Every one of the 362 items: the value is the rarity's, halved exactly for a
// consumable that is not a Spell Scroll, and an artifact is priceless.
func TestMagicItemValuesOfAllItems(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	items := c.MagicItems()
	if len(items) != 362 {
		t.Fatalf("%d items", len(items))
	}
	for _, e := range items {
		v, ok := c.MagicItemValue(e.Key)
		switch e.Rarity {
		case RarityVaries:
			if ok {
				t.Errorf("%s: a family that varies has a value", e.Key)
			}
		case RarityArtifact:
			if !ok || !v.Priceless || v.PO != 0 || v.Halved {
				t.Errorf("%s: an artifact = %+v", e.Key, v)
			}
		default:
			want := c.c.treasure.rarityPO[e.Rarity]
			halved := e.Consumable && !e.SpellScroll
			if halved {
				want /= c.c.treasure.consumableDivisor
			}
			if !ok || v.Halved != halved || v.PO != want || v.RarityPO != c.c.treasure.rarityPO[e.Rarity] || v.Priceless {
				t.Errorf("%s: %+v, want %d PO, halved %v", e.Key, v, want, halved)
			}
		}
	}
}

// The seeds of the drawing do not depend on a Portuguese name: the units of a rarity
// are in the order of their keys.
func TestTreasureUnitsAreInKeyOrder(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for r, units := range c.c.treasure.units {
		if !slices.IsSortedFunc(units, func(a, b MagicItemUnit) int { return strings.Compare(a.Key, b.Key) }) {
			t.Errorf("%s: the units are not in key order", r)
		}
		if len(units) != len(c.c.magicUnits[r]) {
			t.Errorf("%s: %d units, want %d", r, len(units), len(c.c.magicUnits[r]))
		}
	}
}
