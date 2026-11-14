package rules

import (
	"fmt"
	"io/fs"
	"slices"
)

// Ammunition (SRD 5.1, "Weapons": the ammunition property): a weapon with the property fires one kind of
// ammunition, and each attack spends one piece. effects/ammunition.json says which kind each such weapon fires.

const weaponPropertyAmmunition = "weapon-property:ammunition"

// loadAmmunition reads effects/ammunition.json, which maps the key of each weapon with the ammunition property to the
// key of the ammunition it fires. It refuses a key that is not a catalog weapon with the property, ammunition that
// is not catalog ammunition, and a weapon with the property that the file leaves out.
func (c *content) loadAmmunition(fsys fs.FS) error {
	const name = "effects/ammunition.json"
	var f map[string]string
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	for _, w := range sortedKeys(f) {
		eq, ok := c.equipment[w]
		if !ok || eq.Weapon == nil || !slices.Contains(eq.Weapon.Properties, weaponPropertyAmmunition) {
			return fmt.Errorf("%s: %s is not a weapon with the ammunition property", name, w)
		}
		if a, ok := c.equipment[f[w]]; !ok || a.Kind != "ammunition" {
			return fmt.Errorf("%s: %s fires %q, which is not ammunition of the catalog", name, w, f[w])
		}
	}
	for _, k := range sortedKeys(c.equipment) {
		if eq := c.equipment[k]; eq.Weapon != nil && slices.Contains(eq.Weapon.Properties, weaponPropertyAmmunition) && f[k] == "" {
			return fmt.Errorf("%s: %s has the ammunition property and fires nothing", name, k)
		}
	}
	c.ammunition = f
	return nil
}

// AmmunitionOf is the key of the ammunition a weapon fires, and false for a weapon that has none.
func (c *Content) AmmunitionOf(weaponKey string) (string, bool) {
	a, ok := c.c.ammunition[weaponKey]
	return a, ok
}

// AmmunitionKinds lists the keys of the ammunition the SRD has, sorted.
func (c *Content) AmmunitionKinds() []string {
	var out []string
	for _, k := range sortedKeys(c.c.equipment) {
		if c.c.equipment[k].Kind == "ammunition" {
			out = append(out, k)
		}
	}
	return out
}

// AmmunitionNamePT is the Portuguese name of a kind of ammunition, or "" for a key that is not one.
func (c *Content) AmmunitionNamePT(key string) string {
	if eq, ok := c.c.equipment[key]; ok && eq.Kind == "ammunition" {
		return c.c.namePT(key)
	}
	return ""
}
