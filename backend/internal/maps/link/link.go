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
	// Senses are its special senses, derived from the sheet. In Wild Shape they
	// are the beast's (slice 9.10 has the characters module send them here).
	Senses vision.Senses
	// Extra are the other pairs of eyes the player has right now, such as a
	// familiar the player sees through (slice 9.10): each one a viewer of its
	// own, added to the character's, standing where the creature stands. Empty
	// until 9.10.
	Extra []vision.Viewer
}

// CombatPositions says where the combatants of a running combat stand, by
// character, in squares of the map's grid. Running is false when no combat that
// is not ended is on the map; the positions are then empty and the tokens say
// where everyone stands.
type CombatPositions struct {
	Running   bool
	Positions map[string]grid.Square
}
