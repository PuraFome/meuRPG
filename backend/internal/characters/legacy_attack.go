package characters

import (
	"regexp"
	"strconv"
	"strings"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// Before Etapa 6 a basic sheet had one attack: a bonus and the damage as
// free text, such as "1d6+2 cortante". Now it has structured attacks. Old
// sheets are converted when they are read (upgradeLegacyAttack), so no data
// migration is needed: the stored JSON stays as it was until the master
// saves the sheet again, and the read is the same for everyone.

// legacyDamagePattern reads "NdS", an optional "+K" or "-K", and the type:
// "1d6+2 cortante", "2d8 - 1 perfurante", "1d4 piercing damage".
var legacyDamagePattern = regexp.MustCompile(
	`^(\d+)\s*d\s*(\d+)\s*(?:([+\-\x{2212}\x{2013}])\s*(\d+))?\s*(?:de\s+)?([a-z ]+?)\s*(?:de dano|dano|damage)?$`)

// legacyDamageTypes maps the words people wrote, in Portuguese and English,
// without accents, to the damage type.
var legacyDamageTypes = map[string]charactersv1.DamageType{
	"acido":       charactersv1.DamageType_DAMAGE_TYPE_ACID,
	"acid":        charactersv1.DamageType_DAMAGE_TYPE_ACID,
	"concussao":   charactersv1.DamageType_DAMAGE_TYPE_BLUDGEONING,
	"contundente": charactersv1.DamageType_DAMAGE_TYPE_BLUDGEONING,
	"bludgeoning": charactersv1.DamageType_DAMAGE_TYPE_BLUDGEONING,
	"frio":        charactersv1.DamageType_DAMAGE_TYPE_COLD,
	"cold":        charactersv1.DamageType_DAMAGE_TYPE_COLD,
	"fogo":        charactersv1.DamageType_DAMAGE_TYPE_FIRE,
	"fire":        charactersv1.DamageType_DAMAGE_TYPE_FIRE,
	"energia":     charactersv1.DamageType_DAMAGE_TYPE_FORCE,
	"force":       charactersv1.DamageType_DAMAGE_TYPE_FORCE,
	"eletrico":    charactersv1.DamageType_DAMAGE_TYPE_LIGHTNING,
	"eletrica":    charactersv1.DamageType_DAMAGE_TYPE_LIGHTNING,
	"relampago":   charactersv1.DamageType_DAMAGE_TYPE_LIGHTNING,
	"lightning":   charactersv1.DamageType_DAMAGE_TYPE_LIGHTNING,
	"necrotico":   charactersv1.DamageType_DAMAGE_TYPE_NECROTIC,
	"necrotic":    charactersv1.DamageType_DAMAGE_TYPE_NECROTIC,
	"perfurante":  charactersv1.DamageType_DAMAGE_TYPE_PIERCING,
	"piercing":    charactersv1.DamageType_DAMAGE_TYPE_PIERCING,
	"veneno":      charactersv1.DamageType_DAMAGE_TYPE_POISON,
	"poison":      charactersv1.DamageType_DAMAGE_TYPE_POISON,
	"psiquico":    charactersv1.DamageType_DAMAGE_TYPE_PSYCHIC,
	"psychic":     charactersv1.DamageType_DAMAGE_TYPE_PSYCHIC,
	"radiante":    charactersv1.DamageType_DAMAGE_TYPE_RADIANT,
	"radiant":     charactersv1.DamageType_DAMAGE_TYPE_RADIANT,
	"cortante":    charactersv1.DamageType_DAMAGE_TYPE_SLASHING,
	"slashing":    charactersv1.DamageType_DAMAGE_TYPE_SLASHING,
	"trovejante":  charactersv1.DamageType_DAMAGE_TYPE_THUNDER,
	"trovao":      charactersv1.DamageType_DAMAGE_TYPE_THUNDER,
	"thunder":     charactersv1.DamageType_DAMAGE_TYPE_THUNDER,
}

// accentFolder drops the accents the damage words can carry.
var accentFolder = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e",
	"í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")

// attackFromLegacy turns the old attack bonus and damage text into one
// attack named "Ataque". It reports false when the text is not "NdS+K type"
// or the numbers would not pass the rules for attacks, so nothing wrong is
// ever shown as if it were right.
func attackFromLegacy(attackBonus int32, damage string) (*charactersv1.BasicAttack, bool) {
	text := accentFolder.Replace(strings.ToLower(strings.TrimSpace(damage)))
	m := legacyDamagePattern.FindStringSubmatch(text)
	if m == nil {
		return nil, false
	}
	// The numbers are short; parsing them as 32-bit integers turns an
	// absurd digit run into an error instead of an overflow.
	count, err1 := strconv.ParseInt(m[1], 10, 32)
	sides, err2 := strconv.ParseInt(m[2], 10, 32)
	var bonus int64
	if m[4] != "" {
		sign := "-"
		if m[3] == "+" {
			sign = ""
		}
		var err3 error
		if bonus, err3 = strconv.ParseInt(sign+m[4], 10, 32); err3 != nil {
			return nil, false
		}
	}
	dtype, ok := legacyDamageTypes[strings.TrimSpace(m[5])]
	if err1 != nil || err2 != nil || !ok {
		return nil, false
	}
	a := &charactersv1.BasicAttack{
		Name:            legacyAttackName,
		AttackBonus:     attackBonus,
		DamageDiceCount: int32(count),
		DamageDiceSides: int32(sides),
		DamageBonus:     int32(bonus),
		DamageType:      dtype,
	}
	if checkAttack(a, "") != nil {
		return nil, false
	}
	return a, true
}

// upgradeLegacyAttack moves the old attack of a basic sheet into attacks
// when the sheet has no attacks yet and the old text can be read. Otherwise
// the old fields stay as they are, and the app shows the text as a note.
func upgradeLegacyAttack(b *charactersv1.BasicSheet) {
	if len(b.GetAttacks()) > 0 || strings.TrimSpace(b.GetDamage()) == "" {
		return
	}
	if a, ok := attackFromLegacy(b.GetAttackBonus(), b.GetDamage()); ok {
		b.Attacks = []*charactersv1.BasicAttack{a}
		b.AttackBonus, b.Damage = 0, ""
	}
}
