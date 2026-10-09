package combat

import (
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Effects that last (RN-22): the pure part. The play module reads the combat
// and the rows; the arithmetic of what a condition does, of exhaustion and of
// the turn clock is here, tested without a database. Every rule is SRD 5.1:
// "Conditions", "Duration" and "Concentration".

// The condition keys the effects' arithmetic reads.
const (
	condIncapacitated = "condition:incapacitated"
	condGrappled      = "condition:grappled"
	condRestrained    = "condition:restrained"
	condParalyzed     = "condition:paralyzed"
	condPetrified     = "condition:petrified"
	condStunned       = "condition:stunned"
	condUnconscious   = "condition:unconscious"
)

// incapacitatedBy are the conditions that leave a creature without actions and
// reactions: Incapacitated itself and the ones that include it.
var incapacitatedBy = []string{condIncapacitated, condParalyzed, condPetrified, condStunned, condUnconscious}

// noMoveBy are the conditions that stop a creature from moving at all.
var noMoveBy = []string{condParalyzed, condPetrified, condStunned, condUnconscious}

// speedZeroBy are the conditions that bring the speed to 0: the ones above and
// the two that only set it to 0.
var speedZeroBy = []string{condGrappled, condRestrained, condParalyzed, condPetrified, condStunned, condUnconscious}

// autoFailBy are the conditions that fail Strength and Dexterity saving
// throws by themselves.
var autoFailBy = []string{condParalyzed, condPetrified, condStunned, condUnconscious}

// autoCritBy are the conditions that turn a hit from within 5 feet into a
// critical hit.
var autoCritBy = []string{condParalyzed, condUnconscious}

// AutoCritReachFt is how near an attacker must be for the automatic critical:
// within 5 feet (SRD, Paralyzed and Unconscious).
const AutoCritReachFt = 5

func hasAny(conditions, of []string) bool {
	return slices.ContainsFunc(conditions, func(k string) bool { return slices.Contains(of, k) })
}

// Incapacitated says the creature cannot take actions or reactions.
func Incapacitated(conditions []string) bool { return hasAny(conditions, incapacitatedBy) }

// CannotMove says the creature cannot move at all (a Paralyzed, Petrified,
// Stunned or Unconscious one). Grappled and Restrained leave a speed of 0 and
// no more; SpeedZero covers all of them.
func CannotMove(conditions []string) bool { return hasAny(conditions, noMoveBy) }

// SpeedZero says the conditions bring the speed to 0.
func SpeedZero(conditions []string) bool { return hasAny(conditions, speedZeroBy) }

// AutoFailsSave says the creature fails a saving throw of the ability by
// itself, with no d20: Strength and Dexterity of a Paralyzed, Petrified,
// Stunned or Unconscious creature.
func AutoFailsSave(conditions []string, ability string) bool {
	return (ability == "str" || ability == "dex") && hasAny(conditions, autoFailBy)
}

// AutoCrit says a hit from distanceFt away is a critical hit against a creature
// with these conditions: Paralyzed or Unconscious, and the attacker within 5 feet.
func AutoCrit(conditions []string, distanceFt int) bool {
	return distanceFt <= AutoCritReachFt && hasAny(conditions, autoCritBy)
}

// DisadvantageOnDexSaves says Restrained gives disadvantage on Dexterity saving
// throws (SRD, Restrained).
func DisadvantageOnDexSaves(conditions []string) bool {
	return slices.Contains(conditions, condRestrained)
}

// DodgeHolds says the benefit of the Dodge action still holds: it is lost when
// the creature is incapacitated or its speed drops to 0 (SRD, Dodge).
func DodgeHolds(conditions []string, speedFt int) bool {
	return !Incapacitated(conditions) && speedFt > 0 && !SpeedZero(conditions)
}

// ---- exhaustion ----

// MaxExhaustion is the level that kills (SRD, Conditions: Exhaustion).
const MaxExhaustion = 6

// Exhaustion is what a level of exhaustion does; each level adds to the ones
// below it (the creature suffers the effect of its current level and every
// lower one).
type Exhaustion struct {
	// DisadvantageOnChecks is level 1: disadvantage on ability checks.
	DisadvantageOnChecks bool
	// SpeedHalved is level 2.
	SpeedHalved bool
	// DisadvantageOnAttacksAndSaves is level 3: attack rolls and saving throws.
	DisadvantageOnAttacksAndSaves bool
	// MaxHPHalved is level 4.
	MaxHPHalved bool
	// SpeedZero is level 5.
	SpeedZero bool
	// Dead is level 6.
	Dead bool
}

// ExhaustionAt returns the effects of a level, 0 to 6 (anything else is
// clamped into it).
func ExhaustionAt(level int) Exhaustion {
	level = min(max(level, 0), MaxExhaustion)
	return Exhaustion{
		DisadvantageOnChecks:          level >= 1,
		SpeedHalved:                   level >= 2,
		DisadvantageOnAttacksAndSaves: level >= 3,
		MaxHPHalved:                   level >= 4,
		SpeedZero:                     level >= 5,
		Dead:                          level >= 6,
	}
}

// ExhaustedMaxHP is the hit point maximum under a level of exhaustion: halved
// (rounded down, at least 1) from level 4 on.
func ExhaustedMaxHP(maximum, level int) int {
	if !ExhaustionAt(level).MaxHPHalved || maximum <= 1 {
		return maximum
	}
	return max(maximum/2, 1)
}

// ExhaustedSpeedFt is the speed under a level of exhaustion: halved at level 2
// (rounded down), 0 at level 5.
func ExhaustedSpeedFt(speedFt, level int) int {
	switch e := ExhaustionAt(level); {
	case e.SpeedZero:
		return 0
	case e.SpeedHalved:
		return speedFt / 2
	}
	return speedFt
}

// ---- modifiers ----

// ExtraDie is a die an effect adds to (Sign 1) or takes from (Sign -1) a roll.
type ExtraDie struct {
	// Source is the effect it comes from (its key), Faces the die ("4" is a d4).
	Source string
	Faces  int
	Sign   int
}

// Signed is the die's face with its sign: the number added to the roll.
func (d ExtraDie) Signed(face int) int { return d.Sign * face }

// ExtraDice are the dice the modifiers add to a roll of the kind appliesTo
// (rules.RollAppliesAttack or rules.RollAppliesSave). A creature under two
// blessings rolls a d4 for each: the caller removes the repeated spell
// (SRD, "Combining Magical Effects": the same spell's effects do not combine).
func ExtraDice(source string, modifiers []rules.EffectModifier, appliesTo string) []ExtraDie {
	var out []ExtraDie
	for _, m := range modifiers {
		if m.Kind == rules.ModifierRollDie && slices.Contains(m.AppliesTo, appliesTo) {
			out = append(out, ExtraDie{Source: source, Faces: m.Die, Sign: m.Sign})
		}
	}
	return out
}

// ArmorClassBonus is what the modifiers add to the armor class.
func ArmorClassBonus(modifiers []rules.EffectModifier) int {
	n := 0
	for _, m := range modifiers {
		if m.Kind == rules.ModifierACBonus {
			n += m.Value
		}
	}
	return n
}

// SpeedPercent is the speed, in percent of the normal one, the modifiers leave:
// 100 with none; two doublings are not stacked (the same spell does not combine),
// so the highest multiplier wins.
func SpeedPercent(modifiers []rules.EffectModifier) int {
	pct := 100
	found := false
	for _, m := range modifiers {
		if m.Kind != rules.ModifierSpeedMultiplier {
			continue
		}
		if !found || m.Value > pct {
			pct, found = m.Value, true
		}
	}
	return pct
}

// SaveAdvantage says the modifiers give advantage on a saving throw of the ability.
func SaveAdvantage(modifiers []rules.EffectModifier, ability string) bool {
	return slices.ContainsFunc(modifiers, func(m rules.EffectModifier) bool {
		return m.Kind == rules.ModifierSaveAdvantage && slices.Contains(m.Abilities, ability)
	})
}

// ---- the turn clock ----

// A phase of a turn: where in a combatant's turn an effect ends or acts.
const (
	PhaseStart = "start"
	PhaseEnd   = "end"
)

// EndsAt is when an effect ends: the round, the combatant whose turn it is
// anchored to ("" for the start of the round) and the phase of that turn. The
// server works it out when the effect is made; "Resta N rodadas" is the
// difference to it.
type EndsAt struct {
	Round       int
	CombatantID string
	Phase       string
}

// Timed says there is an end on the clock.
func (e EndsAt) Timed() bool { return e.Round > 0 }

// RoundsLeft is how many rounds are left until the effect ends, from the round
// that is running: 0 when it ends in this round or is not timed.
func (e EndsAt) RoundsLeft(round int) int {
	if !e.Timed() {
		return 0
	}
	return max(e.Round-round, 0)
}

// EndsAtStartOf is the end of a duration in rounds: it ends at the start of the
// anchor's turn rounds after the casting (one minute is ten rounds). The anchor
// is the caster: cast in round 1, a minute ends in round 11 on the caster's turn.
func EndsAtStartOf(castRound, rounds int, anchor string) EndsAt {
	return EndsAt{Round: castRound + rounds, CombatantID: anchor, Phase: PhaseStart}
}

// ExpiresAtStart says the effect ends as the turn of the group starts in
// round: its anchor is in the group and the round has come, or the round has
// passed and the anchor never got there (it left the fight, or was out of the
// order). An effect with no anchor ends at the start of its round.
func (e EndsAt) ExpiresAtStart(group []string, round int) bool {
	if !e.Timed() || e.Phase != PhaseStart || round < e.Round {
		return false
	}
	return e.CombatantID == "" || round > e.Round || slices.Contains(group, e.CombatantID)
}

// ExpiresAtEnd says the effect ends as the part of the turn of member ends in
// round: its anchor is the member, the phase is the end and the round has come.
func (e EndsAt) ExpiresAtEnd(member string, round int) bool {
	return e.Timed() && e.Phase == PhaseEnd && e.CombatantID == member && round >= e.Round
}

// ---- who may be a Hold Person target ----

// IsHumanoid says an SRD creature type is humanoid ("humanoid"). A creature the
// master wrote by hand, with no type, is taken as one: a character and an NPC
// made at the table are people unless the master says otherwise.
func IsHumanoid(creatureType string) bool {
	return creatureType == "" || creatureType == "humanoid"
}
