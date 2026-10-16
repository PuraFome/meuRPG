package play

import (
	"slices"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The characterization of what the contests show: GetContestState (the contests, the Helps,
// the grapples, the hidden and the surprised) and the contest side of GetTurnOptions, as the
// master, each player and a player outside every contest read them. They follow the rules the
// tests of combat_contests_test.go already check; these freeze each audience's answer.

// hideFromPlayers hides an NPC from the players' eyes (the master still reads it).
func (c *cx) hideFromPlayers(t *testing.T, label string) {
	t.Helper()
	c.execSQL(t, `UPDATE combatants SET hidden = true WHERE id = $1`, c.id(t, label))
}

// readOptions is the contest state and the special attacks of a combatant's turn as u reads
// them.
func (c *cx) readOptions(t *testing.T, u *user, label string) *playv1.GetTurnOptionsResponse {
	t.Helper()
	return c.mustOptions(t, u, c.refresh(t), label)
}

// TestContestViewNamesNoCreatureThePlayerDoesNotSee (RN-10): a contest whose side is an NPC the
// player does not see reads the id of that side as empty, and the contests of strangers are not
// read at all.
func TestContestViewNamesNoCreatureThePlayerDoesNotSee(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}, "Goblin": {3, 4}}})
	c.hideFromPlayers(t, "Goblin")

	// The hidden Goblin grapples Brisa: she reads her side only.
	c.advance(t, "Goblin")
	c.h.roller.queue(15)
	c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Goblin"), c.id(t, "Brisa"), purposeGrappleP
	})
	mine := c.state(t, a.bia).GetContests()
	if len(mine) != 1 || mine[0].GetInitiatorId() != "" || mine[0].GetDefenderId() != c.id(t, "Brisa") || !mine[0].GetYouAnswer() {
		t.Fatalf("Brisa reads %v, want the contest with the hidden initiator unnamed", mine)
	}
	if got := c.state(t, a.master).GetContests(); len(got) != 1 || got[0].GetInitiatorId() != c.id(t, "Goblin") {
		t.Errorf("the master reads %v, want the Goblin named", got)
	}
	for _, u := range []*user{a.caio, a.ana} {
		if got := c.state(t, u).GetContests(); len(got) != 0 {
			t.Errorf("a player outside the contest reads %v", got)
		}
	}
	c.mustRespond(t, a.bia, mine[0].GetId(), answerSkillAcrobats, 19)

	// Toren grapples the hidden Goblin by the master's hand: Toren reads his side only.
	c.advance(t, "Toren")
	c.h.roller.queue(18)
	c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, "Goblin"), purposeGrappleP
	})
	var toren *playv1.ContestView
	for _, v := range c.state(t, a.caio).GetContests() {
		if v.GetStatus() == playv1.ContestStatus_CONTEST_STATUS_AWAITING_DEFENDER {
			toren = v
		}
	}
	if toren == nil || toren.GetDefenderId() != "" || toren.GetInitiatorId() != c.id(t, "Toren") || toren.GetYouAnswer() || toren.GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_MASTER {
		t.Fatalf("Toren reads %v, want the hidden defender unnamed and the master awaited", toren)
	}
}

// TestContestStateHidesHelpsAndGrapplesBetweenCreaturesThePlayerDoesNotSee freezes helpViews,
// grappleViews and hiddenIDs: the master reads all; a player reads what they see.
func TestContestStateHidesHelpsAndGrapplesBetweenCreaturesThePlayerDoesNotSee(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Goblin": {4, 4}}})
	c.hideFromPlayers(t, "Goblin")

	// A Help for a check between two creatures the players see, and one that names the hidden Goblin.
	c.advance(t, "Brisa")
	if _, err := c.helpCheck(t, a.bia, "Brisa", "Toren", skillAthletics); err != nil {
		t.Fatalf("Help() error = %v", err)
	}
	c.advance(t, c.hob)
	if _, err := c.helpCheck(t, a.master, c.hob, "Goblin", skillAthletics); err != nil {
		t.Fatalf("Help(hob -> Goblin) error = %v", err)
	}
	if got := c.state(t, a.master).GetHelps(); len(got) != 2 {
		t.Fatalf("the master reads %d helps, want 2", len(got))
	}
	for _, u := range []*user{a.caio, a.ana, a.bia} {
		got := c.state(t, u).GetHelps()
		if len(got) != 1 || got[0].GetHelperId() != c.id(t, "Brisa") || got[0].GetAllyId() != c.id(t, "Toren") || got[0].GetKind() != playv1.HelpKind_HELP_KIND_CHECK || got[0].GetTaskKey() != skillAthletics || got[0].GetTaskNamePt() == "" {
			t.Errorf("a player reads helps %v, want only Brisa's for Toren", got)
		}
	}

	// A Help for an attack on the hidden Goblin: it is the master's alone.
	c.advance(t, "Toren")
	if _, err := c.helpAttack(t, a.master, "Toren", "Brisa", "Goblin"); err != nil {
		t.Fatalf("Help(attack on the Goblin) error = %v", err)
	}
	if got := c.state(t, a.master).GetHelps(); len(got) != 3 {
		t.Fatalf("the master reads %d helps, want 3", len(got))
	}
	for _, h := range c.state(t, a.caio).GetHelps() {
		if h.GetTargetId() == c.id(t, "Goblin") {
			t.Errorf("a player reads a help aimed at a creature they do not see: %v", h)
		}
	}

	// The hidden Goblin holds Toren: no player reads that grapple; the master does, with its DC.
	c.advance(t, "Goblin")
	c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Goblin"), c.id(t, "Toren"), purposeGrappleP
		r.Kind, r.EscapeDc, r.Roll = kindEscapeDC, 12, nil
	})
	if g := c.state(t, a.master).GetGrapples(); len(g) != 1 || g[0].GetEscapeDc() != 12 {
		t.Errorf("the master reads grapples %v, want one with the DC", g)
	}
	for _, u := range []*user{a.caio, a.ana, a.bia} {
		if g := c.state(t, u).GetGrapples(); len(g) != 0 {
			t.Errorf("a player reads the grapple of a creature they do not see: %v", g)
		}
	}

	// A hold whose condition is gone is no grapple to read (the row stays until the next change).
	a.execSQL(t, `UPDATE combatants SET conditions = '{}' WHERE id = $1`, c.id(t, "Toren"))
	if g := c.state(t, a.master).GetGrapples(); len(g) != 0 {
		t.Errorf("the master reads a hold whose condition is gone: %v", g)
	}
}

// TestContestStateReadsTwoShovesWaitingForTheirWinners freezes the push of two shoves read in one
// state (the map is read once for both) and what each side reads of them.
func TestContestStateReadsTwoShovesWaitingForTheirWinners(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.h.roller.queue(18)
	first := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeShoveP
	}).GetContest()
	c.mustRespond(t, a.master, first.GetId(), answerSkillAthletics, 2)
	c.mustShove(t, a.caio, first.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE)
	c.advance(t, "Brisa")
	c.h.roller.queue(18)
	second := c.mustStart(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Brisa"), c.id(t, c.hob), purposeShoveP
	}).GetContest()
	c.mustRespond(t, a.master, second.GetId(), answerSkillAthletics, 2)
	// A combat cannot move on while a shove waits for its winner, so the first one is put
	// back to waiting by hand: two shoves wait in one state.
	c.execSQL(t, `UPDATE combat_contests SET status = $2, shove_outcome = NULL, resolved_at = NULL WHERE id = $1`, first.GetId(), contestAwaitingOutcome)

	byID := func(u *user) map[string]*playv1.ContestView {
		out := map[string]*playv1.ContestView{}
		for _, v := range c.state(t, u).GetContests() {
			out[v.GetId()] = v
		}
		return out
	}
	m := byID(a.master)
	if len(m) != 2 {
		t.Fatalf("the master reads %d contests, want 2", len(m))
	}
	for id, v := range m {
		if !v.GetYouChoose() || !v.GetShoveChoice().GetProneAvailable() || v.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_OUTCOME {
			t.Errorf("the master reads shove %s as %v, want a choice to make", id, v)
		}
	}
	if v := byID(a.caio)[first.GetId()]; v == nil || !v.GetYouChoose() || byID(a.caio)[second.GetId()] != nil {
		t.Errorf("Toren reads %v, want only his shove and a choice to make", byID(a.caio))
	}
	if v := byID(a.bia)[second.GetId()]; v == nil || !v.GetYouChoose() || byID(a.bia)[first.GetId()] != nil {
		t.Errorf("Brisa reads %v, want only her shove and a choice to make", byID(a.bia))
	}
	if got := byID(a.ana); len(got) != 0 {
		t.Errorf("Pensantus reads %v, want none", got)
	}
}

// TestTurnOptionsOfAGrappledCombatantReadTheGateAndTheSpentAction freezes the state of a
// grappled creature's turn: Escape is offered with the reason it cannot be taken now (it is not
// her turn; the action is spent), and the master is never refused.
func TestTurnOptionsOfAGrappledCombatantReadTheGateAndTheSpentAction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.grappledByAttack(t, "Brisa", 16)

	// It is the Hobgoblin's turn: Brisa cannot escape now.
	st := c.readOptions(t, a.bia, "Brisa").GetContestState()
	if !st.GetGrappled() || st.GetCanEscape() || st.GetEscapeReason() == nil || st.GetGrapplerId() != c.id(t, c.hob) || len(st.GetEscapeOptions()) != 2 {
		t.Errorf("Brisa's state out of turn = %v, want grappled, no escape, a reason and two skills", st)
	}
	if got := st.GetEscapeReason().GetCode(); got != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN {
		t.Errorf("the escape reason out of turn = %v, want NOT_YOUR_TURN", got)
	}

	// Her turn: she may escape; after a failed try the action is spent.
	c.advance(t, "Brisa")
	if st := c.readOptions(t, a.bia, "Brisa").GetContestState(); !st.GetCanEscape() || st.GetEscapeReason() != nil {
		t.Errorf("Brisa's state in her turn = %v, want to be able to escape", st)
	}
	c.h.roller.queue(2)
	c.mustStart(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose = c.id(t, "Brisa"), purposeEscapeP
	})
	st = c.readOptions(t, a.bia, "Brisa").GetContestState()
	if !st.GetGrappled() || st.GetCanEscape() || st.GetEscapeReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED {
		t.Errorf("Brisa's state with the action spent = %v, want no escape for ACTION_USED", st)
	}
	if st := c.readOptions(t, a.master, "Brisa").GetContestState(); !st.GetCanEscape() || st.GetEscapeReason() != nil {
		t.Errorf("the master reads Brisa's state as %v, want an escape he may always take", st)
	}
	for _, o := range st.GetEscapeOptions() {
		if o.GetSkill() != answerSkillAthletics && o.GetSkill() != answerSkillAcrobats {
			t.Errorf("escape option %v", o)
		}
	}
}

// TestTurnOptionsReadWhoHoldsAndWhoHelps freezes HoldingId and HelpTargetId: the creature the
// combatant holds and the one it was helped to attack, never a creature the player does not see.
func TestTurnOptionsReadWhoHoldsAndWhoHelps(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.grappleWon(t)
	if st := c.readOptions(t, a.caio, "Toren").GetContestState(); st.GetHoldingId() != c.id(t, c.hob) {
		t.Errorf("Toren's state = %v, want him holding the Hobgoblin", st)
	}
	if st := c.readOptions(t, a.master, "Toren").GetContestState(); st.GetHoldingId() != c.id(t, c.hob) {
		t.Errorf("the master reads %v, want Toren holding the Hobgoblin", st)
	}
	c.hideFromPlayers(t, c.hob)
	if st := c.readOptions(t, a.caio, "Toren").GetContestState(); st.GetHoldingId() != "" {
		t.Errorf("Toren reads %v, want a hold on a creature he does not see unnamed", st)
	}
	if st := c.readOptions(t, a.master, "Toren").GetContestState(); st.GetHoldingId() != c.id(t, c.hob) {
		t.Errorf("the master reads %v, want the hold named", st)
	}

	// Brisa helps Toren attack the Hobgoblin (seen again), and Toren reads whom.
	c.execSQL(t, `UPDATE combatants SET hidden = false WHERE id = $1`, c.id(t, c.hob))
	c.advance(t, "Brisa")
	if _, err := c.moveTo(t, a.master, "Brisa", 5, 3); err != nil {
		t.Fatalf("placing Brisa: %v", err)
	}
	if _, err := c.helpAttack(t, a.bia, "Brisa", "Toren", c.hob); err != nil {
		t.Fatalf("Help(attack) error = %v", err)
	}
	if st := c.readOptions(t, a.caio, "Toren").GetContestState(); st.GetHelpTargetId() != c.id(t, c.hob) {
		t.Errorf("Toren reads %v, want the help aimed at the Hobgoblin", st)
	}
	c.hideFromPlayers(t, c.hob)
	if st := c.readOptions(t, a.caio, "Toren").GetContestState(); st.GetHelpTargetId() != "" {
		t.Errorf("Toren reads %v, want a help aimed at an unseen creature unnamed", st)
	}
}

// TestTurnOptionsOfTheSpecialAttacksReadTheEconomy freezes the reasons the grapple and the shove
// are off: the action went to a cantrip, and not the player's turn.
func TestTurnOptionsOfTheSpecialAttacksReadTheEconomy(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Pensantus": {3, 4}}})
	grapple := playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE
	shove := playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_SHOVE

	// Toren's turn: both are on, for one attack, with the targets within reach.
	o := c.readOptions(t, a.caio, "Toren")
	for _, kind := range []playv1.ContestAttackOptionKind{grapple, shove} {
		opt := grappleOption(o, kind)
		var targets []string
		for _, tg := range opt.GetTargets() {
			targets = append(targets, tg.GetCombatantId())
		}
		slices.Sort(targets)
		want := []string{c.id(t, c.hob), c.id(t, "Pensantus")}
		slices.Sort(want)
		if opt == nil || !opt.GetEnabled() || opt.GetAttacksLeft() != 1 || !opt.GetReplacesAttack() || !slices.Equal(targets, want) {
			t.Errorf("Toren's %v = %v, want one enabled attack at the Hobgoblin and Pensantus", kind, opt)
		}
	}

	// Pensantus casts a cantrip: the action is gone, and the options say so.
	c.advance(t, "Pensantus")
	c.h.roller.queue(12, 5)
	a.mustAttack(t, a.ana, c.refresh(t), "Pensantus", fireBolt, c.hob, inAppRoll)
	opt := grappleOption(c.readOptions(t, a.ana, "Pensantus"), grapple)
	if opt.GetEnabled() || opt.GetAttacksLeft() != 0 || opt.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED {
		t.Errorf("Pensantus's grapple after a cantrip = %v, want it off for ACTION_USED with no attack left", opt)
	}
	// The master has the last word: nothing is refused to him, whatever the action spent.
	if opt := grappleOption(c.readOptions(t, a.master, "Pensantus"), shove); !opt.GetEnabled() || opt.GetReason() != nil || opt.GetAttacksLeft() != 0 {
		t.Errorf("the master reads the shove of Pensantus after a cantrip as %v, want it on", opt)
	}

	// Not Toren's turn: the options are off with the gate's reason; the master reads them on.
	o = c.readOptions(t, a.caio, "Toren")
	if opt := grappleOption(o, grapple); opt.GetEnabled() || opt.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN {
		t.Errorf("Toren's grapple out of turn = %v, want it off for NOT_YOUR_TURN", opt)
	}
	if opt := grappleOption(c.readOptions(t, a.master, "Toren"), grapple); opt.GetEnabled() || opt.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN {
		t.Errorf("the master reads Toren's grapple out of turn as %v, want it off for NOT_YOUR_TURN too", opt)
	}
}

// TestTurnOptionsOfAFighterWithExtraAttackReadAttacksUsed freezes the reason once the Attack
// action made all its attacks.
func TestTurnOptionsOfAFighterWithExtraAttackReadAttacksUsed(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		scores := func(str, dex, con, intl int32) *rulesv1.AbilityScores {
			return &rulesv1.AbilityScores{Strength: str, Dexterity: dex, Constitution: con, Intelligence: intl, Wisdom: 10, Charisma: 8}
		}
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, scores(16, 13, 14, 10), []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, scores(10, 14, 12, 16), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores(10, 16, 14, 10), []string{rapier}, nil)
	})
	c := a.arena(t, arenaPlan{})
	grapple := playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE
	if opt := grappleOption(c.readOptions(t, a.caio, "Toren"), grapple); !opt.GetEnabled() || opt.GetAttacksLeft() != 2 {
		t.Fatalf("Toren's grapple = %v, want two attacks left", opt)
	}
	c.h.roller.queue(3)
	c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeGrappleP
	})
	c.mustRespond(t, a.master, c.state(t, a.master).GetContests()[0].GetId(), answerSkillAthletics, 1)
	if opt := grappleOption(c.readOptions(t, a.caio, "Toren"), grapple); !opt.GetEnabled() || opt.GetAttacksLeft() != 1 {
		t.Fatalf("Toren's grapple after one = %v, want one attack left", opt)
	}
	c.h.roller.queue(3)
	c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeShoveP
	})
	opt := grappleOption(c.readOptions(t, a.caio, "Toren"), grapple)
	if opt.GetEnabled() || opt.GetAttacksLeft() != 0 || opt.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ATTACKS_USED {
		t.Errorf("Toren's grapple after both = %v, want it off for ATTACKS_USED", opt)
	}
	if !slices.ContainsFunc(c.state(t, a.master).GetContests(), func(v *playv1.ContestView) bool {
		return v.GetPurpose() == purposeShoveP
	}) {
		t.Error("the shove is not in the master's state")
	}
}
