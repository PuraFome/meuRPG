package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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
	if got.Max != 7 || got.Recharge != RechargeDawn || got.Empty != ChargesEmptyD20 || got.Lost != LostGone || got.Regain != (DiceFormula{Count: 1, Sides: 6, Bonus: 1}) || got.RegainAll {
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
	for _, good := range []string{
		`{"item:wand":{"max":7,"recharge":"dawn","regain":"1d6+1","empty":"d20","lost":"gone"}}`,
		`{"item:wand":{"max":3,"start":"1d4-1","recharge":"none","empty":"none"}}`,
		`{"item:wand":{"max":24,"recharge":"dawns_after_empty","wait_dawns":7,"regain":"all","empty":"none"}}`,
		`{"item:wand":{"max":50,"recharge":"dawn","regain":"4d6+2","empty":"d20","empty_high_regain":"1d12+1"}}`,
		`{"item:wand":{"max":12,"recharge":"none","empty":"always","lost":"plain"}}`,
	} {
		if err := loadCharges(chargesContent(), good); err != nil {
			t.Fatalf("a good file %s: %v", good, err)
		}
	}
	for name, body := range map[string]string{
		"an unknown field":               `{"item:wand":{"max":7,"recharge":"none","empty":"none","x":1}}`,
		"an unknown item":                `{"item:nope":{"max":7,"recharge":"none","empty":"none"}}`,
		"a family":                       `{"item:family":{"max":7,"recharge":"none","empty":"none"}}`,
		"no charges":                     `{"item:wand":{"max":0,"recharge":"none","empty":"none"}}`,
		"too many charges":               `{"item:wand":{"max":101,"recharge":"none","empty":"none"}}`,
		"an unknown recharge":            `{"item:wand":{"max":7,"recharge":"weekly","empty":"none"}}`,
		"an unknown empty":               `{"item:wand":{"max":7,"recharge":"none","empty":"boom"}}`,
		"dawn with nothing regained":     `{"item:wand":{"max":7,"recharge":"dawn","empty":"none"}}`,
		"a regain without a recharge":    `{"item:wand":{"max":7,"recharge":"none","regain":"1d3","empty":"none"}}`,
		"a regain that is no dice":       `{"item:wand":{"max":7,"recharge":"dawn","regain":"lots","empty":"none"}}`,
		"a regain of zero dice":          `{"item:wand":{"max":7,"recharge":"dawn","regain":"0","empty":"none"}}`,
		"dawns without a wait":           `{"item:wand":{"max":7,"recharge":"dawns_after_empty","regain":"all","empty":"none"}}`,
		"a wait without dawns":           `{"item:wand":{"max":7,"recharge":"dawn","wait_dawns":7,"regain":"all","empty":"none"}}`,
		"dawns that regain dice":         `{"item:wand":{"max":7,"recharge":"dawns_after_empty","wait_dawns":7,"regain":"1d3","empty":"none"}}`,
		"a start that is no dice":        `{"item:wand":{"max":3,"start":"x","recharge":"none","empty":"none"}}`,
		"a start that rolls past max":    `{"item:wand":{"max":3,"start":"1d8+1","recharge":"none","empty":"none"}}`,
		"a twenty without a d20":         `{"item:wand":{"max":7,"recharge":"dawn","regain":"1d3","empty":"none","empty_high_regain":"1d8"}}`,
		"a twenty that is no dice":       `{"item:wand":{"max":7,"recharge":"dawn","regain":"1d3","empty":"d20","empty_high_regain":"x"}}`,
		"a d20 that does nothing":        `{"item:wand":{"max":7,"recharge":"dawn","regain":"1d3","empty":"d20"}}`,
		"a loss that is unknown":         `{"item:wand":{"max":7,"recharge":"none","empty":"always","lost":"vanished"}}`,
		"a loss at once with no how":     `{"item:wand":{"max":7,"recharge":"none","empty":"always"}}`,
		"a loss on an item that is none": `{"item:wand":{"max":7,"recharge":"none","empty":"none","lost":"gone"}}`,
		"a file that is not an object":   `[1]`,
	} {
		if err := loadCharges(chargesContent(), body); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

// A luck blade rolls its charges, so a new one has none until the master sets them; the goat figurine waits 7 dawns.
func TestItemChargesStartAndWait(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for key, start := range map[string]DiceFormula{"item:luck-blade": {Count: 1, Sides: 4, Bonus: -1}, "item:nine-lives-stealer": {Count: 1, Sides: 8, Bonus: 1}} {
		got, _ := c.ItemCharges(key)
		if !got.HasStart() || got.Start != start || got.Recharge != RechargeNone {
			t.Errorf("%s: %+v", key, got)
		}
	}
	if w, _ := c.ItemCharges("item:wand-of-fear"); w.HasStart() {
		t.Error("a wand rolls its charges")
	}
	goats, _ := c.ItemCharges("item:figurine-of-wondrous-power-ivory-goats")
	if goats.Recharge != RechargeDawnsAfterEmpty || goats.WaitDawns != 7 || !goats.RegainAll || goats.Max != 24 {
		t.Errorf("goats: %+v", goats)
	}
}

// What the last charge does is the item's: wands are gone, three staffs turn into a plain quarterstaff, the Staff of
// Power keeps its bonus, and the Staff of the Magi only has the 20.
func TestItemChargesLoss(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := map[string][2]string{
		"item:wand-of-wonder": {ChargesEmptyD20, LostGone}, "item:staff-of-fire": {ChargesEmptyD20, LostGone},
		"item:staff-of-healing": {ChargesEmptyD20, LostGone}, "item:staff-of-charming": {ChargesEmptyD20, LostPlain},
		"item:staff-of-striking": {ChargesEmptyD20, LostPlain}, "item:staff-of-the-woodlands": {ChargesEmptyD20, LostPlain},
		"item:staff-of-power": {ChargesEmptyD20, LostPartly}, "item:staff-of-the-magi": {ChargesEmptyD20, ""},
		"item:ring-of-three-wishes": {ChargesEmptyAlways, LostPlain}, "item:gem-of-brightness": {ChargesEmptyAlways, LostPlain},
		"item:scarab-of-protection": {ChargesEmptyAlways, LostGone}, "item:cubic-gate": {ChargesEmptyNone, ""},
	}
	for key, w := range want {
		got, ok := c.ItemCharges(key)
		if !ok || got.Empty != w[0] || got.Lost != w[1] {
			t.Errorf("%s: %+v, want %v", key, got, w)
		}
	}
}

// A maximum that changed under a stored count never leaves a number out of 0 to the maximum.
func TestItemChargesLeftFollowsTheMaximum(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ max, used, want int32 }{
		{7, 0, 7}, {7, 3, 4}, {7, 7, 0}, {7, 99, 0}, {10, 3, 7}, {2, 3, 0}, {7, -4, 7},
	} {
		if got := (ItemCharges{Max: tc.max}).Left(tc.used); got != tc.want {
			t.Errorf("max %d used %d: left %d, want %d", tc.max, tc.used, got, tc.want)
		}
	}
}

var (
	chargesMaxText   = regexp.MustCompile(`(?i)(?:\b(?:has|have|with|starts with)|of its) (\d+) charges`)
	chargesRegain    = regexp.MustCompile(`(?i)regains? (all|\d+d\d+(?: \+ \d+)?) expended charges daily at dawn`)
	chargesRegainAll = regexp.MustCompile(`(?i)regain all expended charges daily at dawn`)
)

// wordingItWontParse lists the entries whose text the patterns above cannot read, with why: each is checked by hand.
var wordingItWontParse = map[string]string{
	"item:luck-blade":                             "the charges are rolled (1d4 - 1), so the text states no maximum",
	"item:nine-lives-stealer":                     "the charges are rolled (1d8 + 1), so the text states no maximum",
	"item:figurine-of-wondrous-power-ivory-goats": "the text states 24 charges but regains them after 7 days, not at dawn",
}

// Each entry's maximum and regain match what the SRD text states, so a number mistyped in the data fails here.
func TestItemChargesMatchTheSRDText(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	checked := 0
	for _, e := range c.MagicItems() {
		ic, ok := c.ItemCharges(e.Key)
		if !ok {
			continue
		}
		if _, skip := wordingItWontParse[e.Key]; skip {
			continue
		}
		m, _ := c.MagicItem(e.Key)
		text := strings.Join(m.Desc, " ")
		got := chargesMaxText.FindStringSubmatch(text)
		if got == nil {
			t.Errorf("%s: the text states no maximum the pattern reads; fix the data or list it in wordingItWontParse", e.Key)
			continue
		}
		if n, _ := strconv.Atoi(got[1]); n != int(ic.Max) {
			t.Errorf("%s: max %d, the text says %s", e.Key, ic.Max, got[1])
		}
		checkRegainText(t, e.Key, ic, text)
		checked++
	}
	if checked < 45 {
		t.Errorf("only %d entries were checked against their text", checked)
	}
}

// checkRegainText compares what an entry regains at dawn with the sentence of the text.
func checkRegainText(t *testing.T, key string, ic ItemCharges, text string) {
	t.Helper()
	m := chargesRegain.FindStringSubmatch(text)
	switch {
	case ic.Recharge != RechargeDawn && m != nil:
		t.Errorf("%s: the text regains charges at dawn and the data says %s", key, ic.Recharge)
	case ic.Recharge == RechargeDawn && m == nil && !chargesRegainAll.MatchString(text):
		t.Errorf("%s: the data regains charges at dawn and the text does not say so", key)
	case ic.Recharge == RechargeDawn && ic.RegainAll != strings.EqualFold(m[1], "all"):
		t.Errorf("%s: regain all is %v, the text says %q", key, ic.RegainAll, m[1])
	case ic.Recharge == RechargeDawn && !ic.RegainAll:
		got := fmt.Sprintf("%dd%d", ic.Regain.Count, ic.Regain.Sides)
		if ic.Regain.Bonus != 0 {
			got += fmt.Sprintf("+%d", ic.Regain.Bonus)
		}
		if want := strings.ReplaceAll(m[1], " ", ""); got != want {
			t.Errorf("%s: regain %s, the text says %s", key, got, want)
		}
	}
}
