package link

import (
	"errors"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

// What the ammunition calls refuse with: the characters module returns them, the play module turns them into the
// typed refusal the app reads.
var (
	// ErrNoAmmunition: no piece of the kind is left (or the character never held any).
	ErrNoAmmunition = errors.New("no ammunition left")
	// ErrAmmunitionRecovered: the pieces of the character's last combat were already taken back.
	ErrAmmunitionRecovered = errors.New("the ammunition of the last combat was already recovered")
	// ErrNothingToRecover: the character fired nothing in that combat.
	ErrNothingToRecover = errors.New("no ammunition was fired in that combat")
)

// DawnRoll rolls count dice of the given sides and adds the bonus: the total of dice such as 1d4+1. The caller rolls
// them, so the characters module never holds the table's dice (RN-18).
type DawnRoll func(count, sides, bonus int) (int32, error)

// DawnItem is one magic item a dawn gave charges back to.
type DawnItem struct {
	CharacterID string
	// Item is the sheet's entry after the dawn.
	Item *charactersv1.SheetMagicItem
	// Regained is the charges the item gained.
	Regained int32
}
