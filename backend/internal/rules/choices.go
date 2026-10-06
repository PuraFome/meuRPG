package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// effectHints turns situational effects into Hints: advantage and
// disadvantage (roll_mode), bonuses that only count in some situations
// (tagged proficiency and modifier effects) and notes with a text.
func (x *deriver) effectHints() {
	levels := x.skillLevel
	for _, a := range x.active {
		e := a.effect
		if !x.applies(a) {
			continue
		}
		switch {
		case e.Type == "roll_mode":
			targets := make([]string, len(e.Targets))
			for i, t := range e.Targets {
				targets[i] = hintTarget(t)
			}
			x.hint(a, targets, e.Roll, 0)
		case e.Type == "proficiency" && len(e.Tags) > 0 && strings.HasPrefix(e.Proficiency, "skill:"):
			// Such as Artificer's Lore: History with expertise, only about
			// magic items. The hint carries the whole bonus.
			s, ok := x.c.skills[e.Proficiency]
			if !ok {
				continue
			}
			lvl := max(levels[e.Proficiency], proficiencyLevelOf(e.Level))
			v := x.mods[Ability(s.Ability)] + x.profBonus(lvl)
			x.hint(a, []string{e.Proficiency}, "bonus", v)
		case e.Type == "modifier" && len(e.Tags) > 0:
			v, ok := x.value(a)
			if ok {
				x.hint(a, []string{hintTarget(e.Target)}, "bonus", v)
			}
		case e.Type == "note" && e.TextPT != "":
			v := 0
			if e.value != nil {
				var ok bool
				if v, ok = x.value(a); !ok {
					continue
				}
			}
			x.hint(a, nil, "note", v)
		}
	}
}

// hintTarget writes skill targets as skill keys ("skill.history" becomes
// "skill:history"), and leaves the others ("save.int") as they are.
func hintTarget(t string) string {
	if s, ok := strings.CutPrefix(t, "skill."); ok {
		return "skill:" + s
	}
	return t
}

// hint adds one Hint. The effect's TextPT may carry {value}, replaced by
// the signed value ("+8").
func (x *deriver) hint(a activeEffect, targets []string, mode string, value int) {
	e := a.effect
	text := e.TextPT
	if text == "" {
		text = x.c.namePT(a.owner)
	}
	text = strings.ReplaceAll(text, "{value}", signed(value))
	text = strings.ReplaceAll(text, "{n}", strconv.Itoa(value))
	h := Hint{Source: a.owner, Targets: targets, Mode: mode, Tags: e.Tags, TextPT: text}
	if len(targets) > 0 {
		h.Target = targets[0]
	}
	if mode == "bonus" {
		h.Value = value
	}
	x.d.Hints = append(x.d.Hints, h)
}

func signed(n int) string {
	if n >= 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// checkChoices reports choices the rules would not allow: the number of
// chosen skills and expertise, the custom background's skills and the
// multiclass prerequisites. Spell choices are checked in spellcasting.
func (x *deriver) checkChoices() {
	c := x.c

	// Skills: the player chooses the starting class's skills, one more from
	// some multiclass classes, and those that effects or traits offer
	// (Skill Versatility, the bard's Bonus Proficiencies). Race and
	// background skills come automatically and do not count.
	automatic := x.automaticSkills
	allowed := 0
	for i, oc := range x.classes {
		switch {
		case i == 0:
			allowed += oc.class.SkillChoices.Choose
		case oc.class.Multiclass.SkillChoices != nil:
			allowed += oc.class.Multiclass.SkillChoices.Choose
		}
	}
	for _, t := range x.ownedTraits() {
		if t.ProficiencyChoices > 0 && len(t.ProficiencyOptions) > 0 && strings.HasPrefix(t.ProficiencyOptions[0], "proficiency:skill-") {
			allowed += t.ProficiencyChoices
		}
	}
	expertiseAllowed := 0
	for _, a := range x.active {
		if a.effect.Type == "choice" {
			switch a.effect.Choice {
			case "skill":
				allowed += a.effect.Count
			case "expertise":
				expertiseAllowed += a.effect.Count
			}
		}
	}
	chosen := map[string]bool{}
	for i, s := range x.b.SkillProficiencies {
		if _, ok := c.skills[s]; !ok {
			x.issue(IssueUnknownKey, fmt.Sprintf("full.skill_proficiency_keys[%d]", i), "A perícia escolhida não existe.")
			continue
		}
		if !automatic[s] {
			chosen[s] = true
		}
	}
	if len(x.classes) > 0 {
		switch n := len(chosen); {
		case n > allowed:
			x.issue(IssueSkillCount, "full.skill_proficiency_keys", "Há %d perícias escolhidas; o personagem escolhe %d.", n, allowed)
		case n < allowed:
			x.issue(IssueSkillCount, "full.skill_proficiency_keys", "Faltam %d perícias para escolher.", allowed-n)
		}
	}

	// Expertise comes from features such as the rogue's Expertise.
	for _, f := range x.d.Features {
		if feat, ok := c.features[f.Key]; ok {
			expertiseAllowed += feat.ExpertiseChoices
		}
	}
	if n := len(x.b.Expertise); n > expertiseAllowed {
		x.issue(IssueExpertise, "full.expertise_skill_keys", "Há %d perícias com especialização; o personagem tem %d.", n, expertiseAllowed)
	}

	// A custom background grants two skills, like every SRD background.
	if x.customBackground() {
		if n := len(x.b.CustomBackgroundSkills); n < CustomBackgroundSkillCount {
			x.issue(IssueSkillCount, "full.custom_background.skill_keys", "O antecedente personalizado concede %d perícias; faltam %d.", CustomBackgroundSkillCount, CustomBackgroundSkillCount-n)
		}
		// SRD 5.1 "Customizing a Background": two tools or languages, a feature and
		// the equipment too (question 82).
		if n := len(x.b.CustomBackgroundProficiencies); n < CustomBackgroundProficiencyCount {
			x.issue(IssueMissing, "full.custom_background.proficiency_keys", "O antecedente personalizado concede %d ferramentas ou idiomas; faltam %d.", CustomBackgroundProficiencyCount, CustomBackgroundProficiencyCount-n)
		}
		if x.b.CustomBackgroundFeatureName == "" || x.b.CustomBackgroundFeature == "" {
			x.issue(IssueMissing, "full.custom_background.feature_name", "O antecedente personalizado tem uma característica: falta o nome ou o texto dela.")
		}
		if x.b.CustomBackgroundEquipment == "" {
			x.issue(IssueMissing, "full.custom_background.equipment", "O antecedente personalizado traz equipamento: falta descrevê-lo.")
		}
	}

	// Multiclassing needs the prerequisites of every class, the first one
	// included.
	if len(x.classes) > 1 {
		for _, oc := range x.classes {
			if !x.meetsMulticlass(oc) {
				x.issue(IssueMulticlass, fmt.Sprintf("full.classes[%d].class_key", oc.index), "Os atributos não cumprem o pré-requisito de multiclasse de %s.", c.namePT(oc.key))
			}
		}
	}
}

func (x *deriver) meetsMulticlass(oc ownedClass) bool {
	mc := oc.class.Multiclass
	for k, v := range mc.Minimums {
		if x.scores[Ability(k)] < v {
			return false
		}
	}
	if len(mc.AnyOf) == 0 {
		return true
	}
	for k, v := range mc.AnyOf {
		if x.scores[Ability(k)] >= v {
			return true
		}
	}
	return false
}

// summarize names a Build's race and classes for a list row.
func summarize(b Build, c *content) Summary {
	var s Summary
	if sub, ok := c.subraces[b.Subrace]; ok && sub.Race == b.Race {
		s.RaceNamePT = c.namePT(b.Subrace)
	} else if _, ok := c.races[b.Race]; ok {
		s.RaceNamePT = c.namePT(b.Race)
	}
	var parts []string
	for _, cl := range b.Classes {
		if _, ok := c.classes[cl.Class]; !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d", c.namePT(cl.Class), cl.Level))
		s.TotalLevel += cl.Level
	}
	s.ClassSummaryPT = strings.Join(parts, " / ")
	return s
}
