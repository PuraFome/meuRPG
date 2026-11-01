package rules

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// The summons of a table spell. A table spell with a Summon brings the table's creatures (or SRD creatures)
// to its caster through the same checks as the SRD's summoning spells (SummonOptions, CheckSummon): the
// number of creatures is a formula of the caster and of the slot (slot()), plus a number for each circle above
// the spell's own, and the cost is the spell slot of the cast. The spell is registered with the SRD's own
// (Content.SummonSpells), so every path that reads a summoning spell reads this one.

// The turn modes of the creatures a table spell summons.
const (
	// SummonTurnOwnInitiative: each creature rolls its own initiative and takes its own turn.
	SummonTurnOwnInitiative = "own_initiative"
	// SummonTurnOwnerRollGroup: the creatures of a casting take one initiative roll, as the SRD's Conjure Animals.
	SummonTurnOwnerRollGroup = "owner_roll_group"
	// SummonTurnAfterOwner: the creatures act right after their owner. Reserved; the rules refuse it.
	SummonTurnAfterOwner = "after_owner"
)

// The roles of the creatures a table spell summons.
const (
	// SummonRoleCompanion: the creature is the owner's, acts on its own and is played by the owner.
	SummonRoleCompanion = "companion"
	// SummonRoleMount: reserved; the rules refuse it.
	SummonRoleMount = "mount"
)

// What a new casting does to the creatures of the same spell.
const (
	// SummonReplacesNone: the creatures of the old casting stay.
	SummonReplacesNone = "none"
	// SummonReplacesSame: the creatures of the old casting of the same spell are dismissed.
	SummonReplacesSame = "same_summon"
)

// The limits of a table spell's summon.
const (
	// MaxTableSummonCreatures is the most creatures a summon lists.
	MaxTableSummonCreatures = 8
	// MaxTableSummonCount is the most creatures one casting brings.
	MaxTableSummonCount = 20
	// MaxTableSummonPerSlot is the most creatures a circle above the spell's own adds.
	MaxTableSummonPerSlot = 10
	// MaxTableSummonLive is the most creatures of one spell a character keeps.
	MaxTableSummonLive = 40
)

// TableSummon is what a table spell summons. CreatureKeys are table creatures ("creature:<slug>@mesa") or SRD
// creatures. Count is a formula of the caster and of slot() (the circle of the slot);
// a formula that fails when it runs counts as 1. CountPerSlotLevel creatures are added for each circle above the
// spell's own, and the sum is clamped to 1..MaxTableSummonCount. TurnMode, Role and Replaces are "" for their first value.
// MaxLive is 0 (no limit of its own) or the most creatures of this spell the caster keeps; a casting that would
// pass it is refused.
//
// A feature's summon (TableFeature.Summon) is paid with a resource instead of a slot: ResourceKey names a resource
// of the feature's entry (or its class) and ResourceCost is the uses a casting spends (1; 0 reads as 1). It says its
// own CastingTime (empty is 1 action), Duration (empty is instantaneous) and Concentration, which a spell takes from
// itself. slot() is 0 in its count, and CountPerSlotLevel must be 0. A spell's summon has none of these.
type TableSummon struct {
	CreatureKeys      []string
	Count             string
	CountPerSlotLevel int
	TurnMode          string
	Role              string
	MaxLive           int
	Replaces          string
	ResourceKey       string
	ResourceCost      int
	CastingTime       TableCastingTime
	Duration          TableDuration
	Concentration     bool
}

// tableSummonRule is a table summon as the content keeps it: the compiled count and the checked fields. feature
// and resource are the feature that gives the summon and the resource a casting spends a use of, empty for a spell's.
type tableSummonRule struct {
	countP   *formula.Program
	turnMode string
	maxLive  int
	replaces string
	feature  string
	resource string
}

// SummonRule is what a table spell's summon says beyond the choice of creatures.
type SummonRule struct {
	// TurnMode is SummonTurnOwnInitiative or SummonTurnOwnerRollGroup.
	TurnMode string
	// MaxLive is the most creatures of the spell the caster keeps, 0 for no limit of its own.
	MaxLive int
	// ReplacesSame says a new casting dismisses the creatures of the old one.
	ReplacesSame bool
	// Feature is the key of the feature that gives the summon, and Resource the resource a casting spends one
	// use of; both are empty for a table spell's summon (its cost is the slot).
	Feature, Resource string
}

// SummonRule is the rule of the table spell's summon; false for an SRD summoning spell and for a spell that does
// not summon.
func (c *Content) SummonRule(spell string) (SummonRule, bool) {
	t, ok := c.c.tableSummons[spell]
	if !ok {
		return SummonRule{}, false
	}
	return SummonRule{TurnMode: t.turnMode, MaxLive: t.maxLive, ReplacesSame: t.replaces == SummonReplacesSame, Feature: t.feature, Resource: t.resource}, true
}

// summonCount is how many creatures a casting at the circle brings: the SRD's fixed count and number for each
// circle, or a table spell's formula for the build and the circle.
func (c *Content) summonCount(spell string, def *summonDef, circle int, b Build) int {
	extra := def.perLevel * (circle - def.spellLevel)
	rule, table := c.c.tableSummons[spell]
	if !table {
		return def.count + extra
	}
	n := 1
	d := Derive(b, c)
	env := OwnerEnv(d, b.Shield && !d.ShieldIdle)
	env.Slot = func() int { return circle }
	if rule.feature != "" {
		env.Slot = func() int { return 0 } // a feature's summon spends a use, not a slot
	}
	if v, err := rule.countP.Int(env); err == nil {
		n = v
	}
	return clamp(n+extra, 1, MaxTableSummonCount)
}

// addSummon checks a table spell's summon and registers it, nil-safe: a spell with none is not a summoning spell.
// The violations are the summon's own, each at its path ("summon.count", "summon.creature_keys[1]"...). feature is
// the key of the feature that gives the summon, "" for a table spell's own.
func (b *overlayBuilder) addSummon(c *entryErrors, key, path string, ts *TableSpell, feature string) {
	s := ts.Summon
	if s == nil {
		return
	}
	checkSummonSpell(c, key, path, ts)
	rule := &tableSummonRule{turnMode: cmp.Or(s.TurnMode, SummonTurnOwnInitiative), maxLive: s.MaxLive, replaces: cmp.Or(s.Replaces, SummonReplacesNone), feature: feature}
	checkSummonModes(c, key, path, s, rule)
	if feature == "" {
		checkSpellSummonOnly(c, key, path, s)
	} else {
		rule.resource = checkFeatureSummonSpend(c, key, path, s)
	}
	b.checkSummonCreatureKeys(c, key, path, s.CreatureKeys)
	if s.CountPerSlotLevel < 0 || s.CountPerSlotLevel > MaxTableSummonPerSlot {
		c.at(key, path, ".summon.count_per_slot_level", ReasonValue, "the creatures added for each circle above the spell's own are 0 to %d", MaxTableSummonPerSlot)
	}
	if s.MaxLive < 0 || s.MaxLive > MaxTableSummonLive {
		c.at(key, path, ".summon.max_live", ReasonValue, "the most creatures of the spell kept at a time are 0 (no limit of its own) to %d", MaxTableSummonLive)
	}
	rule.countP = b.checkSummonCount(c, key, path, s)
	if len(c.list) > 0 {
		return
	}
	b.n.summons[key] = &summonDef{spellLevel: ts.Level, creatures: slices.Clone(s.CreatureKeys), perLevel: s.CountPerSlotLevel, attack: SummonAttackFull}
	b.n.tableSummons[key] = rule
}

// checkSummonCount compiles the count and checks it against the most creatures the summon keeps.
func (b *overlayBuilder) checkSummonCount(c *entryErrors, key, path string, s *TableSummon) *formula.Program {
	var countP *formula.Program
	if strings.TrimSpace(s.Count) == "" {
		c.at(key, path, ".summon.count", ReasonValue, "the count is a formula (\"2\", \"1 + slot()\")")
	} else if p, err := b.n.compileSlotted(s.Count); err != nil {
		c.at(key, path, ".summon.count", ReasonFormula, "the formula does not work: %v", err)
	} else {
		countP = p
	}
	if n, err := strconv.Atoi(strings.TrimSpace(s.Count)); err == nil && s.MaxLive > 0 && min(n, MaxTableSummonCount) > s.MaxLive {
		c.at(key, path, ".summon.max_live", ReasonValue, "the count is %d but the spell keeps at most %d creatures: the first casting would be refused", n, s.MaxLive)
	}
	return countP
}

// checkSpellSummonOnly refuses what only a feature's summon says: a spell's cost is its slot, and its casting time,
// duration and concentration are the spell's own.
func checkSpellSummonOnly(c *entryErrors, key, path string, s *TableSummon) {
	if s.ResourceKey != "" || s.ResourceCost != 0 {
		c.at(key, path, ".summon.resource_key", ReasonValue, "a spell's summon is paid with the spell slot: only a feature's summon spends the use of a resource")
	}
	if s.CastingTime != (TableCastingTime{}) || s.Duration != (TableDuration{}) || s.Concentration {
		c.at(key, path, ".summon", ReasonValue, "the casting time, duration and concentration of a spell's summon are the spell's own")
	}
}

// checkFeatureSummonSpend checks what a feature's summon spends and returns the resource: one use of a resource, a
// casting spending 1 (0 reads as 1); no slot, so nothing is added for a circle above the spell's own. That the
// resource exists is checked once every entry is there (see pendingEffects.summon).
func checkFeatureSummonSpend(c *entryErrors, key, path string, s *TableSummon) string {
	if !validResourceName(s.ResourceKey) {
		c.at(key, path, ".summon.resource_key", ReasonValue, "a feature's summon spends the uses of a resource of the feature: the resource name has 1 to 40 characters of a-z, 0-9 and _")
	}
	if s.ResourceCost != 0 && s.ResourceCost != 1 {
		c.at(key, path, ".summon.resource_cost", ReasonValue, "a casting spends 1 use of the resource")
	}
	if s.CountPerSlotLevel != 0 {
		c.at(key, path, ".summon.count_per_slot_level", ReasonValue, "a feature's summon spends no slot: no creature is added for a higher circle")
	}
	return s.ResourceKey
}

// checkSummonSpell checks what the spell is next to its summon: a leveled spell that only the caster is the
// target of and that does nothing else, as the SRD's summoning spells (the creatures appear next to the caster).
func checkSummonSpell(c *entryErrors, key, path string, ts *TableSpell) {
	if ts.Level < 1 {
		c.at(key, path, ".summon", ReasonValue, "a cantrip does not summon: the cost of a summon is a spell slot")
	}
	if ts.Ritual {
		c.at(key, path, ".ritual", ReasonValue, "a summoning spell is not a ritual: the cost of a summon is a spell slot")
	}
	if ts.Target.Kind != TargetSelf {
		c.at(key, path, ".target.kind", ReasonValue, "a summoning spell has a target of only the caster: the creatures appear next to it")
	}
	if ts.Attack != "" || ts.Save != nil || len(ts.Damage) > 0 || ts.Heal != nil || ts.Zone != nil {
		c.at(key, path, ".summon", ReasonValue, "a summoning spell has no spell attack, saving throw, damage, healing or area")
	}
}

// checkSummonModes checks the turn mode, the role and what the casting replaces.
func checkSummonModes(c *entryErrors, key, path string, s *TableSummon, rule *tableSummonRule) {
	switch rule.turnMode {
	case SummonTurnOwnInitiative, SummonTurnOwnerRollGroup:
	case SummonTurnAfterOwner:
		c.at(key, path, ".summon.turn_mode", ReasonValue, "acting right after the owner is not available yet: a summon rolls its own initiative or takes the owner's group roll")
	default:
		c.at(key, path, ".summon.turn_mode", ReasonValue, "the turn mode of a summon is its own initiative or the owner's group roll")
	}
	switch cmp.Or(s.Role, SummonRoleCompanion) {
	case SummonRoleCompanion:
	case SummonRoleMount:
		c.at(key, path, ".summon.role", ReasonValue, "a mount is not available yet: a summon is a companion")
	default:
		c.at(key, path, ".summon.role", ReasonValue, "the role of a summon is companion")
	}
	if rule.replaces != SummonReplacesNone && rule.replaces != SummonReplacesSame {
		c.at(key, path, ".summon.replaces", ReasonValue, "a new casting replaces nothing or the creatures of the same spell")
	}
}

// checkSummonCreatureKeys checks the creatures a summon lists: at least one and at most MaxTableSummonCreatures,
// none twice, each a creature of the table (in this overlay) or the SRD.
func (b *overlayBuilder) checkSummonCreatureKeys(c *entryErrors, key, path string, keys []string) {
	if len(keys) < 1 || len(keys) > MaxTableSummonCreatures {
		c.at(key, path, ".summon.creature_keys", ReasonLimit, "%d creatures; a summon lists 1 to %d", len(keys), MaxTableSummonCreatures)
		return
	}
	for i, k := range keys {
		at := fmt.Sprintf(".summon.creature_keys[%d]", i)
		_, srd := b.base.monsters[k]
		switch {
		case slices.Index(keys, k) != i:
			c.at(key, path, at, ReasonValue, "the creature %q is listed twice", truncateRunes(k, 60))
		case !srd && (!strings.HasPrefix(k, CreatureKeyPrefix) || !b.entries[k]):
			c.at(key, path, at, ReasonReference, "the creature %q does not exist", truncateRunes(k, 60))
		}
	}
}
