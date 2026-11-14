package rules

import (
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

var chargesWord = regexp.MustCompile(`\bcharges?\b`)

// Every item of the catalog that a sheet can hold and whose text speaks of charges has its numbers, so no such item
// is added without them.
func TestEveryItemWithChargesHasTheirNumbers(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for _, e := range c.MagicItems() {
		m, _ := c.MagicItem(e.Key)
		if e.IsFamily() && !e.Standalone {
			if _, ok := c.ItemCharges(e.Key); ok {
				t.Errorf("%s is a family, which no sheet holds, and has charges", e.Key)
			}
			continue
		}
		text := ""
		for _, p := range m.Desc {
			text += p + " "
		}
		_, has := c.ItemCharges(e.Key)
		if chargesWord.MatchString(text) && !has {
			t.Errorf("%s speaks of charges and has no numbers in effects/item_charges.json", e.Key)
		}
		if !chargesWord.MatchString(text) && has {
			t.Errorf("%s has charges numbers and its text never says charge", e.Key)
		}
	}
}

// A wand of magic missiles has 7 charges, regains 1d6+1 at dawn and rolls a d20 when the last one goes.
func TestItemChargesOfAWand(t *testing.T) {
	t.Parallel()
	got, ok := loadForTest(t).ItemCharges("item:wand-of-magic-missiles")
	if !ok {
		t.Fatal("no charges")
	}
	if got.Max != 7 || got.Recharge != RechargeDawn || got.Empty != ChargesEmptyD20 || got.Regain != (DiceFormula{Count: 1, Sides: 6, Bonus: 1}) || got.RegainAll {
		t.Errorf("got %+v", got)
	}
	if _, ok := loadForTest(t).ItemCharges("item:bag-of-holding"); ok {
		t.Error("a bag of holding has charges")
	}
}

// The eyes of charming regain all their charges, and a staff of power may regain some on a natural 20.
func TestItemChargesRegainAllAndTwenty(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	eyes, _ := c.ItemCharges("item:eyes-of-charming")
	if !eyes.RegainAll || eyes.Max != 3 {
		t.Errorf("eyes: %+v", eyes)
	}
	staff, _ := c.ItemCharges("item:staff-of-power")
	if staff.EmptyHighRegain != (DiceFormula{Count: 1, Sides: 8, Bonus: 2}) || staff.Max != 20 {
		t.Errorf("staff: %+v", staff)
	}
}

func chargesContent() *content {
	return &content{magicItems: map[string]*srd51.MagicItem{
		"item:wand":   {Key: "item:wand", Category: "wand"},
		"item:family": {Key: "item:family", Category: "ring", Variants: []string{"item:variant"}},
	}}
}

func loadCharges(c *content, body string) error {
	return c.loadItemCharges(fstest.MapFS{"effects/item_charges.json": {Data: []byte(body)}})
}

// The loader refuses what the closed schema does not allow.
func TestLoadItemChargesRefuses(t *testing.T) {
	t.Parallel()
	if err := loadCharges(chargesContent(), `{"item:wand":{"max":7,"recharge":"dawn","regain":"1d6+1","empty":"d20"}}`); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	for name, body := range map[string]string{
		"an unknown field":             `{"item:wand":{"max":7,"recharge":"none","empty":"none","x":1}}`,
		"an unknown item":              `{"item:nope":{"max":7,"recharge":"none","empty":"none"}}`,
		"a family":                     `{"item:family":{"max":7,"recharge":"none","empty":"none"}}`,
		"no charges":                   `{"item:wand":{"max":0,"recharge":"none","empty":"none"}}`,
		"too many charges":             `{"item:wand":{"max":101,"recharge":"none","empty":"none"}}`,
		"an unknown recharge":          `{"item:wand":{"max":7,"recharge":"weekly","empty":"none"}}`,
		"an unknown empty":             `{"item:wand":{"max":7,"recharge":"none","empty":"boom"}}`,
		"dawn with nothing regained":   `{"item:wand":{"max":7,"recharge":"dawn","empty":"none"}}`,
		"a regain without dawn":        `{"item:wand":{"max":7,"recharge":"none","regain":"1d3","empty":"none"}}`,
		"a regain that is no dice":     `{"item:wand":{"max":7,"recharge":"dawn","regain":"lots","empty":"none"}}`,
		"a regain of zero dice":        `{"item:wand":{"max":7,"recharge":"dawn","regain":"0","empty":"none"}}`,
		"a twenty without a d20":       `{"item:wand":{"max":7,"recharge":"dawn","regain":"1d3","empty":"none","empty_high_regain":"1d8"}}`,
		"a twenty that is no dice":     `{"item:wand":{"max":7,"recharge":"dawn","regain":"1d3","empty":"d20","empty_high_regain":"x"}}`,
		"a file that is not an object": `[1]`,
	} {
		if err := loadCharges(chargesContent(), body); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}
