package characters

import (
	"bytes"
	"encoding/json"
	goflag "flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

var updateSpellcastingBlocks = goflag.Bool("update-spellcasting-blocks", false, "write testdata/spellcasting_blocks.golden.json again")

// TestSpellcastingBlocksOfEveryCreature freezes the note each Spellcasting trait of
// the SRD's creatures writes (the header and each list line), keyed by creature
// and trait.
func TestSpellcastingBlocksOfEveryCreature(t *testing.T) {
	t.Parallel()
	content := loadRules(t)
	got := map[string]string{}
	for _, e := range creatureEntries(t, content) {
		c, _ := content.CreatureByKey(e.Key)
		for _, tr := range c.Traits {
			if strings.Contains(tr.Name, "pellcasting") {
				got[e.Key+" "+tr.Name] = spellcastingBlock(content, tr)
			}
		}
	}
	b, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	path := filepath.Join("testdata", "spellcasting_blocks.golden.json")
	if *updateSpellcastingBlocks {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden (run with -update-spellcasting-blocks): %v", err)
	}
	if !bytes.Equal(want, b) {
		t.Errorf("the spellcasting blocks changed; diff testdata/spellcasting_blocks.golden.json against:\n%s", b)
	}
}

// TestSpellcastingBlockShapes: the shapes the SRD does not have today.
func TestSpellcastingBlockShapes(t *testing.T) {
	t.Parallel()
	content := loadRules(t)
	for _, tt := range []struct {
		name string
		t    rules.CreatureAbility
		want string
	}{
		{"nothing it reads", rules.CreatureAbility{Name: "Spellcasting", Text: "It casts."}, ""},
		{
			"a label it does not know is left out",
			rules.CreatureAbility{Name: "Spellcasting", Text: "Its spell save DC 12.\nSometimes: fly, light\nAt will: light"},
			"Conjuração, CD 12.\nÀ vontade: Luz.",
		},
		{
			"only a header",
			rules.CreatureAbility{Name: "Innate Spellcasting", Text: "Its innate spellcasting ability is Charisma."},
			"Conjuração inata, habilidade Carisma.\n",
		},
		{
			"a single slot and a per-day line",
			rules.CreatureAbility{Name: "Spellcasting", Text: "A 1st-level spellcaster. +4 to hit with spell attacks.\n1st level (1 slot): shield\n2/day: light"},
			"Conjuração de 1º nível, ataque de magia +4.\n1º nível (1 espaço): Escudo Arcano.\n2/dia: Luz.",
		},
	} {
		if got := spellcastingBlock(content, tt.t); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, got, tt.want)
		}
	}
}
