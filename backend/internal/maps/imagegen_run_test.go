package maps

import (
	"context"
	"strings"
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
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
