package rules

import (
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The treasure generator's content (MR-044). Two files
// in effects/, both written by hand and checked by the loader:
//
//   - magic_item_values.json: the SRD 5.2.1 table "Magic Item Rarities and
//     Values" (p. 205, CC BY 4.0), the only part of the 2024 rules that
//     reaches the treasure. Values are in gold pieces (PO).
//   - treasure.json: OUR tables, in Portuguese. The SRD 5.1 has no random
//     treasure tables, so the coins per party-level band, the gems and art
//     objects with their names, and how many of each (and of which rarity of
//     magic item) a treasure holds are ours: our numbers and our words, with no
//     table from a book or from another site. generate.go rolls them.

// The two modes of a treasure.
const (
	// TreasureIndividual is what one creature carries: coins only.
	TreasureIndividual = "individual"
	// TreasureHoard ("covil") is what a lair guards: coins, gems, art objects and
	// magic items.
	TreasureHoard = "hoard"
)

// The coins. PC copper, PP silver ("peças de prata", 10 PP = 1 PO), PE electrum,
// PO gold and PL platinum.
const (
	CoinCopper   = "pc"
	CoinSilver   = "pp"
	CoinElectrum = "pe"
	CoinGold     = "po"
	CoinPlatinum = "pl"
)

// coinCopper is how many copper pieces a coin is worth.
var coinCopper = map[string]int{CoinCopper: 1, CoinSilver: 10, CoinElectrum: 50, CoinGold: 100, CoinPlatinum: 1000}

// Coins lists the coin types from the least valuable.
func Coins() []string {
	return []string{CoinCopper, CoinSilver, CoinElectrum, CoinGold, CoinPlatinum}
}

// What a treasure can hold at most, so its description always fits a map
// point's 2,000 characters and its total always fits a treasure's 1,000,000 PO.
const (
	maxTreasureGems  = 12
	maxTreasureArt   = 6
	maxTreasureItems = 6
	// maxTreasureValue is the most a treasure may be worth (a map point's limit).
	maxTreasureValue = 1_000_000
)

// MinTreasureLevel and MaxTreasureLevel bound the party level a treasure is
// generated for.
const (
	MinTreasureLevel = 1
	MaxTreasureLevel = 20
)

// treasureDice is a roll: Count d Sides, times Mult, plus Bonus ("2d6*100",
// "1d4+1"). Anything below 0 counts as 0.
type treasureDice struct {
	count, sides, mult, bonus int
}

var treasureDiceRE = regexp.MustCompile(`^(\d{1,2})d(\d{1,3})(?:\*(\d{1,5}))?(?:([+-])(\d{1,6}))?$`)

func parseTreasureDice(s string) (treasureDice, error) {
	m := treasureDiceRE.FindStringSubmatch(s)
	if m == nil {
		return treasureDice{}, fmt.Errorf("%q is not a roll like 2d6, 2d6*100 or 1d4+1", s)
	}
	d := treasureDice{mult: 1}
	d.count, _ = strconv.Atoi(m[1])
	d.sides, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		d.mult, _ = strconv.Atoi(m[3])
	}
	if m[5] != "" {
		d.bonus, _ = strconv.Atoi(m[5])
		if m[4] == "-" {
			d.bonus = -d.bonus
		}
	}
	if d.count < 1 || d.sides < 2 || d.mult < 1 {
		return treasureDice{}, fmt.Errorf("%q needs at least 1 die, 2 sides and a multiplier of 1", s)
	}
	return d, nil
}

// min and max are the smallest and the largest result.
func (d treasureDice) min() int { return max(d.count*d.mult+d.bonus, 0) }
func (d treasureDice) max() int { return max(d.count*d.sides*d.mult+d.bonus, 0) }

// coinRoll is one line of a coins table: the coin, what to roll, and the chance
// (1 to 100 percent) that the treasure has any of it.
type coinRoll struct {
	coin   string
	dice   treasureDice
	chance int
}

// treasureTier is a value tier of a gem or an art object, with the names it
// can have.
type treasureTier struct {
	valuePO int
	names   []string
}

// weighted is a choice with a weight: a value tier or a rarity.
type weighted struct {
	value  int    // a value tier, in PO
	rarity string // or a rarity
	weight int
}

// pickRoll is how many gems, art objects or magic items a hoard holds: the
// chance that it has any, how many, and the weights of what each one is.
type pickRoll struct {
	chance  int
	count   treasureDice
	choices []weighted
}

type treasureMode struct {
	coins []coinRoll
	// gems, art and items are nil when the mode has none (the individual one).
	gems, art, items *pickRoll
}

type treasureBand struct {
	from, to          int
	individual, hoard treasureMode
}

// treasureTables is what the two files hold, once checked.
type treasureTables struct {
	// rarityPO is the SRD 5.2.1 value of each rarity but the artifact's (which
	// has none), and consumableDivisor what a consumable's value is divided by.
	rarityPO          map[string]int
	consumableDivisor int
	valuesSource      string
	gems, art         []treasureTier
	bands             []treasureBand
	// units are the magic items a treasure rolls, by rarity, in the order of their
	// keys: the order of the drawing never depends on a Portuguese name, so fixing a
	// name does not change what a seed gives.
	units map[string][]MagicItemUnit
}

type valuesFile struct {
	Source            string         `json:"source"`
	ValuesPO          map[string]int `json:"values_po"`
	ConsumableDivisor int            `json:"consumable_divisor"`
}

type treasureFile struct {
	Source string             `json:"source"`
	Gems   []treasureTierJSON `json:"gems"`
	Art    []treasureTierJSON `json:"art"`
	Bands  []treasureBandJSON `json:"bands"`
}

// treasureTierJSON is a value tier of gems or art objects, with its names.
type treasureTierJSON struct {
	ValuePO int      `json:"value_po"`
	Names   []string `json:"names_pt"`
}

// treasureBandJSON is the treasure of a band of levels.
type treasureBandJSON struct {
	From       int          `json:"from"`
	To         int          `json:"to"`
	Individual treasureJSON `json:"individual"`
	Hoard      treasureJSON `json:"hoard"`
}

type treasureJSON struct {
	Coins []struct {
		Coin      string `json:"coin"`
		Dice      string `json:"dice"`
		ChancePct int    `json:"chance_pct"`
	} `json:"coins"`
	Gems  *pickJSON `json:"gems"`
	Art   *pickJSON `json:"art"`
	Items *pickJSON `json:"items"`
}

type pickJSON struct {
	Count     string `json:"count"`
	ChancePct int    `json:"chance_pct"`
	Tiers     []struct {
		ValuePO int `json:"value_po"`
		Weight  int `json:"weight"`
	} `json:"tiers"`
	Rarities []struct {
		Rarity string `json:"rarity"`
		Weight int    `json:"weight"`
	} `json:"rarities"`
}

// loadTreasure reads effects/magic_item_values.json and effects/treasure.json.
// It runs after the magic items are built: every rarity a table rolls must have
// items to roll. The files refuse anything out of shape: a rarity with no value
// (or one for the artifact, which has none), a roll that does not parse, a
// coin, a tier or a rarity that does not exist, a name used twice, bands that
// leave a level out, and a treasure that could hold more than the limits above.
func (c *content) loadTreasure(fsys fs.FS) error {
	if err := c.loadMagicItemValues(fsys); err != nil {
		return err
	}
	const name = treasureFileName
	var f treasureFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	if strings.TrimSpace(f.Source) == "" {
		return fmt.Errorf("%s: no source", name)
	}
	t := &c.treasure
	t.units = map[string][]MagicItemUnit{}
	for r, units := range c.magicUnits {
		sorted := slices.Clone(units)
		slices.SortStableFunc(sorted, func(a, b MagicItemUnit) int { return strings.Compare(a.Key, b.Key) })
		t.units[r] = sorted
	}
	names := map[string]bool{}
	var err error
	if t.gems, err = treasureTiers("gem", f.Gems, names); err != nil {
		return err
	}
	if t.art, err = treasureTiers("art", f.Art, names); err != nil {
		return err
	}
	return c.loadTreasureBands(f.Bands)
}

// treasureFileName is the file of the treasure tables.
const treasureFileName = "effects/treasure.json"

// The bounds of a value tier and of its names.
const (
	maxTierValuePO = 10000
	maxTierName    = 60
)

// treasureTiers checks the value tiers of a kind ("gem" or "art"): values
// that rise within bounds, and names, none repeated across the kinds (names
// holds those seen).
func treasureTiers(kind string, in []treasureTierJSON, names map[string]bool) ([]treasureTier, error) {
	const name = treasureFileName
	var out []treasureTier
	for i, tier := range in {
		switch {
		case tier.ValuePO < 1 || tier.ValuePO > maxTierValuePO:
			return nil, fmt.Errorf("%s: %s tier %d: value_po must be 1 to 10,000", name, kind, i)
		case i > 0 && tier.ValuePO <= out[i-1].valuePO:
			return nil, fmt.Errorf("%s: %s tiers must rise in value (%d)", name, kind, tier.ValuePO)
		case len(tier.Names) == 0:
			return nil, fmt.Errorf("%s: %s tier %d PO has no names", name, kind, tier.ValuePO)
		}
		if err := checkTierNames(kind, tier.Names, names); err != nil {
			return nil, err
		}
		out = append(out, treasureTier{valuePO: tier.ValuePO, names: slices.Clone(tier.Names)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no %s tiers", name, kind)
	}
	return out, nil
}

// checkTierNames refuses an empty name, one too long and one seen before.
func checkTierNames(kind string, tierNames []string, names map[string]bool) error {
	for _, n := range tierNames {
		if strings.TrimSpace(n) == "" || utf8.RuneCountInString(n) > maxTierName || names[n] {
			return fmt.Errorf("%s: %s name %q is empty, too long or repeated", treasureFileName, kind, n)
		}
		names[n] = true
	}
	return nil
}

// loadTreasureBands checks the bands, which must cover every level from
// MinTreasureLevel to MaxTreasureLevel in order, and their two modes.
func (c *content) loadTreasureBands(bands []treasureBandJSON) error {
	const name = treasureFileName
	if len(bands) == 0 {
		return fmt.Errorf("%s: no bands", name)
	}
	next := MinTreasureLevel
	for _, b := range bands {
		if b.From != next || b.To < b.From || b.To > MaxTreasureLevel {
			return fmt.Errorf("%s: band %d-%d must start at level %d and stay within %d", name, b.From, b.To, next, MaxTreasureLevel)
		}
		next = b.To + 1
		band, err := c.treasureBand(b)
		if err != nil {
			return err
		}
		c.treasure.bands = append(c.treasure.bands, band)
	}
	if next != MaxTreasureLevel+1 {
		return fmt.Errorf("%s: the bands stop at level %d, not %d", name, next-1, MaxTreasureLevel)
	}
	return nil
}

// treasureBand checks a band's individual mode, then its hoard.
func (c *content) treasureBand(b treasureBandJSON) (treasureBand, error) {
	band := treasureBand{from: b.From, to: b.To}
	where := fmt.Sprintf("%s: band %d-%d", treasureFileName, b.From, b.To)
	var err error
	if band.individual, err = c.checkTreasureMode(where+", individual", b.Individual, false); err != nil {
		return band, err
	}
	band.hoard, err = c.checkTreasureMode(where+", hoard", b.Hoard, true)
	return band, err
}

func (c *content) loadMagicItemValues(fsys fs.FS) error {
	const name = "effects/magic_item_values.json"
	var f valuesFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	if strings.TrimSpace(f.Source) == "" {
		return fmt.Errorf("%s: no source", name)
	}
	if f.ConsumableDivisor < 1 {
		return fmt.Errorf("%s: consumable_divisor must be at least 1", name)
	}
	// Every rarity but the artifact (priceless) has a value, and a rarer item is
	// worth more.
	prev := 0
	for _, r := range MagicRarities() {
		v, ok := f.ValuesPO[r]
		switch {
		case r == RarityArtifact && ok:
			return fmt.Errorf("%s: the artifact is priceless, it has no value", name)
		case r == RarityArtifact:
			continue
		case !ok || v <= prev:
			return fmt.Errorf("%s: %s needs a value above the previous rarity's", name, r)
		}
		prev = v
	}
	if len(f.ValuesPO) != len(MagicRarities())-1 {
		return fmt.Errorf("%s: values for rarities that do not exist", name)
	}
	c.treasure.rarityPO = f.ValuesPO
	c.treasure.consumableDivisor = f.ConsumableDivisor
	c.treasure.valuesSource = f.Source
	return nil
}

// maxChancePct is the highest chance a roll may have, in percent.
const maxChancePct = 100

// copperPerGold is how many copper pieces a gold piece is worth.
const copperPerGold = 100

// checkTreasureMode checks a mode's coins and, for a hoard, its gems, art and
// items, and the most its gold could be worth.
func (c *content) checkTreasureMode(where string, in treasureJSON, hoard bool) (treasureMode, error) {
	var m treasureMode
	coins, err := checkTreasureCoins(where, in)
	m.coins = coins
	if err != nil {
		return m, err
	}
	if !hoard {
		if in.Gems != nil || in.Art != nil || in.Items != nil {
			return m, fmt.Errorf("%s: an individual treasure is coins only", where)
		}
		return m, nil
	}
	if m.gems, err = c.checkPick(where+", gems", in.Gems, maxTreasureGems, c.treasure.gems); err != nil {
		return m, err
	}
	if m.art, err = c.checkPick(where+", art", in.Art, maxTreasureArt, c.treasure.art); err != nil {
		return m, err
	}
	if m.items, err = c.checkPick(where+", items", in.Items, maxTreasureItems, nil); err != nil {
		return m, err
	}
	if worst := worstTreasureGold(m); worst > maxTreasureValue {
		return m, fmt.Errorf("%s: could be worth %d PO, over the %d PO limit", where, worst, maxTreasureValue)
	}
	return m, nil
}

// checkTreasureCoins checks the coin rolls: each coin once, a roll that
// parses and a chance of 1 to 100. It returns the rolls read before a fault.
func checkTreasureCoins(where string, in treasureJSON) ([]coinRoll, error) {
	var out []coinRoll
	if len(in.Coins) == 0 {
		return out, fmt.Errorf("%s: no coins", where)
	}
	seen := map[string]bool{}
	for _, cr := range in.Coins {
		if _, ok := coinCopper[cr.Coin]; !ok || seen[cr.Coin] {
			return out, fmt.Errorf("%s: coin %q does not exist or is repeated", where, cr.Coin)
		}
		seen[cr.Coin] = true
		d, err := parseTreasureDice(cr.Dice)
		if err != nil {
			return out, fmt.Errorf("%s: %s: %w", where, cr.Coin, err)
		}
		if cr.ChancePct < 1 || cr.ChancePct > maxChancePct {
			return out, fmt.Errorf("%s: %s: chance_pct must be 1 to 100", where, cr.Coin)
		}
		out = append(out, coinRoll{coin: cr.Coin, dice: d, chance: cr.ChancePct})
	}
	return out, nil
}

// worstTreasureGold is the most a hoard's gold (coins, gems and art) could be
// worth. The gold becomes a point's value and must fit what a map point
// holds; the items do not count: they are never part of it.
func worstTreasureGold(m treasureMode) int {
	worst := 0
	for _, cr := range m.coins {
		worst += cr.dice.max() * coinCopper[cr.coin] / copperPerGold
	}
	top := func(p *pickRoll) int {
		best := 0
		for _, w := range p.choices {
			best = max(best, w.value)
		}
		return best * p.count.max()
	}
	return worst + top(m.gems) + top(m.art)
}

// checkPick checks a gems, art or items entry. tiers is the list the value tiers
// must come from, nil for the items, which weigh rarities.
func (c *content) checkPick(where string, in *pickJSON, limit int, tiers []treasureTier) (*pickRoll, error) {
	if in == nil {
		return nil, fmt.Errorf("%s: missing", where)
	}
	count, err := parseTreasureDice(in.Count)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if count.max() > limit || count.min() < 0 {
		return nil, fmt.Errorf("%s: %s can give more than %d", where, in.Count, limit)
	}
	if in.ChancePct < 1 || in.ChancePct > maxChancePct {
		return nil, fmt.Errorf("%s: chance_pct must be 1 to 100", where)
	}
	p := &pickRoll{chance: in.ChancePct, count: count}
	if tiers != nil {
		p.choices, err = pickTiers(where, in, tiers)
	} else {
		p.choices, err = c.pickRarities(where, in)
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// pickTiers reads the weighted tiers of gems or art, each one of tiers, once.
func pickTiers(where string, in *pickJSON, tiers []treasureTier) ([]weighted, error) {
	if len(in.Rarities) > 0 || len(in.Tiers) == 0 {
		return nil, fmt.Errorf("%s: needs tiers, not rarities", where)
	}
	var out []weighted
	seen := map[int]bool{}
	for _, t := range in.Tiers {
		ok := slices.ContainsFunc(tiers, func(x treasureTier) bool { return x.valuePO == t.ValuePO })
		if !ok || seen[t.ValuePO] || t.Weight < 1 {
			return nil, fmt.Errorf("%s: tier %d PO does not exist, is repeated or has no weight", where, t.ValuePO)
		}
		seen[t.ValuePO] = true
		out = append(out, weighted{value: t.ValuePO, weight: t.Weight})
	}
	return out, nil
}

// pickRarities reads the weighted rarities of the items: each a rarity a
// treasure rolls, once, with items to roll.
func (c *content) pickRarities(where string, in *pickJSON) ([]weighted, error) {
	if len(in.Tiers) > 0 || len(in.Rarities) == 0 {
		return nil, fmt.Errorf("%s: needs rarities, not tiers", where)
	}
	var out []weighted
	seen := map[string]bool{}
	for _, r := range in.Rarities {
		if !treasureRarity(r.Rarity) || seen[r.Rarity] || r.Weight < 1 {
			return nil, fmt.Errorf("%s: rarity %q is not one a treasure rolls, is repeated or has no weight", where, r.Rarity)
		}
		seen[r.Rarity] = true
		if len(c.magicUnits[r.Rarity]) == 0 {
			return nil, fmt.Errorf("%s: there are no %s magic items", where, r.Rarity)
		}
		out = append(out, weighted{rarity: r.Rarity, weight: r.Weight})
	}
	return out, nil
}

// treasureRarity says whether a treasure may roll the rarity. The artifact
// has no value and a treasure never holds one; "varies" is a family, not a
// rarity an item has.
func treasureRarity(r string) bool {
	return r != RarityArtifact && r != RarityVaries && slices.Contains(MagicRarities(), r)
}
