package rules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// maxSpellDice bounds the dice count of a table spell's damage or healing, per
// roll and per level, so the sums stay far from anything the formulas bound.
const maxSpellDice = 30

// plainDice parses "8d6": dice only, no bonus and no modifier.
func plainDice(s string) (DiceFormula, bool) {
	f, ok := ParseDice(s)
	return f, ok && f.Count >= 1 && f.Count <= maxSpellDice && f.Bonus == 0 && !f.AddsModifier && !strings.ContainsAny(s, "+- ")
}

// extraDice checks an optional "extra dice" text of the same die as base: ""
// (none) or plain dice. It returns the count (0 for none).
func extraDice(key, field, s string, base DiceFormula) (int, error) {
	if s == "" {
		return 0, nil
	}
	f, ok := plainDice(s)
	if !ok || f.Sides != base.Sides {
		return 0, ovErr(key, "%s %q is not plain dice of the same die as the base (%dd%d)", field, s, base.Count, base.Sides)
	}
	return f.Count, nil
}

// addSpell registers a table spell and its structured details: the SRD's own
// strings are written from the structured fields and parsed back by the SRD's
// builder, so the table's spell is exactly as the combat code reads an SRD spell.
func (b *overlayBuilder) addSpell(ts *TableSpell) error {
	key := ts.Key
	if ts.Level < 0 || ts.Level > 9 {
		return ovErr(key, "the spell level is 0 to 9")
	}
	if sch, ok := b.base.named[ts.School]; !ok || sch == nil || !strings.HasPrefix(ts.School, "school:") {
		return ovErr(key, "school %q is not in the SRD", ts.School)
	}
	castingTime, err := castingTimeText(key, ts.CastingTime)
	if err != nil {
		return err
	}
	rng, err := rangeText(key, ts.Range)
	if err != nil {
		return err
	}
	duration, err := durationText(key, ts.Duration, ts.Concentration)
	if err != nil {
		return err
	}
	if ts.Ritual && ts.Level == 0 {
		return ovErr(key, "a cantrip is not a ritual")
	}
	comps := []string{}
	if ts.Components.Verbal {
		comps = append(comps, "V")
	}
	if ts.Components.Somatic {
		comps = append(comps, "S")
	}
	if ts.Components.Material {
		comps = append(comps, "M")
	}
	if ts.Components.Material != (ts.Components.MaterialPT != "") {
		return ovErr(key, "the material text goes with the M component, and only with it")
	}
	if err := checkText(key, slices.Concat(ts.DescPT, ts.HigherLevelPT, []string{ts.Components.MaterialPT})); err != nil {
		return err
	}
	var classes []string
	for _, c := range ts.Classes {
		if !b.isClass(c) {
			return ovErr(key, "the class %q of the spell list does not exist", c)
		}
		if slices.Contains(classes, c) {
			return ovErr(key, "the class %q is listed twice", c)
		}
		classes = append(classes, c)
	}
	target, err := checkTarget(key, ts.Target)
	if err != nil {
		return err
	}
	if target.Kind == TargetSelf && ts.Range.Kind != RangeSelf {
		return ovErr(key, "a spell that only affects the caster has the range Self (Pessoal)").at("", ReasonValue)
	}
	if ts.Range.Kind == RangeSelf && (target.Kind == TargetCreature || target.Kind == TargetCreatures) {
		// Pessoal reaches the caster, or an area that comes out of the caster
		// (Mãos Flamejantes): a spell that picks creatures has a distance or Toque.
		return ovErr(key, "a spell with the range Self (Pessoal) reaches only the caster or an area; one that picks creatures has a distance or Touch (Toque)").at("", ReasonValue)
	}
	if target.Kind == TargetArea && ts.Attack != "" {
		return ovErr(key, "a spell attack hits one creature: it cannot have an area target").at("", ReasonValue)
	}

	sp := &srd51.Spell{
		Key: key, Name: ts.NamePT, Level: ts.Level, School: ts.School, Classes: classes,
		Ritual: ts.Ritual, Concentration: ts.Concentration, CastingTime: castingTime, Range: rng,
		Duration: duration, Components: comps, Material: ts.Components.MaterialPT,
		Desc: slices.Clone(ts.DescPT), HigherLevel: slices.Clone(ts.HigherLevelPT),
	}
	switch ts.Attack {
	case "":
	case "melee", "ranged":
		sp.AttackType = ts.Attack
	default:
		return ovErr(key, "the spell attack is melee or ranged")
	}
	if ts.Save != nil {
		if sp.AttackType != "" {
			return ovErr(key, "a spell has an attack or a saving throw, not both")
		}
		if _, ok := abilityIndex[ts.Save.Ability]; !ok {
			return ovErr(key, "the saving throw is not one of the six abilities")
		}
		if ts.Save.OnSuccess != "half" && ts.Save.OnSuccess != "none" {
			return ovErr(key, "on a successful save the spell does half or nothing")
		}
		sp.SaveAbility, sp.SaveSuccess = string(ts.Save.Ability), ts.Save.OnSuccess
	}
	if len(ts.Damage) > 4 {
		return ovErr(key, "a spell has at most 4 damage types")
	}
	for _, d := range ts.Damage {
		sd, err := b.spellDamage(key, ts.Level, d)
		if err != nil {
			return err
		}
		sp.Damage = append(sp.Damage, sd)
	}
	if ts.Save != nil && ts.Save.OnSuccess == "half" && len(sp.Damage) == 0 {
		return ovErr(key, "a save for half damage needs damage")
	}
	if ts.Heal != nil {
		if sp.HealAtSlotLevel, err = spellHeal(key, ts.Level, *ts.Heal); err != nil {
			return err
		}
	}

	b.register(ts.TableEntry)
	b.n.spells[key] = sp
	b.n.spellTargets[key] = target
	return nil
}

// spellDamage writes one damage type as the SRD's table: by slot level for a
// leveled spell, by character level (the tiers 1, 5, 11 and 17) for a cantrip.
func (b *overlayBuilder) spellDamage(key string, level int, d TableSpellDamage) (srd51.SpellDamage, error) {
	if dt, ok := b.base.named[d.Type]; !ok || dt == nil || !strings.HasPrefix(d.Type, "damage-type:") {
		return srd51.SpellDamage{}, ovErr(key, "damage type %q is not in the SRD", d.Type)
	}
	base, ok := plainDice(d.Dice)
	if !ok {
		return srd51.SpellDamage{}, ovErr(key, "damage dice %q are not plain dice, such as 8d6", d.Dice)
	}
	out := srd51.SpellDamage{DamageType: d.Type}
	if level == 0 {
		if d.PerSlotLevel != "" {
			return out, ovErr(key, "a cantrip grows by tier, not by slot level")
		}
		per, err := extraDice(key, "the dice per tier", d.PerTier, base)
		if err != nil {
			return out, err
		}
		out.AtCharacterLevel = map[string]string{}
		for i, lvl := range []int{1, 5, 11, 17} {
			out.AtCharacterLevel[strconv.Itoa(lvl)] = diceText(base.Count+per*i, base.Sides)
		}
		if per == 0 {
			out.AtCharacterLevel = map[string]string{"1": diceText(base.Count, base.Sides)}
		}
		return out, nil
	}
	if d.PerTier != "" {
		return out, ovErr(key, "only a cantrip grows by tier")
	}
	per, err := extraDice(key, "the dice per slot level", d.PerSlotLevel, base)
	if err != nil {
		return out, err
	}
	out.AtSlotLevel = map[string]string{}
	for slot := level; slot <= 9; slot++ {
		out.AtSlotLevel[strconv.Itoa(slot)] = diceText(base.Count+per*(slot-level), base.Sides)
	}
	return out, nil
}

// spellHeal writes the healing as the SRD's table ("1d8 + MOD" by slot level).
func spellHeal(key string, level int, h TableSpellHeal) (map[string]string, error) {
	if level == 0 {
		return nil, ovErr(key, "a cantrip does not heal")
	}
	base, ok := plainDice(h.Dice)
	if !ok {
		return nil, ovErr(key, "healing dice %q are not plain dice, such as 1d8", h.Dice)
	}
	per, err := extraDice(key, "the healing dice per slot level", h.PerSlotLevel, base)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for slot := level; slot <= 9; slot++ {
		text := diceText(base.Count+per*(slot-level), base.Sides)
		if h.AddsModifier {
			text += " + MOD"
		}
		out[strconv.Itoa(slot)] = text
	}
	return out, nil
}

func diceText(count, sides int) string { return fmt.Sprintf("%dd%d", count, sides) }

// checkTarget checks a spell's target and returns it with only the fields its
// kind uses.
func checkTarget(key string, t SpellTarget) (SpellTarget, error) {
	if t.Label != "" {
		return t, ovErr(key, "a table spell's target has no text of its own: the server writes it")
	}
	switch t.Kind {
	case TargetSelf:
		if t.Count != 0 || t.PerSlotLevel != 0 || t.Shape != "" || t.SizeFt != 0 {
			return t, ovErr(key, "a %s target takes nothing else", t.Kind)
		}
	case TargetCreature:
		// One creature, and, if the master wants, one more for each circle above
		// ("uma criatura, mais uma por círculo", as Hold Person).
		if t.Count != 0 || t.PerSlotLevel < 0 || t.PerSlotLevel > 10 || t.Shape != "" || t.SizeFt != 0 {
			return t, ovErr(key, "one creature takes only 0 to 10 more per slot level")
		}
	case TargetCreatures:
		if t.Count < 2 || t.Count > 20 || t.PerSlotLevel < 0 || t.PerSlotLevel > 10 || t.Shape != "" || t.SizeFt != 0 {
			return t, ovErr(key, "several creatures: a count of 2 to 20, and 0 to 10 more per slot level")
		}
	case TargetArea:
		if !slices.Contains([]string{ShapeCone, ShapeCube, ShapeCylinder, ShapeLine, ShapeSphere}, t.Shape) {
			return t, ovErr(key, "an area is a cone, cube, cylinder, line or sphere")
		}
		if t.SizeFt < 5 || t.SizeFt > 300 || t.SizeFt%5 != 0 || t.Count != 0 || t.PerSlotLevel != 0 {
			return t, ovErr(key, "an area is 5 to 300 feet, in steps of 5")
		}
	default:
		return t, ovErr(key, "the target is self, creature, creatures or area")
	}
	return t, nil
}

func castingTimeText(key string, ct TableCastingTime) (string, error) {
	if ct.TriggerPT != "" && ct.Unit != CastReaction {
		return "", ovErr(key, "only a reaction has a trigger in its casting time")
	}
	switch ct.Unit {
	case CastAction, CastBonusAction, CastReaction:
		if ct.Amount != 1 && ct.Amount != 0 {
			return "", ovErr(key, "an action, bonus action or reaction takes 1")
		}
		text := "1 " + strings.ReplaceAll(ct.Unit, "_", " ")
		if ct.TriggerPT != "" {
			if utf8.RuneCountInString(ct.TriggerPT) > 200 || strings.ContainsFunc(ct.TriggerPT, unicode.IsControl) || strings.TrimSpace(ct.TriggerPT) != ct.TriggerPT {
				return "", ovErr(key, "the casting time trigger is one line of at most 200 characters")
			}
			// parseCastingTime reads what follows the first comma as the trigger.
			text += ", " + ct.TriggerPT
		}
		return text, nil
	case CastMinute, CastHour:
		if ct.Amount < 1 || ct.Amount > 60 {
			return "", ovErr(key, "casting time is 1 to 60 %ss", ct.Unit)
		}
		return plural(ct.Amount, ct.Unit), nil
	}
	return "", ovErr(key, "the casting time is an action, bonus action, reaction, minutes or hours")
}

func rangeText(key string, r TableRange) (string, error) {
	switch r.Kind {
	case RangeSelf:
		return "Self", nil
	case RangeTouch:
		return "Touch", nil
	case RangeSight:
		return "Sight", nil
	case RangeUnlimited:
		return "Unlimited", nil
	case RangeRanged:
		if r.DistanceFt < 5 || r.DistanceFt > 5280 || r.DistanceFt%5 != 0 {
			return "", ovErr(key, "the range is 5 to 5280 feet, in steps of 5")
		}
		return strconv.Itoa(r.DistanceFt) + " feet", nil
	}
	return "", ovErr(key, "the range is self, touch, sight, unlimited or a distance")
}

func durationText(key string, d TableDuration, concentration bool) (string, error) {
	switch d.Kind {
	case DurationInstantaneous:
		if concentration {
			return "", ovErr(key, "concentration needs a duration")
		}
		return "Instantaneous", nil
	case DurationUntilDispelled:
		if concentration {
			return "", ovErr(key, "concentration needs a timed duration")
		}
		return "Until dispelled", nil
	case DurationTimed:
		if d.Amount < 1 || d.Amount > 999 || !slices.Contains([]string{DurationRound, DurationMinute, DurationHour, DurationDay}, d.Unit) {
			return "", ovErr(key, "a timed duration is 1 to 999 rounds, minutes, hours or days")
		}
		text := plural(d.Amount, d.Unit)
		if d.UpTo || concentration {
			// A concentration spell always lasts "up to" its duration.
			text = "Up to " + text
		}
		return text, nil
	}
	return "", ovErr(key, "the duration is instantaneous, timed or until dispelled")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}
