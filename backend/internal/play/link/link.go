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
}

// Action is a standard action: its key ("standard:dash") and Portuguese name.
type Action struct {
	Key, Name string
}

// Turn is what a combatant used in the current turn, for working out what it
// can still do.
type Turn struct {
	ActionUsed, BonusActionUsed, ReactionUsed bool
	// Dashed says the Dash action doubled the speed.
	Dashed bool
	// SpeedFt is the combatant's walking speed in a combat, and MovementUsedFt
	// the feet walked this turn.
	SpeedFt, MovementUsedFt int
}
