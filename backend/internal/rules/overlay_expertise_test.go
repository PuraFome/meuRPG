package rules

import (
	"errors"
	"testing"
)

// A proficiency effect of level "expertise" doubles the proficiency bonus of the
// skill it names, proficient or not. With OnlyProficient it doubles it only where
// the character already has the proficiency from any other source when the sheet
// is worked out, so the effect never gives the proficiency itself.

// expertiseContent is a class whose level 1 has the given effects.
func expertiseContent(t *testing.T, effects ...Effect) (*Content, string) {
	t.Helper()
	base := loadForTest(t)
	tc := genClass(base, genKinds[0])
	tc.Levels[0].Features = []TableFeature{tf("pericia-dobrada", "Perícia dobrada", effects...)}
	return withOverlayOn(t, base, Overlay{Revision: 1, Classes: []TableClass{tc}}), tc.Key
}

func skillLevel(d Derived, key string) ProficiencyLevel {
	for _, s := range d.Skills {
		if s.Key == key {
			return s.Proficiency
		}
	}
	return -1
}

func expertise(skill string, only bool) Effect {
	return Effect{Type: "proficiency", Proficiency: skill, Level: "expertise", OnlyProficient: only}
}

func TestExpertiseOnlyWhereProficient(t *testing.T) {
	t.Parallel()
	c, class := expertiseContent(t, expertise("skill:arcana", true), expertise("skill:history", true), expertise("skill:religion", true))
	b := standard(class, 1)
	// Arcana is the character's own pick; Religion comes from the background; History is not theirs.
	b.SkillProficiencies = []string{"skill:arcana"}
	d := Derive(b, c)
	for skill, want := range map[string]ProficiencyLevel{
		"skill:arcana":   ProficiencyExpertise,
		"skill:religion": ProficiencyExpertise,
		"skill:history":  ProficiencyNone,
		"skill:insight":  ProficiencyFull, // the background's, no effect names it
		"skill:stealth":  ProficiencyNone,
	} {
		if got := skillLevel(d, skill); got != want {
			t.Errorf("%s = %v, want %v", skill, got, want)
		}
	}
	// The doubled bonus is the character's: twice the proficiency bonus, plus the modifier.
	mod := 0
	for _, a := range d.Abilities {
		if a.Ability == INT {
			mod = a.Modifier
		}
	}
	for _, s := range d.Skills {
		if s.Key == "skill:arcana" && s.Bonus != mod+2*d.ProficiencyBonus {
			t.Errorf("arcana bonus = %d, want %d", s.Bonus, mod+2*d.ProficiencyBonus)
		}
	}
}

// TestExpertiseWithoutTheFlagGivesTheProficiency: an effect of level expertise
// without the flag keeps working as before.
func TestExpertiseWithoutTheFlagGivesTheProficiency(t *testing.T) {
	t.Parallel()
	c, class := expertiseContent(t, expertise("skill:history", false))
	d := Derive(standard(class, 1), c)
	if got := skillLevel(d, "skill:history"); got != ProficiencyExpertise {
		t.Errorf("history = %v, want expertise whether or not the character was proficient", got)
	}
}

// TestExpertiseCountsProficiencyFromAnySource: the proficiency may come from an
// effect that comes later in the list, from a feature or from a race; the effect
// itself does not count, and two of them do not lift each other.
func TestExpertiseCountsProficiencyFromAnySource(t *testing.T) {
	t.Parallel()
	c, class := expertiseContent(t,
		expertise("skill:history", true),                                 // proficient through the next effect
		Effect{Type: "proficiency", Proficiency: "skill:history"},        // a proficiency of the table's
		expertise("skill:nature", true), expertise("skill:nature", true), // nobody gives Nature
	)
	d := Derive(standard(class, 1), c)
	if got := skillLevel(d, "skill:history"); got != ProficiencyExpertise {
		t.Errorf("history = %v, want expertise: the proficiency comes from another effect", got)
	}
	if got := skillLevel(d, "skill:nature"); got != ProficiencyNone {
		t.Errorf("nature = %v, want none: the effects do not lift each other", got)
	}

	// A race's skill counts too (the half-orc's Menacing).
	elf := standard(class, 1)
	elf.Race = "race:half-orc"
	c2, class2 := expertiseContent(t, expertise("skill:intimidation", true))
	elf.Classes[0].Class = class2
	if got := skillLevel(Derive(elf, c2), "skill:intimidation"); got != ProficiencyExpertise {
		t.Errorf("intimidation of a half-orc = %v, want expertise", got)
	}
}

func TestExpertiseOfEverySkillOnlyWhereProficient(t *testing.T) {
	t.Parallel()
	c, class := expertiseContent(t, expertise("skill:*", true))
	b := standard(class, 1)
	b.SkillProficiencies = []string{"skill:arcana", "skill:stealth"}
	d := Derive(b, c)
	doubled := 0
	for _, s := range d.Skills {
		if s.Proficiency == ProficiencyExpertise {
			doubled++
		}
	}
	// The two picks and the background's two.
	if doubled != 4 || skillLevel(d, "skill:arcana") != ProficiencyExpertise || skillLevel(d, "skill:history") != ProficiencyNone {
		t.Errorf("%d skills doubled, want the character's four proficient ones", doubled)
	}
}

// TestSRDExpertiseIsUnchanged: the rogue's Expertise still doubles the skills the
// sheet picks, and refuses one without proficiency.
func TestSRDExpertiseIsUnchanged(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := standard("class:rogue", 1)
	b.SkillProficiencies = []string{"skill:stealth", "skill:perception", "skill:acrobatics", "skill:deception"}
	b.Expertise = []string{"skill:stealth", "skill:perception"}
	d := Derive(b, c)
	if skillLevel(d, "skill:stealth") != ProficiencyExpertise || skillLevel(d, "skill:perception") != ProficiencyExpertise || skillLevel(d, "skill:acrobatics") != ProficiencyFull {
		t.Errorf("the rogue's expertise = %v %v %v", skillLevel(d, "skill:stealth"), skillLevel(d, "skill:perception"), skillLevel(d, "skill:acrobatics"))
	}
	b.Expertise = []string{"skill:stealth", "skill:arcana"}
	found := false
	for _, i := range Derive(b, c).Issues {
		found = found || i.Code == IssueExpertise
	}
	if !found {
		t.Error("expertise in a skill without proficiency raised no issue")
	}
}

// TestExpertiseFlagIsChecked: the flag belongs to a proficiency effect of level
// expertise on a skill.
func TestExpertiseFlagIsChecked(t *testing.T) {
	t.Parallel()
	base := loadForTest(t)
	cases := []struct {
		name string
		e    Effect
	}{
		{"on a full proficiency", Effect{Type: "proficiency", Proficiency: "skill:arcana", Level: "full", OnlyProficient: true}},
		{"without a level", Effect{Type: "proficiency", Proficiency: "skill:arcana", OnlyProficient: true}},
		{"on a saving throw", Effect{Type: "proficiency", Proficiency: "save.wis", Level: "expertise", OnlyProficient: true}},
		{"on another effect", Effect{Type: "modifier", Target: "ac", Mode: "add", Value: "1", OnlyProficient: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			class := genClass(base, genKinds[0])
			class.Levels[0].Features = []TableFeature{tf("x", "X", tc.e)}
			_, err := base.With(Overlay{Revision: 1, Classes: []TableClass{class}, Strict: []string{class.Key}})
			var oe *OverlayError
			if !errors.As(err, &oe) || oe.Field != "classes[0].levels[0].features[0].effects[0].only_proficient" || oe.Reason != ReasonValue {
				t.Errorf("err = %v, want only_proficient refused (bad_value)", err)
			}
		})
	}
}

// TestExpertiseFlagIsOnTheMenu: the proficiency effect offers the switch to the
// editor.
func TestExpertiseFlagIsOnTheMenu(t *testing.T) {
	t.Parallel()
	for _, mt := range loadForTest(t).EffectMenu().Types {
		if mt.Type != "proficiency" {
			continue
		}
		for _, f := range mt.Fields {
			if f.Name == "only_proficient" && f.Kind == "yes_no" && !f.Required {
				return
			}
		}
		t.Fatalf("the proficiency effect has no only_proficient switch: %+v", mt.Fields)
	}
	t.Fatal("no proficiency type on the menu")
}
