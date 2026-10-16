package play

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of SetShownImage (onscreen.go) with the real maps module's
// gallery and, where the maps module's copy of a fog map's image matters, a
// stand-in that hands back a copy of its own.

// showMaps is the MapKeeper of the harness with PrepareShow and LeaveImage that a test may replace.
type showMaps struct {
	MapKeeper
	prepare func(ctx context.Context, campaignID, imageID string) (*playv1.ShownImage, ShownCopy, error)
	leave   func(ctx context.Context, tx pgx.Tx, campaignID, imageID string, at time.Time) error
}

func (m *showMaps) PrepareShow(ctx context.Context, campaignID, imageID string) (*playv1.ShownImage, ShownCopy, error) {
	if m.prepare != nil {
		return m.prepare(ctx, campaignID, imageID)
	}
	return m.MapKeeper.PrepareShow(ctx, campaignID, imageID)
}

func (m *showMaps) LeaveImage(ctx context.Context, tx pgx.Tx, campaignID, imageID string, at time.Time) error {
	if m.leave != nil {
		return m.leave(ctx, tx, campaignID, imageID, at)
	}
	return m.MapKeeper.LeaveImage(ctx, tx, campaignID, imageID, at)
}

// fakeCopy is a ShownCopy that records what became of it.
type fakeCopy struct {
	id        string
	created   bool
	insertErr error
	inserted  bool
	discarded bool
}

func (c *fakeCopy) Insert(context.Context, pgx.Tx) (string, bool, error) {
	c.inserted = true
	return c.id, c.created, c.insertErr
}

func (c *fakeCopy) Discard(context.Context) { c.discarded = true }

// shownTable is a campaign with an open session, a master, a player and gallery images.
type shownTable struct {
	*harness
	master, ana *user
	campaign    string
	maps        *showMaps
}

func newShownTable(t *testing.T) *shownTable {
	t.Helper()
	h := newHarness(t)
	master, ana := h.newUser("Mestre"), h.newUser("Ana")
	st := &shownTable{harness: h, master: master, ana: ana, campaign: h.newCampaign(master, "Mesa", ana)}
	st.maps = &showMaps{MapKeeper: h.svc.maps}
	h.svc.maps = st.maps
	return st
}

func (st *shownTable) image(t *testing.T, campaign string) string {
	t.Helper()
	id := newKey()
	if _, err := st.pool.Exec(t.Context(),
		`INSERT INTO gallery_images (id, campaign_id, name, content_type, width, height, byte_size, created_at) VALUES ($1, $2, 'imagem', 'image/png', 40, 30, 100, now())`,
		id, campaign); err != nil {
		t.Fatalf("insert a gallery image: %v", err)
	}
	return id
}

func (st *shownTable) show(t *testing.T, image string, keep bool) (*playv1.SetShownImageResponse, error) {
	t.Helper()
	res, err := st.master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: st.campaign, ImageId: image, Keep: keep}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (st *shownTable) dbShown(t *testing.T) (id string, keep bool) {
	t.Helper()
	var shown *string
	if err := st.pool.QueryRow(t.Context(), `SELECT shown_image_id, shown_image_keep FROM game_sessions WHERE campaign_id = $1 AND ended_at IS NULL`, st.campaign).Scan(&shown, &keep); err != nil {
		t.Fatalf("read the shown image: %v", err)
	}
	return deref(shown), keep
}

func (st *shownTable) left(t *testing.T) []string {
	t.Helper()
	res, err := st.ana.play.ListLeftImages(t.Context(), connect.NewRequest(&playv1.ListLeftImagesRequest{CampaignId: st.campaign}))
	if err != nil {
		t.Fatalf("ListLeftImages() error = %v", err)
	}
	var ids []string
	for _, im := range res.Msg.GetImages() {
		ids = append(ids, im.GetId())
	}
	return ids
}

func (w *watcher) wantShown(t *testing.T, what, image string, left bool) {
	t.Helper()
	ev := w.nextChange(t)
	if got := ev.GetShownImageChanged(); got == nil || got.GetImage().GetId() != image {
		t.Fatalf("%s: event = %v, want shown_image_changed to %q", what, ev, image)
	}
	if left {
		if ev := w.nextChange(t); ev.GetLeftImagesChanged() == nil {
			t.Fatalf("%s: event = %v, want left_images_changed", what, ev)
		}
	}
}

func TestSetShownImage_RefusalsBeforeAndWithoutASession(t *testing.T) {
	t.Parallel()
	st := newShownTable(t)
	other := st.image(t, st.campaign)
	foreign := st.harness.newCampaign(st.master, "Outra")
	foreignImage := st.image(t, foreign)

	_, err := st.show(t, "nope", false)
	rmWant(t, "image not a UUID", err, connect.CodeNotFound, "image not found")
	_, err = st.show(t, newKey(), false)
	rmWant(t, "unknown image", err, connect.CodeNotFound, "")
	_, err = st.show(t, foreignImage, false)
	rmWant(t, "image of another campaign", err, connect.CodeNotFound, "")
	_, err = st.show(t, other, false)
	wantBlocked(t, "show with no session", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION)
	_, err = st.show(t, "", false)
	wantBlocked(t, "show nothing with no session", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION)
	// A player may not.
	st.master.start(t, st.campaign)
	_, err = st.ana.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: st.campaign, ImageId: other}))
	rmWant(t, "a player", err, connect.CodePermissionDenied, "")
}

func TestSetShownImage_ShowKeepReplaceAndStop(t *testing.T) {
	t.Parallel()
	st := newShownTable(t)
	a, b := st.image(t, st.campaign), st.image(t, st.campaign)
	st.master.start(t, st.campaign)
	pw := st.ana.watch(t, st.campaign)
	pw.ready(t)

	first, err := st.show(t, a, false)
	if err != nil || first.GetShownImage().GetId() != a || first.GetKeep() {
		t.Fatalf("show A = %v, %v; want A shown, not kept", first, err)
	}
	pw.wantShown(t, "show A", a, false)
	if id, keep := st.dbShown(t); id != a || keep {
		t.Errorf("session shows %q keep %v, want A not kept", id, keep)
	}

	// The same image again, to turn the switch on: nothing for the players.
	again, err := st.show(t, a, true)
	if err != nil || again.GetShownImage().GetId() != a || !again.GetKeep() {
		t.Fatalf("show A again = %v, %v; want A shown, kept", again, err)
	}
	if id, keep := st.dbShown(t); id != a || !keep {
		t.Errorf("session shows %q keep %v, want A kept", id, keep)
	}

	// Another image replaces the kept one: A is left with the players.
	if _, err := st.show(t, b, false); err != nil {
		t.Fatalf("show B error = %v", err)
	}
	pw.wantShown(t, "show B", b, true)
	if got := st.left(t); len(got) != 1 || got[0] != a {
		t.Errorf("left images = %v, want [A]", got)
	}

	// Nothing: B was not kept, so it does not leave; keep is ignored without an image.
	none, err := st.show(t, "", true)
	if err != nil || none.GetKeep() || none.GetShownImage() != nil {
		t.Fatalf("show nothing = %v, %v; want no image and keep off", none, err)
	}
	pw.wantShown(t, "stop", "", false)
	if id, keep := st.dbShown(t); id != "" || keep {
		t.Errorf("session shows %q keep %v, want nothing", id, keep)
	}
	if got := st.left(t); len(got) != 1 {
		t.Errorf("left images = %v, want only A", got)
	}
	// Stopping again changes nothing.
	if _, err := st.show(t, "", false); err != nil {
		t.Fatalf("stop again error = %v", err)
	}
}

func TestSetShownImage_AnImageDeletedMeanwhileIsNotFound(t *testing.T) {
	t.Parallel()
	st := newShownTable(t)
	st.master.start(t, st.campaign)
	ghost := newKey()
	st.maps.prepare = func(context.Context, string, string) (*playv1.ShownImage, ShownCopy, error) {
		return &playv1.ShownImage{Id: ghost}, nil, nil // the image was deleted after the check
	}
	_, err := st.show(t, st.image(t, st.campaign), false)
	rmWant(t, "image deleted meanwhile", err, connect.CodeNotFound, "image not found")
}

func TestSetShownImage_PrepareAndLeaveFailures(t *testing.T) {
	t.Parallel()
	st := newShownTable(t)
	a, b := st.image(t, st.campaign), st.image(t, st.campaign)
	st.master.start(t, st.campaign)
	boom := errors.New("boom")
	st.maps.prepare = func(context.Context, string, string) (*playv1.ShownImage, ShownCopy, error) {
		return nil, nil, boom
	}
	_, err := st.show(t, a, false)
	rmWant(t, "prepare fails", err, connect.CodeInternal, "")
	st.maps.prepare = nil

	if _, err := st.show(t, a, true); err != nil {
		t.Fatalf("show A error = %v", err)
	}
	st.maps.leave = func(context.Context, pgx.Tx, string, string, time.Time) error { return boom }
	_, err = st.show(t, b, false)
	rmWant(t, "leave fails", err, connect.CodeInternal, "")
	if id, keep := st.dbShown(t); id != a || !keep {
		t.Errorf("session shows %q keep %v after a failed replace, want A kept", id, keep)
	}
}

func TestSetShownImage_TheCopyOfAFogMapsImage(t *testing.T) {
	t.Parallel()
	st := newShownTable(t)
	original, copyID, winner := st.image(t, st.campaign), st.image(t, st.campaign), st.image(t, st.campaign)
	st.master.start(t, st.campaign)
	pw := st.ana.watch(t, st.campaign)
	pw.ready(t)
	prep := func(c *fakeCopy) {
		st.maps.prepare = func(context.Context, string, string) (*playv1.ShownImage, ShownCopy, error) {
			return &playv1.ShownImage{Id: copyID}, c, nil
		}
	}

	// The copy's row is made: it is the image shown, and it is kept.
	made := &fakeCopy{id: copyID, created: true}
	prep(made)
	res, err := st.show(t, original, false)
	if err != nil || res.GetShownImage().GetId() != copyID || !made.inserted || made.discarded {
		t.Fatalf("show with a new copy = %v, %v (inserted %v, discarded %v); want the copy shown and kept", res, err, made.inserted, made.discarded)
	}
	pw.wantShown(t, "new copy", copyID, false)

	// Another show committed the copy first: its image is the one shown and these files go.
	lost := &fakeCopy{id: winner, created: false}
	prep(lost)
	res, err = st.show(t, original, false)
	if err != nil || res.GetShownImage().GetId() != winner || !lost.discarded {
		t.Fatalf("show with a lost copy = %v, %v (discarded %v); want the winner shown and the files discarded", res, err, lost.discarded)
	}
	pw.wantShown(t, "lost copy", winner, false)

	// The copy's row cannot be made (the gallery is full): the files go and the call is refused.
	full := &fakeCopy{insertErr: connect.NewError(connect.CodeResourceExhausted, errors.New("gallery full"))}
	prep(full)
	_, err = st.show(t, original, false)
	rmWant(t, "gallery full", err, connect.CodeResourceExhausted, "")
	if !full.discarded {
		t.Error("the files of a copy that could not be inserted were kept")
	}
	if id, _ := st.dbShown(t); id != winner {
		t.Errorf("session shows %q, want the winner still", id)
	}
}

func TestSetShownImage_ACopyIsDiscardedWhenNothingIsShown(t *testing.T) {
	t.Parallel()
	st := newShownTable(t)
	cp := &fakeCopy{id: newKey(), created: true}
	st.maps.prepare = func(context.Context, string, string) (*playv1.ShownImage, ShownCopy, error) {
		return &playv1.ShownImage{Id: cp.id}, cp, nil
	}
	_, err := st.show(t, st.image(t, st.campaign), false)
	wantBlocked(t, "show with no session", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION)
	if cp.inserted || !cp.discarded {
		t.Errorf("copy inserted %v, discarded %v; want its files discarded and no row", cp.inserted, cp.discarded)
	}
}
