package rules

import "strings"

// The ranged_attack effect: what a character's attacks at a distance get from a
// table feat that makes them a sharper shot. The SRD has no such feat; a table's
// feat or feature brings it (a spell sniper's longer reach and the cover its spells
// see past, a sharpshooter's shots that ignore cover and keep their aim at long
// range). The engine stays generic: it reads only the effect's switches.

// What a ranged_attack effect changes (its `applies`).
const (
	// RangedAttackSpell is the attack roll of a spell that needs one.
	RangedAttackSpell = "spell"
	// RangedAttackWeapon is a ranged weapon attack, and a thrown weapon used at range.
	RangedAttackWeapon = "weapon"
)

// The bounds of a ranged_attack's range_multiplier: a spell reaches twice or three
// times its range.
const (
	MinSpellRangeMultiplier = 2
	MaxSpellRangeMultiplier = 3
)

// rangedAttackApplies is the closed list of a ranged_attack's `applies`.
var rangedAttackApplies = []string{RangedAttackSpell, RangedAttackWeapon}

// checkRangedAttack says what is wrong with a ranged_attack effect, and at which
// field of TableEffect, or "" when it is right: it names the attacks it changes,
// a range multiplier is 2 or 3 and only for spells, the long range is only for
// weapons, it does at least one thing, and it is never situational (no tags).
func checkRangedAttack(e *Effect) (attr, msg string) {
	switch {
	case e.Applies != RangedAttackSpell && e.Applies != RangedAttackWeapon:
		return ".applies", "a ranged_attack names the attacks it changes: applies is spell or weapon"
	case e.RangeMultiplier != 0 && e.Applies != RangedAttackSpell:
		return ".range_multiplier", "a range_multiplier is for the spells of a ranged_attack, not its weapons"
	case e.RangeMultiplier != 0 && (e.RangeMultiplier < MinSpellRangeMultiplier || e.RangeMultiplier > MaxSpellRangeMultiplier):
		return ".range_multiplier", "a range_multiplier is 2 or 3"
	case e.NoLongRangeDisadvantage && e.Applies != RangedAttackWeapon:
		return ".no_long_range_disadvantage", "no_long_range_disadvantage is for the weapons of a ranged_attack, not its spells"
	case !e.IgnoreCover && e.RangeMultiplier == 0 && !e.NoLongRangeDisadvantage:
		return ".type", "a ranged_attack that changes nothing: set ignore_cover, range_multiplier or no_long_range_disadvantage"
	case len(e.Tags) > 0:
		return ".tags", "a ranged_attack always applies: it takes no tags"
	}
	return "", ""
}

// rangedAttackStray is the first of the ranged_attack fields an effect of another
// type sets, as a TableEffect path (".ignore_cover"), or "".
func rangedAttackStray(e *Effect) string {
	switch {
	case e.IgnoreCover:
		return ".ignore_cover"
	case e.RangeMultiplier != 0:
		return ".range_multiplier"
	case e.NoLongRangeDisadvantage:
		return ".no_long_range_disadvantage"
	}
	return ""
}

// rangedAttacks fills the Derived fields of the ranged_attack effects the character
// has: the largest spell range multiplier, and whether some effect makes its spell
// attacks or its ranged weapon attacks ignore cover, or its weapons keep their aim at
// long range. It runs before attacks, whose cantrip lines read the multiplier.
func (x *deriver) rangedAttacks() {
	for _, a := range x.active {
		e := a.effect
		if e.Type != "ranged_attack" || !x.applies(a) {
			continue
		}
		switch e.Applies {
		case RangedAttackSpell:
			x.d.SpellAttackRangeMultiplier = max(x.d.SpellAttackRangeMultiplier, e.RangeMultiplier)
			x.d.SpellAttackIgnoresCover = x.d.SpellAttackIgnoresCover || e.IgnoreCover
		case RangedAttackWeapon:
			x.d.RangedWeaponIgnoresCover = x.d.RangedWeaponIgnoresCover || e.IgnoreCover
			x.d.RangedWeaponNoLongRangeDisadvantage = x.d.RangedWeaponNoLongRangeDisadvantage || e.NoLongRangeDisadvantage
		}
	}
}

// SpellRangeFt is how far a spell reaches when this character casts it, in feet: a
// spell that needs an attack roll (attackType "melee" or "ranged") and has a range in
// feet (rangeKind RangeRanged) reaches SpellAttackRangeMultiplier times its rangeFt;
// any other spell, and a character with no multiplier, its own rangeFt. Touch, self
// and sight ranges never change.
func (d Derived) SpellRangeFt(attackType, rangeKind string, rangeFt int) int {
	if attackType == "" || rangeKind != RangeRanged || d.SpellAttackRangeMultiplier < MinSpellRangeMultiplier {
		return rangeFt
	}
	return rangeFt * d.SpellAttackRangeMultiplier
}

// rangedAttackErrorAttr is effectError for what compileEffect says of the ranged_attack
// fields: one set on an effect of another type, or what checkRangedAttack refuses. ok
// is false for any other message (a broken `when`, an empty tag), which effectError
// reads as usual.
func rangedAttackErrorAttr(e *Effect, msg string) (attr string, ok bool) {
	switch {
	case strings.Contains(msg, "belong to ranged_attack"):
		return rangedAttackStray(e), true
	case e.Type != "ranged_attack":
		return "", false
	}
	attr, refused := checkRangedAttack(e)
	return attr, refused != "" && strings.Contains(msg, refused)
}
