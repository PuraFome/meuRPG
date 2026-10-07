package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The real wiring, as run() builds it, must pass its own check: every Set... call
// is made. The pool never connects (pgx opens connections lazily), and nothing
// in the wiring talks to the database. If someone drops a setter, or adds a
// collaborator to a CheckWired without wiring it, this test fails (and so does
// the server at startup).
func TestTheRealWiringIsComplete(t *testing.T) {
	t.Parallel()
	pool, err := pgxpool.New(t.Context(), "postgresql://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	t.Cleanup(pool.Close)
	content, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD() error = %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	m, err := wireModules(logger, pool, content, nil, &gen.Fake{}, 20)
	if err != nil {
		t.Fatalf("wireModules() error = %v", err)
	}
	if m.campaigns == nil || m.characters == nil || m.play == nil || m.maps == nil || m.progression == nil || m.notes == nil {
		t.Errorf("a module was not built: %+v", m)
	}
	t.Cleanup(func() { m.play.Close() })
}
