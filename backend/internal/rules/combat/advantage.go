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
	// Effects that last (RN-22): exhaustion, Faerie Fire, the advantage on saving
	// throws an effect gives (Haste) and the disadvantage an effect gives the
	// creature's own attack rolls (Vicious Mockery).
	SourceExhaustionAttack = "exhaustion_attack"
	SourceExhaustionSave   = "exhaustion_save"
	SourceExhaustionCheck  = "exhaustion_check"
	SourceOutlinedTarget   = "outlined_target"
	SourceEffectSave       = "effect_save"
	SourceEffectCheck      = "effect_check"
	SourceEffectAttack     = "effect_attack"
	// Saving throws and ability checks.
	SourceDodgingSave     = "dodging_save"
	SourceDangerSense     = "danger_sense"
	SourceRageStrength    = "rage_strength"
	SourceRestrainedSave  = "restrained_save"
	SourceStealthArmor    = "stealth_armor"
	SourceArmorUntrained  = "armor_untrained"
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
	// the sources that are one; for SourceEffectAttack, the key of the effect that
	// gives it ("spell:vicious-mockery"); "" for the others.
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
	// Exhaustion is its level of exhaustion, 0 to 6 (SRD, Conditions).
	Exhaustion int
	// Outlined says Faerie Fire outlines it: attacks against it have advantage if the
	// attacker sees it, and it gets nothing from being invisible.
	Outlined bool
	// SaveAdvantage are the abilities an effect that lasts gives it advantage on the
	// saving throws of (Haste: Dexterity).
	SaveAdvantage []string
	// CheckAdvantage are the abilities an effect that lasts gives it advantage on the ability
	// checks of (Enhance Ability).
	CheckAdvantage []string
	// AttackDisadvantage is the key of the effect that lasts that gives its own attack rolls
	// disadvantage (Vicious Mockery: "spell:vicious-mockery"), "" for none.
	AttackDisadvantage string
	// ArmorUntrained says it wears armor or a shield it is not proficient with: disadvantage
	// on the checks, saves and weapon attacks that use Strength or Dexterity (SRD 5.1, "Armor").
	ArmorUntrained bool
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
	// NoLongRangeDisadvantage says the attacker's own feature takes away the
	// disadvantage of attacking beyond the normal range, up to the long range (a table
	// feat's ranged_attack effect, for weapon attacks): the caller sets it only for a
	// weapon attack. Beyond the long range the attack is still refused.
	NoLongRangeDisadvantage bool
	// StrengthOrDexterityWeapon says it is a weapon attack made with Strength or Dexterity:
	// the attacks armor without proficiency spoils. A spell attack is made with the casting
	// ability, so it is not one.
	StrengthOrDexterityWeapon bool
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
// and Targets", "Ranged Attacks", the spell Vicious Mockery). The app does not keep
// who a Frightened creature fears, so Frightened always counts as the source being in
// sight.
func AttackMode(s AttackScene) AttackModeResult {
	within, known := s.withinFive()
	var out AttackModeResult
	out.Sources = conditionSources(s)
	out.Sources = append(out.Sources, proneTargetSources(s, within, known)...)
	out.CriticalOnHit = known && within && (s.Target.Has(conditionParalyzed) || s.Target.Has(conditionUnconscious))
	out.Sources = appendRules(out.Sources, sceneRules(s))
	out.Sources = append(out.Sources, rangedSources(s)...)
	out.Sources = append(out.Sources, attackerEffectSources(s)...)
	return out
}

// sourceRule is a source that applies when on is true.
type sourceRule struct {
	on        bool
	kind      string
	effect    RollMode
	condition string
}

// appendRules appends the sources of the rules that apply, in order.
func appendRules(out []Source, rs []sourceRule) []Source {
	for _, r := range rs {
		if r.on {
			out = append(out, Source{Kind: r.kind, Effect: r.effect, Condition: r.condition})
		}
	}
	return out
}

// appendConditions appends the sources of the creature's conditions, in the
// order of the table.
func appendConditions(out []Source, c Creature, table []conditionSource) []Source {
	for _, r := range table {
		if c.Has(r.condition) {
			out = append(out, Source{Kind: r.kind, Effect: r.effect, Condition: r.condition})
		}
	}
	return out
}

// conditionSources are the sources of the conditions of the attacker and of the
// target, of the attacker's exhaustion and of Faerie Fire's outline.
func conditionSources(s AttackScene) []Source {
	out := appendConditions(nil, s.Attacker, attackerConditionSources)
	out = appendConditions(out, s.Target, targetConditionSources)
	if s.Attacker.Exhaustion >= exhaustionAttackLevel {
		out = append(out, Source{Kind: SourceExhaustionAttack, Effect: ModeDisadvantage})
	}
	if s.Target.Outlined {
		// Faerie Fire: the target gets nothing from being invisible, and an attacker that
		// sees it has advantage.
		out = slices.DeleteFunc(out, func(src Source) bool { return src.Kind == SourceInvisibleTarget })
		if !s.TargetUnseen {
			out = append(out, Source{Kind: SourceOutlinedTarget, Effect: ModeAdvantage})
		}
	}
	return out
}

// proneTargetSources is the source of a prone target, when the distance is
// known: advantage within 5 ft, disadvantage beyond.
func proneTargetSources(s AttackScene, within, known bool) []Source {
	if !s.Target.Has(conditionProne) || !known {
		return nil
	}
	effect := ModeDisadvantage
	if within {
		effect = ModeAdvantage
	}
	return []Source{{Kind: SourceProneTarget, Effect: effect, Condition: conditionProne}}
}

// sceneRules are the sources of the features and the scene, in the order
// they are listed: Reckless Attack, Dodge (the target must see the
// attacker), Pack Tactics, Unseen Attackers and Targets (an invisible
// creature is listed by its condition) and Help (SRD 5.1, "Help").
func sceneRules(s AttackScene) []sourceRule {
	targetSees := !s.AttackerUnseen && !s.Attacker.Has(conditionInvisible) && !s.Target.Has(conditionBlinded)
	return []sourceRule{
		{s.Attacker.Reckless && s.StrengthMelee && s.OwnTurn, SourceRecklessAttacker, ModeAdvantage, ""},
		{s.Target.Reckless, SourceRecklessTarget, ModeAdvantage, ""},
		{s.Target.dodgeHolds() && targetSees, SourceDodgingTarget, ModeDisadvantage, ""},
		{s.Attacker.PackTactics && s.AllyNearTarget, SourcePackTactics, ModeAdvantage, ""},
		{s.AttackerUnseen && !s.Attacker.Has(conditionInvisible), SourceUnseenAttacker, ModeAdvantage, ""},
		{s.TargetUnseen && !s.Target.Has(conditionInvisible), SourceUnseenTarget, ModeDisadvantage, ""},
		{s.Helped, SourceHelp, ModeAdvantage, ""},
		{s.Attacker.ArmorUntrained && s.StrengthOrDexterityWeapon, SourceArmorUntrained, ModeDisadvantage, ""},
	}
}

// attackerEffectSources are the disadvantages an effect that lasts gives the attacker's own
// attack rolls (SRD 5.1, Vicious Mockery: the next attack roll the creature makes before the end
// of its next turn). The effect is the source, keyed by its own key.
func attackerEffectSources(s AttackScene) []Source {
	if s.Attacker.AttackDisadvantage == "" {
		return nil
	}
	return []Source{{Kind: SourceEffectAttack, Effect: ModeDisadvantage, Condition: s.Attacker.AttackDisadvantage}}
}

// rangedSources are the disadvantages of a ranged attack: beyond the normal range, and
// with a hostile creature within 5 ft of the attacker (SRD 5.1, "Ranged Attacks").
func rangedSources(s AttackScene) []Source {
	var out []Source
	if !s.Ranged {
		return out
	}
	if !s.NoLongRangeDisadvantage && s.DistanceKnown && s.ReachFt > 0 && s.DistanceFt > s.ReachFt && (s.LongRangeFt == 0 || s.DistanceFt <= s.LongRangeFt) {
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
	// StealthArmor says the check is Dexterity (Stealth) in armor that gives disadvantage
	// on it (SRD 5.1, "Armor").
	StealthArmor bool
}

// SaveMode lists the circumstances of a saving throw or an ability check (SRD
// 5.1, Conditions, "Dodge", Barbarian "Rage" and "Danger Sense").
func SaveMode(s SaveScene) []Source {
	rs := dexSaveRules(s)
	rs = append(rs, sourceRule{s.Ability == strengthAbility && s.Creature.Raging, SourceRageStrength, ModeAdvantage, ""})
	rs = append(rs, sourceRule{s.Creature.ArmorUntrained && physicalAbility(s.Ability), SourceArmorUntrained, ModeDisadvantage, ""})
	rs = append(rs, saveRules(s)...)
	rs = append(rs, checkRules(s)...)
	return appendRules(nil, rs)
}

// physicalAbility says the ability is Strength or Dexterity.
func physicalAbility(ability string) bool {
	return ability == strengthAbility || ability == dexterityAbility
}

// dexSaveRules are the sources of a Dexterity saving throw: Dodge, Danger
// Sense and Restrained. None applies to a check or to another save.
func dexSaveRules(s SaveScene) []sourceRule {
	if s.Check || s.Ability != dexterityAbility {
		return nil
	}
	c := s.Creature
	return []sourceRule{
		{c.dodgeHolds(), SourceDodgingSave, ModeAdvantage, ""},
		{dangerSenseHolds(s), SourceDangerSense, ModeAdvantage, ""},
		{c.Has(conditionRestrained), SourceRestrainedSave, ModeDisadvantage, conditionRestrained},
	}
}

// saveRules are the sources of any saving throw: an effect's advantage and
// exhaustion. None applies to a check.
func saveRules(s SaveScene) []sourceRule {
	if s.Check {
		return nil
	}
	c := s.Creature
	return []sourceRule{
		{slices.Contains(c.SaveAdvantage, s.Ability), SourceEffectSave, ModeAdvantage, ""},
		{c.Exhaustion >= exhaustionAttackLevel, SourceExhaustionSave, ModeDisadvantage, ""},
	}
}

// dangerSenseHolds says Danger Sense gives its advantage: the creature sees
// the effect, and is neither blinded, deafened nor incapacitated.
func dangerSenseHolds(s SaveScene) bool {
	c := s.Creature
	return c.DangerSense && s.EffectVisible && !c.Has(conditionBlinded) && !c.Has(conditionDeafened) && !c.Incapacitated()
}

// checkRules are the sources of an ability check that come after Rage. None
// applies to a saving throw.
func checkRules(s SaveScene) []sourceRule {
	if !s.Check {
		return nil
	}
	c := s.Creature
	return []sourceRule{
		{slices.Contains(c.CheckAdvantage, s.Ability), SourceEffectCheck, ModeAdvantage, ""},
		{c.Exhaustion >= exhaustionCheckLevel, SourceExhaustionCheck, ModeDisadvantage, ""},
		{s.StealthArmor && s.Ability == dexterityAbility, SourceStealthArmor, ModeDisadvantage, ""},
		{c.Has(conditionPoisoned), SourcePoisonedCheck, ModeDisadvantage, conditionPoisoned},
		{c.Has(conditionFrightened), SourceFrightenedCheck, ModeDisadvantage, conditionFrightened},
	}
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

// exhaustionAttackLevel is the level of exhaustion that gives disadvantage on attack rolls
// and saving throws (SRD, Conditions: level 3); level 1 gives it on ability checks.
const exhaustionAttackLevel = 3

// exhaustionCheckLevel is the level of exhaustion that gives disadvantage on ability
// checks (SRD, Conditions: level 1).
const exhaustionCheckLevel = 1

// pairOfDice is the d20 a roll with advantage or disadvantage takes.
const pairOfDice = 2
