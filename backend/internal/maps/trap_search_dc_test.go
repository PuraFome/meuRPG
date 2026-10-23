package maps

import (
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// Finding fix/trap-search-skill-key: SearchTraps takes the search kind ("perception",
// "investigation") for the two skills every trap can be found with and the "skill:" key only
// for another skill; the spec never lists the first two (cleanAlsoFind refuses their
// "skill:" keys), so a Perception spec is its notice DC and is met by the kind "perception".
// This guards that the two sides stay reconciled.
func TestSearchDCPerceptionAndInvestigationUseTheSpecDCs(t *testing.T) {
	spec := &mapsv1.TrapSpec{NoticeDc: 12, FindDc: 15, AlsoFindSkillKeys: []string{"skill:arcana"}}
	cases := []struct {
		skill string
		dc    int
		ok    bool
	}{
		{"perception", 12, true},
		{"investigation", 15, true},
		{"skill:arcana", 15, true},
		{"skill:history", 0, false},
		{skillPerception, 0, false}, // the check key is not a search kind: the play module never sends it
	}
	for _, c := range cases {
		dc, ok := searchDC(c.skill, spec)
		if dc != c.dc || ok != c.ok {
			t.Errorf("searchDC(%q) = %d, %v; want %d, %v", c.skill, dc, ok, c.dc, c.ok)
		}
	}
	s := &Service{rules: namedRules{}}
	for _, k := range []string{skillPerception, skillInvestigation} {
		if _, err := s.cleanAlsoFind([]string{k}); err == nil {
			t.Errorf("cleanAlsoFind accepted %q", k)
		}
	}
}
