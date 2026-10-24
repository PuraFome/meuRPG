package play

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A scene open to only some players (MR-015, RN-10): OpenScene takes the players in the scene, the
// master changes who is in with SetSceneAudience, and a player outside it gets no scene at all,
// exactly as when none is open.

// userIDs are the accounts of the users.
func userIDs(us ...*user) []string {
	out := make([]string, 0, len(us))
	for _, u := range us {
		out = append(out, u.id)
	}
	return out
}

func audienceOf(us ...*user) *playv1.SceneAudience {
	return &playv1.SceneAudience{UserIds: userIDs(us...)}
}

// openFor opens the scene to the users, as the master.
func (a *armed) openFor(t *testing.T, point string, us ...*user) *playv1.OpenSceneInfo {
	t.Helper()
	res, err := a.master.play.OpenScene(t.Context(), connect.NewRequest(&playv1.OpenSceneRequest{CampaignId: a.campaignID, PointId: point, Audience: audienceOf(us...)}))
	if err != nil {
		t.Fatalf("OpenScene(%v) error = %v", userIDs(us...), err)
	}
	return res.Msg.GetScene()
}

func (a *armed) setAudience(u *user, audience *playv1.SceneAudience) (*playv1.SetSceneAudienceResponse, error) {
	res, err := u.play.SetSceneAudience(a.h.t.Context(), connect.NewRequest(&playv1.SetSceneAudienceRequest{CampaignId: a.campaignID, Audience: audience}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustSetAudience(t *testing.T, us ...*user) *playv1.OpenSceneInfo {
	t.Helper()
	res, err := a.setAudience(a.master, audienceOf(us...))
	if err != nil {
		t.Fatalf("SetSceneAudience(%v) error = %v", userIDs(us...), err)
	}
	return res.GetScene()
}

func (a *armed) readScene(t *testing.T, u *user) *playv1.OpenSceneInfo {
	t.Helper()
	res, err := u.play.GetOpenScene(t.Context(), connect.NewRequest(&playv1.GetOpenSceneRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("GetOpenScene() error = %v", err)
	}
	return res.Msg.GetScene()
}

// hintWords reads what the streams got up to a marker, as short words, one string per stream.
func hintWords(t *testing.T, a *armed, ws ...*watcher) []string {
	t.Helper()
	a.master.markCurrentMap(t, a.campaignID, a.mapID, ws...)
	out := make([]string, len(ws))
	for i, w := range ws {
		var words []string
		for _, ev := range w.beforeMarker(t) {
			switch {
			case ev.GetSceneChanged() != nil:
				words = append(words, "scene_changed")
			case ev.GetStageChanged() != nil:
				words = append(words, "stage_changed")
			case ev.GetGroupCheckChanged() != nil:
				words = append(words, "group_check_changed")
			case ev.GetLeftImagesChanged() != nil:
				words = append(words, "left_images_changed")
			case ev.GetShownImageChanged() != nil:
				id := ev.GetShownImageChanged().GetImage().GetId()
				if id == "" {
					id = "none"
				}
				words = append(words, "image:"+id)
			case ev.GetSceneCheckRolled() != nil:
				words = append(words, "scene_check_rolled")
			default:
				words = append(words, "other")
			}
		}
		out[i] = strings.Join(words, ",")
	}
	return out
}

func wantWords(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s: streams got %q, want %q (master, then the players in the order asked)", what, got, want)
	}
}

// scenePlayerNames lists the characters' names of a master's view of who is in or out, sorted.
func scenePlayerNames(list []*playv1.ScenePlayer) []string {
	var out []string
	for _, p := range list {
		out = append(out, p.GetCharacterName())
	}
	slices.Sort(out)
	return out
}

// audienceTable is a table with the master's, Ana's, Caio's and Bia's streams open, and a scene with one action.
type audienceTable struct {
	*armed
	point, action  string
	wm, wa, wc, wb *watcher
}

func newAudienceTable(t *testing.T) *audienceTable {
	t.Helper()
	a := newArmed(t)
	pt, actions := a.h.newScene(a.mapID, "A carroça", true, 3, sceneSpec{"skill:investigation", new(int32(12))})
	at := &audienceTable{armed: a, point: pt, action: actions[0]}
	at.wm, at.wa, at.wc, at.wb = a.master.watch(t, a.campaignID), a.ana.watch(t, a.campaignID), a.caio.watch(t, a.campaignID), a.bia.watch(t, a.campaignID)
	for _, w := range []*watcher{at.wm, at.wa, at.wc, at.wb} {
		w.ready(t)
	}
	return at
}

// hints are what the four streams got since the last call, in the order master, Ana, Caio, Bia.
func (at *audienceTable) hints(t *testing.T) []string {
	t.Helper()
	return hintWords(t, at.armed, at.wm, at.wa, at.wc, at.wb)
}

func TestSceneAudience_OpenToSomePlayersHidesItFromTheOthers(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	info := at.openFor(t, at.point, at.ana, at.caio)

	// The master's answer says who is in and who is out, and since when.
	aud := info.GetAudience()
	if aud == nil || aud.GetEveryone() {
		t.Fatalf("the master's audience = %v, want a scene open to chosen players", aud)
	}
	if got := scenePlayerNames(aud.GetInside()); !slices.Equal(got, []string{"Pensantus", "Toren"}) {
		t.Errorf("inside = %v, want Pensantus and Toren", got)
	}
	if got := scenePlayerNames(aud.GetOutside()); !slices.Equal(got, []string{"Brisa"}) {
		t.Errorf("outside = %v, want Brisa", got)
	}
	for _, p := range aud.GetInside() {
		if p.GetUserId() == "" || p.GetCharacterId() == "" {
			t.Errorf("inside player %v lacks the account or the character", p)
		}
	}
	if info.GetOpenedAt() == nil {
		t.Error("the master's scene has no opened_at")
	}
	// The same read by the master later.
	if got := at.readScene(t, at.master).GetAudience(); got == nil || got.GetEveryone() || len(got.GetOutside()) != 1 {
		t.Errorf("GetOpenScene (master) audience = %v, want the same", got)
	}

	// The players in the scene read it, without the audience; the one out reads "no scene", as when none is open.
	for _, u := range []*user{at.ana, at.caio} {
		got := at.readScene(t, u)
		if got.GetPointId() != at.point || got.GetAudience() != nil {
			t.Errorf("a player in the scene reads %v, want the scene and no audience", got)
		}
	}
	if got := at.readScene(t, at.bia); got != nil {
		t.Errorf("Bia (out of the scene) reads %v, want no scene", got)
	}
	// Only the master and the players in the scene hear of it.
	wantWords(t, "opening", at.hints(t), "scene_changed", "scene_changed", "scene_changed", "")
}

func TestSceneAudience_TheDefaultIsEveryone(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openScene(t, at.point)
	aud := at.readScene(t, at.master).GetAudience()
	if aud == nil || !aud.GetEveryone() || len(aud.GetOutside()) != 0 || len(aud.GetInside()) != 3 {
		t.Fatalf("the master's audience = %v, want everyone in and none out", aud)
	}
	for _, u := range []*user{at.ana, at.caio, at.bia} {
		if got := at.readScene(t, u); got.GetPointId() != at.point {
			t.Errorf("a player reads %v, want the scene", got)
		}
	}
	wantWords(t, "opening to everyone", at.hints(t), "scene_changed", "scene_changed", "scene_changed", "scene_changed")

	// A player who joins the campaign later is in: the scene is open to the table, not to a list.
	late := at.h.newUser("Duda")
	at.h.join(at.master, at.campaignID, late)
	if got := at.readScene(t, late); got.GetPointId() != at.point {
		t.Errorf("a player who joined later reads %v, want the scene open to everyone", got)
	}
}

func TestSceneAudience_APlayerWhoJoinsLaterIsOutOfAChosenScene(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openFor(t, at.point, at.ana)
	late := at.h.newUser("Duda")
	at.h.join(at.master, at.campaignID, late)
	if got := at.readScene(t, late); got != nil {
		t.Errorf("a player who joined later reads %v, want no scene: the list is exactly the players named", got)
	}
}

func TestSceneAudience_OpenRefusals(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	ctx := t.Context()
	stranger := at.h.newUser("Estranha")
	open := func(aud *playv1.SceneAudience) error {
		_, err := at.master.play.OpenScene(ctx, connect.NewRequest(&playv1.OpenSceneRequest{CampaignId: at.campaignID, PointId: at.point, Audience: aud}))
		return err
	}
	for name, aud := range map[string]*playv1.SceneAudience{
		"the master":             {UserIds: []string{at.master.id}},
		"an account that is not": {UserIds: []string{newKey()}},
		"not a UUID":             {UserIds: []string{"ana"}},
		"someone outside":        {UserIds: []string{stranger.id}},
		"nobody":                 {},
		"everyone and a list":    {Everyone: true, UserIds: []string{at.ana.id}},
	} {
		err := open(aud)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("OpenScene(%s) error = %v, want invalid_argument", name, err)
		}
	}
	if got := at.readScene(t, at.master); got != nil {
		t.Errorf("a refused opening opened %v", got)
	}
	// Everyone, spelled out, is the default.
	if err := open(&playv1.SceneAudience{Everyone: true}); err != nil {
		t.Errorf("OpenScene(everyone) error = %v", err)
	}
	// A repeated account counts once.
	at.mustSetAudience(t, at.ana, at.ana)
	if got := at.readScene(t, at.master).GetAudience(); len(got.GetInside()) != 1 {
		t.Errorf("inside after a repeated account = %v, want one", got.GetInside())
	}
}

func TestSceneAudience_SetSceneAudienceRefusals(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	ctx := t.Context()
	// No scene is open.
	_, err := at.setAudience(at.master, audienceOf(at.ana))
	wantSceneRefusal(t, "SetSceneAudience(no scene)", err, connect.CodeFailedPrecondition, sceneNoScene, "no scene is open")
	at.openScene(t, at.point)
	_, err = at.setAudience(at.ana, audienceOf(at.ana))
	wantRefusal(t, "a player", err, connect.CodePermissionDenied, "")
	_, err = at.setAudience(at.master, nil)
	wantRefusal(t, "no audience", err, connect.CodeInvalidArgument, "")
	_, err = at.setAudience(at.master, &playv1.SceneAudience{})
	wantRefusal(t, "nobody", err, connect.CodeInvalidArgument, "")
	_, err = at.setAudience(at.master, &playv1.SceneAudience{UserIds: []string{at.master.id}})
	wantRefusal(t, "the master", err, connect.CodeInvalidArgument, "")
	other := at.h.newUser("Estranha")
	_, err = other.play.SetSceneAudience(ctx, connect.NewRequest(&playv1.SetSceneAudienceRequest{CampaignId: at.campaignID, Audience: audienceOf(at.ana)}))
	wantRefusal(t, "a stranger", err, connect.CodeNotFound, "")
	// No session at all.
	campaign := at.h.newCampaign(at.master, "Sem sessão", at.ana)
	_, err = at.master.play.SetSceneAudience(ctx, connect.NewRequest(&playv1.SetSceneAudienceRequest{CampaignId: campaign, Audience: audienceOf(at.ana)}))
	wantSceneNoSession(t, "SetSceneAudience(no session)", err)
}

func TestSceneAudience_ChangingWhoIsInIsLiveForTheOnesAffected(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openFor(t, at.point, at.ana, at.caio)
	at.hints(t)

	// Caio goes out and Bia comes in: the one who went out reads "no scene" at once and hears it;
	// the one who came in reads it and hears it; Ana, who stays, hears the change too; nobody else.
	info := at.mustSetAudience(t, at.ana, at.bia)
	if got := scenePlayerNames(info.GetAudience().GetInside()); !slices.Equal(got, []string{"Brisa", "Pensantus"}) {
		t.Errorf("inside = %v, want Brisa and Pensantus", got)
	}
	if got := scenePlayerNames(info.GetAudience().GetOutside()); !slices.Equal(got, []string{"Toren"}) {
		t.Errorf("outside = %v, want Toren", got)
	}
	if got := at.readScene(t, at.caio); got != nil {
		t.Errorf("Caio (taken out) reads %v, want no scene", got)
	}
	if got := at.readScene(t, at.bia); got.GetPointId() != at.point {
		t.Errorf("Bia (brought in) reads %v, want the scene", got)
	}
	wantWords(t, "Caio out, Bia in", at.hints(t), "scene_changed", "scene_changed", "scene_changed", "scene_changed")

	// Nobody changes: nothing is sent.
	at.mustSetAudience(t, at.bia, at.ana)
	wantWords(t, "the same players", at.hints(t), "", "", "", "")

	// Ana alone: Bia is out now and hears it, Caio (out already) hears nothing.
	at.mustSetAudience(t, at.ana)
	wantWords(t, "Bia out", at.hints(t), "scene_changed", "scene_changed", "", "scene_changed")

	// Opened to everyone: Caio and Bia come in and hear it, Ana is told too.
	res, err := at.setAudience(at.master, &playv1.SceneAudience{Everyone: true})
	if err != nil || !res.GetScene().GetAudience().GetEveryone() {
		t.Fatalf("SetSceneAudience(everyone) = %v, %v", res, err)
	}
	wantWords(t, "everyone", at.hints(t), "scene_changed", "scene_changed", "scene_changed", "scene_changed")
	for _, u := range []*user{at.caio, at.bia} {
		if at.readScene(t, u).GetPointId() != at.point {
			t.Error("a player brought in by 'everyone' does not read the scene")
		}
	}
	// And back to some: the ones left out hear it.
	at.mustSetAudience(t, at.caio)
	wantWords(t, "Caio alone", at.hints(t), "scene_changed", "scene_changed", "scene_changed", "scene_changed")
	if at.readScene(t, at.ana) != nil || at.readScene(t, at.bia) != nil {
		t.Error("the players taken out of the scene still read it")
	}
}

func TestSceneAudience_RollsFollowTheAudience(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openFor(t, at.point, at.ana)
	roll := func(u *user) error {
		_, err := sceneInApp(at.armed, u, at.action, newKey(), 14)
		return err
	}
	// The one out is refused as when no scene is open: the same code, reason and message.
	err := roll(at.caio)
	wantSceneRefusal(t, "an outsider's roll", err, connect.CodeFailedPrecondition, sceneNoScene, "no scene is open")
	if err := roll(at.ana); err != nil {
		t.Fatalf("a roll by the player in the scene: %v", err)
	}
	// Taken out, her rolls stop at once; the roll she made stays, and comes back with her.
	at.mustSetAudience(t, at.caio)
	err = roll(at.ana)
	wantSceneRefusal(t, "a roll after going out", err, connect.CodeFailedPrecondition, sceneNoScene, "no scene is open")
	at.mustSetAudience(t, at.caio, at.ana)
	if got := at.readScene(t, at.ana).GetRolls(); len(got) != 1 {
		t.Errorf("Ana's rolls after coming back = %d, want the one she made", len(got))
	}
}

func TestSceneAudience_GrantingAnAttemptToAnOutsiderIsRefused(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openFor(t, at.point, at.ana)
	_, err := sceneGrant(at.armed, at.master, at.action, at.toren.GetId(), newKey())
	wantRefusal(t, "a grant to a character out of the scene", err, connect.CodeNotFound, "character not found")
	if _, err := sceneGrant(at.armed, at.master, at.action, at.pens.GetId(), newKey()); err != nil {
		t.Errorf("a grant to a character in the scene: %v", err)
	}
	wantWords(t, "the grant", at.hints(t), "scene_changed", "scene_changed", "", "")
}

func TestSceneAudience_TheStageAndItsHintsGoToTheSceneOnly(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openFor(t, at.point, at.ana, at.caio)
	at.hints(t)
	ctx := t.Context()
	if _, err := at.master.play.PutOnStage(ctx, connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: at.campaignID, CharacterId: at.goblin.GetId()})); err != nil {
		t.Fatalf("PutOnStage() error = %v", err)
	}
	wantWords(t, "an NPC on the stage", at.hints(t), "stage_changed", "stage_changed", "stage_changed", "")
	if got := at.readScene(t, at.ana).GetStage(); len(got) != 1 {
		t.Errorf("Ana reads %d NPCs on the stage, want 1", len(got))
	}
	// A scene of other players replaces it: the stage empties, and the players of the first hear it too.
	pt2, _ := at.h.newScene(at.mapID, "A ponte", false, 1)
	at.openFor(t, pt2, at.bia)
	wantWords(t, "another scene", at.hints(t), "scene_changed,stage_changed", "scene_changed,stage_changed", "scene_changed,stage_changed", "scene_changed,stage_changed")
	if at.readScene(t, at.ana) != nil || at.readScene(t, at.bia).GetPointId() != pt2 {
		t.Error("the second scene is not Bia's alone")
	}
}

func TestSceneAudience_OpeningTheOpenSceneAgainKeepsItsAudienceUnlessTheMasterSaysOne(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openFor(t, at.point, at.ana)
	at.hints(t)
	// With no audience, it changes nothing (a second tap must not widen the scene).
	at.openScene(t, at.point)
	if got := at.readScene(t, at.caio); got != nil {
		t.Errorf("Caio reads %v after the scene was opened again with no audience", got)
	}
	wantWords(t, "opened again", at.hints(t), "", "", "", "")
	// With an audience, it is applied.
	at.openFor(t, at.point, at.caio)
	if at.readScene(t, at.caio).GetPointId() != at.point || at.readScene(t, at.ana) != nil {
		t.Error("opening the open scene with another audience did not apply it")
	}
	// Closing it and opening it again, with none: everyone.
	if _, err := at.master.play.CloseScene(t.Context(), connect.NewRequest(&playv1.CloseSceneRequest{CampaignId: at.campaignID})); err != nil {
		t.Fatalf("CloseScene() error = %v", err)
	}
	at.openScene(t, at.point)
	if got := at.readScene(t, at.bia); got.GetPointId() != at.point {
		t.Errorf("Bia reads %v after a new opening with no audience, want the scene", got)
	}
}

func TestSceneAudience_TheGroupCheckGoesToTheScenesPlayersOnly(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	ctx := t.Context()
	at.openFor(t, at.point, at.ana, at.caio)
	at.hints(t)
	res, err := at.master.contests.RequestGroupCheck(ctx, connect.NewRequest(&playv1.RequestGroupCheckRequest{
		CampaignId: at.campaignID, IdempotencyKey: newKey(), SkillKey: "skill:stealth", Dc: 12, ShowDc: true,
	}))
	if err != nil {
		t.Fatalf("RequestGroupCheck() error = %v", err)
	}
	check := res.Msg.GetGroupCheck()
	if check.GetAskedCount() != 2 {
		t.Errorf("asked = %d, want the two characters in the scene", check.GetAskedCount())
	}
	wantWords(t, "the group check", at.hints(t), "group_check_changed", "group_check_changed", "group_check_changed", "")
	for _, u := range []*user{at.ana, at.caio} {
		got, err := u.contests.GetGroupCheck(ctx, connect.NewRequest(&playv1.GetGroupCheckRequest{CampaignId: at.campaignID}))
		if err != nil || got.Msg.GetGroupCheck().GetId() != check.GetId() {
			t.Errorf("a player in the scene reads the group check %v, %v; want it", got, err)
		}
	}
	got, err := at.bia.contests.GetGroupCheck(ctx, connect.NewRequest(&playv1.GetGroupCheckRequest{CampaignId: at.campaignID}))
	if err != nil || got.Msg.GetGroupCheck() != nil {
		t.Errorf("Bia (out of the scene) reads the group check %v, %v; want none", got, err)
	}
	// Naming a character out of the scene is refused, as naming one that does not exist.
	if _, err := at.master.contests.CloseGroupCheck(ctx, connect.NewRequest(&playv1.CloseGroupCheckRequest{CampaignId: at.campaignID, IdempotencyKey: newKey(), GroupCheckId: check.GetId()})); err != nil {
		t.Fatalf("CloseGroupCheck() error = %v", err)
	}
	_, err = at.master.contests.RequestGroupCheck(ctx, connect.NewRequest(&playv1.RequestGroupCheckRequest{
		CampaignId: at.campaignID, IdempotencyKey: newKey(), SkillKey: "skill:stealth", CharacterIds: []string{at.bri.GetId()},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("asking a character out of the scene error = %v, want invalid_argument", err)
	}
	// A player taken out of the scene cannot answer the check that was asked of them.
	if _, err := at.master.contests.RequestGroupCheck(ctx, connect.NewRequest(&playv1.RequestGroupCheckRequest{
		CampaignId: at.campaignID, IdempotencyKey: newKey(), SkillKey: "skill:stealth",
	})); err != nil {
		t.Fatalf("RequestGroupCheck() again error = %v", err)
	}
	at.mustSetAudience(t, at.ana)
	_, err = at.caio.contests.RollGroupCheck(ctx, connect.NewRequest(&playv1.RollGroupCheckRequest{CampaignId: at.campaignID, IdempotencyKey: newKey(), Roll: &playv1.CheckRollInput{Roll: &playv1.CheckRollInput_RollInApp{RollInApp: true}}}))
	if err == nil {
		t.Error("a player taken out of the scene rolled the group check")
	}
}

// The session summary counts the checks the players passed in scenes that showed their DC (summary.go). A roll
// made in a scene open to only some players counts for the master, for the player who rolled and for the
// players who were in the scene, and for no other.
func TestSceneAudience_TheSummaryDoesNotCountAScenesRollsForThoseOutside(t *testing.T) {
	t.Parallel()
	at := newAudienceTable(t)
	at.openFor(t, at.point, at.ana, at.bia)
	at.checkRoll(t, at.ana, at.action, 18) // DC 12: passed, in a scene that shows its DC
	var session string
	if err := at.h.pool.QueryRow(t.Context(), `SELECT id FROM game_sessions WHERE campaign_id = $1`, at.campaignID).Scan(&session); err != nil {
		t.Fatalf("find the session: %v", err)
	}
	if _, err := at.master.play.EndGameSession(t.Context(), connect.NewRequest(&playv1.EndGameSessionRequest{CampaignId: at.campaignID, GameSessionId: session})); err != nil {
		t.Fatalf("EndGameSession() error = %v", err)
	}
	passedBy := func(u *user) int32 {
		sum, err := at.summary(t, u, session)
		if err != nil {
			t.Fatalf("GetSessionSummary() error = %v", err)
		}
		for _, c := range sum.GetCategories() {
			if c.GetKind() == playv1.HighlightKind_HIGHLIGHT_KIND_CHECKS_PASSED {
				return c.GetValue()
			}
		}
		return 0
	}
	for _, u := range []*user{at.master, at.ana, at.bia} {
		if got := passedBy(u); got != 1 {
			t.Errorf("checks passed in the summary of a player in the scene = %d, want 1", got)
		}
	}
	if got := passedBy(at.caio); got != 0 {
		t.Errorf("checks passed in the summary of Caio (out of the scene) = %d, want 0", got)
	}
}
