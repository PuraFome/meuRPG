package rules

import (
	"fmt"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

func TestSpellEffectsOfTheSRD(t *testing.T) {
	t.Parallel()
	c, err := LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		key   string
		level int
		want  SpellEffect
	}{
		{"spell:sleep", 1, SpellEffect{Kind: SpellKindHPPool, Dice: DiceFormula{Count: 5, Sides: 8}, Condition: "condition:unconscious"}},
		{"spell:sleep", 3, SpellEffect{Kind: SpellKindHPPool, Dice: DiceFormula{Count: 9, Sides: 8}, Condition: "condition:unconscious"}},
		{"spell:sleep", 0, SpellEffect{Kind: SpellKindHPPool, Dice: DiceFormula{Count: 5, Sides: 8}, Condition: "condition:unconscious"}},
		{"spell:color-spray", 2, SpellEffect{Kind: SpellKindHPPool, Dice: DiceFormula{Count: 8, Sides: 10}, Condition: "condition:blinded"}},
		{"spell:power-word-stun", 8, SpellEffect{Kind: SpellKindHPThreshold, Threshold: 150, Condition: "condition:stunned"}},
		{"spell:power-word-kill", 9, SpellEffect{Kind: SpellKindHPThreshold, Threshold: 100, Dies: true}},
		{"spell:spare-the-dying", 0, SpellEffect{Kind: SpellKindZeroHP}},
		{"spell:heal", 6, SpellEffect{Kind: SpellKindFlatHeal, Heal: 70, Ends: []string{"condition:blinded", "condition:deafened"}}},
		{"spell:heal", 9, SpellEffect{Kind: SpellKindFlatHeal, Heal: 100, Ends: []string{"condition:blinded", "condition:deafened"}}},
		{"spell:false-life", 1, SpellEffect{Kind: SpellKindTempHP, Dice: DiceFormula{Count: 1, Sides: 4}, Amount: 4}},
		{"spell:false-life", 3, SpellEffect{Kind: SpellKindTempHP, Dice: DiceFormula{Count: 1, Sides: 4}, Amount: 14}},
		{"spell:aid", 2, SpellEffect{Kind: SpellKindMaxHP, Amount: 5}},
		{"spell:aid", 5, SpellEffect{Kind: SpellKindMaxHP, Amount: 20}},
	}
	for _, tt := range tests {
		got, ok := c.SpellEffect(tt.key, tt.level)
		if !ok || got.Kind != tt.want.Kind || got.Dice != tt.want.Dice || got.Condition != tt.want.Condition || got.Threshold != tt.want.Threshold ||
			got.Dies != tt.want.Dies || got.Heal != tt.want.Heal || got.Amount != tt.want.Amount || !slices.Equal(got.Ends, tt.want.Ends) {
			t.Errorf("SpellEffect(%s, %d) = %+v, %v; want %+v", tt.key, tt.level, got, ok, tt.want)
		}
	}
	if _, ok := c.SpellEffect("spell:fireball", 3); ok {
		t.Error("Fireball reads no hit points")
	}
	// Toll the Dead is not in the SRD 5.1 and never enters (the repo is public).
	if _, ok := c.SpellDetails("spell:toll-the-dead"); ok {
		t.Error("Toll the Dead is not an SRD spell")
	}
	// The caller's copy of Ends is its own.
	e, _ := c.SpellEffect("spell:heal", 6)
	e.Ends[0] = "x"
	if e2, _ := c.SpellEffect("spell:heal", 6); e2.Ends[0] != "condition:blinded" {
		t.Error("SpellEffect shares its Ends slice")
	}
}

// The loader refuses what the closed kinds do not allow.
func TestLoadSpellEffectsRefuses(t *testing.T) {
	t.Parallel()
	c := &content{
		spells: map[string]*srd51.Spell{"spell:sleep": {Key: "spell:sleep", Level: 1}},
		named:  map[string]*srd51.Named{"condition:unconscious": {}, "condition:blinded": {}, "spell:x": {}},
	}
	load := func(body string) error {
		return c.loadSpellEffects(fstest.MapFS{"effects/spells.json": {Data: []byte(body)}})
	}
	good := `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d8","dice_per_level":"2d8","condition":"condition:unconscious"}}}`
	if err := load(good); err != nil {
		t.Fatalf("a good file: %v", err)
	}
	for _, sides := range tableDiceSides { // every die at a table is allowed
		body := fmt.Sprintf(`{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d%d","dice_per_level":"2d%[1]d","condition":"condition:unconscious"}}}`, sides)
		if err := load(body); err != nil {
			t.Errorf("a pool of d%d: %v", sides, err)
		}
	}
	bad := map[string]string{
		"an unknown kind":             `{"spells":{"spell:sleep":{"kind":"mind_blast"}}}`,
		"ignores_cover with dice":     `{"spells":{"spell:sleep":{"kind":"ignores_cover","dice":"5d8"}}}`,
		"an unknown field":            `{"spells":{"spell:sleep":{"kind":"zero_hp_target","power":9}}}`,
		"a spell the SRD lacks":       `{"spells":{"spell:toll-the-dead":{"kind":"zero_hp_target"}}}`,
		"a pool of other dice":        `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d8","dice_per_level":"2d10","condition":"condition:unconscious"}}}`,
		"a pool of a die that is not": `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d7","dice_per_level":"2d7","condition":"condition:unconscious"}}}`,
		"a pool of 1000 faces":        `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d1000","dice_per_level":"2d1000","condition":"condition:unconscious"}}}`,
		"a pool of too many dice":     `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"31d8","dice_per_level":"2d8","condition":"condition:unconscious"}}}`,
		"a pool with no condition":    `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d8","dice_per_level":"2d8"}}}`,
		"a condition that is not":     `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d8","dice_per_level":"2d8","condition":"spell:x"}}}`,
		"a threshold with both":       `{"spells":{"spell:sleep":{"kind":"hp_threshold","threshold":100,"dies":true,"condition":"condition:blinded"}}}`,
		"a threshold with neither":    `{"spells":{"spell:sleep":{"kind":"hp_threshold","threshold":100}}}`,
		"a threshold of 0":            `{"spells":{"spell:sleep":{"kind":"hp_threshold","dies":true}}}`,
		"a heal that ends nothing":    `{"spells":{"spell:sleep":{"kind":"flat_heal","amount":70}}}`,
		"a temp_hp with no dice":      `{"spells":{"spell:sleep":{"kind":"temp_hp","amount":4,"amount_per_level":5}}}`,
		"a temp_hp that never grows":  `{"spells":{"spell:sleep":{"kind":"temp_hp","dice":"1d4","amount":4}}}`,
		"a temp_hp with a condition":  `{"spells":{"spell:sleep":{"kind":"temp_hp","dice":"1d4","amount":4,"amount_per_level":5,"condition":"condition:blinded"}}}`,
		"a max_hp of nothing":         `{"spells":{"spell:sleep":{"kind":"max_hp","amount_per_level":5}}}`,
		"a max_hp with dice":          `{"spells":{"spell:sleep":{"kind":"max_hp","amount":5,"dice":"1d4"}}}`,
		"a zero target with dice":     `{"spells":{"spell:sleep":{"kind":"zero_hp_target","dice":"1d4"}}}`,
		"a revive with no source":     `{"spells":{"spell:sleep":{"kind":"revive","hit_points":1,"window_rounds":10}}}`,
		"a revive of 0 hit points":    `{"spells":{"spell:sleep":{"kind":"revive","window_rounds":10,"source":"SRD"}}}`,
		"a revive with no window":     `{"spells":{"spell:sleep":{"kind":"revive","hit_points":1,"source":"SRD"}}}`,
		"a revive with dice":          `{"spells":{"spell:sleep":{"kind":"revive","hit_points":1,"window_rounds":10,"source":"SRD","dice":"1d4"}}}`,
		"a window on another kind":    `{"spells":{"spell:sleep":{"kind":"zero_hp_target","window_rounds":10}}}`,
		"hit points on a summon":      `{"spells":{"spell:sleep":{"kind":"summon","hit_points":1}}}`,
	}
	for name, body := range bad {
		if err := load(body); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
}

// Chama Sagrada's save gets no benefit from cover (SRD 5.1): the content says so,
// in its data, and the spell is no hit point spell.
func TestSacredFlameIgnoresCover(t *testing.T) {
	t.Parallel()
	c, err := LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD() error = %v", err)
	}
	if !c.IgnoresCover("spell:sacred-flame") || c.IgnoresCover("spell:burning-hands") {
		t.Errorf("IgnoresCover: sacred flame %v, burning hands %v; want true, false", c.IgnoresCover("spell:sacred-flame"), c.IgnoresCover("spell:burning-hands"))
	}
	if _, ok := c.SpellEffect("spell:sacred-flame", 0); ok {
		t.Errorf("Sacred Flame reads no hit points")
	}
}

// SRD 5.1, Revivify: 3rd level, a creature that died in the last minute returns with 1 hit point.
// A minute is 10 rounds (SRD 5.1, "The Order of Combat": a round is about 6 seconds). The cleric
// and the paladin have it in their lists (SRD 5.1, the class spell lists), and the Life domain
// always has it prepared from level 5.
func TestRevivifyIsTheSRDsAndOnTheClericAndPaladinLists(t *testing.T) {
	t.Parallel()
	c, err := LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	r, ok := c.Revive("spell:revivify")
	if !ok || r.HitPoints != 1 || r.WindowRounds != 10 {
		t.Fatalf("Revive(revivify) = %+v, %v; want 1 hit point and a window of 10 rounds", r, ok)
	}
	if _, ok := c.Revive("spell:raise-dead"); ok {
		t.Errorf("Revive(raise-dead) = true: only Revivify is the app's revival")
	}
	if _, ok := c.SpellEffect("spell:revivify", 3); ok {
		t.Errorf("SpellEffect(revivify) = true: it reads no hit points")
	}
	d, ok := c.SpellDetails("spell:revivify")
	if !ok || d.Spell.Level != 3 || d.CastingTime.Unit != CastAction || d.CastingTime.Amount != 1 || d.Range.Kind != RangeTouch || !d.Components.Material {
		t.Fatalf("the spell = %+v, %v; want 3rd level, 1 action, touch, with a material", d, ok)
	}
	for _, class := range []string{"class:cleric", "class:paladin"} {
		got := c.ListSpells(SpellFilter{Class: class, Levels: []int{3}, Query: "revivificar"})
		if len(got) != 1 || got[0].Key != "spell:revivify" {
			t.Errorf("the %s list at the 3rd level for \"revivificar\" = %v, want Revivificar", class, spellKeys(got))
		}
	}
	for _, class := range []string{"class:wizard", "class:bard", "class:druid", "class:sorcerer", "class:warlock", "class:ranger"} {
		if got := c.ListSpells(SpellFilter{Class: class, Query: "revivificar"}); len(got) != 0 {
			t.Errorf("the %s list has Revivificar: %v; the SRD gives it to the cleric and the paladin only", class, spellKeys(got))
		}
	}
	if name := c.NamePT("spell:revivify"); name != "Revivificar" {
		t.Errorf("NamePT(revivify) = %q, want Revivificar", name)
	}
}
