package rules

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// summonLoadCases are summon entries, one fault at a time and some at once,
// for loadSummon; the spell is of the 3rd circle.
func summonLoadCases() map[string]string {
	list := func(rest string) string {
		return `{"attack":"none","creatures":["monster:a","monster:b"],"count":1` + rest + `}`
	}
	band := func(rest string) string {
		return `{"attack":"full","type":"beast","options":[{"count":1,"max_cr":"2"},{"count":8,"max_cr":"1/4"}]` + rest + `}`
	}
	unlock := func(u string) string { return list(`,"unlocks":[` + u + `]`) }
	return map[string]string{
		"ok: a list":                     list(``),
		"ok: a list growing per circle":  list(`,"count_per_level":2`),
		"ok: a list with unlocks":        unlock(`{"feature":"feature:pact","creatures":["monster:c"],"attack":"reaction"},{"feature":"feature:other","creatures":["monster:d"],"attack":"full"}`),
		"ok: bands":                      band(``),
		"ok: bands with multipliers":     band(`,"multiplier_at_level":{"9":4,"5":2,"7":3}`),
		"bad: attack empty":              `{"creatures":["monster:a"],"count":1}`,
		"bad: attack unknown":            `{"attack":"bite","creatures":["monster:a"],"count":1}`,
		"bad: neither list nor bands":    `{"attack":"none"}`,
		"bad: both list and bands":       `{"attack":"none","creatures":["monster:a"],"options":[{"count":1,"max_cr":"1"}]}`,
		"bad: list count 0":              `{"attack":"none","creatures":["monster:a"]}`,
		"bad: list count per circle < 0": list(`,"count_per_level":-1`),
		"bad: list with a type":          list(`,"type":"beast"`),
		"bad: list with multipliers":     list(`,"multiplier_at_level":{"5":2}`),
		"bad: list unknown creature":     `{"attack":"none","creatures":["monster:a","monster:nope"],"count":1}`,
		"bad: list twice":                `{"attack":"none","creatures":["monster:a","monster:a"],"count":1}`,
		"bad: unlock unknown feature":    unlock(`{"feature":"feature:nope","creatures":["monster:c"],"attack":"full"}`),
		"bad: unlock no creatures":       unlock(`{"feature":"feature:pact","attack":"full"}`),
		"bad: unlock no attack":          unlock(`{"feature":"feature:pact","creatures":["monster:c"]}`),
		"bad: unlock unknown creature":   unlock(`{"feature":"feature:pact","creatures":["monster:nope"],"attack":"full"}`),
		"bad: unlock repeats the list":   unlock(`{"feature":"feature:pact","creatures":["monster:a"],"attack":"full"}`),
		"bad: unlock repeats an unlock":  unlock(`{"feature":"feature:pact","creatures":["monster:c"],"attack":"full"},{"feature":"feature:other","creatures":["monster:c"],"attack":"full"}`),
		"bad: bands with a count":        band(`,"count":1`),
		"bad: bands with a count per":    band(`,"count_per_level":1`),
		"bad: bands with unlocks":        band(`,"unlocks":[{"feature":"feature:pact"}]`),
		"bad: bands of no type":          `{"attack":"full","options":[{"count":1,"max_cr":"2"}]}`,
		"bad: bands of an unknown type":  `{"attack":"full","type":"dragonkin","options":[{"count":1,"max_cr":"2"}]}`,
		"bad: band count 0":              `{"attack":"full","type":"beast","options":[{"count":1,"max_cr":"2"},{"max_cr":"2"}]}`,
		"bad: band cr unknown":           `{"attack":"full","type":"beast","options":[{"count":1,"max_cr":"1/3"}]}`,
		"bad: band cr empty":             `{"attack":"full","type":"beast","options":[{"count":1}]}`,
		"bad: multiplier not a circle":   band(`,"multiplier_at_level":{"five":2}`),
		"bad: multiplier at own circle":  band(`,"multiplier_at_level":{"3":2}`),
		"bad: multiplier at circle 10":   band(`,"multiplier_at_level":{"10":2}`),
		"bad: multiplier of 1":           band(`,"multiplier_at_level":{"5":1}`),
		"ok: multiplier at circle 9":     band(`,"multiplier_at_level":{"9":2}`),
		"order: attack before shape":     `{"attack":"x"}`,
		"order: shape before list count": `{"attack":"none","creatures":["monster:nope"],"options":[{"count":1,"max_cr":"1"}]}`,
		"order: count before creatures":  `{"attack":"none","creatures":["monster:nope"]}`,
		"order: unknown before twice":    `{"attack":"none","creatures":["monster:nope","monster:nope"],"count":1}`,
		"order: list before unlocks":     `{"attack":"none","creatures":["monster:nope"],"count":1,"unlocks":[{"feature":"feature:nope"}]}`,
		"order: unlock feature first":    unlock(`{"feature":"feature:nope","attack":"x"}`),
		"order: unlock shape first":      unlock(`{"feature":"feature:pact","creatures":["monster:nope"],"attack":"x"}`),
		"order: bands count before type": `{"attack":"full","type":"dragonkin","count":1,"options":[{"count":1,"max_cr":"2"}]}`,
		"order: type before bands":       `{"attack":"full","type":"dragonkin","options":[{"count":0,"max_cr":"2"}]}`,
		"order: bands before multiplier": `{"attack":"full","type":"beast","options":[{"count":0,"max_cr":"2"}],"multiplier_at_level":{"1":1}}`,
	}
}

func summonContent() *content {
	return &content{
		monsters: map[string]*srd51.Monster{"monster:a": {}, "monster:b": {}, "monster:c": {}, "monster:d": {}},
		features: map[string]*srd51.Feature{"feature:pact": {}, "feature:other": {}},
	}
}

// TestLoadSummonCharacterization freezes loadSummon's refusals and the
// definition it stores.
func TestLoadSummonCharacterization(t *testing.T) {
	t.Parallel()
	got := map[string]string{}
	for name, doc := range summonLoadCases() {
		var in summonJSON
		if err := json.Unmarshal([]byte(doc), &in); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		c := summonContent()
		fail := func(format string, a ...any) error { return fmt.Errorf("spell:s: "+format, a...) }
		err := c.loadSummon("spell:s", 3, &in, fail)
		text := errText(err)
		if err == nil {
			text += fmt.Sprintf(" %+v", *c.summons["spell:s"])
		} else if c.summons != nil {
			t.Errorf("%s: a refused entry was stored", name)
		}
		if strings.HasPrefix(name, "ok:") != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
		got[name] = text
	}
	goldenText(t, "summon-load-refusals", got)
}

// maxSummonCircleForTest is the highest circle of a slot.
const maxSummonCircleForTest = 9

// TestSummonOptionsCharacterization freezes what SummonOptions offers for each
// summoning spell of the content at each circle, with and without the Pact of
// the Chain, and its refusals.
func TestSummonOptionsCharacterization(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	builds := map[string]Build{"none": {}, "chain": warlockOfTheChain()}
	got := map[string]string{}
	for _, spell := range append(c.SummonSpells(), "spell:fireball") {
		for circle := range maxSummonCircleForTest + 2 {
			for bn, b := range builds {
				opts, err := c.SummonOptions(spell, circle, b)
				text := errText(err)
				if err == nil {
					text = fmt.Sprintf("%+v", opts)
				}
				got[fmt.Sprintf("%s %d %s", spell, circle, bn)] = text
			}
		}
	}
	goldenText(t, "summon-options", got)
}
