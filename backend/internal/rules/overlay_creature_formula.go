package rules

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The numbers of a table creature as formulas of its owner. A table creature keeps
// its static adjustments (overlay_creature.go); on top of them, each number may be a formula that reads
// the owner (level(), classLevel("x"), prof(), mod("wis")) and the circle of the slot the owner cast with
// (slot(), 0 outside a slot cast). The formulas are compiled once, when the overlay is built, and kept on
// the creature's entry; ResolveCreature runs them for an owner and ApplyResolved writes the result on the
// stat block. Nothing here knows how a creature is given or summoned: that is up to the caller.
//
// slot() is legal only in these fields (the compiler is made with formula.Compiler.WithSlot): effects,
// conditions and every other formula of the content refuse it.

// The limits of the formula fields and of the creature's own attacks.
const (
	// MaxCreatureAttacks is the most attacks one creature adds.
	MaxCreatureAttacks = 8
	// MinCreatureAttackBonus and MaxCreatureAttackBonus bound the to-hit a formula gives.
	MinCreatureAttackBonus = -10
	MaxCreatureAttackBonus = 30
	// The bounds of a creature's own attack.
	maxCreatureReachFt    = 30
	maxCreatureRangeFt    = 600
	maxCreatureLongFt     = 1200
	maxCreatureAttackDice = 20
)

// TableCreatureAttack is an attack action a table creature adds to its base's. It becomes an action keyed
// "<creature key>#<slug of the name>" with an attack roll, as the SRD stat blocks' attacks are. A melee
// attack has a reach (0 is 5 ft.) and no range; a ranged one has a range, a long range (0 for none) and no
// reach. ToHit and DamageBonus are formulas, and the specific beats the general: an attack's own ToHit wins
// over the creature's AttackBonusFormula, and its own DamageBonus over the creature's damage bonus. An
// attack with no ToHit of its own gets the creature's attack bonus, else +0; one with no DamageBonus gets
// the creature's damage bonus. It is not part of the base's Multiattack.
type TableCreatureAttack struct {
	NamePT                         string
	Melee                          bool
	ReachFt, RangeFt, LongRangeFt  int
	DiceCount, DiceSides           int
	DamageBonus, DamageType, ToHit string
}

// creatureSpec is a table creature as the content keeps it: the static adjustments and the compiled
// formulas. It is immutable once built.
type creatureSpec struct {
	key, namePT, baseKey       string
	hp, ac, damageBonus        int
	traits                     []TableCreatureTrait
	hpP, acP, damageP, attackP *formula.Program
	attacks                    []attackSpec
}

type attackSpec struct {
	def             TableCreatureAttack
	toHitP, damageP *formula.Program
}

// ResolvedCreature is a table creature's numbers for one owner: every formula run and clamped.
// HitPoints and ArmorClass are 0 when the base's stand.
// DamageBonus is the bonus the creature-level formula or the static field gives each attack with no bonus of its own.
// AttackBonus replaces the base's only when HasAttackBonus.
// AttackBonus is also the to-hit of an own attack with no to_hit of its own.
// Attacks are the creature's own, with their final numbers.
// Failed lists the fields (as ".hit_points_formula") whose formula failed to run, which then keep the static value.
type ResolvedCreature struct {
	Key, NamePT, BaseKey string
	HitPoints            int
	ArmorClass           int
	DamageBonus          int
	AttackBonus          int
	HasAttackBonus       bool
	Traits               []TableCreatureTrait
	Attacks              []ResolvedAttack
	Failed               []string
}

// ResolvedAttack is a TableCreatureAttack with its to-hit and damage bonus worked out.
// Key is the key of the attack's action, "<creature key>#<slug of the name>": what a snapshot is matched by.
type ResolvedAttack struct {
	TableCreatureAttack
	Key                           string
	AttackBonus, DamageBonusValue int
}

// OwnerEnv is the formula environment of an owner whose sheet is d: its level, class levels, ability scores
// and modifiers, proficiency bonus, armor and walking speed. shield says whether the owner carries a
// shield that counts (the sheet does not say, so the caller does; it is what shield() answers). It has no
// slot; ResolveCreature sets the slot.
func OwnerEnv(d Derived, shield bool) *formula.Env {
	levels := map[string]int{}
	for _, cl := range d.Classes {
		levels[strings.TrimPrefix(cl.ClassKey, "class:")] += cl.Level
	}
	mods, scores := map[Ability]int{}, map[Ability]int{}
	for _, a := range d.Abilities {
		mods[a.Ability], scores[a.Ability] = a.Modifier, a.Score
	}
	return &formula.Env{
		Level:      func() int { return d.TotalLevel },
		ClassLevel: func(class string) int { return levels[class] },
		Mod:        func(a string) int { return mods[Ability(a)] },
		Score:      func(a string) int { return scores[Ability(a)] },
		Prof:       func() int { return d.ProficiencyBonus },
		Armor:      func() string { return d.ArmorCategory },
		Shield:     func() bool { return shield },
		SpeedWalk:  func() int { return d.SpeedWalkFt },
	}
}

// ResolveCreature works out the numbers of the table creature key for the owner (env: OwnerEnv, or the
// environment the feature formulas run on) that cast it with a slot of the given circle (0 outside a slot
// cast; out of 0 to 9 is 0). Each formula is run and clamped: hit points 1 to MaxCreatureHitPoints, armor
// class 1 to MaxCreatureArmorClass, damage bonus MinCreatureDamageBonus to MaxCreatureDamageBonus, attack
// bonus MinCreatureAttackBonus to MaxCreatureAttackBonus. A result beyond the formula engine's limit
// (formula.MaxResult) is a failed formula.
// A field with no formula keeps the static value. ok is false for a key that is not a table creature, or a
// nil env. It reads the content only.
func (c *Content) ResolveCreature(key string, env *formula.Env, slot int) (ResolvedCreature, bool) {
	s, ok := c.c.creatureSpecs[key]
	if !ok || env == nil {
		return ResolvedCreature{}, false
	}
	return s.resolve(env, slot, false), true
}

// IsTableCreature says whether key is a creature the table added.
func (c *Content) IsTableCreature(key string) bool {
	_, ok := c.c.creatureSpecs[key]
	return ok
}

// CreatureHasFormulas says whether key is a table creature with a formula in any number or attack: the creature
// whose numbers depend on its owner. A table creature with none, and an SRD creature, have the same stat block
// for everyone.
func (c *Content) CreatureHasFormulas(key string) bool {
	s, ok := c.c.creatureSpecs[key]
	if !ok {
		return false
	}
	if s.hpP != nil || s.acP != nil || s.damageP != nil || s.attackP != nil {
		return true
	}
	return slices.ContainsFunc(s.attacks, func(a attackSpec) bool { return a.toHitP != nil || a.damageP != nil })
}

// ResolvedDerived is the stat block of the table creature r for combat: MonsterDerived and the plan of its
// actions, with r applied (ApplyResolved). What r says about the creature itself (its base, name, traits and
// the shape of its attacks) is ignored: it comes from the content's entry for r.Key, and only r's numbers are
// used, clamped to their ranges. The own attacks of r are matched to the entry's by their key: an attack in
// both keeps the numbers of r, one new in the entry has the static numbers, and one the entry lost is dropped.
// ok is false when r.Key is not a table creature or the stat block cannot be built.
func (c *Content) ResolvedDerived(r ResolvedCreature) (Derived, *MonsterPlan, bool) {
	s, table := c.c.creatureSpecs[r.Key]
	if !table {
		return Derived{}, nil, false
	}
	base, ok := c.c.monsters[s.baseKey]
	if !ok {
		return Derived{}, nil, false
	}
	m, ok := ApplyResolved(base, s.trusted(r))
	if !ok {
		return Derived{}, nil, false
	}
	plan, err := c.c.planOf(m)
	if err != nil {
		return Derived{}, nil, false
	}
	return c.c.monsterDerived(m), plan, true
}

// trusted is r with the identity and the attack shapes of the entry and the numbers of r, clamped.
func (s *creatureSpec) trusted(r ResolvedCreature) ResolvedCreature {
	t := ResolvedCreature{
		Key: s.key, NamePT: s.namePT, BaseKey: s.baseKey, Traits: slices.Clone(s.traits),
		HitPoints: clampOrZero(r.HitPoints, 1, MaxCreatureHitPoints), ArmorClass: clampOrZero(r.ArmorClass, 1, MaxCreatureArmorClass),
		DamageBonus:    clamp(r.DamageBonus, MinCreatureDamageBonus, MaxCreatureDamageBonus),
		HasAttackBonus: r.HasAttackBonus,
	}
	if t.HasAttackBonus {
		t.AttackBonus = clamp(r.AttackBonus, MinCreatureAttackBonus, MaxCreatureAttackBonus)
	}
	own := map[string]ResolvedAttack{}
	for _, a := range r.Attacks {
		if _, dup := own[a.Key]; !dup {
			own[a.Key] = a
		}
	}
	static := s.resolve(nil, 0, true).Attacks
	for i, a := range s.attacks {
		ra := static[i]
		if got, ok := own[ra.Key]; ok {
			ra.AttackBonus = clamp(got.AttackBonus, MinCreatureAttackBonus, MaxCreatureAttackBonus)
			ra.DamageBonusValue = clamp(got.DamageBonusValue, MinCreatureDamageBonus, MaxCreatureDamageBonus)
		}
		ra.TableCreatureAttack = a.def
		t.Attacks = append(t.Attacks, ra)
	}
	return t
}

// attackKey is the key of the action of the own attack called name on the creature key.
func attackKey(key, name string) string { return key + "#" + slugOf(name) }

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }

// clampOrZero is clamp, except that 0 stays 0 (the base's value).
func clampOrZero(v, lo, hi int) int {
	if v == 0 {
		return 0
	}
	return clamp(v, lo, hi)
}

// formulaRun runs the programs of one resolve for one owner, keeping the fields whose formula failed.
type formulaRun struct {
	env    formula.Env
	failed []string
}

// int runs p and clamps the result to lo..hi; ok is false when there is no formula or it failed.
func (f *formulaRun) int(p *formula.Program, field string, lo, hi int) (int, bool) {
	if p == nil {
		return 0, false
	}
	v, err := p.Int(&f.env)
	if err != nil {
		f.failed = append(f.failed, field)
		return 0, false
	}
	return clamp(v, lo, hi), true
}

// resolve runs the spec's formulas. With ownerless no formula is run: the static fields stand, and the own
// attacks have +0 to hit and the static damage bonus.
func (s *creatureSpec) resolve(env *formula.Env, slot int, ownerless bool) ResolvedCreature {
	r := ResolvedCreature{
		Key: s.key, NamePT: s.namePT, BaseKey: s.baseKey, Traits: slices.Clone(s.traits),
		HitPoints: s.hp, ArmorClass: s.ac, DamageBonus: s.damageBonus,
	}
	if ownerless {
		env = &formula.Env{}
	}
	circle := slot
	if circle < 0 || circle > MaxSlotLevel {
		circle = 0
	}
	f := &formulaRun{env: *env}
	f.env.Slot = func() int { return circle }
	if !ownerless {
		s.resolveNumbers(f, &r)
	}
	s.resolveAttacks(f, &r, ownerless)
	r.Failed = f.failed
	return r
}

// resolveNumbers runs the creature-level formulas; a field with no formula, or a failed one, keeps r's value.
func (s *creatureSpec) resolveNumbers(f *formulaRun, r *ResolvedCreature) {
	if v, ok := f.int(s.hpP, ".hit_points_formula", 1, MaxCreatureHitPoints); ok {
		r.HitPoints = v
	}
	if v, ok := f.int(s.acP, ".armor_class_formula", 1, MaxCreatureArmorClass); ok {
		r.ArmorClass = v
	}
	if v, ok := f.int(s.damageP, ".damage_bonus_formula", MinCreatureDamageBonus, MaxCreatureDamageBonus); ok {
		r.DamageBonus = v
	}
	r.AttackBonus, r.HasAttackBonus = f.int(s.attackP, ".attack_bonus_formula", MinCreatureAttackBonus, MaxCreatureAttackBonus)
}

// resolveAttacks works out the creature's own attacks, the specific beating the general: the to-hit is the
// attack's own formula, else the creature's attack bonus, else 0; the damage bonus is the attack's own
// formula, else the creature's. Without an owner no formula is run.
func (s *creatureSpec) resolveAttacks(f *formulaRun, r *ResolvedCreature, ownerless bool) {
	for i, a := range s.attacks {
		ra := ResolvedAttack{TableCreatureAttack: a.def, Key: attackKey(s.key, a.def.NamePT), AttackBonus: r.AttackBonus, DamageBonusValue: r.DamageBonus}
		if !ownerless {
			if v, ok := f.int(a.toHitP, fmt.Sprintf(".attacks[%d].to_hit", i), MinCreatureAttackBonus, MaxCreatureAttackBonus); ok {
				ra.AttackBonus = v
			}
			if v, ok := f.int(a.damageP, fmt.Sprintf(".attacks[%d].damage_bonus", i), MinCreatureDamageBonus, MaxCreatureDamageBonus); ok {
				ra.DamageBonusValue = v
			}
		}
		r.Attacks = append(r.Attacks, ra)
	}
}

var toHitTextRe = regexp.MustCompile(`[+-]\d+ to hit`)

// ApplyResolved is the stat block of base with r written on it: the key and name of the table creature; hit
// points (the roll is cleared); armor class (its kind and notes are cleared); the damage bonus added once
// to the first damage part of each attack action of the base; the attack bonus in place of the base's; the
// extra traits after the base's; and the creature's own attacks after its actions, each an action with an
// attack roll. The text of an action is the base's, except for the to-hit it states. base is not changed.
// With the static values, it is the stat block the content keeps.
// ok is false when an attack of r has dice that cannot be read.
// The to-hit rewrite matches "+N to hit"; other wordings keep the base's text.
func ApplyResolved(base *srd51.Monster, r ResolvedCreature) (*srd51.Monster, bool) {
	m := *base
	m.Key, m.Name = r.Key, r.NamePT
	if r.HitPoints > 0 {
		m.HitPoints, m.HitPointsRoll = r.HitPoints, ""
	}
	if r.ArmorClass > 0 {
		// The base's kind of armor class ("natural") and its notes no longer describe the value.
		m.ArmorClass, m.ArmorClassType, m.ArmorClassDesc, m.ArmorClassItems, m.ArmorClassAlts = r.ArmorClass, "", "", nil, nil
	}
	if r.DamageBonus != 0 || r.HasAttackBonus || len(r.Attacks) > 0 {
		m.Actions = slices.Clone(base.Actions)
		for i := range m.Actions {
			applyToAction(&m.Actions[i], r)
		}
		for _, a := range r.Attacks {
			act, ok := attackAction(a)
			if !ok {
				return nil, false
			}
			m.Actions = append(m.Actions, act)
		}
	}
	if len(r.Traits) > 0 {
		m.SpecialAbilities = slices.Clone(base.SpecialAbilities)
		for _, t := range r.Traits {
			m.SpecialAbilities = append(m.SpecialAbilities, srd51.MonsterAbility{Name: t.NamePT, Desc: t.TextPT})
		}
	}
	return &m, true
}

// applyToAction writes the bonuses of r on one action of the base (a copy of it).
func applyToAction(a *srd51.MonsterAction, r ResolvedCreature) {
	if a.HasAttack {
		if r.HasAttackBonus {
			a.AttackBonus = r.AttackBonus
			a.Desc = toHitTextRe.ReplaceAllString(a.Desc, fmt.Sprintf("%+d to hit", r.AttackBonus))
		}
		if r.DamageBonus != 0 && len(a.Damage) > 0 {
			a.Damage = slices.Clone(a.Damage)
			a.Damage[0].Dice = addDiceBonus(a.Damage[0].Dice, r.DamageBonus)
		}
	}
}

// attackAction is the action of a creature's own attack, worded as the SRD words its attacks so the plan
// and the sheet read it the same way.
func attackAction(a ResolvedAttack) (srd51.MonsterAction, bool) {
	dice := addDiceBonus(fmt.Sprintf("%dd%d", a.DiceCount, a.DiceSides), a.DamageBonusValue)
	f, ok := ParseDice(dice)
	if !ok {
		return srd51.MonsterAction{}, false
	}
	avg := f.Count*(f.Sides+1)/2 + f.Bonus
	typ := strings.TrimPrefix(a.DamageType, "damage-type:")
	var head string
	if a.Melee {
		head = fmt.Sprintf("Melee Weapon Attack: %+d to hit, reach %d ft., one target.", a.AttackBonus, cmp.Or(a.ReachFt, unarmedReachFt))
	} else {
		rng := strconv.Itoa(a.RangeFt)
		if a.LongRangeFt > 0 {
			rng += "/" + strconv.Itoa(a.LongRangeFt)
		}
		head = fmt.Sprintf("Ranged Weapon Attack: %+d to hit, range %s ft., one target.", a.AttackBonus, rng)
	}
	return srd51.MonsterAction{
		Name: a.NamePT, HasAttack: true, AttackBonus: a.AttackBonus,
		Desc:   fmt.Sprintf("%s Hit: %d (%s) %s damage.", head, max(avg, 0), dice, typ),
		Damage: []srd51.MonsterDamage{{Dice: dice, DamageType: a.DamageType}},
	}, true
}

// checkCreatureFormulas checks the formula fields and the creature's own attacks, compiles the formulas
// (slot() is legal) and returns the creature's spec. Every violation goes to c with its field path.
func (b *overlayBuilder) checkCreatureFormulas(c *entryErrors, o *TableCreature, used map[string]bool, path string) *creatureSpec {
	s := &creatureSpec{
		key: o.Key, namePT: o.NamePT, baseKey: o.BaseKey,
		hp: o.HitPoints, ac: o.ArmorClass, damageBonus: o.DamageBonus, traits: slices.Clone(o.Traits),
	}
	compile := func(src, attr string) *formula.Program {
		if src == "" {
			return nil
		}
		p, err := b.n.compileSlotted(src)
		if err != nil {
			c.at(o.Key, path, attr, ReasonFormula, "the formula does not work: %v", err)
			return nil
		}
		return p
	}
	s.hpP = compile(o.HitPointsFormula, ".hit_points_formula")
	s.acP = compile(o.ArmorClassFormula, ".armor_class_formula")
	s.damageP = compile(o.DamageBonusFormula, ".damage_bonus_formula")
	s.attackP = compile(o.AttackBonusFormula, ".attack_bonus_formula")
	if len(o.Attacks) > MaxCreatureAttacks {
		c.at(o.Key, path, ".attacks", ReasonLimit, "%d attacks; the limit is %d per creature", len(o.Attacks), MaxCreatureAttacks)
		return s
	}
	for i := range o.Attacks {
		at := fmt.Sprintf(".attacks[%d]", i)
		a := &o.Attacks[i]
		b.checkCreatureAttack(c, o.Key, a, used, path, at)
		s.attacks = append(s.attacks, attackSpec{
			def:     *a,
			toHitP:  compile(a.ToHit, at+".to_hit"),
			damageP: compile(a.DamageBonus, at+".damage_bonus"),
		})
	}
	return s
}

// checkCreatureAttack checks the name, the shape (reach or range), the dice and the damage type of one attack.
func (b *overlayBuilder) checkCreatureAttack(c *entryErrors, key string, a *TableCreatureAttack, used map[string]bool, path, at string) {
	switch err := checkEntryName(key, a.NamePT); {
	case err != nil:
		c.at(key, path, at+".name_pt", ReasonName, "the name must be 1 to %d characters on one line, without spaces at the ends", maxNameRunes)
	case slugOf(a.NamePT) == "":
		c.at(key, path, at+".name_pt", ReasonName, "the name must have a letter or a digit from a to z")
	case used[slugOf(a.NamePT)]:
		c.at(key, path, at+".name_pt", ReasonName, "the name is already a trait or an action of the creature")
	default:
		used[slugOf(a.NamePT)] = true
	}
	checkAttackShape(c, key, a, path, at)
	if a.DiceCount < 1 || a.DiceCount > maxCreatureAttackDice {
		c.at(key, path, at+".dice_count", ReasonValue, "the number of dice is 1 to %d", maxCreatureAttackDice)
	}
	if !slices.Contains(tableDiceSides, a.DiceSides) {
		c.at(key, path, at+".dice_sides", ReasonValue, "the dice have %v sides", tableDiceSides)
	}
	if dt, found := b.base.named[a.DamageType]; !found || dt == nil || !strings.HasPrefix(a.DamageType, "damage-type:") {
		c.at(key, path, at+".damage_type", ReasonReference, "damage type %q is not in the SRD", truncateRunes(a.DamageType, 40))
	}
}

// checkAttackShape checks the reach of a melee attack or the range of a ranged one.
func checkAttackShape(c *entryErrors, key string, a *TableCreatureAttack, path, at string) {
	if a.Melee {
		if a.ReachFt < 0 || a.ReachFt > maxCreatureReachFt {
			c.at(key, path, at+".reach_ft", ReasonValue, "the reach is 0 (5 ft.) to %d ft.", maxCreatureReachFt)
		}
		if a.RangeFt != 0 || a.LongRangeFt != 0 {
			c.at(key, path, at+".range_ft", ReasonValue, "a melee attack has a reach, not a range")
		}
	} else {
		if a.RangeFt < 1 || a.RangeFt > maxCreatureRangeFt {
			c.at(key, path, at+".range_ft", ReasonValue, "the range of a ranged attack is 1 to %d ft.", maxCreatureRangeFt)
		}
		if a.LongRangeFt != 0 && (a.LongRangeFt <= a.RangeFt || a.LongRangeFt > maxCreatureLongFt) {
			c.at(key, path, at+".long_range_ft", ReasonValue, "the long range is 0 (none) or more than the range, up to %d ft.", maxCreatureLongFt)
		}
		if a.ReachFt != 0 {
			c.at(key, path, at+".reach_ft", ReasonValue, "a ranged attack has a range, not a reach")
		}
	}
}

// truncateRunes is s cut to n characters, with "..." when it was longer.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
