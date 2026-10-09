package play

import (
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Hide (SRD 5.1, "Hide", "Hiding", "Passive Perception", "Unseen Attackers and Targets"). These
// tests need the database (MEURPG_TEST_DATABASE_URL). The fixture is the arena of
// combat_contests_test.go with a rogue Toren (Dexterity 16: Stealth +3, Cunning Action), a wizard
// Pensantus (Stealth +2, Magic Missile) and a fighter Brisa. The enemies are a Hobgoblin (passive
// Perception 10, a stat block with a known number) and a Goblin of a basic sheet (passive
// Perception 10, no known number).

const cunningHide = "feature:cunning-action:hide"

// sneakScores are a rogue's: Dexterity 16, so Stealth +3.
func sneakScores() *rulesv1.AbilityScores {
	return &rulesv1.AbilityScores{Strength: 12, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}
}

// newSneaks is the party of the hide tests: a rogue, a wizard with Magic Missile, a fighter.
func newSneaks(t *testing.T) *armed {
	t.Helper()
	return newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:rogue", "race:human", 3, sneakScores(), []string{rapier}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{magicMissileSpell}, []string{magicMissileSpell})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
	})
}

// hide calls Hide as u with the d20 the roller will give.
func (c *cx) hide(t *testing.T, u *user, label, actionKey string, face int) (*playv1.HideResponse, error) {
	t.Helper()
	if face > 0 { // 0: the test queued the dice itself
		c.h.roller.queue(face)
	}
	res, err := u.contests.Hide(t.Context(), connect.NewRequest(&playv1.HideRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, label), ActionKey: actionKey, Roll: rollIn(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cx) mustHide(t *testing.T, u *user, label, actionKey string, face int) *playv1.HideAttemptView {
	t.Helper()
	res, err := c.hide(t, u, label, actionKey, face)
	if err != nil {
		t.Fatalf("Hide(%s) error = %v", label, err)
	}
	return res.GetAttempt()
}

// resolveHide calls ResolveHide as u.
func (c *cx) resolveHide(t *testing.T, u *user, attemptID string, edit func(*playv1.ResolveHideRequest)) (*playv1.ResolveHideResponse, error) {
	t.Helper()
	req := &playv1.ResolveHideRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), AttemptId: attemptID}
	if edit != nil {
		edit(req)
	}
	res, err := u.contests.ResolveHide(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cx) mustResolveHide(t *testing.T, attemptID string, edit func(*playv1.ResolveHideRequest)) *playv1.HideAttemptView {
	t.Helper()
	res, err := c.resolveHide(t, c.master, attemptID, edit)
	if err != nil {
		t.Fatalf("ResolveHide() error = %v", err)
	}
	return res.GetAttempt()
}

// observerOf is what the master reads of one creature that could see the hider.
func observerOf(t *testing.T, v *playv1.HideAttemptView, id string) *playv1.HideObserver {
	t.Helper()
	for _, o := range v.GetObservers() {
		if o.GetCombatantId() == id {
			return o
		}
	}
	t.Fatalf("the attempt has no observer %s: %v", id, v.GetObservers())
	return nil
}

// outsiders are people who are not in the campaign: a stranger, a pending member and the master
// of another campaign (which has an open session of its own). Each is asked with the campaign it
// belongs to, so the call crosses the campaigns.
type outsiders struct {
	stranger, pending, other *user
	otherCampaign            string
}

func (a *armed) outsiders(t *testing.T) outsiders {
	t.Helper()
	o := outsiders{stranger: a.h.newUser("Intruso"), pending: a.h.newUser("Pendente"), other: a.h.newUser("Outro mestre")}
	a.h.joinPending(a.master, a.campaignID, o.pending)
	o.otherCampaign = a.h.newCampaign(o.other, "Outra mesa")
	o.other.start(t, o.otherCampaign)
	return o
}

// refused fails unless every outsider is refused with not_found by call; the master of the other
// campaign calls with its own campaign's id, with this campaign's combat.
func (o outsiders) refused(t *testing.T, name string, call func(u *user, campaignID string) error, campaignID string) {
	t.Helper()
	wantCode(t, name+" as a stranger", call(o.stranger, campaignID), connect.CodeNotFound)
	wantCode(t, name+" as a pending member", call(o.pending, campaignID), connect.CodeNotFound)
	wantCode(t, name+" as the master of another campaign, on this campaign", call(o.other, campaignID), connect.CodeNotFound)
	wantCode(t, name+" as the master of another campaign, with its own campaign's id", call(o.other, o.otherCampaign), connect.CodeNotFound)
}

// TestHideTotalEqualToPassivePerceptionKeepsTheHiderNoticed (SRD 5.1, "Hiding": the SRD says
// nothing of a tie, the app keeps the hider noticed; one point more hides). The hider's Stealth
// is compared with each creature's passive Perception; a Hide with Cunning Action spends the
// bonus action and not the action.
func TestHideTotalEqualToPassivePerceptionKeepsTheHiderNoticed(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	hob, goblin := c.id(t, c.hob), c.id(t, "Goblin")

	// Stealth +3: d20 7 makes 10, the passive Perception of both.
	tried := c.mustHide(t, a.caio, "Toren", cunningHide, 7)
	if tried.GetStatus() != playv1.HideAttemptStatus_HIDE_ATTEMPT_STATUS_PENDING || tried.GetRoll().GetModifier() != 3 || tried.GetRoll().GetTotal() != 10 {
		t.Fatalf("the attempt = %v, want pending, Stealth +3 and a total of 10", tried)
	}
	if got := c.state(t, a.master).GetHiddenIds(); len(got) != 0 {
		t.Errorf("hidden before the master decided: %v", got)
	}
	opts := a.mustOptions(t, a.caio, c.refresh(t), "Toren")
	if eco := opts.GetOptions().GetEconomy(); !eco.GetBonusAction().GetUsed() || eco.GetAction().GetUsed() {
		t.Errorf("economy = %v, want the bonus action spent and the action free", eco)
	}

	applied := c.mustResolveHide(t, tried.GetId(), nil)
	if applied.GetStatus() != playv1.HideAttemptStatus_HIDE_ATTEMPT_STATUS_APPLIED {
		t.Fatalf("status = %v, want applied", applied.GetStatus())
	}
	for _, id := range []string{hob, goblin} {
		if o := observerOf(t, applied, id); !o.GetNoticed() || o.GetPassivePerception() != 10 {
			t.Errorf("observer %s = %v, want it to notice the hider at 10 against 10 (a tie keeps the hider noticed)", id, o)
		}
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if got := c.state(t, u).GetHiddenIds(); len(got) != 0 {
			t.Errorf("%s reads %v hidden after a tie, want nobody", who, got)
		}
	}
	// Noticed, Toren's attack has no advantage.
	if tg := targetOf(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), rapier, c.hob); tg == nil || len(tg.GetSources()) != 0 || tg.GetRollMode() != playv1.RollMode_ROLL_MODE_NORMAL {
		t.Errorf("the Hobgoblin as a target = %v, want a normal roll with no source", tg)
	}

	// The next round, one point more: 8 + 3 = 11 beats both.
	c.advance(t, "Goblin")
	c.advance(t, "Toren")
	beat := c.mustHide(t, a.caio, "Toren", cunningHide, 8)
	if beat.GetRoll().GetTotal() != 11 {
		t.Fatalf("total = %d, want 11", beat.GetRoll().GetTotal())
	}
	applied = c.mustResolveHide(t, beat.GetId(), nil)
	for _, id := range []string{hob, goblin} {
		if o := observerOf(t, applied, id); o.GetNoticed() {
			t.Errorf("observer %s notices a total of 11 against 10: %v", id, o)
		}
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if got := c.state(t, u).GetHiddenIds(); !slices.Equal(got, []string{c.id(t, "Toren")}) {
			t.Errorf("%s reads %v hidden, want Toren", who, got)
		}
	}
	if tg := targetOf(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), rapier, c.hob); tg == nil || tg.GetRollMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE ||
		len(tg.GetSources()) != 1 || tg.GetSources()[0].GetKind() != playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_UNSEEN_ATTACKER {
		t.Errorf("the Hobgoblin as a target = %v, want advantage with the unseen attacker as its only source", tg)
	}
}

// TestHideIsComparedWithEachObserversOwnPassivePerception (SRD 5.1, "Passive Perception": 10 plus
// the bonus, 5 less with disadvantage): a poisoned Hobgoblin has disadvantage on ability checks,
// so its passive Perception is 5 and a total of 7 beats it; the Goblin's 10 does not. The hider
// is hidden while one creature has not noticed. Only the master reads who noticed.
func TestHideIsComparedWithEachObserversOwnPassivePerception(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	hob, goblin := c.id(t, c.hob), c.id(t, "Goblin")
	a.setConditions(t, c.refresh(t), c.hob, "condition:poisoned")

	tried := c.mustHide(t, a.caio, "Toren", cunningHide, 4) // 4 + 3 = 7
	// The master reads the suggestion of a pending attempt before he decides.
	pending := c.state(t, a.master).GetHideAttempts()
	if len(pending) != 1 || observerOf(t, pending[0], hob).GetNoticed() || !observerOf(t, pending[0], goblin).GetNoticed() {
		t.Fatalf("the master's pending attempt = %v, want the Hobgoblin not noticing (7 against 5) and the Goblin noticing (7 against 10)", pending)
	}
	applied := c.mustResolveHide(t, tried.GetId(), nil)
	if o := observerOf(t, applied, hob); o.GetPassivePerception() != 5 || o.GetNoticed() {
		t.Errorf("the poisoned Hobgoblin = %v, want a passive Perception of 5 and not noticing", o)
	}
	if o := observerOf(t, applied, goblin); o.GetPassivePerception() != 10 || !o.GetNoticed() || o.GetKnown() {
		t.Errorf("the Goblin = %v, want a passive Perception of 10 (no known number) and noticing", o)
	}
	if got := c.state(t, a.master).GetHiddenIds(); !slices.Equal(got, []string{c.id(t, "Toren")}) {
		t.Errorf("the master reads %v hidden, want Toren (one creature has not noticed)", got)
	}

	// Toren's player reads their own roll and that they are hidden, never who noticed or any Perception.
	mine := c.state(t, a.caio)
	if len(mine.GetHideAttempts()) != 1 || mine.GetHideAttempts()[0].GetRoll().GetTotal() != 7 || !slices.Equal(mine.GetHiddenIds(), []string{c.id(t, "Toren")}) {
		t.Errorf("Toren's state = %v, want their roll and that they are hidden", mine)
	}
	for who, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia} {
		raw := asJSON(t, c.state(t, u)) + asJSON(t, &playv1.GetEncounterResponse{Encounter: c.get(t, u)}) + asJSON(t, a.log(t, u, c.e))
		for _, leak := range []string{`"observers"`, `"passivePerception"`, `"noticed"`, `"seesClearly`} {
			if strings.Contains(raw, leak) {
				t.Errorf("%s reads %s: %s", who, leak, raw)
			}
		}
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if s := c.state(t, u); len(s.GetHideAttempts()) != 0 || len(s.GetHiddenIds()) != 0 {
			t.Errorf("%s reads %v of Toren's hiding, want nothing", who, s)
		}
	}
}

// TestMasterRefusesAHideWithItsReasonAndOnlyTheHiderReadsIt (SRD 5.1, "Hiding": the master decides
// whether there is a place to hide): the refusal leaves the hider noticed and the action spent;
// the sentence is the master's and the hider's player's alone.
func TestMasterRefusesAHideWithItsReasonAndOnlyTheHiderReadsIt(t *testing.T) {
	t.Parallel()
	const reason = "LEAKCANARY-hide-refusal-1"
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	tried := c.mustHide(t, a.caio, "Toren", "", 19) // 19 + 3: it would beat everybody, but there is no cover

	// A player does not decide, and an unknown attempt is not found.
	if _, err := c.resolveHide(t, a.caio, tried.GetId(), nil); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player deciding a Hide = %v, want permission_denied", err)
	}
	if _, err := c.resolveHide(t, a.master, newKey(), nil); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an attempt that does not exist = %v, want not_found", err)
	}
	refused := c.mustResolveHide(t, tried.GetId(), func(r *playv1.ResolveHideRequest) { r.Refuse, r.Refusal = true, reason })
	if refused.GetStatus() != playv1.HideAttemptStatus_HIDE_ATTEMPT_STATUS_REFUSED || refused.GetRefusal() != reason {
		t.Fatalf("refused attempt = %v, want refused with the master's sentence", refused)
	}
	if got := c.state(t, a.master).GetHiddenIds(); len(got) != 0 {
		t.Errorf("a refused Hide hides %v", got)
	}
	if used := a.mustOptions(t, a.caio, c.refresh(t), "Toren").GetOptions().GetEconomy().GetAction().GetUsed(); !used {
		t.Error("the action of a refused Hide came back")
	}
	_, err := c.resolveHide(t, a.master, tried.GetId(), nil)
	wantContestBlocked(t, "deciding a Hide twice", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_HIDE_NOT_PENDING)

	// The sentence is read by the master and by Toren's player (the controls), by nobody else.
	reads := func(u *user) string {
		return jsonOf(c.state(t, u)) + jsonOf(c.get(t, u)) + jsonOf(a.log(t, u, c.e))
	}
	for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
		if !strings.Contains(reads(u), reason) {
			t.Errorf("%s does not read the refusal (the control of the leak check)", who)
		}
	}
	for who, u := range map[string]*user{"Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if got := reads(u); strings.Contains(got, reason) {
			t.Errorf("%s reads the refusal of a Hide that is not theirs: %s", who, got)
		}
	}
	// The log: the lines of a Hide are the master's.
	for who, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana} {
		for _, e := range contestLogEntries(a.log(t, u, c.e)) {
			if l := e.GetContest().GetLine(); l == playv1.ContestLogLine_CONTEST_LOG_LINE_HIDE_TRIED || l == playv1.ContestLogLine_CONTEST_LOG_LINE_HIDE_REFUSED {
				t.Errorf("%s reads the log line %v", who, l)
			}
		}
	}
	var master bool
	for _, e := range contestLogEntries(a.log(t, a.master, c.e)) {
		master = master || e.GetContest().GetLine() == playv1.ContestLogLine_CONTEST_LOG_LINE_HIDE_REFUSED
	}
	if !master {
		t.Error("the master does not read the refusal in the log")
	}
}

// TestMasterSaysACreatureSeesTheHiderClearly (SRD 5.1, "Hiding": cover the app does not see): the
// creature the master names notices the hider whatever the totals, and the others still compare.
func TestMasterSaysACreatureSeesTheHiderClearly(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	hob, goblin := c.id(t, c.hob), c.id(t, "Goblin")

	tried := c.mustHide(t, a.caio, "Toren", cunningHide, 18) // 21: beats 10 whatever
	applied := c.mustResolveHide(t, tried.GetId(), func(r *playv1.ResolveHideRequest) { r.SeesClearlyIds = []string{hob} })
	if o := observerOf(t, applied, hob); !o.GetNoticed() {
		t.Errorf("the Hobgoblin the master said sees clearly = %v, want it noticing", o)
	}
	if o := observerOf(t, applied, goblin); o.GetNoticed() {
		t.Errorf("the Goblin = %v, want it deceived by 21", o)
	}
	if got := c.state(t, a.caio).GetHiddenIds(); len(got) != 1 {
		t.Errorf("Toren is hidden from the Goblin still: %v", got)
	}
	// The creature that sees clearly gives no advantage.
	if tg := targetOf(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), rapier, c.hob); tg == nil || len(tg.GetSources()) != 0 {
		t.Errorf("the Hobgoblin as a target = %v, want no unseen attacker against a creature that sees clearly", tg)
	}

	// Every creature seeing clearly leaves the hider noticed by all.
	c.advance(t, "Goblin")
	c.advance(t, "Toren")
	again := c.mustHide(t, a.caio, "Toren", cunningHide, 18)
	c.mustResolveHide(t, again.GetId(), func(r *playv1.ResolveHideRequest) { r.SeesClearlyIds = []string{hob, goblin, hob} })
	if got := c.state(t, a.master).GetHiddenIds(); len(got) != 0 {
		t.Errorf("hidden from nobody: the master reads %v hidden", got)
	}
}

// TestAttackFromHidingHasAdvantageAndGivesThePositionAwayHitOrMiss (SRD 5.1, "Unseen Attackers and
// Targets": an attacker that is unseen has advantage, and the attack gives its position away, hit
// or miss). The master's undo of the attack gives the hiding back.
func TestAttackFromHidingHasAdvantageAndGivesThePositionAwayHitOrMiss(t *testing.T) {
	t.Parallel()
	for name, faces := range map[string][]int32{"hit": {2, 19}, "miss": {2, 3}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newSneaks(t)
			c := a.arena(t, arenaPlan{})
			tried := c.mustHide(t, a.caio, "Toren", cunningHide, 12) // 15 beats both
			c.mustResolveHide(t, tried.GetId(), nil)
			toren := c.id(t, "Toren")

			res := a.mustAttack(t, a.caio, c.refresh(t), "Toren", rapier, c.hob, func(r *playv1.RollAttackRequest) { r.D20Faces = faces })
			roll := res.GetRoll()
			if roll.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || len(roll.GetD20().GetFaces()) != 2 {
				t.Fatalf("the attack from hiding = %v, want advantage over two d20", roll)
			}
			if (name == "hit") != (roll.GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_HIT || roll.GetOutcome() == playv1.AttackOutcome_ATTACK_OUTCOME_CRITICAL_HIT) {
				t.Fatalf("outcome = %v, want a %s", roll.GetOutcome(), name)
			}
			var source bool
			for _, s := range roll.GetSources() {
				source = source || s.GetKind() == playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_UNSEEN_ATTACKER
			}
			if !source {
				t.Errorf("the attack's sources = %v, want the unseen attacker", roll.GetSources())
			}
			for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
				if got := c.state(t, u).GetHiddenIds(); len(got) != 0 {
					t.Errorf("%s reads %v hidden after the attack, want the hiding ended", who, got)
				}
			}
			// Other players never read that Toren was hidden: the line of the end is not in their log.
			for _, e := range contestLogEntries(a.log(t, a.ana, c.e)) {
				if e.GetContest().GetLine() == playv1.ContestLogLine_CONTEST_LOG_LINE_HIDE_APPLIED {
					t.Errorf("Pensantus's player reads the master's decision: %v", e)
				}
			}
			if name != "miss" {
				return
			}
			if err := a.undo(t, a.master, c.refresh(t), a.log(t, a.master, c.e).GetUndoableEventId()); err != nil {
				t.Fatalf("UndoLastAction(attack) error = %v", err)
			}
			for who, u := range map[string]*user{"the master": a.master, "Toren's player": a.caio} {
				if got := c.state(t, u).GetHiddenIds(); !slices.Equal(got, []string{toren}) {
					t.Errorf("%s reads %v hidden after the undo, want Toren hidden again", who, got)
				}
			}
			if tg := targetOf(a.mustOptions(t, a.caio, c.refresh(t), "Toren"), rapier, c.hob); tg.GetRollMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE {
				t.Errorf("after the undo the attack is %v, want advantage again", tg)
			}
		})
	}
}

// TestCastingASpellFromHidingEndsTheHidingAndTheUndoGivesItBack (SRD 5.1, "Unseen Attackers and
// Targets": casting a spell gives the position away).
func TestCastingASpellFromHidingEndsTheHidingAndTheUndoGivesItBack(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	pens := c.id(t, "Pensantus")
	c.advance(t, "Pensantus")
	tried := c.mustHide(t, a.ana, "Pensantus", "", 14) // Stealth +2: 16
	if tried.GetRoll().GetModifier() != 2 {
		t.Fatalf("Pensantus's Stealth = %+d, want +2", tried.GetRoll().GetModifier())
	}
	c.mustResolveHide(t, tried.GetId(), nil)
	if got := c.state(t, a.ana).GetHiddenIds(); !slices.Equal(got, []string{pens}) {
		t.Fatalf("Pensantus reads %v hidden, want herself", got)
	}

	c.advance(t, "Goblin")
	c.advance(t, "Pensantus")
	a.mustCast(t, a.ana, c.refresh(t), "Pensantus", magicMissileSpell, slotOfLevel(1), []*playv1.SpellTarget{darts(a, t, c.hob, 3)}, noCastRoll)
	for who, u := range map[string]*user{"the master": a.master, "Pensantus's player": a.ana} {
		if got := c.state(t, u).GetHiddenIds(); len(got) != 0 {
			t.Errorf("%s reads %v hidden after the cast, want the hiding ended", who, got)
		}
	}
	if err := a.undo(t, a.master, c.refresh(t), a.log(t, a.master, c.e).GetUndoableEventId()); err != nil {
		t.Fatalf("UndoLastAction(cast) error = %v", err)
	}
	if got := c.state(t, a.ana).GetHiddenIds(); !slices.Equal(got, []string{pens}) {
		t.Errorf("after the undo Pensantus reads %v hidden, want herself", got)
	}
}

// TestHideCostsTheActionAndOnlyTheCombatantsOwnPlayerOnItsTurnMayRollIt: the standard Hide spends
// the action, the master hides an NPC and nobody else reads that it is hidden.
func TestHideCostsTheActionAndOnlyTheCombatantsOwnPlayerOnItsTurnMayRollIt(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})

	_, err := c.hide(t, a.bia, "Brisa", "", 10)
	wantBlockedBy(t, "hiding off turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN)
	if _, err := c.hide(t, a.bia, "Toren", "", 10); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("hiding with another player's combatant = %v, want permission_denied", err)
	}
	if _, err := c.hide(t, a.caio, "Toren", "standard:dash", 10); err == nil {
		t.Error("a Hide with the key of another action was taken")
	}
	c.mustHide(t, a.caio, "Toren", "", 10)
	_, err = c.hide(t, a.caio, "Toren", "", 10)
	wantBlockedBy(t, "a second Hide with the action spent", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED)
	if used := a.mustOptions(t, a.caio, c.refresh(t), "Toren").GetOptions().GetEconomy(); !used.GetAction().GetUsed() || used.GetBonusAction().GetUsed() {
		t.Errorf("economy = %v, want the action spent and the bonus action free", used)
	}

	// The master hides the Hobgoblin on its turn: the players read nothing of it.
	c.advance(t, c.hob)
	npc := c.mustHide(t, a.master, c.hob, "", 15)
	c.mustResolveHide(t, npc.GetId(), nil)
	if got := c.state(t, a.master).GetHiddenIds(); !slices.Equal(got, []string{c.id(t, c.hob)}) {
		t.Errorf("the master reads %v hidden, want the Hobgoblin", got)
	}
	for who, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia} {
		if s := c.state(t, u); len(s.GetHiddenIds()) != 0 || len(s.GetHideAttempts()) != 0 {
			t.Errorf("%s reads %v of the Hobgoblin's hiding, want nothing", who, s)
		}
	}
}

// TestHideAndItsDecisionAreIdempotent: the same key and request answers the same attempt and
// changes nothing more (the roll is not made again); the same key with another request is refused.
func TestHideAndItsDecisionAreIdempotent(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	hide := &playv1.HideRequest{CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, "Toren"), ActionKey: cunningHide, Roll: rollIn()}
	c.h.roller.queue(7)
	first, err := a.caio.contests.Hide(t.Context(), connect.NewRequest(hide))
	if err != nil {
		t.Fatalf("Hide() error = %v", err)
	}
	c.h.roller.queue(19) // a retry that rolled again would read 22
	again, err := a.caio.contests.Hide(t.Context(), connect.NewRequest(hide))
	if err != nil {
		t.Fatalf("Hide(retry) error = %v", err)
	}
	if again.Msg.GetAttempt().GetId() != first.Msg.GetAttempt().GetId() || again.Msg.GetAttempt().GetRoll().GetTotal() != 10 {
		t.Errorf("the retry = %v, want the same attempt with the same total as %v", again.Msg.GetAttempt(), first.Msg.GetAttempt())
	}
	if n := len(c.state(t, a.master).GetHideAttempts()); n != 1 {
		t.Errorf("%d attempts after a retry, want 1", n)
	}
	other := &playv1.HideRequest{CampaignId: hide.GetCampaignId(), EncounterId: hide.GetEncounterId(), IdempotencyKey: hide.GetIdempotencyKey(), CombatantId: hide.GetCombatantId(), ActionKey: cunningHide, Roll: faces(15)}
	_, err = a.caio.contests.Hide(t.Context(), connect.NewRequest(other))
	wantCode(t, "the same key with another roll", err, connect.CodeInvalidArgument)

	resolve := &playv1.ResolveHideRequest{CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), AttemptId: first.Msg.GetAttempt().GetId(), Refuse: true, Refusal: "Sem onde se esconder."}
	if _, err := a.master.contests.ResolveHide(t.Context(), connect.NewRequest(resolve)); err != nil {
		t.Fatalf("ResolveHide() error = %v", err)
	}
	replayed, err := a.master.contests.ResolveHide(t.Context(), connect.NewRequest(resolve))
	if err != nil || replayed.Msg.GetAttempt().GetStatus() != playv1.HideAttemptStatus_HIDE_ATTEMPT_STATUS_REFUSED {
		t.Errorf("ResolveHide(retry) = %v, %v, want the same refusal", replayed, err)
	}
	flipped := &playv1.ResolveHideRequest{CampaignId: resolve.GetCampaignId(), EncounterId: resolve.GetEncounterId(), IdempotencyKey: resolve.GetIdempotencyKey(), AttemptId: resolve.GetAttemptId()}
	_, err = a.master.contests.ResolveHide(t.Context(), connect.NewRequest(flipped))
	wantCode(t, "the same key with the opposite decision", err, connect.CodeInvalidArgument)
}

// TestHideCallsRefuseWhoIsNotInTheCampaign: a stranger, a pending member and the master of another
// campaign (whatever campaign id they send) are refused with not_found, and a player cannot decide.
func TestHideCallsRefuseWhoIsNotInTheCampaign(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{})
	o := a.outsiders(t)
	o.refused(t, "GetContestState", func(u *user, campaignID string) error {
		_, err := u.contests.GetContestState(t.Context(), connect.NewRequest(&playv1.GetContestStateRequest{CampaignId: campaignID, EncounterId: c.e.GetId()}))
		return err
	}, a.campaignID)
	o.refused(t, "Hide", func(u *user, campaignID string) error {
		_, err := u.contests.Hide(t.Context(), connect.NewRequest(&playv1.HideRequest{
			CampaignId: campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), CombatantId: c.id(t, "Toren"), Roll: rollIn(),
		}))
		return err
	}, a.campaignID)
	o.refused(t, "ResolveHide", func(u *user, campaignID string) error {
		_, err := u.contests.ResolveHide(t.Context(), connect.NewRequest(&playv1.ResolveHideRequest{
			CampaignId: campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), AttemptId: newKey(), Refuse: true,
		}))
		return err
	}, a.campaignID)
}
