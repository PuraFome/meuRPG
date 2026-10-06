package characters

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// maxLiveContents is how many campaign contents (the SRD plus one table's layer)
// the live source keeps, the plan's budget (ADR-0018, section 5): about 3 MB each
// with a real table, so 8 stay far below the instance's 512 MiB (docs/operacao.md).
const maxLiveContents = 8

// TableSource is the ContentSource that gives a campaign its own content: the SRD
// with the table's entries on top (MR-025, RN-23, ADR-0018).
//
// For reads the campaign's content revision in the caller's transaction (never
// through the pool while one is open: PR #121's rule), and that read is also what
// orders a content write against a sheet write. The content is cached by
// (campaign, revision), at most maxLiveContents of them, the least recently used
// dropped first. A campaign whose master wrote nothing (revision 0) gets the SRD
// content itself, with no query beyond the revision.
//
// A miss inside a transaction is built for that request alone: it reads the rows
// in the same transaction, so they are the revision's, and nobody waits on a
// shared single-flight (a request holding a connection waiting for another that
// needs one is how a pool deadlocks). Two requests that miss at once both build.
type TableSource struct {
	queries *charactersdb.Queries
	srd     *rules.Content

	mu    sync.Mutex
	cache map[liveKey]*rules.Content
	order []liveKey // least recently used first
}

type liveKey struct {
	campaignID string
	revision   int32
}

// NewTableSource returns the live source over the SRD content (rules.LoadSRD).
func NewTableSource(pool *pgxpool.Pool, srd *rules.Content) *TableSource {
	return &TableSource{queries: charactersdb.New(pool), srd: srd, cache: map[liveKey]*rules.Content{}}
}

// For implements ContentSource. TableRules are the SRD's until the table's rules
// (RN-24, slice 10.4a) have a place to be stored.
func (s *TableSource) For(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, TableRules, error) {
	c, err := s.content(ctx, tx, campaignID)
	return c, DefaultTableRules(), err
}

func (s *TableSource) content(ctx context.Context, tx pgx.Tx, campaignID string) (*rules.Content, error) {
	id, ok := parseUUID(campaignID)
	if !ok {
		return s.srd, nil // an ID that is no campaign has no content of its own
	}
	q := s.queries
	if tx != nil {
		q = q.WithTx(tx)
	}
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
