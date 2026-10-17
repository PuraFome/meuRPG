package characters

import (
	"reflect"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The hints of the racial advantages on saving throws become the combat traits that apply when a save says what it is
// against; the other hints (a bonus, a note, an advantage on a skill, a condition the combat does not know) do not.
func TestTaggedSavesOfTheHints(t *testing.T) {
	hints := []rules.Hint{
		{Source: "trait:brave", Mode: "advantage", Target: "save.all", Targets: []string{"save.all"}, Tags: []string{"against:frightened"}},
		{Source: "trait:gnome-cunning", Mode: "advantage", Target: "save.int", Targets: []string{"save.int", "save.wis", "save.cha"}, Tags: []string{"against:magic"}},
		{Source: "trait:dwarven-resilience", Mode: "advantage", Targets: []string{"save.all"}, Tags: []string{"against:poison"}},
		{Source: "trait:fey-ancestry", Mode: "advantage", Targets: []string{"save.all"}, Tags: []string{"against:charmed"}},
		{Source: "feature:x", Mode: "advantage", Targets: []string{"save.all"}, Tags: []string{"against:something-else"}},
		{Source: "trait:lucky", Mode: "note"},
		{Source: "feature:y", Mode: "advantage", Targets: []string{"skill:stealth"}, Tags: []string{"against:magic"}},
		{Source: "feature:z", Mode: "advantage", Targets: []string{"save.all"}},
	}
	want := []link.TaggedSave{
		{Source: "trait:brave", Against: "frightened"},
		{Source: "trait:gnome-cunning", Against: "magic", Abilities: []string{"int", "wis", "cha"}},
		{Source: "trait:dwarven-resilience", Against: "poison"},
		{Source: "trait:fey-ancestry", Against: "charmed"},
	}
	if got := taggedSavesOf(hints); !reflect.DeepEqual(got, want) {
		t.Errorf("taggedSavesOf = %+v, want %+v", got, want)
	}
}
