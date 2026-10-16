package progression

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1/progressionv1connect"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/progression/link"
)

// The rig of the characterization tests of the history and the planned
// milestones (viewsOf, milestoneViews, AddMilestone, MoveMilestone): a table
// whose members call a progression service of its own, on a pool of its own,
// that logs into a buffer and can fail once any call it makes to another
// module, or any query of progressiondb by name, so every branch of the reads
// and of the writes (each refusal, each failure and its log line) is frozen as
// it is. A failed query is one whose context is already canceled when it
// starts, which is what the driver answers with no statement sent.

var updateHistoryGolden = flag.Bool("update-history-golden", false, "write the golden files of the history and the milestones again")

// errHistoryInjected is the failure an armed fault of another module returns.
var errHistoryInjected = errors.New("injected failure")

// historyFaults arms one-shot failures: of a call to another module by name,
// and of a progressiondb query by its sqlc name, after skipping a number of
// runs of it.
type historyFaults struct {
	mu      sync.Mutex
	armed   map[string]bool
	queries map[string]int // query name -> runs to let through before the failing one
}

// arm makes the next call named name fail with errHistoryInjected.
func (f *historyFaults) arm(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.armed == nil {
		f.armed = map[string]bool{}
	}
	f.armed[name] = true
}

// armQuery makes the run of the query that comes after skip others fail.
func (f *historyFaults) armQuery(name string, skip int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.queries == nil {
		f.queries = map[string]int{}
	}
	f.queries[name] = skip
}

// take reports whether the call named name fails, once.
func (f *historyFaults) take(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.armed[name] {
		return nil
	}
	delete(f.armed, name)
	return errHistoryInjected
}

// pending is what is armed and has not fired yet, so a test sees a fault that
// never fired (a branch it thought it reached).
func (f *historyFaults) pending() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for name := range f.armed {
		out = append(out, name)
	}
	for name := range f.queries {
		out = append(out, "query "+name)
	}
	sort.Strings(out)
	return out
}

// sqlcName finds the "-- name: X :kind" line sqlc puts at the top of a query.
var sqlcName = regexp.MustCompile(`^-- name: (\w+) `)

// TraceQueryStart cancels the context of the armed query, so it fails before
// anything is sent.
func (f *historyFaults) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	m := sqlcName.FindStringSubmatch(data.SQL)
	if m == nil {
		return ctx
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	skip, ok := f.queries[m[1]]
	if !ok {
		return ctx
	}
	if skip > 0 {
		f.queries[m[1]] = skip - 1
		return ctx
	}
	delete(f.queries, m[1])
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	return canceled
}

// TraceQueryEnd is the other half of pgx.QueryTracer.
func (*historyFaults) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

type historyFaultParty struct {
	inner Party
	f     *historyFaults
}

func (p historyFaultParty) Names(ctx context.Context, campaignID string, ids []string) (map[string]string, error) {
	if err := p.f.take("names"); err != nil {
		return nil, err
	}
	return p.inner.Names(ctx, campaignID, ids)
}

func (p historyFaultParty) Party(ctx context.Context, tx pgx.Tx, campaignID string) ([]link.Member, error) {
	return p.inner.Party(ctx, tx, campaignID)
}

func (p historyFaultParty) AddExperience(ctx context.Context, tx pgx.Tx, campaignID, characterID string, delta int32, at time.Time) (int32, int32, error) {
	return p.inner.AddExperience(ctx, tx, campaignID, characterID, delta, at)
}

type historyFaultCombats struct {
	Combats
	f *historyFaults
}

func (c historyFaultCombats) EncounterNames(ctx context.Context, campaignID string, ids []string) (map[string]string, error) {
	if err := c.f.take("encounter_names"); err != nil {
		return nil, err
	}
	return c.Combats.EncounterNames(ctx, campaignID, ids)
}

type historyFaultProfiles struct {
	Profiles
	f *historyFaults
}

func (p historyFaultProfiles) DisplayNames(ctx context.Context, userIDs []string) (map[string]string, error) {
	if err := p.f.take("display_names"); err != nil {
		return nil, err
	}
	return p.Profiles.DisplayNames(ctx, userIDs)
}

type historyFaultCampaigns struct {
	Campaigns
	f *historyFaults
}

func (c historyFaultCampaigns) CampaignXPMode(ctx context.Context, tx pgx.Tx, campaignID string) (campaignsv1.XpMode, error) {
	name := "xp_mode"
	if tx != nil {
		name = "xp_mode_in_tx"
	}
	if err := c.f.take(name); err != nil {
		return 0, err
	}
	return c.Campaigns.CampaignXPMode(ctx, tx, campaignID)
}

// historyLogBuffer is the JSON log of the rig's service.
type historyLogBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *historyLogBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// historyRig is a table served by a progression service of its own.
type historyRig struct {
	*table
	faults *historyFaults
	logs   *historyLogBuffer
	client func(u *user) progressionv1connect.ProgressionServiceClient
	// labels names the IDs in the goldens; unnamed UUIDs are numbered.
	labels  map[string]string
	unnamed map[string]string
}

// newHistoryRig is newTable whose master and players call a progression
// service that logs at debug level into the rig and goes through its faults.
func newHistoryRig(t *testing.T, mode campaignsv1.XpMode, n int) *historyRig {
	t.Helper()
	tb := newTable(t, mode, n)
	rig := &historyRig{table: tb, faults: &historyFaults{}, logs: &historyLogBuffer{}, labels: map[string]string{}, unnamed: map[string]string{}}
	pool, err := db.NewPoolWith(t.Context(), tb.h.pool.Config().ConnString(), func(cfg *pgxpool.Config) {
		cfg.MaxConns = 2
		cfg.ConnConfig.Tracer = rig.faults
	})
	if err != nil {
		t.Fatalf("NewPoolWith() error = %v", err)
	}
	t.Cleanup(pool.Close)
	cfg := tb.h.cfg
	cfg.Pool = pool
	cfg.Logger = slog.New(slog.NewJSONHandler(rig.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg.Party = historyFaultParty{inner: cfg.Party, f: rig.faults}
	cfg.Combats = historyFaultCombats{Combats: cfg.Combats, f: rig.faults}
	cfg.Profiles = historyFaultProfiles{Profiles: cfg.Profiles, f: rig.faults}
	cfg.Campaigns = historyFaultCampaigns{Campaigns: cfg.Campaigns, f: rig.faults}
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	svc.Mount(mux.Handle, fakeSessions{}, tb.h.camps, connect.WithRequireConnectProtocolHeader())
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	rig.client = func(u *user) progressionv1connect.ProgressionServiceClient {
		next := ts.Client().Transport
		if u.id == "" {
			return progressionv1connect.NewProgressionServiceClient(ts.Client(), ts.URL)
		}
		return progressionv1connect.NewProgressionServiceClient(&http.Client{Transport: userTransport{userID: u.id, next: next}}, ts.URL)
	}
	for _, u := range append([]*user{tb.master}, tb.players...) {
		u.xp = rig.client(u)
	}
	rig.label(tb.campaign, "campaign")
	rig.label(tb.master.id, "master")
	for i, pc := range tb.pcs {
		rig.label(pc.GetId(), strings.ToLower(pc.GetName()))
		rig.label(tb.players[i].id, "player-"+strings.ToLower(pc.GetName()))
	}
	return rig
}

// label names an ID in the goldens.
func (rig *historyRig) label(id, name string) { rig.labels[id] = "<" + name + ">" }

var historyUUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// redact puts the labels in place of the IDs, and numbers the others in the
// order the rig first meets them.
func (rig *historyRig) redact(s string) string {
	return historyUUID.ReplaceAllStringFunc(s, func(id string) string {
		if l, ok := rig.labels[id]; ok {
			return l
		}
		if l, ok := rig.unnamed[id]; ok {
			return l
		}
		l := fmt.Sprintf("<uuid%d>", len(rig.unnamed)+1)
		rig.unnamed[id] = l
		return l
	})
}

// historyTime is a timestamp as protojson writes it.
var historyTime = regexp.MustCompile(`"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z"`)

// redactTimes puts the rank of each time, oldest first, in its place: the
// clock is the test's, the order is the behavior.
func redactTimes(t *testing.T, s string) string {
	t.Helper()
	seen := map[string]time.Time{}
	for _, raw := range historyTime.FindAllString(s, -1) {
		at, err := time.Parse(time.RFC3339Nano, strings.Trim(raw, `"`))
		if err != nil {
			t.Fatalf("parse the time %s: %v", raw, err)
		}
		seen[raw] = at
	}
	ordered := make([]string, 0, len(seen))
	for raw := range seen {
		ordered = append(ordered, raw)
	}
	sort.Slice(ordered, func(i, j int) bool { return seen[ordered[i]].Before(seen[ordered[j]]) })
	rank := make(map[string]string, len(ordered))
	for i, raw := range ordered {
		rank[raw] = fmt.Sprintf(`"<t%02d>"`, i+1)
	}
	return historyTime.ReplaceAllStringFunc(s, func(raw string) string { return rank[raw] })
}

// historyRead is one read of a golden: the answer as JSON, or the refusal.
func (rig *historyRig) historyRead(t *testing.T, msg proto.Message, err error) any {
	t.Helper()
	if err != nil {
		return rig.redact(historyRefusal(err))
	}
	raw, merr := protojson.Marshal(msg)
	if merr != nil {
		t.Fatal(merr)
	}
	var tree any
	if err := json.Unmarshal([]byte(rig.redact(string(raw))), &tree); err != nil {
		t.Fatal(err)
	}
	return tree
}

// wantHistoryGolden compares what a viewer read (or what a test's steps did)
// to its file in testdata, the IDs replaced by labels and the times by their
// rank.
func (rig *historyRig) wantHistoryGolden(t *testing.T, name string, reads map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // the labels keep their angle brackets
	enc.SetIndent("", "  ")
	if err := enc.Encode(reads); err != nil {
		t.Fatal(err)
	}
	got := []byte(redactTimes(t, rig.redact(buf.String())))
	path := filepath.Join("testdata", name+".golden.json")
	if *updateHistoryGolden {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the golden file: %v (write it with -update-history-golden)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from %s:\n--- got\n%s\n--- want\n%s", name, path, got, want)
	}
}

// takeLogs returns the service's log lines written since the last call, but
// the authorization's: the level, the message and the attributes sorted,
// without the time.
func (rig *historyRig) takeLogs(t *testing.T) []string {
	t.Helper()
	rig.logs.mu.Lock()
	raw := rig.logs.b.String()
	rig.logs.b.Reset()
	rig.logs.mu.Unlock()
	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		msg, _ := rec["msg"].(string)
		if !strings.HasPrefix(msg, "progression:") {
			continue
		}
		keys := make([]string, 0, len(rec))
		for k := range rec {
			if k != "time" && k != "level" && k != "msg" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		parts := []string{fmt.Sprint(rec["level"]), msg}
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%v", k, rec[k]))
		}
		out = append(out, rig.redact(strings.Join(parts, " ")))
	}
	return out
}

// historyRefusal is a refusal as the app reads it: the code, the message and
// the XPBlocked detail, when there is one.
func historyRefusal(err error) string {
	if err == nil {
		return "ok"
	}
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok {
		return "not a connect error: " + err.Error()
	}
	out := ce.Code().String() + ": " + ce.Message()
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			continue
		}
		if b, ok := v.(*progressionv1.XPBlocked); ok {
			out += fmt.Sprintf(" [reason=%s character=%s xp_mode=%s treasure=%s]", b.GetReason(), b.GetCharacterId(), b.GetXpMode(), b.GetTreasurePointId())
		}
	}
	return out
}

// wantHistoryLines fails the test unless got is want, line for line, and
// prints got as Go so a reader sees the whole difference.
func wantHistoryLines(t *testing.T, what string, got, want []string) {
	t.Helper()
	if slices.Equal(got, want) {
		return
	}
	var b strings.Builder
	for _, g := range got {
		fmt.Fprintf(&b, "\t%q,\n", g)
	}
	t.Errorf("%s:\n got:\n%s want:\n%s", what, b.String(), strings.Join(want, "\n"))
}

// plannedRows is the campaign's planned_milestones as they are stored, by
// position: the position, the text, and whether a key and a hash were kept.
func (rig *historyRig) plannedRows(t *testing.T) []string {
	t.Helper()
	rows, err := rig.h.pool.Query(t.Context(), `
		SELECT position, text, create_key IS NOT NULL, create_hash IS NOT NULL
		FROM planned_milestones WHERE campaign_id = $1 ORDER BY position, id`, rig.campaign)
	if err != nil {
		t.Fatalf("read the planned milestones: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var position int32
		var text string
		var key, hash bool
		if err := rows.Scan(&position, &text, &key, &hash); err != nil {
			t.Fatalf("scan a planned milestone: %v", err)
		}
		out = append(out, fmt.Sprintf("%d %s key=%v hash=%v", position, text, key, hash))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read the planned milestones: %v", err)
	}
	return out
}

// campaignEvents is the number of session events of the campaign.
func (rig *historyRig) campaignEvents(t *testing.T) int {
	t.Helper()
	var n int
	if err := rig.h.pool.QueryRow(t.Context(), `
		SELECT count(*) FROM session_events e JOIN game_sessions s ON s.id = e.game_session_id WHERE s.campaign_id = $1`, rig.campaign).Scan(&n); err != nil {
		t.Fatalf("count the session events: %v", err)
	}
	return n
}

// wantNoPendingHistoryFault fails the test if an armed fault never fired.
func (rig *historyRig) wantNoPendingHistoryFault(t *testing.T, call string) {
	t.Helper()
	if p := rig.faults.pending(); len(p) > 0 {
		t.Errorf("%s: armed faults that never fired: %v", call, p)
		rig.faults.mu.Lock()
		rig.faults.armed, rig.faults.queries = nil, nil
		rig.faults.mu.Unlock()
	}
}
