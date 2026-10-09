package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The contests and the special actions of a combat (SRD 5.1, "Contests", "Grappling",
// "Shoving a Creature", "Hide", "Help", "Group Checks", "Surprise"). These tests need the
// database (MEURPG_TEST_DATABASE_URL). The fixture is the party of the canonical fight (Toren,
// Pensantus and Brisa) against a Goblin (a basic sheet with no skills) and a Hobgoblin, an SRD
// creature: Strength 13 and Dexterity 12, so Athletics and Acrobatics are +1, passive
// Perception 10.

// cx is the arena: the party and the enemies on the map, the combat begun.
type cx struct {
	*armed
	e *playv1.Encounter
	// hob is the Hobgoblin's label.
	hob string
}

// arenaPlan says where everybody stands and what the master adds.
type arenaPlan struct {
	// at overrides the squares: the default is Toren (3,3) with the Hobgoblin on his east
	// (4,3), Pensantus (3,6), Brisa (8,8) and the Goblin (9,3).
	at map[string][2]int32
	// initiative of the players' d20s, in the order Toren, Brisa, Pensantus.
	toren, brisa, pens int32
	// monster is the SRD creature added besides the Goblin; "" is the Hobgoblin.
	monster string
	// setup leaves the combat in SETUP.
	setup bool
}

func (a *armed) arena(t *testing.T, p arenaPlan) *cx {
	t.Helper()
	if p.toren == 0 {
		p.toren, p.brisa, p.pens = 20, 15, 12
	}
	creature := hobgoblin
	if p.monster != "" {
		creature = p.monster
	}
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{1},
		players: map[string]int32{"Toren": p.toren, "Brisa": p.brisa, "Pensantus": p.pens},
		reveal:  []string{"Goblin"},
		setup:   true,
	})
	a.h.roller.queue(8)
	added, err := a.addMonsters(t, e, newKey(), func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Hidden = creature, 1, proto.Bool(false)
	})
	if err != nil {
		t.Fatalf("AddMonsters() error = %v", err)
	}
	e = added.GetEncounter()
	hob := ""
	for _, c := range e.GetCombatants() {
		if c.GetId() == added.GetCombatantIds()[0] {
			hob = c.GetLabel()
		}
	}
	at := map[string][2]int32{"Toren": {3, 3}, hob: {4, 3}, "Pensantus": {3, 6}, "Brisa": {8, 8}, "Goblin": {9, 3}}
	for label, sq := range p.at {
		if label == "Hobgoblin" {
			label = hob
		}
		at[label] = sq
	}
	for label, sq := range at {
		if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: byLabel(t, e, label).GetId(), IdempotencyKey: newKey(), Col: sq[0], Row: sq[1],
		})); err != nil {
			t.Fatalf("MoveCombatant(%s) error = %v", label, err)
		}
	}
	c := &cx{armed: a, hob: hob}
	if p.setup {
		c.e = a.get(t, a.master)
		return c
	}
	c.e = a.begin(t, a.get(t, a.master))
	return c
}

// refresh reads the combat again as the master.
func (c *cx) refresh(t *testing.T) *playv1.Encounter {
	t.Helper()
	c.e = c.get(t, c.master)
	return c.e
}

// rollIn is the app's roll; faces are the typed physical dice.
func rollIn() *playv1.CheckRollInput {
	return &playv1.CheckRollInput{Roll: &playv1.CheckRollInput_RollInApp{RollInApp: true}}
}

func faces(f ...int32) *playv1.CheckRollInput {
	return &playv1.CheckRollInput{Roll: &playv1.CheckRollInput_D20Faces{D20Faces: &playv1.D20Faces{Faces: f}}}
}

// start calls StartContest as u.
func (c *cx) startContest(t *testing.T, u *user, edit func(*playv1.StartContestRequest)) (*playv1.StartContestResponse, error) {
	t.Helper()
	req := &playv1.StartContestRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), Roll: rollIn()}
	edit(req)
	res, err := u.contests.StartContest(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cx) mustStart(t *testing.T, u *user, edit func(*playv1.StartContestRequest)) *playv1.StartContestResponse {
	t.Helper()
	res, err := c.startContest(t, u, edit)
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	return res
}

// grapple is StartContest GRAPPLE by label, with the d20 the roller will give.
func (c *cx) grapple(t *testing.T, u *user, initiator, target string, face int) (*playv1.StartContestResponse, error) {
	t.Helper()
	c.h.roller.queue(face)
	return c.startContest(t, u, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, initiator), c.id(t, target), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
}

func (c *cx) respond(t *testing.T, u *user, contestID string, skill playv1.ContestSkill, face int) (*playv1.RespondContestResponse, error) {
	t.Helper()
	c.h.roller.queue(face)
	res, err := u.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: contestID, Skill: skill, Roll: rollIn(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cx) mustRespond(t *testing.T, u *user, contestID string, skill playv1.ContestSkill, face int) *playv1.RespondContestResponse {
	t.Helper()
	res, err := c.respond(t, u, contestID, skill, face)
	if err != nil {
		t.Fatalf("RespondContest() error = %v", err)
	}
	return res
}

func (c *cx) state(t *testing.T, u *user) *playv1.GetContestStateResponse {
	t.Helper()
	res, err := u.contests.GetContestState(t.Context(), connect.NewRequest(&playv1.GetContestStateRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId()}))
	if err != nil {
		t.Fatalf("GetContestState() error = %v", err)
	}
	return res.Msg
}

func (c *cx) combatant(t *testing.T, u *user, label string) *playv1.Combatant {
	t.Helper()
	return byLabel(t, c.get(t, u), label)
}

func (c *cx) hasCondition(t *testing.T, label, key string) bool {
	t.Helper()
	for _, k := range c.combatant(t, c.master, label).GetConditions() {
		if k == key {
			return true
		}
	}
	return false
}

// wantContestBlocked checks a failed_precondition with the ContestBlocked detail.
func wantContestBlocked(t *testing.T, call string, err error, want playv1.ContestBlockedReason) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: no error, want ContestBlocked %v", call, want)
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("%s: code = %v (%v), want failed_precondition", call, connect.CodeOf(err), err)
	}
	var got *playv1.ContestBlocked
	for _, d := range err.(*connect.Error).Details() {
		v, derr := d.Value()
		if derr != nil {
			continue
		}
		if b, ok := v.(*playv1.ContestBlocked); ok {
			got = b
		}
	}
	if got == nil || got.GetReason() != want {
		t.Fatalf("%s: reason = %v, want %v (%v)", call, got.GetReason(), want, err)
	}
}

// grappleOptions is the Grappling and Shoving lines of a combatant's turn.
func grappleOption(o *playv1.GetTurnOptionsResponse, kind playv1.ContestAttackOptionKind) *playv1.ContestAttackOption {
	for _, a := range o.GetContestAttackOptions() {
		if a.GetKind() == kind {
			return a
		}
	}
	return nil
}

// TestGrappleBeatingTheTargetsRollGrabsItAndTheMasterAnswersForTheNPC (SRD 5.1, "Grappling",
// "Contests"): Toren grapples the Hobgoblin with Strength (Athletics); the contest waits for the
// master, who picks the Hobgoblin's skill and rolls; the higher total wins. A player never reads
// the Hobgoblin's total, skill or modifier, only who won.
func TestGrappleBeatingTheTargetsRollGrabsItAndTheMasterAnswersForTheNPC(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})

	// Toren is Athletics +3 (Strength 16): d20 15 makes 18.
	started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(15)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	contest := started.GetContest()
	if contest.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_DEFENDER || contest.GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_MASTER {
		t.Fatalf("contest = %v, want awaiting the master", contest)
	}
	if roll := contest.GetInitiatorRoll(); roll.GetTotal() != 18 || roll.GetFaces()[0] != 15 {
		t.Errorf("Toren's roll = %v, want d20 15 for 18", roll)
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Fatal("the Hobgoblin is Grappled before it answered")
	}
	// The action went with the attack.
	if o := a.mustOptions(t, a.caio, c.e, "Toren"); grappleOption(o, playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE).GetEnabled() {
		t.Error("a second grapple is offered with the action spent")
	}

	// The master sees the contest, the suggestion and what each skill would roll.
	masterView := c.state(t, a.master).GetContests()[0]
	if !masterView.GetYouAnswer() || len(masterView.GetAnswerOptions()) != 2 {
		t.Fatalf("master's contest = %v, want to answer with two skills", masterView)
	}
	for _, o := range masterView.GetAnswerOptions() {
		if o.GetModifier() != 1 || !o.GetKnown() {
			t.Errorf("option %v, want +1 (Strength 13, Dexterity 12) and known", o)
		}
	}
	// Toren cannot answer for the Hobgoblin.
	if _, err := a.caio.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: contest.GetId(),
		Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: rollIn(),
	})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player answering for an NPC = %v, want permission_denied", err)
	}

	// The Hobgoblin rolls Athletics 9 + 1 = 10 and loses.
	answered := c.mustRespond(t, a.master, contest.GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 9)
	if got := answered.GetContest(); got.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_INITIATOR || got.GetDefenderRoll().GetTotal() != 10 {
		t.Fatalf("answered contest = %v, want Toren to win against 10", got)
	}
	if !c.hasCondition(t, c.hob, condGrappled) {
		t.Fatal("the Hobgoblin is not Grappled after losing")
	}

	// Toren reads that he won, never the Hobgoblin's roll, skill or numbers (RN-20).
	for _, view := range c.state(t, a.caio).GetContests() {
		if view.GetDefenderRoll() != nil || view.GetEscapeDc() != 0 || len(view.GetAnswerOptions()) != 0 {
			t.Errorf("Toren's contest = %v, want no roll of the Hobgoblin", view)
		}
		if view.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_INITIATOR {
			t.Errorf("Toren reads winner %v, want INITIATOR", view.GetWinner())
		}
	}
	if raw := asJSON(t, &playv1.GetContestStateResponse{Contests: c.state(t, a.caio).GetContests()}); strings.Contains(raw, `"total":10`) {
		t.Errorf("Toren's state carries the Hobgoblin's total: %s", raw)
	}
	for _, who := range []*user{a.ana, a.bia} {
		if n := len(c.state(t, who).GetContests()); n != 0 {
			t.Errorf("a player outside the contest reads %d contests, want none", n)
		}
	}

	// The log: "Toren agarrou o Hobgoblin", for everyone, with no number.
	for who, u := range map[string]*user{"master": a.master, "Toren": a.caio, "Pensantus": a.ana} {
		var found bool
		for _, e := range logEntries(a.log(t, u, c.e)) {
			if e.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_CONTEST && e.GetContest().GetLine() == playv1.ContestLogLine_CONTEST_LOG_LINE_GRAPPLED {
				found = true
				if e.GetActorLabel() != "Toren" || e.GetTargetLabel() != c.hob {
					t.Errorf("%s reads %v, want Toren grappling the %s", who, e, c.hob)
				}
			}
		}
		if !found {
			t.Errorf("%s reads no grapple line", who)
		}
	}
}

// TestGrappleTieOrLossChangesNothing (SRD 5.1, "Contests": a tie leaves the situation as it was).
func TestGrappleTieOrLossChangesNothing(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})

	// Toren 10 + 3 = 13 against the Hobgoblin's 12 + 1 = 13: a tie.
	started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(10)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	answered := c.mustRespond(t, a.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ACROBATICS, 12)
	if got := answered.GetContest().GetWinner(); got != playv1.ContestWinner_CONTEST_WINNER_TIE {
		t.Fatalf("winner = %v, want TIE", got)
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("a tie grappled the Hobgoblin")
	}
	var failed bool
	for _, e := range logEntries(a.log(t, a.caio, c.e)) {
		failed = failed || e.GetContest().GetLine() == playv1.ContestLogLine_CONTEST_LOG_LINE_GRAPPLE_FAILED
	}
	if !failed {
		t.Error("the log has no \"não conseguiu agarrar\" line for the tie")
	}
}

// TestGrappleAndShoveAreOneOfTheAttacksOfTheAttackAction (SRD 5.1, "Grappling": the special attack
// replaces one of the attacks): the first spends the action, a second needs Extra Attack.
func TestGrappleAndShoveAreOneOfTheAttacksOfTheAttackAction(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, scores16(), []string{battleaxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1, scores16(), nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
	})
	c := a.arena(t, arenaPlan{})

	opts := a.mustOptions(t, a.caio, c.e, "Toren")
	grapple := grappleOption(opts, playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE)
	if !grapple.GetEnabled() || !grapple.GetReplacesAttack() || grapple.GetAttacksLeft() != 2 {
		t.Fatalf("grapple option = %v, want enabled, replacing an attack, 2 attacks left", grapple)
	}
	first := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(12)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	c.mustRespond(t, a.master, first.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2)

	// The second attack of the Attack action is a shove.
	opts = a.mustOptions(t, a.caio, c.e, "Toren")
	if shove := grappleOption(opts, playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_SHOVE); !shove.GetEnabled() || shove.GetAttacksLeft() != 1 {
		t.Fatalf("shove option = %v, want enabled with 1 attack left", shove)
	}
	second := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(12)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE
	})
	won := c.mustRespond(t, a.master, second.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2)
	if won.GetContest().GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_OUTCOME || !won.GetContest().GetYouChoose() {
		t.Fatalf("a won shove = %v, want it to wait for its winner's choice", won.GetContest())
	}
	// A shove that waits for its outcome holds the shover's next contest.
	_, err := c.grapple(t, a.caio, "Toren", c.hob, 12)
	wantContestBlocked(t, "a contest while the shove waits", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_CONTEST_OPEN)
	c.mustShove(t, a.caio, second.GetContest().GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE)

	// The Attack action made all its attacks.
	_, err = c.grapple(t, a.caio, "Toren", c.hob, 12)
	wantBlockedBy(t, "a third special attack", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ATTACKS_USED)
}

// TestGrappleRefusesATargetTooBigOutOfReachOrNotYours (SRD 5.1, "Grappling": no more than one size
// larger, within reach). The master has the last word.
func TestGrappleRefusesATargetTooBigOutOfReachOrNotYours(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{monster: "monster:giant-constrictor-snake"})

	// The snake is Huge, Toren is Medium.
	_, err := c.grapple(t, a.caio, "Toren", c.hob, 10)
	wantContestBlocked(t, "grappling a Huge snake", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_TARGET_TOO_BIG)
	opts := a.mustOptions(t, a.caio, c.e, "Toren")
	var listed bool
	for _, tg := range grappleOption(opts, playv1.ContestAttackOptionKind_CONTEST_ATTACK_OPTION_KIND_GRAPPLE).GetTargets() {
		if tg.GetCombatantId() == c.id(t, c.hob) {
			listed = true
			if tg.GetEligible() || tg.GetReason() != playv1.ContestTargetReason_CONTEST_TARGET_REASON_TOO_BIG {
				t.Errorf("the snake = %v, want not eligible: too big", tg)
			}
		}
	}
	if !listed {
		t.Error("the Huge snake next to Toren is not listed with its reason")
	}

	// A Goblin 9 squares away is not even listed, and attacking it is refused.
	_, err = c.grapple(t, a.caio, "Toren", "Goblin", 10)
	wantBlockedBy(t, "grappling the far Goblin", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_OUT_OF_REACH)

	// Not on turn: Brisa cannot start a contest on Toren's turn.
	_, err = c.grapple(t, a.bia, "Brisa", "Goblin", 10)
	wantBlockedBy(t, "grappling off turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN)

	// Toren's player cannot start one for Brisa, and a stranger to the combat is refused.
	if _, err := c.grapple(t, a.caio, "Brisa", "Goblin", 10); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("grappling for another's combatant = %v, want permission_denied", err)
	}
}

// scores16 are the scores of a strong fighter: Strength 16, so Athletics +3.
func scores16() *rulesv1.AbilityScores {
	return &rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}
}

// logEntries are every entry of the log, the latest first.
func logEntries(l *playv1.ListCombatLogResponse) []*playv1.CombatLogEntry {
	var out []*playv1.CombatLogEntry
	for _, r := range l.GetRounds() {
		out = append(out, r.GetEntries()...)
	}
	return out
}

// resolveShove calls ResolveShove as u.
func (c *cx) resolveShove(t *testing.T, u *user, contestID string, outcome playv1.ShoveOutcome) (*playv1.ResolveShoveResponse, error) {
	t.Helper()
	res, err := u.contests.ResolveShove(t.Context(), connect.NewRequest(&playv1.ResolveShoveRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: contestID, Outcome: outcome,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cx) mustShove(t *testing.T, u *user, contestID string, outcome playv1.ShoveOutcome) *playv1.ResolveShoveResponse {
	t.Helper()
	res, err := c.resolveShove(t, u, contestID, outcome)
	if err != nil {
		t.Fatalf("ResolveShove(%v) error = %v", outcome, err)
	}
	return res
}
