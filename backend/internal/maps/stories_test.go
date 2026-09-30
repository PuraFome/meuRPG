package maps

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg" // image.DecodeConfig reads the stored JPEGs
	"net/http"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// MR-019's acceptance criteria (docs/produto/historias.md), as far as the
// server goes. The screens get their Playwright tests with the gallery UI.

// MR-019, first criterion: the master uploads a JPEG, PNG or WebP of up to
// 10 MB; it shows up in the gallery, and the stored file has none of the
// original's metadata (EXIF, with its GPS position).
func TestMR019_MasterUploadsAnImageWithoutItsMetadata(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	original := jpegWithGPS(t, 64, 48)
	if !bytes.Contains(original, []byte("Exif")) || !bytes.Contains(original, []byte(gpsNote)) {
		t.Fatal("the test JPEG has no EXIF to remove")
	}

	uploaded := master.mustUpload(campaign, "Taverna do Pônei.jpg", original)
	if uploaded.GetName() != "Taverna do Pônei" || uploaded.GetContentType() != "image/jpeg" ||
		uploaded.GetWidth() != 64 || uploaded.GetHeight() != 48 || uploaded.GetCampaignId() != campaign {
		t.Errorf("uploaded = %v, want the 64x48 JPEG named after the file", uploaded)
	}

	// It shows up in the gallery.
	list := master.list(campaign)
	if len(list.GetImages()) != 1 || list.GetImages()[0].GetId() != uploaded.GetId() {
		t.Fatalf("gallery = %v, want the uploaded image", list.GetImages())
	}

	// The stored file, read straight from the blob store, and the file the
	// server sends, have no EXIF, and are the same image.
	imageKey, thumbnailKey := blobKeys(campaign, uploaded.GetId())
	served := master.get(uploaded.GetUrl())
	for name, content := range map[string][]byte{
		"stored image":     storedContent(h, imageKey),
		"stored thumbnail": storedContent(h, thumbnailKey),
		"served image":     served.body,
	} {
		for _, marker := range []string{"Exif", gpsNote, "II*\x00"} {
			if bytes.Contains(content, []byte(marker)) {
				t.Errorf("the %s still has %q", name, marker)
			}
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(content))
		if err != nil || format != "jpeg" || cfg.Width != 64 || cfg.Height != 48 {
			t.Errorf("the %s is a %s of %dx%d (%v), want a 64x48 JPEG", name, format, cfg.Width, cfg.Height, err)
		}
	}
	if int(uploaded.GetByteSize()) != len(served.body) {
		t.Errorf("byte_size = %d, but the image has %d bytes", uploaded.GetByteSize(), len(served.body))
	}
}

func storedContent(h *harness, key string) []byte {
	h.t.Helper()
	_, content := h.storedFile(key)
	return content
}

// MR-019, second criterion: a player who asks for the campaign's gallery is
// refused. Nor can they upload, rename or delete.
func TestMR019_PlayersCannotListTheGallery(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master, player := h.newUser("Mestre"), h.newUser("Jogadora")
	campaign := h.newCampaign(master, player)
	img := master.mustUpload(campaign, "mapa.png", pngImage(t, 20, 20))
	ctx := t.Context()

	_, err := player.gallery.ListGalleryImages(ctx, connect.NewRequest(&mapsv1.ListGalleryImagesRequest{CampaignId: campaign}))
	wantCode(t, "ListGalleryImages as a player", err, connect.CodePermissionDenied)
	_, err = player.gallery.RenameGalleryImage(ctx, connect.NewRequest(&mapsv1.RenameGalleryImageRequest{CampaignId: campaign, ImageId: img.GetId(), Name: "Meu"}))
	wantCode(t, "RenameGalleryImage as a player", err, connect.CodePermissionDenied)
	_, err = player.gallery.DeleteGalleryImage(ctx, connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: img.GetId()}))
	wantCode(t, "DeleteGalleryImage as a player", err, connect.CodePermissionDenied)
	if res := player.upload(campaign, "outro.png", pngImage(t, 10, 10)); res.status != http.StatusForbidden || res.errorBody(t).Code != "permission_denied" {
		t.Errorf("upload as a player: status %d, body %s; want 403 permission_denied", res.status, res.body)
	}
	if got := master.list(campaign).GetImages(); len(got) != 1 || got[0].GetName() != "mapa" {
		t.Errorf("after the player's attempts, gallery = %v; want only the master's image, unchanged", got)
	}
}

// MR-019, third criterion: an image a map uses cannot be deleted, and the
// app says which map it is in (the ImageInUse detail names it). The rest
// of it (several maps, the files staying, deleting once no map uses it) is
// TestDeletingAnImageAMapUses.
func TestMR019_AnImageAMapUsesCannotBeDeleted(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	master := h.newUser("Mestre")
	campaign := h.newCampaign(master)
	img := master.mustUpload(campaign, "masmorra.png", pngImage(t, 30, 30))
	dungeon := master.createMap(campaign, "Covil dos goblins", img.GetId())
	ctx := t.Context()

	_, err := master.gallery.DeleteGalleryImage(ctx, connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: img.GetId()}))
	wantCode(t, "DeleteGalleryImage of an image in use", err, connect.CodeFailedPrecondition)
	var named []string
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		for _, d := range ce.Details() {
			if msg, derr := d.Value(); derr == nil {
				if inUse, ok := msg.(*mapsv1.ImageInUse); ok {
					for _, m := range inUse.GetMaps() {
						named = append(named, m.GetId()+" "+m.GetName())
					}
				}
			}
		}
	}
	if want := dungeon.GetId() + " Covil dos goblins"; len(named) != 1 || named[0] != want {
		t.Errorf("the refusal names %v, want [%s]", named, want)
	}
	if got := master.list(campaign).GetImages(); len(got) != 1 {
		t.Errorf("gallery = %v, want the image still there", got)
	}
}
