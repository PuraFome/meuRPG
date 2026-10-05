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
	"errors"
	"slices"
	"time"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
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
	// PassivePerception is the derived passive Wisdom (Perception) score: what
	// notices a trap by passing near it (MR-035, D5). Light penalties are the maps
	// module's to apply.
	PassivePerception int
	// Eyes is the familiar the player looks through right now ("Ver pelos olhos
	// do familiar", MR-036), nil when they do not. The maps module puts it on the
	// map, at the square where the creature stands (its token, or its combatant
	// while a combat runs), as the player's viewer instead of the character's own:
	// looking through the familiar the character is blind and deaf (SRD), so its own
	// view is switched off while the other characters' still count for "Visão do grupo".
	Eyes *FamiliarEyes
}

// FamiliarEyes is a creature the player sees through: which one, and with which
// senses (an owl's darkvision, a bat's blindsight).
type FamiliarEyes struct {
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

// Eyes is what a creature notices with: its passive Perception and its senses
// (MR-035, MR-037). A character's own come from its derived sheet
// (PartyMember).
type Eyes struct {
	// Passive is the passive Wisdom (Perception) score, before any light penalty.
	Passive int
	Senses  vision.Senses
}

// Observer is someone who looks for traps: a player's character, or one of its
// creatures, standing on a square of the map (MR-035, D5). The maps module works
// out what it sees and notices from the square given, which is why a combat move
// passes it: the move committed a moment ago, and the combatants' squares the
// maps module reads for itself may not have caught up.
type Observer struct {
	// CharacterID is the player's character that learns of what is found: the
	// searcher, the one that moved, or the owner of the creature that moved.
	CharacterID string
	// At is where the observer stands.
	At grid.Square
	// Creature, when set, replaces the character's own passive Perception and
	// senses with the creature's (a creature notices with its own eyes).
	Creature *Eyes
}

// Trap is a trap point as the play module needs it, with the master's data. It
// never goes to a player as it is (RN-10).
type Trap struct {
	PointID, MapID, Name string
	// Squares are the squares of the trap's area: empty for a map with no grid.
	Squares []grid.Square
	// State is "armed", "triggered" or "disarmed", and TriggeredAt when it fired
	// (nil for a trap that never did).
	State       string
	TriggeredAt *time.Time
	// OnEnter says the trap fires when a player character or a creature enters its
	// area ("Ao entrar na área"); false is the master's "Manual". It is not a
	// secret once the character knows the trap, so it is set for every trap.
	OnEnter bool
	// Spec is the trap as the master sees it: DCs, trigger, effect. The state is set.
	Spec *mapsv1.TrapSpec
	// Public says everyone who sees the map sees the trap: it was revealed to all
	// or it fired.
	Public bool
	// KnownBy are the characters the trap was revealed to.
	KnownBy []string
}

// Armed says the trap can fire.
func (t Trap) Armed() bool { return t.State == "armed" }

// Knows says the character knows the trap: it was revealed to it, or everyone
// sees it.
func (t Trap) Knows(characterID string) bool {
	return t.Public || slices.Contains(t.KnownBy, characterID)
}

// Covers says the trap's area holds the square.
func (t Trap) Covers(sq grid.Square) bool { return slices.Contains(t.Squares, sq) }

// SearchResult is what a search found.
type SearchResult struct {
	// Found are the points the search revealed to the searcher's character.
	Found []string
	// Users are the players to tell the map changed (the searcher's).
	Users []string
}

// ErrTrapNotArmed is the error of firing a trap that is not armed.
var ErrTrapNotArmed = errors.New("the trap is not armed")

// ErrSearchNeedsTwoDice is the error of a Perception search with one die when a square
// within 3 m that the searcher sees is lightly obscured: the roll has disadvantage.
var ErrSearchNeedsTwoDice = errors.New("the search needs two dice")
