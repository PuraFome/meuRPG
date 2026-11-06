package play

import (
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The table's roll_minimum on the d20 rolls that are not attacks (#757).
// An ability check, an initiative and a save that no earlier test reaches count a d20 below the minimum as the minimum.
// The faces stay as they came up, and the roll says what it counted as and which feature did it.
// Whoever reads the faces of a roll reads what it counted as, and nobody else reads either.
// A retry of the same request gives the stored roll again.
// These tests need the database (MEURPG_TEST_DATABASE_URL).

// minimumOf is a table feat that counts a d20 below the value on these rolls as the value, given to the characters.
func (a *armed) minimumOf(t *testing.T, value string, rolls []string, who ...*charactersv1.Character) string {
	t.Helper()
	feat := a.rerollFeat(t, &rulesv1.TableEffect{Type: "roll_minimum", Rolls: rolls, Value: value})
	for _, c := range who {
		a.giveFeat(t, c, feat)
	}
	return feat
}

// minimumOf10 is minimumOf with the value 10.
func (a *armed) minimumOf10(t *testing.T, rolls []string, who ...*charactersv1.Character) string {
	t.Helper()
	return a.minimumOf(t, "10", rolls, who...)
}

// wantCheckCounted checks a check roll shows the face as it came up, what it counted as, the feature and the total.
func wantCheckCounted(t *testing.T, what string, r *playv1.CheckRoll, face, counted int32, source string) {
	t.Helper()
	if len(r.GetFaces()) != 1 || r.GetFaces()[0] != face || r.TreatedAs == nil || r.GetTreatedAs() != counted ||
		r.GetTreatedAsSource() != source || r.GetTotal() != counted+r.GetModifier() {
		t.Errorf("%s = %v; want the face %d counted as %d by %q, total %d", what, r, face, counted, source, counted+r.GetModifier())
	}
}

// wantCheckPlain checks a check roll counts the face as it came up and says nothing of a minimum.
func wantCheckPlain(t *testing.T, what string, r *playv1.CheckRoll, face int32) {
	t.Helper()
	if len(r.GetFaces()) != 1 || r.GetFaces()[0] != face || r.TreatedAs != nil || r.GetTreatedAsSource() != "" || r.GetTotal() != face+r.GetModifier() {
		t.Errorf("%s = %v; want the plain %d and no mention of a minimum", what, r, face)
	}
}

// A scene check counts the minimum: Pensantus (Investigation +4) rolls a 3 and the feat counts it 10, 14 against DC 12.
// The master and the roller read the 3 counted as 10, the other players read no roll, and a retry gives the stored roll.
func TestASceneCheckCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	feat := a.minimumOf10(t, []string{"check"}, a.pens)
	point, actions := a.h.newScene(a.mapID, "A carroça", true, 3, sceneSpec{"skill:investigation", new(int32(12))})
	a.openScene(t, point)
	key := newKey()
	first, err := sceneInApp(a, a.ana, actions[0], key, 3)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	r := first.GetRoll()
	wantCheckCounted(t, "the roll", checkOfDice(r.GetRoll()), 3, 10, feat)
	if !r.GetPassed() || r.GetRoll().GetTotal() != 14 {
		t.Errorf("the roll = %v, want 14 against DC 12, a pass", r)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Pensantus's player", a.ana}} {
		rolls := a.readScene(t, who.u).GetRolls()
		if len(rolls) != 1 {
			t.Fatalf("%s reads the rolls %v, want the one", who.name, rolls)
		}
		wantCheckCounted(t, who.name+"'s roll", checkOfDice(rolls[0].GetRoll()), 3, 10, feat)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"Toren's player", a.caio}, {"Brisa's player", a.bia}} {
		if rolls := a.readScene(t, who.u).GetRolls(); len(rolls) != 0 {
			t.Errorf("%s reads the rolls %v, want none", who.name, rolls)
		}
	}
	a.h.roller.queue(17)
	again, err := sceneRollWith(a, a.ana, a.campaignID, actions[0], key, &playv1.RollSceneCheckRequest{Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true}})
	if err != nil || !proto.Equal(again.GetRoll(), r) {
		t.Errorf("the retry = %v, %v; want the stored roll %v", again, err, r)
	}
}

// checkOfDice reads a scene's dice roll as the check roll the other checks carry.
func checkOfDice(d *playv1.DiceRoll) *playv1.CheckRoll {
	return &playv1.CheckRoll{
		Faces: d.GetFaces(), Modifier: d.GetModifier(), Total: d.GetTotal(),
		TreatedAs: d.TreatedAs, TreatedAsSource: d.GetTreatedAsSource(),
	}
}

// A group check counts the minimum of each member, in the player's own roll and in the master's roll for a player.
// Toren's player and the master read Toren's 4 counted as 10.
// The others read nothing of it.
// The retry of a roll gives the stored result.
func TestAGroupCheckCountsTheMinimumOfThePlayersRollAndOfTheMastersRollForAPlayer(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	feat := a.minimumOf10(t, []string{"check"}, a.toren, a.bri)
	asked := a.mustRequestGroup(t, nil) // Stealth, DC 12
	key := newKey()
	a.h.roller.queue(4)
	own, err := a.caio.contests.RollGroupCheck(t.Context(), connect.NewRequest(&playv1.RollGroupCheckRequest{
		CampaignId: a.campaignID, IdempotencyKey: key, GroupCheckId: asked.GetId(), Roll: rollIn(),
	}))
	if err != nil {
		t.Fatalf("RollGroupCheck() error = %v", err)
	}
	wantCheckCounted(t, "Toren's roll", memberOf(own.Msg.GetGroupCheck(), a.toren.GetId()).GetRoll(), 4, 10, feat)
	// The master rolls Brisa's: the 5 counts 10 too.
	forBrisa, err := a.rollForPlayer(t, a.master, asked.GetId(), a.bri.GetId(), 5)
	if err != nil {
		t.Fatalf("RollForPlayer() error = %v", err)
	}
	wantCheckCounted(t, "Brisa's roll for the master", memberOf(forBrisa.GetGroupCheck(), a.bri.GetId()).GetRoll(), 5, 10, feat)
	a.mustRollGroup(t, a.ana, asked.GetId(), 4) // Pensantus has no feature
	all := a.groupView(t, a.master)
	wantCheckCounted(t, "the master's read of Toren", memberOf(all, a.toren.GetId()).GetRoll(), 4, 10, feat)
	wantCheckCounted(t, "the master's read of Brisa", memberOf(all, a.bri.GetId()).GetRoll(), 5, 10, feat)
	wantCheckPlain(t, "the master's read of Pensantus", memberOf(all, a.pens.GetId()).GetRoll(), 4)
	mine := a.groupView(t, a.caio)
	if len(mine.GetMembers()) != 1 {
		t.Fatalf("Toren's player reads %v, want only their own", mine.GetMembers())
	}
	wantCheckCounted(t, "Toren's player's read", mine.GetMembers()[0].GetRoll(), 4, 10, feat)
	// Pensantus's player reads the roll of Pensantus alone, plain.
	theirs := a.groupView(t, a.ana)
	if len(theirs.GetMembers()) != 1 || theirs.GetMembers()[0].GetCharacterId() != a.pens.GetId() {
		t.Fatalf("Pensantus's player reads %v, want only their own", theirs.GetMembers())
	}
	wantCheckPlain(t, "Pensantus's player's read", theirs.GetMembers()[0].GetRoll(), 4)
	// A retry of the roll gives what was stored.
	a.h.roller.queue(17)
	again, err := a.caio.contests.RollGroupCheck(t.Context(), connect.NewRequest(&playv1.RollGroupCheckRequest{
		CampaignId: a.campaignID, IdempotencyKey: key, GroupCheckId: asked.GetId(), Roll: rollIn(),
	}))
	if err != nil || !proto.Equal(again.Msg.GetGroupCheck(), own.Msg.GetGroupCheck()) {
		t.Errorf("the retry = %v, %v; want the stored answer %v", again, err, own.Msg.GetGroupCheck())
	}
}

// A Hide counts the minimum of the Stealth check: Toren's 4 counts 10, the master and Toren's player read it and the others read no attempt.
// A retry of the same request gives the same attempt.
func TestAHideCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	feat := a.minimumOf10(t, []string{"check"}, a.toren)
	c := a.arena(t, arenaPlan{})
	key := newKey()
	a.h.roller.queue(4)
	hide := func() (*playv1.HideAttemptView, error) {
		res, err := a.caio.contests.Hide(t.Context(), connect.NewRequest(&playv1.HideRequest{
			CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: key, CombatantId: c.id(t, "Toren"), ActionKey: cunningHide, Roll: rollIn(),
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetAttempt(), nil
	}
	first, err := hide()
	if err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	wantCheckCounted(t, "the attempt", first.GetRoll(), 4, 10, feat)
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Toren's player", a.caio}} {
		got := c.state(t, who.u).GetHideAttempts()
		if len(got) != 1 {
			t.Fatalf("%s reads the attempts %v, want the one", who.name, got)
		}
		wantCheckCounted(t, who.name+"'s read", got[0].GetRoll(), 4, 10, feat)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"Pensantus's player", a.ana}, {"Brisa's player", a.bia}} {
		if got := c.state(t, who.u).GetHideAttempts(); len(got) != 0 {
			t.Errorf("%s reads the attempts %v, want none", who.name, got)
		}
	}
	a.h.roller.queue(17)
	again, err := hide()
	if err != nil || !proto.Equal(again, first) {
		t.Errorf("the retry = %v, %v; want the stored attempt %v", again, err, first)
	}
}

// A contest counts the minimum of the defender: Toren answers the hobgoblin's grapple with a 4 that counts 10.
// The hobgoblin has no feature, so its 12 stays.
// The master and Toren's player read the counted number, Pensantus's player reads no roll, and a retry gives the stored answer.
func TestAContestCountsTheMinimumOfTheDefender(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	feat := a.minimumOf10(t, []string{"check"}, a.toren)
	c := a.arena(t, arenaPlan{})
	// Toren defends: the hobgoblin grapples him and Toren answers with Athletics.
	c.advance(t, c.hob)
	c.h.roller.queue(12)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Toren"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	id := started.GetContest().GetId()
	key := newKey()
	answer := func() (*playv1.ContestView, error) {
		res, err := a.caio.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
			CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: key, ContestId: id, Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: rollIn(),
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetContest(), nil
	}
	a.h.roller.queue(4)
	answered, err := answer()
	if err != nil {
		t.Fatalf("RespondContest() error = %v", err)
	}
	wantCheckCounted(t, "the defender's roll", answered.GetDefenderRoll(), 4, 10, feat)
	wantCheckPlain(t, "the initiator's roll", started.GetContest().GetInitiatorRoll(), 12)
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Toren's player", a.caio}} {
		got := c.state(t, who.u).GetContests()
		if len(got) != 1 || got[0].GetId() != id {
			t.Fatalf("%s reads the contests %v, want the one", who.name, got)
		}
		wantCheckCounted(t, who.name+"'s read of the defender", got[0].GetDefenderRoll(), 4, 10, feat)
	}
	for _, v := range c.state(t, a.ana).GetContests() {
		if v.GetDefenderRoll() != nil || v.GetInitiatorRoll() != nil {
			t.Errorf("Pensantus's player reads the contest %v, want no roll", v)
		}
	}
	a.h.roller.queue(17)
	again, err := answer()
	if err != nil || !proto.Equal(again, answered) {
		t.Errorf("the retry = %v, %v; want the stored answer %v", again, err, answered)
	}
}

// A contest counts the minimum of the initiator: Toren's grapple rolls a 3 that counts 10.
// The master and Toren's player read it, Pensantus's player reads no roll, and a retry gives the stored contest.
func TestAContestCountsTheMinimumOfTheInitiator(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	feat := a.minimumOf10(t, []string{"check"}, a.toren)
	c := a.arena(t, arenaPlan{})
	key := newKey()
	start := func() (*playv1.ContestView, error) {
		res, err := a.caio.contests.StartContest(t.Context(), connect.NewRequest(&playv1.StartContestRequest{
			CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: key, Roll: rollIn(),
			InitiatorId: c.id(t, "Toren"), TargetId: c.id(t, c.hob), Purpose: playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg.GetContest(), nil
	}
	a.h.roller.queue(3)
	started, err := start()
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	wantCheckCounted(t, "the initiator's roll", started.GetInitiatorRoll(), 3, 10, feat)
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Toren's player", a.caio}} {
		got := c.state(t, who.u).GetContests()
		if len(got) != 1 || got[0].GetId() != started.GetId() {
			t.Fatalf("%s reads the contests %v, want the one", who.name, got)
		}
		wantCheckCounted(t, who.name+"'s read of the initiator", got[0].GetInitiatorRoll(), 3, 10, feat)
	}
	for _, v := range c.state(t, a.ana).GetContests() {
		if v.GetInitiatorRoll() != nil {
			t.Errorf("Pensantus's player reads the contest %v, want no roll", v)
		}
	}
	a.h.roller.queue(17)
	again, err := start()
	if err != nil || !proto.Equal(again, started) {
		t.Errorf("the retry = %v, %v; want the stored contest %v", again, err, started)
	}
}

// A hint try counts the minimum of the skill check: Toren's 2 counts 10 and the DC 10 is reached.
// The roller reads the 2 counted as 10 in the answer, the master reads it in the run, and a retry replays the stored roll.
// The other players read no try at all.
func TestAHintTryCountsTheMinimum(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	feat := p.minimumOf10(t, []string{"check"}, p.toren)
	puz := p.riddle(t, "Enigma", hintCheck("skill:investigation", 10))
	p.show(t, puz.GetId())
	key := newKey()
	first, err := p.tryHintKey(t, p.caio, puz.GetId(), 2, key)
	if err != nil {
		t.Fatalf("TryPuzzleHint() error = %v", err)
	}
	wantCountedAs(t, "the roller's roll", first.GetRoll(), 2, 10, first.GetRoll().GetTotal(), feat)
	if first.GetRoll().GetTotal() != 10+first.GetRoll().GetModifier() || !first.GetPassed() {
		t.Errorf("the try = %v, want the 10 plus the bonus, a pass against DC 10", first)
	}
	tries := p.masterRun(t, puz.GetId()).GetHintTries()
	if len(tries) != 1 {
		t.Fatalf("the master reads the tries %v, want the one", tries)
	}
	wantCountedAs(t, "the master's roll", tries[0].GetRoll(), 2, 10, first.GetRoll().GetTotal(), feat)
	again, err := p.tryHintKey(t, p.caio, puz.GetId(), 2, key)
	if err != nil || !again.GetReplayed() || !proto.Equal(again.GetRoll(), first.GetRoll()) {
		t.Errorf("the retry = %v, %v; want the stored roll replayed", again, err)
	}
	for _, u := range []*user{p.ana, p.bia} {
		if got := jsonOf(p.read(t, u, puz.GetId())); strings.Contains(got, "treated") {
			t.Errorf("a player's read %s, want no mention of what the try counted as", got)
		}
	}
}

// A search for traps counts the minimum on both dice of a Perception search.
// Pensantus (Perception +0) rolls 3 and 4.
// The feat counts both 10.
// The searcher reads both numbers counted in the answer, the master and the searcher read them in the activity, the others read no search, and a retry gives the stored answer.
func TestATrapSearchCountsTheMinimumOnBothDice(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	feat := r.minimumOf10(t, []string{"check"}, r.pens)
	r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc, s.FindDc = 16, 16 })
	r.place(t, r.pens.GetId(), 8, 7)
	key := newKey()
	search := func() (*playv1.SearchForTrapsResponse, error) {
		return r.trapSearchCall(t, r.ana, func(q *playv1.SearchForTrapsRequest) {
			trapSearchFaces(perception, 3, 4)(q)
			q.IdempotencyKey = key
		})
	}
	first, err := search()
	if err != nil {
		t.Fatalf("SearchForTraps() error = %v", err)
	}
	wantCheckCounted(t, "the first die", checkOfDice(first.GetRoll()), 3, 10, feat)
	wantCheckCounted(t, "the second die", checkOfDice(first.GetSecondRoll()), 4, 10, feat)
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", r.master}, {"Pensantus's player", r.ana}} {
		var found *playv1.TrapSearchResult
		for _, l := range r.activity(t, who.u) {
			if l.GetSearch() != nil {
				found = l.GetSearch()
			}
		}
		if found == nil {
			t.Fatalf("%s reads no search in the activity", who.name)
		}
		wantCheckCounted(t, who.name+"'s first die", checkOfDice(found.GetRoll()), 3, 10, feat)
		wantCheckCounted(t, who.name+"'s second die", checkOfDice(found.GetSecondRoll()), 4, 10, feat)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"Toren's player", r.caio}, {"Brisa's player", r.bia}} {
		for _, l := range r.activity(t, who.u) {
			if l.GetSearch() != nil {
				t.Errorf("%s reads the search %v, want none", who.name, l.GetSearch())
			}
		}
	}
	again, err := search()
	if err != nil {
		t.Fatalf("the retry error = %v", err)
	}
	again.Sources, first.Sources = nil, nil
	if !proto.Equal(again, first) {
		t.Errorf("the retry = %v, want the stored answer %v", again, first)
	}
}

// An initiative counts the minimum of the Dexterity check: Toren's 3 counts 10, so his initiative is 10 plus his bonus.
// The master and Toren's player read the face and what it counted as, the other players read the initiative alone, and a retry with the same key gives the same initiative.
func TestAnInitiativeCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a, e := setupWithToren(t, "")
	feat := a.minimumOf10(t, []string{"check"}, a.toren)
	key := newKey()
	submit := func(face int32) (*playv1.SubmitInitiativeResponse, error) {
		res, err := a.caio.combat.SubmitInitiative(t.Context(), connect.NewRequest(&playv1.SubmitInitiativeRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: byLabel(t, e, "Toren").GetId(), IdempotencyKey: key,
			Roll: &playv1.SubmitInitiativeRequest_D20Face{D20Face: face},
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	res, err := submit(3)
	if err != nil {
		t.Fatalf("SubmitInitiative() error = %v", err)
	}
	toren := byLabel(t, res.GetEncounter(), "Toren")
	want := 10 + toren.GetInitiativeBonus()
	if toren.GetInitiative() != want || toren.GetInitiativeFace() != 3 || toren.InitiativeTreatedAs == nil || toren.GetInitiativeTreatedAs() != 10 || toren.GetInitiativeTreatedAsSource() != feat {
		t.Errorf("Toren after the roll = initiative %d, face %d, counted as %v by %q; want the 3 counted as 10 by the feat, initiative %d",
			toren.GetInitiative(), toren.GetInitiativeFace(), toren.InitiativeTreatedAs, toren.GetInitiativeTreatedAsSource(), want)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Toren's player", a.caio}} {
		got := byLabel(t, a.get(t, who.u), "Toren")
		if got.GetInitiative() != want || got.GetInitiativeFace() != 3 || got.GetInitiativeTreatedAs() != 10 || got.GetInitiativeTreatedAsSource() != feat {
			t.Errorf("%s reads %v, want the 3 counted as 10 by the feat, initiative %d", who.name, got, want)
		}
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"Pensantus's player", a.ana}, {"Brisa's player", a.bia}} {
		got := byLabel(t, a.get(t, who.u), "Toren")
		if got.GetInitiative() != want || got.InitiativeFace != nil || got.InitiativeTreatedAs != nil || got.GetInitiativeTreatedAsSource() != "" {
			t.Errorf("%s reads %v, want the initiative %d alone, no face and no counted number", who.name, got, want)
		}
	}
	again, err := submit(3)
	if err != nil {
		t.Fatalf("the retry error = %v", err)
	}
	if got := byLabel(t, again.GetEncounter(), "Toren"); got.GetInitiative() != want || got.GetInitiativeFace() != 3 || got.GetInitiativeTreatedAs() != 10 {
		t.Errorf("the retry = %v, want the stored initiative %d", got, want)
	}
}

// A Counterspell check counts the minimum of the reactor: Pensantus's 3 counts 10, and with her +4 it reaches DC 14.
// The 9 of the usual answer is a 13 and the spell goes through.
// Here the minimum counters it.
// The reactor reads the 3 counted as 10 in the answer, the master's log gives the total that counted and no face.
// A retry of the answer changes nothing.
func TestACounterspellCheckCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a, e, w := upcastWindow(t)
	feat := a.minimumOf10(t, []string{"check"}, a.pens)
	key := newKey()
	answer := func() (*playv1.AnswerReactionResponse, error) {
		return a.answerReaction(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
			counterspellTypedAnswer(3)(r)
			r.IdempotencyKey = key
		})
	}
	res, err := answer()
	if err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	c := res.GetResult().GetCounterspell()
	if !c.GetCountered() || c.GetCheckDc() != 14 {
		t.Fatalf("result = %v, want the spell countered on a 14 against 14", c)
	}
	wantCountedAs(t, "the check", c.GetCheck(), 3, 10, 14, feat)
	lineOf := func(u *user) string {
		en := a.logEntryWith(t, u, e, func(en *playv1.CombatLogEntry) bool {
			return en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW
		})
		return en.GetReactionTextPt()
	}
	if got := lineOf(a.master); !strings.Contains(got, "teste de habilidade 14 contra CD 14") {
		t.Errorf("the master's log line = %q, want the check of 14 against 14", got)
	}
	if got := lineOf(a.caio); strings.Contains(got, "teste de habilidade") || strings.Contains(got, "14") {
		t.Errorf("a player's log line = %q, want no numbers", got)
	}
	// The retry answers with the combat alone and adds no second line to the log.
	if again, err := answer(); err != nil || again.GetResult() != nil {
		t.Errorf("the retry = %v, %v; want the combat alone", again, err)
	}
	n := 0
	for _, r := range a.log(t, a.master, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
				n++
			}
		}
	}
	if n != 1 {
		t.Errorf("the log holds %d reaction lines after the retry, want 1", n)
	}
}

// The Dexterity save against Hellish Rebuke counts the minimum of the aggressor, a player's character.
// Toren's 2 counts 10 and, with his +1, is a 11 against DC 12.
// The master who answers and his log read the 2 counted as 10, the players read the line without dice, and a retry adds no second line.
func TestAHellishRebukeSaveCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a, e := rebukeFight(t, "Toren")
	feat := a.minimumOf10(t, []string{"save"}, a.pens)
	hit := a.mustAttack(t, a.master, e, "Toren", battleaxe, "Diabrete", d20(15))
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(3))
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
	if w == nil {
		t.Fatal("Diabrete has no Hellish Rebuke window after being hurt")
	}
	key := newKey()
	answer := func() (*playv1.AnswerReactionResponse, error) {
		return a.answerReaction(t, a.master, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
			r.Answer, r.UseRacial, r.IdempotencyKey = playv1.ReactionChoice_REACTION_CHOICE_USE, true, key
		})
	}
	a.h.roller.queue(2, 5, 5, 5)
	res, err := answer()
	if err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	rebuke := res.GetResult().GetHellishRebuke()
	save := rebuke.GetSave()
	wantCountedAs(t, "the save", save, 2, 10, 10+save.GetModifier(), feat)
	if rebuke.GetSaved() != (save.GetTotal() >= rebuke.GetSaveDc()) {
		t.Errorf("the result = %v, want the save judged on the number that counted", rebuke)
	}
	if !a.logSays(t, a.master, e, "(2, conta como 10)") {
		t.Error("the master's log does not say the 2 counted as 10")
	}
	for name, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana, "Bia": a.bia} {
		if a.logSays(t, u, e, "conta como") || a.logSays(t, u, e, "1d20") {
			t.Errorf("%s reads the dice of the save", name)
		}
		if !a.logSays(t, u, e, "Repreensão Infernal") {
			t.Errorf("%s reads no Hellish Rebuke line", name)
		}
	}
	a.h.roller.queue(19, 5, 5, 5)
	if again, err := answer(); err != nil || again.GetResult() != nil {
		t.Errorf("the retry = %v, %v; want the combat alone", again, err)
	}
	n := 0
	for _, l := range logEntries(a.log(t, a.master, e)) {
		if strings.Contains(l.GetReactionTextPt(), "Repreensão Infernal") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the log holds %d Hellish Rebuke lines after the retry, want 1", n)
	}
}

// The save of a monk's Stunning Strike counts the minimum of the target.
// Capanga, an NPC with a full sheet, rolls a 2 on his Constitution save and the feat counts it 10.
// Only the master reads the NPC's save, so the line he reads says the 2 counted as 10 and the monk's player reads the result alone.
func TestAMonkRiderSaveCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:monk", 5, ""), abilities(10, 16, 14, 14, 8), nil, nil, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, abilities(10, 14, 12, 16, 8), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, abilities(10, 16, 14, 10, 8), []string{rapier}, nil)
	})
	npc := a.master.fullNPC(t, a.campaignID, "Capanga", "race:human", "class:fighter", 3, abilities(14, 12, 14, 10, 10), []string{rapier})
	feat := a.minimumOf10(t, []string{"save"}, npc)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: npc.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Capanga"},
		at: map[string][2]int32{"Toren": {3, 3}, "Capanga": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	})
	wantHit(t, "the monk's strike", a.mustAttack(t, a.caio, e, "Toren", unarmed, "Capanga", d20(19)), hit)
	offers := ridersOfKind(a.get(t, a.caio), playv1.HitRiderKind_HIT_RIDER_KIND_STUNNING_STRIKE)
	if len(offers) != 1 {
		t.Fatalf("offers = %v, want Stunning Strike", a.get(t, a.caio).GetHitRiders())
	}
	a.h.roller.queue(2)
	if _, err := a.useRider(t, a.caio, e, offers[0].GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_STUN); err != nil {
		t.Fatalf("UseHitRider(stun) error = %v", err)
	}
	if l := a.lastRiderLine(t); l.GetD20() != 2 || l.GetTreatedAs() != 10 || l.GetTreatedAsSource() != feat {
		t.Errorf("the master reads %v, want the 2 counted as 10 by the feat", l)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"the monk's player", a.caio}, {"Pensantus's player", a.ana}, {"Brisa's player", a.bia}} {
		if l := a.lastRiderLineOf(t, who.u); l.GetD20() != 0 || l.TreatedAs != nil || l.GetTreatedAsSource() != "" {
			t.Errorf("%s reads %v, want the result alone", who.name, l)
		}
	}
}

// trapSaveOf is the save a combatant made against the trap as u reads it in the combat log, nil when u reads none.
func (a *armed) trapSaveOf(t *testing.T, u *user, label string) *playv1.TrapSaveRoll {
	t.Helper()
	var save *playv1.TrapSaveRoll
	for _, en := range a.logEntries(t, u, a.get(t, u)) {
		for _, c := range en.GetTrap().GetCaught() {
			if c.GetTargetLabel() == label {
				save = c.GetSave()
			}
		}
	}
	return save
}

// The saving throw against a trap counts the minimum of the target, whether it is a player's character or an NPC.
// Toren (a player) rolls a 3 and Capanga (an NPC with a full sheet) a 2.
// The feat counts both 10.
// Toren's 12 saves against DC 12, Capanga's 11 does not.
// The master reads both rolls with the counted number, Toren's player reads Toren's, the others read the outcome alone.
func TestATrapSaveCountsTheMinimumOfAPlayerAndOfAnNPC(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	npc := r.master.fullNPC(t, r.campaignID, "Capanga", "race:human", "class:fighter", 3, abilities(14, 12, 14, 10, 10), []string{rapier})
	feat := r.minimumOf10(t, []string{"save"}, r.toren, npc)
	net := r.trap(t, "Rede", 9, 7, func(s *mapsv1.TrapSpec) {
		s.Trigger = rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL
		s.Effect = &rulesv1.TrapEffect{
			Save: &rulesv1.TrapSaveEffect{
				Ability: rulesv1.Ability_ABILITY_DEXTERITY, Dc: 12, AppliesTo: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
				OnFail: &rulesv1.TrapOnFail{Damage: []*rulesv1.TrapDamage{{Dice: "1d6", DamageTypeKey: "damage-type:bludgeoning"}}},
				OnPass: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE,
			},
			Targets: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
		}
	})
	r.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: npc.GetId()}}, npcRolls: []int{2},
		players: map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Capanga"},
		at: map[string][2]int32{"Toren": {6, 7}, "Pensantus": {5, 8}, "Brisa": {4, 7}, "Capanga": {12, 13}},
	})
	r.h.roller.queue(3, 2, 4)
	key := newKey()
	fire := func() (*playv1.FireTrapResponse, error) {
		res, err := r.master.play.FireTrap(t.Context(), connect.NewRequest(&playv1.FireTrapRequest{
			CampaignId: r.campaignID, MapId: r.mapID, PointId: net.GetId(), TargetIds: []string{r.id(t, "Toren"), r.id(t, "Capanga")}, IdempotencyKey: key,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	res, err := fire()
	if err != nil {
		t.Fatalf("FireTrap() error = %v", err)
	}
	if len(res.GetFiring().GetCaught()) != 2 {
		t.Fatalf("the firing caught %v, want Toren and Capanga", res.GetFiring().GetCaught())
	}
	for _, who := range []struct {
		label string
		face  int32
		saved bool
	}{{"Toren", 3, true}, {"Capanga", 2, false}} {
		save := r.trapSaveOf(t, r.master, who.label)
		if save == nil || (save.GetOutcome() == playv1.SaveOutcome_SAVE_OUTCOME_SAVED) != who.saved {
			t.Fatalf("the master reads %s's save %v, want saved = %v", who.label, save, who.saved)
		}
		wantCountedAs(t, who.label+"'s save", save.GetRoll(), who.face, 10, 10+save.GetRoll().GetModifier(), feat)
	}
	wantCountedAs(t, "Toren's save as his player reads it", r.trapSaveOf(t, r.caio, "Toren").GetRoll(), 3, 10, 10+r.trapSaveOf(t, r.caio, "Toren").GetRoll().GetModifier(), feat)
	for _, who := range []struct {
		name  string
		u     *user
		label string
	}{{"Toren's player on Capanga", r.caio, "Capanga"}, {"Pensantus's player on Toren", r.ana, "Toren"}, {"Pensantus's player on Capanga", r.ana, "Capanga"}} {
		save := r.trapSaveOf(t, who.u, who.label)
		if save == nil || save.GetRoll() != nil {
			t.Errorf("%s reads the save %v, want the outcome alone", who.name, save)
		}
	}
	r.h.roller.queue(19, 19, 19)
	if again, err := fire(); err != nil || !proto.Equal(again.GetFiring(), res.GetFiring()) {
		t.Errorf("the retry = %v, %v; want the stored firing %v", again, err, res.GetFiring())
	}
}

// Reliable Talent and a table minimum compete for the floor of a skill check.
// Toren is a rogue of level 11, expert in Stealth, and not proficient in Athletics.
// The higher floor wins, a tie keeps Reliable Talent, and a minimum of attacks does not reach a check.
// Reliable Talent needs the proficiency, a table minimum does not.
func TestReliableTalentAndATableMinimumCompeteForTheFloorOfACheck(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		value   string
		rolls   []string
		skill   string
		counted int32
		source  string
	}{
		{"the higher minimum wins", "12", []string{"check"}, stealthKey, 12, ""},
		{"a tie keeps Reliable Talent", "10", []string{"check"}, stealthKey, 10, "feature:reliable-talent"},
		{"a lower minimum leaves Reliable Talent", "8", []string{"check"}, stealthKey, 10, "feature:reliable-talent"},
		{"a minimum of attacks is no floor of a check", "15", []string{"attack"}, stealthKey, 10, "feature:reliable-talent"},
		{"the minimum floors a skill Reliable Talent does not reach", "10", []string{"check"}, "skill:athletics", 10, ""},
		{"no feature floors a skill without proficiency", "15", []string{"attack"}, "skill:athletics", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newReliableSneaks(t)
			feat := a.minimumOf(t, tc.value, tc.rolls, a.toren)
			asked := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = tc.skill })
			roll := memberOf(a.mustRollGroup(t, a.caio, asked.GetId(), 4), a.toren.GetId()).GetRoll()
			source := tc.source
			if source == "" {
				source = feat
			}
			if tc.counted == 0 {
				wantCheckPlain(t, "Toren's 4", roll, 4)
				return
			}
			wantCheckCounted(t, "Toren's 4", roll, 4, tc.counted, source)
		})
	}
}

// A pair of dice on a check counts the minimum on the die that counts, and the faces stay.
// Pensantus searches with Investigation (+4) with advantage on the check: the [2, 4] counts the 4, and the 4 counts as 10.
func TestAMinimumCountsTheDieThatCountsOfAPairOnACheck(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	feat := r.minimumOf10(t, []string{"check"}, r.pens)
	r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc, s.FindDc = 16, 16 })
	r.place(t, r.pens.GetId(), 8, 7)
	r.master.liveSession(t, r.campaignID)
	r.giveTrapSearchEffect(t, r.pens.GetId(), "spell:enhance-ability", `[{"kind":"check_advantage","abilities":["wis","int"]}]`)
	res, err := r.trapSearchCall(t, r.ana, trapSearchFaces(investigation, 2, 4))
	if err != nil {
		t.Fatalf("SearchForTraps() error = %v", err)
	}
	d := res.GetRoll()
	if res.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || len(d.GetFaces()) != 2 || d.GetFaces()[0] != 2 || d.GetFaces()[1] != 4 || d.GetCountedIndex() != 1 {
		t.Fatalf("the roll = %v, want the pair [2 4] with advantage and the 4 counting", res)
	}
	wantCountedAs(t, "the roll", d, 4, 10, 14, feat)
}

// A d20 of 1 under a minimum of 10 is an ordinary 10 on a check, but Lucky is asked about the 1 that came up.
// Dalila, a halfling rogue, rolls an Athletics check (+0, no proficiency, so no Reliable Talent) and shows a 1.
// The question is about the 1.
// Keeping it writes the roll with the face 1 counted as 10 by the feat, and rolling it again counts the new face against the minimum too.
func TestAOneWithAMinimumOf10OnACheckAsksLuckyAboutTheOneAndCountsTheTen(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, id := joinDalila(t, a.h, a.master, a.campaignID)
	feat := a.minimumOf10(t, []string{"check"}, &charactersv1.Character{Id: id, CampaignId: a.campaignID})
	point, actions := a.h.newScene(a.mapID, "O muro do pátio", true, 3, sceneSpec{key: "skill:athletics", dc: new(int32(25))})
	a.openScene(t, point)

	held, err := a.rollSceneTyped(t, dalila, actions[0], newKey(), 1)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	if held.GetRoll() != nil {
		t.Fatalf("the held roll = %v, want the Lucky question alone", held)
	}
	offer := a.heldLuckyOffer(t, dalila)
	if f := offer.GetD20().GetFaces(); len(f) != 1 || f[0] != 1 {
		t.Fatalf("the Lucky question shows %v, want the 1 as it came up", f)
	}
	kept, err := answerOutsideLucky(t, dalila, a.campaignID, offer.GetHoldId(), outsideKeepTheOne)
	if err != nil {
		t.Fatalf("AnswerOutsideLucky(keep) error = %v", err)
	}
	r := kept.GetSceneCheck().GetRoll().GetRoll()
	wantCheckCounted(t, "the kept roll", checkOfDice(r), 1, 10, feat)
	if len(r.GetRerolled()) != 0 {
		t.Errorf("the kept roll = %v, want nothing rolled again", r)
	}

	_, err = a.rollSceneTyped(t, dalila, actions[0], newKey(), 1)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	a.h.roller.queue(4)
	done, err := answerOutsideLucky(t, dalila, a.campaignID, a.heldLuckyOffer(t, dalila).GetHoldId(), outsideRerollInApp)
	if err != nil {
		t.Fatalf("AnswerOutsideLucky(reroll) error = %v", err)
	}
	rolled := done.GetSceneCheck().GetRoll().GetRoll()
	wantCheckCounted(t, "the new roll", checkOfDice(rolled), 4, 10, feat)
	wantLuckyReroll(t, "the new roll", rolled, 0, 4)
}

// The saving throw of a Turn Undead keeps what the minimum did through the event it is stored as.
// No undead can carry a feat today (they come from stat blocks, which have none), so the database cannot reach this path.
// The stored event is replayed into the proto: the counted turn shows the face and the number, a plain one shows neither.
func TestTheSaveOfATurnedUndeadKeepsWhatTheMinimumDidInTheStoredEvent(t *testing.T) {
	t.Parallel()
	stored := channelEvent{DC: 14, Turns: []channelTurn{
		{Target: "z1", Label: "Zumbi 1", D20: 3, Modifier: 0, Total: 10, Turned: true, countedRoll: countedRoll{CountedAs: 10, CountedBy: "feat:dado-sortudo@mesa"}},
		{Target: "z2", Label: "Zumbi 2", D20: 15, Modifier: 0, Total: 15},
	}}
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("encode the event: %v", err)
	}
	var back channelEvent
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("decode the event: %v", err)
	}
	got := channelProto(&back).GetTurns()
	if len(got) != 2 {
		t.Fatalf("turns = %v, want 2", got)
	}
	if got[0].GetD20() != 3 || got[0].GetTotal() != 10 || got[0].TreatedAs == nil || got[0].GetTreatedAs() != 10 || got[0].GetTreatedAsSource() != "feat:dado-sortudo@mesa" {
		t.Errorf("the counted turn = %v, want the 3 counted as 10 by the feat, total 10", got[0])
	}
	if got[1].GetD20() != 15 || got[1].TreatedAs != nil || got[1].GetTreatedAsSource() != "" {
		t.Errorf("the plain turn = %v, want the 15 alone", got[1])
	}
}

// heldLuckyOffer is the one Lucky question u has open.
func (a *armed) heldLuckyOffer(t *testing.T, u *user) *playv1.HeldRollOffer {
	t.Helper()
	offers := a.listHeldRolls(t, u)
	if len(offers) != 1 || offers[0].GetLucky() == nil {
		t.Fatalf("held rolls = %v, want one Lucky question", offers)
	}
	return offers[0]
}

// An event stored with `reliable_talent` before the floor existed replays the 10 it counted.
func TestAStoredReliableTalentKeyStillGivesItsFloor(t *testing.T) {
	t.Parallel()
	var ev actionEvent
	if err := json.Unmarshal([]byte(`{"reliable_talent":true}`), &ev); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got, want := ev.floor(), (rollFloor{Value: 10, Source: "feature:reliable-talent"}); got != want {
		t.Errorf("floor() = %v, want %v", got, want)
	}
	if err := json.Unmarshal([]byte(`{"floor":12,"floor_by":"effect:x"}`), &ev); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got, want := ev.floor(), (rollFloor{Value: 12, Source: "effect:x"}); got != want {
		t.Errorf("floor() = %v, want %v", got, want)
	}
}

// A group check that is a saving throw picks its floor like a scene save does: a minimum limited to saves floors it and one limited to checks does not, and the other way round for a Stealth check.
func TestAGroupCheckPicksTheFloorByTheKindOfTheRoll(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		rolls   []string
		key     string
		counted bool
	}{
		{"a save minimum floors a group save", []string{"save"}, "save:con", true},
		{"a save minimum does not floor a group check", []string{"save"}, stealthKey, false},
		{"a check minimum does not floor a group save", []string{"check"}, "save:con", false},
		{"a check minimum floors a group check", []string{"check"}, stealthKey, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newArmed(t)
			feat := a.minimumOf10(t, tc.rolls, a.toren)
			asked := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = tc.key })
			got := memberOf(a.mustRollGroup(t, a.caio, asked.GetId(), 4), a.toren.GetId()).GetRoll()
			if tc.counted {
				wantCheckCounted(t, "Toren's 4", got, 4, 10, feat)
			} else {
				wantCheckPlain(t, "Toren's 4", got, 4)
			}
		})
	}
}
