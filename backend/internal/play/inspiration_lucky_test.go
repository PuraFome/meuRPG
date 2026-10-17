package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The halfling's Lucky on the d20 rolls that are not a weapon attack (SRD 5.1, Halfling; the
// weapon attacks are combat_lucky_test.go): the scene check, the group check, the trap search, the
// puzzle hint try, and, in a combat, the spell attack against one target, the effect's saving
// throw and the concentration save. A 1 is held with the question "Tirou 1: rolar de novo?" before
// the result, AnswerOutsideLucky rolls it again (or keeps the 1), and the new roll stands. The
// tests that use the database need MEURPG_TEST_DATABASE_URL. Dalila is a halfling rogue 11
// (Reliable Talent, expertise in Acrobatics, Investigation, Perception and Stealth) who joins the
// table of each fixture; in a combat the party of newCasters is made of halflings.

// joinDalila adds Dalila's player to the campaign with her halfling rogue 11.
func joinDalila(t *testing.T, h *harness, master *user, campaignID string) (*user, string) {
	t.Helper()
	dalila := h.newUser("Dalila")
	h.join(master, campaignID, dalila)
	return dalila, dalila.rogueOf(t, campaignID, "Dalila", "race:halfling", 11).GetId()
}

// answerOutsideLuckyKey calls AnswerOutsideLucky as u with the key given.
func answerOutsideLuckyKey(t *testing.T, u *user, campaignID, holdID, key string, edit func(*playv1.AnswerOutsideLuckyRequest)) (*playv1.AnswerOutsideLuckyResponse, error) {
	t.Helper()
	req := &playv1.AnswerOutsideLuckyRequest{CampaignId: campaignID, HoldId: holdID, IdempotencyKey: key}
	edit(req)
	res, err := u.resource.AnswerOutsideLucky(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// answerOutsideLucky calls AnswerOutsideLucky as u, with a key of its own.
func answerOutsideLucky(t *testing.T, u *user, campaignID, holdID string, edit func(*playv1.AnswerOutsideLuckyRequest)) (*playv1.AnswerOutsideLuckyResponse, error) {
	t.Helper()
	return answerOutsideLuckyKey(t, u, campaignID, holdID, newKey(), edit)
}

func outsideRerollInApp(r *playv1.AnswerOutsideLuckyRequest) {
	r.Reroll, r.Roll = true, &playv1.AnswerOutsideLuckyRequest_RollInApp{RollInApp: true}
}

func outsideRerollTyped(face int32) func(*playv1.AnswerOutsideLuckyRequest) {
	return func(r *playv1.AnswerOutsideLuckyRequest) {
		r.Reroll, r.Roll = true, &playv1.AnswerOutsideLuckyRequest_TypedFace{TypedFace: face}
	}
}

func outsideKeepTheOne(r *playv1.AnswerOutsideLuckyRequest) { r.Reroll = false }

// rollSceneTyped calls RollSceneCheck as u with a typed d20 and the key given.
func (a *armed) rollSceneTyped(t *testing.T, u *user, action, key string, face int32) (*playv1.RollSceneCheckResponse, error) {
	t.Helper()
	res, err := u.play.RollSceneCheck(t.Context(), connect.NewRequest(&playv1.RollSceneCheckRequest{
		CampaignId: a.campaignID, ActionId: action, IdempotencyKey: key, Roll: &playv1.RollSceneCheckRequest_D20Face{D20Face: face},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// wantLuckyOffer checks a held roll's Lucky question: the d20 as it came up, the die that shows
// the 1, and no attack (a roll that is not a weapon attack names none).
func wantLuckyOffer(t *testing.T, what string, offer *playv1.LuckyOffer, faces []int32, index int32) {
	t.Helper()
	if offer == nil {
		t.Fatalf("%s: no Lucky question", what)
	}
	got := offer.GetD20().GetFaces()
	if len(got) != len(faces) || offer.GetLuckyIndex() != index || offer.GetAttackKey() != "" || offer.GetTargetId() != "" || offer.GetHoldId() == "" {
		t.Errorf("%s: the question = %v, want the faces %v and the die %d", what, offer, faces, index)
		return
	}
	for i := range faces {
		if got[i] != faces[i] {
			t.Errorf("%s: the question's faces = %v, want %v", what, got, faces)
		}
	}
}

// A scene check (an ability check, SRD 5.1): Dalila's typed 1 is held with the Lucky question,
// and nothing is written; only she reads the question (RN-10), and her next roll and the die's
// answer wait for it. Rolled again to 6, Reliable Talent counts the new face as 10 (the order the
// rules give: the reroll first, then what the feature does to the d20 that stands): 10 + 12 = 22.
// The same answer again gives the same roll and rolls nothing; another answer is refused; the
// first request again gives the written roll. A human's 1 is never held.
func TestLuckyOnASceneCheck(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, _ := joinDalila(t, a.h, a.master, a.campaignID)
	point, actions := a.h.newScene(a.mapID, "O muro do pátio", true, 3, sceneSpec{key: "skill:acrobatics", dc: new(int32(15))}, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
	a.openScene(t, point)

	rollKey := newKey()
	held, err := a.rollSceneTyped(t, dalila, actions[0], rollKey, 1)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	if held.GetRoll() != nil || held.GetInspirationOffer() != nil {
		t.Fatalf("the held roll = %v, want the Lucky question alone", held)
	}
	offer := held.GetLuckyOffer()
	wantLuckyOffer(t, "the scene check", offer, []int32{1}, 0)
	if n := a.countSceneRolls(t); n != 0 {
		t.Errorf("%d scene rolls written while the question waits, want 0", n)
	}
	// A reload asks again, the roller only; another player reads nothing of it.
	if got := a.outsideState(t, dalila); len(got.GetLuckyOffers()) != 1 || got.GetLuckyOffers()[0].GetHoldId() != offer.GetHoldId() || len(got.GetOffers()) != 0 {
		t.Errorf("Dalila reads %v, want the Lucky question %s alone", got, offer.GetHoldId())
	}
	if got := a.outsideState(t, a.caio); len(got.GetLuckyOffers()) != 0 || len(got.GetOffers()) != 0 {
		t.Errorf("another player reads %v, want no question", got)
	}
	// One question at a time: another roll waits, and the die's answer has no question to answer yet.
	_, err = a.rollSceneTyped(t, dalila, actions[1], newKey(), 12)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING)
	_, err = a.answerOutside(t, dalila, offer.GetHoldId(), false, false, 0)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING)
	// Nobody else answers it, and the master answers none (RN-10, RN-18: the player's own roll).
	if _, err := answerOutsideLucky(t, a.caio, a.campaignID, offer.GetHoldId(), outsideKeepTheOne); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another player's answer = %v, want not_found", err)
	}
	if _, err := answerOutsideLucky(t, a.master, a.campaignID, offer.GetHoldId(), outsideKeepTheOne); err == nil {
		t.Error("the master answered a player's Lucky question")
	}
	if _, err := answerOutsideLucky(t, dalila, a.campaignID, offer.GetHoldId(), outsideRerollTyped(21)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a typed 21 = %v, want invalid_argument", err)
	}

	// Rolled again in the app: a 6, counted as 10.
	answerKey := newKey()
	a.h.roller.queue(6)
	done, err := answerOutsideLuckyKey(t, dalila, a.campaignID, offer.GetHoldId(), answerKey, outsideRerollInApp)
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	roll := done.GetSceneCheck().GetRoll()
	wantTreated(t, "the scene roll after Lucky", roll.GetRoll(), 6, 10)
	wantLuckyReroll(t, "the scene roll", roll.GetRoll(), 0, 6)
	if roll.GetRoll().GetTotal() != 22 || !roll.GetPassed() {
		t.Errorf("the scene roll = %v, want 10 + 12 = 22, a pass against DC 15", roll)
	}
	if n := a.countSceneRolls(t); n != 1 {
		t.Errorf("%d scene rolls after the answer, want 1", n)
	}
	// The same answer again: the same roll, nothing rolled (the roller would give a 10).
	again, err := answerOutsideLuckyKey(t, dalila, a.campaignID, offer.GetHoldId(), answerKey, outsideRerollInApp)
	if err != nil || again.GetSceneCheck().GetRoll().GetRoll().GetFaces()[0] != 6 {
		t.Fatalf("the answer's retry = %v, %v; want the same 6", again, err)
	}
	if _, err := answerOutsideLucky(t, dalila, a.campaignID, offer.GetHoldId(), outsideKeepTheOne); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("another answer = %v, want failed_precondition", err)
	}
	// The first request again: the roll it ended up as, never another question.
	first, err := a.rollSceneTyped(t, dalila, actions[0], rollKey, 1)
	if err != nil || first.GetLuckyOffer() != nil || first.GetRoll().GetRoll().GetFaces()[0] != 6 {
		t.Fatalf("the roll's retry = %v, %v; want the written roll of 6", first, err)
	}
	wantLuckyReroll(t, "the roll's retry", first.GetRoll().GetRoll(), 0, 6)
	if n := a.countSceneRolls(t); n != 1 {
		t.Errorf("%d scene rolls after the retries, want 1", n)
	}

	// A human's 1 is a 1.
	plain, err := a.rollSceneTyped(t, a.caio, actions[0], newKey(), 1)
	if err != nil || plain.GetLuckyOffer() != nil || plain.GetRoll().GetRoll().GetFaces()[0] != 1 || len(plain.GetRoll().GetRoll().GetRerolled()) != 0 {
		t.Errorf("Tavo's 1 = %v, %v; want the roll of 1 and no question", plain, err)
	}
}

// Keeping the 1 writes the roll as it came up (Athletics: no proficiency, so no Reliable Talent),
// and a d20 that shows no 1 is never held: it is written at once, Reliable Talent and all.
func TestLuckyKeepsTheOneAndAnotherFaceGoesOn(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, _ := joinDalila(t, a.h, a.master, a.campaignID)
	point, actions := a.h.newScene(a.mapID, "O muro do pátio", true, 3, sceneSpec{key: "skill:acrobatics", dc: new(int32(15))}, sceneSpec{key: "skill:athletics", dc: new(int32(25))})
	a.openScene(t, point)
	held, err := a.rollSceneTyped(t, dalila, actions[1], newKey(), 1)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	kept, err := answerOutsideLucky(t, dalila, a.campaignID, held.GetLuckyOffer().GetHoldId(), outsideKeepTheOne)
	if err != nil {
		t.Fatalf("AnswerOutsideLucky(keep) error = %v", err)
	}
	if r := kept.GetSceneCheck().GetRoll().GetRoll(); len(r.GetFaces()) != 1 || r.GetFaces()[0] != 1 || len(r.GetRerolled()) != 0 || r.GetTotal() != 1+r.GetModifier() {
		t.Errorf("the kept roll = %v, want the 1 as it came up (no proficiency in Athletics), nothing rolled again", r)
	}
	plain, err := a.rollSceneTyped(t, dalila, actions[0], newKey(), 2)
	if err != nil || plain.GetLuckyOffer() != nil || plain.GetRoll() == nil {
		t.Fatalf("a 2 = %v, %v; want the roll written and no question", plain, err)
	}
	wantTreated(t, "a 2 on Acrobatics", plain.GetRoll().GetRoll(), 2, 10)
}

// Lucky first, then the Bardic Inspiration die, on the new roll: Dalila holds Orla's d8; her 1 asks
// Lucky alone; rolled again to 6, the die's question comes on the 6 (nothing written yet); the die
// adds its 5 to 10 + 12, and the roll keeps what Lucky did.
func TestLuckyThenTheDieOnASceneCheck(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, dalilaID := joinDalila(t, a.h, a.master, a.campaignID)
	if _, err := a.giveOutside(t, a.bia, dalilaID); err != nil {
		t.Fatalf("give error = %v", err)
	}
	point, actions := a.h.newScene(a.mapID, "O muro do pátio", true, 3, sceneSpec{key: "skill:acrobatics", dc: new(int32(25))})
	a.openScene(t, point)
	held, err := a.rollSceneTyped(t, dalila, actions[0], newKey(), 1)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	if held.GetInspirationOffer() != nil {
		t.Errorf("the die's question came with Lucky's: %v", held.GetInspirationOffer())
	}
	wantLuckyOffer(t, "the scene check with a die", held.GetLuckyOffer(), []int32{1}, 0)
	key := newKey()
	next, err := answerOutsideLuckyKey(t, dalila, a.campaignID, held.GetLuckyOffer().GetHoldId(), key, outsideRerollTyped(6))
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	insp := next.GetSceneCheck().GetInspirationOffer()
	if next.GetSceneCheck().GetRoll() != nil || insp == nil || insp.GetSides() != 8 || insp.GetD20().GetFaces()[0] != 6 {
		t.Fatalf("after Lucky = %v, want the d8's question on the 6 and no roll", next.GetSceneCheck())
	}
	wantLuckyReroll(t, "the die's question", insp.GetD20(), 0, 6)
	if n := a.countSceneRolls(t); n != 0 {
		t.Errorf("%d scene rolls while the die's question waits, want 0", n)
	}
	if got := a.outsideState(t, dalila); len(got.GetOffers()) != 1 || len(got.GetLuckyOffers()) != 0 {
		t.Errorf("Dalila reads %v, want the die's question alone", got)
	}
	// The Lucky answer again reads the hold as it is; another Lucky answer is refused.
	if again, err := answerOutsideLuckyKey(t, dalila, a.campaignID, held.GetLuckyOffer().GetHoldId(), key, outsideRerollTyped(6)); err != nil || again.GetSceneCheck().GetInspirationOffer().GetHoldId() != insp.GetHoldId() {
		t.Errorf("the Lucky answer's retry = %v, %v; want the die's question again", again, err)
	}
	if _, err := answerOutsideLucky(t, dalila, a.campaignID, held.GetLuckyOffer().GetHoldId(), outsideRerollTyped(18)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("a second Lucky answer = %v, want failed_precondition", err)
	}
	used, err := a.answerOutside(t, dalila, insp.GetHoldId(), true, false, 5)
	if err != nil {
		t.Fatalf("AnswerOutsideInspiration() error = %v", err)
	}
	roll := used.GetSceneCheck().GetRoll()
	if roll.GetRoll().GetTotal() != 10+12+5 || len(roll.GetBonusDice()) != 1 || roll.GetBonusDice()[0].GetFace() != 5 {
		t.Errorf("the roll = %v, want 10 (the 6, by Reliable Talent) + 12 + the d8's 5", roll)
	}
	wantLuckyReroll(t, "the roll with the die", roll.GetRoll(), 0, 6)
	if n := a.dieRows(t, dalilaID); n != 0 {
		t.Errorf("%d dice after using it, want 0", n)
	}
}

// A group check: the player's own roll asks (in the app, a 1, rolled again in the app to 14); the
// roll the master makes for a halfling never does (it waits for a decision, MR-014).
func TestLuckyOnAGroupCheckAndNotOnTheMastersRoll(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	dalila, dalilaID := joinDalila(t, a.h, a.master, a.campaignID)
	view := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc = 20 })
	held, err := a.rollGroup(t, dalila, view.GetId(), 1)
	if err != nil {
		t.Fatalf("RollGroupCheck() error = %v", err)
	}
	if held.GetGroupCheck() != nil || held.GetInspirationOffer() != nil {
		t.Fatalf("held = %v, want the Lucky question alone", held)
	}
	wantLuckyOffer(t, "the group check", held.GetLuckyOffer(), []int32{1}, 0)
	if m := memberOf(a.groupView(t, a.master), dalilaID); m.GetAnswered() {
		t.Error("the master reads Dalila's roll as answered while it waits")
	}
	a.h.roller.queue(14)
	done, err := answerOutsideLucky(t, dalila, a.campaignID, held.GetLuckyOffer().GetHoldId(), outsideRerollInApp)
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	m := memberOf(done.GetGroupCheck().GetGroupCheck(), dalilaID)
	roll := m.GetRoll()
	if !m.GetAnswered() || len(roll.GetFaces()) != 1 || roll.GetFaces()[0] != 14 || roll.GetTotal() != 14+roll.GetModifier() {
		t.Errorf("the member = %v, want the 14 and the modifier", m)
	}
	if got := roll.GetRerolled(); len(got) != 1 || got[0].GetIndex() != 0 || got[0].GetFrom() != 1 || got[0].GetTo() != 14 || got[0].GetSourceKey() != "trait:lucky" {
		t.Errorf("the member's rerolled = %v, want die 0 from 1 to 14 by Lucky", got)
	}
	if r := memberOf(a.groupView(t, a.master), dalilaID).GetRoll(); len(r.GetRerolled()) != 1 {
		t.Errorf("the master reads %v, want the reroll too", r)
	}
	a.mustCloseGroup(t, view.GetId())

	second := a.mustRequestGroup(t, nil)
	if _, err := a.rollForPlayer(t, a.master, second.GetId(), dalilaID, 1); err != nil {
		t.Fatalf("RollForPlayer() error = %v", err)
	}
	if r := memberOf(a.groupView(t, a.master), dalilaID).GetRoll(); r.GetFaces()[0] != 1 || len(r.GetRerolled()) != 0 || !r.GetRolledByMaster() {
		t.Errorf("the master's roll for Dalila = %v, want the 1 written as it came up", r)
	}
	if got := a.outsideState(t, dalila); len(got.GetLuckyOffers()) != 0 {
		t.Errorf("Dalila reads %v after the master's roll, want no question", got.GetLuckyOffers())
	}
}

// A trap search out of a combat (an ability check): the 1 is held before anything is written,
// and the new face (4, counted as 10 by Reliable Talent: 10 + 10 = 20) finds the pit of DC 14.
// The master's activity shows what Lucky did.
func TestLuckyOnATrapSearch(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	hole := r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc, s.FindDc = 0, 14 })
	dalila, dalilaID := joinDalila(t, r.h, r.master, r.campaignID)
	r.place(t, dalilaID, 8, 7)
	held, err := r.search(t, dalila, investigation, 1)
	if err != nil {
		t.Fatalf("SearchForTraps() error = %v", err)
	}
	if held.GetRoll() != nil || len(held.GetFoundPointIds()) != 0 {
		t.Fatalf("the held search = %v, want the question and nothing found", held)
	}
	wantLuckyOffer(t, "the trap search", held.GetLuckyOffer(), []int32{1}, 0)
	done, err := answerOutsideLucky(t, dalila, r.campaignID, held.GetLuckyOffer().GetHoldId(), outsideRerollTyped(4))
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	res := done.GetTrapSearch()
	wantTreated(t, "the search after Lucky", res.GetRoll(), 4, 10)
	wantLuckyReroll(t, "the search", res.GetRoll(), 0, 4)
	if len(res.GetFoundPointIds()) != 1 || res.GetFoundPointIds()[0] != hole.GetId() {
		t.Errorf("found %v, want the pit", res.GetFoundPointIds())
	}
	var shown *playv1.TrapSearchResult
	for _, act := range r.activity(t, r.master) {
		if act.GetSearch() != nil {
			shown = act.GetSearch()
		}
	}
	if shown == nil {
		t.Fatal("the master's activity has no search")
	}
	wantLuckyReroll(t, "the master's line", shown.GetRoll(), 0, 4)
}

// A puzzle hint try: the 1 is held, rolled again to 4 (10 by Reliable Talent + 10 reaches DC 15),
// and the hint is hers. The first request again (it typed the 1) finds the try it became, and the
// master's run shows the reroll.
func TestLuckyOnAPuzzleHintTry(t *testing.T) {
	t.Parallel()
	p := newPuzzleTable(t)
	dalila, _ := joinDalila(t, p.h, p.master, p.campaignID)
	puz := p.riddle(t, "A porta da Cripta pergunta", hintCheck("skill:investigation", 15))
	p.show(t, puz.GetId())
	key := newKey()
	held, err := p.tryHintKey(t, dalila, puz.GetId(), 1, key)
	if err != nil {
		t.Fatalf("TryPuzzleHint() error = %v", err)
	}
	if held.GetRun() != nil || held.GetRoll() != nil {
		t.Fatalf("the held try = %v, want the question alone", held)
	}
	wantLuckyOffer(t, "the hint try", held.GetLuckyOffer(), []int32{1}, 0)
	done, err := answerOutsideLucky(t, dalila, p.campaignID, held.GetLuckyOffer().GetHoldId(), outsideRerollTyped(4))
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	try := done.GetHintTry()
	if !try.GetPassed() || len(try.GetRun().GetHints()) != 1 {
		t.Errorf("the try = %v, want a pass and the first hint", try)
	}
	wantTreated(t, "the try after Lucky", try.GetRoll(), 4, 10)
	wantLuckyReroll(t, "the try", try.GetRoll(), 0, 4)
	again, err := p.tryHintKey(t, dalila, puz.GetId(), 1, key)
	if err != nil || !again.GetReplayed() || !again.GetPassed() || again.GetRoll().GetFaces()[0] != 4 {
		t.Fatalf("the first request again = %v, %v; want the try replayed, with the 4", again, err)
	}
	if m := p.masterRun(t, puz.GetId()); len(m.GetHintTries()) != 1 {
		t.Errorf("the master reads %d tries, want 1", len(m.GetHintTries()))
	} else {
		wantLuckyReroll(t, "the master's try", m.GetHintTries()[0].GetRoll(), 0, 4)
	}
}

// A spell attack against one target (an attack roll): Brisa, a halfling cleric next to the goblin,
// casts Guiding Bolt with disadvantage (a hostile creature within 5 ft of a ranged attack, SRD
// 5.1): the app rolls 1 and 13, the 1 counts, and the cast is held before anything is spent. Her
// weapon and another cast wait (LUCKY_PENDING). The 1 rolled again to 15, the pair is 15 and 13 and
// the lower, 13, counts.
func TestLuckyOnASpellAttack(t *testing.T) {
	t.Parallel()
	a := newCastersOf(t, 3, "race:halfling")
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 4}},
	})
	a.passTo(t, e, "Brisa")
	e = a.get(t, a.master)
	inApp := func(r *playv1.CastSpellRequest) { r.Roll = &playv1.CastSpellRequest_RollInApp{RollInApp: true} }
	a.h.roller.queue(1, 13)
	res, err := a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), inApp)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	if res.GetCast() != nil || res.GetInspirationOffer() != nil {
		t.Fatalf("the cast = %v, want the Lucky question alone", res)
	}
	wantLuckyOffer(t, "the spell attack", res.GetLuckyOffer(), []int32{1, 13}, 0)
	if d := res.GetLuckyOffer().GetD20(); d.GetCountedIndex() != 0 {
		t.Errorf("the question's counted die = %d, want 0 (the lower, with disadvantage)", d.GetCountedIndex())
	}
	if byLabel(t, a.get(t, a.bia), "Brisa").GetActionUsed() {
		t.Error("the action was spent before the answer")
	}
	if got := a.outsideState(t, a.bia).GetLuckyOffers(); len(got) != 1 {
		t.Errorf("Brisa's player reads %v, want the question", got)
	}
	if got := a.outsideState(t, a.caio).GetLuckyOffers(); len(got) != 0 {
		t.Errorf("another player reads %v, want none", got)
	}
	_, err = a.attack(t, a.bia, e, "Brisa", maceKey, "Goblin", d20(15))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING)
	_, err = a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), inApp)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_LUCKY_PENDING)

	out, err := answerOutsideLucky(t, a.bia, a.campaignID, res.GetLuckyOffer().GetHoldId(), outsideRerollTyped(15))
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	cast := out.GetSpellCast().GetCast()
	if len(cast.GetTargets()) != 1 {
		t.Fatalf("the cast = %v, want one target", cast)
	}
	roll := cast.GetTargets()[0].GetAttackRoll()
	if f := roll.GetFaces(); len(f) != 2 || f[0] != 15 || f[1] != 13 || roll.GetCountedIndex() != 1 || roll.GetTotal() != 13+roll.GetModifier() {
		t.Errorf("the attack roll = %v, want 15 and 13 with the 13 counted", roll)
	}
	wantLuckyReroll(t, "the spell attack", roll, 0, 15)
	if !byLabel(t, a.get(t, a.bia), "Brisa").GetActionUsed() {
		t.Error("the action is not spent after the answer")
	}
}

// A concentration save (a saving throw): Pensantus, a halfling, types a 1; the window stays open
// with the question; rolled again to 12, 12 + 1 keeps the concentration (DC 10), and the log line
// says what Lucky did. A gnome's 1 is never asked about.
func TestLuckyOnAConcentrationSave(t *testing.T) {
	t.Parallel()
	a, e := concentratingHitOn(t, newCastersOf(t, 3, "race:halfling"), 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatal("Pensantus has no concentration window")
	}
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(1))
	if err != nil {
		t.Fatalf("ResolveConcentrationSave() error = %v", err)
	}
	if res.GetResult() != nil || res.GetInspirationOffer() != nil {
		t.Fatalf("the save = %v, want the Lucky question alone", res)
	}
	wantLuckyOffer(t, "the concentration save", res.GetLuckyOffer(), []int32{1}, 0)
	if a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE) == nil {
		t.Error("the window closed before the player answered")
	}
	out, err := answerOutsideLucky(t, a.ana, a.campaignID, res.GetLuckyOffer().GetHoldId(), outsideRerollTyped(12))
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	r := out.GetConcentrationSave().GetResult()
	if !r.GetKept() || r.GetSave().GetFaces()[0] != 12 || r.GetSave().GetTotal() != 13 {
		t.Errorf("the result = %v, want a kept save of 12 + 1", r)
	}
	wantLuckyReroll(t, "the concentration save", r.GetSave(), 0, 12)
	found := false
	for _, l := range logEntries(a.log(t, a.ana, e)) {
		found = found || strings.Contains(l.GetReactionTextPt(), "Sortudo: 1 → 12.")
	}
	if !found {
		t.Error("Pensantus's player reads no log line with \"Sortudo: 1 → 12.\"")
	}

	g, ge := concentratingHit(t, 6)
	gw := g.windowOf(t, g.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	plain, err := g.resolveConcentration(t, g.ana, ge, gw.GetId(), concentrationFace(1))
	if err != nil || plain.GetLuckyOffer() != nil || plain.GetResult() == nil {
		t.Errorf("the gnome's 1 = %v, %v; want the result and no question", plain, err)
	}
}

// The master's roll of a halfling's concentration save never asks: the 1 stands (a decision waits
// for these, MR-014).
func TestTheMastersSaveForAHalflingNeverAsksLucky(t *testing.T) {
	t.Parallel()
	a, e := concentratingHitOn(t, newCastersOf(t, 3, "race:halfling"), 6)
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	res, err := a.resolveConcentration(t, a.master, e, w.GetId(), concentrationFace(1))
	if err != nil || res.GetLuckyOffer() != nil || res.GetResult() == nil {
		t.Fatalf("ResolveConcentrationSave(master) = %v, %v; want the result and no question", res, err)
	}
	if r := res.GetResult(); r.GetKept() || r.GetSave().GetFaces()[0] != 1 || len(r.GetSave().GetRerolled()) != 0 {
		t.Errorf("the result = %v, want the failed 1, nothing rolled again", r)
	}
}

// An effect's saving throw (Hold Person on Toren, a halfling, at the end of his turn): the typed 1
// is held with the window open; rolled again in the app to 17, the result and the log line keep
// what Lucky did.
func TestLuckyOnAnEffectSave(t *testing.T) {
	t.Parallel()
	a := newCastersOf(t, 3, "race:halfling")
	e := a.closeFight(t)
	a.mustAddEffect(t, e, holdPerson, []string{"Toren"}, a.rounds(t, 10, "Toren"))
	a.passTo(t, e, "Toren")
	if _, err := a.endTurnRaw(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	w := a.windowOf(t, a.caio, playv1.ReactionKind_REACTION_KIND_EFFECT_SAVE)
	if w == nil {
		t.Fatalf("Toren's windows = %v, want the save of Hold Person", a.windows(t, a.caio))
	}
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), effectSaveFace(1))
	if err != nil {
		t.Fatalf("RollEffectSave() error = %v", err)
	}
	if res.GetResult() != nil || res.GetInspirationOffer() != nil {
		t.Fatalf("the save = %v, want the Lucky question alone", res)
	}
	wantLuckyOffer(t, "the effect save", res.GetLuckyOffer(), []int32{1}, 0)
	a.h.roller.queue(17)
	out, err := answerOutsideLucky(t, a.caio, a.campaignID, res.GetLuckyOffer().GetHoldId(), outsideRerollInApp)
	if err != nil {
		t.Fatalf("AnswerOutsideLucky() error = %v", err)
	}
	r := out.GetEffectSave().GetResult()
	if r.GetD20() != 17 || len(r.GetD20Faces()) != 1 || r.GetD20Faces()[0] != 17 {
		t.Errorf("the result = %v, want the 17", r)
	}
	if got := r.GetRerolled(); len(got) != 1 || got[0].GetIndex() != 0 || got[0].GetFrom() != 1 || got[0].GetTo() != 17 || got[0].GetSourceKey() != "trait:lucky" {
		t.Errorf("the result's rerolled = %v, want die 0 from 1 to 17 by Lucky", got)
	}
	var line *playv1.CombatLogEffect
	for _, l := range logEntries(a.log(t, a.caio, e)) {
		if l.GetEffect().GetD20() == 17 {
			line = l.GetEffect()
		}
	}
	if line == nil || len(line.GetRerolled()) != 1 || line.GetRerolled()[0].GetTo() != 17 {
		t.Errorf("Toren's player's log line = %v, want the save with the reroll", line)
	}
}

// A hint try keeps the die Lucky rolled again: the faces are the new ones, the 1 is put back to
// match the request that typed it, and the roll says what Lucky did.
func TestAHintTryKeepsTheLuckyReroll(t *testing.T) {
	t.Parallel()
	one := playdb.PuzzleHintTry{D20: 4, Modifier: 10, Total: 20, Physical: true, LuckyIndex: new(int32(0))}
	if !sameTypedFaces(one, rollInput{typed: 1}) || sameTypedFaces(one, rollInput{typed: 4}) {
		t.Error("a single d20: want the typed 1 to match the try, and the 4 not")
	}
	wantLuckyReroll(t, "a single d20", hintRollProto(one), 0, 4)
	// With disadvantage the dice were 15 and 1, then 15 and 13: the 13 counts (D20) and was second.
	two := playdb.PuzzleHintTry{D20: 13, D20B: 15, Counted: 1, Modifier: 2, Total: 15, Physical: true, LuckyIndex: new(int32(1))}
	if !sameTypedFaces(two, rollInput{typedFaces: []int{15, 1}}) || sameTypedFaces(two, rollInput{typedFaces: []int{15, 13}}) {
		t.Error("a pair: want the typed 15 and 1 to match the try, and 15 and 13 not")
	}
	wantLuckyReroll(t, "a pair", hintRollProto(two), 1, 13)
	plain := playdb.PuzzleHintTry{D20: 7, Modifier: 2, Total: 9, Physical: true}
	if !sameTypedFaces(plain, rollInput{typed: 7}) || len(hintRollProto(plain).GetRerolled()) != 0 {
		t.Error("a try without Lucky: want the typed 7 to match and nothing rolled again")
	}
}

// The text log lines add "Sortudo: 1 → 14." for the d20 Lucky rolled again, and nothing otherwise.
func TestLuckyNote(t *testing.T) {
	t.Parallel()
	if got := luckyNote(&luckyReroll{Index: 0, From: 1, To: 14}); got != " Sortudo: 1 → 14." {
		t.Errorf("luckyNote = %q, want %q", got, " Sortudo: 1 → 14.")
	}
	if got := luckyNote(nil); got != "" {
		t.Errorf("luckyNote(nil) = %q, want empty", got)
	}
}
