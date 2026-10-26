package maps

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
	"github.com/PuraFome/meuRPG/backend/internal/platform/blob"
)

// garbageGen answers every call with bytes that are no image.
type garbageGen struct{}

func (garbageGen) Generate(context.Context, gen.Request) (gen.Image, error) {
	return gen.Image{MimeType: "image/png", Data: []byte("this is not a picture")}, nil
}

func (garbageGen) Model() string { return "test-garbage" }

// A picture the server cannot decode comes back from a call that was billed: the
// request fails as "no image", keeps its slot spent, and the gallery stays empty.
func TestMR039_AnUnusablePictureKeepsTheSlotSpent(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withGenerator(garbageGen{}, 5))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)

	res := master.mustGenerate(campaign, "uma cena")
	g := res.GetGeneration()
	if g.GetState() != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_FAILED || g.GetFailure() != mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_NO_IMAGE ||
		!g.GetSlotSpent() || res.GetImage() != nil || !strings.Contains(g.GetReasonPt(), "imagem") {
		t.Errorf("request = %v, image %v; want FAILED, no image, slot spent", g, res.GetImage())
	}
	if got := res.GetStatus().GetRemaining(); got != 4 {
		t.Errorf("remaining = %d after a picture that cannot be used, want 4", got)
	}
}

// deadlineBlobs is a file store whose every Put fails the way the cloud store did in
// production: the deadline ran out, and nothing was written.
type deadlineBlobs struct{ blob.Store }

func (deadlineBlobs) Put(context.Context, string, string, io.Reader) error {
	return fmt.Errorf("blob: cloud storage put: %w", context.DeadlineExceeded)
}

// timedOutGen is a model whose call left and never answered in time.
type timedOutGen struct{}

func (timedOutGen) Generate(context.Context, gen.Request) (gen.Image, error) {
	return gen.Image{}, fmt.Errorf("gemini: request: %w", context.DeadlineExceeded)
}

func (timedOutGen) Model() string { return "test-timeout" }

// The model answered with a good picture and the file store failed: the fault is the
// server's, so the request fails with its own reason and the master's slot comes back.
func TestMR039_APictureThatCannotBeStoredGivesTheSlotBack(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withFake(&gen.Fake{}, 5))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	h.svc.blobs = deadlineBlobs{Store: h.blobs}
	before := master.imageStatus(campaign).GetUsedThisMonth()

	res := master.mustGenerate(campaign, "uma cena")
	g := res.GetGeneration()
	if g.GetState() != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_FAILED ||
		g.GetFailure() != mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_STORAGE_FAILED ||
		g.GetSlotSpent() || res.GetImage() != nil {
		t.Fatalf("request = %v, image %v; want FAILED, STORAGE_FAILED, slot back", g, res.GetImage())
	}
	if !strings.Contains(g.GetReasonPt(), "A imagem foi feita, mas não deu para guardá-la") || !strings.Contains(g.GetReasonPt(), "não conta no seu limite") {
		t.Errorf("reason_pt = %q, want the storage sentence", g.GetReasonPt())
	}
	if used := master.imageStatus(campaign).GetUsedThisMonth(); used != before {
		t.Errorf("used this month = %d after a storage failure, want %d (before)", used, before)
	}
	if got := res.GetStatus().GetRemaining(); got != 5 {
		t.Errorf("remaining = %d, want 5", got)
	}
}

// A request that left and timed out with no picture may have been billed: it keeps the
// slot spent and is told as the service not answering (the opposite of the storage case).
func TestMR039_ARequestThatLeftAndTimedOutKeepsTheSlotAndIsUnavailable(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withGenerator(timedOutGen{}, 5))
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	before := master.imageStatus(campaign).GetUsedThisMonth()

	res := master.mustGenerate(campaign, "uma cena")
	g := res.GetGeneration()
	if g.GetState() != mapsv1.ImageGenerationState_IMAGE_GENERATION_STATE_FAILED ||
		g.GetFailure() != mapsv1.ImageGenerationFailure_IMAGE_GENERATION_FAILURE_UNAVAILABLE || !g.GetSlotSpent() {
		t.Fatalf("request = %v; want FAILED, UNAVAILABLE, slot spent", g)
	}
	if used := master.imageStatus(campaign).GetUsedThisMonth(); used != before+1 {
		t.Errorf("used this month = %d, want %d", used, before+1)
	}
}
