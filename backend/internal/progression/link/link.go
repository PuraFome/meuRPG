// Package link holds the small plain types that cross the borders between
// the progression module and the modules it reads while it gives XP: the
// characters module (who is in the party, the XP on their sheets) and the
// play module (the combat's defeated NPCs, the session's log).
//
// Modules never import each other's code (docs/arquitetura.md), so the types
// both sides agree on live here, in a package that imports nothing of the
// project. Package progression declares the interfaces that use them;
// cmd/api connects the real services (the same arrangement as play/link).
package link

// Member is a living player character of a campaign, with the numbers the XP
// needs from its sheet.
type Member struct {
	ID   string
	Name string
	// PlayerUserID is the account that plays it, empty when the player
	// deleted theirs (RN-16).
	PlayerUserID string
	// Level is the total level, 1 to 20.
	Level int32
	// XP is the sheet's experience points, 0 to 1,000,000.
	XP int32
	// NextLevelXP is the XP that reaches Level+1, 0 at level 20.
	NextLevelXP int32
}

// Encounter is what an enemies award needs from a combat.
type Encounter struct {
	// Name is what the master called the combat.
	Name string
	// Ended says the combat is over.
	Ended bool
	// XP is the sum of the XP values of its defeated NPC combatants.
	XP int32
}
