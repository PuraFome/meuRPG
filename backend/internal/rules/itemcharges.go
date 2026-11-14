package rules

import (
	"fmt"
	"io/fs"
)

// What happens when the last charge of an item is spent (SRD 5.1, the text of each wand, staff, ring and talisman).
const (
	// ChargesEmptyNone: nothing happens; the item waits for its charges.
	ChargesEmptyNone = "none"
	// ChargesEmptyD20: the owner rolls a d20 and on a 1 the item is lost; on a 20 some items regain charges.
	ChargesEmptyD20 = "d20"
	// ChargesEmptyDestroyed: the item is spent and lost at once.
	ChargesEmptyDestroyed = "destroyed"
)

// maxItemCharges is the most charges an item holds (the Gem of Brightness has 50).
const maxItemCharges = 100

// ItemCharges is what a catalog item says about its charges: how many it holds, when and how many come back, and what
// the last one does. The numbers are written by hand in effects/item_charges.json from each item's text.
type ItemCharges struct {
	// Max is the charges the item holds.
	Max int32
	// Recharge is RechargeDawn (the charges named by Regain return at dawn) or RechargeNone (they never return by
	// themselves).
	Recharge string
	// Regain is the dice of the charges that return at dawn; RegainAll says they all return instead.
	Regain    DiceFormula
	RegainAll bool
	// Empty is one of the ChargesEmpty values.
	Empty string
	// EmptyHighRegain is the dice of the charges an item regains when the d20 of ChargesEmptyD20 shows 20; zero
	// dice mean the item has no such rule.
	EmptyHighRegain DiceFormula
}

// chargesFile is one entry of effects/item_charges.json, which maps an item's key to it.
type chargesFile struct {
	Max             int32  `json:"max"`
	Recharge        string `json:"recharge"`
	Regain          string `json:"regain,omitempty"`
	Empty           string `json:"empty"`
	EmptyHighRegain string `json:"empty_high_regain,omitempty"`
}

// loadItemCharges reads effects/item_charges.json and refuses an entry of an item that is no catalog item or is a
// family (a sheet holds one of its variants), a count out of range, an unknown recharge or last-charge rule, a regain
// that is no dice, a regain without dawn, and a dawn without a regain.
func (c *content) loadItemCharges(fsys fs.FS) error {
	const name = "effects/item_charges.json"
	var f map[string]chargesFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	c.itemCharges = map[string]ItemCharges{}
	for _, k := range sortedKeys(f) {
		ic, err := c.itemChargesOf(k, f[k])
		if err != nil {
			return fmt.Errorf("%s: %s: %w", name, k, err)
		}
		c.itemCharges[k] = ic
	}
	return nil
}

func (c *content) itemChargesOf(key string, e chargesFile) (ItemCharges, error) {
	m, ok := c.magicItems[key]
	switch {
	case !ok:
		return ItemCharges{}, fmt.Errorf("is not a magic item")
	case len(m.Variants) > 0 && !m.Standalone:
		return ItemCharges{}, fmt.Errorf("is a family: give the numbers to its variants")
	case e.Max < 1 || e.Max > maxItemCharges:
		return ItemCharges{}, fmt.Errorf("max must be 1 to %d", maxItemCharges)
	case e.Recharge != RechargeDawn && e.Recharge != RechargeNone:
		return ItemCharges{}, fmt.Errorf("recharge must be %q or %q", RechargeDawn, RechargeNone)
	case e.Empty != ChargesEmptyNone && e.Empty != ChargesEmptyD20 && e.Empty != ChargesEmptyDestroyed:
		return ItemCharges{}, fmt.Errorf("empty is none, d20 or destroyed")
	}
	ic := ItemCharges{Max: e.Max, Recharge: e.Recharge, Empty: e.Empty}
	if err := readRegain(&ic, e); err != nil {
		return ItemCharges{}, err
	}
	if e.EmptyHighRegain != "" {
		d, ok := ParseDice(e.EmptyHighRegain)
		if e.Empty != ChargesEmptyD20 || !ok || d.Count < 1 {
			return ItemCharges{}, fmt.Errorf("empty_high_regain is dice, and only for an item that rolls a d20")
		}
		ic.EmptyHighRegain = d
	}
	return ic, nil
}

// readRegain reads the regain of an entry: "all" or dice, and only for an item that recharges at dawn.
func readRegain(ic *ItemCharges, e chargesFile) error {
	if (e.Regain == "") != (e.Recharge == RechargeNone) {
		return fmt.Errorf("regain is given when, and only when, the item recharges at dawn")
	}
	switch e.Regain {
	case "":
	case "all":
		ic.RegainAll = true
	default:
		d, ok := ParseDice(e.Regain)
		if !ok || d.Count < 1 {
			return fmt.Errorf("regain is \"all\" or dice such as 1d6+1")
		}
		ic.Regain = d
	}
	return nil
}

// ItemCharges is the charges of a catalog item, and false for an item with none.
func (c *Content) ItemCharges(key string) (ItemCharges, bool) {
	ic, ok := c.c.itemCharges[key]
	return ic, ok
}
