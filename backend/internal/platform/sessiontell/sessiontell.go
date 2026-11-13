// Package sessiontell is the little a module that does not own the live session may tell its streams through the play module's
// InSession: four hints, and nothing of the hub or the session's number. The play module implements it; the modules that call
// InSession declare the host interface themselves, with this type as the argument of their closure.
package sessiontell

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Tell collects what a write tells the live streams once its transaction committed. The session's number moves once if the write
// called any of them.
type Tell interface {
	// Touch marks a write that sends no event but changes what the screens show.
	Touch()
	// CreaturesChanged tells the master and the owner's player that their creature lists changed.
	CreaturesChanged(ownerUserID string)
	// EncounterChanged tells the streams that a combat changed (an empty id tells nothing).
	EncounterChanged(encounterID string)
	// CombatLogChangedFull tells the master, and only the master, that the log of the combat in progress must be read again from
	// the start: the table's rules decide what its lines show.
	CombatLogChangedFull()
}

// Host runs a write of another module that changes what a live session shows, under the play module's funnel: the open session's row
// is taken first, fn writes on that transaction and tells through t, and once it committed the session's number moves once if fn told
// and what fn told is sent with it. With no open session fn runs in a plain transaction and t tells nothing. play.Service implements it.
type Host interface {
	InSession(ctx context.Context, campaignID string, fn func(tx pgx.Tx, t Tell) error) error
}

// None is the Tell of a write that runs with no play module: it tells nothing. A type that keeps some of the hints embeds it.
type None struct{}

func (None) Touch()                  {}
func (None) CreaturesChanged(string) {}
func (None) EncounterChanged(string) {}
func (None) CombatLogChangedFull()   {}
