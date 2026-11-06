package rules

import "slices"

// listsGear says key is the armor, the shield (when carried at all) or a weapon the
// sheet lists.
func (b Build) listsGear(key string) bool {
	switch key {
	case "":
		return false
	case shieldKey:
		return b.Shield
	case b.Armor:
		return true
	}
	return slices.Contains(b.Weapons, key)
}

// IsCarried says the piece of gear key is carried, not equipped.
func (b Build) IsCarried(key string) bool { return slices.Contains(b.Carried, key) }

// ArmorOn is the armor key when it is equipped, or empty.
func (b Build) ArmorOn() string {
	if b.IsCarried(b.Armor) {
		return ""
	}
	return b.Armor
}

// ShieldOn says the sheet has a shield and it is equipped.
func (b Build) ShieldOn() bool { return b.Shield && !b.IsCarried(shieldKey) }

// equip leaves in the deriver's Build only the armor and the shield that are equipped,
// so everything downstream reads them as the sheet's gear. A carried weapon stays
// listed, to keep its index in the issues, and is skipped where weapons count.
func (x *deriver) equip() {
	x.b.Armor = x.b.ArmorOn()
	x.b.Shield = x.b.ShieldOn()
}
