package rules

import (
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
	}
	for _, tt := range tests {
		got, ok := c.SpellEffect(tt.key, tt.level)
		if !ok || got.Kind != tt.want.Kind || got.Dice != tt.want.Dice || got.Condition != tt.want.Condition || got.Threshold != tt.want.Threshold ||
			got.Dies != tt.want.Dies || got.Heal != tt.want.Heal || !slices.Equal(got.Ends, tt.want.Ends) {
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
	bad := map[string]string{
		"an unknown kind":          `{"spells":{"spell:sleep":{"kind":"mind_blast"}}}`,
		"an unknown field":         `{"spells":{"spell:sleep":{"kind":"zero_hp_target","power":9}}}`,
		"a spell the SRD lacks":    `{"spells":{"spell:toll-the-dead":{"kind":"zero_hp_target"}}}`,
		"a pool of other dice":     `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d8","dice_per_level":"2d10","condition":"condition:unconscious"}}}`,
		"a pool with no condition": `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d8","dice_per_level":"2d8"}}}`,
		"a condition that is not":  `{"spells":{"spell:sleep":{"kind":"hp_pool","dice":"5d8","dice_per_level":"2d8","condition":"spell:x"}}}`,
		"a threshold with both":    `{"spells":{"spell:sleep":{"kind":"hp_threshold","threshold":100,"dies":true,"condition":"condition:blinded"}}}`,
		"a threshold with neither": `{"spells":{"spell:sleep":{"kind":"hp_threshold","threshold":100}}}`,
		"a threshold of 0":         `{"spells":{"spell:sleep":{"kind":"hp_threshold","dies":true}}}`,
		"a heal that ends nothing": `{"spells":{"spell:sleep":{"kind":"flat_heal","amount":70}}}`,
		"a zero target with dice":  `{"spells":{"spell:sleep":{"kind":"zero_hp_target","dice":"1d4"}}}`,
	}
	for name, body := range bad {
		if err := load(body); err == nil {
			t.Errorf("%s: the loader accepted it", name)
		}
	}
}
