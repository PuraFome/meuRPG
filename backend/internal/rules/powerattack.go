package rules

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// The bounds of a power_attack: the penalty taken off the attack roll, the flat damage added to a hit and
// how many weapon properties its filter may name.
const (
	MinPowerAttackPenalty    = 1
	MaxPowerAttackPenalty    = 10
	MinPowerAttackDamage     = 1
	MaxPowerAttackDamage     = 20
	MaxPowerAttackProperties = 4
)

// weaponPropertyPrefix starts the key of a weapon property of the content.
const weaponPropertyPrefix = "weapon-property:"

// PowerAttack is a power_attack effect of a feature the character has: a trade the player may make on
// a weapon attack, AttackPenalty off the attack roll for DamageBonus flat on the damage of a hit. The engine
// is generic; the table names it.
type PowerAttack struct {
	// Key is the feature that gives it, NamePT its name.
	Key, NamePT string
	// AttackPenalty is the number taken off the attack roll and DamageBonus the flat damage added to a hit
	// (a critical hit does not double it).
	AttackPenalty, DamageBonus int
	// Weapon is one of the ExtraDamage*Weapon constants, always set.
	Weapon string
	// Properties are the "weapon-property:" keys the weapon must all have; none is no filter.
	// PropertyNamesPT are their Portuguese names, in the same order.
	Properties, PropertyNamesPT []string
	// OnlyProficient says the weapon line must be one the character is proficient with.
	OnlyProficient bool
	// Resource is the resource each use of the trade spends, one use each; empty for none.
	Resource string
	// TextPT is the line the master reads.
	TextPT string
}

// Allows says whether the trade goes with the attack line a: its kind (a spell attack never does), melee or
// ranged, every property the effect names and, when asked, the character's proficiency with it. When it does
// not, whyPT is the reason in Portuguese. A pseudo-weapon or natural attack line has no properties, so a
// property filter never matches it, and the lines count as proficient as they say.
func (p PowerAttack) Allows(a Attack) (ok bool, whyPT string) {
	switch {
	case a.Kind != "weapon":
		return false, "arma de outro tipo"
	case p.Weapon == ExtraDamageMeleeWeapon && !a.Melee, p.Weapon == ExtraDamageRangedWeapon && a.Melee:
		return false, "arma de outro tipo"
	}
	for i, k := range p.Properties {
		if !slices.Contains(a.Properties, k) {
			return false, "sem a propriedade " + p.propertyName(i)
		}
	}
	if p.OnlyProficient && !a.Proficient {
		return false, "sem proficiência"
	}
	return true, ""
}

// propertyName is the Portuguese name of the i-th property, or its key without the prefix.
func (p PowerAttack) propertyName(i int) string {
	if i < len(p.PropertyNamesPT) && p.PropertyNamesPT[i] != "" {
		return p.PropertyNamesPT[i]
	}
	return strings.TrimPrefix(p.Properties[i], weaponPropertyPrefix)
}

// isWeaponProperty says the key is a weapon property the content has a name for.
func (c *content) isWeaponProperty(k string) bool {
	n, ok := c.named[k]
	return ok && n != nil && strings.HasPrefix(k, weaponPropertyPrefix)
}

// powerAttackShape names the field of a power_attack that is wrong, why in the reason's words (ReasonValue
// or ReasonReference) and the message; field is "" when it is right. The weapon is the closed list of
// extra_damage's, and the properties are the content's.
func powerAttackShape(e *Effect, isProperty func(string) bool) (field, reason, msg string) {
	switch {
	case e.AttackPenalty < MinPowerAttackPenalty || e.AttackPenalty > MaxPowerAttackPenalty:
		return "attack_penalty", ReasonValue, fmt.Sprintf("attack_penalty goes from %d to %d", MinPowerAttackPenalty, MaxPowerAttackPenalty)
	case e.DamageBonus < MinPowerAttackDamage || e.DamageBonus > MaxPowerAttackDamage:
		return "damage_bonus", ReasonValue, fmt.Sprintf("damage_bonus goes from %d to %d", MinPowerAttackDamage, MaxPowerAttackDamage)
	case e.Weapon != "" && !slices.Contains(extraDamageWeapons, e.Weapon):
		return "weapon", ReasonValue, fmt.Sprintf("unknown weapon %q", e.Weapon)
	case len(e.WeaponProperties) > MaxPowerAttackProperties:
		return "weapon_properties", ReasonValue, fmt.Sprintf("weapon_properties has at most %d properties", MaxPowerAttackProperties)
	}
	for i, k := range e.WeaponProperties {
		switch {
		case !isProperty(k):
			return "weapon_properties", ReasonReference, fmt.Sprintf("weapon_properties: %q is not a weapon property", k)
		case slices.Contains(e.WeaponProperties[:i], k):
			return "weapon_properties", ReasonValue, fmt.Sprintf("the weapon property %q is listed twice in weapon_properties", k)
		}
	}
	return "", "", ""
}

// powerAttack checks the fields of a power_attack effect.
func (x effectCompiler) powerAttack() error {
	if field, _, msg := powerAttackShape(x.e, x.c.isWeaponProperty); field != "" {
		return x.fail("%s", msg)
	}
	return nil
}

// strayPowerAttack refuses the fields of power_attack on an effect of another type (weapon, only_proficient and
// damage_bonus are shared with other types and checked where they are).
func (x effectCompiler) strayPowerAttack() error {
	e := x.e
	if e.Type != "power_attack" && (e.AttackPenalty != 0 || len(e.WeaponProperties) > 0) {
		return x.fail("attack_penalty and weapon_properties belong to power_attack")
	}
	return nil
}

// checkPowerAttack checks a power_attack of the table's content: the penalty and the bonus, the weapon and its
// properties, and the resource it spends, which an entry or its class defines.
func (b *overlayBuilder) checkPowerAttack(site effectSite, e *Effect) error {
	if field, reason, msg := powerAttackShape(e, b.base.isWeaponProperty); field != "" {
		return site.fail("."+field, reason, "%s", msg)
	}
	if e.Resource != "" && !b.resourceDefined(site.owner, e.Resource) {
		return site.fail(".resource", ReasonReference, "the power attack spends %q, which no resource effect of this entry or its class defines", e.Resource)
	}
	return nil
}

// powerAttacks fills Derived.PowerAttacks from the power_attack effects the character has. A resource the
// character does not have at this level leaves the effect out, as for an extra_damage.
func (x *deriver) powerAttacks() {
	for _, a := range x.active {
		e := a.effect
		if e.Type != "power_attack" || !x.applies(a) {
			continue
		}
		if e.Resource != "" && !slices.ContainsFunc(x.d.Resources, func(r Resource) bool { return r.Key == e.Resource }) {
			continue
		}
		p := PowerAttack{
			Key: a.owner, NamePT: x.c.namePT(a.owner), AttackPenalty: e.AttackPenalty, DamageBonus: e.DamageBonus,
			Weapon: cmp.Or(e.Weapon, ExtraDamageAnyWeapon), OnlyProficient: e.OnlyProficient, Resource: e.Resource, TextPT: e.TextPT,
		}
		for _, k := range e.WeaponProperties {
			p.Properties = append(p.Properties, k)
			p.PropertyNamesPT = append(p.PropertyNamesPT, x.c.namePT(k))
		}
		x.d.PowerAttacks = append(x.d.PowerAttacks, p)
	}
}

// menuWeaponProperties are the weapon properties of the content, by Portuguese name.
func (c *content) menuWeaponProperties() []MenuValue {
	var out []MenuValue
	for _, k := range sortedKeys(c.named) {
		if c.isWeaponProperty(k) {
			out = append(out, MenuValue{Key: k, NamePT: c.namePT(k)})
		}
	}
	sortMenuByName(out)
	return out
}
