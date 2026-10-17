package maps

import (
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// SetSceneImages and DeleteGalleryImage: each refusal with its exact code and
// message in the order the checks run, and who hears each change.

// imageRefusal renders a refusal as "code: message", or "ok" for no error.
func imageRefusal(err error) string {
	if err == nil {
		return "ok"
	}
	ce, ok := err.(*connect.Error) //nolint:errorlint // the handlers return the Connect error itself
	if !ok {
		return "not a connect error: " + err.Error()
	}
	return ce.Code().String() + ": " + ce.Message()
}

// imageEvents renders what a watcher heard before the probe's map_changed.
func imageEvents(w *watcher, probeMapID string) string {
	var out []string
	for _, ev := range w.drain(probeMapID) {
		switch {
		case ev.GetMapChanged() != nil:
			out = append(out, "map_changed")
		case ev.GetSceneChanged() != nil:
			out = append(out, "scene_changed")
		case ev.GetShownImageChanged() != nil:
			out = append(out, "shown_image_changed")
		case ev.GetStageChanged() != nil:
			out = append(out, "stage_changed")
		case ev.GetLeftImagesChanged() != nil:
			out = append(out, "left_images_changed")
		default:
			out = append(out, fmt.Sprintf("%T", ev.GetEvent()))
		}
	}
	return strings.Join(out, ",")
}

func TestSetSceneImagesRefusalsAndWhoHears(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	set := func(u *user, mapID, pointID string, ids ...string) error {
		_, err := u.setSceneImages(s.campaign, mapID, pointID, ids...)
		return err
	}
	var ids []string
	for range maxSceneImages + 1 {
		ids = append(ids, s.master.newImage(s.campaign))
	}
	foreign := s.master.newImage(s.h.newCampaign(s.master))
	battle := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada"})
	otherMap := s.master.createMap(s.campaign, "Outro", s.master.newImage(s.campaign))

	for _, c := range []struct {
		name string
		call func() error
		want string
	}{
		{"a player", func() error { return set(s.ana, s.mapID, pid, ids[0]) }, "permission_denied: only the campaign's master can do this"},
		{"map not a UUID", func() error { return set(s.master, "x", pid, ids[0]) }, "not_found: map not found"},
		{"point not a UUID", func() error { return set(s.master, s.mapID, "x", ids[0]) }, "not_found: point not found"},
		{"point wins over too many", func() error { return set(s.master, s.mapID, "x", ids...) }, "not_found: point not found"},
		{"too many images", func() error { return set(s.master, s.mapID, pid, ids...) }, "invalid_argument: a scene holds at most 8 images"},
		{"too many wins over a bad id", func() error { return set(s.master, s.mapID, pid, append([]string{"x"}, ids...)...) }, "invalid_argument: a scene holds at most 8 images"},
		{"an id that is not a UUID", func() error { return set(s.master, s.mapID, pid, ids[0], "x") }, "invalid_argument: image_ids must be UUIDs"},
		{"an image twice", func() error { return set(s.master, s.mapID, pid, ids[0], ids[1], ids[0]) }, "invalid_argument: image_ids must not repeat an image"},
		{"a bad id wins over a repeat after it", func() error { return set(s.master, s.mapID, pid, ids[0], "x", ids[0]) }, "invalid_argument: image_ids must be UUIDs"},
		{"a bad id wins over an unknown point", func() error { return set(s.master, s.mapID, newKey(), "x") }, "invalid_argument: image_ids must be UUIDs"},
		{"unknown map", func() error { return set(s.master, newKey(), pid, ids[0]) }, "not_found: map not found"},
		{"unknown point", func() error { return set(s.master, s.mapID, newKey(), ids[0]) }, "not_found: point not found"},
		{"a point of another map", func() error { return set(s.master, otherMap.GetId(), pid, ids[0]) }, "not_found: point not found"},
		{"not a scene point", func() error { return set(s.master, s.mapID, battle.GetId(), ids[0]) }, "invalid_argument: only a SCENE point has actions and clues"},
		{"an image of another campaign", func() error { return set(s.master, s.mapID, pid, ids[0], foreign) }, "not_found: image not found"},
		{"an unknown image", func() error { return set(s.master, s.mapID, pid, newKey()) }, "not_found: image not found"},
		{"not a scene point with no images", func() error { return set(s.master, s.mapID, battle.GetId()) }, "invalid_argument: only a SCENE point has actions and clues"},
		{"the most the scene holds", func() error { return set(s.master, s.mapID, pid, ids[:maxSceneImages]...) }, "ok"},
	} {
		if got := imageRefusal(c.call()); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if got := sceneImageIDs(s.masterPointOf(pid).GetImages()); strings.Join(got, ",") != strings.Join(ids[:maxSceneImages], ",") {
		t.Errorf("the list after the refusals = %v, want the 8 set last", got)
	}

	// Who hears each change: the scene is open, so everyone hears it, and a map
	// the players see hears the point's change too.
	probe := s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()
	s.master.setPointRevealed(s.campaign, s.point, true)
	s.master.mustOpenScene(s.campaign, pid)
	type step struct {
		name string
		do   func() error
		want [2]string // master, Ana
	}
	for _, st := range []step{
		{"a new list", func() error { return set(s.master, s.mapID, pid, ids[1], ids[0]) }, [2]string{"map_changed,scene_changed", "map_changed,scene_changed"}},
		{"the same list", func() error { return set(s.master, s.mapID, pid, ids[1], ids[0]) }, [2]string{"", ""}},
		{"the same images in another order", func() error { return set(s.master, s.mapID, pid, ids[0], ids[1]) }, [2]string{"map_changed,scene_changed", "map_changed,scene_changed"}},
		{"cleared", func() error { return set(s.master, s.mapID, pid) }, [2]string{"map_changed,scene_changed", "map_changed,scene_changed"}},
		{"cleared again", func() error { return set(s.master, s.mapID, pid) }, [2]string{"", ""}},
		{"a refused list", func() error { return set(s.master, s.mapID, pid, foreign) }, [2]string{"", ""}},
	} {
		mw, aw := s.master.watch(s.campaign), s.ana.watch(s.campaign)
		_ = st.do()
		s.master.setMapRevealed(s.campaign, probe, true)
		got := [2]string{imageEvents(mw, probe), imageEvents(aw, probe)}
		if got != st.want {
			t.Errorf("%s: the master and Ana heard %q, want %q", st.name, got, st.want)
		}
		s.master.setMapRevealed(s.campaign, probe, false)
	}
}

func TestDeleteGalleryImageRefusalsAndWhoHears(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	del := func(u *user, campaign, image string) error {
		_, err := u.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: campaign, ImageId: image}))
		return err
	}
	plain := s.master.newImage(s.campaign)
	inUse := s.master.newImage(s.campaign)
	s.master.createMap(s.campaign, "Mapa do Vale", inUse)
	foreign := s.master.newImage(s.h.newCampaign(s.master))

	for _, c := range []struct {
		name string
		call func() error
		want string
	}{
		{"a player", func() error { return del(s.ana, s.campaign, plain) }, "permission_denied: only the campaign's master can do this"},
		{"an id that is not a UUID", func() error { return del(s.master, s.campaign, "x") }, "not_found: image not found"},
		{"an unknown image", func() error { return del(s.master, s.campaign, newKey()) }, "not_found: image not found"},
		{"an image of another campaign", func() error { return del(s.master, s.campaign, foreign) }, "not_found: image not found"},
		{"an image a map uses", func() error { return del(s.master, s.campaign, inUse) }, "failed_precondition: a map uses this image; change the map's image first"},
	} {
		if got := imageRefusal(c.call()); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	// The refusal names the maps that use the image.
	err := del(s.master, s.campaign, inUse)
	ce, _ := err.(*connect.Error) //nolint:errorlint // the handlers return the Connect error itself
	var named []string
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatalf("detail: %v", derr)
		}
		if use, ok := v.(*mapsv1.ImageInUse); ok {
			for _, m := range use.GetMaps() {
				named = append(named, m.GetName())
			}
		}
	}
	if strings.Join(named, ",") != "Mapa do Vale" {
		t.Errorf("the refusal names the maps %v, want [Mapa do Vale]", named)
	}

	// Who hears the delete: the image shown, kept for the players, or an NPC's portrait.
	probe := s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()
	show := func(image string, keep bool) {
		t.Helper()
		if _, err := s.master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: s.campaign, ImageId: image, Keep: keep})); err != nil {
			t.Fatalf("SetShownImage() error = %v", err)
		}
	}
	shownKept := s.master.newImage(s.campaign)
	leftOnly := s.master.newImage(s.campaign)
	portrait := s.master.newImage(s.campaign)
	s.master.createNPC(s.campaign, "Ludo", portrait)
	show(leftOnly, true)
	show(shownKept, true)
	for _, st := range []struct {
		name  string
		image string
		want  string
	}{
		{"shown (and kept: it is left only once it stops being shown)", shownKept, "shown_image_changed"},
		{"kept only", leftOnly, "left_images_changed"},
		{"a portrait", portrait, "stage_changed"},
		{"a plain image", plain, ""},
	} {
		aw := s.ana.watch(s.campaign)
		if err := del(s.master, s.campaign, st.image); err != nil {
			t.Fatalf("%s: DeleteGalleryImage() error = %v", st.name, err)
		}
		s.master.setMapRevealed(s.campaign, probe, true)
		if got := imageEvents(aw, probe); got != st.want {
			t.Errorf("%s: Ana heard %q, want %q", st.name, got, st.want)
		}
		s.master.setMapRevealed(s.campaign, probe, false)
	}
}
