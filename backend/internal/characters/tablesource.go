package characters

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// maxLiveContents is how many campaign contents (the SRD plus one table's layer)
// the live source keeps, the plan's budget (ADR-0018, section 5): about 3 MB each
// with a real table, so 8 stay far below the instance's 512 MiB (docs/operations.md).
const maxLiveContents = 8

// TableSource is the ContentSource that gives a campaign what it plays with: the
// SRD with the table's own entries on top (MR-025, RN-23, ADR-0018) and the table
// rules its master saved (RN-24).
//
// It reads the campaign's content revision in the caller's transaction (never
// through the pool while one is open: PR #121's rule), and that read is also what
// orders a content write against a sheet write. The content is cached by
// (campaign, revision), at most maxLiveContents of them, the least recently used
// dropped first. A campaign whose master wrote nothing (revision 0) gets the SRD
// content itself, with no query beyond the revision. The table rules are read
// through the same tx, by a TableRulesReader (nil: the defaults, no read).
//
// A miss inside a transaction is built for that request alone: it reads the rows
// in the same transaction, so they are the revision's, and nobody waits on a
// shared single-flight (a request holding a connection waiting for another that
// needs one is how a pool deadlocks). Two requests that miss at once both build.
// A miss outside a transaction reads the revision and the rows in one
// transaction (db.InTx's, a plain one, which is enough for a snapshot), so the content cached under a revision is that revision's.
type TableSource struct {
	pool    *pgxpool.Pool
	queries *charactersdb.Queries
	srd     *rules.Content
	tables  TableRulesReader

	mu    sync.Mutex
	cache map[liveKey]*rules.Content
	order []liveKey // least recently used first
}

type liveKey struct {
	campaignID string
	revision   int32
}

// NewTableSource returns the live source over the SRD content (rules.LoadSRD)
// and the reader of the saved table rules (campaigns.Service; nil for the SRD
// defaults).
func NewTableSource(pool *pgxpool.Pool, srd *rules.Content, tables TableRulesReader) *TableSource {
	return &TableSource{pool: pool, queries: charactersdb.New(pool), srd: srd, tables: tables, cache: map[liveKey]*rules.Content{}}
}

// For implements ContentSource.
func (s *TableSource) For(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, TableRules, error) {
	if _, ok := parseUUID(campaignID); !ok {
		return s.srd, DefaultTableRules(), nil // an ID that is no campaign has nothing of its own
	}
	c, err := s.ContentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, TableRules{}, err
	}
	if s.tables == nil {
		return c, DefaultTableRules(), nil
	}
	r, err := s.tables.StoredTableRules(ctx, tx, campaignID)
	if err != nil {
		return nil, TableRules{}, fmt.Errorf("read the table rules: %w", err)
	}
	return c, r, nil
}

// ContentFor implements ContentSource: the content, without the table rules.
func (s *TableSource) ContentFor(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, error) {
	id, ok := parseUUID(campaignID)
	if !ok {
		return s.srd, nil // an ID that is no campaign has no content of its own
	}
	if tx != nil {
		return s.contentIn(ctx, s.queries.WithTx(tx), id)
	}
	// Outside a transaction a hit costs one read of the revision; only a miss
	// opens a transaction, to read the revision again with the rows.
	revision, err := s.queries.GetContentRevision(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.srd, nil
	}
	if err != nil {
		return nil, wrap("read the content revision", err)
	}
	if c := s.cached(liveKey{id, revision}); c != nil {
		return c, nil
	}
	var c *rules.Content
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		c, err = s.contentIn(ctx, s.queries.WithTx(tx), id)
		return err
	})
	return c, err
}

// contentIn is the content of a campaign read with q, which is one transaction's
// (the revision and the rows are one snapshot).
func (s *TableSource) contentIn(ctx context.Context, q *charactersdb.Queries, id string) (*rules.Content, error) {
	revision, err := q.GetContentRevision(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.srd, nil
	}
	if err != nil {
		return nil, wrap("read the content revision", err)
	}
	key := liveKey{id, revision}
	if c := s.cached(key); c != nil {
		return c, nil
	}
	rows, err := q.ListCampaignContent(ctx, id)
	if err != nil {
		return nil, wrap("read the table's content", err)
	}
	overlay, err := overlayOf(rows, int(revision))
	if err != nil {
		return nil, wrap("read the table's content", err)
	}
	// The options the master switched off, read in the same transaction as the
	// revision: every write of one starts by bumping it.
	if overlay.Off, err = q.ListContentOff(ctx, id); err != nil {
		return nil, wrap("read the options switched off", err)
	}
	c, err := s.srd.With(overlay)
	if err != nil {
		// Every write checks the whole overlay before it commits, so a stored one
		// that fails is a bug (or an engine change that an entry no longer
		// passes): the campaign cannot be read, which is better than a silent SRD.
		return nil, wrap("add the table's content to the rules", err)
	}
	s.store(key, c)
	return c, nil
}

func (s *TableSource) cached(key liveKey) *rules.Content {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.cache[key]
	if !ok {
		return nil
	}
	s.touch(key)
	return c
}

// touch moves key to the most recent end. The caller holds mu.
func (s *TableSource) touch(key liveKey) {
	for i, k := range s.order {
		if k == key {
			s.order = append(append(s.order[:i:i], s.order[i+1:]...), key)
			return
		}
	}
	s.order = append(s.order, key)
}

func (s *TableSource) store(key liveKey, c *rules.Content) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cache[key]; !ok && len(s.order) >= maxLiveContents {
		delete(s.cache, s.order[0])
		s.order = s.order[1:]
	}
	s.cache[key] = c
	s.touch(key)
}

// cached counts (for tests): how many contents the source holds.
func (s *TableSource) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cache)
}
