package combat

import "slices"

// Advantage and disadvantage (SRD 5.1, "Advantage and Disadvantage"): a roll
// has advantage or disadvantage from circumstances, however many there are of
// each; when it has both it is a normal roll, even if only one of them is on
// the other side. This file is the pure part: the play module reads the combat,
// works out the facts of Creature and AttackScene, and writes the Portuguese
// sentence of each source.

// RollMode is how a d20 is rolled.
type RollMode int

const (
	// ModeNormal is one d20.
	ModeNormal RollMode = iota
	// ModeAdvantage rolls two d20 and keeps the higher.
	ModeAdvantage
	// ModeDisadvantage rolls two d20 and keeps the lower.
	ModeDisadvantage
)

// Source kinds: the circumstances that give a roll advantage or disadvantage.
const (
	// An attack roll against a target with a condition, or by an attacker with one.
	SourceProneTarget        = "prone_target"
	SourceProneAttacker      = "prone_attacker"
	SourceRestrainedTarget   = "restrained_target"
	SourceRestrainedAttacker = "restrained_attacker"
	SourceBlindedTarget      = "blinded_target"
	SourceBlindedAttacker    = "blinded_attacker"
	SourcePoisonedAttacker   = "poisoned_attacker"
	SourceFrightenedAttacker = "frightened_attacker"
	SourceInvisibleTarget    = "invisible_target"
	SourceInvisibleAttacker  = "invisible_attacker"
	SourceStunnedTarget      = "stunned_target"
	SourceParalyzedTarget    = "paralyzed_target"
	SourceUnconsciousTarget  = "unconscious_target"
	SourcePetrifiedTarget    = "petrified_target"
	// Barbarian: Reckless Attack, for the attacker's own Strength melee attacks and
	// for every attack against the barbarian until its next turn.
	SourceRecklessAttacker = "reckless_attacker"
	SourceRecklessTarget   = "reckless_target"
	// The Dodge action on the target.
	SourceDodgingTarget = "dodging_target"
	// A monster's Pack Tactics.
	SourcePackTactics = "pack_tactics"
	// Unseen Attackers and Targets.
	SourceUnseenAttacker = "unseen_attacker"
	SourceUnseenTarget   = "unseen_target"
	// Help: an ally's Help aimed at the target (SRD 5.1, "Help").
	SourceHelp = "help"
	// Ranged Attacks: beyond the normal range, and a hostile creature within 5 ft.
	SourceLongRange     = "long_range"
	SourceHostileNearby = "hostile_nearby"
	// Saving throws and ability checks.
	SourceDodgingSave     = "dodging_save"
	SourceDangerSense     = "danger_sense"
	SourceRageStrength    = "rage_strength"
	SourceRestrainedSave  = "restrained_save"
	SourcePoisonedCheck   = "poisoned_check"
	SourceFrightenedCheck = "frightened_check"
	SourcePerceptionDim   = "perception_dim"
)

// The condition keys and numbers the rules read.
const (
	conditionProne         = "condition:prone"
	conditionRestrained    = "condition:restrained"
	conditionBlinded       = "condition:blinded"
	conditionDeafened      = "condition:deafened"
	conditionPoisoned      = "condition:poisoned"
	conditionFrightened    = "condition:frightened"
	conditionInvisible     = "condition:invisible"
	conditionStunned       = "condition:stunned"
	conditionParalyzed     = "condition:paralyzed"
	conditionUnconscious   = "condition:unconscious"
	conditionPetrified     = "condition:petrified"
	conditionIncapacitated = "condition:incapacitated"
	conditionGrappled      = "condition:grappled"

	// adjacentFt is "within 5 feet", the SRD's melee distance, and the reach an attack
	// has when it says none.
	adjacentFt = 5

	strengthAbility  = "str"
	dexterityAbility = "dex"
)

// Source is one circumstance that gives a roll advantage or disadvantage.
type Source struct {
	// Kind is a Source* constant.
	Kind string
	// Effect is ModeAdvantage or ModeDisadvantage.
	Effect RollMode
	// Condition is the condition key ("condition:prone") the source comes from, for
	// the sources that are one; "" for the others.
	Condition string
}

// Resolve is the mode the sources make together: any number of one side and
// none of the other is that side; at least one of each is a normal roll
// (SRD, "Advantage and Disadvantage").
func Resolve(sources []Source) RollMode {
	var adv, dis bool
	for _, s := range sources {
		switch s.Effect {
		case ModeAdvantage:
			adv = true
		case ModeDisadvantage:
			dis = true
		case ModeNormal:
		}
	}
	switch {
	case adv && !dis:
		return ModeAdvantage
	case dis && !adv:
		return ModeDisadvantage
	}
	return ModeNormal
}

// Better says whether a mode is better for the roller than another one:
// advantage is better than normal, and normal than disadvantage.
func (m RollMode) Better(than RollMode) bool {
	return m.rank() > than.rank()
}

func (m RollMode) rank() int {
	switch m {
	case ModeAdvantage:
		return 1
	case ModeDisadvantage:
		return -1
	case ModeNormal:
	}
	return 0
}

// Dice is how many d20 the mode rolls: 1 for a normal roll, 2 for the others.
func (m RollMode) Dice() int {
	if m == ModeNormal {
		return 1
	}
	return pairOfDice
}

// Pick returns the index of the die that counts among the d20 faces: the
// higher for advantage (the first on a tie), the lower for disadvantage. One
// face is index 0.
func (m RollMode) Pick(faces []int) int {
	if len(faces) < pairOfDice {
		return 0
	}
	switch m {
	case ModeAdvantage:
		if faces[1] > faces[0] {
			return 1
		}
	case ModeDisadvantage:
		if faces[1] < faces[0] {
			return 1
		}
	case ModeNormal:
	}
	return 0
}

// Creature is what the advantage rules read about one of the two sides of a roll.
type Creature struct {
	// Conditions are the condition keys ("condition:prone") it carries.
	Conditions []string
	// Dodging says it took the Dodge action and the effect has not ended.
	Dodging bool
	// Reckless says the barbarian attacked recklessly this turn.
	Reckless bool
	// PackTactics says the monster it is has the Pack Tactics trait.
	PackTactics bool
	// DangerSense says it has the barbarian's Danger Sense.
	DangerSense bool
	// Raging says it is in a rage whose benefits hold (no heavy armor).
	Raging bool
}

// Has says whether it carries a condition.
func (c Creature) Has(condition string) bool {
	return slices.Contains(c.Conditions, condition)
}

// incapacitating are the conditions that make a creature incapacitated (SRD,
// Conditions): the Incapacitated condition itself and the ones that include it.
var incapacitating = []string{
	conditionIncapacitated, conditionParalyzed, conditionPetrified, conditionStunned, conditionUnconscious,
}

// speedZero are the conditions that bring a creature's speed to 0.
var speedZero = []string{
	conditionGrappled, conditionRestrained, conditionParalyzed, conditionPetrified, conditionStunned, conditionUnconscious,
}

// Incapacitated says the creature cannot take actions or reactions.
func (c Creature) Incapacitated() bool {
	return slices.ContainsFunc(incapacitating, c.Has)
}

// dodgeHolds says the Dodge benefit still stands: it is lost when the creature
// is incapacitated or its speed drops to 0 (SRD, "Dodge").
func (c Creature) dodgeHolds() bool {
	return c.Dodging && !c.Incapacitated() && !slices.ContainsFunc(speedZero, c.Has)
}

// DodgeHolds is dodgeHolds for the play module's displays.
func (c Creature) DodgeHolds() bool { return c.dodgeHolds() }

// AttackScene is everything that decides the mode of one attack roll. The
// play module fills what it knows; a fact it cannot know (the squares in a
// combat without a map) is left unknown, and the rule that needs it adds no source.
type AttackScene struct {
	Attacker, Target Creature
	// Ranged says the attack is a ranged weapon attack, a thrown weapon or a spell
	// attack: the rules of "Ranged Attacks" apply to it. A melee attack is not.
	Ranged bool
	// StrengthMelee says it is a melee weapon attack made with Strength: the only
	// attack Reckless Attack gives advantage to.
	StrengthMelee bool
	// OwnTurn says the attacker acts on its own turn: Reckless Attack gives it
	// advantage only on that turn.
	OwnTurn bool
	// DistanceKnown says DistanceFt is the real distance in feet between the two.
	// Without a map it is false.
	DistanceKnown bool
	DistanceFt    int
	// ReachFt is the attack's reach (melee) or normal range (ranged); LongRangeFt
	// the long range, 0 when it has none.
	ReachFt, LongRangeFt int
	// AttackerUnseen says the target cannot see the attacker (hidden, or out of
	// sight in the dark); TargetUnseen says the attacker cannot see the target.
	AttackerUnseen, TargetUnseen bool
	// Helped says an ally's Help aimed at the target holds for this attacker.
	Helped bool
	// AllyNearTarget says an ally of the attacker that is not incapacitated is
	// within 5 ft of the target (Pack Tactics).
	AllyNearTarget bool
	// HostileNearAttacker says a hostile creature that can see the attacker and
	// is not incapacitated is within 5 ft of it (a ranged attack in close combat).
	HostileNearAttacker bool
}

// withinFive says the attacker is within 5 ft of the target. Without a map a
// melee attack with the usual 5 ft reach is, and a ranged one is unknown.
func (s AttackScene) withinFive() (within, known bool) {
	switch {
	case s.DistanceKnown:
		return s.DistanceFt <= adjacentFt, true
	case !s.Ranged && s.ReachFt <= adjacentFt:
		return true, true
	}
	return false, false
}

// AttackModeResult is the mode an attack roll has: its sources, and whether a hit is
// a critical hit by itself.
type AttackModeResult struct {
	Sources []Source
	// CriticalOnHit says a hit is a critical hit: the attacker is within 5 ft of a
	// paralyzed or unconscious target (SRD, Conditions).
	CriticalOnHit bool
}

// conditionSource is a condition that gives an attack roll advantage or disadvantage.
type conditionSource struct {
	condition string
	kind      string
	effect    RollMode
}

// attackerConditionSources are the conditions of the attacker that change its attack
// rolls, and targetConditionSources those of the target that change the rolls against
// it (SRD 5.1, Conditions). A prone target depends on the distance and is apart.
var attackerConditionSources = []conditionSource{
	{conditionProne, SourceProneAttacker, ModeDisadvantage},
	{conditionRestrained, SourceRestrainedAttacker, ModeDisadvantage},
	{conditionBlinded, SourceBlindedAttacker, ModeDisadvantage},
	{conditionPoisoned, SourcePoisonedAttacker, ModeDisadvantage},
	{conditionFrightened, SourceFrightenedAttacker, ModeDisadvantage},
	{conditionInvisible, SourceInvisibleAttacker, ModeAdvantage},
}

var targetConditionSources = []conditionSource{
	{conditionRestrained, SourceRestrainedTarget, ModeAdvantage},
	{conditionBlinded, SourceBlindedTarget, ModeAdvantage},
	{conditionStunned, SourceStunnedTarget, ModeAdvantage},
	{conditionParalyzed, SourceParalyzedTarget, ModeAdvantage},
	{conditionUnconscious, SourceUnconsciousTarget, ModeAdvantage},
	{conditionPetrified, SourcePetrifiedTarget, ModeAdvantage},
	{conditionInvisible, SourceInvisibleTarget, ModeDisadvantage},
}

// AttackMode lists the circumstances of an attack roll (SRD 5.1, Conditions,
// "Dodge", Barbarian "Reckless Attack", Monsters "Pack Tactics", "Unseen Attackers
// and Targets", "Ranged Attacks"). The app does not keep who a Frightened creature
// fears, so Frightened always counts as the source being in sight.
func AttackMode(s AttackScene) AttackModeResult {
	var out AttackModeResult
	add := func(kind string, effect RollMode, condition string) {
		out.Sources = append(out.Sources, Source{Kind: kind, Effect: effect, Condition: condition})
	}
	within, known := s.withinFive()

	// The conditions of the attacker and of the target.
	for _, r := range attackerConditionSources {
		if s.Attacker.Has(r.condition) {
			add(r.kind, r.effect, r.condition)
		}
	}
	for _, r := range targetConditionSources {
		if s.Target.Has(r.condition) {
			add(r.kind, r.effect, r.condition)
		}
	}
	if s.Target.Has(conditionProne) && known {
		if within {
			add(SourceProneTarget, ModeAdvantage, conditionProne)
		} else {
			add(SourceProneTarget, ModeDisadvantage, conditionProne)
		}
	}
	if known && within && (s.Target.Has(conditionParalyzed) || s.Target.Has(conditionUnconscious)) {
		out.CriticalOnHit = true
	}

	// Reckless Attack.
	if s.Attacker.Reckless && s.StrengthMelee && s.OwnTurn {
		add(SourceRecklessAttacker, ModeAdvantage, "")
	}
	if s.Target.Reckless {
		add(SourceRecklessTarget, ModeAdvantage, "")
	}

	// Dodge: the benefit needs the target to see the attacker.
	targetSees := !s.AttackerUnseen && !s.Attacker.Has(conditionInvisible) && !s.Target.Has(conditionBlinded)
	if s.Target.dodgeHolds() && targetSees {
		add(SourceDodgingTarget, ModeDisadvantage, "")
	}

	// Pack Tactics.
	if s.Attacker.PackTactics && s.AllyNearTarget {
		add(SourcePackTactics, ModeAdvantage, "")
	}

	// Unseen Attackers and Targets. An invisible attacker is listed by its condition.
	if s.AttackerUnseen && !s.Attacker.Has(conditionInvisible) {
		add(SourceUnseenAttacker, ModeAdvantage, "")
	}
	if s.TargetUnseen && !s.Target.Has(conditionInvisible) {
		add(SourceUnseenTarget, ModeDisadvantage, "")
	}

	// Help: the attack roll of the creature an ally helped (SRD 5.1, "Help").
	if s.Helped {
		add(SourceHelp, ModeAdvantage, "")
	}

	out.Sources = append(out.Sources, rangedSources(s)...)
	return out
}

// rangedSources are the disadvantages of a ranged attack: beyond the normal range, and
// with a hostile creature within 5 ft of the attacker (SRD 5.1, "Ranged Attacks").
func rangedSources(s AttackScene) []Source {
	var out []Source
	if !s.Ranged {
		return out
	}
	if s.DistanceKnown && s.ReachFt > 0 && s.DistanceFt > s.ReachFt && (s.LongRangeFt == 0 || s.DistanceFt <= s.LongRangeFt) {
		out = append(out, Source{Kind: SourceLongRange, Effect: ModeDisadvantage})
	}
	if s.HostileNearAttacker {
		out = append(out, Source{Kind: SourceHostileNearby, Effect: ModeDisadvantage})
	}
	return out
}

// SaveScene is what decides the mode of a saving throw or an ability check.
type SaveScene struct {
	Creature Creature
	// Ability is the ability of the save or the check ("str", "dex"...).
	Ability string
	// Check says it is an ability check, not a saving throw.
	Check bool
	// EffectVisible says the creature sees the effect it saves against (Danger
	// Sense): a spell it sees, a trap it has found.
	EffectVisible bool
}

// SaveMode lists the circumstances of a saving throw or an ability check (SRD
// 5.1, Conditions, "Dodge", Barbarian "Rage" and "Danger Sense").
func SaveMode(s SaveScene) []Source {
	var out []Source
	add := func(kind string, effect RollMode, condition string) {
		out = append(out, Source{Kind: kind, Effect: effect, Condition: condition})
	}
	c := s.Creature
	if !s.Check {
		if s.Ability == dexterityAbility && c.dodgeHolds() {
			add(SourceDodgingSave, ModeAdvantage, "")
		}
		if s.Ability == dexterityAbility && c.DangerSense && s.EffectVisible &&
			!c.Has(conditionBlinded) && !c.Has(conditionDeafened) && !c.Incapacitated() {
			add(SourceDangerSense, ModeAdvantage, "")
		}
		if s.Ability == dexterityAbility && c.Has(conditionRestrained) {
			add(SourceRestrainedSave, ModeDisadvantage, conditionRestrained)
		}
	}
	if s.Ability == strengthAbility && c.Raging {
		add(SourceRageStrength, ModeAdvantage, "")
	}
	if s.Check {
		if c.Has(conditionPoisoned) {
			add(SourcePoisonedCheck, ModeDisadvantage, conditionPoisoned)
		}
		if c.Has(conditionFrightened) {
			add(SourceFrightenedCheck, ModeDisadvantage, conditionFrightened)
		}
	}
	return out
}

// AutoFailsSave says the creature fails a saving throw without rolling it: a
// stunned, paralyzed, unconscious or petrified creature fails Strength and
// Dexterity saves (SRD, Conditions).
func AutoFailsSave(c Creature, ability string) bool {
	if ability != strengthAbility && ability != dexterityAbility {
		return false
	}
	return c.Has(conditionStunned) || c.Has(conditionParalyzed) || c.Has(conditionUnconscious) || c.Has(conditionPetrified)
}

// pairOfDice is the d20 a roll with advantage or disadvantage takes.
const pairOfDice = 2
