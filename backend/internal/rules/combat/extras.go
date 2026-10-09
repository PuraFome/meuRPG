package combat

import (
	"fmt"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What is added to the damage of a hit, on top of the weapon (SRD 5.1,
// "Damage Rolls"). Two families: the extras a player chooses to add to a hit
// (Sneak Attack, Divine Smite, Hunter's Mark, Colossus Slayer), each with the
// condition that makes it valid, and the automatic lines the server applies
// (Rage, Improved Divine Smite, Great Weapon Fighting). The conditions are
// checked here, from the facts the play module reads; a condition that fails
// does not hide the extra, it leaves the line disabled with its reason.

// The keys of the damage parts that are not the weapon.
const (
	ExtraSneakAttack = "sneak-attack"
	ExtraDivineSmite = "divine-smite"
	// ExtraDivineSmiteExtra is the extra 1d8 of Divine Smite against an undead or a
	// fiend. It is a part of its own, always rolled with the smite, and the server
	// counts it only when the target is one: the player is never told why.
	ExtraDivineSmiteExtra = "divine-smite-extra"
	ExtraHuntersMark      = "hunters-mark"
	ExtraColossusSlayer   = "colossus-slayer"
	// The automatic lines.
	AutoRage                = "rage"
	AutoImprovedDivineSmite = "improved-divine-smite"
	AutoGreatWeaponFighting = "great-weapon-fighting"
)

// The numbers of the extras (SRD 5.1, Barbarian, Paladin, Ranger, Rogue).
const (
	smiteBaseDice            = 2
	smiteMaxDice             = 5
	smiteSides               = 8
	sneakAttackSides         = 6
	huntersMarkSides         = 6
	colossusSlayerSides      = 8
	improvedSmiteSides       = 8
	rageBonusLow             = 2
	rageBonusMid             = 3
	rageBonusHigh            = 4
	rageMidLevel             = 9
	rageHighLevel            = 16
	greatWeaponRerollsAtMost = 2
)

// Extra is one line of the damage of a hit besides the weapon's own.
type Extra struct {
	// Key is an Extra* or Auto* constant.
	Key string
	// Count d Sides are the dice, before a critical hit doubles them; Flat is a fixed
	// number added without rolling (Rage), never doubled.
	Count, Sides, Flat int
	// Auto says the server applies it with no choice.
	Auto bool
	// Selected says the line comes marked; Available that it can be marked at all.
	Selected, Available bool
	// Reason says, in Portuguese, why the line is marked, or why it is disabled.
	Reason string
	// NeedsSlot says marking it spends a spell slot, which the player picks.
	NeedsSlot bool
	// Conditional says the dice count only when a fact about the target holds, which
	// the player is not told (Divine Smite against an undead or a fiend).
	Conditional bool
}

// ExtraScene is what decides which extras a hit may carry.
type ExtraScene struct {
	// Weapon says the attack is a weapon attack (an unarmed strike too), not a spell.
	Weapon bool
	// Melee, Finesse and Ranged describe the weapon: Ranged is a ranged weapon (a
	// thrown melee weapon is not one).
	Melee, Finesse, Ranged bool
	// UsesStrength says a melee weapon attack is made with Strength (Rage).
	UsesStrength bool
	// WeaponName is the weapon, for the reasons that name it.
	WeaponName string
	// Mode is the mode the attack was rolled with.
	Mode RollMode
	// WithoutMap says the combat has no squares: whether an enemy is near the target
	// is for the master to say.
	WithoutMap bool
	// EnemyNearTarget says an enemy of the target that is not incapacitated is within
	// 5 ft of it, as the roller sees it.
	EnemyNearTarget bool
	// SneakAttackDice is the rogue's dice (0: no Sneak Attack). SneakUsed says it was
	// used this turn already.
	SneakAttackDice int
	SneakUsed       bool
	// DivineSmite says the character has the feature; SmiteSlotFree that a slot is free.
	DivineSmite, SmiteSlotFree bool
	// ImprovedDivineSmite is the 11th level feature.
	ImprovedDivineSmite bool
	// HuntersMarkKnown says the character has the spell; HuntersMarkActive that it
	// concentrates on it; TargetMarked that the target is the marked creature.
	HuntersMarkKnown, HuntersMarkActive, TargetMarked bool
	// ColossusSlayer says the character has the feature; ColossusUsed that it was used
	// this turn; TargetHurt that the target is below its hit point maximum, and
	// TargetState is the word the players read for how hurt it is.
	ColossusSlayer, ColossusUsed, TargetHurt bool
	TargetState                              string
	TargetLabel                              string
	// Raging says a rage whose benefits hold; BarbarianLevel is the class level.
	Raging         bool
	BarbarianLevel int
}

// SneakAttackDice is the dice of Sneak Attack at a rogue level: 1d6 at level 1 and
// one more at each odd level (SRD, Rogue).
func SneakAttackDice(rogueLevel int) int {
	if rogueLevel < 1 {
		return 0
	}
	return (rogueLevel + 1) / 2
}

// SmiteDice is the dice of Divine Smite for the level of the slot spent: 2d8 for
// a 1st-level slot, one more for each level above it, at most 5d8 (SRD, Paladin).
func SmiteDice(slotLevel int) int {
	return min(smiteBaseDice+max(slotLevel-1, 0), smiteMaxDice)
}

// RageBonus is the damage Rage adds to a melee weapon attack made with Strength:
// +2, +3 from level 9 and +4 from level 16 (SRD, Barbarian).
func RageBonus(barbarianLevel int) int {
	switch {
	case barbarianLevel >= rageHighLevel:
		return rageBonusHigh
	case barbarianLevel >= rageMidLevel:
		return rageBonusMid
	}
	return rageBonusLow
}

// Offered lists the extras a player may add to a hit, in the order the sheet shows
// them, each with the condition that holds or the reason it does not. An extra the
// character does not have is not listed.
func Offered(s ExtraScene) []Extra {
	var out []Extra
	if s.SneakAttackDice > 0 {
		out = append(out, sneakAttack(s))
	}
	if s.DivineSmite {
		out = append(out, divineSmite(s)...)
	}
	if s.HuntersMarkKnown {
		out = append(out, huntersMark(s))
	}
	if s.ColossusSlayer {
		out = append(out, colossusSlayer(s))
	}
	return out
}

func sneakAttack(s ExtraScene) Extra {
	e := Extra{Key: ExtraSneakAttack, Count: s.SneakAttackDice, Sides: sneakAttackSides}
	off := func(reason string) Extra {
		e.Reason = reason
		return e
	}
	switch {
	case !s.Weapon || (!s.Finesse && !s.Ranged):
		name := s.WeaponName
		if name == "" {
			name = "A arma"
		}
		return off(name + " não é de acuidade nem à distância.")
	case s.SneakUsed:
		return off("Já usado neste turno (uma vez por turno).")
	case s.Mode == ModeAdvantage:
		e.Available, e.Selected = true, true
		e.Reason = "Você tem vantagem neste ataque · uma vez por turno"
	case s.Mode == ModeDisadvantage:
		return off("Você tem desvantagem neste ataque.")
	case s.WithoutMap:
		e.Available = true
		e.Reason = "O mestre confirma se há um aliado perto do alvo."
	case s.EnemyNearTarget:
		e.Available, e.Selected = true, true
		e.Reason = "Um inimigo do alvo está a 1,5 m e você não tem desvantagem · uma vez por turno"
	default:
		return off("Sem vantagem e sem um inimigo do alvo a 1,5 m.")
	}
	return e
}

func divineSmite(s ExtraScene) []Extra {
	e := Extra{Key: ExtraDivineSmite, Count: smiteBaseDice, Sides: smiteSides, NeedsSlot: true}
	extra := Extra{Key: ExtraDivineSmiteExtra, Count: 1, Sides: smiteSides, Conditional: true}
	switch {
	case !s.Weapon || !s.Melee:
		e.Reason = "Só vale depois de um acerto corpo a corpo com arma."
	case !s.SmiteSlotFree:
		e.Reason = "Sem espaço de magia livre."
	default:
		e.Available = true
		e.Reason = "Depois do acerto corpo a corpo · gasta 1 espaço de magia"
		extra.Available = true
	}
	return []Extra{e, extra}
}

func huntersMark(s ExtraScene) Extra {
	e := Extra{Key: ExtraHuntersMark, Count: 1, Sides: huntersMarkSides}
	switch {
	case !s.Weapon:
		e.Reason = "Só vale num ataque com arma."
	case !s.HuntersMarkActive:
		e.Reason = "Sem a Marca do Caçador."
	case !s.TargetMarked:
		name := s.TargetLabel
		if name == "" {
			name = "O alvo"
		}
		e.Reason = name + " não é o alvo marcado."
	default:
		e.Available, e.Selected = true, true
		name := s.TargetLabel
		if name == "" {
			name = "O alvo"
		}
		e.Reason = name + " é o alvo marcado · a sua concentração"
	}
	return e
}

func colossusSlayer(s ExtraScene) Extra {
	e := Extra{Key: ExtraColossusSlayer, Count: 1, Sides: colossusSlayerSides}
	state := s.TargetState
	switch {
	case !s.Weapon:
		e.Reason = "Só vale num ataque com arma."
	case s.ColossusUsed:
		e.Reason = "Já usado neste turno (uma vez por turno)."
	case !s.TargetHurt:
		if state == "" {
			state = "Ileso"
		}
		e.Reason = fmt.Sprintf("O alvo está %s: precisa estar abaixo do máximo de PV.", state)
	default:
		e.Available, e.Selected = true, true
		e.Reason = "Uma vez por turno"
		if state != "" {
			e.Reason = fmt.Sprintf("Uma vez por turno · alvo %s (abaixo do máximo de PV)", state)
		}
	}
	return e
}

// Automatic lists the lines the server adds with no choice: Rage's bonus on a
// melee weapon attack made with Strength (a fixed number, so a critical hit does
// not double it) and Improved Divine Smite's 1d8 on any melee weapon hit.
func Automatic(s ExtraScene) []Extra {
	var out []Extra
	if s.Raging && s.Weapon && s.Melee && s.UsesStrength {
		out = append(out, Extra{
			Key: AutoRage, Flat: RageBonus(s.BarbarianLevel), Auto: true, Available: true, Selected: true,
			Reason: "Corpo a corpo com Força, em fúria (automático)",
		})
	}
	if s.ImprovedDivineSmite && s.Weapon && s.Melee {
		out = append(out, Extra{
			Key: AutoImprovedDivineSmite, Count: 1, Sides: improvedSmiteSides, Auto: true, Available: true, Selected: true,
			Reason: "Golpe corpo a corpo de paladino do 11º nível (automático)",
		})
	}
	return out
}

// ExtraDice is how the dice of an extra are made under the table's critical rule:
// how many are rolled and how much is added without rolling. The dice of an extra
// double on a critical hit like the weapon's (SRD, "Damage Rolls"); Flat never does.
func ExtraDice(e Extra, critical bool, rule CriticalRule) (count, fixed int) {
	if e.Count == 0 {
		return 0, 0
	}
	return CriticalDice(rules.DiceFormula{Count: e.Count, Sides: e.Sides}, critical, rule)
}

// Reroll is a die rerolled by Great Weapon Fighting.
type Reroll struct {
	// Index is the position of the die among the weapon's dice.
	Index int
	// From is the face that was rerolled and To the face that counts.
	From, To int
}

// GreatWeaponFighting rerolls the 1s and 2s of the weapon's damage dice once each
// and keeps the new roll (SRD, Fighter, Fighting Style). roll rolls one die of the
// given sides.
func GreatWeaponFighting(faces []int, sides int, roll func(sides int) (int, error)) (kept []int, rerolls []Reroll, err error) {
	kept = append([]int(nil), faces...)
	for i, f := range faces {
		if f > greatWeaponRerollsAtMost {
			continue
		}
		to, err := roll(sides)
		if err != nil {
			return nil, nil, err
		}
		kept[i] = to
		rerolls = append(rerolls, Reroll{Index: i, From: f, To: to})
	}
	return kept, rerolls, nil
}
