package maps

import (
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
)

// MR-039: "Usar como imagem do mapa" refuses a map that is no longer the one the
// picture was drawn for. These tests change the request's basis in the database to
// reach the checks the RPCs cannot reach one by one.

func wantUseMapChanged(t *testing.T, call string, err error) {
	t.Helper()
	wantGenerationBlocked(t, call, err, mapsv1.ImageGenerationBlockedReason_IMAGE_GENERATION_BLOCKED_REASON_MAP_CHANGED)
}

// A picture whose size is not the one the request recorded is refused as a changed map.
func TestMR039_UseIsRefusedWhenTheSizeOfTheBasisChanged(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	m := c.master
	picture := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna"))
	if _, err := c.h.pool.Exec(t.Context(), "UPDATE image_requests SET map_width = map_width + 1 WHERE image_id = $1", picture.GetId()); err != nil {
		t.Fatalf("change the basis: %v", err)
	}
	_, err := m.useAsMapImage(c.campaign, picture.GetId())
	wantUseMapChanged(t, "Use with another width in the basis", err)
	if got := m.getMapImage(c.campaign, c.mapID); got != c.imageID {
		t.Errorf("the map's image changed in a refused Use: %s", got)
	}
}

// An edit is used over the picture its chain used, however long the chain; a
// chain that lost a request, or that was made over another map image, is refused.
func TestMR039_UseFollowsTheChainOfEdits(t *testing.T) {
	t.Parallel()
	t.Run("two edits deep", func(t *testing.T) {
		t.Parallel()
		c := newCave(t, withFake(&gen.Fake{}, 20))
		m := c.master
		t1 := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna"))
		e1 := wantDone(t, "edit", mustEdit(t, m, c.campaign, t1.GetId()))
		e2 := wantDone(t, "edit of the edit", mustEdit(t, m, c.campaign, e1.GetId()))
		if _, err := m.useAsMapImage(c.campaign, t1.GetId()); err != nil {
			t.Fatalf("Use of the picture error = %v", err)
		}
		used, err := m.useAsMapImage(c.campaign, e2.GetId())
		if err != nil || used.GetMap().GetImage().GetId() != e2.GetId() {
			t.Errorf("Use of the second edit = %v, %v; want the edit as the map's image", used.GetMap().GetImage().GetId(), err)
		}
	})
	t.Run("a request of the chain is gone", func(t *testing.T) {
		t.Parallel()
		c := newCave(t, withFake(&gen.Fake{}, 20))
		m := c.master
		t1 := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna"))
		e1 := wantDone(t, "edit", mustEdit(t, m, c.campaign, t1.GetId()))
		if _, err := m.useAsMapImage(c.campaign, t1.GetId()); err != nil {
			t.Fatalf("Use of the picture error = %v", err)
		}
		if _, err := c.h.pool.Exec(t.Context(), "DELETE FROM image_requests WHERE image_id = $1", t1.GetId()); err != nil {
			t.Fatalf("delete the request: %v", err)
		}
		_, err := m.useAsMapImage(c.campaign, e1.GetId())
		wantUseMapChanged(t, "Use of an edit whose source lost its request", err)
	})
	t.Run("the chain was made over another map image", func(t *testing.T) {
		t.Parallel()
		c := newCave(t, withFake(&gen.Fake{}, 20))
		m := c.master
		t1 := wantDone(t, "textured map", m.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna"))
		e1 := wantDone(t, "edit", mustEdit(t, m, c.campaign, t1.GetId()))
		if _, err := m.useAsMapImage(c.campaign, t1.GetId()); err != nil {
			t.Fatalf("Use of the picture error = %v", err)
		}
		if _, err := c.h.pool.Exec(t.Context(), "UPDATE image_requests SET map_image_id = $2 WHERE image_id = $1", t1.GetId(), e1.GetId()); err != nil {
			t.Fatalf("change the basis: %v", err)
		}
		_, err := m.useAsMapImage(c.campaign, e1.GetId())
		wantUseMapChanged(t, "Use of an edit over another map image", err)
	})
}

// A Use of an image that is not a generated picture of the campaign is not_found.
func TestMR039_UseOfAnImageThatIsNotAPictureOfTheMapIsNotFound(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	m := c.master
	_, err := m.useAsMapImage(c.campaign, "not-a-uuid")
	wantCode(t, "Use of a bad id", err, connect.CodeNotFound)
	_, err = m.useAsMapImage(c.campaign, c.imageID)
	wantCode(t, "Use of an uploaded image", err, connect.CodeNotFound)
	_, err = m.useAsMapImage(c.campaign, "00000000-0000-4000-8000-000000000001")
	wantCode(t, "Use of an image that does not exist", err, connect.CodeNotFound)
}
