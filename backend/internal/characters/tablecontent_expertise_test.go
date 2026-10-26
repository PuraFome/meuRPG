package characters

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// An expertise effect that applies only where the character is proficient
// (TableEffect.only_proficient, MR-025). Each test starts its own database.

func proficiencyOf(c *charactersv1.Character, skill string) rulesv1.ProficiencyLevel {
	for _, s := range c.GetDerived().GetSkills() {
		if s.GetKey() == skill {
			return s.GetProficiency()
		}
	}
	return rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_UNSPECIFIED
}

// TestTableExpertiseOnlyWhereProficient: the class doubles Arcana and History
// only where the character is proficient; the sheet is proficient in Arcana, so
// only Arcana is doubled, and the effect is stored with its switch.
func TestTableExpertiseOnlyWhereProficient(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, rafa := h.newUser("Samuel"), h.newUser("Rafa")
	campaign := h.newCampaign(master, "Mirathel", rafa)

	body := testClass("Estudiosa")
	body.Levels[0].Features = append(body.Levels[0].Features, &rulesv1.TableFeature{NamePt: "Estudo dobrado", Effects: []*rulesv1.TableEffect{
		{Type: "proficiency", Proficiency: "skill:arcana", Level: "expertise", OnlyProficient: true},
		{Type: "proficiency", Proficiency: "skill:nature", Level: "expertise", OnlyProficient: true},
		{Type: "proficiency", Proficiency: "skill:survival", Level: "expertise"},
	}})
	class := master.addEntry(t, campaign, body)

	stored := master.entries(t, campaign)[class.GetKey()].GetTableClass().GetLevels()[0].GetFeatures()
	if last := stored[len(stored)-1].GetEffects(); !last[0].GetOnlyProficient() || last[2].GetOnlyProficient() {
		t.Errorf("stored effects = %v, want only_proficient kept where set", last)
	}

	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: &rulesv1.AbilityScores{Strength: 10, Dexterity: 13, Constitution: 14, Intelligence: 16, Wisdom: 12, Charisma: 8},
		RaceKey:    "race:human",
		Background: &charactersv1.FullSheet_BackgroundKey{BackgroundKey: "background:acolyte"},
		Languages:  testLanguages,
		Classes:    []*charactersv1.ClassLevel{{ClassKey: class.GetKey(), Level: 1}},
		// Arcana and History are the player's picks from the class's list; Survival is the class's.
		SkillProficiencyKeys: []string{"skill:arcana", "skill:history"},
		HitPoints:            hitPointsAverage(),
	}}}
	c := rafa.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, "Estudiosa", sheet)
	for skill, want := range map[string]rulesv1.ProficiencyLevel{
		"skill:arcana":   rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_EXPERTISE,  // proficient: doubled
		"skill:nature":   rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_NONE,       // not proficient: stays out
		"skill:history":  rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_PROFICIENT, // named by no effect
		"skill:survival": rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_EXPERTISE,  // the effect without the switch
	} {
		if got := proficiencyOf(c, skill); got != want {
			t.Errorf("%s = %v, want %v", skill, got, want)
		}
	}
}

// TestTableExpertiseSwitchRefusals: the switch goes with the level expertise on a
// skill, and the refusal names the field.
func TestTableExpertiseSwitchRefusals(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")
	for name, e := range map[string]*rulesv1.TableEffect{
		"a full proficiency": {Type: "proficiency", Proficiency: "skill:arcana", Level: "full", OnlyProficient: true},
		"a saving throw":     {Type: "proficiency", Proficiency: "save.wis", Level: "expertise", OnlyProficient: true},
		"another effect":     {Type: "modifier", Target: "ac", Mode: "add", Value: "1", OnlyProficient: true},
	} {
		body := testClass("Outra")
		body.Levels[0].Features[0].Effects = []*rulesv1.TableEffect{e}
		vs := master.refused(t, campaign, body)
		if len(vs) != 1 || vs[0].GetField() != "table_class.levels[0].features[0].effects[0].only_proficient" || vs[0].GetReason() != "bad_value" {
			t.Errorf("%s: violations %v, want only_proficient refused (bad_value)", name, vs)
		}
	}
}

// TestEffectMenuOffersTheExpertiseSwitch: the menu the editor reads has the
// switch on the proficiency effect.
func TestEffectMenuOffersTheExpertiseSwitch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Samuel")
	campaign := h.newCampaign(master, "Mirathel")
	for _, ty := range master.effectMenu(t, campaign).GetTypes() {
		if ty.GetType() != "proficiency" {
			continue
		}
		for _, f := range ty.GetFields() {
			if f.GetName() == "only_proficient" && f.GetKind() == "yes_no" && !f.GetRequired() {
				return
			}
		}
		t.Fatalf("the proficiency effect has no only_proficient switch: %v", ty.GetFields())
	}
	t.Fatal("no proficiency effect on the menu")
}
