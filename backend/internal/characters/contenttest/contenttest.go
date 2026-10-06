// Package contenttest is a characters.ContentSource for the DB test suites. It
// does what the real one will (10.1c reads the campaign's content revision):
// a query, in the caller's transaction when there is one and through the pool
// when not. The test pools have one connection and an Acquire tracer, so a
// content read made with a nil tx inside a transaction fails the test at once,
// in every suite that builds the characters service with it.
package contenttest

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/characters"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Source asks the database, then answers like characters.SRDSource.
type Source struct {
	Pool    *pgxpool.Pool
	Content *rules.Content
}

// ContentFor implements characters.ContentSource: the same probe, the SRD content.
func (s Source) ContentFor(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, error) {
	c, _, err := s.For(ctx, tx, campaignID)
	return c, err
}

// For implements characters.ContentSource.
func (s Source) For(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, characters.TableRules, error) {
	var err error
	if tx != nil {
		_, err = tx.Exec(ctx, "SELECT 1")
	} else {
		_, err = s.Pool.Exec(ctx, "SELECT 1")
	}
	if err != nil {
		return nil, characters.TableRules{}, err
	}
	return characters.NewSRDSource(s.Content).For(ctx, tx, campaignID)
}
