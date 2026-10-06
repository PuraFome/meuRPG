// Package contenttest is the characters.ContentSource for the DB test suites of
// the other packages: the real one, the live source that reads the campaign's
// content revision (MR-025, ADR-0018) in the caller's transaction when there is
// one and through the pool when not. The test pools have one connection and an
// Acquire tracer, so a content read made with a nil tx inside a transaction fails
// the test at once, in every suite that builds the characters service with it.
package contenttest

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/characters"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// NewSource returns the live source over the SRD content and the test pool.
func NewSource(pool *pgxpool.Pool, srd *rules.Content) characters.ContentSource {
	return characters.NewTableSource(pool, srd)
}
