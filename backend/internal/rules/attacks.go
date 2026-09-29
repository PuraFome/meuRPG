package rules

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// attacks computes one line per carried weapon and per attack cantrip.
//
// A weapon attacks with STR, or DEX if it is ranged; a finesse weapon (and
// a monk weapon, with Martial Arts) takes the better of the two. The
// proficiency bonus is added with proficiency in the weapon or its
// category. The damage adds the same ability modifier. "attack.weapon.*"
// and "damage.weapon.*" effects add on top (the Archery style).
//
// A cantrip that deals damage uses the damage for the character's total
// level (Fire Bolt: 1d10, then 2d10 at level 5) and the spell attack or
// save DC of a class that has it on its list.
func (x *deriver) attacks() {
	c := x.c
	martialArts := x.hasHandler("monk.martial_arts")
	martialDie := x.martialArtsDie()
	for i, key := range x.b.Weapons {
		eq, ok := c.equipment[key]
		if !ok || eq.Weapon == nil {
			x.issue(IssueUnknownKey, fmt.Sprintf("full.weapon_keys[%d]", i), "A arma escolhida não existe no conteúdo %s.", c.version)
			continue
		}
		w := eq.Weapon
		kind := w.Range // "melee" or "ranged"
		ab := STR
		if kind == "ranged" {
			ab = DEX
		}
		monkWeapon := martialArts && slices.Contains(w.Properties, "weapon-property:monk")
		if (slices.Contains(w.Properties, "weapon-property:finesse") || monkWeapon) && x.mods[DEX] > x.mods[STR] {
			ab = DEX
		}
		proficient := x.weaponProficient(key, w.Category)
		bonus := x.mods[ab]
		if proficient {
			bonus += x.prof
		}
		dmg := x.modifiers("damage.weapon."+kind, x.mods[ab])
		dice := w.Damage
		if monkWeapon && martialDie > 0 {
			dice = biggerDie(dice, martialDie)
		}
		a := Attack{
			Key: key, Name: eq.Name, NamePT: c.namePT(key), Kind: "weapon", Ability: ab,
			AttackBonus: x.modifiers("attack.weapon."+kind, bonus), Proficient: proficient,
			DamageType: w.DamageType, DamageTypeNamePT: c.namePT(w.DamageType),
		}
		if dice != "" {
			a.Damage = withModifier(dice, dmg)
		}
		if w.TwoHandedDamage != "" {
			a.VersatileDamage = withModifier(w.TwoHandedDamage, dmg)
		}
		switch {
		case w.ThrowNormalFt > 0:
			a.RangeFt, a.LongRangeFt = w.ThrowNormalFt, w.ThrowLongFt
		default:
			a.RangeFt, a.LongRangeFt = w.NormalRangeFt, w.LongRangeFt
		}
		x.d.Attacks = append(x.d.Attacks, a)
	}

	for _, key := range x.b.Cantrips {
		s, ok := c.spells[key]
		if !ok || s.Level != 0 || len(s.Damage) == 0 || len(s.Damage[0].AtCharacterLevel) == 0 {
			continue
		}
		sc := x.casterFor(s.Classes)
		if sc == nil {
			continue
		}
		dmg := x.modifiers("damage.spell."+strings.TrimPrefix(key, "spell:"), 0)
		a := Attack{
			Key: key, Name: s.Name, NamePT: c.namePT(key), Kind: "spell", Ability: sc.Ability,
			Damage:     withModifier(damageAt(s.Damage[0].AtCharacterLevel, x.d.TotalLevel), dmg),
			DamageType: s.Damage[0].DamageType, DamageTypeNamePT: c.namePT(s.Damage[0].DamageType),
			Proficient: true, RangeFt: feet(s.Range),
		}
		switch {
		case s.AttackType != "":
			a.AttackBonus = sc.AttackBonus
		case s.SaveAbility != "":
			a.SaveDC = sc.SaveDC
			a.SaveAbility = Ability(s.SaveAbility)
		}
		x.d.Attacks = append(x.d.Attacks, a)
	}
}

// casterFor picks the Spellcasting of a class that has the spell on its
// list, or the first one.
func (x *deriver) casterFor(classes []string) *Spellcasting {
	for i := range x.d.Spellcasting {
		if slices.Contains(classes, x.d.Spellcasting[i].Class) {
			return &x.d.Spellcasting[i]
		}
	}
	if len(x.d.Spellcasting) > 0 {
		return &x.d.Spellcasting[0]
	}
	return nil
}

// martialArtsDie is the monk's Martial Arts die at its level (4 for d4), or
// 0, from the class table's martial_arts column.
func (x *deriver) martialArtsDie() int {
	for _, oc := range x.classes {
		raw := x.c.classLevels[oc.key][oc.level-1].ClassSpecific
		if len(raw) == 0 {
			continue
		}
		var cs struct {
			MartialArts *struct {
				DiceValue int `json:"dice_value"`
			} `json:"martial_arts"`
		}
		if json.Unmarshal(raw, &cs) == nil && cs.MartialArts != nil {
			return cs.MartialArts.DiceValue
		}
	}
	return 0
}

// biggerDie returns "1d<die>" when it beats the weapon's "1d<n>".
func biggerDie(dice string, die int) string {
	count, faces, ok := parseDice(dice)
	if !ok || count != 1 || faces >= die {
		return dice
	}
	return "1d" + strconv.Itoa(die)
}

func parseDice(dice string) (count, faces int, ok bool) {
	n, f, found := strings.Cut(strings.TrimSpace(dice), "d")
	if !found {
		return 0, 0, false
	}
	count, err1 := strconv.Atoi(n)
	faces, err2 := strconv.Atoi(f)
	return count, faces, err1 == nil && err2 == nil
}

// withModifier writes dice and a modifier as the sheet shows them: "1d6+1",
// "1d4-1", or just "1d10".
func withModifier(dice string, mod int) string {
	switch {
	case mod > 0:
		return fmt.Sprintf("%s+%d", dice, mod)
	case mod < 0:
		return fmt.Sprintf("%s%d", dice, mod)
	}
	return dice
}

// damageAt picks the damage for a character level from a cantrip's table,
// whose keys are the levels where it grows ("1", "5", "11", "17").
func damageAt(table map[string]string, level int) string {
	best, bestLevel := "", 0
	for k, v := range table {
		l, err := strconv.Atoi(k)
		if err != nil || l > level || l < bestLevel {
			continue
		}
		best, bestLevel = v, l
	}
	return best
}

// feet reads the number of an SRD range such as "120 feet"; "Touch", "Self"
// and the like give 0.
func feet(r string) int {
	n, _, _ := strings.Cut(r, " ")
	v, err := strconv.Atoi(n)
	if err != nil {
		return 0
	}
	return v
}
