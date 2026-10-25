package maps

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	notesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's answers on scene visibility (MR-015, MR-028, MR-029, MR-030, RN-10): the notes of a scene
// open to only some players, a clue revealed to a player outside it, and the deletion of a shown image.

const partialSceneName = "A carroça tombada"

// openTo opens the scene point to exactly these players.
func (u *user) openTo(campaignID, pointID string, players ...*user) {
	u.h.t.Helper()
	aud := &playv1.SceneAudience{}
	for _, p := range players {
		aud.UserIds = append(aud.UserIds, p.id)
	}
	if _, err := u.play.OpenScene(u.h.t.Context(), connect.NewRequest(&playv1.OpenSceneRequest{CampaignId: campaignID, PointId: pointID, Audience: aud})); err != nil {
		u.h.t.Fatalf("OpenScene(%d players) error = %v", len(players), err)
	}
}

func (u *user) setSceneAudience(campaignID string, aud *playv1.SceneAudience) {
	u.h.t.Helper()
	if _, err := u.play.SetSceneAudience(u.h.t.Context(), connect.NewRequest(&playv1.SetSceneAudienceRequest{CampaignId: campaignID, Audience: aud})); err != nil {
		u.h.t.Fatalf("SetSceneAudience() error = %v", err)
	}
}

// readsSceneName says whether anything the player's notes screen reads holds the scene's name.
func readsSceneName(u *user, campaignID string) bool {
	all := asJSON(u.h.t, u.listNotes(campaignID, "")) + asJSON(u.h.t, &notesv1.ListNoteScenesResponse{Scenes: u.noteScenes(campaignID)})
	return strings.Contains(all, partialSceneName)
}

// Only the players of a scene open to only some players may tag notes with it: for anyone else it is a scene
// the group has not discovered, and its name is nowhere in their notes. When the scene opens to everyone the
// group discovers it, as before.
func TestMR030_OnlyThePlayersOfAPartialSceneTagNotesWithIt(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	s.master.openTo(s.campaign, pid, s.ana)

	if got := s.ana.noteScenes(s.campaign); len(got) != 1 || got[0].GetId() != pid || got[0].GetName() != partialSceneName {
		t.Errorf("Ana's tag picker = %v, want the scene she is in", got)
	}
	note := s.ana.createNote(s.campaign, "A carroça", pid)
	if note.GetScenePointId() != pid || note.GetSceneName() != partialSceneName {
		t.Errorf("Ana's tagged note = %v, want the scene's tag", note)
	}
	if got := noteTexts(s.ana.listNotes(s.campaign, pid)); len(got) != 1 {
		t.Errorf("Ana's notes of the scene = %v, want her note", got)
	}

	// Caio is outside: the scene is undiscovered for him, refused as one that does not exist, and its name is in nothing he reads.
	if got := s.caio.noteScenes(s.campaign); len(got) != 0 {
		t.Errorf("Caio's tag picker = %v, want none", got)
	}
	_, errOut := s.caio.tryCreateNote(s.campaign, "nota", pid)
	_, errNone := s.caio.tryCreateNote(s.campaign, "nota", newKey())
	if connect.CodeOf(errOut) != connect.CodeInvalidArgument || errOut.Error() != errNone.Error() {
		t.Errorf("Caio tagging the scene: %v, want the refusal of a scene that does not exist: %v", errOut, errNone)
	}
	if got := s.caio.listNotes(s.campaign, pid); len(got.GetNotes()) != 0 {
		t.Errorf("Caio filters by the scene and reads %v, want nothing", got.GetNotes())
	}
	if readsSceneName(s.caio, s.campaign) {
		t.Error("the scene's name is in what Caio's notes screen reads")
	}
	if !readsSceneName(s.ana, s.campaign) {
		t.Error("the scene's name is not in what Ana's notes screen reads: the positive control is missing")
	}

	// Open to everyone, the group discovers it: Caio may tag it now.
	s.master.setSceneAudience(s.campaign, &playv1.SceneAudience{Everyone: true})
	if got := s.caio.noteScenes(s.campaign); len(got) != 1 || got[0].GetId() != pid {
		t.Errorf("Caio's tag picker = %v after the scene opened to everyone, want it", got)
	}
	s.caio.createNote(s.campaign, "A carroça, do Caio", pid)
}

// A player the master brings into a scene, or who joins it, tags notes with it from then on, and keeps
// that after leaving it: what a player read, they read. A player who was never in it never does.
func TestMR030_APlayerBroughtInOrJoinedTagsNotesWithTheScene(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	s.master.openTo(s.campaign, pid, s.ana)
	if _, err := s.ana.tryCreateNote(s.campaign, "nota", pid); err != nil {
		t.Fatalf("Ana tagging the scene error = %v", err)
	}

	// Brought in by the master.
	s.master.setSceneAudience(s.campaign, &playv1.SceneAudience{UserIds: []string{s.ana.id, s.caio.id}})
	if _, err := s.caio.tryCreateNote(s.campaign, "nota do Caio", pid); err != nil {
		t.Errorf("Caio, brought into the scene, tagging it error = %v", err)
	}
	// Taken out again, he still has what he read, and Ana is not touched.
	s.master.setSceneAudience(s.campaign, &playv1.SceneAudience{UserIds: []string{s.ana.id}})
	if got := s.caio.noteScenes(s.campaign); len(got) != 1 {
		t.Errorf("Caio's tag picker = %v after he was taken out, want the scene he read", got)
	}

	// A second scene, with the switch on: a player who joins it may tag it, one who does not join may not.
	second := s.master.createPoint(&mapsv1.CreateMapPointRequest{CampaignId: s.campaign, MapId: s.mapID, Kind: mapsv1.MapPointKind_MAP_POINT_KIND_SCENE, Name: "A ponte"})
	s.master.openTo(s.campaign, second.GetId(), s.ana)
	if _, err := s.master.play.SetScenePlayersMayJoin(t.Context(), connect.NewRequest(&playv1.SetScenePlayersMayJoinRequest{CampaignId: s.campaign, PlayersMayJoin: true})); err != nil {
		t.Fatalf("SetScenePlayersMayJoin() error = %v", err)
	}
	if _, err := s.caio.tryCreateNote(s.campaign, "nota", second.GetId()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("Caio tagging a scene he only could join error = %v, want invalid_argument", err)
	}
	if _, err := s.caio.play.JoinScene(t.Context(), connect.NewRequest(&playv1.JoinSceneRequest{CampaignId: s.campaign})); err != nil {
		t.Fatalf("JoinScene() error = %v", err)
	}
	if _, err := s.caio.tryCreateNote(s.campaign, "nota da ponte", second.GetId()); err != nil {
		t.Errorf("Caio, who joined the scene, tagging it error = %v", err)
	}
}

// A clue revealed to a player outside the scene is allowed: it reaches their notes, and nothing else of the
// scene does (RN-10): not the scene, not its name, not a tag on the clue.
func TestMR029_AClueRevealedToAnOutsiderReachesTheirNotesAndNothingElse(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	pid := s.point.GetId()
	const clueText = "PISTA-DO-FORA-5521"
	clue := s.master.addClue(s.campaign, s.mapID, pid, clueText)
	s.master.openTo(s.campaign, pid, s.ana)
	s.master.mustRevealClue(s.campaign, clue.GetId(), s.other.GetId()) // Toren, Caio's character: Caio is outside

	notes := s.caio.listNotes(s.campaign, "")
	var got *notesv1.Note
	for _, n := range notes.GetNotes() {
		if n.GetKind() == notesv1.NoteKind_NOTE_KIND_CLUE && n.GetText() == clueText {
			got = n
		}
	}
	if got == nil {
		t.Fatalf("Caio's notes = %v, want the clue he was given", notes.GetNotes())
	}
	if got.GetScenePointId() != "" || got.GetSceneName() != "" {
		t.Errorf("the clue carries the scene %q/%q to an outsider, want no tag", got.GetScenePointId(), got.GetSceneName())
	}
	if readsSceneName(s.caio, s.campaign) {
		t.Error("the scene's name is in what Caio's notes screen reads")
	}
	if scene := s.caio.getScene(s.campaign); scene != nil {
		t.Errorf("Caio reads the scene %v, want none", scene)
	}
	if strings.Contains(asJSON(t, s.caio.mustGetMap(s.campaign, s.mapID)), clueText) {
		t.Error("the clue is in the map Caio reads")
	}
}

// The deletion of the shown image tells only the players who saw it (MR-028, RN-10): the master and the
// scene's players, not the player outside the scene.
func TestDeleteGalleryImage_OfAnImageShownToSomePlayersTellsOnlyThem(t *testing.T) {
	t.Parallel()
	s := newScenes(t, true)
	s.master.openTo(s.campaign, s.point.GetId(), s.ana)
	probe := s.master.createMap(s.campaign, "Sonda", s.master.newImage(s.campaign)).GetId()
	show := func(image string, keep bool) {
		t.Helper()
		if _, err := s.master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{CampaignId: s.campaign, ImageId: image, Keep: keep})); err != nil {
			t.Fatalf("SetShownImage() error = %v", err)
		}
	}
	del := func(image string) {
		t.Helper()
		if _, err := s.master.gallery.DeleteGalleryImage(t.Context(), connect.NewRequest(&mapsv1.DeleteGalleryImageRequest{CampaignId: s.campaign, ImageId: image})); err != nil {
			t.Fatalf("DeleteGalleryImage() error = %v", err)
		}
	}
	heard := func(what string, want [3]string, do func()) {
		t.Helper()
		mw, aw, cw := s.master.watch(s.campaign), s.ana.watch(s.campaign), s.caio.watch(s.campaign)
		do()
		s.master.setMapRevealed(s.campaign, probe, true)
		got := [3]string{imageEvents(mw, probe), imageEvents(aw, probe), imageEvents(cw, probe)}
		s.master.setMapRevealed(s.campaign, probe, false)
		if got != want {
			t.Errorf("%s: the master, Ana and Caio heard %q, want %q", what, got, want)
		}
	}

	shown := s.master.newImage(s.campaign)
	show(shown, false)
	heard("the shown image of the scene's players", [3]string{"shown_image_changed", "shown_image_changed", ""}, func() { del(shown) })

	// An image kept: shown to the scene's players, then replaced, it is left with them; deleting it tells them.
	kept := s.master.newImage(s.campaign)
	show(kept, true)
	show(s.master.newImage(s.campaign), false)
	heard("the image left with the scene's players", [3]string{"left_images_changed", "left_images_changed", ""}, func() { del(kept) })

	// Shown to the whole table, everyone hears it.
	table := s.master.newImage(s.campaign)
	if _, err := s.master.play.SetShownImage(t.Context(), connect.NewRequest(&playv1.SetShownImageRequest{
		CampaignId: s.campaign, ImageId: table, Audience: &playv1.ImageAudience{Kind: playv1.ImageAudienceKind_IMAGE_AUDIENCE_KIND_WHOLE_TABLE},
	})); err != nil {
		t.Fatalf("SetShownImage(table) error = %v", err)
	}
	heard("the shown image of the whole table", [3]string{"shown_image_changed", "shown_image_changed", "shown_image_changed"}, func() { del(table) })
}
