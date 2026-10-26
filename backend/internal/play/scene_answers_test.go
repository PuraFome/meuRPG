package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The master's answers on scene visibility (MR-015, MR-028, MR-031, RN-10): an image left again keeps the
// widest audience, a hidden NPC speaks in the conversation by name and stays hidden on the stage, the
// player of a dead character follows a scene like anyone, and a roll stays in the summary of the players
// who saw it when its action is hidden afterwards.

// leaveWith shows image img (kept) to the audience, then replaces it with a whole-table image, which
// leaves img with the audience it had.
func (at *sceneTable) leaveWith(t *testing.T, img string, aud *playv1.ImageAudience) {
	t.Helper()
	at.mustShow(t, img, true, aud)
	at.mustShow(t, at.image(t), false, imageAudience(kindTable))
}

func TestImageAudience_LeavingAnImageAgainKeepsTheWidestAudience(t *testing.T) {
	t.Parallel()
	at := newSceneTable(t)
	a := at.image(t)

	// Left to Ana, then left again to Caio: both have it, Bia never.
	at.leaveWith(t, a, imageAudience(kindChosen, at.ana))
	if got := at.leftFor(t, at.ana); !slices.Equal(got, []string{a}) {
		t.Fatalf("Ana's left images = %v, want [A]", got)
	}
	at.hints(t)
	at.leaveWith(t, a, imageAudience(kindChosen, at.caio))
	for u, want := range map[*user]bool{at.ana: true, at.caio: true, at.bia: false} {
		got := at.leftFor(t, u)
		if has := slices.Contains(got, a); has != want {
			t.Errorf("after leaving A again to Caio, a player's left images = %v, A wanted: %v", got, want)
		}
	}
	res, err := at.master.play.ListLeftImages(t.Context(), connect.NewRequest(&playv1.ListLeftImagesRequest{CampaignId: at.campaignID}))
	if err != nil {
		t.Fatalf("ListLeftImages() error = %v", err)
	}
	for _, im := range res.Msg.GetImages() {
		if im.GetId() != a {
			continue
		}
		if got := im.GetAudience(); got.GetKind() != kindChosen || !slices.Equal(audienceIDs(got), sortedIDs(at.ana, at.caio)) {
			t.Errorf("the master reads the left image's audience %v, want Ana and Caio", got)
		}
	}

	// Left once more to the whole table, then to Ana alone: the widest stays, so everyone has it.
	at.leaveWith(t, a, imageAudience(kindTable))
	at.leaveWith(t, a, imageAudience(kindChosen, at.ana))
	for _, u := range []*user{at.ana, at.caio, at.bia} {
		if got := at.leftFor(t, u); !slices.Contains(got, a) {
			t.Errorf("after leaving A to the table and then to Ana, a player's left images = %v, want A in it", got)
		}
	}

	// Taking it back is the only way to narrow it: left again, it is Ana's alone.
	if _, err := at.master.play.TakeBackLeftImage(t.Context(), connect.NewRequest(&playv1.TakeBackLeftImageRequest{CampaignId: at.campaignID, ImageId: a})); err != nil {
		t.Fatalf("TakeBackLeftImage() error = %v", err)
	}
	at.leaveWith(t, a, imageAudience(kindChosen, at.ana))
	if got := at.leftFor(t, at.caio); slices.Contains(got, a) {
		t.Errorf("Caio's left images = %v after the image was taken back and left to Ana, want it gone", got)
	}
}

func TestSceneAudience_ThePlayerOfADeadCharacterFollowsTheSceneLikeAnyone(t *testing.T) {
	t.Parallel()
	at := newSceneTable(t)
	// Toren dies: Caio is still a player of the table, with no living character.
	if _, err := at.h.pool.Exec(t.Context(), `UPDATE characters SET status = 'dead', died_at = now() WHERE id = $1`, at.toren.GetId()); err != nil {
		t.Fatalf("kill Toren: %v", err)
	}
	at.openFor(t, at.point, at.ana, at.caio)
	for _, u := range []*user{at.ana, at.caio} {
		if got := at.readScene(t, u); got.GetPointId() != at.point {
			t.Errorf("a player named in the scene reads %v, want the scene", got)
		}
	}
	if got := at.readScene(t, at.bia); got != nil {
		t.Errorf("Bia (not named) reads %v, want no scene", got)
	}
	// Taken out and brought in again by SetSceneAudience, the same.
	at.mustSetAudience(t, at.ana)
	if got := at.readScene(t, at.caio); got != nil {
		t.Errorf("Caio, taken out, reads %v, want no scene", got)
	}
	at.mustSetAudience(t, at.caio)
	if got := at.readScene(t, at.caio); got.GetPointId() != at.point {
		t.Errorf("Caio, brought in again, reads %v, want the scene", got)
	}
	// An image chosen for him is his too.
	img := at.image(t)
	at.mustShow(t, img, false, imageAudience(kindChosen, at.caio))
	at.wantSeen(t, "an image chosen for the player of a dead character", img, false, true, false)
	// Rolling still needs a living character.
	at.h.roller.queue(15)
	_, err := at.caio.play.RollSceneCheck(t.Context(), connect.NewRequest(&playv1.RollSceneCheckRequest{
		CampaignId: at.campaignID, ActionId: at.action, IdempotencyKey: newKey(), Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true},
	}))
	if err == nil {
		t.Error("the player of a dead character rolled an action of the scene, want it refused")
	}
	// Someone who is not a player of the campaign is still refused.
	stranger := at.h.newUser("Estranha")
	_, err = at.setAudience(at.master, audienceOf(stranger))
	wantRefusal(t, "a stranger in the audience", err, connect.CodeInvalidArgument, "the audience must name players of the campaign")
	_, err = at.setAudience(at.master, &playv1.SceneAudience{UserIds: []string{at.master.id}})
	wantRefusal(t, "the master in the audience", err, connect.CodeInvalidArgument, "the audience must name players of the campaign")
}

// portraitNPC makes an NPC with a portrait.
func (at *sceneTable) portraitNPC(t *testing.T, name, portrait string) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Basic{Basic: &charactersv1.BasicSheet{HitPointsMax: 9, ArmorClass: 11, SpeedFt: 30, PortraitImageId: portrait}}}
	res, err := at.master.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: at.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_MINION, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

// lines are the lines of the conversation the user reads.
func (at *sceneTable) lines(t *testing.T, u *user) []*playv1.SessionMessage {
	t.Helper()
	res, err := u.play.ListSessionMessages(t.Context(), connect.NewRequest(&playv1.ListSessionMessagesRequest{CampaignId: at.campaignID}))
	if err != nil {
		t.Fatalf("ListSessionMessages() error = %v", err)
	}
	return res.Msg.GetMessages()
}

// stageNames are the names on the stage the user reads.
func stageNames(info *playv1.OpenSceneInfo) []string {
	var out []string
	for _, n := range info.GetStage() {
		out = append(out, n.GetName())
	}
	return out
}

// A hidden NPC speaks in the conversation (MR-051, MR-031, RN-10): the line names it for its readers, and
// nothing else of the NPC reaches them: not its portrait, not the stage. Speaking in the conversation does
// not reveal it on the stage; only "Dar a fala" does.
func TestConversation_AHiddenStageNPCSpeaksByNameAndStaysHiddenOnTheStage(t *testing.T) {
	t.Parallel()
	at := newSceneTable(t)
	at.openScene(t, at.point)
	const name = "Zqvxorth-Kwjl-7731"
	portrait := at.image(t)
	npc := at.portraitNPC(t, name, portrait)
	if _, err := at.master.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: at.campaignID, CharacterId: npc.GetId(), Hidden: true})); err != nil {
		t.Fatalf("PutOnStage(hidden) error = %v", err)
	}
	at.hints(t)

	at.mustSay(at.master, asNPC, "Quem vem lá?", speaker(npc.GetId()), to(at.ana.id))

	ana := at.lines(t, at.ana)
	if len(ana) != 1 || ana[0].GetSpeakerName() != name {
		t.Fatalf("Ana reads the lines %v, want one naming %q", ana, name)
	}
	if ana[0].GetSpeakerPortrait() != nil {
		t.Errorf("Ana's line carries the hidden NPC's portrait %v", ana[0].GetSpeakerPortrait())
	}
	if got := at.lines(t, at.caio); len(got) != 0 {
		t.Errorf("Caio (not a reader) reads %v, want nothing", got)
	}
	if ok, err := at.h.svc.ImageInConversation(t.Context(), at.campaignID, at.ana.id, portrait); err != nil || ok {
		t.Errorf("Ana may open the hidden NPC's portrait through the line = %v, %v; want no", ok, err)
	}

	// The stage is unchanged: the master has the NPC marked hidden, no player has it, and nobody was told.
	for _, u := range []*user{at.ana, at.caio, at.bia} {
		if got := stageNames(at.readScene(t, u)); len(got) != 0 {
			t.Errorf("a player reads the stage %v, want it without the hidden NPC", got)
		}
	}
	if stage := at.readScene(t, at.master).GetStage(); len(stage) != 1 || !stage[0].GetHidden() || stage[0].GetName() != name {
		t.Errorf("the master reads the stage %v, want the NPC, hidden", stage)
	}
	for i, w := range at.hints(t) {
		if w == "stage_changed" || w == "scene_changed" {
			t.Errorf("stream %d got %q for a line of the conversation", i, w)
		}
	}

	// "Dar a fala" is what shows it.
	if _, err := at.master.play.SetSpeaker(t.Context(), connect.NewRequest(&playv1.SetSpeakerRequest{CampaignId: at.campaignID, CharacterId: npc.GetId()})); err != nil {
		t.Fatalf("SetSpeaker() error = %v", err)
	}
	if got := stageNames(at.readScene(t, at.caio)); !slices.Equal(got, []string{name}) {
		t.Errorf("after the floor is given, Caio reads the stage %v, want the NPC", got)
	}
}

// A roll made in a scene stays in the summary of the players who saw it or rolled it when the master hides
// its action afterwards: it happened. A player who was not in the scene never counts it (RN-10).
func TestSummary_ARollOnAnActionHiddenAfterwardsStaysForThoseWhoSawIt(t *testing.T) {
	t.Parallel()
	at := newSceneTable(t)
	at.openFor(t, at.point, at.ana, at.bia)
	at.checkRoll(t, at.ana, at.action, 18) // DC 12: passed, in a scene that shows its DC
	if _, err := at.h.pool.Exec(t.Context(), `UPDATE scene_actions SET hidden = true WHERE id = $1`, at.action); err != nil {
		t.Fatalf("hide the action: %v", err)
	}
	var session string
	if err := at.h.pool.QueryRow(t.Context(), `SELECT id FROM game_sessions WHERE campaign_id = $1`, at.campaignID).Scan(&session); err != nil {
		t.Fatalf("find the session: %v", err)
	}
	if _, err := at.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: at.campaignID, GameSessionId: session})); err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	for u, want := range map[*user]int32{at.master: 1, at.ana: 1, at.bia: 1, at.caio: 0} {
		sum, err := at.summary(t, u, session)
		if err != nil {
			t.Fatalf("GetSessionSummary() error = %v", err)
		}
		var got int32
		for _, c := range sum.GetCategories() {
			if c.GetKind() == playv1.HighlightKind_HIGHLIGHT_KIND_CHECKS_PASSED {
				got = c.GetValue()
			}
		}
		if got != want {
			t.Errorf("checks passed in the summary = %d, want %d", got, want)
		}
	}
}
