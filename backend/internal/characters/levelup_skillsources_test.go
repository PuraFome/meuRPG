package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A skill the sheet has from its race or its background is held, whatever source the level offers it from: the option says so.
func TestALevelSkillSourceMarksTheSkillsFromRaceAndBackgroundAsHeld(t *testing.T) {
	t.Parallel()
	c := loadSRD(t)
	b := wizardBuild("", false)
	b.Race = "race:elf"
	b.SkillProficiencies = []string{"skill:arcana"}
	held := map[string]bool{}
	for _, s := range rules.Derive(b, c).Skills {
		held[s.Key] = s.Proficiency >= rules.ProficiencyFull
	}
	if !held["skill:perception"] || !held["skill:insight"] {
		t.Fatalf("the fixture's elf and acolyte do not hold perception and insight: %v", held)
	}
	got := skillSourcesToProto(c, b, []rules.LevelSkillSource{{
		Owner: "class:wizard", Count: 1,
		From: []string{"skill:perception", "skill:insight", "skill:arcana", "skill:history"},
	}})
	want := map[string]bool{"skill:perception": true, "skill:insight": true, "skill:arcana": true, "skill:history": false}
	if len(got) != 1 || len(got[0].GetFrom()) != len(want) {
		t.Fatalf("sources = %v", got)
	}
	for _, o := range got[0].GetFrom() {
		if o.GetAlreadyHave() != want[o.GetKey()] {
			t.Errorf("%s already_have = %v, want %v", o.GetKey(), o.GetAlreadyHave(), want[o.GetKey()])
		}
	}
}
