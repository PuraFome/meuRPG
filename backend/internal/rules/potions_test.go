package rules

import (
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

func potionsContent() *content {
	return &content{
		magicItems: map[string]*srd51.MagicItem{
			"item:heal":  {Key: "item:heal", Category: "potion"},
			"item:fire":  {Key: "item:fire", Category: "potion"},
			"item:wand":  {Key: "item:wand", Category: "wand"},
			"item:other": {Key: "item:other", Category: "potion"},
		},
		consumables: map[string]bool{"item:heal": true, "item:fire": true, "item:other": true},
		named:       map[string]*srd51.Named{"damage-type:fire": {}},
	}
}

func loadPotions(c *content, body string) error {
	return c.loadPotions(fstest.MapFS{"effects/potions.json": {Data: []byte(body)}})
}

func TestLoadPotionsAccepts(t *testing.T) {
	t.Parallel()
	c := potionsContent()
	body := `{"item:heal":{"kind":"heal","dice":"2d4+2"},"item:fire":{"kind":"resistance","damage_type":"damage-type:fire","rounds":600}}`
	if err := loadPotions(c, body); err != nil {
		t.Fatal(err)
	}
	got, ok := (&Content{c: c}).PotionEffect("item:heal")
	if !ok || got.Kind != PotionHeal || got.Heal != (DiceFormula{Count: 2, Sides: 4, Bonus: 2}) {
		t.Errorf("heal: %+v %v", got, ok)
	}
	got, ok = (&Content{c: c}).PotionEffect("item:fire")
	if !ok || got.Kind != PotionResistance || got.DamageType != "damage-type:fire" || got.Rounds != 600 {
		t.Errorf("resistance: %+v %v", got, ok)
	}
	if _, ok := (&Content{c: c}).PotionEffect("item:other"); ok {
		t.Error("a potion with no entry has an effect")
	}
}

func TestLoadPotionsRefuses(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"an unknown field":             `{"item:heal":{"kind":"heal","dice":"2d4","x":1}}`,
		"an unknown item":              `{"item:nope":{"kind":"heal","dice":"2d4"}}`,
		"an item that is not a use":    `{"item:wand":{"kind":"heal","dice":"2d4"}}`,
		"an unknown kind":              `{"item:heal":{"kind":"boom","dice":"2d4"}}`,
		"a heal with no dice":          `{"item:heal":{"kind":"heal"}}`,
		"a heal that is no dice":       `{"item:heal":{"kind":"heal","dice":"lots"}}`,
		"a heal of a flat number":      `{"item:heal":{"kind":"heal","dice":"5"}}`,
		"a heal with a damage type":    `{"item:heal":{"kind":"heal","dice":"2d4","damage_type":"damage-type:fire"}}`,
		"a heal with a duration":       `{"item:heal":{"kind":"heal","dice":"2d4","rounds":600}}`,
		"a resistance with dice":       `{"item:fire":{"kind":"resistance","dice":"2d4","damage_type":"damage-type:fire","rounds":600}}`,
		"a resistance with no type":    `{"item:fire":{"kind":"resistance","rounds":600}}`,
		"a resistance to no type":      `{"item:fire":{"kind":"resistance","damage_type":"damage-type:nope","rounds":600}}`,
		"a resistance to all":          `{"item:fire":{"kind":"resistance","damage_type":"all","rounds":600}}`,
		"a resistance with no time":    `{"item:fire":{"kind":"resistance","damage_type":"damage-type:fire"}}`,
		"a resistance past an hour":    `{"item:fire":{"kind":"resistance","damage_type":"damage-type:fire","rounds":601}}`,
		"a file that is not an object": `[1]`,
	} {
		if err := loadPotions(potionsContent(), body); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

var potionHeals = regexp.MustCompile(`regain (\d+d\d+) \+ (\d+) hit points`)

// Every potion of the file is a catalog consumable, each healing potion's dice are the ones its SRD text states, and
// each resistance potion names the damage type its text states, for one hour (600 rounds).
func TestPotionEffectsMatchTheCatalog(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	n := 0
	for _, key := range sortedKeys(c.c.potions) {
		n++
		e, _ := c.PotionEffect(key)
		m, ok := c.MagicItem(key)
		if !ok {
			t.Errorf("%s is not in the catalog", key)
			continue
		}
		if !m.Consumable {
			t.Errorf("%s is not a consumable", key)
		}
		text := strings.Join(m.Desc, " ")
		switch e.Kind {
		case PotionHeal:
			got := potionHeals.FindStringSubmatch(text)
			if got == nil || got[1]+"+"+got[2] != e.Heal.Text() {
				t.Errorf("%s heals %s, its text says %v", key, e.Heal.Text(), got)
			}
		case PotionResistance:
			want := strings.TrimPrefix(e.DamageType, "damage-type:")
			if !strings.Contains(text, "resistance to "+want+" damage for 1 hour") || e.Rounds != 600 {
				t.Errorf("%s: %+v does not match %q", key, e, text)
			}
		}
	}
	if n != 14 {
		t.Errorf("%d potions have effects, want 4 healing and 10 resistance", n)
	}
	for _, key := range []string{"item:potion-of-healing", "item:potion-of-resistance"} {
		if _, ok := c.PotionEffect(key); ok {
			t.Errorf("%s is a family with no effect of its own", key)
		}
	}
}
