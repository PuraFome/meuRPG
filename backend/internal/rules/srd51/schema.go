package srd51

import "encoding/json"

// This file is the format of the files in data/: the SRD 5.1 content that
// cmd/srdimport writes and package rules reads. It is our own normalized
// shape, not the 5e-database one: only the fields the engine uses, and
// every reference as a stable key with a kind prefix, such as
// "class:wizard", "feature:arcane-recovery" or "spell:fire-bolt". Text
// ("desc") is the SRD's English text, as it came.
//
// Every file is a JSON array sorted by key (levels.json by class, subclass
// and level), so a new snapshot shows as a small, readable diff.

// Manifest describes a snapshot: where it came from and the sha256 of every
// input and output file.
type Manifest struct {
	// SourceRepo and SourceCommit pin the 5e-srd-api commit the snapshot
	// was made from, and SourcePath is the folder of the JSON files in it.
	SourceRepo   string `json:"source_repo"`
	SourceCommit string `json:"source_commit"`
	SourcePath   string `json:"source_path"`
	// SnapshotVersion is "srd51@" plus the first 12 characters of
	// SourceCommit. The content version adds the effects revision to it.
	SnapshotVersion string     `json:"snapshot_version"`
	Inputs          []FileHash `json:"inputs"`
	Outputs         []FileHash `json:"outputs"`
}

// FileHash is a file name and the hex sha256 of its bytes.
type FileHash struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// AbilityScore is one of the six abilities. Its key is the bare index
// ("str"), like rules.Ability.
type AbilityScore struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	FullName string   `json:"full_name"`
	Desc     []string `json:"desc"`
}

// Skill is a skill and the ability it uses.
type Skill struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Ability string   `json:"ability"`
	Desc    []string `json:"desc"`
}

// Choice is "choose N from these keys".
type Choice struct {
	Choose int      `json:"choose"`
	From   []string `json:"from"`
}

// Race is a race.
type Race struct {
	Key            string         `json:"key"`
	Name           string         `json:"name"`
	SpeedFt        int            `json:"speed_ft"`
	Size           string         `json:"size"`
	AbilityBonuses map[string]int `json:"ability_bonuses"`
	// AbilityBonusChoices is the half-elf's "+1 to two other abilities".
	AbilityBonusChoices *Choice `json:"ability_bonus_choices,omitempty"`
	// Languages are always known; LanguageChoices more are chosen.
	Languages       []string `json:"languages"`
	LanguageChoices int      `json:"language_choices,omitempty"`
	Traits          []string `json:"traits"`
	Subraces        []string `json:"subraces"`
}

// Subrace is a subrace of one race.
type Subrace struct {
	Key            string         `json:"key"`
	Name           string         `json:"name"`
	Race           string         `json:"race"`
	Desc           string         `json:"desc"`
	AbilityBonuses map[string]int `json:"ability_bonuses"`
	Traits         []string       `json:"traits"`
}

// Trait is a racial trait.
type Trait struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Races    []string `json:"races"`
	Subraces []string `json:"subraces"`
	Desc     []string `json:"desc"`
	// Proficiencies are proficiency keys the trait grants.
	Proficiencies []string `json:"proficiencies"`
	// ProficiencyChoices is how many proficiencies the trait lets the
	// player choose (Skill Versatility: 2), and ProficiencyOptions from
	// which.
	ProficiencyChoices int      `json:"proficiency_choices,omitempty"`
	ProficiencyOptions []string `json:"proficiency_options,omitempty"`
	LanguageChoices    int      `json:"language_choices,omitempty"`
	// Parent is set on an option of another trait, such as one color of
	// Draconic Ancestry.
	Parent string `json:"parent,omitempty"`
	// Options are the keys of this trait's options, to choose one.
	Options []string `json:"options,omitempty"`
}

// Class is a class.
type Class struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	HitDie       int      `json:"hit_die"`
	SavingThrows []string `json:"saving_throws"`
	// SkillChoices is the class's choice of skills at level 1.
	SkillChoices Choice `json:"skill_choices"`
	// Proficiencies are the proficiency keys of a character who starts in
	// this class, saving throws left out.
	Proficiencies []string          `json:"proficiencies"`
	Multiclass    Multiclass        `json:"multiclass"`
	Spellcasting  *ClassSpellcaster `json:"spellcasting,omitempty"`
	Subclasses    []string          `json:"subclasses"`
	// SubclassLevel is the first class level with a subclass feature.
	SubclassLevel int `json:"subclass_level"`
}

// Multiclass is what taking this class as a second class needs and gives.
type Multiclass struct {
	// Minimums must all be met; with AnyOf set, one of AnyOf is enough
	// instead (the fighter: STR 13 or DEX 13).
	Minimums map[string]int `json:"minimums,omitempty"`
	AnyOf    map[string]int `json:"any_of,omitempty"`
	// Proficiencies and SkillChoices are what a multiclass character gets.
	Proficiencies []string `json:"proficiencies"`
	SkillChoices  *Choice  `json:"skill_choices,omitempty"`
}

// ClassSpellcaster says when a class starts casting and with which
// ability.
type ClassSpellcaster struct {
	Level   int    `json:"level"`
	Ability string `json:"ability"`
}

// Level is one row of a class table, or of a subclass table when Subclass
// is set.
type Level struct {
	Class     string   `json:"class"`
	Subclass  string   `json:"subclass,omitempty"`
	Level     int      `json:"level"`
	ProfBonus int      `json:"prof_bonus,omitempty"`
	Features  []string `json:"features"`
	// Spellcasting is the row's casting columns, for casting classes.
	Spellcasting *LevelSpellcasting `json:"spellcasting,omitempty"`
	// ClassSpecific and SubclassSpecific are the class table's other
	// columns (rage count, ki points, sneak attack dice...), as they came.
	ClassSpecific    json.RawMessage `json:"class_specific,omitempty"`
	SubclassSpecific json.RawMessage `json:"subclass_specific,omitempty"`
}

// LevelSpellcasting is the casting columns of a class table row. Slots[0]
// is the number of 1st-level slots; for the warlock these are pact slots.
type LevelSpellcasting struct {
	CantripsKnown int    `json:"cantrips_known"`
	SpellsKnown   int    `json:"spells_known"`
	Slots         [9]int `json:"slots"`
}

// Subclass is a subclass of one class.
type Subclass struct {
	Key    string   `json:"key"`
	Name   string   `json:"name"`
	Class  string   `json:"class"`
	Flavor string   `json:"flavor"`
	Desc   []string `json:"desc"`
	// Spells are the subclass's always-available spells (domain, oath,
	// circle and patron spells).
	Spells []SubclassSpell `json:"spells,omitempty"`
}

// SubclassSpell is a spell a subclass gives at a class level, sometimes
// only with a chosen feature (the druid's land).
type SubclassSpell struct {
	Spell        string   `json:"spell"`
	ClassLevel   int      `json:"class_level"`
	WithFeatures []string `json:"with_features,omitempty"`
}

// Feature is a class or subclass feature.
type Feature struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Class    string   `json:"class"`
	Subclass string   `json:"subclass,omitempty"`
	Level    int      `json:"level"`
	Desc     []string `json:"desc"`
	// Parent is set on an option of another feature, such as one fighting
	// style.
	Parent string `json:"parent,omitempty"`
	// Options are the keys of the options to choose OptionsChoose of.
	Options       []string `json:"options,omitempty"`
	OptionsChoose int      `json:"options_choose,omitempty"`
	// ExpertiseChoices is how many skills the feature doubles.
	ExpertiseChoices int `json:"expertise_choices,omitempty"`
}

// Background is a background.
type Background struct {
	Key             string            `json:"key"`
	Name            string            `json:"name"`
	Skills          []string          `json:"skills"`
	Proficiencies   []string          `json:"proficiencies,omitempty"`
	LanguageChoices int               `json:"language_choices,omitempty"`
	Feature         BackgroundFeature `json:"feature"`
}

// BackgroundFeature is a background's feature.
type BackgroundFeature struct {
	Key  string   `json:"key"`
	Name string   `json:"name"`
	Desc []string `json:"desc"`
}

// Proficiency is something a character can be proficient in.
type Proficiency struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// Kind is "armor", "weapon", "tool", "skill", "saving-throw" or "other".
	Kind string `json:"kind"`
	// Refs are what it covers: an equipment key, an equipment category
	// such as "equipment-category:light-armor", a skill key, or an ability.
	Refs []string `json:"refs"`
}

// Equipment is an armor, a weapon or a tool.
type Equipment struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// Kind is "armor", "weapon" or "tool".
	Kind   string  `json:"kind"`
	Armor  *Armor  `json:"armor,omitempty"`
	Weapon *Weapon `json:"weapon,omitempty"`
}

// Armor is the armor part of an Equipment.
type Armor struct {
	// Category is "light", "medium", "heavy" or "shield".
	Category            string `json:"category"`
	BaseAC              int    `json:"base_ac"`
	DexBonus            bool   `json:"dex_bonus"`
	MaxDexBonus         int    `json:"max_dex_bonus,omitempty"`
	StrMinimum          int    `json:"str_minimum,omitempty"`
	StealthDisadvantage bool   `json:"stealth_disadvantage,omitempty"`
}

// Weapon is the weapon part of an Equipment.
type Weapon struct {
	// Category is "simple" or "martial"; Range is "melee" or "ranged".
	Category        string   `json:"category"`
	Range           string   `json:"range"`
	Damage          string   `json:"damage,omitempty"`
	DamageType      string   `json:"damage_type,omitempty"`
	TwoHandedDamage string   `json:"two_handed_damage,omitempty"`
	Properties      []string `json:"properties"`
	// NormalRangeFt is the reach (5 or 10) of a melee weapon, or the
	// normal range of a ranged one; LongRangeFt the long range.
	NormalRangeFt int `json:"normal_range_ft,omitempty"`
	LongRangeFt   int `json:"long_range_ft,omitempty"`
	// ThrowNormalFt and ThrowLongFt are set for thrown weapons.
	ThrowNormalFt int `json:"throw_normal_ft,omitempty"`
	ThrowLongFt   int `json:"throw_long_ft,omitempty"`
}

// Spell is a spell.
type Spell struct {
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	Level         int      `json:"level"`
	School        string   `json:"school"`
	Classes       []string `json:"classes"`
	Subclasses    []string `json:"subclasses,omitempty"`
	Ritual        bool     `json:"ritual"`
	Concentration bool     `json:"concentration"`
	CastingTime   string   `json:"casting_time"`
	Range         string   `json:"range"`
	Duration      string   `json:"duration"`
	Components    []string `json:"components"`
	Material      string   `json:"material,omitempty"`
	// AttackType is "melee" or "ranged" for spell attacks. SaveAbility is
	// set for spells that ask for a saving throw.
	AttackType  string `json:"attack_type,omitempty"`
	SaveAbility string `json:"save_ability,omitempty"`
	SaveSuccess string `json:"save_success,omitempty"`
	// Damage is one entry per damage type (Ice Storm has two).
	Damage          []SpellDamage     `json:"damage,omitempty"`
	HealAtSlotLevel map[string]string `json:"heal_at_slot_level,omitempty"`
	Desc            []string          `json:"desc"`
	HigherLevel     []string          `json:"higher_level,omitempty"`
}

// SpellDamage is one damage type of a spell. AtCharacterLevel (cantrips) and
// AtSlotLevel map a level, as text, to dice such as "2d10".
type SpellDamage struct {
	DamageType       string            `json:"damage_type,omitempty"`
	AtCharacterLevel map[string]string `json:"at_character_level,omitempty"`
	AtSlotLevel      map[string]string `json:"at_slot_level,omitempty"`
}

// Language is a language.
type Language struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Script string `json:"script,omitempty"`
}

// Named is a small SRD list entry: a damage type, a magic school, a weapon
// property or a condition.
type Named struct {
	Key  string   `json:"key"`
	Name string   `json:"name"`
	Desc []string `json:"desc"`
}
