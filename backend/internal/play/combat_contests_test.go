package play

import (
	"errors"
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
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("%s: error = %v, want a connect error", call, err)
	}
	for _, d := range ce.Details() {
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
		for _, e := range contestLogEntries(a.log(t, u, c.e)) {
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
	for _, e := range contestLogEntries(a.log(t, a.caio, c.e)) {
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

// contestLogEntries are every entry of the log, the latest first.
func contestLogEntries(l *playv1.ListCombatLogResponse) []*playv1.CombatLogEntry {
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

// advance passes the turns until it is the combatant's.
func (c *cx) advance(t *testing.T, label string) *playv1.Encounter {
	t.Helper()
	want := c.id(t, label)
	for range 12 {
		e := c.refresh(t)
		if e.GetCurrentCombatantId() == want {
			return e
		}
		if _, err := c.master.combat.EndTurn(t.Context(), connect.NewRequest(&playv1.EndTurnRequest{
			CampaignId: c.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: e.GetCurrentCombatantId(), ExpectedRound: e.GetRound(),
		})); err != nil {
			t.Fatalf("EndTurn() error = %v", err)
		}
	}
	t.Fatalf("the turn never reached %s", label)
	return nil
}

// moveTo calls MoveCombatant as u.
func (c *cx) moveTo(t *testing.T, u *user, label string, col, row int32) (*playv1.MoveCombatantResponse, error) {
	t.Helper()
	res, err := u.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), CombatantId: c.id(t, label), IdempotencyKey: newKey(), Col: col, Row: row,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (c *cx) moveOptions(t *testing.T, u *user, label string) *playv1.GetMoveOptionsResponse {
	t.Helper()
	res, err := u.combat.GetMoveOptions(t.Context(), connect.NewRequest(&playv1.GetMoveOptionsRequest{
		CampaignId: c.campaignID, EncounterId: c.e.GetId(), CombatantId: c.id(t, label),
	}))
	if err != nil {
		t.Fatalf("GetMoveOptions(%s) error = %v", label, err)
	}
	return res.Msg
}

// square is where a combatant stands, as the master sees it.
func (c *cx) square(t *testing.T, label string) [2]int32 {
	t.Helper()
	cb := c.combatant(t, c.master, label)
	return [2]int32{cb.GetCol(), cb.GetRow()}
}

// grappleWon makes Toren win a grapple on the Hobgoblin (d20 18 against 3).
func (c *cx) grappleWon(t *testing.T) {
	t.Helper()
	started := c.mustStart(t, c.caio, func(r *playv1.StartContestRequest) {
		c.h.roller.queue(18)
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	c.mustRespond(t, c.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2)
	if !c.hasCondition(t, c.hob, condGrappled) {
		t.Fatal("the grapple did not take")
	}
}

// grappledByAttack makes the Hobgoblin grapple a combatant with a fixed escape DC, the master's
// word after its attack hit (SRD 5.1, the giant constrictor snake's "escape DC 16").
func (c *cx) grappledByAttack(t *testing.T, victim string, dc int32) {
	t.Helper()
	c.advance(t, c.hob)
	c.mustStart(t, c.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, victim), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
		r.Kind, r.EscapeDc, r.Roll = playv1.ContestKind_CONTEST_KIND_ESCAPE_DC, dc, nil
	})
}

// TestEscapeAgainstAFixedDCNeverShowsTheDC (SRD 5.1, "Escaping a Grapple" and the monsters' "escape
// DC"): a grapple that came from an attack is escaped with Athletics or Acrobatics against its DC,
// the grappler rolls nothing; Brisa reads only whether she escaped (RN-20).
func TestEscapeAgainstAFixedDCNeverShowsTheDC(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.grappledByAttack(t, "Brisa", 16)
	if !c.hasCondition(t, "Brisa", condGrappled) {
		t.Fatal("Brisa is not Grappled by the attack")
	}
	if s := c.state(t, a.master).GetGrapples(); len(s) != 1 || s[0].GetEscapeDc() != 16 {
		t.Fatalf("the master reads grapples %v, want one with the DC 16", s)
	}
	for _, u := range []*user{a.bia, a.caio} {
		for _, g := range c.state(t, u).GetGrapples() {
			if g.GetEscapeDc() != 0 {
				t.Errorf("a player reads the escape DC: %v", g)
			}
		}
	}

	// Brisa's turn: grappled, speed 0 for movement, Escape offered.
	c.advance(t, "Brisa")
	o := a.mustOptions(t, a.bia, c.e, "Brisa")
	st := o.GetContestState()
	if !st.GetGrappled() || !st.GetCanEscape() || len(st.GetEscapeOptions()) != 2 {
		t.Fatalf("Brisa's contest state = %v, want grappled and able to escape with two skills", st)
	}
	if st.GetGrapplerId() != c.id(t, c.hob) {
		t.Errorf("grappler = %s, want the Hobgoblin she sees", st.GetGrapplerId())
	}
	if left := o.GetOptions().GetEconomy().GetMovement().GetLeftDft(); left != 0 {
		t.Errorf("movement left = %d, a grappled creature's speed is 0", left)
	}

	// Acrobatics +3 (Dexterity 16): d20 12 is 15, under the DC 16: still grappled, the action spent.
	c.h.roller.queue(12)
	failed := c.mustStart(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose, r.Skill = c.id(t, "Brisa"), playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE, playv1.ContestSkill_CONTEST_SKILL_ACROBATICS
	})
	if got := failed.GetContest(); got.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_RESOLVED || got.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_DEFENDER || got.GetEscapeDc() != 0 {
		t.Fatalf("failed escape = %v, want resolved for the grappler with no DC in it", got)
	}
	if !c.hasCondition(t, "Brisa", condGrappled) {
		t.Fatal("a failed escape freed her")
	}
	c.h.roller.queue(20)
	_, err := c.startContest(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose = c.id(t, "Brisa"), playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE
	})
	wantBlockedBy(t, "a second escape in the turn", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ACTION_USED)

	// The next round: 14 + 3 = 17 reaches the DC.
	c.advance(t, c.hob)
	c.advance(t, "Brisa")
	c.h.roller.queue(14)
	escaped := c.mustStart(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose, r.Skill = c.id(t, "Brisa"), playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE, playv1.ContestSkill_CONTEST_SKILL_ACROBATICS
	})
	if escaped.GetContest().GetWinner() != playv1.ContestWinner_CONTEST_WINNER_INITIATOR {
		t.Fatalf("escape = %v, want it to work", escaped.GetContest())
	}
	if c.hasCondition(t, "Brisa", condGrappled) {
		t.Error("Brisa is still Grappled after escaping")
	}
	if g := c.state(t, a.master).GetGrapples(); len(g) != 0 {
		t.Errorf("the hold stays after the escape: %v", g)
	}
}

// TestAPlayerChoosesTheSkillWhenAnNPCGrappleHer (SRD 5.1, "Grappling": the target chooses Athletics
// or Acrobacia): the master rolls for the Hobgoblin, the player picks and rolls, the totals are
// compared; and escaping is contested by the grappler's Athletics alone.
func TestAPlayerChoosesTheSkillWhenAnNPCGrapplesHer(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.advance(t, c.hob)

	// The master rolls Athletics for the Hobgoblin: 17 + 1 = 18.
	c.h.roller.queue(17)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})
	mine := c.state(t, a.bia).GetContests()
	if len(mine) != 1 || !mine[0].GetYouAnswer() || mine[0].GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_PLAYER {
		t.Fatalf("Brisa's contests = %v, want one for her to answer", mine)
	}
	if mine[0].GetInitiatorRoll() != nil {
		t.Errorf("Brisa reads the Hobgoblin's roll before she answers: %v", mine[0].GetInitiatorRoll())
	}
	options := map[playv1.ContestSkill]int32{}
	for _, o := range mine[0].GetAnswerOptions() {
		options[o.GetSkill()] = o.GetModifier()
	}
	if options[playv1.ContestSkill_CONTEST_SKILL_ACROBATICS] != 3 || options[playv1.ContestSkill_CONTEST_SKILL_ATHLETICS] != 0 {
		t.Fatalf("Brisa's options = %v, want Acrobacia +3 and Atletismo +0", options)
	}
	// Someone else cannot answer for her, but the master can.
	if _, err := a.caio.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: started.GetContest().GetId(),
		Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: rollIn(),
	})); err == nil {
		t.Error("another player answered for Brisa")
	}
	// Acrobatics 8 + 3 = 11 against 18: she loses and is Grappled.
	answered := c.mustRespond(t, a.bia, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ACROBATICS, 8)
	if got := answered.GetContest(); got.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_INITIATOR || got.GetInitiatorRoll() != nil || got.GetDefenderRoll().GetTotal() != 11 {
		t.Fatalf("answer = %v, want the Hobgoblin to win, Brisa to read only her own 11", got)
	}
	if !c.hasCondition(t, "Brisa", condGrappled) {
		t.Fatal("Brisa is not Grappled")
	}

	// Escape against the Hobgoblin's Athletics (the master rolls it): she wins 12 + 3 = 15 against 5 + 1.
	c.advance(t, "Brisa")
	c.h.roller.queue(12)
	esc := c.mustStart(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose, r.Skill = c.id(t, "Brisa"), playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE, playv1.ContestSkill_CONTEST_SKILL_ACROBATICS
	})
	if esc.GetContest().GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_MASTER {
		t.Fatalf("escape = %v, want it to wait for the master", esc.GetContest())
	}
	masterSide := c.state(t, a.master).GetContests()[0]
	if len(masterSide.GetAnswerOptions()) != 1 || masterSide.GetAnswerOptions()[0].GetSkill() != playv1.ContestSkill_CONTEST_SKILL_ATHLETICS {
		t.Errorf("the grappler answers with Athletics alone, got %v", masterSide.GetAnswerOptions())
	}
	c.mustRespond(t, a.master, esc.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ACROBATICS, 5) // the skill is forced to Athletics
	if c.hasCondition(t, "Brisa", condGrappled) {
		t.Error("Brisa is still Grappled after winning the escape")
	}
}

// TestShoveKnocksProneOrPushesOneSquareAndABlockedPushStays (SRD 5.1, "Shoving a Creature").
func TestShoveKnocksProneOrPushesOneSquareAndABlockedPushStays(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	won := func() *playv1.ContestView {
		started := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
			c.h.roller.queue(18)
			r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE
		})
		return c.mustRespond(t, a.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 2).GetContest()
	}

	// Pushed: the square behind the target, straight away from Toren (4,3) -> (5,3).
	view := won()
	if !view.GetShoveChoice().GetPushAvailable() || !view.GetShoveChoice().GetProneAvailable() {
		t.Fatalf("shove choice = %v, want both", view.GetShoveChoice())
	}
	pushed := c.mustShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH)
	if pushed.GetContest().GetShoveOutcome() != playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH {
		t.Fatalf("outcome = %v", pushed.GetContest())
	}
	if got := c.square(t, c.hob); got != [2]int32{5, 3} {
		t.Fatalf("the Hobgoblin stands on %v, want (5,3)", got)
	}
	// A retry of the same call answers the same and moves nobody again.
	again := &playv1.ResolveShoveRequest{CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: view.GetId(), Outcome: playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE}
	if _, err := a.caio.contests.ResolveShove(t.Context(), connect.NewRequest(again)); err == nil {
		t.Error("a second choice on a shove already resolved was accepted")
	}

	// Next round: knock prone.
	c.advance(t, c.hob)
	c.advance(t, "Toren")
	if _, err := c.moveTo(t, a.master, c.hob, 4, 3); err != nil {
		t.Fatalf("placing the Hobgoblin: %v", err)
	}
	view = won()
	c.mustShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PRONE)
	if !c.hasCondition(t, c.hob, condProne) {
		t.Error("the Hobgoblin is not Derrubado")
	}
	if c.square(t, c.hob) != [2]int32{4, 3} {
		t.Errorf("knocking prone moved the Hobgoblin to %v", c.square(t, c.hob))
	}

	// A creature Toren sees behind the target blocks the push: the Goblin goes to (5,3), behind the
	// Hobgoblin at (4,3).
	c.advance(t, c.hob)
	c.advance(t, "Toren")
	if _, err := c.moveTo(t, a.master, "Goblin", 5, 3); err != nil {
		t.Fatalf("placing the Goblin: %v", err)
	}
	view = won()
	if view.GetShoveChoice().GetPushAvailable() || view.GetShoveChoice().GetPushBlocked() != playv1.ShoveBlockedReason_SHOVE_BLOCKED_REASON_CREATURE {
		t.Fatalf("shove choice = %v, want the push blocked by a creature", view.GetShoveChoice())
	}
	_, err := c.resolveShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_PUSH)
	wantContestBlocked(t, "pushing into a creature", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_PUSH_BLOCKED)
	stays := c.mustShove(t, a.caio, view.GetId(), playv1.ShoveOutcome_SHOVE_OUTCOME_STAYS)
	if stays.GetContest().GetShoveOutcome() != playv1.ShoveOutcome_SHOVE_OUTCOME_STAYS || c.square(t, c.hob) != [2]int32{4, 3} {
		t.Errorf("a blocked push moved the Hobgoblin: %v", c.square(t, c.hob))
	}
	var line bool
	for _, e := range contestLogEntries(a.log(t, a.ana, c.e)) {
		line = line || e.GetContest().GetLine() == playv1.ContestLogLine_CONTEST_LOG_LINE_SHOVE_STAYS
	}
	if !line {
		t.Error("the log has no \"não sai do lugar\" line")
	}
}

// TestDraggingAGrappledCreatureHalvesTheSpeedAndLeavesItBehind (SRD 5.1, "Moving a Grappled
// Creature"): half the speed, the creature on the square the grappler left before its last one,
// and no room is refused.
func TestDraggingAGrappledCreatureHalvesTheSpeedAndLeavesItBehind(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.grappleWon(t)
	if _, err := a.master.combat.EndTurn(t.Context(), connect.NewRequest(&playv1.EndTurnRequest{CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ExpectedCombatantId: c.id(t, "Toren"), ExpectedRound: 1})); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	c.advance(t, c.hob)
	c.advance(t, "Toren")

	opts := c.moveOptions(t, a.caio, "Toren")
	if opts.GetDraggingCombatantId() != c.id(t, c.hob) || !opts.GetDraggingHalved() {
		t.Fatalf("move options = dragging %q halved %v, want the Hobgoblin at half speed", opts.GetDraggingCombatantId(), opts.GetDraggingHalved())
	}
	if opts.GetMovementLeftDft() != 150 {
		t.Errorf("movement left = %d, want 150 (half of 30 ft)", opts.GetMovementLeftDft())
	}
	var north *playv1.ReachableSquare
	for _, r := range opts.GetReachable() {
		if r.GetCol() == 3 && r.GetRow() == 1 {
			north = r
		}
		if r.GetDraggedTo() == nil {
			t.Fatalf("a square without the place of the dragged creature: %v", r)
		}
	}
	if north == nil || north.GetDraggedTo().GetCol() != 3 || north.GetDraggedTo().GetRow() != 2 {
		t.Fatalf("(3,1) = %v, want the Hobgoblin left on (3,2)", north)
	}

	// Four squares cost 40 ft at half speed: too far; two squares fit.
	_, err := c.moveTo(t, a.caio, "Toren", 3, 0)
	if err != nil {
		t.Fatalf("a three-square drag: %v", err)
	}
	if got := c.square(t, "Toren"); got != [2]int32{3, 0} {
		t.Fatalf("Toren stands on %v", got)
	}
	if got := c.square(t, c.hob); got != [2]int32{3, 1} {
		t.Errorf("the Hobgoblin stands on %v, want (3,1), behind Toren", got)
	}
	if !c.hasCondition(t, c.hob, condGrappled) {
		t.Error("dragging ended the grapple")
	}
	for _, u := range []*user{a.master, a.caio} { // the log says the drag spent twice the line
		got := lastMove(t, a.log(t, u, c.refresh(t)))
		if got == nil || got.GetDistanceDft() != 150 || got.GetSpentDft() != 300 || !got.GetMoveDragging() || got.GetMoveCrawling() {
			t.Errorf("drag log = %v, want distance 150, spent 300, dragging", got)
		}
	}
	if used := c.combatant(t, a.caio, "Toren"); used.GetMovementLeftDft() != 0 {
		t.Errorf("movement left = %d, want 0 after dragging three squares", used.GetMovementLeftDft())
	}
}

// TestADragWithNoRoomBehindIsRefused: the square the creature would be left on is taken.
func TestADragWithNoRoomBehindIsRefused(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.grappleWon(t)
	c.advance(t, c.hob)
	c.advance(t, "Toren")
	// Pensantus stands on (3,2), where the Hobgoblin would be left when Toren goes to (3,1).
	if _, err := c.moveTo(t, a.master, "Pensantus", 3, 2); err != nil {
		t.Fatalf("placing Pensantus: %v", err)
	}
	_, err := c.moveTo(t, a.caio, "Toren", 3, 1)
	wantContestBlocked(t, "dragging onto a taken square", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NO_ROOM_TO_DRAG)
	if got := c.square(t, "Toren"); got != [2]int32{3, 3} {
		t.Errorf("a refused drag moved Toren to %v", got)
	}
	var refused bool
	for _, r := range c.moveOptions(t, a.caio, "Toren").GetRefused() {
		refused = refused || (r.GetCol() == 3 && r.GetRow() == 1 && r.GetReason() == playv1.MoveRefusal_MOVE_REFUSAL_NO_ROOM_TO_DRAG)
	}
	if !refused {
		t.Error("the move preview does not refuse (3,1) for lack of room")
	}
}

// TestAGrappleEndsWhenTheGrapplerIsIncapacitatedOrTheTargetLeavesItsReach (SRD 5.1, Conditions,
// Grappled), and the master's forced move counts as an effect that removes it from the reach.
func TestAGrappleEndsWhenTheGrapplerIsIncapacitatedOrTheTargetLeavesItsReach(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.grappleWon(t)

	// The master takes the target far away with a forced move: out of the reach, the hold ends.
	if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), CombatantId: c.id(t, c.hob), IdempotencyKey: newKey(), Col: 9, Row: 8, Forced: true,
	})); err != nil {
		t.Fatalf("forced move: %v", err)
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Fatal("the Hobgoblin is still Grappled out of Toren's reach")
	}

	// Grapple again; Toren falls Incapacitated (a condition the master marks): the grapple ends.
	c.advance(t, c.hob)
	c.advance(t, "Toren")
	if _, err := c.moveTo(t, a.master, c.hob, 4, 3); err != nil {
		t.Fatalf("placing the Hobgoblin: %v", err)
	}
	c.grappleWon(t)
	set := func(who string, keys ...string) {
		if _, err := a.master.combat.SetCombatantConditions(t.Context(), connect.NewRequest(&playv1.SetCombatantConditionsRequest{
			CampaignId: a.campaignID, EncounterId: c.e.GetId(), CombatantId: c.id(t, who), IdempotencyKey: newKey(), Conditions: &playv1.ConditionList{Keys: keys},
		})); err != nil {
			t.Fatalf("SetCombatantConditions() error = %v", err)
		}
	}
	set("Toren", "condition:incapacitated")
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("the grapple outlived its grappler being incapacitated")
	}
}

// TestAContestWaitsInAReactionWindowThatNamesWhoItWaitsFor (W7-X decision 3): a contest the
// defender must answer is a window of kind CONTEST; the defender's player and the master hold
// it, everybody else reads "Esperando <nome>" (or "Esperando o mestre" for an NPC), and it
// closes by itself when the contest is settled. AnswerReaction does not answer it.
func TestAContestWaitsInAReactionWindowThatNamesWhoItWaitsFor(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})
	c.advance(t, c.hob)
	c.h.roller.queue(17)
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	})

	windowOf := func(e *playv1.Encounter) *playv1.ReactionWindow {
		for _, w := range e.GetReactionWindows() {
			if w.GetKind() == playv1.ReactionKind_REACTION_KIND_CONTEST {
				return w
			}
		}
		return nil
	}
	own := windowOf(c.get(t, a.bia))
	if own == nil || !own.GetForYou() || own.GetContest().GetContestId() != started.GetContest().GetId() {
		t.Fatalf("Brisa's window = %v, want the contest she answers", own)
	}
	if windowOf(c.get(t, a.master)) == nil {
		t.Error("the master has no window to answer")
	}
	other := c.get(t, a.caio)
	if windowOf(other) != nil {
		t.Errorf("another player reads a window to answer: %v", windowOf(other))
	}
	if got := other.GetReactionWait().GetTitlePt(); got != "Esperando Brisa" {
		t.Errorf("another player reads %q, want %q", got, "Esperando Brisa")
	}
	// A contest is not answered with AnswerReaction.
	if _, err := a.bia.combat.AnswerReaction(t.Context(), connect.NewRequest(&playv1.AnswerReactionRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), WindowId: own.GetId(), Answer: playv1.ReactionChoice_REACTION_CHOICE_PASS,
	})); err == nil {
		t.Error("AnswerReaction answered a contest")
	}
	// The player leaves the roll to the master: the window is now his, and players read it.
	if _, err := a.bia.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey(), ContestId: started.GetContest().GetId(),
		Skill: playv1.ContestSkill_CONTEST_SKILL_ACROBATICS, DeferToMaster: true,
	})); err != nil {
		t.Fatalf("RespondContest(leave to the master) error = %v", err)
	}
	if got := c.get(t, a.caio).GetReactionWait().GetTitlePt(); got != "Esperando o mestre" {
		t.Errorf("after the deferral another player reads %q, want %q", got, "Esperando o mestre")
	}
	if windowOf(c.get(t, a.bia)) != nil && windowOf(c.get(t, a.bia)).GetForYou() && windowOf(c.get(t, a.bia)).GetAnswerNow() {
		t.Error("Brisa still holds the window after leaving the roll to the master")
	}
	c.mustRespond(t, a.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ACROBATICS, 8)
	if windowOf(c.get(t, a.master)) != nil || c.get(t, a.caio).GetReactionWait() != nil {
		t.Error("the window outlives the contest")
	}
}

// TestAGrappleEndsWhenItsGrapplerFallsToZeroHitPoints (SRD 5.1, "Grappled": the condition
// ends if the grappler is incapacitated): a player's character at 0 hit points holds nobody.
func TestAGrappleEndsWhenItsGrapplerFallsToZeroHitPoints(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	c.grappleWon(t)
	a.hurt(t, a.toren, 0)
	// The next change of the combat settles the holds.
	if _, err := a.master.combat.SetCombatantConditions(t.Context(), connect.NewRequest(&playv1.SetCombatantConditionsRequest{
		CampaignId: a.campaignID, EncounterId: c.e.GetId(), CombatantId: c.id(t, "Goblin"), IdempotencyKey: newKey(), Conditions: &playv1.ConditionList{Keys: []string{"condition:poisoned"}},
	})); err != nil {
		t.Fatalf("SetCombatantConditions() error = %v", err)
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("the grapple outlived its grappler falling to 0 hit points")
	}
}
