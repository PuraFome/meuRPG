package rules

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// TestReplacedSkillsListsASkillOnceAndNeverOneAKeptTraitGives: a replaced trait's skill stays
// out of replacedSkills when a kept trait of the race gives it too, and two replaced traits
// that give the same skill list it once.
func TestReplacedSkillsListsASkillOnceAndNeverOneAKeptTraitGives(t *testing.T) {
	t.Parallel()
	trait := func(key string, skills ...string) *srd51.Trait {
		return &srd51.Trait{Key: key, Proficiencies: skills}
	}
	c := &content{
		races: map[string]*srd51.Race{
			"race:r":  {Key: "race:r", Traits: []string{"trait:a", "trait:kept", "trait:c"}},
			"race:r2": {Key: "race:r2", Traits: []string{"trait:a", "trait:e", "trait:kept"}},
		},
		traits: map[string]*srd51.Trait{
			"trait:a":    trait("trait:a", "proficiency:skill-perception", "proficiency:skill-arcana"),
			"trait:e":    trait("trait:e", "proficiency:skill-arcana"),
			"trait:kept": trait("trait:kept", "proficiency:skill-perception"),
			"trait:c":    trait("trait:c"),
		},
	}
	cases := []struct {
		name     string
		race     string
		replaces []string
		want     []string
	}{
		{"a kept trait gives the same skill", "race:r", []string{"trait:a"}, []string{"skill:arcana"}},
		{"two replaced traits give the same skill", "race:r2", []string{"trait:a", "trait:e"}, []string{"skill:arcana"}},
		{"a trait with no skill", "race:r", []string{"trait:c"}, nil},
		{"nothing replaced", "race:r", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := c.replacedSkills(&srd51.Subrace{Race: tc.race, ReplacesTraitKeys: tc.replaces})
			if !slices.Equal(got, tc.want) {
				t.Errorf("replacedSkills() = %v, want %v", got, tc.want)
			}
		})
	}
}
