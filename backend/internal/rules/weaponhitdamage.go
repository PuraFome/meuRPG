package rules

import (
	"fmt"
	"slices"
)

// The bounds of a weapon_hit_damage.
const (
	maxHitDice         = 20
	maxHitPerSlotDice  = 10
	maxHitConditions = 8
)

var (
	// hitTriggers are the values of a weapon_hit_damage's trigger.
	hitTriggers = []string{HitTriggerNextHit, HitTriggerNextMeleeHit, HitTriggerEachHit, HitTriggerMarkedTarget}
	// hitDice are the dice a weapon_hit_damage may roll.
	hitDice = []int{4, 6, 8, 10, 12}
)

// HitTriggerNeedsConcentration says the trigger lasts as long as the caster concentrates.
func HitTriggerNeedsConcentration(trigger string) bool {
	return trigger == HitTriggerEachHit || trigger == HitTriggerMarkedTarget
}

// checkWeaponHitDamage checks the shape of a weapon_hit_damage: its own fields, and that no other kind's is set.
func checkWeaponHitDamage(m EffectModifier) error {
	if err := checkHitStrays(m); err != nil {
		return err
	}
	if !slices.Contains(hitTriggers, m.Trigger) {
		return modifierField("trigger", "weapon_hit_damage needs a trigger: next_hit, next_melee_hit, each_hit or marked_target")
	}
	if !slices.Contains(hitDice, m.Die) {
		return modifierField("die", "weapon_hit_damage rolls a d4, d6, d8, d10 or d12")
	}
	return checkHitCounts(m)
}

func checkHitCounts(m EffectModifier) error {
	switch {
	case m.Dice < 1 || m.Dice > maxHitDice:
		return modifierField("dice", "weapon_hit_damage needs dice from 1 to %d", maxHitDice)
	case m.PerSlotDice < 0 || m.PerSlotDice > maxHitPerSlotDice:
		return modifierField("per_slot_dice", "per_slot_dice is from 0 to %d", maxHitPerSlotDice)
	case m.MaxDice < 0 || m.MaxDice > maxHitDice || (m.MaxDice != 0 && m.MaxDice < m.Dice):
		return modifierField("max_dice", "max_dice is from dice to %d", maxHitDice)
	}
	return checkHitSave(m)
}

func checkHitSave(m EffectModifier) error {
	switch {
	case m.Save == "" && len(m.OnFailConditions) > 0:
		return modifierField("on_fail_conditions", "on_fail_conditions needs a save")
	case m.Save == "":
		return nil
	case !slices.Contains(saveAbilities, m.Save):
		return modifierField("save", "save is an ability: str, dex, con, int, wis or cha")
	case len(m.OnFailConditions) == 0:
		return modifierField("on_fail_conditions", "a save needs at least one condition for a fail")
	case len(m.OnFailConditions) > maxHitConditions:
		return modifierField("on_fail_conditions", "on_fail_conditions has at most %d conditions", maxHitConditions)
	}
	return nil
}

// checkHitStrays refuses a field this kind does not read.
func checkHitStrays(m EffectModifier) error {
	stray := []struct {
		name string
		set  bool
	}{
		{"sign", m.Sign != 0},
		{"value", m.Value != 0},
		{"applies_to", len(m.AppliesTo) > 0},
		{"skill", m.Skill != ""},
		{"abilities", len(m.Abilities) > 0},
		{"conditions", len(m.Conditions) > 0},
		{"except_source", m.ExceptSource},
		{"once", m.Once},
		{"choose", m.Choose},
		{"from_caster", m.FromCaster},
		{"allowed", len(m.Allowed) > 0},
		{"max_weapon_attacks", m.MaxWeaponAttacks != 0},
		{"resistance_kind", m.ResistanceKind != ""},
		{"damage_types", len(m.DamageTypes) > 0},
		{"claim", m.Claim != 0},
		{"unclaimed", m.Unclaimed},
	}
	for _, f := range stray {
		if f.set {
			return modifierField(f.name, "weapon_hit_damage does not read %s", f.name)
		}
	}
	return nil
}

// checkWeaponHitContent checks the damage type and the conditions against the SRD content.
func (c *content) checkWeaponHitContent(m EffectModifier) error {
	if m.DamageType != "" && !c.isLiveDamageType(m.DamageType) {
		return modifierField("damage_type", "%q is not an SRD damage type", m.DamageType)
	}
	for j, k := range m.OnFailConditions {
		switch {
		case !c.isCondition(k):
			return modifierField(fmt.Sprintf("on_fail_conditions[%d]", j), "%q is not an SRD condition", k)
		case slices.Contains(m.OnFailConditions[:j], k):
			return modifierField(fmt.Sprintf("on_fail_conditions[%d]", j), "the condition %q is listed twice", k)
		}
	}
	return nil
}

// spellHitRefusal refuses a weapon_hit_damage a table spell cannot keep: each_hit and marked_target last as long
// as the concentration does.
func spellHitRefusal(m EffectModifier, concentration bool) error {
	if m.Kind == ModifierWeaponHitDamage && HitTriggerNeedsConcentration(m.Trigger) && !concentration {
		return modifierField("trigger", "%s needs a spell that asks for concentration", m.Trigger)
	}
	return nil
}
