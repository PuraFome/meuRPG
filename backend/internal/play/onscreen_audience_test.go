package play

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The shown image has an audience (MR-028, RN-10): the whole table, the scene's players, or chosen
// players. The master changes it while the image is shown; narrowing takes the image away at once.

const notPartialMessage = "no scene open to only some players is open"

const (
	kindTable  = playv1.ImageAudienceKind_IMAGE_AUDIENCE_KIND_WHOLE_TABLE
	kindScene  = playv1.ImageAudienceKind_IMAGE_AUDIENCE_KIND_SCENE_PLAYERS
	kindChosen = playv1.ImageAudienceKind_IMAGE_AUDIENCE_KIND_CHOSEN_PLAYERS
)

func imageAudience(kind playv1.ImageAudienceKind, us ...*user) *playv1.ImageAudience {
	return &playv1.ImageAudience{Kind: kind, UserIds: userIDs(us...)}
}

// image makes a gallery image of the campaign.
func (at *audienceTable) image(t *testing.T) string {
	t.Helper()
	id := newKey()
	if _, err := at.h.pool.Exec(t.Context(),
		`INSERT INTO gallery_images (id, campaign_id, name, content_type, width, height, byte_size, created_at) VALUES ($1, $2, 'imagem', 'image/png', 40, 30, 100, now())`,
		id, at.campaignID); err != nil {
		t.Fatalf("insert a gallery image: %v", err)
	}
	return id
}

func (at *audienceTable) show(image string, keep bool, aud *playv1.ImageAudience) (*playv1.SetShownImageResponse, error) {
	res, err := at.master.play.SetShownImage(at.h.t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: at.campaignID, ImageId: image, Keep: keep, Audience: aud}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (at *audienceTable) mustShow(t *testing.T, image string, keep bool, aud *playv1.ImageAudience) *playv1.ShownImage {
	t.Helper()
	res, err := at.show(image, keep, aud)
	if err != nil {
		t.Fatalf("SetShownImage(%q) error = %v", image, err)
	}
	return res.GetShownImage()
}

func (at *audienceTable) setImageAudience(u *user, aud *playv1.ImageAudience) (*playv1.ShownImage, error) {
	res, err := u.play.SetShownImageAudience(at.h.t.Context(), connect.NewRequest(&playv1.SetShownImageAudienceRequest{CampaignId: at.campaignID, Audience: aud}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetShownImage(), nil
}

func (at *audienceTable) mustSetImageAudience(t *testing.T, aud *playv1.ImageAudience) *playv1.ShownImage {
	t.Helper()
	img, err := at.setImageAudience(at.master, aud)
	if err != nil {
		t.Fatalf("SetShownImageAudience(%v) error = %v", aud, err)
	}
	return img
}

// live is the image the user reads in GetLiveSession.
func (at *audienceTable) live(t *testing.T, u *user) *playv1.ShownImage {
	t.Helper()
	res, err := u.play.GetLiveSession(t.Context(), connect.NewRequest(&playv1.GetLiveSessionRequest{CampaignId: at.campaignID}))
	if err != nil {
		t.Fatalf("GetLiveSession() error = %v", err)
	}
	return res.Msg.GetShownImage()
}

// leftFor lists the images left with the players that the user reads.
func (at *audienceTable) leftFor(t *testing.T, u *user) []string {
	t.Helper()
	res, err := u.play.ListLeftImages(t.Context(), connect.NewRequest(&playv1.ListLeftImagesRequest{CampaignId: at.campaignID}))
	if err != nil {
		t.Fatalf("ListLeftImages() error = %v", err)
	}
	var ids []string
	for _, im := range res.Msg.GetImages() {
		ids = append(ids, im.GetId())
	}
	return ids
}

// fetches says whether the image route would give the user the image: the service's answer to the maps module.
func (at *audienceTable) fetches(t *testing.T, u *user, image string) bool {
	t.Helper()
	ok, err := at.h.svc.ImageShownTo(t.Context(), at.campaignID, u.id, image)
	if err != nil {
		t.Fatalf("ImageShownTo() error = %v", err)
	}
	return ok
}

// wantSeen checks, for each of Ana, Caio and Bia, whether they read the image in GetLiveSession and may fetch it.
func (at *audienceTable) wantSeen(t *testing.T, what, image string, ana, caio, bia bool) {
	t.Helper()
	for u, want := range map[*user]bool{at.ana: ana, at.caio: caio, at.bia: bia} {
		got := at.live(t, u).GetId() == image
		if got != want {
			t.Errorf("%s: a player reads the shown image = %v, want %v", what, got, want)
		}
		if fetch := at.fetches(t, u, image); fetch != want {
			t.Errorf("%s: a player may fetch the image = %v, want %v", what, fetch, want)
		}
	}
	if got := at.live(t, at.master).GetId(); got != image {
		t.Errorf("%s: the master reads %q, want the image", what, got)
	}
}

func audienceIDs(a *playv1.ImageAudience) []string {
	ids := slices.Clone(a.GetUserIds())
	slices.Sort(ids)
	return ids
}

func sortedIDs(us ...*user) []string {
	ids := userIDs(us...)
	slices.Sort(ids)
	return ids
}

func TestImageAudience_TheDefaultFollowsTheScene(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	a, b, c := at.image(t), at.image(t), at.image(t)

	// No scene is open: the whole table, whatever the default.
	shown := at.mustShow(t, a, false, nil)
	if shown.GetAudience().GetKind() != kindTable || len(shown.GetAudience().GetUserIds()) != 0 {
		t.Errorf("audience with no scene = %v, want the whole table", shown.GetAudience())
	}
	at.wantSeen(t, "no scene", a, true, true, true)
	wantWords(t, "shown with no scene", at.hints(t), "image:"+a, "image:"+a, "image:"+a, "image:"+a)

	// The scene's players are asked for when no scene is partial: refused, nothing changes.
	_, err := at.show(b, false, imageAudience(kindScene))
	wantSceneRefusal(t, "scene players with no scene", err, connect.CodeFailedPrecondition, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_SCENE_NOT_PARTIAL, notPartialMessage)
	at.openScene(t, at.point)
	_, err = at.show(b, false, imageAudience(kindScene))
	wantSceneRefusal(t, "scene players with a scene open to everyone", err, connect.CodeFailedPrecondition, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_SCENE_NOT_PARTIAL, notPartialMessage)
	at.wantSeen(t, "after the refusals", a, true, true, true)
	at.hints(t)

	// A scene open to only some players: the default is the scene's players.
	at.mustSetAudience(t, at.ana, at.caio)
	at.hints(t)
	shown = at.mustShow(t, b, false, nil)
	if shown.GetAudience().GetKind() != kindScene || !slices.Equal(audienceIDs(shown.GetAudience()), sortedIDs(at.ana, at.caio)) {
		t.Errorf("audience in a partial scene = %v, want the scene's players", shown.GetAudience())
	}
	at.wantSeen(t, "scene players", b, true, true, false)
	// Bia saw A and stops seeing anything; the ones in the scene get B.
	wantWords(t, "shown to the scene", at.hints(t), "image:"+b, "image:"+b, "image:"+b, "image:none")
	// A player never reads the audience.
	if got := at.live(t, at.ana).GetAudience(); got != nil {
		t.Errorf("a player reads the audience %v", got)
	}
	if got := at.live(t, at.master).GetAudience(); got.GetKind() != kindScene {
		t.Errorf("the master reads the audience %v, want the scene's players", got)
	}

	// The master says the whole table: everyone gets it, as an image shown with no scene.
	shown = at.mustShow(t, c, false, imageAudience(kindTable))
	if shown.GetAudience().GetKind() != kindTable {
		t.Errorf("audience = %v, want the whole table", shown.GetAudience())
	}
	at.wantSeen(t, "whole table in a partial scene", c, true, true, true)
	wantWords(t, "shown to the table", at.hints(t), "image:"+c, "image:"+c, "image:"+c, "image:"+c)

	// Chosen players.
	d := at.image(t)
	shown = at.mustShow(t, d, false, imageAudience(kindChosen, at.bia))
	if shown.GetAudience().GetKind() != kindChosen || !slices.Equal(audienceIDs(shown.GetAudience()), sortedIDs(at.bia)) {
		t.Errorf("audience = %v, want Bia", shown.GetAudience())
	}
	at.wantSeen(t, "chosen", d, false, false, true)
	wantWords(t, "shown to Bia", at.hints(t), "image:"+d, "image:none", "image:none", "image:"+d)
}

func TestImageAudience_TheSameImageAgainKeepsItsAudience(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	a := at.image(t)
	at.mustShow(t, a, false, imageAudience(kindChosen, at.ana))
	at.hints(t)
	// Turning "Deixar com os jogadores" on is not a new showing: nobody hears, and the audience stays.
	shown := at.mustShow(t, a, true, nil)
	if shown.GetAudience().GetKind() != kindChosen {
		t.Errorf("audience after the switch = %v, want chosen players", shown.GetAudience())
	}
	wantWords(t, "the switch", at.hints(t), "", "", "", "")
	at.wantSeen(t, "after the switch", a, true, false, false)
	// Showing it again with an audience is the same as changing it.
	at.mustShow(t, a, true, imageAudience(kindTable))
	at.wantSeen(t, "shown again to the table", a, true, true, true)
	wantWords(t, "widened by showing again", at.hints(t), "image:"+a, "", "image:"+a, "image:"+a)
}

func TestImageAudience_NarrowingTakesTheImageAwayAtOnce(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	a := at.image(t)
	at.mustShow(t, a, false, nil)
	at.wantSeen(t, "shown", a, true, true, true)
	at.hints(t)

	// Only Ana: Caio and Bia read "no image" and the image route refuses them; Ana hears nothing.
	img := at.mustSetImageAudience(t, imageAudience(kindChosen, at.ana))
	if img.GetId() != a || img.GetAudience().GetKind() != kindChosen {
		t.Errorf("SetShownImageAudience answered %v, want the image with its chosen players", img)
	}
	at.wantSeen(t, "narrowed to Ana", a, true, false, false)
	wantWords(t, "narrowed", at.hints(t), "image:"+a, "", "image:none", "image:none")

	// Widening to Bia: she is told, the others hear nothing.
	at.mustSetImageAudience(t, imageAudience(kindChosen, at.ana, at.bia))
	at.wantSeen(t, "Bia added", a, true, false, true)
	wantWords(t, "Bia added", at.hints(t), "image:"+a, "", "", "image:"+a)

	// The same audience again, in another order: nothing at all.
	at.mustSetImageAudience(t, imageAudience(kindChosen, at.bia, at.ana, at.bia))
	wantWords(t, "the same audience", at.hints(t), "", "", "", "")

	// "Mostrar a toda a mesa": Caio comes in.
	at.mustSetImageAudience(t, imageAudience(kindTable))
	at.wantSeen(t, "the whole table", a, true, true, true)
	wantWords(t, "the whole table", at.hints(t), "image:"+a, "", "image:"+a, "")

	// Chosen players from the whole table: everyone else loses it.
	at.mustSetImageAudience(t, imageAudience(kindChosen, at.caio))
	at.wantSeen(t, "Caio alone", a, false, true, false)
	wantWords(t, "Caio alone", at.hints(t), "image:"+a, "image:none", "", "image:none")
}

func TestImageAudience_SetShownImageAudienceRefusals(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	stranger := at.h.newUser("Estranha")
	// Nothing is shown.
	_, err := at.setImageAudience(at.master, imageAudience(kindTable))
	wantRefusal(t, "no image is shown", err, connect.CodeNotFound, "")
	at.mustShow(t, at.image(t), false, nil)
	_, err = at.setImageAudience(at.ana, imageAudience(kindTable))
	wantRefusal(t, "a player", err, connect.CodePermissionDenied, "")
	_, err = at.setImageAudience(at.master, nil)
	wantRefusal(t, "no audience", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, &playv1.ImageAudience{})
	wantRefusal(t, "an unspecified kind", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, imageAudience(kindChosen))
	wantRefusal(t, "chosen players: nobody", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, imageAudience(kindChosen, at.master))
	wantRefusal(t, "chosen players: the master", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, imageAudience(kindChosen, stranger))
	wantRefusal(t, "chosen players: someone outside", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, &playv1.ImageAudience{Kind: kindChosen, UserIds: []string{"x"}})
	wantRefusal(t, "chosen players: not a UUID", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, imageAudience(kindTable, at.ana))
	wantRefusal(t, "the whole table with a list", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, imageAudience(kindScene, at.ana))
	wantRefusal(t, "the scene's players with a list", err, connect.CodeInvalidArgument, "")
	_, err = at.setImageAudience(at.master, imageAudience(kindScene))
	wantSceneRefusal(t, "the scene's players with no partial scene", err, connect.CodeFailedPrecondition, playv1.SceneBlockedReason_SCENE_BLOCKED_REASON_SCENE_NOT_PARTIAL, notPartialMessage)
	// The refusals changed nothing, and SetShownImage refuses the same audiences.
	at.wantSeen(t, "after the refusals", at.live(t, at.master).GetId(), true, true, true)
	_, err = at.show(at.image(t), false, imageAudience(kindChosen, stranger))
	wantRefusal(t, "show to someone outside", err, connect.CodeInvalidArgument, "")
	// No session.
	campaign := at.h.newCampaign(at.master, "Sem sessão", at.ana)
	_, err = at.master.play.SetShownImageAudience(t.Context(), connect.NewRequest(&playv1.SetShownImageAudienceRequest{CampaignId: campaign, Audience: imageAudience(kindTable)}))
	wantBlocked(t, "no session", err, playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION)
}

func TestImageAudience_TheScenesPlayersFollowTheScene(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	a := at.image(t)
	at.openFor(t, at.point, at.ana, at.caio)
	at.mustShow(t, a, false, nil)
	at.wantSeen(t, "scene players", a, true, true, false)
	at.hints(t)

	// Caio goes out of the scene: he loses the image at once (and the scene); Bia, out already, hears nothing of it.
	at.mustSetAudience(t, at.ana)
	at.wantSeen(t, "Caio out", a, true, false, false)
	if got := at.live(t, at.master).GetAudience(); got.GetKind() != kindScene || !slices.Equal(audienceIDs(got), sortedIDs(at.ana)) {
		t.Errorf("the master reads the audience %v, want the scene's players: Ana", got)
	}
	wantWords(t, "Caio out", at.hints(t), "scene_changed,image:"+a, "scene_changed", "scene_changed,image:none", "")

	// Bia comes in and gets the image with the scene.
	at.mustSetAudience(t, at.ana, at.bia)
	at.wantSeen(t, "Bia in", a, true, false, true)
	wantWords(t, "Bia in", at.hints(t), "scene_changed,image:"+a, "scene_changed", "", "scene_changed,image:"+a)
}

func TestImageAudience_WhenTheSceneStopsBeingTheImagesItStaysWithTheSamePlayers(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cases := []struct {
		name string
		stop func(t *testing.T, at *audienceTable)
	}{
		{"the scene closes", func(t *testing.T, at *audienceTable) {
			if _, err := at.master.play.CloseScene(ctx, connect.NewRequest(&playv1.CloseSceneRequest{CampaignId: at.campaignID})); err != nil {
				t.Fatalf("CloseScene() error = %v", err)
			}
		}},
		{"the scene opens to everyone", func(t *testing.T, at *audienceTable) {
			if _, err := at.setAudience(at.master, &playv1.SceneAudience{Everyone: true}); err != nil {
				t.Fatalf("SetSceneAudience(everyone) error = %v", err)
			}
		}},
		{"another scene opens to other players", func(t *testing.T, at *audienceTable) {
			other, _ := at.h.newScene(at.mapID, "A ponte", false, 1)
			at.openFor(t, other, at.bia)
		}},
		{"another scene opens to everyone", func(t *testing.T, at *audienceTable) {
			other, _ := at.h.newScene(at.mapID, "A ponte", false, 1)
			at.openScene(t, other)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			at := newAudienceTable(t)
			a := at.image(t)
			at.openFor(t, at.point, at.ana, at.caio)
			at.mustShow(t, a, false, nil)
			at.hints(t)
			tc.stop(t, at)
			// Nothing goes to anyone else: Bia gets no image, now or later, and Ana and Caio keep it.
			at.wantSeen(t, tc.name, a, true, true, false)
			got := at.live(t, at.master).GetAudience()
			if got.GetKind() != kindChosen || !slices.Equal(audienceIDs(got), sortedIDs(at.ana, at.caio)) {
				t.Errorf("the master reads the audience %v, want chosen players: Ana and Caio", got)
			}
			for _, w := range at.hints(t)[1:] {
				if strings.Contains(w, "image:") {
					t.Errorf("a player was sent the image when the scene stopped being its own: %q", w)
				}
			}
			// And it does not follow another scene's players any more.
			if _, err := at.master.play.OpenScene(ctx, connect.NewRequest(&playv1.OpenSceneRequest{CampaignId: at.campaignID, PointId: at.point, Audience: audienceOf(at.bia)})); err != nil {
				t.Fatalf("OpenScene() error = %v", err)
			}
			at.wantSeen(t, tc.name+", then a scene of Bia", a, true, true, false)
		})
	}
}

func TestImageAudience_AnImageLeftWithThePlayersKeepsTheAudienceItHad(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	a, b, c := at.image(t), at.image(t), at.image(t)
	at.openFor(t, at.point, at.ana, at.caio)
	at.mustShow(t, a, true, nil) // kept, for the scene's players
	at.hints(t)

	// Another image replaces it: A is left with Ana and Caio only.
	at.mustShow(t, b, false, imageAudience(kindTable))
	if got := at.leftFor(t, at.ana); !slices.Equal(got, []string{a}) {
		t.Errorf("Ana's left images = %v, want [A]", got)
	}
	if got := at.leftFor(t, at.caio); !slices.Equal(got, []string{a}) {
		t.Errorf("Caio's left images = %v, want [A]", got)
	}
	if got := at.leftFor(t, at.bia); len(got) != 0 {
		t.Errorf("Bia's left images = %v, want none", got)
	}
	// The master reads every left image, each with its audience.
	res, err := at.master.play.ListLeftImages(t.Context(), connect.NewRequest(&playv1.ListLeftImagesRequest{CampaignId: at.campaignID}))
	if err != nil || len(res.Msg.GetImages()) != 1 {
		t.Fatalf("ListLeftImages (master) = %v, %v; want one image", res, err)
	}
	if got := res.Msg.GetImages()[0].GetAudience(); got.GetKind() != kindChosen || !slices.Equal(audienceIDs(got), sortedIDs(at.ana, at.caio)) {
		t.Errorf("the left image's audience = %v, want chosen players: Ana and Caio", got)
	}
	// The master and the two players hear the list changed; Bia does not. (The shown image went to everyone.)
	words := at.hints(t)
	wantWords(t, "the image left", words, "image:"+b+",left_images_changed", "image:"+b+",left_images_changed", "image:"+b+",left_images_changed", "image:"+b)

	// Taking it back: the same ones hear.
	if _, err := at.master.play.TakeBackLeftImage(t.Context(), connect.NewRequest(&playv1.TakeBackLeftImageRequest{CampaignId: at.campaignID, ImageId: a})); err != nil {
		t.Fatalf("TakeBackLeftImage() error = %v", err)
	}
	wantWords(t, "taken back", at.hints(t), "left_images_changed", "left_images_changed", "left_images_changed", "")

	// An image kept for the whole table is left for the whole table, and the session's end leaves it with the same players.
	at.mustShow(t, c, true, imageAudience(kindChosen, at.bia))
	at.mustShow(t, "", false, nil)
	if got := at.leftFor(t, at.bia); !slices.Equal(got, []string{c}) {
		t.Errorf("Bia's left images = %v, want [C]", got)
	}
	if got := at.leftFor(t, at.ana); len(got) != 0 {
		t.Errorf("Ana's left images = %v, want none: C was Bia's", got)
	}
	d := at.image(t)
	at.mustShow(t, d, true, imageAudience(kindChosen, at.caio))
	var session string
	if err := at.h.pool.QueryRow(t.Context(), `SELECT id FROM game_sessions WHERE campaign_id = $1`, at.campaignID).Scan(&session); err != nil {
		t.Fatalf("find the session: %v", err)
	}
	if _, err := at.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: at.campaignID, GameSessionId: session})); err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	if got := at.leftFor(t, at.caio); !slices.Equal(got, []string{d}) {
		t.Errorf("Caio's left images after the session = %v, want [D]", got)
	}
	if got := at.leftFor(t, at.ana); len(got) != 0 {
		t.Errorf("Ana's left images after the session = %v, want none", got)
	}
}

func TestImageAudience_AWholeTableImageIsLeftWithTheWholeTable(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	a := at.image(t)
	at.mustShow(t, a, true, nil)
	at.mustShow(t, "", false, nil)
	for _, u := range []*user{at.ana, at.caio, at.bia} {
		if got := at.leftFor(t, u); !slices.Equal(got, []string{a}) {
			t.Errorf("a player's left images = %v, want [A]", got)
		}
	}
	res, err := at.master.play.ListLeftImages(t.Context(), connect.NewRequest(&playv1.ListLeftImagesRequest{CampaignId: at.campaignID}))
	if err != nil || res.Msg.GetImages()[0].GetAudience().GetKind() != kindTable {
		t.Errorf("the master reads the left image %v, %v; want the whole table", res, err)
	}
}
