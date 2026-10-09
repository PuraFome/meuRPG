package rules

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// Effects that last in a combat (RN-22, SRD 5.1: "Conditions", "Duration",
// "Concentration", the spells Bless, Bane, Haste, Hold Person and Web, and the
// Dodge action). effects/combat_effects.json is handwritten and checked by the
// loader: what each of those spells and actions puts on a creature, for how
// long, how it ends, and what the card tells the player. The play module reads
// it (Content.CombatSpellEffect, CombatEffect, ConditionInfo); the clock, the
// exhaustion table and the conditions' arithmetic are pure and live in
// rules/combat.

// The kinds of duration of a combat effect.
const (
	// EffectDurationRounds lasts a number of rounds, ending at the start of the
	// turn of its anchor (the caster's) that many rounds later: ten rounds for the
	// SRD's "1 minute", counted from the round of the casting.
	EffectDurationRounds = "rounds"
	// EffectDurationUntilStartOfTurnOf ends at the start of a combatant's next turn.
	EffectDurationUntilStartOfTurnOf = "until_start_of_turn_of"
	// EffectDurationUntilEndOfTurnOf ends at the end of a combatant's next turn.
	EffectDurationUntilEndOfTurnOf = "until_end_of_turn_of"
	// EffectDurationConcentration lasts while the caster keeps concentrating, with
	// no time limit of its own.
	EffectDurationConcentration = "concentration"
	// EffectDurationUntilDismissed lasts until the master ends it.
	EffectDurationUntilDismissed = "until_dismissed"
	// EffectDurationLongRest lasts until a long rest.
	EffectDurationLongRest = "long_rest"
)

// RoundsPerMinute is the SRD's length of a minute in rounds: a round is about
// 6 seconds ("Combat"), so a minute is 10 rounds.
const RoundsPerMinute = secondsPerMinute / SecondsPerRound

// The kinds of modifier an effect gives.
const (
	// ModifierRollDie adds (Sign +1) or subtracts (Sign -1) a die to the roll.
	ModifierRollDie = "roll_die"
	// ModifierACBonus adds Value to the armor class.
	ModifierACBonus = "ac_bonus"
	// ModifierSpeedMultiplier multiplies the speed by Value percent.
	ModifierSpeedMultiplier = "speed_multiplier"
	// ModifierSaveAdvantage gives advantage on the saving throws of Abilities.
	ModifierSaveAdvantage = "save_advantage"
	// ModifierExtraAction gives one more action each turn, limited to Allowed.
	ModifierExtraAction = "extra_action"
	// ModifierNoMove stops the creature from moving.
	ModifierNoMove = "no_move"
	// ModifierNoAction stops the creature from taking actions.
	ModifierNoAction = "no_action"
)

// The rolls a ModifierRollDie applies to.
const (
	RollAppliesAttack = "attack"
	RollAppliesSave   = "save"
	RollAppliesCheck  = "check"
)

// The audience values of an effect's visibility: who of the players reads it.
const (
	// EffectVisibilityPublic effects show to every player (the label in the order,
	// the condition on the combatant); the other kind shows to the target's player
	// and the master only.
	EffectVisibilityPublic = "public"
	EffectVisibilityOwner  = "owner"
)

// maxPlayerLabel is the length of the free label the master may put on an effect.
const maxPlayerLabel = 30

// EffectModifier is one change an effect makes to a creature's numbers.
type EffectModifier struct {
	Kind string `json:"kind"`
	// Die and Sign are for ModifierRollDie: a d4 added (+1) or subtracted (-1);
	// AppliesTo says to which rolls.
	Die       int      `json:"die,omitempty"`
	Sign      int      `json:"sign,omitempty"`
	AppliesTo []string `json:"applies_to,omitempty"`
	// Value is the armor class bonus, or the speed multiplier in percent.
	Value int `json:"value,omitempty"`
	// Abilities are the ability keys of ModifierSaveAdvantage.
	Abilities []string `json:"abilities,omitempty"`
	// Allowed are the standard actions an extra action may be ("attack",
	// "dash"...), and MaxWeaponAttacks how many weapon attacks that action makes.
	Allowed          []string `json:"allowed,omitempty"`
	MaxWeaponAttacks int      `json:"max_weapon_attacks,omitempty"`
}

// EffectDuration is how long an effect lasts.
type EffectDuration struct {
	Kind   string `json:"kind"`
	Rounds int    `json:"rounds,omitempty"`
}

// EffectSave is the saving throw an effect asks again: at the end of the
// target's turn (Hold Person) or at the start of it (Web). The DC is the
// caster's, kept on the effect when it is cast.
type EffectSave struct {
	Ability string `json:"ability"`
	// OnPass is what a passed save does: "end" ends the effect on the target. A
	// start-of-turn save that fails adds OnFail, an effect key.
	OnPass string `json:"on_pass,omitempty"`
	OnFail string `json:"on_fail,omitempty"`
	// OnDamage says the save is asked again each time the target takes damage, with
	// advantage (Hideous Laughter).
	OnDamage bool `json:"on_damage,omitempty"`
}

// EffectTrigger is damage an effect deals to a creature that starts its turn
// under it, with no saving throw (the Web that burns).
type EffectTrigger struct {
	Dice        string `json:"dice"`
	DamageType  string `json:"damage_type"`
	MaxTriggers int    `json:"max_triggers"`
}

// EffectDef is what an effect of the catalog is: a spell's, an action's or one
// the app adds (the lethargy a Haste leaves, the web caught in fire).
type EffectDef struct {
	// Key is the effect's key: a spell's ("spell:bless") or "effect:<name>".
	Key string `json:"-"`
	// NamePT is its Portuguese name (effects/names_pt.json).
	NamePT string `json:"-"`
	// Kind is the source: "spell", "feature" or "master".
	Kind string `json:"kind,omitempty"`
	// Duration is the effect's own; a spell's comes from the spell (Content.SpellDetails).
	Duration *EffectDuration `json:"duration,omitempty"`
	// Concentration is read from the spell; it is not in the file.
	Concentration bool `json:"-"`
	// Conditions are the condition keys it gives its targets.
	Conditions []string         `json:"conditions,omitempty"`
	Modifiers  []EffectModifier `json:"modifiers,omitempty"`
	// EndSave is the save at the end of the target's turn; StartSave the one at the
	// start of it.
	EndSave   *EffectSave    `json:"end_save,omitempty"`
	StartSave *EffectSave    `json:"start_save,omitempty"`
	Trigger   *EffectTrigger `json:"turn_trigger,omitempty"`
	// Applies says which targets get the effect when a spell is cast: "all" (Bless,
	// Haste), or "failed_save" (Bane, Hold Person: the ones that failed the save the
	// spell asks).
	Applies string `json:"applies,omitempty"`
	// TargetType limits the targets to a creature type ("humanoid": Hold Person).
	TargetType string `json:"target_type,omitempty"`
	// OnEnd is the effect that follows when this one ends (Haste's lethargy).
	OnEnd string `json:"on_end,omitempty"`
	// EndsEffectsOf are the effect keys that end on the target when this one ends
	// (the web burns away, and nobody is held by it).
	EndsEffectsOf []string `json:"ends_effects_of,omitempty"`
	// BreakFree is the ability of the check that frees a creature the effect holds
	// (Web: a Strength check against the caster's DC, as an action).
	BreakFree string `json:"break_free,omitempty"`
	// PlayerLabelPT is the label the other players read on the target when the
	// effect is visible ("Preso numa teia"); empty: the condition's name.
	PlayerLabelPT string `json:"player_label_pt,omitempty"`
	// Visibility is the default audience: EffectVisibilityPublic or ...Owner.
	Visibility string `json:"visibility,omitempty"`
	// TagsPT are the short labels of the card ("+1d4 em ataques e resistências"),
	// and ChangesPT the lines of "O que isso muda".
	TagsPT    []string `json:"tags_pt,omitempty"`
	ChangesPT []string `json:"changes_pt,omitempty"`
}

// ConditionInfo is what a condition shows to the table: who reads it and what
// it does to the creature, in our words (SRD 5.1, "Conditions").
type ConditionInfo struct {
	Key string `json:"-"`
	// Visibility is EffectVisibilityPublic (the table sees it on the creature) or
	// EffectVisibilityOwner (only the creature's player and the master).
	Visibility string `json:"visibility"`
	// ChangesPT are the lines "O que isso muda" of the creature's own card.
	ChangesPT []string `json:"changes_pt"`
}

type combatEffectsFile struct {
	Source     string                   `json:"source"`
	Conditions map[string]ConditionInfo `json:"conditions"`
	Spells     map[string]*EffectDef    `json:"spells"`
	Effects    map[string]*EffectDef    `json:"effects"`
}

// combatEffects is the loaded file.
type combatEffects struct {
	conditions map[string]ConditionInfo
	spells     map[string]*EffectDef
	effects    map[string]*EffectDef
}

var (
	effectKinds     = []string{"spell", "feature", "master"}
	effectApplies   = []string{"all", "failed_save"}
	effectDurations = []string{EffectDurationRounds, EffectDurationUntilStartOfTurnOf, EffectDurationUntilEndOfTurnOf,
		EffectDurationConcentration, EffectDurationUntilDismissed, EffectDurationLongRest}
	modifierKinds = []string{ModifierRollDie, ModifierACBonus, ModifierSpeedMultiplier, ModifierSaveAdvantage, ModifierExtraAction, ModifierNoMove, ModifierNoAction}
	saveAbilities = []string{"str", "dex", "con", "int", "wis", "cha"}
	// extraActions are the standard actions an extra action may be: the keys of
	// effects/standard_actions.json.
	extraActions = []string{"attack", "dash", "disengage", "hide", "use-an-object"}
)

// loadCombatEffects reads and checks effects/combat_effects.json.
func (c *content) loadCombatEffects(fsys fs.FS) error {
	const name = "effects/combat_effects.json"
	var f combatEffectsFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	if strings.TrimSpace(f.Source) == "" {
		return fmt.Errorf("%s: it needs a source line", name)
	}
	out := &combatEffects{conditions: map[string]ConditionInfo{}, spells: map[string]*EffectDef{}, effects: map[string]*EffectDef{}}
	for key, info := range f.Conditions {
		if _, ok := c.named[key]; !ok {
			return fmt.Errorf("%s: %q is not an SRD condition", name, key)
		}
		if info.Visibility != EffectVisibilityPublic && info.Visibility != EffectVisibilityOwner {
			return fmt.Errorf("%s: %s: visibility must be %q or %q", name, key, EffectVisibilityPublic, EffectVisibilityOwner)
		}
		if len(info.ChangesPT) == 0 {
			return fmt.Errorf("%s: %s needs its lines in changes_pt", name, key)
		}
		info.Key = key
		out.conditions[key] = info
	}
	for key := range c.named {
		if strings.HasPrefix(key, "condition:") && out.conditions[key].Key == "" {
			return fmt.Errorf("%s: the condition %s is missing", name, key)
		}
	}
	for key, def := range f.Spells {
		sp, ok := c.spells[key]
		if !ok {
			return fmt.Errorf("%s: %q is not an SRD spell", name, key)
		}
		def.Key, def.Kind, def.NamePT = key, "spell", c.namesPT[key]
		def.Concentration = sp.Concentration
		if def.Duration != nil {
			return fmt.Errorf("%s: %s: a spell's duration comes from the spell, leave duration out", name, key)
		}
		out.spells[key] = def
	}
	for key, def := range f.Effects {
		if !strings.HasPrefix(key, "effect:") {
			return fmt.Errorf("%s: effect key %q must start with \"effect:\"", name, key)
		}
		def.Key, def.NamePT = key, c.namesPT[key]
		if def.NamePT == "" {
			return fmt.Errorf("%s: %s has no Portuguese name in effects/names_pt.json", name, key)
		}
		if def.Kind == "" {
			def.Kind = "feature"
		}
		out.effects[key] = def
	}
	for k := range c.namesPT {
		if strings.HasPrefix(k, "effect:") && out.effects[k] == nil {
			return fmt.Errorf("effects/names_pt.json names %q, which is not in combat_effects.json", k)
		}
	}
	for _, def := range allDefs(out) {
		if err := c.checkEffectDef(name, def, out); err != nil {
			return err
		}
	}
	c.combatEffects = out
	return nil
}

// allDefs lists the effects of the file, spells first, in key order.
func allDefs(f *combatEffects) []*EffectDef {
	var keys []string
	for k := range f.spells {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var effectKeys []string
	for k := range f.effects {
		effectKeys = append(effectKeys, k)
	}
	slices.Sort(effectKeys)
	var out []*EffectDef
	for _, k := range keys {
		out = append(out, f.spells[k])
	}
	for _, k := range effectKeys {
		out = append(out, f.effects[k])
	}
	return out
}

// checkEffectDef refuses what an effect of the file may not say.
func (c *content) checkEffectDef(file string, d *EffectDef, all *combatEffects) error {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%s: %s: %s", file, d.Key, fmt.Sprintf(format, a...))
	}
	if !slices.Contains(effectKinds, d.Kind) {
		return bad("kind must be spell, feature or master")
	}
	if d.Duration != nil {
		if !slices.Contains(effectDurations, d.Duration.Kind) {
			return bad("unknown duration kind %q", d.Duration.Kind)
		}
		if (d.Duration.Kind == EffectDurationRounds) != (d.Duration.Rounds > 0) {
			return bad("rounds go with the rounds duration, and only with it")
		}
	}
	if d.Applies != "" && !slices.Contains(effectApplies, d.Applies) {
		return bad("applies must be all or failed_save")
	}
	if d.Visibility != "" && d.Visibility != EffectVisibilityPublic && d.Visibility != EffectVisibilityOwner {
		return bad("unknown visibility %q", d.Visibility)
	}
	if d.TargetType != "" && d.TargetType != "humanoid" {
		return bad("the only target type limit is humanoid")
	}
	for _, k := range d.Conditions {
		if _, ok := all.conditions[k]; !ok {
			return bad("%q is not an SRD condition", k)
		}
	}
	for i, m := range d.Modifiers {
		if err := checkModifier(m); err != nil {
			return bad("modifiers[%d]: %v", i, err)
		}
	}
	for _, s := range []*EffectSave{d.EndSave, d.StartSave} {
		if s == nil {
			continue
		}
		if !slices.Contains(saveAbilities, s.Ability) {
			return bad("a save needs an ability")
		}
		if s.OnPass != "" && s.OnPass != "end" {
			return bad("on_pass can only be end")
		}
		if s.OnFail != "" {
			if _, ok := all.effects[s.OnFail]; !ok {
				return bad("on_fail names %q, which is not an effect of the file", s.OnFail)
			}
		}
	}
	if t := d.Trigger; t != nil {
		f, ok := ParseDice(t.Dice)
		if !ok || f.Count < 1 || f.AddsModifier {
			return bad("turn_trigger.dice %q is not a dice formula", t.Dice)
		}
		if !strings.HasPrefix(t.DamageType, "damage-type:") || c.named[t.DamageType] == nil {
			return bad("turn_trigger.damage_type %q is not an SRD damage type", t.DamageType)
		}
		if t.MaxTriggers < 1 {
			return bad("turn_trigger.max_triggers must be at least 1")
		}
	}
	for _, k := range append([]string{d.OnEnd}, d.EndsEffectsOf...) {
		if k == "" {
			continue
		}
		if _, ok := all.effects[k]; !ok && all.spells[k] == nil {
			return bad("%q is not an effect of the file", k)
		}
	}
	if d.BreakFree != "" && !slices.Contains(saveAbilities, d.BreakFree) {
		return bad("break_free needs an ability")
	}
	if len([]rune(d.PlayerLabelPT)) > maxPlayerLabel {
		return bad("player_label_pt is over %d characters", maxPlayerLabel)
	}
	return nil
}

// checkModifier checks one modifier's fields against its kind.
func checkModifier(m EffectModifier) error {
	if !slices.Contains(modifierKinds, m.Kind) {
		return fmt.Errorf("unknown kind %q", m.Kind)
	}
	switch m.Kind {
	case ModifierRollDie:
		if m.Die < 2 || (m.Sign != 1 && m.Sign != -1) || len(m.AppliesTo) == 0 {
			return fmt.Errorf("roll_die needs die, sign (1 or -1) and applies_to")
		}
		for _, a := range m.AppliesTo {
			if a != RollAppliesAttack && a != RollAppliesSave && a != RollAppliesCheck {
				return fmt.Errorf("applies_to %q: only attack, save and check", a)
			}
		}
	case ModifierACBonus:
		if m.Value < 1 {
			return fmt.Errorf("ac_bonus needs a value")
		}
	case ModifierSpeedMultiplier:
		if m.Value < 0 || m.Value > 400 {
			return fmt.Errorf("speed_multiplier is a percent from 0 to 400")
		}
	case ModifierSaveAdvantage:
		if len(m.Abilities) == 0 {
			return fmt.Errorf("save_advantage needs abilities")
		}
		for _, a := range m.Abilities {
			if !slices.Contains(saveAbilities, a) {
				return fmt.Errorf("unknown ability %q", a)
			}
		}
	case ModifierExtraAction:
		if len(m.Allowed) == 0 || m.MaxWeaponAttacks < 1 {
			return fmt.Errorf("extra_action needs allowed and max_weapon_attacks")
		}
		for _, a := range m.Allowed {
			if !slices.Contains(extraActions, a) {
				return fmt.Errorf("%q is not a standard action", a)
			}
		}
	}
	return nil
}

// CombatSpellEffect returns what a spell puts on its targets in a combat
// (Bênção, Perdição, Velocidade, Imobilizar Pessoa, Teia), and whether it has
// such an effect. The result is shared and must not be modified.
func (c *Content) CombatSpellEffect(spellKey string) (*EffectDef, bool) {
	if c.c.combatEffects == nil {
		return nil, false
	}
	d, ok := c.c.combatEffects.spells[spellKey]
	return d, ok
}

// CombatEffect returns an effect of the catalog that is not a spell ("effect:dodging",
// "effect:lethargy"...). The result is shared and must not be modified.
func (c *Content) CombatEffect(key string) (*EffectDef, bool) {
	if c.c.combatEffects == nil {
		return nil, false
	}
	d, ok := c.c.combatEffects.effects[key]
	return d, ok
}

// CombatEffectKeys lists the keys of the catalog's non-spell effects, sorted.
func (c *Content) CombatEffectKeys() []string {
	if c.c.combatEffects == nil {
		return nil
	}
	keys := make([]string, 0, len(c.c.combatEffects.effects))
	for k := range c.c.combatEffects.effects {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// ConditionInfo says who reads a condition and what it does to its creature.
func (c *Content) ConditionInfo(key string) (ConditionInfo, bool) {
	if c.c.combatEffects == nil {
		return ConditionInfo{}, false
	}
	i, ok := c.c.combatEffects.conditions[key]
	return i, ok
}

// SpellEffectRounds is how many rounds a spell's effect lasts in a combat, from
// the spell's own duration: ten for each minute; 0 for a spell that lasts an
// hour or more (the app does not count that time: the effect lasts until the
// master ends it or the concentration does), and for one that has no timed
// duration.
func (c *Content) SpellEffectRounds(spellKey string) int {
	d, ok := c.c.spellDetails[spellKey]
	if !ok {
		return 0
	}
	seconds, timed := d.Duration.Seconds()
	if !timed {
		return 0
	}
	return seconds / SecondsPerRound
}

// ConditionKeys lists the keys of the conditions the file describes (the SRD's 15), sorted.
func (c *Content) ConditionKeys() []string {
	if c.c.combatEffects == nil {
		return nil
	}
	keys := make([]string, 0, len(c.c.combatEffects.conditions))
	for k := range c.c.combatEffects.conditions {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
