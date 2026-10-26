package campaignpackage_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	pkgv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaignpackage/v1"
	"github.com/PuraFome/meuRPG/backend/internal/campaignpackage"
	"github.com/PuraFome/meuRPG/backend/internal/platform/tasks"
)

// The export and the sweep as tasks: delivered at least once, so a handler run twice must do
// the work once. The test's runner keeps the handlers and the jobs and runs a handler only
// when the test says so, as many times as the test says.

type pkgTaskJob struct {
	kind    tasks.Kind
	payload json.RawMessage
}

type pkgFakeRunner struct {
	mu         sync.Mutex
	handlers   map[tasks.Kind]tasks.Handler
	options    map[tasks.Kind]tasks.Options
	every      map[tasks.Kind]time.Duration
	jobs       []pkgTaskJob
	enqueueErr error
}

func newPkgFakeRunner() *pkgFakeRunner {
	return &pkgFakeRunner{handlers: map[tasks.Kind]tasks.Handler{}, options: map[tasks.Kind]tasks.Options{}, every: map[tasks.Kind]time.Duration{}}
}

func (r *pkgFakeRunner) Register(kind tasks.Kind, h tasks.Handler, opts tasks.Options) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[kind], r.options[kind] = h, opts
}

func (r *pkgFakeRunner) Every(kind tasks.Kind, interval time.Duration, h tasks.Handler, opts tasks.Options) {
	r.Register(kind, h, opts)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.every[kind] = interval
}

func (r *pkgFakeRunner) Enqueue(_ context.Context, job tasks.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.enqueueErr != nil {
		return r.enqueueErr
	}
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		return err
	}
	r.jobs = append(r.jobs, pkgTaskJob{kind: job.Kind, payload: payload})
	return nil
}

func (r *pkgFakeRunner) Start(context.Context) {}
func (r *pkgFakeRunner) Wait(context.Context)  {}
func (r *pkgFakeRunner) handler(k tasks.Kind) tasks.Handler {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handlers[k]
}

func (r *pkgFakeRunner) jobsOf(k tasks.Kind) []pkgTaskJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []pkgTaskJob
	for _, j := range r.jobs {
		if j.kind == k {
			out = append(out, j)
		}
	}
	return out
}

// pkgTaskRig is a rig whose tasks wait for the test to deliver them.
type pkgTaskRig struct {
	*pkgRig
	runner *pkgFakeRunner
}

func newPkgTaskRig(t *testing.T) *pkgTaskRig {
	t.Helper()
	runner := newPkgFakeRunner()
	return &pkgTaskRig{runner: runner, pkgRig: newPkgRig(t, false, func(cfg *campaignpackage.Config) { cfg.Tasks = runner })}
}

// start asks for an export and returns the answer and the one task it enqueued.
func (rig *pkgTaskRig) start(t *testing.T, master *user, campaign string) (*pkgv1.CampaignExport, pkgTaskJob) {
	t.Helper()
	res, err := master.pkg.StartCampaignExport(t.Context(), connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: rand.Text()}))
	if err != nil {
		t.Fatalf("StartCampaignExport() error = %v", err)
	}
	jobs := rig.runner.jobsOf("campaign-export")
	if len(jobs) == 0 {
		t.Fatalf("StartCampaignExport() enqueued no task")
	}
	return res.Msg.GetExport(), jobs[len(jobs)-1]
}

func (rig *pkgTaskRig) deliver(t *testing.T, job pkgTaskJob) error {
	t.Helper()
	return rig.runner.handler(job.kind)(t.Context(), job.payload)
}

func (rig *pkgTaskRig) exportState(t *testing.T, id string) (state, failure string) {
	t.Helper()
	if err := rig.pool.QueryRow(t.Context(), `SELECT state, failure FROM campaign_exports WHERE id = $1`, id).Scan(&state, &failure); err != nil {
		t.Fatalf("read the export: %v", err)
	}
	return state, failure
}

func TestTheTasksAreRegisteredInTheRunnerOfTheConfig(t *testing.T) {
	rig := newPkgTaskRig(t)
	if rig.runner.handler("campaign-export") == nil {
		t.Errorf("campaign-export is not registered")
	}
	if rig.runner.handler("campaign-sweep") == nil || rig.runner.every["campaign-sweep"] != campaignpackage.SweepEvery {
		t.Errorf("campaign-sweep is not scheduled with Every(%v): %v", campaignpackage.SweepEvery, rig.runner.every)
	}
	if d := rig.runner.options["campaign-export"].Deadline; d <= 0 || d > 30*time.Minute {
		t.Errorf("deadline of campaign-export = %v, want within 30 minutes", d)
	}
}

func TestAnExportTaskDeliveredTwiceWritesOneZip(t *testing.T) {
	rig := newPkgTaskRig(t)
	master := rig.newUser("Mestre")
	campaign := rig.newCampaign(master, "Mirathel")
	master.upload(campaign, "Mapa.png", pngImage(t, 40, 30, 1))

	started, job := rig.start(t, master, campaign)
	if started.GetState() != pkgv1.CampaignExportState_CAMPAIGN_EXPORT_STATE_RUNNING {
		t.Errorf("the export asked for = %v, want running (pending is running to the page)", started.GetState())
	}
	var payload map[string]string
	if err := json.Unmarshal(job.payload, &payload); err != nil || len(payload) != 2 || payload["campaign_id"] != campaign || payload["export_id"] != started.GetId() {
		t.Errorf("payload = %s (%v), want only campaign_id and export_id", job.payload, err)
	}
	if state, _ := rig.exportState(t, started.GetId()); state != "pending" {
		t.Fatalf("state before the delivery = %q, want pending", state)
	}

	if err := rig.deliver(t, job); err != nil {
		t.Fatalf("first delivery error = %v", err)
	}
	if state, _ := rig.exportState(t, started.GetId()); state != "done" {
		t.Fatalf("state after the delivery = %q, want done", state)
	}
	var finished, updated time.Time
	if err := rig.pool.QueryRow(t.Context(), `SELECT finished_at, updated_at FROM campaign_exports WHERE id = $1`, started.GetId()).Scan(&finished, &updated); err != nil {
		t.Fatal(err)
	}
	keys := rig.storedKeys("campaigns/" + campaign + "/exports/")
	if len(keys) != 1 {
		t.Fatalf("zips after the first delivery = %v, want 1", keys)
	}

	rig.clock.advance(time.Minute)
	if err := rig.deliver(t, job); err != nil {
		t.Fatalf("second delivery error = %v, want nil", err)
	}
	var finished2, updated2 time.Time
	if err := rig.pool.QueryRow(t.Context(), `SELECT finished_at, updated_at FROM campaign_exports WHERE id = $1`, started.GetId()).Scan(&finished2, &updated2); err != nil {
		t.Fatal(err)
	}
	if !finished2.Equal(finished) || !updated2.Equal(updated) {
		t.Errorf("the second delivery changed the row: finished %v -> %v, updated %v -> %v", finished, finished2, updated, updated2)
	}
	if keys2 := rig.storedKeys("campaigns/" + campaign + "/exports/"); len(keys2) != 1 || keys2[0] != keys[0] {
		t.Errorf("zips after the second delivery = %v, want only %v", keys2, keys)
	}
}

func TestAnExportTaskDeliveredWhileItRunsDoesNothing(t *testing.T) {
	rig := newPkgTaskRig(t)
	master := rig.newUser("Mestre")
	campaign := rig.newCampaign(master, "Mirathel")
	master.upload(campaign, "Mapa.png", pngImage(t, 40, 30, 1))
	started, job := rig.start(t, master, campaign)

	hold, opened := make(chan struct{}), make(chan struct{})
	rig.blobs.set(func(b *pkgFaultyBlobs) { b.hold, b.opened = hold, opened })
	first := make(chan error, 1)
	go func() { first <- rig.deliver(t, job) }()
	<-opened
	if state, _ := rig.exportState(t, started.GetId()); state != "running" {
		t.Fatalf("state while it runs = %q, want running", state)
	}
	if err := rig.deliver(t, job); err != nil {
		t.Errorf("delivery while running error = %v, want nil", err)
	}
	if state, _ := rig.exportState(t, started.GetId()); state != "running" {
		t.Errorf("a duplicate delivery changed the running export to %q", state)
	}
	close(hold)
	if err := <-first; err != nil {
		t.Errorf("first delivery error = %v", err)
	}
	if state, _ := rig.exportState(t, started.GetId()); state != "done" {
		t.Errorf("state at the end = %q, want done", state)
	}
	if keys := rig.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 1 {
		t.Errorf("zips = %v, want 1", keys)
	}
}

func TestAnExportTaskForAnExportThatEndedDoesNothing(t *testing.T) {
	rig := newPkgTaskRig(t)
	master := rig.newUser("Mestre")
	campaign := rig.newCampaign(master, "Mirathel")

	for _, end := range []struct{ name, state string }{{"canceled", "canceled"}, {"failed", "failed"}} {
		t.Run(end.name, func(t *testing.T) {
			started, job := rig.start(t, master, campaign)
			if end.state == "canceled" {
				if _, err := master.pkg.CancelCampaignExport(t.Context(), connect.NewRequest(&pkgv1.CancelCampaignExportRequest{CampaignId: campaign, ExportId: started.GetId()})); err != nil {
					t.Fatalf("CancelCampaignExport() error = %v", err)
				}
			} else if _, err := rig.pool.Exec(t.Context(), `UPDATE campaign_exports SET state = 'failed', failure = 'too_big', finished_at = updated_at WHERE id = $1`, started.GetId()); err != nil {
				t.Fatal(err)
			}
			if err := rig.deliver(t, job); err != nil {
				t.Errorf("delivery error = %v, want nil", err)
			}
			if state, _ := rig.exportState(t, started.GetId()); state != end.state {
				t.Errorf("state = %q, want it still %q", state, end.state)
			}
			if keys := rig.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 0 {
				t.Errorf("zips = %v, want none", keys)
			}
		})
	}

	t.Run("a task with a bad payload or a stranger's id", func(t *testing.T) {
		h := rig.runner.handler("campaign-export")
		if err := h(t.Context(), json.RawMessage(`{"export_id":"nope"}`)); !tasks.IsPermanent(err) {
			t.Errorf("bad id error = %v, want permanent", err)
		}
		if err := h(t.Context(), json.RawMessage(`{"campaign_id":"x","export_id":"`+newHexID()+`"}`)); err != nil {
			t.Errorf("unknown export error = %v, want nil", err)
		}
	})
}

func newHexID() string {
	const hex = "0123456789abcdef"
	b := make([]byte, 32)
	for i := range b {
		b[i] = hex[i%16]
	}
	return string(b)
}

func TestAnExportThatCannotBeEnqueuedFailsAtOnce(t *testing.T) {
	rig := newPkgTaskRig(t)
	master := rig.newUser("Mestre")
	campaign := rig.newCampaign(master, "Mirathel")
	rig.runner.enqueueErr = errors.New("the queue is down")

	res, err := master.pkg.StartCampaignExport(t.Context(), connect.NewRequest(&pkgv1.StartCampaignExportRequest{CampaignId: campaign, IdempotencyKey: "k1"}))
	if err != nil {
		t.Fatalf("StartCampaignExport() error = %v", err)
	}
	e := res.Msg.GetExport()
	wantExportFailed(t, "StartCampaignExport(enqueue fails)", e)
	if state, failure := rig.exportState(t, e.GetId()); state != "failed" || failure != "interrupted" {
		t.Errorf("row = %q, %q, want failed, interrupted", state, failure)
	}

	// The master can ask again once the queue is back.
	rig.runner.mu.Lock()
	rig.runner.enqueueErr = nil
	rig.runner.mu.Unlock()
	again, _ := rig.start(t, master, campaign)
	if again.GetId() == e.GetId() {
		t.Errorf("the new export is the failed one")
	}
}

func TestAnExportTaskForAnExportThatStoppedMovingFailsIt(t *testing.T) {
	rig := newPkgTaskRig(t)
	master := rig.newUser("Mestre")
	campaign := rig.newCampaign(master, "Mirathel")
	started, job := rig.start(t, master, campaign)
	if _, err := rig.pool.Exec(t.Context(), `UPDATE campaign_exports SET state = 'running' WHERE id = $1`, started.GetId()); err != nil {
		t.Fatal(err)
	}

	// Moving lately: another delivery is writing it.
	if err := rig.deliver(t, job); err != nil {
		t.Fatalf("delivery error = %v", err)
	}
	if state, _ := rig.exportState(t, started.GetId()); state != "running" {
		t.Fatalf("state = %q, want the running export left alone", state)
	}

	// Not moved for a while: the instance that claimed it died. It is failed, never rewritten.
	rig.clock.advance(11 * time.Minute)
	if err := rig.deliver(t, job); err != nil {
		t.Fatalf("delivery error = %v", err)
	}
	if state, failure := rig.exportState(t, started.GetId()); state != "failed" || failure != "interrupted" {
		t.Errorf("row = %q, %q, want failed, interrupted", state, failure)
	}
	if keys := rig.storedKeys("campaigns/" + campaign + "/exports/"); len(keys) != 0 {
		t.Errorf("zips = %v, want none", keys)
	}
}

func TestTheSweepTaskIsIdempotent(t *testing.T) {
	rig := newPkgTaskRig(t)
	master := rig.newUser("Mestre")
	campaign := rig.newCampaign(master, "Mirathel")
	started, job := rig.start(t, master, campaign)
	if err := rig.deliver(t, job); err != nil {
		t.Fatal(err)
	}
	master.send("a.meurpg.zip", []byte("not a package, only bytes to keep"))
	rig.clock.advance(25 * time.Hour)

	sweep := rig.runner.handler("campaign-sweep")
	counts := func() (int, int, int) {
		return rig.count(`SELECT count(*)::INT FROM campaign_exports`), rig.count(`SELECT count(*)::INT FROM campaign_imports`), len(rig.storedKeys("campaigns/"+campaign+"/exports/")) + len(rig.storedKeys("imports/"))
	}
	if e, i, k := counts(); e != 1 || i != 1 || k == 0 {
		t.Fatalf("before the sweep: %d exports, %d uploads, %d files", e, i, k)
	}
	for n := 1; n <= 2; n++ {
		if err := sweep(t.Context(), json.RawMessage(`{}`)); err != nil {
			t.Fatalf("sweep %d error = %v", n, err)
		}
		if e, i, k := counts(); e != 0 || i != 0 || k != 0 {
			t.Fatalf("after sweep %d: %d exports (%s), %d uploads, %d files, want none", n, e, started.GetId(), i, k)
		}
	}
}
