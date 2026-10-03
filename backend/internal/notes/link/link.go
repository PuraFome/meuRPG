// Package link holds the small plain types that cross the border between the
// notes module and the maps module, which keeps what a player's notes read of
// the scenes: the scenes the group discovered and the clues the master
// revealed to the player.
//
// Modules never import each other's code (docs/arquitetura.md), so the types
// both sides agree on live here, in a package that imports nothing of the
// project. Package notes declares the interface (notes.Scenes) that uses them;
// cmd/api connects the real one (maps.SessionMaps).
package link

import "time"

// Scene is a scene the group discovered (MR-030): a SCENE point the master
// revealed on the map or opened in a session.
type Scene struct {
	// ID is the map point's.
	ID string
	// Name is the point's current name. It is only ever handed to a player
	// inside a Scene of this list, so an undiscovered scene's name never
	// travels.
	Name string
}

// Clue is a clue the master revealed to one player (MR-029).
type Clue struct {
	// ID is the reveal's, not the master's clue's: a player never learns the
	// master's IDs.
	ID string
	// PointID is the clue's scene; empty when the point was deleted since.
	PointID string
	// Text is the clue as it was when revealed.
	Text string
	// RevealedAt is when it reached the player.
	RevealedAt time.Time
}
