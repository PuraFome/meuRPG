package rules

import (
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// This file turns the SRD's spell strings ("1 action", "60 feet", "Up to 1
// minute") into structured values. The web shows them in Portuguese (60 ft
// becomes "18 m"); the engine never writes that text. Whatever does not fit
// the structure is kept in a Raw field, exactly as the SRD wrote it.

// Casting time units.
const (
	CastAction      = "action"
	CastBonusAction = "bonus_action"
	CastReaction    = "reaction"
	CastMinute      = "minute"
	CastHour        = "hour"
)

// CastingTime is "1 action", "1 bonus action", "10 minutes"...
type CastingTime struct {
	Amount int
	// Unit is one of the Cast* constants, or "" when Raw did not parse.
	Unit string
	// Trigger is the English text after the comma of a reaction ("which
	// you take when..."), empty when the SRD has none. The 5e-database
	// keeps just "1 reaction", so today it is always empty.
	Trigger string
	Raw     string
}

// Range kinds.
const (
	RangeSelf      = "self"
	RangeTouch     = "touch"
	RangeRanged    = "ranged"
	RangeSight     = "sight"
	RangeUnlimited = "unlimited"
	RangeSpecial   = "special"
)

// SpellRange is how far a spell reaches.
type SpellRange struct {
	// Kind is one of the Range* constants.
	Kind string
	// DistanceFt is set for RangeRanged (a mile is 5280 ft).
	DistanceFt int
	Raw        string
}

// SpellComponents are the V, S and M components.
type SpellComponents struct {
	Verbal, Somatic, Material bool
	// MaterialText is the SRD's English description of the material.
	MaterialText string
}

// Duration kinds.
const (
	DurationInstantaneous  = "instantaneous"
	DurationTimed          = "timed"
	DurationUntilDispelled = "until_dispelled"
	DurationSpecial        = "special"
)

// Duration units.
const (
	DurationRound  = "round"
	DurationMinute = "minute"
	DurationHour   = "hour"
	DurationDay    = "day"
)

// SpellDuration is how long a spell lasts.
type SpellDuration struct {
	// Kind is one of the Duration* constants.
	Kind string
	// Amount and Unit (a Duration* unit) are set for DurationTimed; UpTo
	// says "up to" (the spell may end sooner).
	Amount int
	Unit   string
	UpTo   bool
	// Concentration says the caster must keep concentrating.
	Concentration bool
	Raw           string
}

// SpellSave is the saving throw a spell asks for.
type SpellSave struct {
	Ability Ability
	// OnSuccess is "half" (half damage), "none" (no effect) or "other"
	// (something else, in the description).
	OnSuccess string
}

// SpellDamage is one damage type of a spell. The dice are the SRD's text
// by level ("8d6"); DamageAt and ParseDice turn them into numbers.
type SpellDamage struct {
	// Type is a key such as "damage-type:fire", and TypeNamePT its
	// Portuguese name.
	Type, TypeNamePT string
	// BySlotLevel is the dice by the slot level the spell is cast with,
	// ByCharacterLevel by the character's level (cantrips, at the levels
	// where it grows: 1, 5, 11, 17).
	BySlotLevel      map[int]string
	ByCharacterLevel map[int]string
}

// SpellDetails is everything the SRD says about one spell, structured. The
// catalog keeps it apart from the light SpellEntry that ListContent returns,
// because the text is long: the app fetches it per spell (GetSpellDetails).
type SpellDetails struct {
	Spell       SpellEntry
	CastingTime CastingTime
	Range       SpellRange
	Components  SpellComponents
	Duration    SpellDuration
	// AttackType is "melee", "ranged" or "" (no spell attack).
	AttackType string
	// Save is nil when the spell asks for no saving throw.
	Save   *SpellSave
	Damage []SpellDamage
	// HealBySlotLevel is the healing by slot level ("1d8 + MOD"), nil when
	// the spell does not heal.
	HealBySlotLevel map[int]string
	// Description and HigherLevel are the SRD's English paragraphs.
	Description []string
	HigherLevel []string
}

// SpellDetails returns the details of a spell key, and whether it exists.
// The result is shared and must not be modified.
func (c *Content) SpellDetails(key string) (*SpellDetails, bool) {
	d, ok := c.c.spellDetails[key]
	return d, ok
}

// DamageRoll is one damage (or healing) roll of a spell at a level.
type DamageRoll struct {
	// Type is the damage type key; empty for healing.
	Type string
	// Raw is the SRD's dice text; Dice is it parsed, and Parsed false when
	// the text is not a plain formula ("4d6 OR 5d6").
	Raw    string
	Dice   DiceFormula
	Parsed bool
}

// DamageAt lists the damage rolls of the spell cast with a slot of
// slotLevel by a character of characterLevel. A level the table does not
// list uses the highest entry below it.
func (d *SpellDetails) DamageAt(slotLevel, characterLevel int) []DamageRoll {
	var out []DamageRoll
	for _, dm := range d.Damage {
		table, level := dm.BySlotLevel, slotLevel
		if len(dm.ByCharacterLevel) > 0 {
			table, level = dm.ByCharacterLevel, characterLevel
		}
		if raw, ok := atLevel(table, level); ok {
			out = append(out, newRoll(dm.Type, raw))
		}
	}
	return out
}

// HealAt is the healing roll at a slot level, and whether the spell heals.
func (d *SpellDetails) HealAt(slotLevel int) (DamageRoll, bool) {
	raw, ok := atLevel(d.HealBySlotLevel, slotLevel)
	if !ok {
		return DamageRoll{}, false
	}
	return newRoll("", raw), true
}

func newRoll(damageType, raw string) DamageRoll {
	f, ok := ParseDice(raw)
	return DamageRoll{Type: damageType, Raw: raw, Dice: f, Parsed: ok}
}

// atLevel picks the entry for level, or the highest one below it.
func atLevel(table map[int]string, level int) (string, bool) {
	best, found := 0, false
	for l := range table {
		if l <= level && (!found || l > best) {
			best, found = l, true
		}
	}
	if !found {
		return "", false
	}
	return table[best], true
}

// buildSpellDetails structures one SRD spell.
func (c *content) buildSpellDetails(s *srd51.Spell, entry SpellEntry) *SpellDetails {
	d := &SpellDetails{
		Spell:       entry,
		CastingTime: parseCastingTime(s.CastingTime),
		Range:       parseRange(s.Range),
		Components:  SpellComponents{MaterialText: s.Material},
		Duration:    parseDuration(s.Duration, s.Concentration),
		AttackType:  s.AttackType,
		Description: s.Desc,
		HigherLevel: s.HigherLevel,
	}
	for _, comp := range s.Components {
		switch comp {
		case "V":
			d.Components.Verbal = true
		case "S":
			d.Components.Somatic = true
		case "M":
			d.Components.Material = true
		}
	}
	if s.SaveAbility != "" {
		d.Save = &SpellSave{Ability: Ability(s.SaveAbility), OnSuccess: s.SaveSuccess}
	}
	for _, dm := range s.Damage {
		d.Damage = append(d.Damage, SpellDamage{
			Type: dm.DamageType, TypeNamePT: c.namePT(dm.DamageType),
			BySlotLevel: levelTable(dm.AtSlotLevel), ByCharacterLevel: levelTable(dm.AtCharacterLevel),
		})
	}
	d.HealBySlotLevel = levelTable(s.HealAtSlotLevel)
	return d
}

// levelTable turns the SRD's {"2": "4d4"} into {2: "4d4"}.
func levelTable(in map[string]string) map[int]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]string, len(in))
	for k, v := range in {
		if l, err := strconv.Atoi(k); err == nil {
			out[l] = v
		}
	}
	return out
}

var castingUnits = map[string]string{
	"action": CastAction, "bonus action": CastBonusAction, "reaction": CastReaction,
	"minute": CastMinute, "minutes": CastMinute, "hour": CastHour, "hours": CastHour,
}

func parseCastingTime(raw string) CastingTime {
	ct := CastingTime{Raw: raw}
	main, trigger, _ := strings.Cut(raw, ",")
	ct.Trigger = strings.TrimSpace(trigger)
	n, unit, _ := strings.Cut(strings.TrimSpace(main), " ")
	amount, err := strconv.Atoi(n)
	if u, ok := castingUnits[unit]; ok && err == nil {
		ct.Amount, ct.Unit = amount, u
	}
	return ct
}

func parseRange(raw string) SpellRange {
	r := SpellRange{Raw: raw}
	switch strings.ToLower(raw) {
	case "self":
		r.Kind = RangeSelf
	case "touch":
		r.Kind = RangeTouch
	case "sight":
		r.Kind = RangeSight
	case "unlimited":
		r.Kind = RangeUnlimited
	case "special":
		r.Kind = RangeSpecial
	default:
		n, unit, _ := strings.Cut(raw, " ")
		amount, err := strconv.Atoi(n)
		switch {
		case err != nil:
			r.Kind = RangeSpecial
		case unit == "feet":
			r.Kind, r.DistanceFt = RangeRanged, amount
		case unit == "mile" || unit == "miles":
			r.Kind, r.DistanceFt = RangeRanged, amount*5280
		default:
			r.Kind = RangeSpecial
		}
	}
	return r
}

var durationUnits = map[string]string{
	"round": DurationRound, "rounds": DurationRound, "minute": DurationMinute, "minutes": DurationMinute,
	"hour": DurationHour, "hours": DurationHour, "day": DurationDay, "days": DurationDay,
}

func parseDuration(raw string, concentration bool) SpellDuration {
	d := SpellDuration{Raw: raw, Concentration: concentration}
	text := raw
	if rest, ok := strings.CutPrefix(text, "Up to "); ok {
		d.UpTo, text = true, rest
	}
	switch text {
	case "Instantaneous":
		d.Kind = DurationInstantaneous
		return d
	case "Until dispelled":
		d.Kind = DurationUntilDispelled
		return d
	}
	n, unit, _ := strings.Cut(text, " ")
	amount, err := strconv.Atoi(n)
	if u, ok := durationUnits[unit]; ok && err == nil {
		d.Kind, d.Amount, d.Unit = DurationTimed, amount, u
		return d
	}
	d.Kind, d.UpTo = DurationSpecial, false
	return d
}
