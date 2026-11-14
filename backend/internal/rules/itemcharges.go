package rules

import (
	"fmt"
	"io/fs"
)

// What happens when the last charge of an item is spent (SRD 5.1, the text of each wand, staff, ring and talisman).
const (
	// ChargesEmptyNone: nothing happens; the item waits for its charges.
	ChargesEmptyNone = "none"
	// ChargesEmptyD20: the owner rolls a d20. A 1 loses the item as the entry's Lost says, and a 20 gives some
	// charges back on the items that say so.
	ChargesEmptyD20 = "d20"
	// ChargesEmptyAlways: the item is lost at once, as the entry's Lost says.
	ChargesEmptyAlways = "always"
)

// How an item is lost (ItemCharges.Lost).
const (
	// LostGone: the item is destroyed and leaves the sheet's use: a wand that crumbles, a staff that turns to water.
	LostGone = "gone"
	// LostPlain: the item becomes a plain, nonmagical one that stays on the sheet as what it is made of: a staff
	// that becomes a nonmagical quarterstaff.
	LostPlain = "plain"
	// LostPartly: the item keeps its bonus to attack and damage rolls and loses every other property (Staff of
	// Power).
	LostPartly = "partly"
)

// RechargeDawnsAfterEmpty is the Recharge of an item that waits a number of dawns after its last charge and then
// regains them all (the goat figurines of the Figurine of Wondrous Power).
const RechargeDawnsAfterEmpty = "dawns_after_empty"

// maxItemCharges is the most charges an item holds (the Gem of Brightness has 50).
const maxItemCharges = 100

// ItemCharges is what a catalog item says about its charges: how many it holds, when and how many come back, and what
// the last one does. The numbers are written by hand in effects/item_charges.json from each item's text.
type ItemCharges struct {
	// Max is the charges the item holds.
	Max int32
	// Start is the dice of the charges a new copy holds when the SRD rolls them (Luck Blade 1d4 - 1); zero dice mean
	// a new copy is full. A copy with a Start has no count until the master sets it.
	Start DiceFormula
	// Recharge is RechargeDawn (the charges named by Regain return at dawn), RechargeNone (they never return by
	// themselves) or RechargeDawnsAfterEmpty (all return WaitDawns dawns after the last one was spent).
	Recharge  string
	WaitDawns int32
	// Regain is the dice of the charges that return at dawn; RegainAll says they all return instead.
	Regain    DiceFormula
	RegainAll bool
	// Empty is one of the ChargesEmpty values.
	Empty string
	// Lost is how the item is lost, one of LostGone, LostPlain and LostPartly: on a 1 of the d20 when Empty is
	// ChargesEmptyD20, at once when it is ChargesEmptyAlways, and "" when the item is never lost.
	Lost string
	// EmptyHighRegain is the dice of the charges an item regains when the d20 of ChargesEmptyD20 shows 20; zero
	// dice mean the item has no such rule.
	EmptyHighRegain DiceFormula
}

// HasStart says a new copy of the item holds a rolled number of charges that the master sets.
func (ic ItemCharges) HasStart() bool { return ic.Start.Count > 0 }

// Left is the charges an item has when used charges were spent. A count the maximum has left behind (the catalog
// changed) reads as an empty item and never below 0.
func (ic ItemCharges) Left(used int32) int32 {
	return max(0, ic.Max-min(max(used, 0), ic.Max))
}

// chargesFile is one entry of effects/item_charges.json, which maps an item's key to it.
type chargesFile struct {
	Max             int32  `json:"max"`
	Start           string `json:"start,omitempty"`
	Recharge        string `json:"recharge"`
	WaitDawns       int32  `json:"wait_dawns,omitempty"`
	Regain          string `json:"regain,omitempty"`
	Empty           string `json:"empty"`
	Lost            string `json:"lost,omitempty"`
	EmptyHighRegain string `json:"empty_high_regain,omitempty"`
}

// loadItemCharges reads effects/item_charges.json and refuses an entry of an item that is no catalog item or is a
// family (a sheet holds one of its variants), a count out of range, an unknown value, a regain that is no dice, and
// a regain that does not fit its recharge.
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
	if err := c.checkChargesEntry(key, e); err != nil {
		return ItemCharges{}, err
	}
	ic := ItemCharges{Max: e.Max, Recharge: e.Recharge, WaitDawns: e.WaitDawns, Empty: e.Empty, Lost: e.Lost}
	if err := readRegain(&ic, e); err != nil {
		return ItemCharges{}, err
	}
	if err := readDice(&ic.Start, e.Start, "start"); err != nil {
		return ItemCharges{}, err
	}
	if ic.HasStart() && ic.Start.Count*ic.Start.Sides+ic.Start.Bonus != int(e.Max) {
		return ItemCharges{}, fmt.Errorf("start must roll up to max")
	}
	if err := readDice(&ic.EmptyHighRegain, e.EmptyHighRegain, "empty_high_regain"); err != nil {
		return ItemCharges{}, err
	}
	return ic, nil
}

// checkChargesEntry refuses an entry of a key that is no item or a family, and the values out of range or out of
// step with each other.
func (c *content) checkChargesEntry(key string, e chargesFile) error {
	m, ok := c.magicItems[key]
	switch {
	case !ok:
		return fmt.Errorf("is not a magic item")
	case len(m.Variants) > 0 && !m.Standalone:
		return fmt.Errorf("is a family: give the numbers to its variants")
	case e.Max < 1 || e.Max > maxItemCharges:
		return fmt.Errorf("max must be 1 to %d", maxItemCharges)
	case e.Recharge != RechargeDawn && e.Recharge != RechargeNone && e.Recharge != RechargeDawnsAfterEmpty:
		return fmt.Errorf("recharge must be %q, %q or %q", RechargeDawn, RechargeNone, RechargeDawnsAfterEmpty)
	case (e.Recharge == RechargeDawnsAfterEmpty) != (e.WaitDawns > 0):
		return fmt.Errorf("wait_dawns is given when, and only when, the item recharges as %q", RechargeDawnsAfterEmpty)
	case e.Empty != ChargesEmptyNone && e.Empty != ChargesEmptyD20 && e.Empty != ChargesEmptyAlways:
		return fmt.Errorf("empty is none, d20 or always")
	}
	return checkChargesLoss(e)
}

// checkChargesLoss checks how the last charge loses the item: an item lost at once says how, an item that rolls a
// d20 says how on a 1 or what a 20 gives back, and an item that does nothing says neither.
func checkChargesLoss(e chargesFile) error {
	switch {
	case e.Lost != "" && e.Lost != LostGone && e.Lost != LostPlain && e.Lost != LostPartly:
		return fmt.Errorf("lost is gone, plain or partly")
	case e.Empty == ChargesEmptyAlways && e.Lost == "":
		return fmt.Errorf("an item lost at once says how it is lost")
	case e.Empty == ChargesEmptyNone && (e.Lost != "" || e.EmptyHighRegain != ""):
		return fmt.Errorf("lost and empty_high_regain are for an item whose last charge does something")
	case e.Empty == ChargesEmptyD20 && e.Lost == "" && e.EmptyHighRegain == "":
		return fmt.Errorf("an item that rolls a d20 loses the item on a 1 or regains charges on a 20")
	case e.EmptyHighRegain != "" && e.Empty != ChargesEmptyD20:
		return fmt.Errorf("empty_high_regain is only for an item that rolls a d20")
	}
	return nil
}

// readRegain reads the regain of an entry: "all" or dice, as the recharge allows.
func readRegain(ic *ItemCharges, e chargesFile) error {
	if (e.Regain == "") != (e.Recharge == RechargeNone) {
		return fmt.Errorf("regain is given when, and only when, the item recharges")
	}
	switch {
	case e.Regain == "":
		return nil
	case e.Regain == "all":
		ic.RegainAll = true
		return nil
	case e.Recharge == RechargeDawnsAfterEmpty:
		return fmt.Errorf("an item that waits for its dawns regains \"all\"")
	}
	return readDice(&ic.Regain, e.Regain, "regain")
}

// readDice reads dice such as 1d6+1 into d when given, and refuses text that is not dice.
func readDice(d *DiceFormula, text, field string) error {
	if text == "" {
		return nil
	}
	f, ok := ParseDice(text)
	if !ok || f.Count < 1 {
		return fmt.Errorf("%s is dice such as 1d6+1", field)
	}
	*d = f
	return nil
}

// ItemCharges is the charges of a catalog item, and false for an item with none.
func (c *Content) ItemCharges(key string) (ItemCharges, bool) {
	ic, ok := c.c.itemCharges[key]
	return ic, ok
}
