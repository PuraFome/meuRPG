package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The characterization of StartContest: every refusal with its exact code, reason and
// message, and every kind of contest it opens. The tests that already exist for contests
// (combat_contests_test.go, combat_maneuvers_more_test.go) follow the rules; these freeze
// the answers of each branch so a refactor of the function cannot change one.

const (
	purposeGrappleP = playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
	purposeShoveP   = playv1.ContestPurpose_CONTEST_PURPOSE_SHOVE
	purposeEscapeP  = playv1.ContestPurpose_CONTEST_PURPOSE_ESCAPE
	kindEscapeDC    = playv1.ContestKind_CONTEST_KIND_ESCAPE_DC
)

// noContestWritten checks that a refused call left no contest and spent nothing of Toren.
func (c *cx) noContestWritten(t *testing.T) {
	t.Helper()
	if n := len(c.state(t, c.master).GetContests()); n != 0 {
		t.Errorf("a refused StartContest left %d contests", n)
	}
	if toren := c.combatant(t, c.master, "Toren"); toren.GetActionUsed() {
		t.Error("a refused StartContest spent Toren's action")
	}
}

// TestStartContestRefusesBadRequestsBeforeTheTransaction freezes every refusal that comes
// from reading the request alone: the code and the message of each.
func TestStartContestRefusesBadRequestsBeforeTheTransaction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	stranger := a.h.newUser("Intruso")
	toren, hob := c.id(t, "Toren"), c.id(t, c.hob)
	grapple := func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = toren, hob, purposeGrappleP
	}
	masterAttack := func(dc int32) func(r *playv1.StartContestRequest) {
		return func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose, r.Kind, r.EscapeDc, r.Roll = c.id(t, c.hob), toren, purposeGrappleP, kindEscapeDC, dc, nil
		}
	}
	const (
		invalid  = connect.CodeInvalidArgument
		notFound = connect.CodeNotFound
	)
	tests := []struct {
		name string
		as   *user
		edit func(r *playv1.StartContestRequest)
		code connect.Code
		msg  string
	}{
		{"a stranger to the campaign", stranger, grapple, connect.CodeNotFound, "campaign not found"},
		{"an idempotency key that is not a UUID", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.IdempotencyKey = "x" }, invalid, "idempotency_key must be a UUID"},
		{"an encounter that is not a UUID", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.EncounterId = "x" }, notFound, "encounter not found"},
		{"an initiator that is not a UUID", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.InitiatorId = "x" }, notFound, "combatant not found"},
		{"no purpose", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Purpose = 0 }, invalid, "purpose must be GRAPPLE, SHOVE or ESCAPE"},
		{"ESCAPE_DC by a player", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Kind, r.EscapeDc = kindEscapeDC, 12 }, invalid, "kind ESCAPE_DC is the master's grapple that came from an attack"},
		{"ESCAPE_DC for a shove", a.master, func(r *playv1.StartContestRequest) { masterAttack(12)(r); r.Purpose = purposeShoveP }, invalid, "kind ESCAPE_DC is the master's grapple that came from an attack"},
		{"ESCAPE_DC for an escape", a.master, func(r *playv1.StartContestRequest) { masterAttack(12)(r); r.Purpose = purposeEscapeP }, invalid, "kind ESCAPE_DC is the master's grapple that came from an attack"},
		{"an escape DC of 0", a.master, masterAttack(0), invalid, "escape_dc must be 1 to 40"},
		{"an escape DC of 41", a.master, masterAttack(41), invalid, "escape_dc must be 1 to 40"},
		{"an escape DC with the contest kind", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.EscapeDc = 12 }, invalid, "escape_dc is only for kind ESCAPE_DC"},
		{"a grapple whose target is not a UUID", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.TargetId = "x" }, notFound, "combatant not found"},
		{"a shove whose target is missing", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Purpose, r.TargetId = purposeShoveP, "" }, notFound, "combatant not found"},
		{"Acrobatics for a grapple", a.caio, func(r *playv1.StartContestRequest) {
			grapple(r)
			r.Skill = playv1.ContestSkill_CONTEST_SKILL_ACROBATICS
		}, invalid, "a grapple or a shove is a Strength (Athletics) check"},
		{"Acrobatics for a shove", a.caio, func(r *playv1.StartContestRequest) {
			grapple(r)
			r.Purpose, r.Skill = purposeShoveP, playv1.ContestSkill_CONTEST_SKILL_ACROBATICS
		}, invalid, "a grapple or a shove is a Strength (Athletics) check"},
		{"a maneuver for a shove", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Purpose, r.ManeuverKey = purposeShoveP, "x" }, invalid, "a maneuver starts a GRAPPLE that is rolled"},
		{"a maneuver for an escape", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.Purpose, r.ManeuverKey = toren, purposeEscapeP, "x"
		}, invalid, "a maneuver starts a GRAPPLE that is rolled"},
		{"a maneuver for a grapple that came from an attack", a.master, func(r *playv1.StartContestRequest) { masterAttack(12)(r); r.ManeuverKey = "x" }, invalid, "a maneuver starts a GRAPPLE that is rolled"},
		{"no way to roll", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = nil }, invalid, "set roll_in_app or d20_faces"},
		{"roll_in_app false", a.caio, func(r *playv1.StartContestRequest) {
			grapple(r)
			r.Roll = &playv1.CheckRollInput{Roll: &playv1.CheckRollInput_RollInApp{RollInApp: false}}
		}, invalid, "roll_in_app must be true"},
		{"no faces", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = faces() }, invalid, "d20_faces must have one or two faces"},
		{"three faces", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = faces(1, 2, 3) }, invalid, "d20_faces must have one or two faces"},
		{"a face of 0", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = faces(0) }, invalid, "each d20 face must be 1 to 20"},
		{"a face of 21", a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = faces(21) }, invalid, "each d20 face must be 1 to 20"},
	}
	for _, tc := range tests {
		_, err := c.startContest(t, tc.as, tc.edit)
		wantRefusal(t, tc.name, err, tc.code, tc.msg)
	}
	c.noContestWritten(t)
}

// TestStartContestRefusesInsideTheTransaction freezes the refusals the transaction reads
// from the combat: none of them writes, so one arena serves every case.
func TestStartContestRefusesInsideTheTransaction(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	toren, hob := c.id(t, "Toren"), c.id(t, c.hob)
	unknown := newKey()
	encounterBlocked := func(reason playv1.EncounterBlockedReason) func(*testing.T, error) {
		return func(t *testing.T, err error) {
			t.Helper()
			wantEncounterBlocked(t, err, reason)
		}
	}
	contestBlocked := func(reason playv1.ContestBlockedReason) func(*testing.T, error) {
		return func(t *testing.T, err error) {
			t.Helper()
			wantContestBlocked(t, "StartContest", err, reason)
		}
	}
	tests := []struct {
		name   string
		as     *user
		edit   func(r *playv1.StartContestRequest)
		code   connect.Code
		msg    string
		reason func(*testing.T, error)
	}{
		{"an initiator that is not in the combat", a.master, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = unknown, hob, purposeGrappleP
		}, connect.CodeNotFound, "combatant not found", nil},
		{"another player's combatant", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Brisa"), hob, purposeGrappleP
		}, connect.CodePermissionDenied, "only the combatant's player or the master may do this", nil},
		{"not the combatant's turn", a.bia, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Brisa"), c.id(t, "Goblin"), purposeGrappleP
		}, connect.CodeFailedPrecondition, "it is not this combatant's turn", encounterBlocked(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN)},
		{"an escape from nothing", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.Purpose = toren, purposeEscapeP
		}, connect.CodeFailedPrecondition, "the combatant is not grappled", contestBlocked(playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_GRAPPLED)},
		{"a target that is not in the combat", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = toren, unknown, purposeGrappleP
		}, connect.CodeNotFound, "combatant not found", nil},
		{"a grapple on oneself", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = toren, toren, purposeGrappleP
		}, connect.CodeInvalidArgument, "a combatant cannot contest itself", nil},
		{"a shove on oneself", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = toren, toren, purposeShoveP
		}, connect.CodeInvalidArgument, "a combatant cannot contest itself", nil},
		{"a target out of reach", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = toren, c.id(t, "Goblin"), purposeGrappleP
		}, connect.CodeFailedPrecondition, "the target is beyond your reach", encounterBlocked(blockedOutOfReach)},
		{"a maneuver of a character that has none", a.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose, r.ManeuverKey = toren, hob, purposeGrappleP, "maneuver:none"
		}, connect.CodeInvalidArgument, "the character has no such grapple maneuver", nil},
		{"a maneuver of an NPC", a.master, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose, r.ManeuverKey = toren, hob, purposeGrappleP, "maneuver:none"
		}, connect.CodeInvalidArgument, "the character has no such grapple maneuver", nil},
	}
	for _, tc := range tests {
		_, err := c.startContest(t, tc.as, tc.edit)
		if err == nil {
			t.Fatalf("%s: no error, want %v %q", tc.name, tc.code, tc.msg)
		}
		wantRefusal(t, tc.name, err, tc.code, tc.msg)
		if tc.reason != nil {
			tc.reason(t, err)
		}
	}
	c.noContestWritten(t)
}

// TestStartContestWhenTheCombatIsNotRunning freezes the answers of a combat that has not
// begun and of one that ended.
func TestStartContestWhenTheCombatIsNotRunning(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{setup: true})
	grapple := func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeGrappleP
	}
	_, err := c.startContest(t, a.caio, grapple)
	wantRefusal(t, "a combat in SETUP", err, connect.CodeFailedPrecondition, "the combat is not running")
	wantEncounterBlocked(t, err, blockedNotActive)

	c.e = a.begin(t, c.refresh(t))
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	_, err = c.startContest(t, a.caio, grapple)
	wantRefusal(t, "a combat that ended", err, connect.CodeFailedPrecondition, "the combat has ended")
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_ENCOUNTER_ENDED)
}

// TestStartContestRefusesADefeatedTarget (the target is defeated: nothing to contest).
func TestStartContestRefusesADefeatedTarget(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	a.execSQL(t, `UPDATE combatants SET defeated = true WHERE id = $1`, c.id(t, c.hob))
	_, err := c.startContest(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeGrappleP
	})
	wantRefusal(t, "a defeated target", err, connect.CodeFailedPrecondition, "the target is defeated")
	wantEncounterBlocked(t, err, blockedTargetDefeated)
	c.noContestWritten(t)
}

// TestStartContestRefusalsThatFollowAWrite: a contest that waits holds the initiator's next one
// (CONTEST_OPEN), and with the one attack spent a second is refused (ACTION_USED); an escape
// with the action spent too.
func TestStartContestRefusalsThatFollowAWrite(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	first, err := c.grapple(t, a.caio, "Toren", c.hob, 10)
	if err != nil {
		t.Fatalf("StartContest() error = %v", err)
	}
	_, err = c.grapple(t, a.caio, "Toren", c.hob, 10)
	wantRefusal(t, "a second contest while one waits", err, connect.CodeFailedPrecondition, "a contest of this combatant waits already")
	wantContestBlocked(t, "StartContest", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_CONTEST_OPEN)

	c.mustRespond(t, a.master, first.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 20)
	_, err = c.grapple(t, a.caio, "Toren", c.hob, 10)
	wantRefusal(t, "a second attack with no Extra Attack", err, connect.CodeFailedPrecondition, "the action of this turn is used")
	wantEncounterBlocked(t, err, blockedActionUsed)
}

// TestStartContestRefusesTheDiceTheCampaignDoesNotWant: a player is bound by the campaign's dice
// mode, the master is not; typed dice that do not match the roll's mode abort (the screen is stale).
func TestStartContestRefusesTheDiceTheCampaignDoesNotWant(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{})
	grapple := func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeGrappleP
	}

	// Two typed d20 for a roll that takes one (no advantage).
	_, err := c.startContest(t, a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = faces(10, 11) })
	wantRefusal(t, "two faces for a plain roll", err, connect.CodeAborted, "the roll's mode changed: read the options again")
	c.noContestWritten(t)

	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	_, err = c.startContest(t, a.caio, grapple)
	wantRefusal(t, "the app's roll when everybody rolls real dice", err, connect.CodeFailedPrecondition, "this is not how the campaign has you roll your dice")
	wantEncounterBlocked(t, err, blockedWrongDice)
	c.noContestWritten(t)

	// The master rolls either way, for anyone: his typed 15 is Toren's roll, marked as his.
	started := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = faces(15) })
	roll := started.GetContest().GetInitiatorRoll()
	if roll.GetTotal() != 18 || !roll.GetRolledByMaster() || !roll.GetPhysical() {
		t.Errorf("Toren's roll typed by the master = %v, want physical 15 + 3 = 18 rolled by the master", roll)
	}

	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
	c.mustRespond(t, a.master, started.GetContest().GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 20)
	c.advance(t, "Brisa")
	c.advance(t, "Toren") // the next round: the action is back
	_, err = c.startContest(t, a.caio, func(r *playv1.StartContestRequest) { grapple(r); r.Roll = faces(15) })
	wantRefusal(t, "typed dice when everybody rolls in the app", err, connect.CodeFailedPrecondition, "this is not how the campaign has you roll your dice")
	wantEncounterBlocked(t, err, blockedWrongDice)
}

// TestStartContestKinds is the golden of what each kind of start writes: the contest the
// master reads, the log line and what the initiator spent.
func TestStartContestKinds(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	c := a.arena(t, arenaPlan{at: map[string][2]int32{"Brisa": {5, 3}}})

	// A shove waits for the defender's answer like a grapple, and spends the attack.
	c.h.roller.queue(14) // 14 + 3
	shove := c.mustStart(t, a.caio, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, c.hob), purposeShoveP
	}).GetContest()
	if shove.GetPurpose() != purposeShoveP || shove.GetKind() != playv1.ContestKind_CONTEST_KIND_CONTEST ||
		shove.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_AWAITING_DEFENDER || shove.GetWaitingFor() != playv1.ContestWaitFor_CONTEST_WAIT_FOR_MASTER ||
		shove.GetInitiatorRoll().GetTotal() != 17 || shove.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_UNSPECIFIED {
		t.Errorf("shove = %v, want a CONTEST shove waiting for the master with 17", shove)
	}
	if !c.combatant(t, a.master, "Toren").GetActionUsed() {
		t.Error("the shove did not spend Toren's action")
	}
	if c.hasCondition(t, c.hob, condGrappled) {
		t.Error("a shove grappled")
	}
	c.mustRespond(t, a.master, shove.GetId(), playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, 20)

	// A grapple that came from an attack: the master says the DC, nobody rolls, nothing is
	// spent, and it is settled at once.
	c.advance(t, c.hob)
	byAttack := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), purposeGrappleP
		r.Kind, r.EscapeDc, r.Roll = kindEscapeDC, 13, nil
	}).GetContest()
	if byAttack.GetKind() != kindEscapeDC || byAttack.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_RESOLVED ||
		byAttack.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_INITIATOR || byAttack.GetEscapeDc() != 13 || byAttack.GetInitiatorRoll() != nil {
		t.Errorf("grapple by attack = %v, want a resolved ESCAPE_DC contest won by the initiator, DC 13, no roll", byAttack)
	}
	if !c.hasCondition(t, "Brisa", condGrappled) {
		t.Error("Brisa is not Grappled by the attack")
	}
	if c.combatant(t, a.master, c.hob).GetActionUsed() {
		t.Error("a grapple that came from an attack spent the grappler's action")
	}
	var line bool
	for _, e := range contestLogEntries(a.log(t, a.bia, c.e)) {
		line = line || (e.GetContest().GetLine() == playv1.ContestLogLine_CONTEST_LOG_LINE_GRAPPLED && e.GetActorLabel() == c.hob && e.GetTargetLabel() == "Brisa")
	}
	if !line {
		t.Error("the log has no line for the grapple by attack")
	}
	// ... and another grapple on a target already held is not refused: the hold is kept again.
	again := c.mustStart(t, a.master, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.TargetId, r.Purpose = c.id(t, c.hob), c.id(t, "Brisa"), purposeGrappleP
		r.Kind, r.EscapeDc, r.Roll = kindEscapeDC, 20, nil
	}).GetContest()
	if again.GetEscapeDc() != 20 {
		t.Errorf("the second grapple by attack = %v, want the new DC 20", again)
	}

	// An escape against the DC, rolled in the app: 8 + 3 (Acrobatics) = 11 under 20, so the
	// grappler wins and the hold stays.
	c.advance(t, "Brisa")
	c.h.roller.queue(8)
	esc := c.mustStart(t, a.bia, func(r *playv1.StartContestRequest) {
		r.InitiatorId, r.Purpose, r.Skill = c.id(t, "Brisa"), purposeEscapeP, playv1.ContestSkill_CONTEST_SKILL_ACROBATICS
	}).GetContest()
	if esc.GetKind() != kindEscapeDC || esc.GetWinner() != playv1.ContestWinner_CONTEST_WINNER_DEFENDER ||
		esc.GetStatus() != playv1.ContestStatus_CONTEST_STATUS_RESOLVED || esc.GetEscapeDc() != 0 {
		t.Errorf("escape = %v, want resolved for the grappler with the DC hidden from Brisa", esc)
	}
	// The kind of the escape is the hold's (ESCAPE_DC), and the DC itself never reaches Brisa.
}

// TestStartContestByAFamiliarIsRefused: a creature that cannot attack cannot grapple either
// (the spell that brought it says what it may do).
func TestStartContestByAFamiliarIsRefused(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	a.mustCastSummon(t, a.ana, a.pens, findFamiliar, nil, 0, []string{"monster:owl"}, "Nanquim")
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{2},
		players:  map[string]int32{"Pensantus": 20, "Sálvia": 15, "Toren": 12, "Nanquim": 10},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Sálvia": {6, 5}, "Nanquim": {5, 3}},
	})
	e = a.passTo(t, e, "Nanquim")
	_, err := a.master.contests.StartContest(t.Context(), connect.NewRequest(&playv1.StartContestRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey(), InitiatorId: a.id(t, "Nanquim"), TargetId: a.id(t, "Goblin"),
		Purpose: purposeGrappleP, Roll: rollIn(),
	}))
	wantRefusal(t, "a familiar grappling", err, connect.CodeFailedPrecondition, "this creature cannot attack")
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CREATURE_CANNOT_ATTACK)
}
