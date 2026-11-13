package maps

import (
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/maps/images/gen"
)

// The writes of the gallery, a map's image and the dungeons that change what a live session shows run under play's funnel (#735 part 2,
// 4d maps): the session's number moves once with the write, and what the write tells carries it; a repeat of a key moves nothing and
// tells again; a refused call moves nothing; with no open session nothing moves.

// hintSeqOf is the open session's number.
func (h *harness) hintSeqOf(campaignID string) int64 {
	h.t.Helper()
	var n int64
	if err := h.pool.QueryRow(h.t.Context(), `SELECT hint_seq FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL`, campaignID).Scan(&n); err != nil {
		h.t.Fatalf("read the session's number: %v", err)
	}
	return n
}

func (h *harness) wantSeqMoves(campaignID string, by int64, what string, call func()) {
	h.t.Helper()
	before := h.hintSeqOf(campaignID)
	call()
	if got := h.hintSeqOf(campaignID); got != before+by {
		h.t.Errorf("%s: hint_seq = %d, want %d", what, got, before+by)
	}
}

func TestRenameGalleryImageMovesTheNumberOnce(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	img := s.master.newImage(s.campaign)
	rename := func(name string) error {
		_, err := s.master.gallery.RenameGalleryImage(t.Context(), connect.NewRequest(&mapsv1.RenameGalleryImageRequest{CampaignId: s.campaign, ImageId: img, Name: name}))
		return err
	}
	// An image no screen shows has no caption to change: the number stays.
	s.h.wantSeqMoves(s.campaign, 0, "a rename of an image nobody sees", func() {
		if err := rename("Mapa novo"); err != nil {
			t.Fatalf("RenameGalleryImage() error = %v", err)
		}
	})
	if _, err := s.master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: s.campaign, ImageId: img})); err != nil {
		t.Fatalf("SetShownImage() error = %v", err)
	}
	s.h.wantSeqMoves(s.campaign, 1, "a rename of the image on screen", func() {
		if err := rename("Mapa da tela"); err != nil {
			t.Fatalf("RenameGalleryImage(shown) error = %v", err)
		}
	})
	s.h.wantSeqMoves(s.campaign, 0, "a refused rename", func() {
		if err := rename(""); err == nil {
			t.Fatal("RenameGalleryImage(empty) succeeded")
		}
	})
}

func TestDeleteGalleryImageMovesTheNumberOnce(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	img := s.master.newImage(s.campaign)
	s.h.wantSeqMoves(s.campaign, 1, "a delete", func() {
		if _, err := s.master.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: s.campaign, ImageId: img})); err != nil {
			t.Fatalf("DeleteGalleryImage() error = %v", err)
		}
	})
	s.h.wantSeqMoves(s.campaign, 0, "a delete of what is gone", func() {
		if _, err := s.master.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: s.campaign, ImageId: img})); err == nil {
			t.Fatal("DeleteGalleryImage(again) succeeded")
		}
	})
}

func TestDungeonWritesMoveTheNumberOnce(t *testing.T) {
	t.Parallel()
	d := newDungeonTable(t)
	d.master.start(d.campaign)
	seed, _ := testDungeonSeed(t)
	var created *mapsv1.CreateDungeonMapResponse
	d.h.wantSeqMoves(d.campaign, 1, "a new dungeon", func() {
		created = d.master.createDungeon(d.campaign, "Masmorra", testDungeonOptions(), seed)
	})
	d.h.wantSeqMoves(d.campaign, 1, "a redraw", func() {
		if _, err := d.master.redraw(d.campaign, created.GetMap().GetId()); err != nil {
			t.Fatalf("RedrawDungeonMap() error = %v", err)
		}
	})
	d.h.wantSeqMoves(d.campaign, 0, "a redraw of a map that is not there", func() {
		if _, err := d.master.redraw(d.campaign, "6f1c7a52-3b5e-4c55-9d0b-2a51f0c1e0ff"); err == nil {
			t.Fatal("RedrawDungeonMap(unknown) succeeded")
		}
	})
}

func TestUseTheTexturedMapMovesTheNumberOnce(t *testing.T) {
	t.Parallel()
	c := newCave(t, withFake(&gen.Fake{}, 20))
	texture := wantDone(t, "textured map", c.master.mustGenerateFromMap(c.campaign, c.mapID, kindTexturedAPI, "Uma caverna úmida"))
	c.h.wantSeqMoves(c.campaign, 1, "using the textured map", func() {
		if _, err := c.master.useAsMapImage(c.campaign, texture.GetId()); err != nil {
			t.Fatalf("UseGeneratedImageAsMapImage() error = %v", err)
		}
	})
	// The repeat moves no number and tells the master again, in case the first call died before it told.
	watch := c.master.watch(c.campaign)
	c.h.wantSeqMoves(c.campaign, 0, "using it twice", func() {
		if _, err := c.master.useAsMapImage(c.campaign, texture.GetId()); err != nil {
			t.Fatalf("UseGeneratedImageAsMapImage(again) error = %v", err)
		}
	})
	watch.mapChanged(c.mapID)
}
