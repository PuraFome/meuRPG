package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Surprise (SRD 5.1, "Surprise"). These tests need the database (MEURPG_TEST_DATABASE_URL). The
// fixture is newSneaks in the arena of combat_contests_test.go, left in SETUP: the master marks
// who is surprised, then the combat begins. The order is Toren, Brisa, Pensantus, the Hobgoblin,
// the Goblin.

// setSurprised calls SetSurprised as u.
func (c *cx) setSurprised(t *testing.T, u *user, label string, on bool) (*playv1.SetSurprisedResponse, error) {
	t.Helper()
	res, err := u.contests.SetSurprised(t.Context(), connect.NewRequest(&playv1.SetSurprisedRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, label), Surprised: on,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cx) mustSetSurprised(t *testing.T, label string, on bool) *playv1.SurpriseView {
	t.Helper()
	res, err := c.setSurprised(t, c.master, label, on)
	if err != nil {
		t.Fatalf("SetSurprised(%s, %v) error = %v", label, on, err)
	}
	return res.GetSurprise()
}

// suggestions calls GetSurpriseSuggestion as u.
func (c *cx) suggestions(t *testing.T, u *user) (*playv1.GetSurpriseSuggestionResponse, error) {
	t.Helper()
	res, err := u.contests.GetSurpriseSuggestion(t.Context(), connect.NewRequest(&playv1.GetSurpriseSuggestionRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId()}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// suggestionFor is the suggestion for one creature, by label.
func (c *cx) suggestionFor(t *testing.T, r *playv1.GetSurpriseSuggestionResponse, label string) *playv1.SurpriseSuggestion {
	t.Helper()
	for _, s := range r.GetSuggestions() {
		if s.GetCombatantId() == c.id(t, label) {
			return s
		}
	}
	t.Fatalf("no suggestion for %s: %v", label, r)
	return nil
}

func (c *cx) surprisedAs(t *testing.T, u *user) []string {
	t.Helper()
	return c.state(t, u).GetSurprise().GetCombatantIds()
}

// TestSurprisedCreatureDoesNothingInItsFirstTurnAndActsFromTheNext (SRD 5.1, "Surprise": a surprised
// creature cannot move or take an action on its first turn, and cannot take a reaction until that
// turn ends; the app also takes the bonus action): its turn options are disabled with SURPRISED, every
// write of the turn is refused with ContestBlocked SURPRISED, it can pass the turn, and in the next
// round it acts. The player reads only their own state.
func TestSurprisedCreatureDoesNothingInItsFirstTurnAndActsFromTheNext(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{setup: true})
	toren := c.id(t, "Toren")

	view := c.mustSetSurprised(t, "Toren", true)
	if !slices.Equal(view.GetCombatantIds(), []string{toren}) {
		t.Fatalf("the master reads %v surprised, want Toren", view.GetCombatantIds())
	}
	for who, want := range map[string][]string{"the master": {toren}, "Toren's player": {toren}, "Pensantus's player": nil, "Brisa's player": nil} {
		u := map[string]*user{"the master": a.master, "Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia}[who]
		if got := c.surprisedAs(t, u); !slices.Equal(got, want) {
			t.Errorf("%s reads %v surprised, want %v (a player reads only their own)", who, got, want)
		}
	}
	// A retry of the same mark is the same; taking it off and putting it back is the master's right until the combat begins.
	if view := c.mustSetSurprised(t, "Toren", true); len(view.GetCombatantIds()) != 1 {
		t.Errorf("marking twice = %v, want one mark", view.GetCombatantIds())
	}
	c.mustSetSurprised(t, "Pensantus", true)
	if view := c.mustSetSurprised(t, "Pensantus", false); !slices.Equal(view.GetCombatantIds(), []string{toren}) {
		t.Errorf("after taking the mark off Pensantus = %v, want Toren alone", view.GetCombatantIds())
	}

	c.e = a.begin(t, c.e)
	opts := a.mustOptions(t, a.caio, c.e, "Toren")
	if !opts.GetYourTurn() || !opts.GetContestState().GetSurprised() {
		t.Fatalf("Toren's options = %v, want his turn and the surprise in the contest state", opts)
	}
	all := slices.Concat(opts.GetOptions().GetStandardActions(), opts.GetOptions().GetFeatureActions())
	if len(all) == 0 || len(opts.GetOptions().GetAttacks()) == 0 {
		t.Fatalf("Toren's options have %d actions and %d attacks, want some to disable", len(all), len(opts.GetOptions().GetAttacks()))
	}
	for _, o := range all {
		if o.GetEnabled() || o.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_SURPRISED {
			t.Errorf("action %v, want it disabled with SURPRISED", o.GetAction().GetKey())
		}
	}
	for _, o := range opts.GetOptions().GetAttacks() {
		if o.GetEnabled() || o.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_SURPRISED {
			t.Errorf("attack %v, want it disabled with SURPRISED", o.GetAttack().GetKey())
		}
	}
	// Nobody else is told: Pensantus's player reads no surprise in Toren's options (they are not theirs to read).
	if _, err := a.options(t, a.ana, c.e, "Toren"); err == nil {
		t.Error("Pensantus's player read the options of Toren's turn")
	}

	// Every write of the turn is refused for the same reason.
	_, err := a.attack(t, a.caio, c.e, "Toren", rapier, c.hob, d20(15))
	wantContestBlocked(t, "an attack while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	_, err = a.action(t, a.caio, c.e, "Toren", "standard:dodge")
	wantContestBlocked(t, "Dodge while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	_, err = a.action(t, a.master, c.e, "Toren", "standard:dodge")
	wantContestBlocked(t, "Dodge for a surprised creature, as the master", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	_, err = c.hide(t, a.caio, "Toren", cunningHide, 10)
	wantContestBlocked(t, "Hide while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	_, err = c.helpCheck(t, a.caio, "Toren", "Brisa", stealthKey)
	wantContestBlocked(t, "Help while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	_, err = c.grapple(t, a.caio, "Toren", c.hob, 10)
	wantContestBlocked(t, "a grapple while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	_, err = c.moveTo(t, a.caio, "Toren", 3, 2)
	wantContestBlocked(t, "moving while surprised", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_SURPRISED)
	if got := c.square(t, "Toren"); got != [2]int32{3, 3} {
		t.Errorf("a refused move took Toren to %v", got)
	}

	// The turn can be passed, and that ends the surprise: the mark cannot be set again.
	a.mustEndTurn(t, a.caio, c.e)
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if got := c.surprisedAs(t, u); len(got) != 0 {
			t.Errorf("%s reads %v surprised after Toren's turn, want nobody", who, got)
		}
	}
	_, err = c.setSurprised(t, a.master, "Toren", true)
	wantContestBlocked(t, "marking a creature whose first turn ended", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_COMBAT_BEGUN)

	c.advance(t, "Goblin")
	c.advance(t, "Toren")
	if o := a.mustOptions(t, a.caio, c.refresh(t), "Toren"); o.GetContestState().GetSurprised() || !attackOption(o, rapier).GetEnabled() {
		t.Fatalf("Toren's options in round 2 = %v, want him to act", o)
	}
	a.mustAttack(t, a.caio, c.e, "Toren", rapier, c.hob, d20(15))
	if got := c.square(t, "Toren"); got != [2]int32{3, 3} {
		t.Errorf("Toren stands on %v", got)
	}
	if _, err := c.moveTo(t, a.caio, "Toren", 3, 2); err != nil {
		t.Errorf("moving in round 2: %v", err)
	}
}

// TestSurprisedCreatureTakesNoOpportunityAttackUntilItsFirstTurnEnds (SRD 5.1, "Surprise": no
// reaction until that turn ends): Toren leaves the Hobgoblin's reach; the Hobgoblin is offered an
// opportunity attack, unless it is surprised.
func TestSurprisedCreatureTakesNoOpportunityAttackUntilItsFirstTurnEnds(t *testing.T) {
	t.Parallel()
	for name, surprised := range map[string]bool{"surprised": true, "not surprised": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newSneaks(t)
			c := a.arena(t, arenaPlan{setup: true})
			if surprised {
				c.mustSetSurprised(t, c.hob, true)
			}
			c.e = a.begin(t, c.e)
			if _, err := c.moveTo(t, a.caio, "Toren", 3, 1); err != nil {
				t.Fatalf("Toren leaving the reach: %v", err)
			}
			offers := c.get(t, a.master).GetOpportunityOffers()
			switch {
			case surprised && len(offers) != 0:
				t.Errorf("a surprised Hobgoblin is offered %v, want no reaction", offers)
			case !surprised && len(offers) == 0:
				t.Error("the Hobgoblin that is not surprised is offered no opportunity attack (the control of this test)")
			}
		})
	}
}

// TestSurpriseIsMarkedByTheMasterOnlyAndAnswersWhatIsNotFound: only the master marks, a creature that
// is not in the combat is not found, an ended combat is not changed, and the mark is idempotent.
func TestSurpriseIsMarkedByTheMasterOnlyAndAnswersWhatIsNotFound(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{setup: true})

	if _, err := c.setSurprised(t, a.caio, "Toren", true); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player marking their own character = %v, want permission_denied", err)
	}
	if got := c.surprisedAs(t, a.master); len(got) != 0 {
		t.Fatalf("a refused mark left %v", got)
	}
	_, err := a.master.contests.SetSurprised(t.Context(), connect.NewRequest(&playv1.SetSurprisedRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: newKey(), Surprised: true,
	}))
	wantCode(t, "a creature that is not in the combat", err, connect.CodeNotFound)

	mark := &playv1.SetSurprisedRequest{CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, "Brisa"), Surprised: true}
	if _, err := a.master.contests.SetSurprised(t.Context(), connect.NewRequest(mark)); err != nil {
		t.Fatalf("SetSurprised() error = %v", err)
	}
	if res, err := a.master.contests.SetSurprised(t.Context(), connect.NewRequest(mark)); err != nil || len(res.Msg.GetSurprise().GetCombatantIds()) != 1 {
		t.Errorf("SetSurprised(retry) = %v, %v, want the same single mark", res, err)
	}
	flipped := &playv1.SetSurprisedRequest{CampaignId: mark.GetCampaignId(), EncounterId: mark.GetEncounterId(), IdempotencyKey: mark.GetIdempotencyKey(), CombatantId: mark.GetCombatantId()}
	_, err = a.master.contests.SetSurprised(t.Context(), connect.NewRequest(flipped))
	wantCode(t, "the same key with the opposite mark", err, connect.CodeInvalidArgument)
	if got := c.surprisedAs(t, a.master); len(got) != 1 {
		t.Errorf("the master reads %v surprised after the conflict, want Brisa alone", got)
	}
}

// TestSurpriseSuggestionComparesEachHidersStealthWithEachCreaturesPassivePerception (SRD 5.1,
// "Surprise": a creature that notices no threat is surprised; the app compares each hider's Stealth
// with each creature's passive Perception, not the best total of the group). The party sneaks up
// with a group check of Stealth: Toren 15 (+3), Pensantus 12 (+2) and Brisa 8 (+2). The Hobgoblin is poisoned,
// so its passive Perception is 5 and every total beats it; the Goblin's is 10, and Brisa's 8 does
// not.
func TestSurpriseSuggestionComparesEachHidersStealthWithEachCreaturesPassivePerception(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{setup: true})
	a.setConditions(t, c.e, c.hob, "condition:poisoned")

	// Nobody has hidden yet: the creatures notice the party.
	before, err := c.suggestions(t, a.master)
	if err != nil {
		t.Fatalf("GetSurpriseSuggestion() error = %v", err)
	}
	if s := c.suggestionFor(t, before, c.hob); s.GetSuggested() || s.GetReason() != playv1.SurpriseSuggestionReason_SURPRISE_SUGGESTION_REASON_NO_HIDERS || s.GetPassivePerception() != 5 {
		t.Errorf("the suggestion for the Hobgoblin with nobody hiding = %v, want no surprise (no hiders), passive 5", s)
	}

	asked := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc = 0 })
	a.mustRollGroup(t, a.caio, asked.GetId(), 12) // +3 = 15
	a.mustRollGroup(t, a.ana, asked.GetId(), 10)  // +2 = 12
	a.mustRollGroup(t, a.bia, asked.GetId(), 6)   // +2 = 8

	got, err := c.suggestions(t, a.master)
	if err != nil {
		t.Fatalf("GetSurpriseSuggestion() error = %v", err)
	}
	if len(got.GetSuggestions()) != 5 {
		t.Fatalf("%d suggestions, want one for each of the five combatants", len(got.GetSuggestions()))
	}
	h := c.suggestionFor(t, got, c.hob)
	if !h.GetSuggested() || h.GetReason() != playv1.SurpriseSuggestionReason_SURPRISE_SUGGESTION_REASON_HIDERS_BEAT || h.GetPassivePerception() != 5 || len(h.GetHiders()) != 3 {
		t.Errorf("the suggestion for the poisoned Hobgoblin = %v, want it surprised: every hider beats its 5", h)
	}
	g := c.suggestionFor(t, got, "Goblin")
	if g.GetSuggested() || g.GetReason() != playv1.SurpriseSuggestionReason_SURPRISE_SUGGESTION_REASON_NOTICES || g.GetPassivePerception() != 10 {
		t.Errorf("the suggestion for the Goblin = %v, want it noticing: Brisa's 8 does not beat its 10", g)
	}
	beats := map[string]bool{}
	totals := map[string]int32{}
	for _, ht := range g.GetHiders() {
		beats[ht.GetCombatantId()], totals[ht.GetCombatantId()] = ht.GetBeats(), ht.GetStealthTotal()
	}
	if !beats[c.id(t, "Toren")] || !beats[c.id(t, "Pensantus")] || beats[c.id(t, "Brisa")] || totals[c.id(t, "Brisa")] != 8 || totals[c.id(t, "Toren")] != 15 {
		t.Errorf("the Goblin's hiders = %v, want Toren 15 and Pensantus 12 beating it, Brisa 8 not", g.GetHiders())
	}
	// The party has no one hiding from it: the players' side is suggested nothing.
	if p := c.suggestionFor(t, got, "Toren"); p.GetSuggested() || p.GetReason() != playv1.SurpriseSuggestionReason_SURPRISE_SUGGESTION_REASON_NO_HIDERS {
		t.Errorf("the suggestion for Toren = %v, want no surprise: nobody of the other side hides", p)
	}

	// The master decides: the mark shows in the suggestion and takes nothing from the numbers.
	c.mustSetSurprised(t, c.hob, true)
	got, _ = c.suggestions(t, a.master)
	if h := c.suggestionFor(t, got, c.hob); !h.GetSurprised() || !h.GetSuggested() {
		t.Errorf("the Hobgoblin after the mark = %v, want it marked and still suggested", h)
	}
	if g := c.suggestionFor(t, got, "Goblin"); g.GetSurprised() {
		t.Errorf("the Goblin was never marked: %v", g)
	}
}

// TestSurpriseSuggestionTakesAPartyMemberWhoIsNotHidingAsNoticed (decision W7-X 5: a party member who
// is not hiding is noticed): Toren and Brisa hide well, Pensantus never rolls Stealth, so a creature
// that cannot be fooled by Pensantus is not suggested surprised.
func TestSurpriseSuggestionTakesAPartyMemberWhoIsNotHidingAsNoticed(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{setup: true})
	a.setConditions(t, c.e, c.hob, "condition:poisoned")

	asked := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc = 0 })
	a.mustRollGroup(t, a.caio, asked.GetId(), 20)
	a.mustRollGroup(t, a.bia, asked.GetId(), 20) // Pensantus does not hide
	got, err := c.suggestions(t, a.master)
	if err != nil {
		t.Fatalf("GetSurpriseSuggestion() error = %v", err)
	}
	if h := c.suggestionFor(t, got, c.hob); h.GetSuggested() || h.GetReason() != playv1.SurpriseSuggestionReason_SURPRISE_SUGGESTION_REASON_NOTICES {
		t.Errorf("the suggestion for the Hobgoblin = %v, want it noticing: Pensantus is not hiding", h)
	}
}

// TestSurpriseSuggestionCountsWhatTheMasterSaidSeesTheHiderClearly: a creature the master said sees
// the hider clearly is not suggested surprised, whatever the totals.
func TestSurpriseSuggestionCountsWhatTheMasterSaidSeesTheHiderClearly(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	tried := c.mustHide(t, a.caio, "Toren", cunningHide, 20) // 23
	c.mustResolveHide(t, tried.GetId(), func(r *playv1.ResolveHideRequest) { r.SeesClearlyIds = []string{c.id(t, c.hob)} })
	got, err := c.suggestions(t, a.master)
	if err != nil {
		t.Fatalf("GetSurpriseSuggestion() error = %v", err)
	}
	// Only Toren hid, and with the Hide action the others of his side are not hiding, so the Goblin is not
	// suggested either; the point here is the Hobgoblin, who sees him clearly.
	h := c.suggestionFor(t, got, c.hob)
	if h.GetSuggested() || len(h.GetHiders()) == 0 || h.GetHiders()[0].GetBeats() {
		t.Errorf("the suggestion for the Hobgoblin = %v, want it noticing Toren although 23 beats its 10", h)
	}
}

// TestSurpriseCallsRefuseWhoIsNotAllowed: the suggestion is the master's, and a stranger, a pending
// member and the master of another campaign are refused with not_found.
func TestSurpriseCallsRefuseWhoIsNotAllowed(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{setup: true})
	for who, u := range map[string]*user{"a player": a.caio, "another player": a.ana} {
		if _, err := c.suggestions(t, u); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("GetSurpriseSuggestion as %s = %v, want permission_denied", who, err)
		}
	}
	o := a.outsiders(t)
	o.refused(t, "GetSurpriseSuggestion", func(u *user, campaignID string) error {
		_, err := u.contests.GetSurpriseSuggestion(t.Context(), connect.NewRequest(&playv1.GetSurpriseSuggestionRequest{CampaignId: campaignID, EncounterId: c.e.GetId()}))
		return err
	}, a.campaignID)
	o.refused(t, "SetSurprised", func(u *user, campaignID string) error {
		_, err := u.contests.SetSurprised(t.Context(), connect.NewRequest(&playv1.SetSurprisedRequest{
			CampaignId: campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, "Toren"), Surprised: true,
		}))
		return err
	}, a.campaignID)
}
