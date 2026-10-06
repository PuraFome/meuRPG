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

// The treasure generator's content (MR-044, Etapa 10, slice 10.10b). Two files
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
	Source string `json:"source"`
	Gems   []struct {
		ValuePO int      `json:"value_po"`
		Names   []string `json:"names_pt"`
	} `json:"gems"`
	Art []struct {
		ValuePO int      `json:"value_po"`
		Names   []string `json:"names_pt"`
	} `json:"art"`
	Bands []struct {
		From       int          `json:"from"`
		To         int          `json:"to"`
		Individual treasureJSON `json:"individual"`
		Hoard      treasureJSON `json:"hoard"`
	} `json:"bands"`
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
	const name = "effects/treasure.json"
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
	tiers := func(kind string, in []struct {
		ValuePO int      `json:"value_po"`
		Names   []string `json:"names_pt"`
	},
	) ([]treasureTier, error) {
		var out []treasureTier
		for i, in := range in {
			switch {
			case in.ValuePO < 1 || in.ValuePO > 10000:
				return nil, fmt.Errorf("%s: %s tier %d: value_po must be 1 to 10,000", name, kind, i)
			case i > 0 && in.ValuePO <= out[i-1].valuePO:
				return nil, fmt.Errorf("%s: %s tiers must rise in value (%d)", name, kind, in.ValuePO)
			case len(in.Names) == 0:
				return nil, fmt.Errorf("%s: %s tier %d PO has no names", name, kind, in.ValuePO)
			}
			for _, n := range in.Names {
				if strings.TrimSpace(n) == "" || utf8.RuneCountInString(n) > 60 || names[n] {
					return nil, fmt.Errorf("%s: %s name %q is empty, too long or repeated", name, kind, n)
				}
				names[n] = true
			}
			out = append(out, treasureTier{valuePO: in.ValuePO, names: slices.Clone(in.Names)})
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("%s: no %s tiers", name, kind)
		}
		return out, nil
	}
	var err error
	if t.gems, err = tiers("gem", f.Gems); err != nil {
		return err
	}
	if t.art, err = tiers("art", f.Art); err != nil {
		return err
	}
	if len(f.Bands) == 0 {
		return fmt.Errorf("%s: no bands", name)
	}
	next := MinTreasureLevel
	for _, b := range f.Bands {
		if b.From != next || b.To < b.From || b.To > MaxTreasureLevel {
			return fmt.Errorf("%s: band %d-%d must start at level %d and stay within %d", name, b.From, b.To, next, MaxTreasureLevel)
		}
		next = b.To + 1
		band := treasureBand{from: b.From, to: b.To}
		where := fmt.Sprintf("%s: band %d-%d", name, b.From, b.To)
		if band.individual, err = c.checkTreasureMode(where+", individual", b.Individual, false); err != nil {
			return err
		}
		if band.hoard, err = c.checkTreasureMode(where+", hoard", b.Hoard, true); err != nil {
			return err
		}
		t.bands = append(t.bands, band)
	}
	if next != MaxTreasureLevel+1 {
		return fmt.Errorf("%s: the bands stop at level %d, not %d", name, next-1, MaxTreasureLevel)
	}
	return nil
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

func (c *content) checkTreasureMode(where string, in treasureJSON, hoard bool) (treasureMode, error) {
	var m treasureMode
	if len(in.Coins) == 0 {
		return m, fmt.Errorf("%s: no coins", where)
	}
	seen := map[string]bool{}
	for _, cr := range in.Coins {
		if _, ok := coinCopper[cr.Coin]; !ok || seen[cr.Coin] {
			return m, fmt.Errorf("%s: coin %q does not exist or is repeated", where, cr.Coin)
		}
		seen[cr.Coin] = true
		d, err := parseTreasureDice(cr.Dice)
		if err != nil {
			return m, fmt.Errorf("%s: %s: %w", where, cr.Coin, err)
		}
		if cr.ChancePct < 1 || cr.ChancePct > 100 {
			return m, fmt.Errorf("%s: %s: chance_pct must be 1 to 100", where, cr.Coin)
		}
		m.coins = append(m.coins, coinRoll{coin: cr.Coin, dice: d, chance: cr.ChancePct})
	}
	if !hoard {
		if in.Gems != nil || in.Art != nil || in.Items != nil {
			return m, fmt.Errorf("%s: an individual treasure is coins only", where)
		}
		return m, nil
	}
	var err error
	if m.gems, err = c.checkPick(where+", gems", in.Gems, maxTreasureGems, c.treasure.gems); err != nil {
		return m, err
	}
	if m.art, err = c.checkPick(where+", art", in.Art, maxTreasureArt, c.treasure.art); err != nil {
		return m, err
	}
	if m.items, err = c.checkPick(where+", items", in.Items, maxTreasureItems, nil); err != nil {
		return m, err
	}
	// The gold (coins, gems and art) becomes a point's value and must fit what a map
	// point holds; the items do not count: they are never part of it.
	worst := 0
	for _, cr := range m.coins {
		worst += cr.dice.max() * coinCopper[cr.coin] / 100
	}
	top := func(p *pickRoll, value func(weighted) int) int {
		best := 0
		for _, w := range p.choices {
			best = max(best, value(w))
		}
		return best * p.count.max()
	}
	worst += top(m.gems, func(w weighted) int { return w.value })
	worst += top(m.art, func(w weighted) int { return w.value })
	if worst > maxTreasureValue {
		return m, fmt.Errorf("%s: could be worth %d PO, over the %d PO limit", where, worst, maxTreasureValue)
	}
	return m, nil
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
	if in.ChancePct < 1 || in.ChancePct > 100 {
		return nil, fmt.Errorf("%s: chance_pct must be 1 to 100", where)
	}
	p := &pickRoll{chance: in.ChancePct, count: count}
	if tiers != nil {
		if len(in.Rarities) > 0 || len(in.Tiers) == 0 {
			return nil, fmt.Errorf("%s: needs tiers, not rarities", where)
		}
		seen := map[int]bool{}
		for _, t := range in.Tiers {
			ok := slices.ContainsFunc(tiers, func(x treasureTier) bool { return x.valuePO == t.ValuePO })
			if !ok || seen[t.ValuePO] || t.Weight < 1 {
				return nil, fmt.Errorf("%s: tier %d PO does not exist, is repeated or has no weight", where, t.ValuePO)
			}
			seen[t.ValuePO] = true
			p.choices = append(p.choices, weighted{value: t.ValuePO, weight: t.Weight})
		}
		return p, nil
	}
	if len(in.Tiers) > 0 || len(in.Rarities) == 0 {
		return nil, fmt.Errorf("%s: needs rarities, not tiers", where)
	}
	seen := map[string]bool{}
	for _, r := range in.Rarities {
		// The artifact has no value and a treasure never holds one; "varies" is a
		// family, not a rarity an item has.
		if r.Rarity == RarityArtifact || r.Rarity == RarityVaries || !slices.Contains(MagicRarities(), r.Rarity) || seen[r.Rarity] || r.Weight < 1 {
			return nil, fmt.Errorf("%s: rarity %q is not one a treasure rolls, is repeated or has no weight", where, r.Rarity)
		}
		seen[r.Rarity] = true
		if len(c.magicUnits[r.Rarity]) == 0 {
			return nil, fmt.Errorf("%s: there are no %s magic items", where, r.Rarity)
		}
		p.choices = append(p.choices, weighted{rarity: r.Rarity, weight: r.Weight})
	}
	return p, nil
}
