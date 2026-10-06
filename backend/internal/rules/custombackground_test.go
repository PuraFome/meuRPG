package rules

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The free-text "Outro" background (SRD 5.1 "Customizing a Background", question
// 82, MR-025): two skills, two tools or languages in any mix, a feature in the
// player's words and the equipment, and the sheet calculates with it. No database.

func customBuild() Build {
	b := pensantus()
	b.CustomBackgroundProficiencies = []string{"proficiency:thieves-tools", "language:elvish"}
	return b
}

func TestCustomBackgroundDerive(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	d := Derive(customBuild(), c)
	if len(d.Issues) != 0 {
		t.Fatalf("issues = %v, want none", issueCodes(d))
	}
	if d.BackgroundNamePT != "Sábio" || d.BackgroundEquipmentPT != "Um tinteiro, uma pena, uma faca pequena e roupas comuns." {
		t.Errorf("background = %q, equipment %q", d.BackgroundNamePT, d.BackgroundEquipmentPT)
	}
	// The skills.
	for _, s := range []string{"skill:arcana", "skill:history"} {
		if skillOf(d, s).Proficiency != ProficiencyFull {
			t.Errorf("%s is not proficient", s)
		}
	}
	// The tool is a proficiency, the language is a language (the gnome's two are still there).
	if !slices.ContainsFunc(d.Proficiencies, func(p Proficiency) bool { return p.Key == "proficiency:thieves-tools" }) {
		t.Errorf("no thieves' tools in %v", d.Proficiencies)
	}
	var langs []string
	for _, l := range d.Languages {
		langs = append(langs, l.Key)
	}
	for _, want := range []string{"language:common", "language:gnomish", "language:elvish"} {
		if !slices.Contains(langs, want) {
			t.Errorf("languages = %v, want %s", langs, want)
		}
	}
	if slices.Contains(langs, "proficiency:thieves-tools") {
		t.Errorf("a tool is listed as a language: %v", langs)
	}
	// The feature is the player's own text.
	var feature *Feature
	for i := range d.Features {
		if d.Features[i].Key == CustomBackgroundFeatureKey {
			feature = &d.Features[i]
		}
	}
	if feature == nil || feature.NamePT != "Pesquisador" || feature.Source != CustomBackgroundKey || feature.SourcePT != "Sábio" ||
		len(feature.Description) != 1 || !strings.HasPrefix(feature.Description[0], "Quando você não sabe") {
		t.Errorf("feature = %+v", feature)
	}
}

// TestCustomBackgroundIssues: what is missing shows an issue (the app is an assistant,
// not a judge), one for each part.
func TestCustomBackgroundIssues(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for name, tc := range map[string]struct {
		edit func(*Build)
		want []string
	}{
		"complete":             {func(*Build) {}, nil},
		"one skill":            {func(b *Build) { b.CustomBackgroundSkills = b.CustomBackgroundSkills[:1] }, []string{IssueSkillCount + " full.custom_background.skill_keys"}},
		"one tool or language": {func(b *Build) { b.CustomBackgroundProficiencies = b.CustomBackgroundProficiencies[:1] }, []string{IssueMissing + " full.custom_background.proficiency_keys"}},
		"none":                 {func(b *Build) { b.CustomBackgroundProficiencies = nil }, []string{IssueMissing + " full.custom_background.proficiency_keys"}},
		"no feature text":      {func(b *Build) { b.CustomBackgroundFeature = "" }, []string{IssueMissing + " full.custom_background.feature_name"}},
		"no feature name":      {func(b *Build) { b.CustomBackgroundFeatureName = "" }, []string{IssueMissing + " full.custom_background.feature_name"}},
		"no equipment":         {func(b *Build) { b.CustomBackgroundEquipment = "" }, []string{IssueMissing + " full.custom_background.equipment"}},
		"only a name": {func(b *Build) {
			b.CustomBackgroundSkills, b.CustomBackgroundProficiencies = nil, nil
			b.CustomBackgroundFeatureName, b.CustomBackgroundFeature, b.CustomBackgroundEquipment = "", "", ""
		}, []string{
			IssueSkillCount + " full.custom_background.skill_keys", IssueMissing + " full.custom_background.proficiency_keys",
			IssueMissing + " full.custom_background.feature_name", IssueMissing + " full.custom_background.equipment",
		}},
	} {
		b := customBuild()
		tc.edit(&b)
		var got []string
		for _, code := range issueCodes(Derive(b, c)) {
			if strings.Contains(code, "custom_background") {
				got = append(got, code)
			}
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: issues = %v, want %v", name, got, tc.want)
		}
	}
	// No custom background at all, and no background key, is still "Escolha um antecedente".
	b := customBuild()
	b.dropCustomBackground()
	if codes := issueCodes(Derive(b, c)); !slices.Contains(codes, IssueMissing+" full.background_key") {
		t.Errorf("no background: issues = %v", codes)
	}
	// A background key and no custom background has none of it.
	b.Background = "background:acolyte"
	if d := Derive(b, c); d.BackgroundEquipmentPT != "" || slices.ContainsFunc(d.Features, func(f Feature) bool { return f.Key == CustomBackgroundFeatureKey }) {
		t.Error("an SRD background shows a custom feature or equipment")
	}
}

// TestCustomBackgroundValidate: Validate checks the counts, the keys and the lengths.
func TestCustomBackgroundValidate(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	if err := Validate(customBuild(), c); err != nil {
		t.Fatalf("a complete custom background: %v", err)
	}
	long := func(n int) string { return strings.Repeat("a", n) }
	for name, tc := range map[string]struct {
		edit  func(*Build)
		field string
	}{
		"three tools or languages": {func(b *Build) {
			b.CustomBackgroundProficiencies = []string{"proficiency:thieves-tools", "language:elvish", "language:dwarvish"}
		}, "full.custom_background.proficiency_keys"},
		"the same twice":      {func(b *Build) { b.CustomBackgroundProficiencies = []string{"language:elvish", "language:elvish"} }, "full.custom_background.proficiency_keys[1]"},
		"a skill as a tool":   {func(b *Build) { b.CustomBackgroundProficiencies = []string{"skill:arcana"} }, "full.custom_background.proficiency_keys[0]"},
		"armor as a tool":     {func(b *Build) { b.CustomBackgroundProficiencies = []string{"proficiency:light-armor"} }, "full.custom_background.proficiency_keys[0]"},
		"an unknown key":      {func(b *Build) { b.CustomBackgroundProficiencies = []string{"language:klingon"} }, "full.custom_background.proficiency_keys[0]"},
		"a long feature name": {func(b *Build) { b.CustomBackgroundFeatureName = long(41) }, "full.custom_background.feature_name"},
		"a long feature text": {func(b *Build) { b.CustomBackgroundFeature = long(MaxCustomFeatureTextLength + 1) }, "full.custom_background.feature_text"},
		"long equipment":      {func(b *Build) { b.CustomBackgroundEquipment = long(MaxCustomEquipmentLength + 1) }, "full.custom_background.equipment"},
		"a background key and a feature": {func(b *Build) {
			b.Background = "background:acolyte"
			b.CustomBackgroundSkills = nil
			b.CustomBackgroundName = ""
		}, "full.custom_background"},
	} {
		b := customBuild()
		tc.edit(&b)
		err := Validate(b, c)
		ve, ok := errors.AsType[*ValidationError](err)
		if !ok || ve.Field != tc.field {
			t.Errorf("%s: Validate = %v, want a violation of %s", name, err, tc.field)
		}
	}
	// The limits themselves are fine, and so are two languages or two tools.
	b := customBuild()
	b.CustomBackgroundFeatureName, b.CustomBackgroundFeature, b.CustomBackgroundEquipment = long(40), long(MaxCustomFeatureTextLength), long(MaxCustomEquipmentLength)
	b.CustomBackgroundProficiencies = []string{"language:dwarvish", "language:elvish"}
	if err := Validate(b, c); err != nil {
		t.Errorf("the limits: %v", err)
	}
	b.CustomBackgroundProficiencies = []string{"proficiency:thieves-tools", "proficiency:herbalism-kit"}
	if err := Validate(b, c); err != nil {
		t.Errorf("two tools: %v", err)
	}
}

// TestCustomBackgroundIsLockedInTheLevelUp: the level-up changes nothing of the background.
func TestCustomBackgroundIsLockedInTheLevelUp(t *testing.T) {
	t.Parallel()
	before := customBuild()
	for name, edit := range map[string]func(*Build){
		"the tools or languages": func(b *Build) { b.CustomBackgroundProficiencies = []string{"language:elvish"} },
		"the feature name":       func(b *Build) { b.CustomBackgroundFeatureName = "Outro" },
		"the feature":            func(b *Build) { b.CustomBackgroundFeature = "Outro texto." },
		"the equipment":          func(b *Build) { b.CustomBackgroundEquipment = "Nada." },
	} {
		after := before.clone()
		edit(&after)
		if err := checkLocked(before, after); err == nil || err.Field != "full.custom_background" {
			t.Errorf("%s: checkLocked = %v", name, err)
		}
	}
	if err := checkLocked(before, before.clone()); err != nil {
		t.Errorf("a copy: %v", err)
	}
}
