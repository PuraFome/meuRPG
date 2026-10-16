package campaigns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// Characterization tests of the four biggest handlers of the package
// (CreateCampaign, acceptInvite, SetCampaignXpMode, UpdateCampaignDocument):
// every way they fail, with the action and the wrapped error they log, the
// order of their checks, and the events they log. They freeze today's
// behavior so the handlers can be split into phases without changing it.
//
// A database error is made by campaignsQueryFaults: a pgx tracer on the
// service's own pool that hands one named sqlc query an already canceled
// context. pgx then fails that query before sending anything, so the
// connection and the transaction stay usable, and the handler goes down the
// same path as for any other database error. A canceled context is classified
// by rpcerr as Canceled, which is the code these tests see.

// errCampaignsInjected is the cause of every injected failure.
var errCampaignsInjected = errors.New("injected by the characterization test")

// campaignsQueryFaults fails the sqlc queries it is armed with, by name.
// It wraps the test pool's tracer, so the nested-acquisition guard of
// dbtest still watches the pool.
type campaignsQueryFaults struct {
	inner pgx.QueryTracer
	mu    sync.Mutex
	armed map[string]bool
	hits  map[string]int
}

// sqlcName returns the name in the "-- name: X :kind" line sqlc starts
// every query with, or "" for a query that is not sqlc's.
func sqlcName(sql string) string {
	rest, ok := strings.CutPrefix(sql, "-- name: ")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(rest, " ")
	return name
}

func (f *campaignsQueryFaults) arm(names ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range names {
		f.armed[n] = true
	}
}

func (f *campaignsQueryFaults) hitCount(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[name]
}

func (f *campaignsQueryFaults) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = f.inner.TraceQueryStart(ctx, conn, data)
	name := sqlcName(data.SQL)
	f.mu.Lock()
	armed := f.armed[name]
	if armed {
		f.hits[name]++
	}
	f.mu.Unlock()
	if !armed {
		return ctx
	}
	failed, cancel := context.WithCancelCause(ctx)
	cancel(errCampaignsInjected)
	return failed
}

func (f *campaignsQueryFaults) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	f.inner.TraceQueryEnd(ctx, conn, data)
}

func (f *campaignsQueryFaults) TraceAcquireStart(ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireStartData) context.Context {
	if a, ok := f.inner.(pgxpool.AcquireTracer); ok {
		return a.TraceAcquireStart(ctx, pool, data)
	}
	return ctx
}

func (f *campaignsQueryFaults) TraceAcquireEnd(ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	if a, ok := f.inner.(pgxpool.AcquireTracer); ok {
		a.TraceAcquireEnd(ctx, pool, data)
	}
}

// campaignsLogTape keeps every log record of the service as JSON, debug
// included.
type campaignsLogTape struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *campaignsLogTape) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// records returns the logged records, decoded, without the time.
func (l *campaignsLogTape) records(t *testing.T) []map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(l.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		delete(rec, "time")
		out = append(out, rec)
	}
	return out
}

func (l *campaignsLogTape) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Reset()
}

// flakyCampaignProfiles is the identity store with errors on demand.
type flakyCampaignProfiles struct {
	Profiles
	mu                 sync.Mutex
	failDisplayNames   bool
	failVerifiedEmails bool
}

var errCampaignsProfiles = errors.New("profiles are down")

func (p *flakyCampaignProfiles) DisplayNames(ctx context.Context, userIDs []string) (map[string]string, error) {
	p.mu.Lock()
	fail := p.failDisplayNames
	p.mu.Unlock()
	if fail {
		return nil, errCampaignsProfiles
	}
	return p.Profiles.DisplayNames(ctx, userIDs)
}

func (p *flakyCampaignProfiles) VerifiedEmails(ctx context.Context, userID string) ([]string, error) {
	p.mu.Lock()
	fail := p.failVerifiedEmails
	p.mu.Unlock()
	if fail {
		return nil, errCampaignsProfiles
	}
	return p.Profiles.VerifiedEmails(ctx, userID)
}

// failingCampaignCharacters refuses every approval.
type failingCampaignCharacters struct{}

var errCampaignsCharacters = errors.New("characters are down")

func (failingCampaignCharacters) ApprovePendingCharacter(context.Context, pgx.Tx, string, string) error {
	return errCampaignsCharacters
}

// fakeCampaignXPAwards answers AwardedXP with fixed values.
type fakeCampaignXPAwards struct {
	awards int32
	total  int64
	err    error
}

func (f fakeCampaignXPAwards) AwardedXP(context.Context, pgx.Tx, string) (int32, int64, error) {
	return f.awards, f.total, f.err
}

// campaignsFaultRig is a harness whose service runs on a pool with
// campaignsQueryFaults, logs to a campaignsLogTape and reads profiles
// through flakyCampaignProfiles.
type campaignsFaultRig struct {
	*harness
	faults   *campaignsQueryFaults
	logs     *campaignsLogTape
	profiles *flakyCampaignProfiles
}

func newCampaignsFaultRig(t *testing.T, change func(*Config)) *campaignsFaultRig {
	t.Helper()
	rig := &campaignsFaultRig{logs: &campaignsLogTape{}}
	rig.harness = newHarnessWith(t, func(cfg *Config) {
		rig.faults = &campaignsQueryFaults{
			inner: cfg.Pool.Config().ConnConfig.Tracer,
			armed: map[string]bool{},
			hits:  map[string]int{},
		}
		pool, err := db.NewPoolWith(t.Context(), cfg.Pool.Config().ConnString(), func(pc *pgxpool.Config) {
			pc.MaxConns = 1
			pc.ConnConfig.Tracer = rig.faults
		})
		if err != nil {
			t.Fatalf("NewPoolWith() error = %v", err)
		}
		t.Cleanup(pool.Close)
		cfg.Pool = pool
		rig.profiles = &flakyCampaignProfiles{Profiles: cfg.Profiles}
		cfg.Profiles = rig.profiles
		cfg.Logger = slog.New(slog.NewJSONHandler(rig.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		if change != nil {
			change(cfg)
		}
	})
	return rig
}

// wantCampaignsFailureLog checks that a call failed with code, and that the
// service logged exactly one record: the action and the error it wrapped.
func (rig *campaignsFaultRig) wantCampaignsFailureLog(t *testing.T, call string, err error, code connect.Code, msg, wantErr string) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != code {
		t.Fatalf("%s error = %v, want %v", call, err, code)
	}
	recs := rig.logs.records(t)
	if len(recs) != 1 {
		t.Fatalf("%s logged %d records, want 1: %v", call, len(recs), recs)
	}
	if got := recs[0]["msg"]; got != msg {
		t.Errorf("%s logged msg %q, want %q", call, got, msg)
	}
	if got := recs[0]["error"]; got != wantErr {
		t.Errorf("%s logged error %q, want %q", call, got, wantErr)
	}
	if got := recs[0]["code"]; got != code.String() {
		t.Errorf("%s logged code %q, want %q", call, got, code.String())
	}
}

// injectedCampaignsCause is how pgx reports a query whose context was
// already canceled (the "timeout:" is pgx's word for any context error).
const injectedCampaignsCause = "timeout: context already done: context canceled"

// injectedCampaignsErr is how an injected query failure reads once wrapped
// by the handler with prefix.
func injectedCampaignsErr(prefix string) string {
	return prefix + ": " + injectedCampaignsCause
}

func TestCharacterizeCreateCampaignFailures(t *testing.T) {
	t.Parallel()

	t.Run("the order of the checks", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, func(c *Config) { c.Creators = []string{"someone@example.com"} })
		u := rig.newUser("Mestre")
		cases := []struct {
			name    string
			client  func() error
			code    connect.Code
			message string
		}{
			{"no session wins over a bad name", func() error {
				_, err := rig.anonymous().CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{}))
				return err
			}, connect.CodeUnauthenticated, "sign in to continue"},
			{"a bad name wins over a bad xp mode", func() error {
				_, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{Name: "  "}))
				return err
			}, connect.CodeInvalidArgument, "name must not be empty"},
			{"a bad xp mode wins over the allow-list", func() error {
				_, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{Name: "Mesa"}))
				return err
			}, connect.CodeInvalidArgument, "xp_mode must be enemies, gold or milestones"},
			{"the allow-list wins over a bad key", func() error {
				_, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{
					Name: "Mesa", XpMode: campaignsv1.XpMode_XP_MODE_GOLD, IdempotencyKey: "bad\x00key",
				}))
				return err
			}, connect.CodePermissionDenied, "this server does not let this account create campaigns"},
		}
		for _, tc := range cases {
			err := tc.client()
			if connect.CodeOf(err) != tc.code || connectMessage(err) != tc.message {
				t.Errorf("%s: CreateCampaign() error = %v, want %v %q", tc.name, err, tc.code, tc.message)
			}
		}
		if recs := rig.logs.records(t); len(recs) != 0 {
			t.Errorf("refusals logged %v, want nothing", recs)
		}
	})

	t.Run("a bad key after every other check", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		u := rig.newUser("Mestre")
		_, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{
			Name: "Mesa", XpMode: campaignsv1.XpMode_XP_MODE_GOLD, IdempotencyKey: "bad\x00key",
		}))
		wantCode(t, "CreateCampaign(bad key)", err, connect.CodeInvalidArgument)
	})

	t.Run("the allow-list cannot be read", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, func(c *Config) { c.Creators = []string{"someone@example.com"} })
		u := rig.newUser("Mestre")
		rig.profiles.mu.Lock()
		rig.profiles.failVerifiedEmails = true
		rig.profiles.mu.Unlock()
		err := u.create(t)
		rig.wantCampaignsFailureLog(t, "CreateCampaign()", err, connect.CodeInternal,
			"campaigns: cannot check who may create campaigns", "read the caller's verified e-mails: profiles are down")
	})

	for _, tc := range []struct {
		query, wantErr string
	}{
		{"GetCampaignByCreateKey", injectedCampaignsCause},
		{"CountMasteredCampaigns", injectedCampaignsErr("count the campaigns the caller is master of")},
		{"InsertCampaign", injectedCampaignsErr("insert campaign")},
		{"InsertMember", injectedCampaignsErr("insert master")},
	} {
		t.Run(tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			rig := newCampaignsFaultRig(t, nil)
			u := rig.newUser("Mestre")
			rig.faults.arm(tc.query)
			_, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{
				Name: "Mesa", XpMode: campaignsv1.XpMode_XP_MODE_GOLD, IdempotencyKey: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0a1",
			}))
			if rig.faults.hitCount(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			rig.wantCampaignsFailureLog(t, "CreateCampaign()", err, connect.CodeCanceled, "campaigns: cannot create a campaign", tc.wantErr)
			// Nothing was made: the campaign and its master go together.
			var n int
			if err := rig.pool.QueryRow(t.Context(), "SELECT count(*) FROM campaigns").Scan(&n); err != nil {
				t.Fatalf("count campaigns: %v", err)
			}
			if n != 0 {
				t.Errorf("campaigns after the failure = %d, want 0", n)
			}
		})
	}

	t.Run("the cap is not counted when there is none", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, func(c *Config) { c.MaxCampaignsPerUser = -1 })
		u := rig.newUser("Mestre")
		rig.faults.arm("CountMasteredCampaigns")
		if err := u.create(t); err != nil {
			t.Fatalf("CreateCampaign() without a cap error = %v", err)
		}
		if rig.faults.hitCount("CountMasteredCampaigns") != 0 {
			t.Errorf("CountMasteredCampaigns ran without a cap")
		}
	})
}

// acceptInviteWorld is a campaign with a master, a player-to-be and invites.
type acceptInviteWorld struct {
	rig        *campaignsFaultRig
	campaignID string
	master     *user
	guest      *user
}

func newAcceptInviteWorld(t *testing.T) *acceptInviteWorld {
	t.Helper()
	rig := newCampaignsFaultRig(t, nil)
	w := &acceptInviteWorld{rig: rig, master: rig.newUser("Mestre"), guest: rig.newUser("Convidada")}
	w.campaignID = w.master.createCampaign(t, "Mirathel").GetId()
	rig.logs.reset()
	return w
}

// wantAcceptedEvents checks the invite.accepted events logged since the last
// reset: one per entry of pending (its pending_approval), in order.
func (w *acceptInviteWorld) wantAcceptedEvents(t *testing.T, pending ...bool) {
	t.Helper()
	recs := w.rig.logs.records(t)
	if len(recs) != len(pending) {
		t.Fatalf("logged %v, want %d invite.accepted events", recs, len(pending))
	}
	for i, p := range pending {
		rec := recs[i]
		want := map[string]any{
			"level": "DEBUG", "msg": "event", "event": "invite.accepted",
			"campaign_id": w.campaignID, "pending_approval": p,
		}
		if len(rec) != len(want) {
			t.Errorf("event %d = %v, want %v", i, rec, want)
			continue
		}
		for k, v := range want {
			if rec[k] != v {
				t.Errorf("event %d = %v, want %v", i, rec, want)
				break
			}
		}
	}
	w.rig.logs.reset()
}

func TestCharacterizeAcceptInvite(t *testing.T) {
	t.Parallel()

	t.Run("the events of each way in", func(t *testing.T) {
		t.Parallel()
		w := newAcceptInviteWorld(t)
		_, token := w.master.createInvite(t, w.campaignID, 2, 0)
		w.guest.join(t, token)
		w.wantAcceptedEvents(t, false)
		// Again: already a member, nothing is logged.
		w.guest.join(t, token)
		w.wantAcceptedEvents(t)

		// A pending member, then promoted by an invite without approval
		// (Q25): the promotion logs pending_approval false.
		other := w.rig.newUser("Outra")
		_, approval := w.master.createApprovalInvite(t, w.campaignID)
		res, err := other.accept(t, approval)
		if err != nil || !res.GetCampaign().GetAwaitingApproval() {
			t.Fatalf("AcceptInvite(approval) = %v, %v, want pending", res, err)
		}
		w.wantAcceptedEvents(t, true)
		// A pending member again with the same invite: nothing changes.
		if res, err := other.accept(t, approval); err != nil || !res.GetCampaign().GetAwaitingApproval() {
			t.Fatalf("AcceptInvite(approval) again = %v, %v, want pending", res, err)
		}
		w.wantAcceptedEvents(t)
		res, err = other.accept(t, token)
		if err != nil || res.GetCampaign().GetAwaitingApproval() || res.GetCampaign().GetMyRole() != campaignsv1.Role_ROLE_PLAYER {
			t.Fatalf("AcceptInvite(promotion) = %v, %v, want an active player", res, err)
		}
		w.wantAcceptedEvents(t, false)
		if got := w.rig.memberStatus(w.campaignID, other.id); got != "active" {
			t.Errorf("status after the promotion = %q, want active", got)
		}
	})

	t.Run("a pending member is not promoted by an invite that does not work", func(t *testing.T) {
		t.Parallel()
		w := newAcceptInviteWorld(t)
		_, approval := w.master.createApprovalInvite(t, w.campaignID)
		w.guest.join(t, approval)
		invite, token := w.master.createInvite(t, w.campaignID, 1, 0)
		if _, err := w.master.api.RevokeInvite(t.Context(), connect.NewRequest(&campaignsv1.RevokeInviteRequest{
			CampaignId: w.campaignID, InviteId: invite.GetId(),
		})); err != nil {
			t.Fatalf("RevokeInvite() error = %v", err)
		}
		w.rig.logs.reset()
		// FINDING (frozen): a pending member who accepts a revoked invite
		// gets an answer of success (still pending), not the revoked
		// state a stranger would get, because the membership is checked
		// before the invite's state.
		res, err := w.guest.accept(t, token)
		if err != nil || !res.GetCampaign().GetAwaitingApproval() {
			t.Fatalf("AcceptInvite(revoked) as pending = %v, %v, want still pending", res, err)
		}
		w.wantAcceptedEvents(t)
		if got := w.rig.useCount(invite.GetId()); got != 0 {
			t.Errorf("use_count = %d, want 0", got)
		}
	})

	t.Run("a pending member is not promoted by another approval invite", func(t *testing.T) {
		t.Parallel()
		w := newAcceptInviteWorld(t)
		_, first := w.master.createApprovalInvite(t, w.campaignID)
		w.guest.join(t, first)
		second, token := w.master.createApprovalInvite(t, w.campaignID)
		w.rig.logs.reset()
		res, err := w.guest.accept(t, token)
		if err != nil || !res.GetCampaign().GetAwaitingApproval() {
			t.Fatalf("AcceptInvite(second approval) = %v, %v, want still pending", res, err)
		}
		w.wantAcceptedEvents(t)
		if got := w.rig.useCount(second.GetId()); got != 0 {
			t.Errorf("use_count = %d, want 0", got)
		}
	})

	t.Run("promotion without SetCharacters", func(t *testing.T) {
		t.Parallel()
		w := newAcceptInviteWorld(t)
		_, approval := w.master.createApprovalInvite(t, w.campaignID)
		w.guest.join(t, approval)
		_, token := w.master.createInvite(t, w.campaignID, 1, 0)
		w.rig.service.SetCharacters(nil)
		w.rig.logs.reset()
		_, err := w.guest.accept(t, token)
		w.rig.wantCampaignsFailureLog(t, "AcceptInvite()", err, connect.CodeInternal,
			"campaigns: cannot accept an invite", "campaigns: SetCharacters was not called")
	})

	t.Run("promotion when the approval of the character fails", func(t *testing.T) {
		t.Parallel()
		w := newAcceptInviteWorld(t)
		_, approval := w.master.createApprovalInvite(t, w.campaignID)
		w.guest.join(t, approval)
		invite, token := w.master.createInvite(t, w.campaignID, 1, 0)
		w.rig.service.SetCharacters(failingCampaignCharacters{})
		w.rig.logs.reset()
		_, err := w.guest.accept(t, token)
		w.rig.wantCampaignsFailureLog(t, "AcceptInvite()", err, connect.CodeInternal,
			"campaigns: cannot accept an invite", "characters are down")
		// The whole promotion rolled back.
		if got := w.rig.memberStatus(w.campaignID, w.guest.id); got != "pending" {
			t.Errorf("status = %q, want pending", got)
		}
		if got := w.rig.useCount(invite.GetId()); got != 0 {
			t.Errorf("use_count = %d, want 0", got)
		}
	})

	for _, tc := range []struct {
		query, wantErr string
	}{
		{"IncrementInviteUses", injectedCampaignsErr("spend an invite use")},
		{"ActivatePendingMember", injectedCampaignsErr("activate pending member")},
	} {
		t.Run("promotion when "+tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			w := newAcceptInviteWorld(t)
			_, approval := w.master.createApprovalInvite(t, w.campaignID)
			w.guest.join(t, approval)
			_, token := w.master.createInvite(t, w.campaignID, 1, 0)
			w.rig.logs.reset()
			w.rig.faults.arm(tc.query)
			_, err := w.guest.accept(t, token)
			w.rig.wantCampaignsFailureLog(t, "AcceptInvite()", err, connect.CodeCanceled, "campaigns: cannot accept an invite", tc.wantErr)
		})
	}

	for _, tc := range []struct {
		query, wantErr string
	}{
		{"GetInviteByTokenHashForUpdate", injectedCampaignsErr("get invite")},
		{"GetCampaign", injectedCampaignsErr("get campaign")},
		{"GetMembership", injectedCampaignsErr("get membership")},
		{"DeleteExpiredPendingMember", injectedCampaignsErr("delete expired pending member")},
		{"IncrementInviteUses", injectedCampaignsErr("spend an invite use")},
		{"InsertMember", injectedCampaignsErr("insert player")},
	} {
		t.Run(tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			w := newAcceptInviteWorld(t)
			invite, token := w.master.createInvite(t, w.campaignID, 1, 0)
			w.rig.logs.reset()
			w.rig.faults.arm(tc.query)
			_, err := w.guest.accept(t, token)
			if w.rig.faults.hitCount(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			w.rig.wantCampaignsFailureLog(t, "AcceptInvite()", err, connect.CodeCanceled, "campaigns: cannot accept an invite", tc.wantErr)
			if got := w.rig.useCount(invite.GetId()); got != 0 {
				t.Errorf("use_count = %d, want 0", got)
			}
			if got := w.rig.memberStatus(w.campaignID, w.guest.id); got != "" {
				t.Errorf("status = %q, want no membership", got)
			}
		})
	}
}

func TestCharacterizeSetCampaignXpModeFailures(t *testing.T) {
	t.Parallel()

	setMode := func(t *testing.T, u *user, campaignID string, mode campaignsv1.XpMode, confirm bool) (*campaignsv1.SetCampaignXpModeResponse, error) {
		t.Helper()
		res, err := u.api.SetCampaignXpMode(t.Context(), connect.NewRequest(&campaignsv1.SetCampaignXpModeRequest{
			CampaignId: campaignID, XpMode: mode, Confirm: confirm,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}

	t.Run("AwardedXP fails", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		m := rig.newUser("Mestre")
		id := m.createCampaign(t, "Mesa").GetId()
		rig.service.SetXPAwards(fakeCampaignXPAwards{err: errors.New("progression is down")})
		rig.logs.reset()
		_, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_GOLD, false)
		rig.wantCampaignsFailureLog(t, "SetCampaignXpMode()", err, connect.CodeInternal, "campaigns: cannot change the xp mode", "progression is down")
	})

	t.Run("a confirmed change never asks AwardedXP", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		m := rig.newUser("Mestre")
		id := m.createCampaign(t, "Mesa").GetId()
		rig.service.SetXPAwards(fakeCampaignXPAwards{err: errors.New("progression is down")})
		res, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_GOLD, true)
		if err != nil || res.GetXpMode() != campaignsv1.XpMode_XP_MODE_GOLD || !res.GetChangedAt().AsTime().Equal(rig.clock.Now()) {
			t.Fatalf("SetCampaignXpMode(confirm) = %v, %v, want gold changed now", res, err)
		}
	})

	t.Run("no awards yet changes without a confirmation", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		m := rig.newUser("Mestre")
		id := m.createCampaign(t, "Mesa").GetId()
		rig.service.SetXPAwards(fakeCampaignXPAwards{})
		res, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_ENEMIES, false)
		if err != nil || res.GetXpMode() != campaignsv1.XpMode_XP_MODE_ENEMIES || res.GetChangedAt() == nil {
			t.Fatalf("SetCampaignXpMode() = %v, %v, want enemies with a time", res, err)
		}
		// The same mode again answers the time of the first change.
		rig.clock.Advance(1000)
		again, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_ENEMIES, false)
		if err != nil || !again.GetChangedAt().AsTime().Equal(res.GetChangedAt().AsTime()) {
			t.Fatalf("SetCampaignXpMode(same) = %v, %v, want the first change's time", again, err)
		}
	})

	t.Run("the same mode as created has no time", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		m := rig.newUser("Mestre")
		id := m.createCampaign(t, "Mesa").GetId()
		res, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_MILESTONES, false)
		if err != nil || res.GetXpMode() != campaignsv1.XpMode_XP_MODE_MILESTONES || res.ChangedAt != nil {
			t.Fatalf("SetCampaignXpMode(unchanged) = %v, %v, want milestones without a time", res, err)
		}
	})

	t.Run("the role check wins over a bad mode", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		m, p := rig.newUser("Mestre"), rig.newUser("Jogadora")
		id := m.createCampaign(t, "Mesa").GetId()
		_, token := m.createInvite(t, id, 1, 0)
		p.join(t, token)
		_, err := setMode(t, p, id, campaignsv1.XpMode_XP_MODE_UNSPECIFIED, false)
		wantCode(t, "SetCampaignXpMode(player)", err, connect.CodePermissionDenied)
		_, err = setMode(t, m, id, campaignsv1.XpMode_XP_MODE_UNSPECIFIED, false)
		wantCode(t, "SetCampaignXpMode(unspecified)", err, connect.CodeInvalidArgument)
		if got := connectMessage(err); got != "xp_mode must be enemies, gold or milestones" {
			t.Errorf("message = %q", got)
		}
	})

	for _, tc := range []struct {
		query, wantErr string
	}{
		{"GetCampaignForUpdate", injectedCampaignsErr("lock the campaign")},
		{"SetCampaignXPMode", injectedCampaignsErr("set the xp mode")},
	} {
		t.Run(tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			rig := newCampaignsFaultRig(t, nil)
			m := rig.newUser("Mestre")
			id := m.createCampaign(t, "Mesa").GetId()
			rig.logs.reset()
			rig.faults.arm(tc.query)
			_, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_GOLD, false)
			if rig.faults.hitCount(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			rig.wantCampaignsFailureLog(t, "SetCampaignXpMode()", err, connect.CodeCanceled, "campaigns: cannot change the xp mode", tc.wantErr)
		})
	}
}

func TestCharacterizeUpdateCampaignDocumentFailures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, query, wantErr string
		saveFirst            bool
		expected             int32
	}{
		{"the first save", "InsertCampaignDocument", injectedCampaignsErr("save the campaign document"), false, 0},
		{"a later save", "UpdateCampaignDocument", injectedCampaignsErr("save the campaign document"), true, 1},
		{"the read after a stale save", "GetCampaignDocument", injectedCampaignsErr("read the campaign document"), true, 7},
	} {
		t.Run(tc.name+" fails", func(t *testing.T) {
			t.Parallel()
			rig := newCampaignsFaultRig(t, nil)
			m := rig.newUser("Mestre")
			id := m.createCampaign(t, "Mesa").GetId()
			if tc.saveFirst {
				mustSaveDocument(t, m.doc, id, "primeiro", 0)
			}
			rig.logs.reset()
			rig.faults.arm(tc.query)
			_, err := saveDocument(t, m.doc, id, "segundo", tc.expected)
			if rig.faults.hitCount(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			rig.wantCampaignsFailureLog(t, "UpdateCampaignDocument()", err, connect.CodeCanceled, "campaigns: cannot save the campaign document", tc.wantErr)
		})
	}

	t.Run("the editor's name cannot be read", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		m := rig.newUser("Mestre")
		id := m.createCampaign(t, "Mesa").GetId()
		rig.profiles.mu.Lock()
		rig.profiles.failDisplayNames = true
		rig.profiles.mu.Unlock()
		_, err := saveDocument(t, m.doc, id, "texto", 0)
		rig.wantCampaignsFailureLog(t, "UpdateCampaignDocument()", err, connect.CodeInternal,
			"campaigns: cannot save the campaign document", "read the editor's display name: profiles are down")
		// The save itself went through.
		if row, ok := rig.documentRow(id); !ok || row.body != "texto" {
			t.Errorf("document row = %+v, %v, want the saved text", row, ok)
		}
	})

	t.Run("the order of the checks", func(t *testing.T) {
		t.Parallel()
		rig := newCampaignsFaultRig(t, nil)
		m, p := rig.newUser("Mestre"), rig.newUser("Jogadora")
		id := m.createCampaign(t, "Mesa").GetId()
		_, token := m.createInvite(t, id, 1, 0)
		p.join(t, token)
		_, err := saveDocument(t, p.doc, id, "\x00", -1)
		wantCode(t, "UpdateCampaignDocument(player)", err, connect.CodePermissionDenied)
		_, err = saveDocument(t, m.doc, id, "\x00", -1)
		if connect.CodeOf(err) != connect.CodeInvalidArgument || connectMessage(err) != "expected_revision must not be negative" {
			t.Errorf("UpdateCampaignDocument(negative, bad body) error = %v, want the revision first", err)
		}
		_, err = saveDocument(t, m.doc, id, "\x00", 0)
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.HasPrefix(connectMessage(err), "body ") {
			t.Errorf("UpdateCampaignDocument(bad body) error = %v, want the body", err)
		}
		// A stale first save (expected > 0, never saved) and a stale later
		// save by someone else are the same conflict.
		_, err = saveDocument(t, m.doc, id, "texto", 3)
		wantCode(t, "UpdateCampaignDocument(never saved, expected 3)", err, connect.CodeAborted)
		mustSaveDocument(t, m.doc, id, "texto", 0)
		_, err = saveDocument(t, m.doc, id, "outro", 0)
		wantCode(t, "UpdateCampaignDocument(insert over a saved one)", err, connect.CodeAborted)
		_, err = saveDocument(t, m.doc, id, "texto", 0)
		if err != nil {
			t.Errorf("UpdateCampaignDocument(the same first save again) error = %v, want the saved document", err)
		}
	})
}
