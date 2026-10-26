package leaktest

import (
	"bytes"
	"testing"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	notesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1/notesv1connect"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1/playv1connect"
)

// MR-015, MR-028, MR-029, RN-10: the master's answers on scene visibility. A clue revealed to a player outside
// a scene open to only some players is allowed and reaches their notes, and nothing else of the scene does; an
// image left with the players again keeps the widest audience; only the players of the scene tag notes with it.

func (w *world) notesOf(p *person) reply {
	return p.call(notesv1connect.NotesServiceListNotesProcedure, &notesv1.ListNotesRequest{CampaignId: w.campaign})
}

func (w *world) noteScenesOf(p *person) reply {
	return p.call(notesv1connect.NotesServiceListNoteScenesProcedure, &notesv1.ListNoteScenesRequest{CampaignId: w.campaign})
}

func TestRN10_AClueRevealedToAnOutsiderReachesTheirNotesAndNothingElse(t *testing.T) {
	w := newWorld(t)
	ps := w.buildPartialScene()
	master, ana, caio := w.master, w.ana, w.caio
	clueText := w.secrets.marker("clue-to-outsider", caio)
	clue := w.addClue(ps.point, clueText)
	w.openSceneTo(ps.point.GetId(), ana) // Ana is in the scene, Caio is out
	must(master.play.PutOnStage(t.Context(), rq(&playv1.PutOnStageRequest{CampaignId: w.campaign, CharacterId: ps.npc})))
	must(master.maps.RevealSceneClue(t.Context(), rq(&mapsv1.RevealSceneClueRequest{CampaignId: w.campaign, ClueId: clue, CharacterIds: []string{w.toren.GetId()}})))
	needle := w.needlesOf("clue-to-outsider")[0]

	// The positive control for the clue: Caio, outside the scene, has it in his notes; Ana, who was not given it, does not.
	if !bytes.Contains(w.notesOf(caio).body, []byte(needle)) {
		t.Fatalf("Caio's notes lack the clue he was given: %s", shorten(w.notesOf(caio).body))
	}
	if bytes.Contains(w.notesOf(ana).body, []byte(needle)) {
		t.Error("Ana reads a clue that was revealed to Caio")
	}
	// The negative control for the rest: nothing else of the scene reaches him, in his notes or anywhere he reads.
	for what, r := range map[string]reply{
		"his notes":        w.notesOf(caio),
		"the note scenes":  w.noteScenesOf(caio),
		"the open scene":   w.openSceneAs(caio),
		"the live session": caio.call(playv1connect.PlayServiceGetLiveSessionProcedure, &playv1.GetLiveSessionRequest{CampaignId: w.campaign}),
	} {
		if n, found := holdsAny(r.body, ps.needles); found {
			t.Errorf("Caio reads %q of the scene he is not in, in %s: %s", n, what, shorten(r.body))
		}
	}
}

func TestRN10_OnlyThePlayersOfAPartialSceneTagNotesWithIt(t *testing.T) {
	w := newWorld(t)
	ps := w.buildPartialScene()
	ana, caio := w.ana, w.caio
	w.openSceneTo(ps.point.GetId(), ana)
	name, id := w.needlesOf("partial-scene-name")[0], ps.point.GetId()

	// The positive control: Ana picks the scene and tags a note with it; the name is in her notes.
	if r := w.noteScenesOf(ana); !bytes.Contains(r.body, []byte(name)) || !bytes.Contains(r.body, []byte(id)) {
		t.Fatalf("Ana's note scenes lack the scene she is in: %s", shorten(r.body))
	}
	must(ana.notes.CreateNote(t.Context(), rq(&notesv1.CreateNoteRequest{CampaignId: w.campaign, Text: w.secrets.marker("note-partial", ana), ScenePointId: id})))
	if !bytes.Contains(w.notesOf(ana).body, []byte(name)) {
		t.Error("the scene's name is not in Ana's tagged note")
	}

	// Caio, outside: the scene is in none of his lists and he cannot tag with it.
	for what, r := range map[string]reply{"the note scenes": w.noteScenesOf(caio), "his notes": w.notesOf(caio)} {
		if n, found := holdsAny(r.body, ps.needles); found {
			t.Errorf("Caio reads %q in %s", n, what)
		}
	}
	refused := caio.call(notesv1connect.NotesServiceCreateNoteProcedure, &notesv1.CreateNoteRequest{CampaignId: w.campaign, Text: "nota", ScenePointId: id})
	if refused.ok() {
		t.Fatalf("Caio tagged a note with a scene he is not in: %s", refused.body)
	}
	if n, found := holdsAny(refused.body, ps.needles); found {
		t.Errorf("the refusal Caio got holds %q: %s", n, refused.body)
	}
	other := caio.call(notesv1connect.NotesServiceCreateNoteProcedure, &notesv1.CreateNoteRequest{CampaignId: w.campaign, Text: "nota", ScenePointId: newKey()})
	if other.status != refused.status || !bytes.Equal(other.body, refused.body) {
		t.Errorf("tagging the scene = HTTP %d %s, tagging one that does not exist = HTTP %d %s; want the same refusal", refused.status, refused.body, other.status, other.body)
	}
}

func TestRN10_AnImageLeftAgainKeepsTheWidestAudience(t *testing.T) {
	w := newWorld(t)
	ps := w.buildPartialScene()
	w.openSceneTo(ps.point.GetId(), w.ana, w.caio)
	left := w.newShown("left-again", w.ana, w.caio)
	leave := func(aud *playv1.ImageAudience) {
		t.Helper()
		w.showImage(left, true, aud)
		must(w.master.play.SetShownImage(t.Context(), rq(&playv1.SetShownImageRequest{CampaignId: w.campaign}))) // none: the image is left
	}
	listed := func(p *person) reply {
		return p.call(playv1connect.PlayServiceListLeftImagesProcedure, &playv1.ListLeftImagesRequest{CampaignId: w.campaign})
	}
	holds := func(p *person) (read bool, route int) {
		_, read = holdsAny(listed(p).body, left.needles)
		route, _ = p.get("/images/" + left.id)
		return read, route
	}

	// Left to Ana, then left again to Caio: both have it, in the list and on the image route.
	leave(chosen(w.ana))
	if read, route := holds(w.caio); read || route == 200 {
		t.Fatalf("Caio has the image left to Ana (read %v, route %d)", read, route)
	}
	leave(chosen(w.caio))
	for _, p := range []*person{w.master, w.ana, w.caio} {
		if read, route := holds(p); !read || route != 200 {
			t.Errorf("%s lacks the image left to Ana and then to Caio (read %v, route %d)", p.name, read, route)
		}
	}
	// A third player never gets it: not the pending member, the stranger, the signed-out caller.
	for _, p := range []*person{w.pending, w.stranger, w.anonymous()} {
		if read, route := holds(p); read || route == 200 {
			t.Errorf("%s has the image left to Ana and Caio (read %v, route %d)", p.name, read, route)
		}
	}
	// Left to Ana alone once more: Caio keeps it (the widest stays).
	leave(chosen(w.ana))
	if read, route := holds(w.caio); !read || route != 200 {
		t.Errorf("Caio lost the image when it was left to Ana again (read %v, route %d)", read, route)
	}
}
