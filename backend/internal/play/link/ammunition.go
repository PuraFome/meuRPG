package link

import "errors"

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
