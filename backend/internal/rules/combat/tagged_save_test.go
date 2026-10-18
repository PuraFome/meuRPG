package combat

import (
	"slices"
	"testing"
)

// The racial advantages on saving throws that hold only against one source (SRD 5.1, Dwarf, Elf, Gnome and Halfling)
// apply when the save says what it is against, and are listed with the trait that gives them.
func TestSaveModeTaggedAdvantages(t *testing.T) {
	brave := TaggedSave{Source: "trait:brave", Against: AgainstFrightened}
	fey := TaggedSave{Source: "trait:fey-ancestry", Against: AgainstCharmed}
	cunning := TaggedSave{Source: "trait:gnome-cunning", Against: AgainstMagic, Abilities: []string{"int", "wis", "cha"}}
	dwarf := TaggedSave{Source: "trait:dwarven-resilience", Against: AgainstPoison}
	tests := []struct {
		name  string
		scene SaveScene
		want  []string
	}{
		{"brave against a fear", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{brave}}, Ability: "wis", Against: []string{AgainstFrightened}}, []string{"trait:brave"}},
		{"brave against something else", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{brave}}, Ability: "wis", Against: []string{AgainstCharmed}}, nil},
		{"brave when the source is not known", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{brave}}, Ability: "wis"}, nil},
		{"fey ancestry against a charm", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{fey}}, Ability: "wis", Against: []string{AgainstCharmed}}, []string{"trait:fey-ancestry"}},
		{"gnome cunning on a wisdom save against magic", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{cunning}}, Ability: "wis", Against: []string{AgainstMagic}}, []string{"trait:gnome-cunning"}},
		{"gnome cunning is not for a dexterity save", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{cunning}}, Ability: "dex", Against: []string{AgainstMagic}}, nil},
		{"gnome cunning is not for a non-magical source", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{cunning}}, Ability: "wis", Against: []string{AgainstPoison}}, nil},
		{"dwarven resilience against poison", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{dwarf}}, Ability: "con", Against: []string{AgainstPoison}}, []string{"trait:dwarven-resilience"}},
		{"a source with two tags uses both traits", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{cunning, brave}}, Ability: "wis", Against: []string{AgainstMagic, AgainstFrightened}}, []string{"trait:brave", "trait:gnome-cunning"}},
		{"an ability check takes none of them", SaveScene{Creature: Creature{TaggedSaves: []TaggedSave{brave}}, Ability: "wis", Check: true, Against: []string{AgainstFrightened}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, s := range SaveMode(tt.scene) {
				if s.Kind != SourceTaggedSave || s.Effect != ModeAdvantage {
					t.Fatalf("source %+v, want an advantage of Kind %q", s, SourceTaggedSave)
				}
				got = append(got, s.Condition)
			}
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("sources = %v, want %v", got, tt.want)
			}
		})
	}
}
