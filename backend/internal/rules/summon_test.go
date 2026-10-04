package rules

import (
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// warlockOfTheChain is a level 3 warlock with the Pact of the Chain.
func warlockOfTheChain() Build {
	return Build{
		BaseScores: map[Ability]int{STR: 8, DEX: 14, CON: 14, INT: 10, WIS: 10, CHA: 16},
		Race:       "race:human", Background: "background:acolyte",
		Classes:        []ClassLevel{{Class: "class:warlock", Level: 3}},
		FeatureChoices: []string{"feature:pact-of-the-chain"},
	}
}

func keysOf(forms []SummonForm) []string {
	var out []string
	for _, f := range forms {
		out = append(out, f.Key)
	}
	return out
}

// TestSummonOptions: what each summoning spell lets a cast choose.
func TestSummonOptions(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)

	t.Run("Find Familiar: 15 forms, none attacks; 19 with the Pact of the Chain", func(t *testing.T) {
		got, err := c.SummonOptions("spell:find-familiar", 1, pensantus())
		if err != nil {
			t.Fatal(err)
		}
		if !got.Ritual || got.Concentration || got.CastingTime.Unit != CastHour || got.CastingTime.Amount != 1 {
			t.Errorf("Find Familiar is a ritual that takes 1 hour: %+v", got)
		}
		if len(got.Options) != 1 || got.Options[0].Count != 1 || len(got.Options[0].Forms) != 15 {
			t.Fatalf("options = %+v, want one creature out of 15 forms", got.Options)
		}
		for _, f := range got.Options[0].Forms {
			if f.Attack != SummonAttackNone {
				t.Errorf("%s: a familiar can't attack, got %q", f.Key, f.Attack)
			}
		}
		if !slices.Contains(keysOf(got.Options[0].Forms), "monster:raven") || slices.Contains(keysOf(got.Options[0].Forms), "monster:imp") {
			t.Error("a wizard gets the raven, not the imp")
		}
		chain, err := c.SummonOptions("spell:find-familiar", 1, warlockOfTheChain())
		if err != nil {
			t.Fatal(err)
		}
		forms := chain.Options[0].Forms
		if len(forms) != 19 {
			t.Fatalf("the Pact of the Chain: %d forms, want 19", len(forms))
		}
		for _, f := range forms {
			if f.Attack != SummonAttackReaction {
				t.Errorf("%s: with the Pact of the Chain every form attacks with its reaction, got %q", f.Key, f.Attack)
			}
		}
		for _, k := range []string{"monster:imp", "monster:pseudodragon", "monster:quasit", "monster:sprite", "monster:raven"} {
			if !slices.Contains(keysOf(forms), k) {
				t.Errorf("%s is missing from the chain familiar's forms", k)
			}
		}
		// A warlock who picked another pact gets the 15.
		other := warlockOfTheChain()
		other.FeatureChoices = []string{"feature:pact-of-the-blade"}
		if o, _ := c.SummonOptions("spell:find-familiar", 1, other); len(o.Options[0].Forms) != 15 {
			t.Error("the other pacts do not widen Find Familiar")
		}
	})

	t.Run("Animate Dead: 1 creature, 2 more for each circle above the 3rd", func(t *testing.T) {
		for circle, want := range map[int]int{3: 1, 4: 3, 5: 5, 9: 13} {
			got, err := c.SummonOptions("spell:animate-dead", circle, Build{})
			if err != nil {
				t.Fatal(err)
			}
			o := got.Options[0]
			if o.Count != want || !slices.Equal(keysOf(o.Forms), []string{"monster:skeleton", "monster:zombie"}) || o.Forms[0].Attack != SummonAttackFull {
				t.Errorf("circle %d: %+v, want %d skeletons or zombies", circle, o, want)
			}
			if got.Ritual || got.Concentration || got.CastingTime.Unit != CastMinute {
				t.Errorf("Animate Dead takes 1 minute and no concentration: %+v", got)
			}
		}
	})

	t.Run("Conjure Animals: four options, times 2, 3 and 4 at the 5th, 7th and 9th circle", func(t *testing.T) {
		for circle, times := range map[int]int{3: 1, 4: 1, 5: 2, 6: 2, 7: 3, 8: 3, 9: 4} {
			got, err := c.SummonOptions("spell:conjure-animals", circle, Build{})
			if err != nil {
				t.Fatal(err)
			}
			if !got.Concentration || got.Ritual || got.CastingTime.Unit != CastAction {
				t.Errorf("Conjure Animals is an action with concentration: %+v", got)
			}
			wantCounts, wantCRs := []int{1, 2, 4, 8}, []string{"2", "1", "1/2", "1/4"}
			if len(got.Options) != 4 {
				t.Fatalf("circle %d: %d options", circle, len(got.Options))
			}
			for i, o := range got.Options {
				if o.Count != wantCounts[i]*times || o.MaxCR != wantCRs[i] || o.Type != "beast" || len(o.Forms) != 0 || o.Attack != SummonAttackFull {
					t.Errorf("circle %d, option %d = %+v, want %d beasts of CR %s", circle, i, o, wantCounts[i]*times, wantCRs[i])
				}
			}
		}
	})

	if _, err := c.SummonOptions("spell:fireball", 3, Build{}); !errors.Is(err, ErrNotSummonSpell) {
		t.Errorf("Fireball summons nothing: %v", err)
	}
	if _, err := c.SummonOptions("spell:find-steed", 2, Build{}); !errors.Is(err, ErrNotSummonSpell) {
		t.Errorf("Find Steed is out (question 74): %v", err)
	}
	if _, err := c.SummonOptions("spell:animate-dead", 2, Build{}); !errors.Is(err, ErrSummonCircle) {
		t.Errorf("a 2nd circle slot can't cast a 3rd circle spell: %v", err)
	}
	if _, err := c.SummonOptions("spell:animate-dead", 10, Build{}); !errors.Is(err, ErrSummonCircle) {
		t.Errorf("there is no 10th circle: %v", err)
	}
}

func repeat(key string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = key
	}
	return out
}

// TestCheckSummon: a choice the spell does not allow is refused.
func TestCheckSummon(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	ok := []struct {
		name   string
		spell  string
		circle int
		build  Build
		pick   SummonPick
		attack string
	}{
		{"a raven", "spell:find-familiar", 1, pensantus(), SummonPick{0, []string{"monster:raven"}}, SummonAttackNone},
		{"an imp for the Chain", "spell:find-familiar", 1, warlockOfTheChain(), SummonPick{0, []string{"monster:imp"}}, SummonAttackReaction},
		{"a raven for the Chain attacks with its reaction", "spell:find-familiar", 1, warlockOfTheChain(), SummonPick{0, []string{"monster:raven"}}, SummonAttackReaction},
		{"three undead at the 4th circle", "spell:animate-dead", 4, Build{}, SummonPick{0, []string{"monster:skeleton", "monster:zombie", "monster:zombie"}}, SummonAttackFull},
		{"two dire wolves", "spell:conjure-animals", 3, Build{}, SummonPick{1, repeat("monster:dire-wolf", 2)}, SummonAttackFull},
		{"eight wolves", "spell:conjure-animals", 3, Build{}, SummonPick{3, repeat("monster:wolf", 8)}, SummonAttackFull},
		{"sixteen wolves at the 5th circle", "spell:conjure-animals", 5, Build{}, SummonPick{3, repeat("monster:wolf", 16)}, SummonAttackFull},
		{"a CR 2 beast", "spell:conjure-animals", 3, Build{}, SummonPick{0, []string{"monster:giant-elk"}}, SummonAttackFull},
	}
	for _, tt := range ok {
		got, err := c.CheckSummon(tt.spell, tt.circle, tt.build, tt.pick)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if len(got) != len(tt.pick.Creatures) || got[0].Attack != tt.attack || got[0].Key != tt.pick.Creatures[0] {
			t.Errorf("%s: got %+v", tt.name, got)
		}
	}
	bad := []struct {
		name   string
		spell  string
		circle int
		build  Build
		pick   SummonPick
		want   error
	}{
		{"a wolf from Animate Dead", "spell:animate-dead", 3, Build{}, SummonPick{0, []string{"monster:wolf"}}, ErrSummonCreature},
		{"nine wolves at the 3rd circle", "spell:conjure-animals", 3, Build{}, SummonPick{3, repeat("monster:wolf", 9)}, ErrSummonCount},
		{"seven wolves for eight", "spell:conjure-animals", 3, Build{}, SummonPick{3, repeat("monster:wolf", 7)}, ErrSummonCount},
		{"a CR 2 beast in the eight of CR 1/4", "spell:conjure-animals", 3, Build{}, SummonPick{3, repeat("monster:giant-elk", 8)}, ErrSummonCreature},
		{"a CR 1 beast in the four of CR 1/2", "spell:conjure-animals", 3, Build{}, SummonPick{2, repeat("monster:dire-wolf", 4)}, ErrSummonCreature},
		{"a goblin from Conjure Animals", "spell:conjure-animals", 3, Build{}, SummonPick{0, []string{"monster:goblin"}}, ErrSummonCreature},
		{"a swarm is not a beast", "spell:conjure-animals", 3, Build{}, SummonPick{3, repeat("monster:swarm-of-rats", 8)}, ErrSummonCreature},
		{"an option that does not exist", "spell:conjure-animals", 3, Build{}, SummonPick{4, []string{"monster:wolf"}}, ErrSummonOption},
		{"a negative option", "spell:conjure-animals", 3, Build{}, SummonPick{-1, nil}, ErrSummonOption},
		{"an imp without the Chain", "spell:find-familiar", 1, pensantus(), SummonPick{0, []string{"monster:imp"}}, ErrSummonCreature},
		{"two familiars", "spell:find-familiar", 1, pensantus(), SummonPick{0, []string{"monster:raven", "monster:cat"}}, ErrSummonCount},
		{"no creature", "spell:find-familiar", 1, pensantus(), SummonPick{0, nil}, ErrSummonCount},
		{"a slot below the spell", "spell:animate-dead", 2, Build{}, SummonPick{0, []string{"monster:zombie"}}, ErrSummonCircle},
		{"a spell that doesn't summon", "spell:fireball", 3, Build{}, SummonPick{0, nil}, ErrNotSummonSpell},
	}
	for _, tt := range bad {
		if _, err := c.CheckSummon(tt.spell, tt.circle, tt.build, tt.pick); !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
	}
}

// TestSummonLoaderRefuses: the closed "summon" kind is checked at load.
func TestSummonLoaderRefuses(t *testing.T) {
	t.Parallel()
	c := &content{
		spells:   map[string]*srd51.Spell{"spell:find-familiar": {Key: "spell:find-familiar", Level: 1}, "spell:conjure-animals": {Key: "spell:conjure-animals", Level: 3}},
		monsters: map[string]*srd51.Monster{"monster:raven": {}, "monster:imp": {}},
		features: map[string]*srd51.Feature{"feature:pact-of-the-chain": {}},
		named:    map[string]*srd51.Named{},
	}
	load := func(body string) error {
		return c.loadSpellEffects(fstest.MapFS{"effects/spells.json": {Data: []byte(body)}})
	}
	if err := load(`{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"count":1,"attack":"none",
		"unlocks":[{"feature":"feature:pact-of-the-chain","creatures":["monster:imp"],"attack":"reaction"}]},
		"spell:conjure-animals":{"kind":"summon","type":"beast","options":[{"count":1,"max_cr":"2"}],"multiplier_at_level":{"5":2},"attack":"full"}}}`); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	bad := map[string]string{
		"an unknown creature":           `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:dodo"],"count":1,"attack":"none"}}}`,
		"an unknown field":              `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"count":1,"attack":"none","power":9}}}`,
		"a bad challenge rating":        `{"spells":{"spell:conjure-animals":{"kind":"summon","type":"beast","options":[{"count":1,"max_cr":"1/3"}],"attack":"full"}}}`,
		"no attack mode":                `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"count":1}}}`,
		"an unknown attack mode":        `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"count":1,"attack":"always"}}}`,
		"creatures and options":         `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"count":1,"attack":"none","options":[{"count":1,"max_cr":"1"}]}}}`,
		"neither creatures nor options": `{"spells":{"spell:find-familiar":{"kind":"summon","attack":"none"}}}`,
		"a creature listed twice":       `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven","monster:raven"],"count":1,"attack":"none"}}}`,
		"an unlock for a non-feature":   `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"count":1,"attack":"none","unlocks":[{"feature":"feature:nope","creatures":["monster:imp"],"attack":"reaction"}]}}}`,
		"a count of 0":                  `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"attack":"none"}}}`,
		"an unknown creature type":      `{"spells":{"spell:conjure-animals":{"kind":"summon","type":"cat","options":[{"count":1,"max_cr":"1"}],"attack":"full"}}}`,
		"a multiplier below the spell":  `{"spells":{"spell:conjure-animals":{"kind":"summon","type":"beast","options":[{"count":1,"max_cr":"1"}],"multiplier_at_level":{"3":2},"attack":"full"}}}`,
		"summon fields on another kind": `{"spells":{"spell:find-familiar":{"kind":"zero_hp_target","creatures":["monster:raven"]}}}`,
		"hp fields on a summon":         `{"spells":{"spell:find-familiar":{"kind":"summon","creatures":["monster:raven"],"count":1,"attack":"none","dice":"1d4"}}}`,
	}
	for name, body := range bad {
		if err := load(body); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
}
