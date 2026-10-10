package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// giveOutside calls GiveBardicInspirationOutside as u, to the named character.
func (a *armed) giveOutside(t *testing.T, u *user, targetID string) (*playv1.GiveBardicInspirationOutsideResponse, error) {
	t.Helper()
	res, err := u.resource.GiveBardicInspirationOutside(t.Context(), connect.NewRequest(&playv1.GiveBardicInspirationOutsideRequest{
		CampaignId: a.campaignID, TargetCharacterId: targetID, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) outsideState(t *testing.T, u *user) *playv1.GetOutsideInspirationResponse {
	t.Helper()
	res, err := u.resource.GetOutsideInspiration(t.Context(), connect.NewRequest(&playv1.GetOutsideInspirationRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("GetOutsideInspiration() error = %v", err)
	}
	return res.Msg
}

func (a *armed) rollSceneApp(t *testing.T, u *user, action string, face int) (*playv1.RollSceneCheckResponse, error) {
	t.Helper()
	a.h.roller.queue(face)
	res, err := u.play.RollSceneCheck(t.Context(), connect.NewRequest(&playv1.RollSceneCheckRequest{
		CampaignId: a.campaignID, ActionId: action, IdempotencyKey: newKey(), Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) answerOutside(t *testing.T, u *user, hold string, use bool, inApp bool, typed int32) (*playv1.AnswerOutsideInspirationResponse, error) {
	t.Helper()
	req := &playv1.AnswerOutsideInspirationRequest{CampaignId: a.campaignID, HoldId: hold, Use: use, IdempotencyKey: newKey()}
	switch {
	case use && inApp:
		req.Roll = &playv1.AnswerOutsideInspirationRequest_RollInApp{RollInApp: true}
	case use:
		req.Roll = &playv1.AnswerOutsideInspirationRequest_TypedFace{TypedFace: typed}
	}
	res, err := u.resource.AnswerOutsideInspiration(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) usesLeft(t *testing.T) int32 {
	t.Helper()
	left, _ := poolOf(a.vitalsOfCharacter(t, a.bri), rules.BardicInspirationKey)
	return left
}

func (a *armed) dieRows(t *testing.T, characterID string) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM character_inspiration WHERE character_id = $1`, characterID).Scan(&n); err != nil {
		t.Fatalf("count the dice: %v", err)
	}
	return n
}

// Giving the die out of a combat spends one use and stores a d8 (a level 5 bard) on the target
// character for 10 minutes; a second die is refused; the bard, a second use of the same key and
// a combat in progress behave as in the rules.
func TestBardicInspirationOutsideSpendsAUseAndStoresTheDie(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	key := newKey()
	req := &playv1.GiveBardicInspirationOutsideRequest{CampaignId: a.campaignID, TargetCharacterId: a.toren.GetId(), IdempotencyKey: key}
	res, err := a.bia.resource.GiveBardicInspirationOutside(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("GiveBardicInspirationOutside() error = %v", err)
	}
	if d := res.Msg.GetDie(); d.GetSides() != 8 || d.GetSecondsLeft() != 600 || d.GetFromName() != "Orla" {
		t.Errorf("die = %v, want a d8 from Orla with 600 seconds", d)
	}
	if got := a.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
	// The same key again spends nothing; another target with the key is refused.
	if _, err := a.bia.resource.GiveBardicInspirationOutside(t.Context(), connect.NewRequest(req)); err != nil {
		t.Fatalf("retry error = %v", err)
	}
	if got := a.usesLeft(t); got != 2 {
		t.Errorf("uses left after the retry = %d, want 2", got)
	}
	req.TargetCharacterId = a.pens.GetId()
	if _, err := a.bia.resource.GiveBardicInspirationOutside(t.Context(), connect.NewRequest(req)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("the key reused for another target = %v, want invalid_argument", err)
	}
	// One die at a time; not the bard itself; not a player without the feature.
	_, err = a.giveOutside(t, a.bia, a.toren.GetId())
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED)
	_, err = a.giveOutside(t, a.bia, a.bri.GetId())
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_TARGET_REFUSED)
	_, err = a.giveOutside(t, a.caio, a.pens.GetId())
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE)
	if got := a.usesLeft(t); got != 2 {
		t.Errorf("uses left after the refusals = %d, want 2", got)
	}
	// Who reads what: the holder their die, the bard the party, the master every die.
	if d := a.outsideState(t, a.caio).GetMine(); d.GetSides() != 8 || d.GetFromName() != "Orla" {
		t.Errorf("Tavo's player reads %v, want the d8 from Orla", d)
	}
	if d := a.outsideState(t, a.ana).GetMine(); d != nil {
		t.Errorf("Nael's player reads %v, want no die", d)
	}
	bard := a.outsideState(t, a.bia)
	if !bard.GetIsBard() || bard.GetUsesLeft() != 2 || bard.GetUsesMax() != 3 || len(bard.GetTargets()) != 2 {
		t.Errorf("the bard reads %v, want a bard with 2 of 3 uses and 2 characters to give to", bard)
	}
	if got := len(a.outsideState(t, a.master).GetHeld()); got != 1 {
		t.Errorf("the master reads %d dice, want 1", got)
	}
}

// A check the master asked outside a combat: the d20 is rolled and kept, the player decides, and
// using the die adds its face and loses the die; keeping it adds nothing and keeps the die.
func TestBardicInspirationOutsideUsedOnASceneCheckAddsTheFaceAndIsGone(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	if _, err := a.giveOutside(t, a.bia, a.toren.GetId()); err != nil {
		t.Fatalf("give error = %v", err)
	}
	point, actions := a.h.newScene(a.mapID, "Ponte", true, 3, sceneSpec{key: "skill:perception", dc: new(int32(15))}, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
	a.openScene(t, point)
	// The roll is held: no result, no event; the offer shows the d20.
	held, err := a.rollSceneApp(t, a.caio, actions[0], 7)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	offer := held.GetInspirationOffer()
	if held.GetRoll() != nil || offer == nil || offer.GetSides() != 8 || offer.GetD20().GetFaces()[0] != 7 {
		t.Fatalf("held = %v, want the offer of a d8 over a 7 and no roll", held)
	}
	if n := a.countSceneRolls(t); n != 0 {
		t.Errorf("%d scene rolls written while the roll waits, want 0", n)
	}
	// Another roll while the question is open is refused.
	_, err = a.caio.play.RollSceneCheck(t.Context(), connect.NewRequest(&playv1.RollSceneCheckRequest{
		CampaignId: a.campaignID, ActionId: actions[1], IdempotencyKey: newKey(), Roll: &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true},
	}))
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING)
	// Nobody else answers it.
	if _, err := a.answerOutside(t, a.ana, offer.GetHoldId(), false, false, 0); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another player answering = %v, want not_found", err)
	}
	// Use the die: the app rolls a 5.
	a.h.roller.queue(5)
	done, err := a.answerOutside(t, a.caio, offer.GetHoldId(), true, true, 0)
	if err != nil {
		t.Fatalf("AnswerOutsideInspiration() error = %v", err)
	}
	roll := done.GetSceneCheck().GetRoll()
	base := roll.GetRoll().GetModifier()
	if roll.GetRoll().GetTotal() != 7+base+5 || len(roll.GetBonusDice()) != 1 || roll.GetBonusDice()[0].GetFace() != 5 || roll.GetBonusDice()[0].GetSides() != 8 {
		t.Errorf("roll = %v, want 7 + %d + the d8 (5) with the bonus die shown", roll, base)
	}
	if n := a.dieRows(t, a.toren.GetId()); n != 0 {
		t.Errorf("%d dice held after using it, want 0", n)
	}
	if a.outsideState(t, a.caio).GetMine() != nil {
		t.Error("Tavo still reads a die after using it")
	}
	// The next roll is plain.
	plain, err := a.rollSceneApp(t, a.caio, actions[1], 9)
	if err != nil || plain.GetRoll() == nil || plain.GetInspirationOffer() != nil {
		t.Fatalf("plain roll = %v, %v", plain, err)
	}
	// Answering again with another key: the roll was answered.
	if _, err := a.answerOutside(t, a.caio, offer.GetHoldId(), true, true, 0); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("answering twice = %v, want failed_precondition", err)
	}
}

// Keeping the die: the roll is written without it, and the die stays for the next check. A typed
// face (physical dice) is taken as it is and refused outside the die.
func TestBardicInspirationOutsideKeptThenUsedWithTypedFace(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	if _, err := a.giveOutside(t, a.bia, a.toren.GetId()); err != nil {
		t.Fatalf("give error = %v", err)
	}
	point, actions := a.h.newScene(a.mapID, "Ponte", true, 3, sceneSpec{key: "skill:perception", dc: new(int32(15))}, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
	a.openScene(t, point)
	held, err := a.rollSceneApp(t, a.caio, actions[0], 7)
	if err != nil {
		t.Fatalf("RollSceneCheck() error = %v", err)
	}
	kept, err := a.answerOutside(t, a.caio, held.GetInspirationOffer().GetHoldId(), false, false, 0)
	if err != nil {
		t.Fatalf("keep error = %v", err)
	}
	if r := kept.GetSceneCheck().GetRoll(); len(r.GetBonusDice()) != 0 || r.GetRoll().GetTotal() != 7+r.GetRoll().GetModifier() {
		t.Errorf("kept roll = %v, want no bonus die", r)
	}
	if n := a.dieRows(t, a.toren.GetId()); n != 1 {
		t.Errorf("%d dice held after keeping it, want 1", n)
	}
	held, err = a.rollSceneApp(t, a.caio, actions[1], 4)
	if err != nil || held.GetInspirationOffer() == nil {
		t.Fatalf("second roll = %v, %v, want a new offer", held, err)
	}
	if _, err := a.answerOutside(t, a.caio, held.GetInspirationOffer().GetHoldId(), true, false, 9); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a 9 on a d8 = %v, want invalid_argument", err)
	}
	used, err := a.answerOutside(t, a.caio, held.GetInspirationOffer().GetHoldId(), true, false, 6)
	if err != nil {
		t.Fatalf("use error = %v", err)
	}
	if r := used.GetSceneCheck().GetRoll(); r.GetRoll().GetTotal() != 4+r.GetRoll().GetModifier()+6 || r.GetBonusDice()[0].GetFace() != 6 {
		t.Errorf("used roll = %v, want the typed 6 added", r)
	}
}

// The die goes onto the combatant when a combat takes the character in, once, and back to the
// character when the combat ends with what time it has left.
func TestBardicInspirationOutsideCarriesIntoACombatAndBackWithoutDuplicating(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	if _, err := a.giveOutside(t, a.bia, a.toren.GetId()); err != nil {
		t.Fatalf("give error = %v", err)
	}
	a.advanceGameTime(t, 120) // 8 minutes left
	e := a.resourceFight(t)
	if n := a.dieRows(t, a.toren.GetId()); n != 0 {
		t.Errorf("%d dice on the character during the combat, want 0: it is the combatant's", n)
	}
	d := byLabel(t, a.get(t, a.caio), "Tavo").GetInspirationDie()
	if d == nil || d.GetSides() != 8 || d.GetFromLabel() != "Orla" || d.GetExpiresAtRound() != e.GetRound()+80 {
		t.Errorf("Tavo's combat die = %v, want a d8 from Orla that ends in round %d", d, e.GetRound()+80)
	}
	if byLabel(t, a.get(t, a.ana), "Tavo").GetInspirationDie() != nil {
		t.Error("another player reads Tavo's die")
	}
	// A second die is refused in the combat too: Tavo holds one.
	// (the in-combat refusal reads the combatant's die)
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	var left int32
	if err := a.h.pool.QueryRow(t.Context(), `SELECT seconds_left FROM character_inspiration WHERE character_id = $1`, a.toren.GetId()).Scan(&left); err != nil {
		t.Fatalf("the die did not go back to the character: %v", err)
	}
	if left < 474 || left > 480 || left%6 != 0 {
		t.Errorf("seconds left after the combat = %d, want about 480 in whole rounds", left)
	}
}

// 10 minutes of game time and the die is gone; a long rest gives the uses back.
func TestBardicInspirationOutsideExpiresAfterTenMinutesAndUsesComeBackOnALongRest(t *testing.T) {
	t.Parallel()
	a := newResourceTable(t)
	for _, target := range []string{a.toren.GetId(), a.pens.GetId()} {
		if _, err := a.giveOutside(t, a.bia, target); err != nil {
			t.Fatalf("give error = %v", err)
		}
	}
	a.advanceGameTime(t, 599)
	if n := a.dieRows(t, a.toren.GetId()); n != 1 {
		t.Fatalf("%d dice after 599 seconds, want 1", n)
	}
	if got := a.outsideState(t, a.caio).GetMine().GetSecondsLeft(); got != 1 {
		t.Errorf("seconds left = %d, want 1", got)
	}
	a.advanceGameTime(t, 1)
	if n := a.dieRows(t, a.toren.GetId()) + a.dieRows(t, a.pens.GetId()); n != 0 {
		t.Errorf("%d dice after 10 minutes, want 0", n)
	}
	// Expired: the target takes a new one (uses left 1), and the check is plain.
	if _, err := a.giveOutside(t, a.bia, a.toren.GetId()); err != nil {
		t.Fatalf("give after expiry error = %v", err)
	}
	_, err := a.giveOutside(t, a.bia, a.pens.GetId())
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT)
	if _, err := a.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: a.campaignID, Kind: playv1.RestKind_REST_KIND_LONG, IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("TakeRest() error = %v", err)
	}
	if got := a.usesLeft(t); got != 3 {
		t.Errorf("uses left after a long rest = %d, want 3", got)
	}
	if n := a.dieRows(t, a.toren.GetId()); n != 0 {
		t.Errorf("%d dice after a long rest, want 0: the rest outlasts 10 minutes", n)
	}
}

func (a *armed) countSceneRolls(t *testing.T) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM session_events e JOIN game_sessions g ON g.id = e.game_session_id WHERE g.campaign_id = $1 AND e.kind = 'scene_check_rolled'`, a.campaignID).Scan(&n); err != nil {
		t.Fatalf("count the scene rolls: %v", err)
	}
	return n
}
