package rules

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// validate checks what cannot make sense at all, before a sheet is written:
// ranges, list sizes, keys that exist and belong together. Everything that
// is only unusual is left to Derive's Issues.
func validate(b Build, c *content) error {
	fail := func(field, format string, args ...any) error {
		return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
	}

	for _, a := range AllAbilities() {
		field := "full.base_scores." + protoAbility[a]
		v, ok := b.BaseScores[a]
		if !ok {
			return fail(field, "is required")
		}
		if v < MinScore || v > MaxScore {
			return fail(field, "must be %d to %d", MinScore, MaxScore)
		}
	}
	for a := range b.BaseScores {
		if _, ok := abilityIndex[a]; !ok {
			return fail("full.base_scores", "has an unknown ability")
		}
	}
	for a, v := range b.ExtraAbilityBonuses {
		if _, ok := abilityIndex[a]; !ok {
			return fail("full.extra_ability_bonuses", "has an unknown ability")
		}
		if v < -MaxManualBonus || v > MaxManualBonus {
			return fail("full.extra_ability_bonuses."+protoAbility[a], "must be %d to %d", -MaxManualBonus, MaxManualBonus)
		}
	}

	if b.Race == "" {
		return fail("full.race_key", "is required")
	}
	if _, ok := c.races[b.Race]; !ok {
		return fail("full.race_key", "is not a known race")
	}
	if b.Subrace != "" {
		s, ok := c.subraces[b.Subrace]
		if !ok || s.Race != b.Race {
			return fail("full.subrace_key", "is not a subrace of the race")
		}
	}

	if len(b.Classes) == 0 {
		return fail("full.classes", "must have at least one class")
	}
	if len(b.Classes) > MaxClasses {
		return fail("full.classes", "must have at most %d classes", MaxClasses)
	}
	total := 0
	seen := map[string]bool{}
	for i, cl := range b.Classes {
		field := fmt.Sprintf("full.classes[%d]", i)
		if _, ok := c.classes[cl.Class]; !ok {
			return fail(field+".class_key", "is not a known class")
		}
		if seen[cl.Class] {
			return fail(field+".class_key", "repeats a class")
		}
		seen[cl.Class] = true
		if cl.Level < 1 || cl.Level > MaxLevel {
			return fail(field+".level", "must be 1 to %d", MaxLevel)
		}
		total += cl.Level
		if cl.Subclass != "" && cl.CustomSubclassName != "" {
			return fail(field, "has both a subclass and a custom subclass")
		}
		if cl.Subclass != "" {
			s, ok := c.subclasses[cl.Subclass]
			if !ok || s.Class != cl.Class {
				return fail(field+".subclass_key", "is not a subclass of the class")
			}
		}
		if err := checkName(cl.CustomSubclassName); err != "" {
			return fail(field+".custom_subclass_name", "%s", err)
		}
	}
	if total > MaxLevel {
		return fail("full.classes", "levels must add up to at most %d", MaxLevel)
	}

	if b.Background != "" {
		if _, ok := c.backgrounds[b.Background]; !ok {
			return fail("full.background_key", "is not a known background")
		}
		if b.CustomBackgroundName != "" || len(b.CustomBackgroundSkills) > 0 || len(b.CustomBackgroundProficiencies) > 0 ||
			b.CustomBackgroundFeatureName != "" || b.CustomBackgroundFeature != "" || b.CustomBackgroundEquipment != "" {
			return fail("full.custom_background", "cannot be set with background_key")
		}
	}
	if err := checkName(b.CustomBackgroundName); err != "" {
		return fail("full.custom_background.name", "%s", err)
	}
	if err := checkKeys(c, "full.custom_background.skill_keys", b.CustomBackgroundSkills, CustomBackgroundSkillCount, isSkill); err != nil {
		return err
	}
	// Two tools or languages in total, in any mix (SRD 5.1, "Customizing a
	// Background"); fewer shows an Issue, as for the skills.
	if err := checkKeys(c, "full.custom_background.proficiency_keys", b.CustomBackgroundProficiencies, CustomBackgroundProficiencyCount, isToolOrLanguage); err != nil {
		return err
	}
	if err := checkName(b.CustomBackgroundFeatureName); err != "" {
		return fail("full.custom_background.feature_name", "%s", err)
	}
	if utf8.RuneCountInString(b.CustomBackgroundFeature) > MaxCustomFeatureTextLength {
		return fail("full.custom_background.feature_text", "must be at most %d characters", MaxCustomFeatureTextLength)
	}
	if utf8.RuneCountInString(b.CustomBackgroundEquipment) > MaxCustomEquipmentLength {
		return fail("full.custom_background.equipment", "must be at most %d characters", MaxCustomEquipmentLength)
	}
	if err := checkKeys(c, "full.skill_proficiency_keys", b.SkillProficiencies, MaxSkillKeys, isSkill); err != nil {
		return err
	}
	if err := checkKeys(c, "full.expertise_skill_keys", b.Expertise, MaxSkillKeys, isSkill); err != nil {
		return err
	}

	if b.HitPoints.Method != HitPointsFixed && b.HitPoints.Method != HitPointsRolled {
		return fail("full.hit_points.method", "is not a known method")
	}
	if len(b.HitPoints.Rolls) > MaxHitPointRolls {
		return fail("full.hit_points.rolls", "must have at most %d rolls", MaxHitPointRolls)
	}
	for i, r := range b.HitPoints.Rolls {
		if r < 1 || r > MaxRoll {
			return fail(fmt.Sprintf("full.hit_points.rolls[%d]", i), "must be 1 to %d", MaxRoll)
		}
	}

	if b.Armor != "" {
		e, ok := c.equipment[b.Armor]
		if !ok || e.Armor == nil || e.Armor.Category == "shield" {
			return fail("full.armor_key", "is not a known body armor")
		}
	}
	if err := checkKeys(c, "full.weapon_keys", b.Weapons, MaxWeapons, isWeapon); err != nil {
		return err
	}
	if err := checkKeys(c, "full.cantrip_keys", b.Cantrips, MaxCantrips, isCantrip); err != nil {
		return err
	}
	if err := checkKeys(c, "full.known_spell_keys", b.SpellsKnown, MaxKnownSpells, isLeveledSpell); err != nil {
		return err
	}
	if err := checkKeys(c, "full.prepared_spell_keys", b.SpellsPrepared, MaxPreparedSpells, isLeveledSpell); err != nil {
		return err
	}
	return checkKeys(c, "full.feature_choice_keys", b.FeatureChoices, MaxListLength, isOption)
}

// checkName checks a custom name's length; the characters module cleans
// the text itself (one line, no control characters).
func checkName(s string) string {
	if n := utf8.RuneCountInString(s); n > MaxCustomNameLength {
		return fmt.Sprintf("must be at most %d characters", MaxCustomNameLength)
	}
	if strings.TrimSpace(s) != s {
		return "must not start or end with spaces"
	}
	return ""
}

// checkKeys checks a list of keys: at most limit, no repeats, each of the
// right kind.
func checkKeys(c *content, field string, keys []string, limit int, valid func(*content, string) bool) error {
	if len(keys) > limit {
		return &ValidationError{Field: field, Message: fmt.Sprintf("must have at most %d entries", limit)}
	}
	seen := make(map[string]bool, len(keys))
	for i, k := range keys {
		f := fmt.Sprintf("%s[%d]", field, i)
		if seen[k] {
			return &ValidationError{Field: f, Message: "repeats an entry"}
		}
		seen[k] = true
		if !valid(c, k) {
			return &ValidationError{Field: f, Message: "is not a known key of the right kind"}
		}
	}
	return nil
}

func isSkill(c *content, k string) bool {
	_, ok := c.skills[k]
	return ok
}

// isToolOrLanguage: a tool proficiency (the kind a table background's tools
// are) or a language of the SRD.
func isToolOrLanguage(c *content, k string) bool {
	if p, ok := c.proficiencies[k]; ok {
		return p.Kind == "tool" || p.Kind == "other"
	}
	_, ok := c.languages[k]
	return ok
}

func isWeapon(c *content, k string) bool {
	e, ok := c.equipment[k]
	return ok && e.Weapon != nil
}

func isCantrip(c *content, k string) bool {
	s, ok := c.spells[k]
	return ok && s.Level == 0
}

func isLeveledSpell(c *content, k string) bool {
	s, ok := c.spells[k]
	return ok && s.Level > 0
}

// isOption: a feature or trait that is an option of another one.
func isOption(c *content, k string) bool {
	if f, ok := c.features[k]; ok {
		return f.Parent != "" || c.optionParents[k] != ""
	}
	t, ok := c.traits[k]
	return ok && t.Parent != ""
}
