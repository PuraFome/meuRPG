package maps

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
)

// The images of an RP scene (MR-015, "Imagens da cena"): the master attaches
// gallery images to a scene point, in order; only the master ever reads the
// list (RN-10).

func (u *user) setSceneImages(campaignID, mapID, pointID string, ids ...string) (*mapsv1.SetSceneImagesResponse, error) {
	res, err := u.maps.SetSceneImages(u.h.t.Context(), connect.NewRequest(&mapsv1.SetSceneImagesRequest{CampaignId: campaignID, MapId: mapID, PointId: pointID, ImageIds: ids}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func sceneImageIDs(images []*mapsv1.SceneImage) []string {
	out := make([]string, 0, len(images))
	for _, i := range images {
		out = append(out, i.GetId())
	}
	return out
}

func TestMR015_TheMasterKeepsAnOrderedListOfImagesOnAScene(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	a, b, c := s.master.newImage(s.campaign), s.master.newImage(s.campaign), s.master.newImage(s.campaign)

	res, err := s.master.setSceneImages(s.campaign, s.mapID, pid, a, b, c)
	if err != nil {
		t.Fatalf("SetSceneImages() error = %v", err)
	}
	if got := sceneImageIDs(res.GetImages()); !slices.Equal(got, []string{a, b, c}) {
		t.Fatalf("images = %v, want %v", got, []string{a, b, c})
	}
	if res.GetImages()[0].GetName() == "" {
		t.Errorf("the image has no gallery name")
	}
	// Reordered and shortened: the whole list is replaced.
	res, err = s.master.setSceneImages(s.campaign, s.mapID, pid, c, a)
	if err != nil || !slices.Equal(sceneImageIDs(res.GetImages()), []string{c, a}) {
		t.Fatalf("SetSceneImages(c, a) = %v, %v", res, err)
	}
	// The master's map read and the open scene carry the list, in order.
	if got := sceneImageIDs(s.masterPointOf(pid).GetImages()); !slices.Equal(got, []string{c, a}) {
		t.Errorf("the master's GetMap images = %v, want %v", got, []string{c, a})
	}
	if got := sceneImageIDs(s.master.mustOpenScene(s.campaign, pid).GetImages()); !slices.Equal(got, []string{c, a}) {
		t.Errorf("the master's open scene images = %v, want %v", got, []string{c, a})
	}
	// The same list again is a no-op; an empty one clears it.
	if res, err = s.master.setSceneImages(s.campaign, s.mapID, pid, c, a); err != nil || len(res.GetImages()) != 2 {
		t.Fatalf("repeating the list = %v, %v", res, err)
	}
	if res, err = s.master.setSceneImages(s.campaign, s.mapID, pid); err != nil || len(res.GetImages()) != 0 {
		t.Fatalf("clearing the list = %v, %v", res, err)
	}
}

func TestMR015_SceneImageLimitsAndRefusals(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	pid := s.point.GetId()
	var ids []string
	for range maxSceneImages + 1 {
		ids = append(ids, s.master.newImage(s.campaign))
	}
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, pid, ids[:maxSceneImages]...); err != nil {
		t.Fatalf("%d images: error = %v", maxSceneImages, err)
	}
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, pid, ids...); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("%d images error = %v, want invalid_argument", maxSceneImages+1, err)
	}
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, pid, ids[0], ids[0]); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("the same image twice error = %v, want invalid_argument", err)
	}
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, pid, "not-a-uuid"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a bad id error = %v, want invalid_argument", err)
	}
	// An image of another campaign is refused as a whole, and the list stays.
	other := s.h.newCampaign(s.master)
	foreign := s.master.newImage(other)
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, pid, ids[0], foreign); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an image of another campaign error = %v, want not_found", err)
	}
	if got := sceneImageIDs(s.masterPointOf(pid).GetImages()); !slices.Equal(got, ids[:maxSceneImages]) {
		t.Errorf("the list after refused calls = %v, want it unchanged", got)
	}
	// Only a SCENE point has images.
	battle := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE, Name: "Emboscada"})
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, battle.GetId(), ids[0]); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("images on a battle point error = %v, want invalid_argument", err)
	}
}

func TestMR015_DeletingTheImageOrChangingTheKindClearsTheLink(t *testing.T) {
	t.Parallel()
	s := newScenes(t, false)
	pid := s.point.GetId()
	a, b := s.master.newImage(s.campaign), s.master.newImage(s.campaign)
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, pid, a, b); err != nil {
		t.Fatalf("SetSceneImages() error = %v", err)
	}
	if _, err := s.master.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: s.campaign, ImageId: a})); err != nil {
		t.Fatalf("DeleteGalleryImage() error = %v", err)
	}
	if got := sceneImageIDs(s.masterPointOf(pid).GetImages()); !slices.Equal(got, []string{b}) {
		t.Errorf("images after deleting one = %v, want %v", got, []string{b})
	}
	kind := mapsv1.MapPointKind_MAP_POINT_KIND_BATTLE
	if _, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, Kind: &kind}); err != nil {
		t.Fatalf("UpdateMapPoint(kind) error = %v", err)
	}
	back := mapsv1.MapPointKind_MAP_POINT_KIND_SCENE
	if _, err := s.master.updatePoint(&mapsv1.UpdateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, PointId: pid, Kind: &back}); err != nil {
		t.Fatalf("UpdateMapPoint(kind) error = %v", err)
	}
	if got := s.masterPointOf(pid).GetImages(); len(got) != 0 {
		t.Errorf("images after the point stopped being a scene = %v, want none", got)
	}
}

func TestRN10_PlayersNeverReceiveASceneImageList(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	a := s.master.newImage(s.campaign)
	if _, err := s.master.setSceneImages(s.campaign, s.mapID, pid, a); err != nil {
		t.Fatalf("SetSceneImages() error = %v", err)
	}
	// Reveal the point, open the scene: the player reads both.
	s.master.setPointRevealed(s.campaign, s.point, true)
	s.master.mustOpenScene(s.campaign, pid)
	for _, p := range s.ana.mustGetMap(s.campaign, s.mapID).GetPoints() {
		if len(p.GetImages()) != 0 {
			t.Errorf("a player's point %s carries images %v", p.GetId(), p.GetImages())
		}
	}
	scene := s.ana.getScene(s.campaign)
	if scene == nil || len(scene.GetImages()) != 0 {
		t.Fatalf("the player's open scene = %v, want it with no images", scene)
	}
	if raw, err := protojson.Marshal(scene); err != nil || strings.Contains(string(raw), a) {
		t.Errorf("the player's open scene mentions the image id (err %v): %s", err, raw)
	}
	// Positive control: the master reads it.
	if got := s.master.getScene(s.campaign).GetImages(); len(got) != 1 || got[0].GetId() != a {
		t.Errorf("the master's open scene images = %v, want the image", got)
	}
	// A player may not set it.
	if _, err := s.ana.setSceneImages(s.campaign, s.mapID, pid, a); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player's SetSceneImages error = %v, want permission_denied", err)
	}
}
