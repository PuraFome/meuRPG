package rules

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// Every weapon with the ammunition property has the kind of ammunition it fires, and the SRD gives four kinds.
func TestEveryAmmunitionWeaponHasItsAmmunition(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := map[string]string{
		"equipment:shortbow": "equipment:arrow", "equipment:longbow": "equipment:arrow",
		"equipment:crossbow-light": "equipment:crossbow-bolt", "equipment:crossbow-hand": "equipment:crossbow-bolt",
		"equipment:crossbow-heavy": "equipment:crossbow-bolt", "equipment:sling": "equipment:sling-bullet",
		"equipment:blowgun": "equipment:blowgun-needle",
	}
	for weapon, ammo := range want {
		if got, ok := c.AmmunitionOf(weapon); !ok || got != ammo {
			t.Errorf("%s fires %q (%v), want %s", weapon, got, ok, ammo)
		}
	}
	if _, ok := c.AmmunitionOf("equipment:longsword"); ok {
		t.Error("a longsword fires ammunition")
	}
	kinds := c.AmmunitionKinds()
	if !slices.Equal(kinds, []string{"equipment:arrow", "equipment:blowgun-needle", "equipment:crossbow-bolt", "equipment:sling-bullet"}) {
		t.Errorf("kinds = %v", kinds)
	}
	if c.AmmunitionNamePT("equipment:arrow") != "Flecha" {
		t.Errorf("name = %q", c.AmmunitionNamePT("equipment:arrow"))
	}
}

func ammunitionContent() *content {
	return &content{equipment: map[string]*srd51.Equipment{
		"equipment:bow":    {Key: "equipment:bow", Kind: "weapon", Weapon: &srd51.Weapon{Properties: []string{"weapon-property:ammunition"}}},
		"equipment:sword":  {Key: "equipment:sword", Kind: "weapon", Weapon: &srd51.Weapon{}},
		"equipment:arrow":  {Key: "equipment:arrow", Kind: "ammunition"},
		"equipment:rope":   {Key: "equipment:rope", Kind: "tool"},
		"equipment:arrow2": {Key: "equipment:arrow2", Kind: "ammunition"},
	}}
}

func loadAmmunition(c *content, body string) error {
	return c.loadAmmunition(fstest.MapFS{"effects/ammunition.json": {Data: []byte(body)}})
}

// The loader refuses what the closed schema does not allow.
func TestLoadAmmunitionRefuses(t *testing.T) {
	t.Parallel()
	if err := loadAmmunition(ammunitionContent(), `{"equipment:bow":"equipment:arrow"}`); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	for name, body := range map[string]string{
		"a weapon that is not in the catalog":   `{"equipment:nope":"equipment:arrow"}`,
		"a weapon without the property":         `{"equipment:sword":"equipment:arrow"}`,
		"ammunition that is not ammunition":     `{"equipment:bow":"equipment:rope"}`,
		"ammunition that is not in the catalog": `{"equipment:bow":"equipment:nope"}`,
		"a weapon with the property left out":   `{}`,
		"a file that is not an object":          `[1]`,
	} {
		if err := loadAmmunition(ammunitionContent(), body); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}
