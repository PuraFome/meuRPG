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
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
)

// What CreateCampaign, acceptInvite, SetCampaignXpMode and UpdateCampaignDocument
// answer when a query or another module fails, and the order of their checks.
//
// A database error is made by dbtest.QueryFaults: a pgx tracer on the
// service's own pool that hands one named sqlc query an already canceled
// context. pgx then fails that query before sending anything, so the
// connection and the transaction stay usable, and the handler goes down the
// same path as for any other database error. A canceled context is classified
// by rpcerr as Canceled, which is the code these tests see.

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

// errors counts the records logged at ERROR.
func (l *campaignsLogTape) errors(t *testing.T) int {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for line := range strings.SplitSeq(strings.TrimSpace(l.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		if rec["level"] == "ERROR" {
			n++
		}
	}
	return n
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
// dbtest.QueryFaults, logs to a campaignsLogTape and reads profiles
// through flakyCampaignProfiles.
type campaignsFaultRig struct {
	*harness
	faults   *dbtest.QueryFaults
	logs     *campaignsLogTape
	profiles *flakyCampaignProfiles
}

func newCampaignsFaultRig(t *testing.T, change func(*Config)) *campaignsFaultRig {
	t.Helper()
	rig := &campaignsFaultRig{logs: &campaignsLogTape{}}
	rig.harness = newHarnessWith(t, func(cfg *Config) {
		rig.faults = dbtest.NewQueryFaults(cfg.Pool.Config().ConnConfig.Tracer)
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

// wantFailure checks that a call failed with code.
func wantFailure(t *testing.T, call string, err error, code connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != code {
		t.Fatalf("%s error = %v, want %v", call, err, code)
	}
}

func TestCreateCampaignWhenSomethingFails(t *testing.T) {
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
		wantFailure(t, "CreateCampaign()", err, connect.CodeInternal)
		if rig.logs.errors(t) == 0 {
			t.Errorf("a failure the client cannot fix was not logged at ERROR")
		}
	})

	for _, tc := range []struct{ query string }{
		{"GetCampaignByCreateKey"},
		{"CountMasteredCampaigns"},
		{"InsertCampaign"},
		{"InsertMember"},
	} {
		t.Run(tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			rig := newCampaignsFaultRig(t, nil)
			u := rig.newUser("Mestre")
			rig.faults.Arm(tc.query)
			_, err := u.api.CreateCampaign(t.Context(), connect.NewRequest(&campaignsv1.CreateCampaignRequest{
				Name: "Mesa", XpMode: campaignsv1.XpMode_XP_MODE_GOLD, IdempotencyKey: "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0a1",
			}))
			if rig.faults.Hits(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			wantFailure(t, "CreateCampaign()", err, connect.CodeCanceled)
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
		rig.faults.Arm("CountMasteredCampaigns")
		if err := u.create(t); err != nil {
			t.Fatalf("CreateCampaign() without a cap error = %v", err)
		}
		if rig.faults.Hits("CountMasteredCampaigns") != 0 {
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
	return w
}

func TestAcceptInviteWhenSomethingFails(t *testing.T) {
	t.Parallel()

	t.Run("promotion without SetCharacters", func(t *testing.T) {
		t.Parallel()
		w := newAcceptInviteWorld(t)
		_, approval := w.master.createApprovalInvite(t, w.campaignID)
		w.guest.join(t, approval)
		_, token := w.master.createInvite(t, w.campaignID, 1, 0)
		w.rig.service.SetCharacters(nil)
		_, err := w.guest.accept(t, token)
		wantFailure(t, "AcceptInvite()", err, connect.CodeInternal)
	})

	t.Run("promotion when the approval of the character fails", func(t *testing.T) {
		t.Parallel()
		w := newAcceptInviteWorld(t)
		_, approval := w.master.createApprovalInvite(t, w.campaignID)
		w.guest.join(t, approval)
		invite, token := w.master.createInvite(t, w.campaignID, 1, 0)
		w.rig.service.SetCharacters(failingCampaignCharacters{})
		_, err := w.guest.accept(t, token)
		wantFailure(t, "AcceptInvite()", err, connect.CodeInternal)
		// The whole promotion rolled back.
		if got := w.rig.memberStatus(w.campaignID, w.guest.id); got != "pending" {
			t.Errorf("status = %q, want pending", got)
		}
		if got := w.rig.useCount(invite.GetId()); got != 0 {
			t.Errorf("use_count = %d, want 0", got)
		}
	})

	for _, tc := range []struct{ query string }{
		{"IncrementInviteUses"},
		{"ActivatePendingMember"},
	} {
		t.Run("promotion when "+tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			w := newAcceptInviteWorld(t)
			_, approval := w.master.createApprovalInvite(t, w.campaignID)
			w.guest.join(t, approval)
			_, token := w.master.createInvite(t, w.campaignID, 1, 0)
			w.rig.faults.Arm(tc.query)
			_, err := w.guest.accept(t, token)
			wantFailure(t, "AcceptInvite()", err, connect.CodeCanceled)
		})
	}

	for _, tc := range []struct{ query string }{
		{"GetInviteByTokenHashForUpdate"},
		{"GetCampaign"},
		{"GetMembership"},
		{"DeleteExpiredPendingMember"},
		{"IncrementInviteUses"},
		{"InsertMember"},
	} {
		t.Run(tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			w := newAcceptInviteWorld(t)
			invite, token := w.master.createInvite(t, w.campaignID, 1, 0)
			w.rig.faults.Arm(tc.query)
			_, err := w.guest.accept(t, token)
			if w.rig.faults.Hits(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			wantFailure(t, "AcceptInvite()", err, connect.CodeCanceled)
			if got := w.rig.useCount(invite.GetId()); got != 0 {
				t.Errorf("use_count = %d, want 0", got)
			}
			if got := w.rig.memberStatus(w.campaignID, w.guest.id); got != "" {
				t.Errorf("status = %q, want no membership", got)
			}
		})
	}
}

func TestSetCampaignXpModeWhenSomethingFails(t *testing.T) {
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
		_, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_GOLD, false)
		wantFailure(t, "SetCampaignXpMode()", err, connect.CodeInternal)
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

	for _, tc := range []struct{ query string }{
		{"GetCampaignForUpdate"},
		{"SetCampaignXPMode"},
	} {
		t.Run(tc.query+" fails", func(t *testing.T) {
			t.Parallel()
			rig := newCampaignsFaultRig(t, nil)
			m := rig.newUser("Mestre")
			id := m.createCampaign(t, "Mesa").GetId()
			rig.faults.Arm(tc.query)
			_, err := setMode(t, m, id, campaignsv1.XpMode_XP_MODE_GOLD, false)
			if rig.faults.Hits(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			wantFailure(t, "SetCampaignXpMode()", err, connect.CodeCanceled)
		})
	}
}

func TestUpdateCampaignDocumentWhenSomethingFails(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, query string
		saveFirst   bool
		expected    int32
	}{
		{"the first save", "InsertCampaignDocument", false, 0},
		{"a later save", "UpdateCampaignDocument", true, 1},
		{"the read after a stale save", "GetCampaignDocument", true, 7},
	} {
		t.Run(tc.name+" fails", func(t *testing.T) {
			t.Parallel()
			rig := newCampaignsFaultRig(t, nil)
			m := rig.newUser("Mestre")
			id := m.createCampaign(t, "Mesa").GetId()
			if tc.saveFirst {
				mustSaveDocument(t, m.doc, id, "primeiro", 0)
			}
			rig.faults.Arm(tc.query)
			_, err := saveDocument(t, m.doc, id, "segundo", tc.expected)
			if rig.faults.Hits(tc.query) == 0 {
				t.Fatalf("%s never ran", tc.query)
			}
			wantFailure(t, "UpdateCampaignDocument()", err, connect.CodeCanceled)
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
		wantFailure(t, "UpdateCampaignDocument()", err, connect.CodeInternal)
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
