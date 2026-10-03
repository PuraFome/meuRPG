// Package link holds the small plain types that cross the borders between
// the play module and the modules it reads while it runs a combat: the
// characters module (who can fight, with which numbers) and the maps module
// (the grid, the battle point, the tokens).
//
// Modules never import each other's code (docs/arquitetura.md), and the
// numbers a combat needs are not API messages, so the types both sides
// agree on live here, in a package that imports nothing of the project.
// Package play declares the interfaces (CombatRoster, MapKeeper) that use
// them; cmd/api connects the real services.
package link

// Character is what a combat needs to know about a character that fights:
// who it is and the numbers that start a combatant (MR-013). Numbers come
// from the sheet, derived by the rules module for a full sheet, and read
// as they are for a basic one.
type Character struct {
	ID   string
	Name string
	// Player is true for a player's character, false for an NPC.
	Player bool
	// PlayerUserID is the account that plays a player's character. Empty for
	// an NPC, and for a character whose player deleted their account (RN-16).
	PlayerUserID string
	// InitiativeBonus is added to the d20 for the initiative.
	InitiativeBonus int
	// SpeedFt is the walking speed, in feet (5 ft per square of the grid).
	SpeedFt int
	// HitPointsMax is the maximum hit points. A combatant takes it only for
	// an NPC: a player character's hit points are in its vitals.
	HitPointsMax int
	// XPValue is the XP an NPC gives when defeated (its sheet's xp_value), 0
	// for a player's character (MR-016).
	XPValue int
	// PortraitImageID is the gallery image of an NPC's portrait (MR-031), ""
	// for none and for a player's character.
	PortraitImageID string
}

// Grid is a map's battle grid: squares of 1.5 m (5 ft) across the image's
// width and down its height. The zero Grid means the map has none.
type Grid struct {
	Columns int32
	Rows    int32
}

// OK says whether the map has a grid.
func (g Grid) OK() bool { return g.Columns > 0 && g.Rows > 0 }

// BattlePoint is a battle point of a map: where its fight happens.
type BattlePoint struct {
	// MapID is the map the point is on.
	MapID string
	// TargetMapID is the map of the fight, when the master chose one; empty
	// when the fight happens on the session's current map.
	TargetMapID string
}

// TokenPosition is where a character's token stands on a map, in basis
// points of the image's width and height (0 to 10000).
type TokenPosition struct {
	CharacterID string
	XBP, YBP    int32
}

// Sheet is what the actions of a combat need from a character's sheet: the
// armor class an attack must reach, the attacks it can make and the standard
// actions it has (MR-012, MR-014). The characters module fills it from the
// derived full sheet, or from the structured attacks of a basic one.
type Sheet struct {
	// ArmorClass is the number a d20 plus the attack bonus must reach. It
	// never goes to a player (RN-20).
	ArmorClass int
	// Attacks are the weapon attacks and the attack cantrips, in sheet order.
	Attacks []Attack
	// Actions are the standard actions every creature has.
	Actions []Action
	// AttacksPerAction is how many attacks the Attack action makes: 1, or more
	// with Extra Attack.
	AttacksPerAction int
	// FeatureActions are the actions the sheet's class and race features grant
	// (rules.Derived.Actions): their key, name, economy and resource.
	FeatureActions []FeatureAction
	// FighterLevel is what Retomar o fôlego adds to its d10: the character's
	// fighter level, 0 for anyone who is not a fighter.
	FighterLevel int
}

// Attack is one attack of a sheet, with real dice.
type Attack struct {
	// Key identifies it in the sheet: a weapon or spell key, "basic:0" for the
	// first attack of a basic sheet.
	Key string
	// Name is its Portuguese name.
	Name string
	// Save says it asks for a saving throw instead of an attack roll, which
	// the spells slice handles.
	Save bool
	// Spell says it is a cantrip, cast with the whole action: Extra Attack
	// belongs to weapon attacks only.
	Spell bool
	// ToHit is added to the d20.
	ToHit int
	// DiceCount d DiceSides plus DiceBonus is the damage. DiceCount 0 is a flat
	// number.
	DiceCount, DiceSides, DiceBonus int
	// DamageType is a content key such as "damage-type:slashing".
	DamageType string
	// RangeFt is the reach or normal range, LongRangeFt the long range, both
	// in feet; 0 when the sheet says none (a melee attack reaches 5 ft).
	RangeFt, LongRangeFt int
	// Melee says it is a melee weapon, thrown or not: the only kind an
	// opportunity attack can use, with the melee reach.
	Melee bool
}

// Action is a standard action: its key ("standard:dash") and Portuguese name.
type Action struct {
	Key, Name string
}

// FeatureAction is an action a feature grants ("feature:second-wind").
type FeatureAction struct {
	Key, Name string
	// Economy is "action", "bonus_action", "reaction", "free" or "movement".
	Economy string
	// Resource is the key of the resource each use spends, or "".
	Resource string
	// Pool says the resource is a pool of points (Cura pelas mãos), not a count
	// of uses: using the action spends no point.
	Pool bool
}

// Dice is a roll: Count d Sides plus Bonus (Count 0 is a flat number), with the
// damage type for a damage.
type Dice struct {
	Count, Sides, Bonus int
	// DamageType is a key such as "damage-type:fire"; empty for a heal.
	DamageType string
}

// Spell is what a cast needs from a spell and the caster's sheet, resolved at
// the slot level the spell is cast with (MR-014).
type Spell struct {
	Key, Name string
	// Level is the spell's own level, 0 for a cantrip.
	Level int
	// Economy is "action", "bonus_action" or "reaction"; "" for a casting time
	// too long for a fight.
	Economy       string
	Concentration bool
	// RangeKind is "self", "touch", "ranged", "sight", "unlimited" or "special",
	// and RangeFt the distance of a ranged spell.
	RangeKind string
	RangeFt   int
	// AttackType is "melee", "ranged" or "": a spell attack, with ToHit added to
	// the d20.
	AttackType string
	ToHit      int
	// SaveAbility is the ability of the saving throw ("dex"), "" when there is
	// none; SaveOnSuccess is "half", "none" or "other"; SaveDC the caster's DC.
	SaveAbility   string
	SaveOnSuccess string
	SaveDC        int
	// Damage is the spell's damage at the slot level (one damage type), nil when
	// it has none the engine can roll; Heal is its healing with the caster's
	// spellcasting modifier already in Bonus, nil when it does not heal.
	Damage *Dice
	Heal   *Dice
	// Area says the spell hits every creature in an area: any number of targets.
	// ExtraTargetPerLevel says it takes one more target for each slot level
	// above its own.
	Area                bool
	ExtraTargetPerLevel bool
}

// Save is a creature's saving throw: the bonus added to the d20, and whether
// the sheet has one at all. A basic-sheet NPC has none: Known is false and the
// Bonus 0.
type Save struct {
	Bonus int
	Known bool
}

// Named is a content key with its Portuguese name.
type Named struct {
	Key, NamePT string
}

// Turn is what a combatant used in the current turn, for working out what it
// can still do.
type Turn struct {
	ActionUsed, BonusActionUsed, ReactionUsed bool
	// AttacksMade is how many attacks the Attack action made this turn.
	AttacksMade int
	// Dashed says the Dash action doubled the speed.
	Dashed bool
	// SpeedFt is the combatant's walking speed in a combat, and MovementUsedFt
	// the feet walked this turn.
	SpeedFt, MovementUsedFt int
}

// Scene is an RP scene (MR-015): a SCENE point of a map as the session opens
// it, with the master's actions on it. The DCs are in it, so it never goes to
// a player as it is.
type Scene struct {
	PointID string
	// Name and Description are the point's: the players read both in the open
	// scene.
	Name, Description string
	// Actions are in the order the master put them.
	Actions []SceneAction
}

// SceneAction is one check of a scene.
type SceneAction struct {
	ID string
	// Key is "skill:investigation", "ability:str" or "save:wis".
	Key string
	// Name is the master's name for it, "" for none.
	Name string
	// DC is the difficulty class, 0 for none.
	DC int
}

// SceneOption is a scene check with one character's numbers on it (the rules
// engine's SceneOption).
type SceneOption struct {
	// Known says the sheet has this check; a basic sheet has no skills, so its
	// character has no numbers for a scene.
	Known bool
	// CheckName is the check's name in Portuguese: "Investigação".
	CheckName string
	// Bonus is added to the d20.
	Bonus int
	// Passive is the character's passive value, when HasPassive.
	Passive    int
	HasPassive bool
}
