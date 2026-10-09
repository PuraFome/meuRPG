package rules

import (
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// mutatedChoices is effects/choices.json as the content ships it, changed by edit.
func mutatedChoices(t *testing.T, edit func(doc map[string]any)) fs.FS {
	t.Helper()
	raw, err := fs.ReadFile(srd51.Files, "effects/choices.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{"effects/choices.json": {Data: out}}
}

// rows is a list in a decoded document.
func rows(doc map[string]any, key string) []any { return doc[key].([]any) }

// TestChoiceDataIsClosed: the loader of effects/choices.json refuses a row without its
// SRD source, a key the content does not have, a shape that is not one, a missing
// invocation, and a field it does not know.
func TestChoiceDataIsClosed(t *testing.T) {
	t.Parallel()
	loaded := loadForTest(t).c
	row := func(doc map[string]any, key string, i int) map[string]any { return rows(doc, key)[i].(map[string]any) }
	edits := map[string]func(doc map[string]any){
		"an ancestry without a source":      func(d map[string]any) { delete(row(d, "draconic_ancestry", 0), "source") },
		"an ancestry of an unknown trait":   func(d map[string]any) { row(d, "draconic_ancestry", 0)["trait"] = "trait:nope" },
		"an ancestry of an unknown feature": func(d map[string]any) { row(d, "draconic_ancestry", 0)["dragon_ancestor"] = "feature:nope" },
		"an ancestry that is not a damage":  func(d map[string]any) { row(d, "draconic_ancestry", 0)["damage_type"] = "condition:deafened" },
		"an ancestry with a shape of its own": func(d map[string]any) {
			row(d, "draconic_ancestry", 0)["shape"] = "sphere"
		},
		"a line without a width":                func(d map[string]any) { delete(row(d, "draconic_ancestry", 0), "width_ft") },
		"a cone with a width":                   func(d map[string]any) { row(d, "draconic_ancestry", 5)["width_ft"] = 5 },
		"an ancestry saving with a non-ability": func(d map[string]any) { row(d, "draconic_ancestry", 0)["save"] = "luck" },
		"an ancestry listed twice": func(d map[string]any) {
			d["draconic_ancestry"] = append(rows(d, "draconic_ancestry"), row(d, "draconic_ancestry", 0))
		},
		"an invocation without a source":       func(d map[string]any) { delete(row(d, "invocation_prerequisites", 0), "source") },
		"an invocation that is no invocation":  func(d map[string]any) { row(d, "invocation_prerequisites", 0)["invocation"] = "feature:second-wind" },
		"an invocation asking a leveled spell": func(d map[string]any) { row(d, "invocation_prerequisites", 0)["spell"] = "spell:fireball" },
		"an invocation asking a feature that is no boon": func(d map[string]any) {
			row(d, "invocation_prerequisites", 0)["feature"] = "feature:second-wind"
		},
		"an invocation asking a level beyond 20": func(d map[string]any) { row(d, "invocation_prerequisites", 0)["level"] = 21 },
		"an invocation without a row":            func(d map[string]any) { d["invocation_prerequisites"] = rows(d, "invocation_prerequisites")[1:] },
		"an invocation listed twice": func(d map[string]any) {
			d["invocation_prerequisites"] = append(rows(d, "invocation_prerequisites"), row(d, "invocation_prerequisites", 0))
		},
		"a favored enemy that is no creature type": func(d map[string]any) {
			d["favored_enemy"].(map[string]any)["types"] = []any{"beast"}
		},
		"favored enemies without a source": func(d map[string]any) { delete(d["favored_enemy"].(map[string]any), "source") },
		"a terrain that is not a terrain": func(d map[string]any) {
			d["natural_explorer"].(map[string]any)["terrains"] = []any{"forest"}
		},
		"a terrain listed twice": func(d map[string]any) {
			d["natural_explorer"].(map[string]any)["terrains"] = []any{"terrain:forest", "terrain:forest"}
		},
		"a resistance of an unknown trait": func(d map[string]any) { row(d, "racial_resistances", 0)["trait"] = "trait:nope" },
		"a resistance without a source":    func(d map[string]any) { delete(row(d, "racial_resistances", 0), "source") },
		"breath dice that are not dice": func(d map[string]any) {
			d["breath_weapon"].(map[string]any)["dice"].([]any)[0].(map[string]any)["dice"] = "lots"
		},
		"breath dice that do not rise": func(d map[string]any) {
			d["breath_weapon"].(map[string]any)["dice"].([]any)[1].(map[string]any)["level"] = 1
		},
		"a summary of a feature that is not": func(d map[string]any) {
			d["option_summaries"].(map[string]any)["feature:nope"] = "Nada."
		},
		"an option without its summary": func(d map[string]any) {
			delete(d["option_summaries"].(map[string]any), "feature:fighter-fighting-style-archery")
		},
		"a field the loader does not know": func(d map[string]any) { d["surprise"] = 1 },
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := *loaded
			if err := c.loadChoiceData(mutatedChoices(t, edit)); err == nil {
				t.Error("the loader accepted it")
			}
		})
	}
	c := *loaded
	if err := c.loadChoiceData(mutatedChoices(t, func(map[string]any) {})); err != nil {
		t.Errorf("the file as shipped: %v", err)
	}
}
