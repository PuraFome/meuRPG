// Package tablerules is the shape of the rules a table chooses on "Regras da
// mesa" (MR-025, RN-24), shared by the modules that read them: campaigns stores
// them (campaign_table_rules), characters reads them through its ContentSource
// (the hit points of a level-up, the ability scores of a new sheet), maps reads
// FogOnNewMaps when it creates a map, and the combat reads the critical and the
// death saves (slice 10.4b). It is a plain value with no behavior, so the
// modules share a type without importing each other.
//
// The zero value is the SRD's defaults, which are what the app did before the
// rules existed: each field says what a table changes, never what the SRD
// already does.
package tablerules

// Rules are the table's rules. The dice mode is not here: it is the campaign's
// own setting (campaigns.dice_mode, RN-18), and "Estilo da mesa" is worked out
// from it and two of these fields.
type Rules struct {
	// HitPoints is how a level-up's hit points are decided. The zero value is
	// HitPointsPlayerChooses: roll or take the average.
	HitPoints HitPointsRule
	// AbilityMethodsOff are the ways of making a new sheet's ability scores the
	// table does not allow. The zero value allows all of them.
	AbilityMethodsOff AbilityMethods
	// CriticalMaxPlusRoll makes a critical hit the maximum of the dice plus a
	// roll, instead of the SRD's doubled dice.
	CriticalMaxPlusRoll bool
	// DeathSavesHidden hides a character's death saves from the players who are
	// not its owner, instead of the SRD's visible ones.
	DeathSavesHidden bool
	// CombatWithoutMap makes "Iniciar combate" start without a map by default.
	CombatWithoutMap bool
	// FogOnNewMaps makes a map created from now on start with the fog of war on.
	FogOnNewMaps bool
	// FeatsAllowed lets the table play with feats, an optional rule of the game:
	// the level-up's Ability Score Improvement step then offers a feat in place of
	// the ability increase. The zero value is the SRD's: feats are not used.
	FeatsAllowed bool
	// EnemyReactionsAlways makes every action of a player against an enemy wait
	// for the master's one-tap check ("Reações dos inimigos: Sempre"), instead of
	// waiting only when an enemy has a reaction.
	EnemyReactionsAlways bool
	// Reminders are the table's house rules: short texts shown on "Regras da
	// mesa" and never enforced; none by default.
	Reminders []string
}

// HitPointsRule is how a level-up's hit points are decided.
type HitPointsRule string

// The hit points rules. The zero value is the SRD's: the player chooses.
const (
	// HitPointsPlayerChooses lets the player pick rolling or the average.
	HitPointsPlayerChooses HitPointsRule = ""
	// HitPointsRoll makes everybody roll the hit die; the average is refused.
	HitPointsRoll HitPointsRule = "roll"
	// HitPointsAverage makes everybody take the average; a roll is refused.
	HitPointsAverage HitPointsRule = "average"
)

// AbilityMethods is a set of ways of making ability scores.
type AbilityMethods struct {
	// StandardArray is 15, 14, 13, 12, 10 and 8; PointBuy the 27-point buy; Roll
	// 4d6 dropping the lowest, rolled and stored by the server; Typed the
	// player's own six values, 3 to 18 before the race's bonus.
	StandardArray, PointBuy, Roll, Typed bool
}
