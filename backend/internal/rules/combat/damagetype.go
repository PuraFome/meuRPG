package combat

import "slices"

// TypeModifiers are the damage types a creature takes double, half or none of
// (SRD 5.1, "Damage Resistance and Vulnerability"), as damage type keys such as
// "damage-type:fire".
type TypeModifiers struct {
	Vulnerable, Resistant, Immune []string
}

// AdjustForType is the damage that lands when amount of damageType hits a
// creature with these modifiers. It applies after every bonus and reduction,
// so it takes the amount the damage already came to: immunity makes it 0,
// resistance halves it (rounded down) and vulnerability doubles it. A creature
// that is both resistant and vulnerable gets both, in that order. A damage with
// no type is never adjusted.
func AdjustForType(amount int, damageType string, m TypeModifiers) int {
	if damageType == "" || amount <= 0 {
		return max(amount, 0)
	}
	if slices.Contains(m.Immune, damageType) {
		return 0
	}
	if slices.Contains(m.Resistant, damageType) {
		amount /= 2
	}
	if slices.Contains(m.Vulnerable, damageType) {
		amount *= 2
	}
	return amount
}
