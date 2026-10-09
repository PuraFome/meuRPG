package rules

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// protoAbility is each ability's field name in the rules.v1.AbilityScores
// proto message, used in Issue and ValidationError fields.
var protoAbility = map[Ability]string{
	STR: "strength", DEX: "dexterity", CON: "constitution",
	INT: "intelligence", WIS: "wisdom", CHA: "charisma",
}

// abbreviationPT is the Portuguese short form of each ability, as on the
// official Brazilian sheet.
var abbreviationPT = map[Ability]string{
	STR: "FOR", DEX: "DES", CON: "CON", INT: "INT", WIS: "SAB", CHA: "CAR",
}

// modifier is floor((score - 10) / 2), also for scores below 10.
func modifier(score int) int {
	d := score - 10
	if d < 0 {
		return (d - 1) / 2
	}
	return d / 2
}

// abilities computes the six scores and modifiers.
func (x *deriver) abilities() {
	raceBonus := map[Ability]int{}
	if x.race != nil {
		for k, v := range x.race.AbilityBonuses {
			if a, ok := ability(k); ok {
				raceBonus[a] += v
			}
		}
	}
	if x.subrace != nil {
		for k, v := range x.subrace.AbilityBonuses {
			if a, ok := ability(k); ok {
				raceBonus[a] += v
			}
		}
	}
	// The abilities the player picked for the race (the half-elf's two +1).
	picked := x.pickedAbilities()
	for _, a := range picked {
		raceBonus[a]++
	}
	x.scores = map[Ability]int{}
	x.mods = map[Ability]int{}
	for _, a := range AllAbilities() {
		field := "full.base_scores." + protoAbility[a]
		base, ok := x.b.BaseScores[a]
		if !ok || base < MinScore || base > MaxScore {
			x.issue(IssueMissing, field, "%s precisa de um valor de %d a %d; os cálculos usam 10.", x.c.namePT(string(a)), MinScore, MaxScore)
			base = 10
		}
		manual := min(max(x.b.ExtraAbilityBonuses[a], -MaxManualBonus), MaxManualBonus)
		score := base + raceBonus[a] + manual
		x.scores[a] = score
		x.mods[a] = modifier(score)
		x.d.Abilities = append(x.d.Abilities, AbilityScore{
			Ability: a, NamePT: x.c.namePT(string(a)),
			Base: base, RaceBonus: raceBonus[a], ManualBonus: manual, Bonus: raceBonus[a] + manual,
			Score: score, Modifier: x.mods[a],
		})
	}
	x.checkRaceBonus()
	// Increases the race lets the player choose (the half-elf's +1 to two
	// abilities) go in the manual bonuses; remind the player while no
	// manual bonus is positive.
	hasManualIncrease := false
	for _, v := range x.b.ExtraAbilityBonuses {
		hasManualIncrease = hasManualIncrease || v > 0
	}
	if x.race != nil && x.race.AbilityBonusChoices != nil && !hasManualIncrease && len(picked) < x.race.AbilityBonusChoices.Choose {
		ch := x.race.AbilityBonusChoices
		names := make([]string, 0, len(ch.From))
		for _, k := range ch.From {
			names = append(names, abbreviationPT[Ability(k)])
		}
		x.d.Hints = append(x.d.Hints, Hint{
			Source: x.race.Key, Mode: "note",
			TextPT: fmt.Sprintf("%s: escolha +1 em %d habilidades (%s) no passo Escolhas.", x.c.namePT(x.race.Key), ch.Choose, strings.Join(names, ", ")),
		})
	}
}

// abilityEffects adds the features that raise a score (Primal Champion) to the
// scores abilities computed, up to each effect's cap, and then reports a score
// above 20 that nothing explains: a positive manual bonus is where a magic item
// that raises a score goes (the SRD lets an item pass 20), and a feature's cap
// lifts the ceiling for the score it raised.
func (x *deriver) abilityEffects() {
	for i, ab := range x.d.Abilities {
		ceiling, score := MaxNormalScore, ab.Score
		for _, a := range x.active {
			e := a.effect
			if e.Type != "modifier" || e.Target != "score."+string(ab.Ability) || len(e.Tags) > 0 || !x.applies(a) {
				continue
			}
			v, ok := x.value(a)
			if !ok {
				continue
			}
			ceiling = max(ceiling, e.Cap)
			score = max(score, min(score+v, e.Cap))
		}
		if score != ab.Score {
			x.d.Abilities[i].Bonus += score - ab.Score
			x.d.Abilities[i].Score, x.d.Abilities[i].Modifier = score, modifier(score)
			x.scores[ab.Ability], x.mods[ab.Ability] = score, modifier(score)
		}
		if score > ceiling && ab.ManualBonus <= 0 {
			x.issue(IssueScoreAbove20, "full.base_scores."+protoAbility[ab.Ability], "%s passa de %d, o máximo normal de um personagem.", x.c.namePT(string(ab.Ability)), ceiling)
		}
	}
}

// ownedTraits are the race's and subrace's traits and the chosen trait
// options, in the order of Derived.Features.
func (x *deriver) ownedTraits() []*srd51.Trait {
	var out []*srd51.Trait
	for _, f := range x.d.Features {
		if t, ok := x.c.traits[f.Key]; ok {
			out = append(out, t)
		}
	}
	return out
}

// proficiencies gathers armor, weapon and tool proficiencies: the starting
// class's (a later class gives its smaller multiclass set), the race's, the
// background's and those granted by effects.
func (x *deriver) proficiencies() {
	c := x.c
	grant := func(key string) {
		if _, ok := c.proficiencies[key]; ok {
			x.proficient[key] = true
		}
	}
	for i, oc := range x.classes {
		list := oc.class.Proficiencies
		if i > 0 {
			list = oc.class.Multiclass.Proficiencies
		}
		for _, p := range list {
			grant(p)
		}
	}
	for _, t := range x.ownedTraits() {
		for _, p := range t.Proficiencies {
			grant(p)
		}
	}
	if bg, ok := c.backgrounds[x.b.Background]; ok {
		for _, p := range bg.Proficiencies {
			grant(p)
		}
	} else if x.b.Background == "" {
		// A custom background's tools; its languages are listed by languages().
		for _, p := range x.b.CustomBackgroundProficiencies {
			grant(p)
		}
	}
	for _, a := range x.active {
		e := a.effect
		if e.Type == "proficiency" && strings.HasPrefix(e.Proficiency, "proficiency:") && len(e.Tags) == 0 && x.applies(a) {
			grant(e.Proficiency)
		}
	}

	kindOrder := map[string]int{"armor": 0, "weapon": 1, "tool": 2, "other": 3}
	for key := range x.proficient {
		p := c.proficiencies[key]
		if p.Kind == "skill" || p.Kind == "saving-throw" {
			continue
		}
		x.d.Proficiencies = append(x.d.Proficiencies, Proficiency{Kind: p.Kind, Key: key, NamePT: x.proficiencyName(key)})
	}
	slices.SortFunc(x.d.Proficiencies, func(a, b Proficiency) int {
		return cmp.Or(cmp.Compare(kindOrder[a.Kind], kindOrder[b.Kind]), comparePT(a.NamePT, b.NamePT))
	})
}

// proficiencyName is the Portuguese name of a proficiency, or of the one
// piece of equipment it covers.
func (x *deriver) proficiencyName(key string) string { return x.c.proficiencyNamePT(key) }

// proficiencyNamePT is the same for the content: the sheet and the effect menu
// name a tool alike.
func (c *content) proficiencyNamePT(key string) string {
	if n, ok := c.namesPT[key]; ok {
		return n
	}
	if p := c.proficiencies[key]; p != nil && len(p.Refs) == 1 {
		if n, ok := c.namesPT[p.Refs[0]]; ok {
			return n
		}
	}
	return c.namesEN[key]
}

// savingThrows: the starting class's saves plus any save proficiency from
// effects.
func (x *deriver) savingThrows() {
	proficient := map[Ability]bool{}
	if len(x.classes) > 0 {
		for _, s := range x.classes[0].class.SavingThrows {
			if a, ok := ability(s); ok {
				proficient[a] = true
			}
		}
	}
	for _, a := range x.active {
		e := a.effect
		if e.Type == "proficiency" && strings.HasPrefix(e.Proficiency, "save.") && len(e.Tags) == 0 && x.applies(a) {
			proficient[Ability(strings.TrimPrefix(e.Proficiency, "save."))] = true
		}
	}
	for _, a := range AllAbilities() {
		bonus := x.mods[a]
		if proficient[a] {
			bonus += x.prof
		}
		bonus = x.modifiers("save.all", x.modifiers("save."+string(a), bonus))
		x.d.SavingThrows = append(x.d.SavingThrows, SavingThrow{
			Ability: a, NamePT: x.c.namePT(string(a)), Proficient: proficient[a], Bonus: bonus,
		})
	}
}

// skillLevels returns each skill's proficiency level, and the skills that
// came automatically (race, background, effects) rather than from the
// player's own choice.
func (x *deriver) skillLevels() (map[string]ProficiencyLevel, map[string]bool) {
	c := x.c
	levels := map[string]ProficiencyLevel{}
	automatic := map[string]bool{}
	raise := func(skill string, to ProficiencyLevel) {
		if levels[skill] < to {
			levels[skill] = to
		}
	}
	if bg, ok := c.backgrounds[x.b.Background]; ok {
		for _, s := range bg.Skills {
			raise(s, ProficiencyFull)
			automatic[s] = true
		}
	}
	if x.b.Background == "" {
		for _, s := range x.b.CustomBackgroundSkills {
			if _, ok := c.skills[s]; ok {
				raise(s, ProficiencyFull)
				automatic[s] = true
			}
		}
	}
	for _, t := range x.ownedTraits() {
		for _, p := range t.Proficiencies {
			if skill, ok := strings.CutPrefix(p, "proficiency:skill-"); ok {
				raise("skill:"+skill, ProficiencyFull)
				automatic["skill:"+skill] = true
			}
		}
	}
	for _, s := range x.b.SkillProficiencies {
		if _, ok := c.skills[s]; ok {
			raise(s, ProficiencyFull)
		}
	}
	for _, a := range x.active {
		e := a.effect
		if e.Type != "proficiency" || !strings.HasPrefix(e.Proficiency, "skill:") || len(e.Tags) > 0 || !x.applies(a) {
			continue
		}
		lvl := proficiencyLevelOf(e.Level)
		if e.Proficiency == "skill:*" {
			for _, s := range c.skillOrder {
				raise(s, lvl)
			}
			continue
		}
		raise(e.Proficiency, lvl)
		if lvl >= ProficiencyFull {
			automatic[e.Proficiency] = true
		}
	}
	for i, s := range x.b.Expertise {
		if _, ok := c.skills[s]; !ok {
			x.issue(IssueUnknownKey, fmt.Sprintf("full.expertise_skill_keys[%d]", i), "A perícia escolhida não existe.")
			continue
		}
		if levels[s] < ProficiencyFull {
			x.issue(IssueExpertise, fmt.Sprintf("full.expertise_skill_keys[%d]", i), "Especialização em %s pede proficiência nessa perícia.", c.namePT(s))
		}
		levels[s] = ProficiencyExpertise
	}
	return levels, automatic
}

func proficiencyLevelOf(s string) ProficiencyLevel {
	switch s {
	case "half":
		return ProficiencyHalf
	case "expertise":
		return ProficiencyExpertise
	default:
		return ProficiencyFull
	}
}

// profBonus is the part of the proficiency bonus a level adds.
func (x *deriver) profBonus(l ProficiencyLevel) int {
	switch l {
	case ProficiencyHalf:
		return x.prof / 2
	case ProficiencyFull:
		return x.prof
	case ProficiencyExpertise:
		return 2 * x.prof
	}
	return 0
}

// skills computes the 18 skills, the three passives and initiative.
func (x *deriver) skills() {
	levels, automatic := x.skillLevels()
	x.skillLevel, x.automaticSkills = levels, automatic
	bonus := map[string]int{}
	for _, key := range x.c.skillOrder {
		s := x.c.skills[key]
		a := Ability(s.Ability)
		b := x.mods[a] + x.profBonus(levels[key])
		b = x.modifiers("skill."+strings.TrimPrefix(key, "skill:"), b)
		bonus[key] = b
		x.d.Skills = append(x.d.Skills, Skill{
			Key: key, NamePT: x.c.namePT(key), Ability: a, Proficiency: levels[key], Bonus: b,
		})
	}
	slices.SortFunc(x.d.Skills, func(a, b Skill) int { return comparePT(a.NamePT, b.NamePT) })
	x.d.PassivePerception = 10 + bonus["skill:perception"]
	x.d.PassiveInvestigation = 10 + bonus["skill:investigation"]
	x.d.PassiveInsight = 10 + bonus["skill:insight"]

	// Initiative is a Dexterity check: Jack of All Trades adds half the
	// proficiency bonus to it, through an "initiative" proficiency effect.
	best := ProficiencyNone
	for _, a := range x.active {
		e := a.effect
		if e.Type == "proficiency" && e.Proficiency == "initiative" && len(e.Tags) == 0 && x.applies(a) {
			best = max(best, proficiencyLevelOf(e.Level))
		}
	}
	x.d.Initiative = x.modifiers("initiative", x.mods[DEX]+x.profBonus(best))
}

// comparePT orders Portuguese names ignoring accents and case, so
// "Atletismo" comes before "Él" and "Íntimo" sorts with the I's.
func comparePT(a, b string) int {
	return cmp.Or(cmp.Compare(foldPT(a), foldPT(b)), cmp.Compare(a, b))
}

var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "è", "e", "í", "i", "ì", "i", "î", "i",
	"ó", "o", "ô", "o", "õ", "o", "ò", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c",
)

func foldPT(s string) string {
	return accentFolder.Replace(strings.ToLower(s))
}

// checkRaceBonus checks the bonuses a table race lets the player place ("+2 and
// +1 to your choice"): the player puts them in the manual bonuses, and an
// Issue says when the manual bonuses do not cover them. A level's Ability Score
// Improvement only adds to them, so it never takes the check back.
func (x *deriver) checkRaceBonus() {
	if x.race == nil {
		return
	}
	want := x.c.raceChoice[x.race.Key]
	if len(want) == 0 {
		return
	}
	var placed []int
	for _, v := range x.b.ExtraAbilityBonuses {
		if v > 0 {
			placed = append(placed, v)
		}
	}
	slices.SortFunc(placed, func(a, b int) int { return b - a })
	covered := len(placed) >= len(want)
	for i := 0; covered && i < len(want); i++ {
		covered = placed[i] >= want[i]
	}
	if covered {
		return
	}
	parts := make([]string, len(want))
	for i, v := range want {
		parts[i] = signed(v)
	}
	x.issue(IssueRaceBonus, "full.extra_ability_bonuses", "%s: distribua %s em habilidades diferentes, à sua escolha, nos bônus manuais.", x.c.namePT(x.race.Key), strings.Join(parts, " e "))
}
