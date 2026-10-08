package progression

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1/progressionv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// afterQuery runs hook once, right after the first statement whose text
// contains match finishes.
type afterQuery struct {
	match string
	hook  func()
	once  sync.Once
}

type queryTextKey struct{}

func (*afterQuery) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTextKey{}, data.SQL)
}

func (a *afterQuery) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if sql, _ := ctx.Value(queryTextKey{}).(string); strings.Contains(sql, a.match) {
		a.once.Do(a.hook)
	}
}

// tracedClient is user u's progression client over a second service on a pool
// of its own whose statements run hook once, after the first one that contains
// match: a request committed by the real server from the hook lands exactly
// between two statements of the read.
func (h *harness) tracedClient(u *user, match string, hook func()) progressionv1connect.ProgressionServiceClient {
	h.t.Helper()
	tracer := &afterQuery{match: match, hook: hook}
	pool, err := db.NewPoolWith(h.t.Context(), h.pool.Config().ConnString(), func(cfg *pgxpool.Config) {
		cfg.MaxConns = 2
		cfg.ConnConfig.Tracer = tracer
	})
	if err != nil {
		h.t.Fatalf("NewPoolWith() error = %v", err)
	}
	h.t.Cleanup(pool.Close)
	cfg := h.cfg
	cfg.Pool = pool
	svc, err := New(cfg)
	if err != nil {
		h.t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	svc.Mount(mux.Handle, fakeSessions{}, h.camps, connect.WithRequireConnectProtocolHeader())
	server := httptest.NewServer(mux)
	h.t.Cleanup(server.Close)
	return progressionv1connect.NewProgressionServiceClient(&http.Client{Transport: userTransport{userID: u.id, next: server.Client().Transport}}, server.URL)
}

// canUndoOf is the IDs of the awards the master is offered "Desfazer" on.
func canUndoOf(awards []*progressionv1.XPAward) []string {
	var ids []string
	for _, a := range awards {
		if a.GetCanUndo() {
			ids = append(ids, a.GetId())
		}
	}
	return ids
}

// The history is one moment: the master who undoes the latest award while the
// page is being read never makes it show that award as live next to a
// "Desfazer" on an older one, which no moment ever had.
func TestListXPAwardsIsOneSnapshotWhileTheMasterUndoes(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 6)
	tb := newTable(t, enemies, 1)
	first := tb.master.manual(t, tb.campaign, 50, tb.ids(1)...).GetAward()
	second := tb.master.manual(t, tb.campaign, 70, tb.ids(1)...).GetAward()

	client := tb.h.tracedClient(tb.master, "FROM xp_awards", func() {
		if _, err := tb.master.undo(tb.campaign, newKey()); err != nil {
			t.Errorf("UndoLastXPAward() from the hook error = %v", err)
		}
	})
	res, err := client.ListXPAwards(t.Context(), connect.NewRequest(&progressionv1.ListXPAwardsRequest{CampaignId: tb.campaign}))
	if err != nil {
		t.Fatalf("ListXPAwards() error = %v", err)
	}
	// Either the history as it was (the second award live and the one to undo) or as it became.
	byID := map[string]*progressionv1.XPAward{}
	for _, a := range res.Msg.GetAwards() {
		byID[a.GetId()] = a
	}
	ids := canUndoOf(res.Msg.GetAwards())
	asItWas := !byID[second.GetId()].GetUndone() && len(ids) == 1 && ids[0] == second.GetId()
	asItBecame := byID[second.GetId()].GetUndone() && len(ids) == 1 && ids[0] == first.GetId()
	if !asItWas && !asItBecame {
		t.Errorf("ListXPAwards(): second award undone = %v, \"Desfazer\" on %v; want the history as it was or as it became, not a mix",
			byID[second.GetId()].GetUndone(), ids)
	}

	// Positive control: the hook did undo, so the next read shows the new history.
	if got := tb.master.history(t, tb.campaign); !got[0].GetUndone() || len(canUndoOf(got)) != 1 || canUndoOf(got)[0] != first.GetId() {
		t.Errorf("history after the undo = %v, want the second undone and \"Desfazer\" on the first", got)
	}
}

// The milestones are one moment too: the marks and the award to undo come from
// the same snapshot.
func TestListMilestonesIsOneSnapshotWhileTheMasterUndoes(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 6)
	tb := newTable(t, milestones, 1)
	a, b := tb.master.plan(t, tb.campaign, "Chegar ao Vale Seco"), tb.master.plan(t, tb.campaign, "Derrotar a Sombra")
	tb.master.reach(t, tb.campaign, a.GetId(), tb.ids(1)...)
	tb.master.reach(t, tb.campaign, b.GetId(), tb.ids(1)...)

	client := tb.h.tracedClient(tb.master, "mode = 'milestone'", func() {
		if _, err := tb.master.undo(tb.campaign, newKey()); err != nil {
			t.Errorf("UndoLastXPAward() from the hook error = %v", err)
		}
	})
	res, err := client.ListMilestones(t.Context(), connect.NewRequest(&progressionv1.ListMilestonesRequest{CampaignId: tb.campaign}))
	if err != nil {
		t.Fatalf("ListMilestones() error = %v", err)
	}
	var marks []*progressionv1.XPAward
	reached := 0
	for _, m := range res.Msg.GetMilestones() {
		marks = append(marks, m.GetMarks()...)
		if m.GetReached() {
			reached++
		}
	}
	// With both marks live, the one to undo is the second; with it undone, the first.
	ids := canUndoOf(marks)
	asItWas := reached == 2 && len(ids) == 1 && ids[0] == marks[1].GetId()
	asItBecame := reached == 1 && len(ids) == 1 && ids[0] == marks[0].GetId()
	if !asItWas && !asItBecame {
		t.Errorf("ListMilestones(): %d reached, \"Desfazer\" on %v; want the list as it was or as it became, not a mix", reached, ids)
	}

	// Positive control: the hook did undo.
	if got := tb.master.milestones(t, tb.campaign); !got[0].GetReached() || got[1].GetReached() {
		t.Errorf("milestones after the undo = %v, want only the first reached", got)
	}
}
