package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Help (SRD 5.1, "Help"). These tests need the database (MEURPG_TEST_DATABASE_URL). The fixture is
// newSneaks (a rogue Toren, a wizard Pensantus, a fighter Brisa) in the arena of
// combat_contests_test.go: the order is Toren, Brisa, Pensantus, the Hobgoblin, the Goblin.

// help calls Help as u.
func (c *cx) help(t *testing.T, u *user, helper string, edit func(*playv1.HelpRequest)) (*playv1.HelpResponse, error) {
	t.Helper()
	req := &playv1.HelpRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, helper)}
	edit(req)
	res, err := u.contests.Help(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// helpCheck is Help with a check: the helper helps the ally with a task.
func (c *cx) helpCheck(t *testing.T, u *user, helper, ally, task string) (*playv1.HelpResponse, error) {
	t.Helper()
	return c.help(t, u, helper, func(r *playv1.HelpRequest) {
		r.Kind, r.AllyId, r.TaskKey = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, ally), task
	})
}

// helpAttack is Help with an attack: the ally's first attack on the target has advantage.
func (c *cx) helpAttack(t *testing.T, u *user, helper, ally, target string) (*playv1.HelpResponse, error) {
	t.Helper()
	return c.help(t, u, helper, func(r *playv1.HelpRequest) {
		r.Kind, r.AllyId, r.TargetId = playv1.HelpKind_HELP_KIND_ATTACK, c.id(t, ally), c.id(t, target)
	})
}

// helped is the Help a call gave; the call must have worked.
func helped(t *testing.T) func(*playv1.HelpResponse, error) *playv1.HelpView {
	return func(res *playv1.HelpResponse, err error) *playv1.HelpView {
		t.Helper()
		if err != nil {
			t.Fatalf("Help() error = %v", err)
		}
		if res.GetHelp() == nil {
			t.Fatal("Help() answered no help")
		}
		return res.GetHelp()
	}
}

func (c *cx) clearHelp(t *testing.T, u *user, helpID string) error {
	t.Helper()
	_, err := u.contests.ClearHelp(t.Context(), connect.NewRequest(&playv1.ClearHelpRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), HelpId: helpID,
	}))
	return err
}

// TestHelpWithACheckGivesTheAllyAdvantageOnItsNextCheckOfTheTaskOnly (SRD 5.1, "Help"): the
// helper spends its action, every player reads the Help ("Ajuda de Brisa"), the ally's next
// Stealth check has advantage and uses the Help up; a Help for another task is not used.
func TestHelpWithACheckGivesTheAllyAdvantageOnItsNextCheckOfTheTaskOnly(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, "Brisa")

	given := helped(t)(c.helpCheck(t, a.bia, "Brisa", "Toren", "skill:stealth"))
	if given.GetKind() != playv1.HelpKind_HELP_KIND_CHECK || given.GetTaskKey() != "skill:stealth" || given.GetTaskNamePt() == "" ||
		given.GetHelperId() != c.id(t, "Brisa") || given.GetAllyId() != c.id(t, "Toren") || given.GetExpiresRound() != 2 {
		t.Fatalf("help = %v, want Brisa's Help to Toren on Stealth, lasting through round 2", given)
	}
	if !a.mustOptions(t, a.bia, c.refresh(t), "Brisa").GetOptions().GetEconomy().GetAction().GetUsed() {
		t.Error("Help did not spend the action")
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if hs := c.state(t, u).GetHelps(); len(hs) != 1 || hs[0].GetId() != given.GetId() {
			t.Errorf("%s reads helps %v, want Brisa's", who, hs)
		}
	}

	// Toren's turn of the next round: the Help is in his option (a source of advantage for all to read).
	c.advance(t, "Goblin")
	c.advance(t, "Toren")
	c.h.roller.queue(3, 15) // the pair; the higher counts
	tried := c.mustHide(t, a.caio, "Toren", "", 0)
	roll := tried.GetRoll()
	if roll.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_ADVANTAGE || len(roll.GetFaces()) != 2 || roll.GetTotal() != 15+3 {
		t.Fatalf("the Stealth check = %v, want advantage and 18", roll)
	}
	var note bool
	for _, n := range roll.GetNotes() {
		note = note || (n.GetKind() == "help" && n.GetAdvantage() && strings.Contains(n.GetLabelPt(), "Brisa"))
	}
	if !note {
		t.Errorf("the roll's notes = %v, want \"Ajuda de Brisa\"", roll.GetNotes())
	}
	if hs := c.state(t, a.master).GetHelps(); len(hs) != 0 {
		t.Errorf("the Help outlives the check it was used on: %v", hs)
	}
}

// TestHelpForAnotherTaskIsNotUsedAndEndsWithTheHelpersNextTurn (SRD 5.1, "Help"; the app's reading:
// until the end of the helper's next turn): a Stealth check does not use a Help for Athletics, and
// the Help is gone once Brisa's turn of the next round ends.
func TestHelpForAnotherTaskIsNotUsedAndEndsWithTheHelpersNextTurn(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, "Brisa")
	given := helped(t)(c.helpCheck(t, a.bia, "Brisa", "Toren", "skill:athletics"))

	c.advance(t, "Goblin")
	c.advance(t, "Toren")
	c.h.roller.queue(9)
	tried := c.mustHide(t, a.caio, "Toren", "", 0)
	if tried.GetRoll().GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_NORMAL || len(tried.GetRoll().GetFaces()) != 1 {
		t.Errorf("the Stealth check = %v, want a normal roll (the Help is for Athletics)", tried.GetRoll())
	}
	if hs := c.state(t, a.caio).GetHelps(); len(hs) != 1 || hs[0].GetId() != given.GetId() {
		t.Fatalf("helps in Toren's turn of round 2 = %v, want the Help still holding", hs)
	}
	c.advance(t, "Brisa")
	if hs := c.state(t, a.caio).GetHelps(); len(hs) != 1 {
		t.Fatalf("helps in Brisa's turn of round 2 = %v, want the Help still holding until her turn ends", hs)
	}
	c.advance(t, "Pensantus")
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if hs := c.state(t, u).GetHelps(); len(hs) != 0 {
			t.Errorf("%s reads helps %v after Brisa's next turn ended, want none", who, hs)
		}
	}
}

// TestHelpWithAnAttackNeedsTheHelperWithin5FeetAndGivesTheFirstAttackAdvantage (SRD 5.1, "Help"):
// Brisa must be within 5 feet of the Hobgoblin; Toren's first attack on it has advantage with the
// source "Ajuda de Brisa", the attack uses the Help up, and the master's undo gives it back.
func TestHelpWithAnAttackNeedsTheHelperWithin5FeetAndGivesTheFirstAttackAdvantage(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, "Brisa")

	// Brisa is on (8,8), far from the Hobgoblin on (4,3).
	_, err := c.helpAttack(t, a.bia, "Brisa", "Toren", c.hob)
	b := wantBlockedBy(t, "helping an attack on a creature out of reach", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)
	if b.GetMissingFt() <= 0 {
		t.Errorf("missing_ft = %d, want how far she is from the reach", b.GetMissingFt())
	}
	if a.mustOptions(t, a.bia, c.refresh(t), "Brisa").GetOptions().GetEconomy().GetAction().GetUsed() {
		t.Fatal("a refused Help spent the action")
	}
	if _, err := c.moveTo(t, a.bia, "Brisa", 5, 3); err != nil {
		t.Fatalf("moving Brisa next to the Hobgoblin: %v", err)
	}
	given := helped(t)(c.helpAttack(t, a.bia, "Brisa", "Toren", c.hob))
	if given.GetKind() != playv1.HelpKind_HELP_KIND_ATTACK || given.GetTargetId() != c.id(t, c.hob) {
		t.Fatalf("help = %v, want an attack Help aimed at the Hobgoblin", given)
	}

	c.advance(t, "Goblin")
	c.advance(t, "Toren")
	tg := targetOf(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), rapier, c.hob)
	if tg == nil || tg.GetRollMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || len(tg.GetSources()) != 1 ||
		tg.GetSources()[0].GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_HELP || !strings.Contains(tg.GetSources()[0].GetTextPt(), "Brisa") {
		t.Fatalf("the Hobgoblin as a target = %v, want advantage from \"Ajuda de Brisa\"", tg)
	}
	res := a.mustAttack(t, a.caio, c.e, "Toren", rapier, c.hob, func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{2, 3} })
	if res.GetRoll().GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE {
		t.Fatalf("the attack = %v, want advantage", res.GetRoll())
	}
	if hs := c.state(t, a.master).GetHelps(); len(hs) != 0 {
		t.Errorf("the Help outlives the attack it was used on (hit or miss): %v", hs)
	}
	if tg := targetOf(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), rapier, c.hob); tg.GetRollMode() != playv1.RollMode_ROLL_MODE_NORMAL {
		t.Errorf("after the Help was used the next attack is %v, want a normal roll", tg.GetRollMode())
	}
	if err := a.undo(t, a.master, c.e, a.log(t, a.master, c.e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction(attack) error = %v", err)
	}
	if hs := c.state(t, a.master).GetHelps(); len(hs) != 1 || hs[0].GetId() != given.GetId() {
		t.Errorf("helps after the undo = %v, want Brisa's Help back", hs)
	}
}

// TestHelpRefusesWhatIsNotAnAllyATaskOrAFreeAction: the reasons of the refusals of Help.
func TestHelpRefusesWhatIsNotAnAllyATaskOrAFreeAction(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})

	// Off turn, and with another player's combatant.
	_, err := c.helpCheck(t, a.bia, "Brisa", "Toren", "skill:stealth")
	wantBlockedBy(t, "helping off turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN)
	if _, err := c.helpCheck(t, a.caio, "Brisa", "Toren", "skill:stealth"); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("helping with another player's combatant = %v, want permission_denied", err)
	}

	c.advance(t, "Brisa")
	for name, call := range map[string]func() (*playv1.HelpResponse, error){
		"helping oneself":           func() (*playv1.HelpResponse, error) { return c.helpCheck(t, a.bia, "Brisa", "Brisa", "skill:stealth") },
		"helping an enemy":          func() (*playv1.HelpResponse, error) { return c.helpCheck(t, a.bia, "Brisa", c.hob, "skill:stealth") },
		"an attack Help on an ally": func() (*playv1.HelpResponse, error) { return c.helpAttack(t, a.bia, "Brisa", "Toren", "Pensantus") },
	} {
		_, err := call()
		wantContestBlocked(t, name, err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AN_ALLY)
	}
	for _, task := range []string{"save:dex", "initiative", "skill:no-such-skill", "ability:", "skill:"} {
		_, err := c.helpCheck(t, a.bia, "Brisa", "Toren", task)
		wantContestBlocked(t, "the task "+task, err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_TASK_NOT_AVAILABLE)
	}
	for name, edit := range map[string]func(*playv1.HelpRequest){
		"no kind":              func(r *playv1.HelpRequest) { r.AllyId = c.id(t, "Toren") },
		"a check with no task": func(r *playv1.HelpRequest) { r.Kind, r.AllyId = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Toren") },
		"a check with a target": func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey, r.TargetId = playv1.HelpKind_HELP_KIND_CHECK, c.id(t, "Toren"), "skill:stealth", c.id(t, c.hob)
		},
		"an attack with a task": func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey, r.TargetId = playv1.HelpKind_HELP_KIND_ATTACK, c.id(t, "Toren"), "skill:stealth", c.id(t, c.hob)
		},
		"an attack with no target": func(r *playv1.HelpRequest) { r.Kind, r.AllyId = playv1.HelpKind_HELP_KIND_ATTACK, c.id(t, "Toren") },
		"a combatant that is no UUID": func(r *playv1.HelpRequest) {
			r.Kind, r.AllyId, r.TaskKey = playv1.HelpKind_HELP_KIND_CHECK, "toren", "skill:stealth"
		},
	} {
		if _, err := c.help(t, a.bia, "Brisa", edit); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s = %v, want invalid_argument", name, err)
		}
	}
	if a.mustOptions(t, a.bia, c.refresh(t), "Brisa").GetOptions().GetEconomy().GetAction().GetUsed() {
		t.Fatal("a refused Help spent the action")
	}

	// One Help takes the action: a second is refused, and the master helps with any combatant.
	helped(t)(c.helpCheck(t, a.bia, "Brisa", "Toren", "skill:stealth"))
	_, err = c.helpCheck(t, a.bia, "Brisa", "Pensantus", "skill:arcana")
	wantBlockedBy(t, "a second Help in the turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED)
	c.advance(t, "Pensantus")
	helped(t)(c.helpCheck(t, a.master, "Pensantus", "Toren", "skill:arcana"))
}

// TestMasterClearsAHelpAndOnlyOnce: ClearHelp takes a Help back for good; a Help used or cleared is
// not awaited, a player cannot clear it and an unknown one is not found.
func TestMasterClearsAHelpAndOnlyOnce(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, "Brisa")
	given := helped(t)(c.helpCheck(t, a.bia, "Brisa", "Toren", "skill:stealth"))

	if err := c.clearHelp(t, a.bia, given.GetId()); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player clearing a Help = %v, want permission_denied", err)
	}
	if err := c.clearHelp(t, a.master, newKey()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("clearing a Help that does not exist = %v, want not_found", err)
	}
	if err := c.clearHelp(t, a.master, given.GetId()); err != nil {
		t.Fatalf("ClearHelp() error = %v", err)
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if hs := c.state(t, u).GetHelps(); len(hs) != 0 {
			t.Errorf("%s reads helps %v after the master cleared it", who, hs)
		}
	}
	wantContestBlocked(t, "clearing a Help twice", c.clearHelp(t, a.master, given.GetId()), playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_AWAITING)

	// A Help that was cleared does not give the ally advantage.
	c.advance(t, "Goblin")
	c.advance(t, "Toren")
	c.h.roller.queue(9)
	if tried := c.mustHide(t, a.caio, "Toren", "", 0); tried.GetRoll().GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_NORMAL {
		t.Errorf("the Stealth check = %v, want a normal roll after the Help was cleared", tried.GetRoll())
	}
}

// TestHelpAndClearHelpAreIdempotent: a retry answers the same Help without taking the action twice,
// and the same key with another request is refused.
func TestHelpAndClearHelpAreIdempotent(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	c.advance(t, "Brisa")
	req := &playv1.HelpRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, "Brisa"),
		Kind: playv1.HelpKind_HELP_KIND_CHECK, AllyId: c.id(t, "Toren"), TaskKey: "skill:stealth",
	}
	first, err := a.bia.contests.Help(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("Help() error = %v", err)
	}
	again, err := a.bia.contests.Help(t.Context(), connect.NewRequest(req))
	if err != nil {
		t.Fatalf("Help(retry) error = %v", err)
	}
	if again.Msg.GetHelp().GetId() != first.Msg.GetHelp().GetId() || len(c.state(t, a.master).GetHelps()) != 1 {
		t.Errorf("the retry = %v, want the same Help and only one in the combat", again.Msg.GetHelp())
	}
	changed := &playv1.HelpRequest{
		CampaignId: req.GetCampaignId(), EncounterId: req.GetEncounterId(), IdempotencyKey: req.GetIdempotencyKey(), CombatantId: req.GetCombatantId(),
		Kind: playv1.HelpKind_HELP_KIND_CHECK, AllyId: c.id(t, "Pensantus"), TaskKey: "skill:stealth",
	}
	_, err = a.bia.contests.Help(t.Context(), connect.NewRequest(changed))
	wantCode(t, "the same key with another ally", err, connect.CodeInvalidArgument)

	clear := &playv1.ClearHelpRequest{CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), HelpId: first.Msg.GetHelp().GetId()}
	if _, err := a.master.contests.ClearHelp(t.Context(), connect.NewRequest(clear)); err != nil {
		t.Fatalf("ClearHelp() error = %v", err)
	}
	if _, err := a.master.contests.ClearHelp(t.Context(), connect.NewRequest(clear)); err != nil {
		t.Errorf("ClearHelp(retry) error = %v, want the same answer", err)
	}
	other := &playv1.ClearHelpRequest{CampaignId: clear.GetCampaignId(), EncounterId: clear.GetEncounterId(), IdempotencyKey: clear.GetIdempotencyKey(), HelpId: newKey()}
	_, err = a.master.contests.ClearHelp(t.Context(), connect.NewRequest(other))
	wantCode(t, "the same key for another Help", err, connect.CodeInvalidArgument)
}

// TestHelpCallsRefuseWhoIsNotInTheCampaign: a stranger, a pending member and the master of another
// campaign are refused with not_found.
func TestHelpCallsRefuseWhoIsNotInTheCampaign(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	o := a.outsiders(t)
	o.refused(t, "Help", func(u *user, campaignID string) error {
		_, err := u.contests.Help(t.Context(), connect.NewRequest(&playv1.HelpRequest{
			CampaignId: campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, "Toren"),
			Kind: playv1.HelpKind_HELP_KIND_CHECK, AllyId: c.id(t, "Brisa"), TaskKey: "skill:stealth",
		}))
		return err
	}, a.campaignID)
	o.refused(t, "ClearHelp", func(u *user, campaignID string) error {
		_, err := u.contests.ClearHelp(t.Context(), connect.NewRequest(&playv1.ClearHelpRequest{CampaignId: campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), HelpId: newKey()}))
		return err
	}, a.campaignID)
}
