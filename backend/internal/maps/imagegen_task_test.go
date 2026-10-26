package maps

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/maps/mapsdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/tasks"
)

// The image-generation task (imagegen_run.go): it is delivered at least once, so the handler is
// idempotent, and what the model gets is rebuilt from the request's row. The tests give the
// service a runner that only records the tasks, and deliver them by hand.

// recordingRunner is a tasks.Runner that runs nothing: it keeps what is enqueued, so a test
// delivers the task itself, once, twice or at the same time. failWith makes Enqueue fail.
type recordingRunner struct {
	mu       sync.Mutex
	handlers map[tasks.Kind]tasks.Handler
	jobs     []tasks.Job
	failWith error
}

var _ tasks.Runner = (*recordingRunner)(nil)

func (r *recordingRunner) Register(kind tasks.Kind, h tasks.Handler, _ tasks.Options) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.handlers == nil {
		r.handlers = map[tasks.Kind]tasks.Handler{}
	}
	r.handlers[kind] = h
}

func (r *recordingRunner) Enqueue(_ context.Context, job tasks.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failWith != nil {
		return r.failWith
	}
	r.jobs = append(r.jobs, job)
	return nil
}

func (r *recordingRunner) Every(tasks.Kind, time.Duration, tasks.Handler, tasks.Options) {}
func (r *recordingRunner) Start(context.Context)                                         {}
func (r *recordingRunner) Wait(context.Context)                                          {}

// last is the payload of the last task enqueued, as the queue would carry it (JSON).
func (r *recordingRunner) last(t *testing.T) json.RawMessage {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.jobs) == 0 {
		t.Fatal("no task was enqueued")
	}
	job := r.jobs[len(r.jobs)-1]
	if job.Kind != TaskImageGeneration {
		t.Fatalf("the task is %q, want %q", job.Kind, TaskImageGeneration)
	}
	payload, err := json.Marshal(job.Payload)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

// deliver hands the last task to the handler, the way the queue does.
func (r *recordingRunner) deliver(t *testing.T) error {
	t.Helper()
	payload := r.last(t)
	r.mu.Lock()
	h := r.handlers[TaskImageGeneration]
	r.mu.Unlock()
	return h(t.Context(), payload)
}

func withRunner(r tasks.Runner) func(*Config) { return func(c *Config) { c.Tasks = r } }

// galleryGenerated counts the generated images of the campaign's gallery.
func (h *harness) galleryGenerated(campaign string) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(h.t.Context(), `SELECT count(*) FROM gallery_images WHERE campaign_id = $1 AND generated`, campaign).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

// The payload holds the two IDs and nothing else.
func TestMR039_TheTaskCarriesOnlyIDs(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	h := newHarness(t, withFake(&gen.Fake{}, 5), withRunner(runner))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)

	res, err := master.generate(campaign, "uma cena secreta")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	if err := json.Unmarshal(runner.last(t), &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"campaign_id": campaign, "request_id": res.GetGeneration().GetId()}
	if !reflect.DeepEqual(payload, want) {
		t.Errorf("payload = %v, want %v", payload, want)
	}
}

// The task delivered twice, also at the same time, makes one picture: one call to the model,
// one gallery image, one slot.
func TestMR039_ADuplicateDeliveryMakesOnePicture(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	fake := &gen.Fake{}
	h := newHarness(t, withFake(fake, 5), withRunner(runner))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)

	res, err := master.generate(campaign, "uma cena")
	if err != nil {
		t.Fatal(err)
	}
	id := res.GetGeneration().GetId()
	if got := master.getGenerationState(campaign, id); got != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_PENDING {
		t.Fatalf("before the task runs the request is %v, want pending", got)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() { errs[i] = runner.deliver(t) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("delivery %d error = %v, want nil", i, err)
		}
	}
	if err := runner.deliver(t); err != nil { // and again, after it ended
		t.Errorf("a delivery after the end error = %v, want nil", err)
	}
	done := master.waitGeneration(campaign, id)
	if done.GetGeneration().GetState() != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_DONE || done.GetImage() == nil {
		t.Fatalf("request = %v, image %v; want done with an image", done.GetGeneration(), done.GetImage())
	}
	if n := len(fake.Calls()); n != 1 {
		t.Errorf("the model was called %d times, want 1", n)
	}
	if n := h.galleryGenerated(campaign); n != 1 {
		t.Errorf("the gallery has %d generated images, want 1", n)
	}
	if got := done.GetStatus().GetUsedThisMonth(); got != 1 {
		t.Errorf("used this month = %d, want 1", got)
	}
}

// A task for a request that ended, was canceled, was never sent by an instance that died, or
// does not exist, does nothing and answers nil: no call, no refund, no second picture.
func TestMR039_ATaskForARequestThatIsNotWaitingDoesNothing(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	fake := &gen.Fake{}
	h := newHarness(t, withFake(fake, 10), withRunner(runner))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	handler := func() tasks.Handler {
		runner.mu.Lock()
		defer runner.mu.Unlock()
		return runner.handlers[TaskImageGeneration]
	}()

	// Canceled before it left: the slot is back, and the task finds nothing to send.
	canceled, err := master.generate(campaign, "cancelada")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := master.imagegen.CancelImageGeneration(t.Context(), connect.NewRequest(&mapsv1.CancelImageGenerationRequest{CampaignId: campaign, GenerationId: canceled.GetGeneration().GetId()})); err != nil {
		t.Fatal(err)
	}
	if err := runner.deliver(t); err != nil {
		t.Errorf("the task of a canceled request error = %v, want nil", err)
	}
	if n := len(fake.Calls()); n != 0 {
		t.Errorf("the model was called %d times for a canceled request", n)
	}
	if got := master.getGenerationState(campaign, canceled.GetGeneration().GetId()); got != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_CANCELED {
		t.Errorf("the canceled request is %v", got)
	}

	// A request the model refused: it ended, and a second delivery changes nothing.
	refused, err := master.generate(campaign, "[recusa]")
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.deliver(t); err != nil {
		t.Fatal(err)
	}
	master.waitGeneration(campaign, refused.GetGeneration().GetId())
	if err := runner.deliver(t); err != nil {
		t.Errorf("the second delivery of a refused request error = %v, want nil", err)
	}
	if n := len(fake.Calls()); n != 1 {
		t.Errorf("the model was called %d times, want 1", n)
	}

	// A request whose instance died in the middle of the call: sent, no picture. It is not
	// called again (it may have been billed); the expiry rules end it, and its slot stays spent.
	lost, err := master.generate(campaign, "perdida no ar")
	if err != nil {
		t.Fatal(err)
	}
	lostID := lost.GetGeneration().GetId()
	if _, err := h.pool.Exec(t.Context(), `UPDATE image_requests SET sent_at = now() WHERE id = $1`, lostID); err != nil {
		t.Fatal(err)
	}
	if err := runner.deliver(t); err != nil {
		t.Errorf("the task of a request already sent error = %v, want nil", err)
	}
	if n := len(fake.Calls()); n != 1 {
		t.Errorf("the model was called again for a request already sent: %d calls, want 1", n)
	}
	got, err := master.getGeneration(campaign, lostID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if g := got.GetGeneration(); g.GetState() != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_PENDING || !g.GetSlotSpent() {
		t.Errorf("the request already sent = %v, want pending with the slot spent", g)
	}

	// A request that is not there, and a payload that makes no sense.
	missing, _ := json.Marshal(imageTask{CampaignID: campaign, RequestID: "00000000-0000-4000-8000-000000000000"})
	if err := handler(t.Context(), missing); err != nil {
		t.Errorf("the task of a request that does not exist error = %v, want nil", err)
	}
	if err := handler(t.Context(), json.RawMessage(`{"campaign_id": 3}`)); !tasks.IsPermanent(err) {
		t.Errorf("a payload that cannot be read error = %v, want a permanent one", err)
	}
}

// The queue refusing the task must not leave a request pending with its slot spent: it ends
// failed, unavailable, never sent, and the slot is back.
func TestMR039_AQueueThatRefusesTheTaskGivesTheSlotBack(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{failWith: errors.New("cloud tasks is unreachable")}
	fake := &gen.Fake{}
	h := newHarness(t, withFake(fake, 5), withRunner(runner))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)

	res, err := master.generate(campaign, "uma cena")
	if err != nil {
		t.Fatalf("GenerateSceneImage() error = %v, want the failed request", err)
	}
	g := res.GetGeneration()
	if g.GetState() != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_FAILED ||
		g.GetFailure() != mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_UNAVAILABLE || g.GetSlotSpent() {
		t.Errorf("request = %v, want failed, unavailable, slot back", g)
	}
	if got := res.GetStatus().GetRemaining(); got != 5 {
		t.Errorf("remaining = %d, want 5", got)
	}
	if n := len(fake.Calls()); n != 0 {
		t.Errorf("the model was called %d times", n)
	}
}

func (u *user) getGenerationState(campaign, id string) mapsv1.ImageGenerationState {
	u.h.t.Helper()
	res, err := u.getGeneration(campaign, id, 0)
	if err != nil {
		u.h.t.Fatal(err)
	}
	return res.GetGeneration().GetState()
}

// What the task rebuilds is what begin prepared: the references (objects, then characters), the
// image an edit adjusts.
func TestMR039_TheTaskRebuildsWhatBeginPrepared(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	h := newHarness(t, withFake(&gen.Fake{}, 10), withRunner(runner))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	ctx := t.Context()
	object := master.mustUpload(campaign, "objeto.png", pngImage(t, 300, 200)).GetId()
	big := master.mustUpload(campaign, "grande.png", pngImage(t, 1500, 1000)).GetId()
	character := master.mustUpload(campaign, "personagem.png", pngImage(t, 100, 150)).GetId()

	// A scene with references.
	n := newRequest{kind: kindScene, key: "rebuild-scene", prompt: "uma cena", ratio: defaultSceneRatio, requestedBy: master.id, name: "Arte", references: []string{object, big}, characters: []string{character}}
	prep, err := h.svc.prepare(ctx, campaign, n, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(prep.references) != 3 || prep.references[2].Character != true {
		t.Fatalf("prepared references = %d, want 3 with the character last", len(prep.references))
	}
	n.prepared = prep
	row, _, err := h.svc.reserve(ctx, n, campaign)
	if err != nil {
		t.Fatal(err)
	}
	got, reason, err := h.svc.prepareFromRow(ctx, row)
	if err != nil || reason != "" || !reflect.DeepEqual(got, prep) {
		t.Errorf("the scene rebuilt differs from the one begin prepared (reason %q, error %v)", reason, err)
	}

	// An edit of a generated image.
	if err := runner.deliver(t); err != nil {
		t.Fatal(err)
	}
	master.waitGeneration(campaign, row.ID)
	done, err := h.svc.queries.GetImageRequest(ctx, mapsdb.GetImageRequestParams{CampaignID: campaign, ID: row.ID})
	if err != nil || done.ImageID == nil {
		t.Fatalf("the scene did not end with an image: %v, %v", done, err)
	}
	edit := newRequest{kind: kindEdit, key: "rebuild-edit", prompt: "mais escura", requestedBy: master.id, source: done.ImageID}
	prep, err = h.svc.prepare(ctx, campaign, edit, nil)
	if err != nil || prep.previous == nil {
		t.Fatalf("prepare(edit) = %+v, %v", prep, err)
	}
	edit.prepared = prep
	row, _, err = h.svc.reserve(ctx, edit, campaign)
	if err != nil {
		t.Fatal(err)
	}
	got, reason, err = h.svc.prepareFromRow(ctx, row)
	if err != nil || reason != "" || !reflect.DeepEqual(got, prep) {
		t.Errorf("the edit rebuilt differs from the one begin prepared (reason %q, error %v)", reason, err)
	}

	// An image the request needs, gone meanwhile: nothing to send, and the reason says so.
	scene := n
	scene.key = "rebuild-scene-2"
	scene.references = []string{big}
	scene.characters = []string{character}
	if prep, err = h.svc.prepare(ctx, campaign, scene, nil); err != nil {
		t.Fatal(err)
	}
	scene.prepared = prep
	row, _, err = h.svc.reserve(ctx, scene, campaign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := master.gallery.DeleteGalleryImage(ctx, connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: character})); err != nil {
		t.Fatal(err)
	}
	if _, reason, _ := h.svc.prepareFromRow(ctx, row); reason != reasonImageMissing {
		t.Errorf("reason with a reference deleted = %q, want %q", reason, reasonImageMissing)
	}
}

// What the task draws for a map is what begin drew, for the scene art (the players' view) and for the
// textured map.
func TestMR039_TheTaskRedrawsTheMapAsBeginDrewIt(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	c := newCave(t, withFake(&gen.Fake{}, 10), withRunner(runner))
	ctx := t.Context()
	for _, layout := range []struct{ kind, layout string }{{kindMapScene, gen.LayoutScene}, {kindTexturedMap, gen.LayoutTexture}} {
		n := newRequest{
			kind: layout.kind, key: "rebuild-" + layout.kind, prompt: "uma caverna", style: "", ratio: defaultSceneRatio, requestedBy: c.master.id, name: "Mapa",
			mapReq: &mapRequest{mapID: c.mapID, layout: layout.layout},
		}
		status, err := c.h.svc.freshStatus(ctx, c.campaign)
		if err != nil {
			t.Fatal(err)
		}
		prep, err := c.h.svc.prepare(ctx, c.campaign, n, status)
		if err != nil {
			t.Fatalf("prepare(%s) error = %v", layout.kind, err)
		}
		n.prepared = prep
		if prep.ratio != "" {
			n.ratio = prep.ratio
		}
		row, _, err := c.h.svc.reserve(ctx, n, c.campaign)
		if err != nil {
			t.Fatal(err)
		}
		got, reason, err := c.h.svc.prepareFromRow(ctx, row)
		if err != nil || reason != "" || !reflect.DeepEqual(got, prep) {
			t.Errorf("%s: the drawing rebuilt differs from the one begin made (reason %q, error %v)", layout.kind, reason, err)
		}
	}
}

// A map that changed between the request and the task: nothing is sent, the request ends
// failed with its own reason, and the slot is back.
func TestMR039_AMapThatChangedBeforeTheTaskRanEndsTheRequest(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	fake := &gen.Fake{}
	c := newCave(t, withFake(fake, 10), withRunner(runner))

	res, err := c.master.generateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna")
	if err != nil {
		t.Fatal(err)
	}
	id := res.GetGeneration().GetId()
	c.master.mustSetGrid(c.campaign, c.mapID, 12) // another grid: the drawing would not be the one agreed to
	if err := runner.deliver(t); err != nil {
		t.Fatalf("the task error = %v, want nil (the failure is recorded)", err)
	}
	got, err := c.master.getGeneration(c.campaign, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	g := got.GetGeneration()
	if g.GetState() != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_FAILED ||
		g.GetFailure() != mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_MAP_CHANGED || g.GetSlotSpent() {
		t.Errorf("request = %v, want failed, MAP_CHANGED, slot back", g)
	}
	if n := len(fake.Calls()); n != 0 {
		t.Errorf("the model was called %d times for a map that changed", n)
	}
	if want := "O mapa mudou depois do pedido. Gere de novo; esta não conta no seu limite."; g.GetReasonPt() != want {
		t.Errorf("reason_pt = %q, want %q", g.GetReasonPt(), want)
	}
}
