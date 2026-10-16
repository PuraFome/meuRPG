package campaignpackage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/platform/ratelimit"
)

// What BeginCampaignImport, StartCampaignExport, the export's zip and Sweep
// answer and leave behind when a query or the blob store fails, and the order
// of their checks.
//
// A database error is made by dbtest.QueryFaults, a pgx tracer on the
// package's own pool that hands one named sqlc query an already canceled
// context: pgx fails that query before sending it, the connection stays
// usable, and the code goes down its path for any database error (rpcerr
// calls it Canceled). The blob store's failures come from pkgFaultyBlobs.

// pkgLogTape keeps every log record of the package as JSON, debug included.
type pkgLogTape struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *pkgLogTape) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// errors counts the records logged at ERROR.
func (l *pkgLogTape) errors(t *testing.T) int {
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

// pkgFaultyBlobs is the test's blob store with failures on demand. Only the
// images an export copies (keys with "/images/") and the export's own zip
// (keys with "/exports/") are touched; uploads and gallery images go through.
type pkgFaultyBlobs struct {
	blob.Store
	mu sync.Mutex
	// openErr fails the Open of an image.
	openErr error
	// readErr makes an opened image's content fail when read.
	readErr error
	// putErr fails the Put of an export's zip at once, reading nothing.
	putErr error
	// hold, when set, makes the next Open of an image close opened and wait
	// for hold to be closed.
	hold, opened chan struct{}
}

func (b *pkgFaultyBlobs) set(change func(b *pkgFaultyBlobs)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	change(b)
}

// Put drains the reader even after the export's context ends, so a canceled
// export fails by its own check of the context and not by a broken pipe.
func (b *pkgFaultyBlobs) Put(ctx context.Context, key, contentType string, r io.Reader) error {
	b.mu.Lock()
	putErr := b.putErr
	b.mu.Unlock()
	if putErr != nil && strings.Contains(key, "/exports/") {
		return putErr
	}
	return b.Store.Put(context.WithoutCancel(ctx), key, contentType, r)
}

type pkgFailingReader struct{ err error }

func (r pkgFailingReader) Read([]byte) (int, error)       { return 0, r.err }
func (r pkgFailingReader) Seek(int64, int) (int64, error) { return 0, r.err }

func (b *pkgFaultyBlobs) Open(ctx context.Context, key string) (*blob.Object, error) {
	if !strings.Contains(key, "/images/") {
		return b.Store.Open(ctx, key)
	}
	b.mu.Lock()
	openErr, readErr, hold, opened := b.openErr, b.readErr, b.hold, b.opened
	b.hold, b.opened = nil, nil
	b.mu.Unlock()
	if hold != nil {
		close(opened)
		<-hold
	}
	switch {
	case openErr != nil:
		return nil, openErr
	case readErr != nil:
		return &blob.Object{Content: pkgFailingReader{readErr}}, nil //nolint:nilerr // the fake hands its error to the reader
	}
	return b.Store.Open(context.WithoutCancel(ctx), key)
}

// pkgRig is a harness whose package service runs on a pool with
// dbtest.QueryFaults, logs to a pkgLogTape and stores through pkgFaultyBlobs.
type pkgRig struct {
	*harness
	faults *dbtest.QueryFaults
	logs   *pkgLogTape
	blobs  *pkgFaultyBlobs
}

func newPkgRig(t *testing.T, noBlobs bool, more func(*campaignpackage.Config)) *pkgRig {
	t.Helper()
	rig := &pkgRig{logs: &pkgLogTape{}}
	rig.harness = newHarness(t, func(o *options) {
		o.noBlobs = noBlobs
		o.configure = func(cfg *campaignpackage.Config) {
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
			cfg.Logger = slog.New(slog.NewJSONHandler(rig.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			if cfg.Blobs != nil {
				rig.blobs = &pkgFaultyBlobs{Store: cfg.Blobs}
				cfg.Blobs = rig.blobs
			}
			if more != nil {
				more(cfg)
			}
		}
	})
	return rig
}

// wantPkgDBFailure checks a call that failed on an injected query: the code
// Canceled.
func wantPkgDBFailure(t *testing.T, call string, err error) {
	t.Helper()
	if connect.CodeOf(err) != connect.CodeCanceled {
		t.Fatalf("%s error = %v, want canceled", call, err)
	}
}

func pkgWantRefusal(t *testing.T, call string, err error, code connect.Code, msg string) {
	t.Helper()
	ce, ok := errors.AsType[*connect.Error](err)
	if !ok || ce.Code() != code || ce.Message() != msg {
		t.Errorf("%s error = %v, want %v %q", call, err, code, msg)
	}
}

func TestBeginCampaignImportWhenSomethingFails(t *testing.T) {
	rig := newPkgRig(t, false, pkgHeavyLimit)
	master := rig.newUser("Mestre")
	ctx := t.Context()
	begin := func(u *user, name, fingerprint string, total int64) (*pkgv1.CampaignImport, error) {
		res, err := u.pkg.BeginCampaignImport(ctx, connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: name, Fingerprint: fingerprint, TotalBytes: total}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetUpload(), nil
	}

	// The order of the checks: the session, the name, the fingerprint, the size.
	_, err := begin(rig.clients(""), "", "", 0)
	pkgWantRefusal(t, "BeginCampaignImport(anonymous)", err, connect.CodeUnauthenticated, "sign in to continue")
	_, err = begin(master, " ", "", 0)
	pkgWantRefusal(t, "BeginCampaignImport(no name)", err, connect.CodeInvalidArgument, "file_name must not be empty")
	for _, fp := range []string{"", "   ", strings.Repeat("é", 201)} {
		_, err = begin(master, "a.zip", fp, 0)
		pkgWantRefusal(t, "BeginCampaignImport(fingerprint "+fp+")", err, connect.CodeInvalidArgument, "fingerprint must be 1 to 200 characters")
	}
	for _, total := range []int64{0, -1, campaignpackage.MaxPackageBytes + 1} {
		_, err = begin(master, "a.zip", "fp", total)
		pkgWantRefusal(t, "BeginCampaignImport(total)", err, connect.CodeInvalidArgument, "total_bytes must be 1 byte to 200 MiB")
	}

	// The edges that pass.
	up, err := begin(master, "a.zip", strings.Repeat("é", 200), campaignpackage.MaxPackageBytes)
	if err != nil || int(up.GetPartCount()) != campaignpackage.PartCount(campaignpackage.MaxPackageBytes) {
		t.Fatalf("BeginCampaignImport(edges) = %v, %v", up, err)
	}

	// Each failing query of the transaction.
	same, err := begin(master, "a.zip", "same", 10)
	if err != nil {
		t.Fatalf("BeginCampaignImport(same) error = %v", err)
	}
	for _, tc := range []struct {
		query, fingerprint string
	}{
		{"GetCampaignImportOfUser", "same"},
		{"ListCampaignImportParts", "same"},
		{"TouchCampaignImport", "same"},
		{"DeleteCampaignImport", "other"},
	} {
		rig.faults.Arm(tc.query)
		_, err := begin(master, "a.zip", tc.fingerprint, 10)
		rig.faults.Disarm()
		if rig.faults.Hits(tc.query) == 0 {
			t.Fatalf("%s never ran", tc.query)
		}
		wantPkgDBFailure(t, "BeginCampaignImport("+tc.query+")", err)
	}
	// The upload is unchanged by the failures, and the next call resumes it.
	again, err := begin(master, "a.zip", "same", 10)
	if err != nil || again.GetId() != same.GetId() {
		t.Fatalf("BeginCampaignImport(same) after the failures = %v, %v, want %s", again, err, same.GetId())
	}

	// Insert: with no upload before, the person has none after.
	other := rig.newUser("Outra")
	rig.faults.Arm("InsertCampaignImport")
	_, err = begin(other, "b.zip", "fp", 10)
	rig.faults.Disarm()
	wantPkgDBFailure(t, "BeginCampaignImport(InsertCampaignImport)", err)
	if n := rig.count(`SELECT count(*)::INT FROM campaign_imports WHERE user_id = $1`, other.id); n != 0 {
		t.Errorf("uploads after a failed insert = %d, want 0", n)
	}
	// Insert after a drop: the drop rolls back with it.
	rig.faults.Arm("InsertCampaignImport")
	_, err = begin(master, "c.zip", "third", 10)
	rig.faults.Disarm()
	wantPkgDBFailure(t, "BeginCampaignImport(drop then insert)", err)
	if n := rig.count(`SELECT count(*)::INT FROM campaign_imports WHERE id = $1`, same.GetId()); n != 1 {
		t.Errorf("the previous upload after a failed insert: %d rows, want 1", n)
	}

	// The rate limit comes after the checks of the request: spend the
	// bucket, then a bad request is still told it is bad.
	limited := rig.newUser("Apressada")
	for i := 0; ; i++ {
		if _, err := begin(limited, "a.zip", "same", 10); connect.CodeOf(err) == connect.CodeResourceExhausted {
			break
		} else if i > pkgLimitBurst {
			t.Fatalf("BeginCampaignImport() never hit the limit, last error %v", err)
		}
	}
	_, err = begin(limited, "a.zip", "same", 0)
	pkgWantRefusal(t, "BeginCampaignImport(limited, bad size)", err, connect.CodeInvalidArgument, "total_bytes must be 1 byte to 200 MiB")
	_, err = begin(limited, "a.zip", "same", 10)
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Errorf("BeginCampaignImport(over the limit) error = %v, want resource_exhausted", err)
	}
}

// pkgLimitBurst is the burst of the Heavy limiter of these tests: enough
// for the calls of a test, so only the loop that spends it hits it.
const pkgLimitBurst = 20

// pkgHeavyLimit is the Heavy limiter of these tests.
func pkgHeavyLimit(cfg *campaignpackage.Config) {
	cfg.Heavy = ratelimit.New(ratelimit.Config{
		PerClient: ratelimit.Rate{Burst: pkgLimitBurst, Every: time.Hour}, Global: ratelimit.Rate{Burst: 1000, Every: time.Millisecond},
		MaxClients: 100,
	})
}

func TestPackagesOffComesFirst(t *testing.T) {
	rig := newPkgRig(t, true, nil)
	master := rig.newUser("Mestre")
	_, err := master.pkg.BeginCampaignImport(t.Context(), connect.NewRequest(&pkgv1.BeginCampaignImportRequest{FileName: " ", Fingerprint: "", TotalBytes: 0}))
	if !isImagesOff(err) {
		t.Errorf("BeginCampaignImport(no blobs, bad request) error = %v, want IMAGES_OFF before the checks", err)
	}
	_, err = master.pkg.StartCampaignExport(t.Context(), connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: rig.newCampaign(master, "Mesa"), IdempotencyKey: "bad\x00key"}))
	if !isImagesOff(err) {
		t.Errorf("StartCampaignExport(no blobs, bad key) error = %v, want IMAGES_OFF before the key", err)
	}
	// Sweep without a store does nothing.
	rig.pkg.Sweep(t.Context())
}

// isImagesOff reports whether err is the IMAGES_OFF refusal.
func isImagesOff(err error) bool {
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		return false
	}
	for _, d := range detailsOf(err) {
		if v, derr := d.Value(); derr == nil {
			if b, ok := v.(*pkgv1.CampaignPackageBlocked); ok {
				return b.GetReason() == pkgv1.CampaignPackageBlockedReason_CAMPAIGN_PACKAGE_BLOCKED_REASON_IMAGES_OFF
			}
		}
	}
	return false
}

func TestStartCampaignExportWhenSomethingFails(t *testing.T) {
	rig := newPkgRig(t, false, pkgHeavyLimit)
	master, player := rig.newUser("Mestre"), rig.newUser("Jogadora")
	campaign := rig.newCampaign(master, "Mirathel")
	rig.invite(master, campaign, player)
	ctx := t.Context()
	start := func(u *user, key string) (*pkgv1.CampaignExport, error) {
		res, err := u.pkg.StartCampaignExport(ctx, connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: key}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetExport(), nil
	}
	_, err := start(player, "bad\x00key")
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("StartCampaignExport(player) error = %v, want permission_denied before the key", err)
	}
	_, err = start(master, "bad\x00key")
	pkgWantRefusal(t, "StartCampaignExport(bad key)", err, connect.CodeInvalidArgument, "idempotency_key must be 1 to 64 characters")

	for _, tc := range []struct{ query string }{
		{"GetCampaignExportByCreateKey"},
		{"GetRunningCampaignExport"},
		{"InsertCampaignExport"},
	} {
		rig.faults.Arm(tc.query)
		_, err := start(master, rand.Text())
		rig.faults.Disarm()
		if rig.faults.Hits(tc.query) == 0 {
			t.Fatalf("%s never ran", tc.query)
		}
		wantPkgDBFailure(t, "StartCampaignExport("+tc.query+")", err)
	}
	if n := rig.count(`SELECT count(*)::INT FROM campaign_exports`); n != 0 {
		t.Fatalf("exports after the failures = %d, want 0", n)
	}

	// An export already running is the answer to a new key, and no second
	// one starts.
	master.upload(campaign, "Mapa.png", pngImage(t, 40, 30, 1))
	hold, opened := make(chan struct{}), make(chan struct{})
	rig.blobs.set(func(b *pkgFaultyBlobs) { b.hold, b.opened = hold, opened })
	first, err := start(master, "first")
	if err != nil {
		t.Fatalf("StartCampaignExport(first) error = %v", err)
	}
	<-opened
	second, err := start(master, "second")
	if err != nil || second.GetId() != first.GetId() || second.GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_RUNNING {
		t.Errorf("StartCampaignExport(second, while running) = %v, %v, want the running %s", second, err, first.GetId())
	}
	close(hold)
	waitCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	rig.pkg.Wait(waitCtx)
	if n := rig.count(`SELECT count(*)::INT FROM campaign_exports`); n != 1 {
		t.Errorf("exports = %d, want 1", n)
	}

	// The rate limit comes after the key: spend the bucket with replays of
	// the first export, then a bad key is still told it is bad.
	for i := 0; ; i++ {
		if _, err := start(master, "first"); connect.CodeOf(err) == connect.CodeResourceExhausted {
			break
		} else if i > pkgLimitBurst {
			t.Fatalf("StartCampaignExport() never hit the limit, last error %v", err)
		}
	}
	_, err = start(master, "bad\x00key")
	pkgWantRefusal(t, "StartCampaignExport(limited, bad key)", err, connect.CodeInvalidArgument, "idempotency_key must be 1 to 64 characters")
	_, err = start(master, "third")
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Errorf("StartCampaignExport(over the limit) error = %v, want resource_exhausted", err)
	}
}

// pkgExportEnd runs an export of campaign to its end, in the background job,
// and returns its row as GetCampaignExport answers it.
func (rig *pkgRig) pkgExportEnd(t *testing.T, master *user, campaign string, during func()) *pkgv1.CampaignExport {
	t.Helper()
	ctx := t.Context()
	started := must(master.pkg.StartCampaignExport(ctx, connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: rand.Text()})))
	if during != nil {
		during()
	}
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	rig.pkg.Wait(waitCtx)
	var state string
	if err := rig.pool.QueryRow(ctx, `SELECT state FROM campaign_exports WHERE id = $1`, started.GetExport().GetId()).Scan(&state); err != nil {
		t.Fatalf("read the export: %v", err)
	}
	got := must(master.pkg.GetCampaignExport(ctx, connect.NewRequest(&pkgv1.GetCampaignExportRequest{CampaignId: campaign}))).GetExport()
	if got.GetId() != started.GetExport().GetId() {
		t.Fatalf("GetCampaignExport() = %v, want %s", got, started.GetExport().GetId())
	}
	return got
}

func wantExportFailed(t *testing.T, call string, e *pkgv1.CampaignExport) {
	t.Helper()
	if e.GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_FAILED || e.GetFailure() != pkgv1.CampaignExportFailure_CAMPAIGN_EXPORT_FAILURE_INTERRUPTED {
		t.Errorf("%s: export = %v, want failed, interrupted", call, e)
	}
}

func TestAnExportThatFailsLeavesNothingBehind(t *testing.T) {
	rig := newPkgRig(t, false, nil)
	master := rig.newUser("Mestre")
	plain := rig.newCampaign(master, "Sem imagens")
	withImages := rig.newCampaign(master, "Com imagens")
	master.upload(withImages, "um.png", pngImage(t, 40, 30, 1))
	master.upload(withImages, "dois.png", pngImage(t, 40, 30, 2))
	errStore := errors.New("the store is down")

	exportsLeft := func(t *testing.T, campaign string) {
		t.Helper()
		if keys := rig.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 0 {
			t.Errorf("export files left: %v", keys)
		}
	}

	t.Run("the zip cannot be stored, found when it is closed", func(t *testing.T) {
		rig.blobs.set(func(b *pkgFaultyBlobs) { b.putErr = errStore })
		defer rig.blobs.set(func(b *pkgFaultyBlobs) { b.putErr = nil })
		e := rig.pkgExportEnd(t, master, plain, nil)
		wantExportFailed(t, "Put fails", e)
		exportsLeft(t, plain)
	})

	t.Run("the zip cannot be stored, found while an entry is written", func(t *testing.T) {
		doc := make([]byte, 120<<10)
		if _, err := rand.Read(doc); err != nil {
			t.Fatal(err)
		}
		if _, err := master.document.UpdateCampaignDocument(t.Context(), connect.NewRequest(&campaignsv1.UpdateCampaignDocumentRequest{
			CampaignId: plain, Body: base64.StdEncoding.EncodeToString(doc)[:150<<10],
		})); err != nil {
			t.Fatalf("UpdateCampaignDocument() error = %v", err)
		}
		rig.blobs.set(func(b *pkgFaultyBlobs) { b.putErr = errStore })
		defer rig.blobs.set(func(b *pkgFaultyBlobs) { b.putErr = nil })
		e := rig.pkgExportEnd(t, master, plain, nil)
		wantExportFailed(t, "Put fails", e)
	})

	t.Run("an image cannot be opened", func(t *testing.T) {
		rig.blobs.set(func(b *pkgFaultyBlobs) { b.openErr = errStore })
		defer rig.blobs.set(func(b *pkgFaultyBlobs) { b.openErr = nil })
		e := rig.pkgExportEnd(t, master, withImages, nil)
		wantExportFailed(t, "Open fails", e)
		if rig.logs.errors(t) == 0 {
			t.Errorf("a failed export was not logged at ERROR")
		}
		exportsLeft(t, withImages)
	})

	t.Run("an image cannot be read", func(t *testing.T) {
		rig.blobs.set(func(b *pkgFaultyBlobs) { b.readErr = errStore })
		defer rig.blobs.set(func(b *pkgFaultyBlobs) { b.readErr = nil })
		e := rig.pkgExportEnd(t, master, withImages, nil)
		wantExportFailed(t, "the content fails", e)
		exportsLeft(t, withImages)
	})

	for _, tc := range []struct{ query string }{
		{"GetCampaignExport"},
		{"SetCampaignExportProgress"},
	} {
		t.Run("the progress cannot be written: "+tc.query, func(t *testing.T) {
			rig.faults.Arm(tc.query)
			defer rig.faults.Disarm()
			e := rig.pkgExportEnd(t, master, withImages, nil)
			wantExportFailed(t, tc.query+" fails", e)
		})
	}

	t.Run("the server goes down in the middle", func(t *testing.T) {
		hold, opened := make(chan struct{}), make(chan struct{})
		rig.blobs.set(func(b *pkgFaultyBlobs) { b.hold, b.opened = hold, opened })
		e := rig.pkgExportEnd(t, master, withImages, func() {
			<-opened
			rig.pkg.Cancel()
			close(hold)
		})
		wantExportFailed(t, "Cancel", e)
		exportsLeft(t, withImages)
	})

	t.Run("the export is canceled by another request in the middle", func(t *testing.T) {
		hold, opened := make(chan struct{}), make(chan struct{})
		rig.blobs.set(func(b *pkgFaultyBlobs) { b.hold, b.opened = hold, opened })
		e := rig.pkgExportEnd(t, master, withImages, func() {
			<-opened
			// What CancelCampaignExport on another instance does: it
			// changes the row but cannot stop this run, which finds out
			// by itself.
			if _, err := rig.pool.Exec(t.Context(), `UPDATE campaign_exports SET state = 'canceled', finished_at = updated_at WHERE state = 'running'`); err != nil {
				t.Errorf("cancel the row: %v", err)
			}
			// The progress is written at most once a second: the next
			// entry's report is the one that reads the row.
			time.Sleep(1100 * time.Millisecond)
			close(hold)
		})
		if e.GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_CANCELED {
			t.Errorf("export = %v, want canceled", e)
		}
		exportsLeft(t, withImages)
	})

	t.Run("a good export after all that", func(t *testing.T) {
		e := rig.pkgExportEnd(t, master, withImages, nil)
		if e.GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_DONE {
			t.Errorf("export = %v, want done", e)
		}
	})
}

func TestSweepWhenSomethingFails(t *testing.T) {
	rig := newPkgRig(t, false, nil)
	master := rig.newUser("Mestre")
	ctx := t.Context()
	campaign := rig.newCampaign(master, "Mirathel")
	_ = master.export(campaign)
	master.send("a.meurpg.zip", []byte("not a package, only bytes to keep"))
	rig.clock.advance(25 * time.Hour)

	exports := func() int { return rig.count(`SELECT count(*)::INT FROM campaign_exports`) }
	uploads := func() int { return rig.count(`SELECT count(*)::INT FROM campaign_imports`) }

	rig.faults.Arm("ListExpiredCampaignExports")
	rig.pkg.Sweep(ctx)
	rig.faults.Disarm()
	if exports() != 1 || uploads() != 1 {
		t.Fatalf("after a failed listing: %d exports, %d uploads, want both kept", exports(), uploads())
	}

	rig.faults.Arm("ListExpiredCampaignImports")
	rig.pkg.Sweep(ctx)
	rig.faults.Disarm()
	if exports() != 0 || uploads() != 1 {
		t.Fatalf("after a failed listing of uploads: %d exports, %d uploads, want 0 and 1", exports(), uploads())
	}

	rig.faults.Arm("DeleteCampaignImport")
	rig.pkg.Sweep(ctx)
	rig.faults.Disarm()
	if uploads() != 1 {
		t.Fatalf("after a failed delete: %d uploads, want 1", uploads())
	}
	if keys := rig.storedKeys("imports/"); len(keys) != 0 {
		t.Errorf("the parts were kept: %v", keys)
	}

	rig.pkg.Sweep(ctx)
	if uploads() != 0 {
		t.Errorf("after a good sweep: %d uploads, want 0", uploads())
	}
}
