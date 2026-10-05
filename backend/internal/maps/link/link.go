// Package link holds the small plain types that cross the border between the
// maps module and the modules it reads for the fog of war (MR-036, Etapa 9):
// the characters module (who sees, and with which senses) and the play module
// (where the combatants stand while a fight runs).
//
// Modules never import each other's code (docs/arquitetura.md), so the types
// both sides agree on live here, in a package that imports nothing of the
// project but the pure rules. Package maps declares the interfaces that use
// them; cmd/api connects the real services.
package link

import (
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
	"github.com/PuraFome/meuRPG/backend/internal/rules/vision"
)

// PartyMember is a living player character as the fog of war needs it: whose
// it is and what it sees with.
type PartyMember struct {
	// CharacterID is the character (a UUID).
	CharacterID string
	// UserID is the account that plays it. Never empty: a character whose
	// player is gone (RN-16) is not listed.
	UserID string
	// Senses are its special senses, derived from the sheet. For a druid in Wild
	// Shape they are the beast's: a wolf has no darkvision (MR-037, D6).
	Senses vision.Senses
	// Eyes is the familiar the player looks through right now ("Ver pelos olhos
	// do familiar", MR-036), nil when they do not. The maps module puts it on the
	// map, at the square where the creature stands (its token, or its combatant
	// while a combat runs), as a viewer of its own beside the character's.
	Eyes *Eyes
}

// Eyes is a creature the player sees through: which one, and with which senses
// (an owl's darkvision, a bat's blindsight).
type Eyes struct {
	CreatureID string
	Senses     vision.Senses
}

// MapCreature is a live creature of a character as a map token needs it: whose it
// is and what it is called. The characters module fills it from character_creatures.
type MapCreature struct {
	// ID is the creature (a UUID) and OwnerCharacterID its owner's character.
	ID, OwnerCharacterID string
	// OwnerUserID is the owner's player; empty if the player deleted the account.
	OwnerUserID string
	// Name is the name its owner gave it, and MonsterKey the SRD creature it is.
	Name, MonsterKey string
}

// CombatPositions says where the combatants of a running combat stand, by
// character, in squares of the map's grid. Running is false when no combat that
// is not ended is on the map; the positions are then empty and the tokens say
// where everyone stands.
type CombatPositions struct {
	Running   bool
	Positions map[string]grid.Square
	// Creatures are where the character's creatures stand, by creature ID (MR-037):
	// the ones that are combatants and have a square, never a dismissed one.
	Creatures map[string]grid.Square
}
