package rules

import "testing"

// openOf is how many picks of the kind the sheet still has open (0 for none).
func openOf(d Derived, kind string) int {
	for _, o := range d.OpenChoices {
		if o.Kind == kind {
			return o.Missing
		}
	}
	return 0
}

// An Expertise, a tool or a language pick that is still open warns like a skill or a spell pick does: it is an open
// choice and an issue on the field the player fills.
func TestOpenExpertiseToolAndLanguagePicksWarn(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)

	t.Run("Expertise of a rogue", func(t *testing.T) {
		t.Parallel()
		b := standard("class:rogue", 1)
		d := Derive(b, c)
		if got := openOf(d, OpenChoiceExpertise); got != 2 {
			t.Fatalf("open expertise = %d, want 2 (%+v)", got, d.OpenChoices)
		}
		if !hasIssueAt(d, IssueMissing, "full.expertise_skill_keys") {
			t.Errorf("issues = %v, want one on the expertise field", d.Issues)
		}
		b.SkillProficiencies = []string{"skill:stealth", "skill:acrobatics", "skill:perception", "skill:deception"}
		b.Expertise = []string{"skill:stealth"}
		if got := openOf(Derive(b, c), OpenChoiceExpertise); got != 1 {
			t.Errorf("one expertise picked: open = %d, want 1", got)
		}
		b.Expertise = []string{"skill:stealth", "skill:acrobatics"}
		d = Derive(b, c)
		if got := openOf(d, OpenChoiceExpertise); got != 0 || hasIssueAt(d, IssueMissing, "full.expertise_skill_keys") {
			t.Errorf("both picked: open = %d, issues %v, want none", got, d.Issues)
		}
	})

	t.Run("a class with no expertise asks none", func(t *testing.T) {
		t.Parallel()
		if got := openOf(Derive(standard("class:fighter", 1), c), OpenChoiceExpertise); got != 0 {
			t.Errorf("open expertise of a fighter = %d, want 0", got)
		}
	})

	t.Run("the tool of a dwarf", func(t *testing.T) {
		t.Parallel()
		b := standard("class:fighter", 1)
		b.Race, b.Subrace = "race:dwarf", "subrace:hill-dwarf"
		d := Derive(b, c)
		if got := openOf(d, OpenChoiceTools); got != 1 || !hasIssueAt(d, IssueMissing, "full.tool_proficiencies") {
			t.Fatalf("open tools = %d, issues %v, want 1 and an issue", got, d.Issues)
		}
		b.ToolProficiencies = []string{"Ferramentas de ferreiro"}
		d = Derive(b, c)
		if got := openOf(d, OpenChoiceTools); got != 0 || hasIssueAt(d, IssueMissing, "full.tool_proficiencies") {
			t.Errorf("one tool listed: open = %d, issues %v, want none", got, d.Issues)
		}
	})

	t.Run("the languages of a human and a background", func(t *testing.T) {
		t.Parallel()
		b := standard("class:fighter", 1) // a human (one language) with the acolyte (two)
		b.Languages = nil
		d := Derive(b, c)
		if got := openOf(d, OpenChoiceLanguages); got != 3 || !hasIssueAt(d, IssueMissing, "full.languages") {
			t.Fatalf("open languages = %d, issues %v, want 3 and an issue", got, d.Issues)
		}
		b.Languages = []string{"Anão", "Élfico"}
		if got := openOf(Derive(b, c), OpenChoiceLanguages); got != 1 {
			t.Errorf("two listed: open = %d, want 1", got)
		}
		b.Languages = []string{"Anão", "Élfico", "Gnômico"}
		d = Derive(b, c)
		if got := openOf(d, OpenChoiceLanguages); got != 0 || hasIssueAt(d, IssueMissing, "full.languages") {
			t.Errorf("three listed: open = %d, issues %v, want none", got, d.Issues)
		}
	})

	t.Run("a race with no language to pick asks none", func(t *testing.T) {
		t.Parallel()
		b := standard("class:fighter", 1)
		b.Race, b.Background = "race:dwarf", ""
		b.CustomBackgroundName = "Soldado"
		if got := openOf(Derive(b, c), OpenChoiceLanguages); got != 0 {
			t.Errorf("open languages of a dwarf with a custom background = %d, want 0", got)
		}
	})
}

// hasIssueAt says an issue with the code is on the field.
func hasIssueAt(d Derived, code, field string) bool {
	for _, i := range d.Issues {
		if i.Code == code && i.Field == field {
			return true
		}
	}
	return false
}

// testLanguages are the three languages a human with the acolyte background is owed (one of the race, two of the
// background): the fixtures list them so that a complete sheet has no language pick open.
var testLanguages = []string{"Anão", "Élfico", "Gnômico"}
