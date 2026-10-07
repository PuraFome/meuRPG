package rules

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
)

// The generator of treasure (MR-044). It is a pure function of the mode, the
// party level and the seed: the same three always give the same treasure, and
// nothing here reads the clock or a random source. The tables it rolls are in
// treasure.go (loaded from effects/treasure.json).

// ErrTreasureMode is returned for a mode that is not TreasureIndividual or
// TreasureHoard, and ErrTreasureLevel for a party level outside 1 to 20.
var (
	ErrTreasureMode  = errors.New("rules: treasure mode is not individual or hoard")
	ErrTreasureLevel = errors.New("rules: party level must be 1 to 20")
)

// TreasureCoin is a stack of one coin: how many, and what they come to in PO.
type TreasureCoin struct {
	// Coin is one of Coins ("pc", "pp", "pe", "po", "pl").
	Coin string
	// Count is how many pieces of the coin; ValuePO their worth in whole PO
	// (rounded down: 7 PC are worth 0 PO).
	Count, ValuePO int
}

// TreasurePiece is one kind of gem or art object, with how many of it.
type TreasurePiece struct {
	NamePT string
	// ValuePO is the worth of one; Count how many there are.
	ValuePO, Count int
}

// TreasureItem is a magic item of a treasure.
type TreasureItem struct {
	// Key is the item ("item:cloak-of-elvenkind"); for a family that was drawn it
	// is the variant ("item:armor-1"). Name is the SRD's name and NamePT ours.
	Key, Name, NamePT string
	Category, Rarity  string
	// ValuePO is the SRD 5.2.1 value of its rarity, halved for a consumable; it
	// is never an artifact's, because a treasure never holds an artifact.
	ValuePO int
	// Consumable says the item is used up, and Halved that ValuePO is half of
	// the rarity's value because of it. SpellScroll marks a Spell Scroll, which
	// the SRD 5.2.1 note leaves out of the halving.
	Consumable, Halved, SpellScroll bool
	// Attunement says it needs attunement; AttunementByPT is who, in Portuguese,
	// or "".
	Attunement     bool
	AttunementByPT string
}

// Treasure is what GenerateTreasure rolled.
type Treasure struct {
	Mode  string
	Level int
	Seed  uint64
	// ContentVersion is Content.Version() when it was rolled: a seed gives the same
	// treasure only within one content version (a change to the tables or to the
	// items changes every seed), so a treasure is placed only under the version it
	// was generated with.
	ContentVersion string
	// Coins lists the coins that came up, from the least valuable.
	Coins []TreasureCoin
	// Gems and Art are grouped by what they are, the cheapest first.
	Gems, Art []TreasurePiece
	Items     []TreasureItem
	// CoinsPO, GemsPO and ArtPO add up what the coins, gems and art come to; GoldPO
	// is their sum: the gold of the treasure point (MR-041). ItemsPO is the value
	// of the magic items, shown apart: they stay items, and never become XP.
	CoinsPO, GemsPO, ArtPO, GoldPO int
	ItemsPO                        int
}

// TreasureBand returns the party-level band of a level, as "1-4", "5-10", "11-16"
// or "17-20", and false for a level outside 1 to 20.
func (c *Content) TreasureBand(level int) (from, to int, ok bool) {
	for _, b := range c.c.treasure.bands {
		if level >= b.from && level <= b.to {
			return b.from, b.to, true
		}
	}
	return 0, 0, false
}

// GenerateTreasure rolls the treasure of a mode ("individual" or "hoard") for a
// party of that level. Coins, gems, art and items each have a stream of their
// own, so the same seed keeps the coins when only the tables of the items change.
// The result is stable within one content version (Content.Version) and no longer.
func (c *Content) GenerateTreasure(mode string, level int, seed uint64) (Treasure, error) {
	if mode != TreasureIndividual && mode != TreasureHoard {
		return Treasure{}, ErrTreasureMode
	}
	var band *treasureBand
	for i := range c.c.treasure.bands {
		if b := &c.c.treasure.bands[i]; level >= b.from && level <= b.to {
			band = b
		}
	}
	if band == nil {
		return Treasure{}, ErrTreasureLevel
	}
	m := band.individual
	if mode == TreasureHoard {
		m = band.hoard
	}
	t := Treasure{Mode: mode, Level: level, Seed: seed, ContentVersion: c.Version()}

	coins := newTreasureRNG(seed, phaseCoins)
	for _, cr := range m.coins {
		if coins.intn(100) >= cr.chance {
			continue
		}
		n := coins.roll(cr.dice)
		if n == 0 {
			continue
		}
		v := n * coinCopper[cr.coin] / 100
		t.Coins = append(t.Coins, TreasureCoin{Coin: cr.coin, Count: n, ValuePO: v})
		t.CoinsPO += v
	}
	slices.SortFunc(t.Coins, func(a, b TreasureCoin) int { return coinCopper[a.Coin] - coinCopper[b.Coin] })

	if m.gems != nil {
		t.Gems, t.GemsPO = c.c.drawPieces(newTreasureRNG(seed, phaseGems), m.gems, c.c.treasure.gems)
	}
	if m.art != nil {
		t.Art, t.ArtPO = c.c.drawPieces(newTreasureRNG(seed, phaseArt), m.art, c.c.treasure.art)
	}
	if m.items != nil {
		t.Items, t.ItemsPO = c.c.drawItems(newTreasureRNG(seed, phaseItems), m.items)
	}
	t.GoldPO = t.CoinsPO + t.GemsPO + t.ArtPO
	return t, nil
}

// drawPieces rolls the gems or the art objects of a hoard, and groups them by
// name: two Agates are "2 x Ágata".
func (c *content) drawPieces(r *treasureRNG, p *pickRoll, tiers []treasureTier) ([]TreasurePiece, int) {
	if r.intn(100) >= p.chance {
		return nil, 0
	}
	type key struct {
		valuePO int
		name    string
	}
	count := map[key]int{}
	for range r.roll(p.count) {
		value := p.choices[r.weighted(p.choices)].value
		for _, t := range tiers {
			if t.valuePO == value {
				count[key{value, t.names[r.intn(len(t.names))]}]++
			}
		}
	}
	keys := make([]key, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b key) int {
		if a.valuePO != b.valuePO {
			return a.valuePO - b.valuePO
		}
		return comparePT(a.name, b.name)
	})
	var out []TreasurePiece
	total := 0
	for _, k := range keys {
		out = append(out, TreasurePiece{NamePT: k.name, ValuePO: k.valuePO, Count: count[k]})
		total += k.valuePO * count[k]
	}
	return out, total
}

// drawItems rolls the magic items of a hoard: a rarity by weight, then one unit
// of that rarity (a family is one unit, so it weighs as one item), then one of
// the unit's options.
func (c *content) drawItems(r *treasureRNG, p *pickRoll) ([]TreasureItem, int) {
	if r.intn(100) >= p.chance {
		return nil, 0
	}
	var out []TreasureItem
	total := 0
	for range r.roll(p.count) {
		rarity := p.choices[r.weighted(p.choices)].rarity
		units := c.treasure.units[rarity]
		unit := units[r.intn(len(units))]
		key := unit.Options[r.intn(len(unit.Options))]
		it := c.treasureItem(key)
		out = append(out, it)
		total += it.ValuePO
	}
	return out, total
}

func (c *content) treasureItem(key string) TreasureItem {
	e := c.magicItemEntry(c.magicItems[key])
	v, _ := c.magicItemValue(e)
	return TreasureItem{
		Key: e.Key, Name: e.Name, NamePT: e.NamePT, Category: e.Category, Rarity: e.Rarity,
		ValuePO: v.PO, Consumable: e.Consumable, Halved: v.Halved, SpellScroll: e.SpellScroll,
		Attunement: e.Attunement, AttunementByPT: e.AttunementByPT,
	}
}

// ItemValue is what a magic item is worth by the SRD 5.2.1 table.
type ItemValue struct {
	// PO is the value in gold pieces; 0 for a Priceless item.
	PO int
	// Priceless is the artifact's value: none.
	Priceless bool
	// Halved says PO is half the rarity's value because the item is consumed. A
	// Spell Scroll is a consumable that the SRD 5.2.1 does not halve, so it is not.
	Halved bool
	// RarityPO is the value of the item's rarity before any halving.
	RarityPO int
}

// MagicItemValue returns the value of a magic item by its key: the SRD 5.2.1
// value of its rarity ("Magic Item Rarities and Values", p. 205), halved for a
// consumable other than a Spell Scroll. ok is false for an unknown key and for a
// family whose rarity "varies" (its variants have values, the family has none).
func (c *Content) MagicItemValue(key string) (ItemValue, bool) {
	m, ok := c.c.magicItems[key]
	if !ok {
		return ItemValue{}, false
	}
	return c.c.magicItemValue(c.c.magicItemEntry(m))
}

func (c *content) magicItemValue(e MagicItemEntry) (ItemValue, bool) {
	switch e.Rarity {
	case RarityVaries:
		return ItemValue{}, false
	case RarityArtifact:
		return ItemValue{Priceless: true}, true
	}
	v := ItemValue{PO: c.treasure.rarityPO[e.Rarity], RarityPO: c.treasure.rarityPO[e.Rarity]}
	if e.Consumable && !e.SpellScroll {
		v.PO /= c.treasure.consumableDivisor
		v.Halved = true
	}
	return v, true
}

// MagicItemValuesSource says where the values come from, as text for the screen.
func (c *Content) MagicItemValuesSource() string { return c.c.treasure.valuesSource }

// treasureRNG is the generator of a stream. It is SplitMix64 (Steele, Lea and
// Flood, OOPSLA 2014) with Lemire's bounded integers (ACM TOMS 29(1), 2019),
// written here and not taken from math/rand, whose stream is not promised to
// stay the same across Go releases: a seed must give the same treasure
// forever.
type treasureRNG struct{ s uint64 }

// The streams.
const (
	phaseCoins uint64 = 0xA1C0_1457_3EE5_0001
	phaseGems  uint64 = 0xA1C0_1457_3EE5_0002
	phaseArt   uint64 = 0xA1C0_1457_3EE5_0003
	phaseItems uint64 = 0xA1C0_1457_3EE5_0004
)

func newTreasureRNG(seed, phase uint64) *treasureRNG {
	// The phase is mixed before it meets the seed, so two streams never start from
	// states that differ by a constant and the streams of different seeds never alias.
	mix := treasureRNG{s: phase}
	return &treasureRNG{s: seed ^ mix.next()}
}

func (r *treasureRNG) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// intn is an integer in [0, n), without bias; 0 for n <= 1 and no output used.
func (r *treasureRNG) intn(n int) int {
	if n <= 1 {
		return 0
	}
	un := uint64(n)
	hi, lo := bits.Mul64(r.next(), un)
	if lo < un {
		t := -un % un
		for lo < t {
			hi, lo = bits.Mul64(r.next(), un)
		}
	}
	return int(hi) //nolint:gosec // G115: hi < n
}

// roll adds up the dice: Count d Sides, times Mult, plus Bonus, never below 0.
func (r *treasureRNG) roll(d treasureDice) int {
	sum := 0
	for range d.count {
		sum += 1 + r.intn(d.sides)
	}
	return max(sum*d.mult+d.bonus, 0)
}

// weighted returns the index of a choice, by weight.
func (r *treasureRNG) weighted(choices []weighted) int {
	total := 0
	for _, c := range choices {
		total += c.weight
	}
	n := r.intn(total)
	for i, c := range choices {
		if n < c.weight {
			return i
		}
		n -= c.weight
	}
	panic(fmt.Sprintf("rules: weighted pick %d of %d", n, total)) // never: n < total
}
