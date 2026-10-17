package play

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Characterization of the group check's refusals and of what each viewer reads, written before
// the functions of combat_groupcheck.go were split into phases: the exact code, reason and message
// of each refusal, the order the checks run in, and a golden of each view.

const (
	groupBadKeyMessage      = "idempotency_key must be a UUID"
	groupNoCheckMessage     = "group check not found"
	groupNoCharacterMessage = "character not found"
	groupNoRollMessage      = "set roll_in_app or d20_faces"
)

// wantGroupBlocked checks a ContestBlocked refusal: the reason and the exact message.
func wantGroupBlocked(t *testing.T, call string, err error, reason playv1.ContestBlockedReason, msg string) {
	t.Helper()
	wantContestBlocked(t, call, err, reason)
	if ce, ok := errors.AsType[*connect.Error](err); !ok || ce.Message() != msg {
		t.Errorf("%s: message = %v, want %q", call, err, msg)
	}
}

// TestGroupCheckRequestRefusalsAndTheirOrder freezes each refusal of RequestGroupCheck with its
// exact message, and which one wins when a request breaks two rules (key, skill, ids, DC, then
// what the database says).
func TestGroupCheckRequestRefusalsAndTheirOrder(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	const (
		skillMsg = "skill_key must be a skill, an ability check or a saving throw"
		dcMsg    = "dc must be 1 to 40, or 0 for none"
		twiceMsg = "character_ids names a character twice"
		whoMsg   = "character_ids must name living player characters of this campaign"
	)
	cases := []struct {
		name string
		edit func(*playv1.RequestGroupCheckRequest)
		code connect.Code
		msg  string
	}{
		{"a key that is no UUID", func(r *playv1.RequestGroupCheckRequest) { r.IdempotencyKey = "x" }, connect.CodeInvalidArgument, groupBadKeyMessage},
		{"a key and a skill both wrong", func(r *playv1.RequestGroupCheckRequest) { r.IdempotencyKey, r.SkillKey = "x", "" }, connect.CodeInvalidArgument, groupBadKeyMessage},
		{"a skill that is none", func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = "nothing" }, connect.CodeInvalidArgument, skillMsg},
		{"a skill and a DC both wrong", func(r *playv1.RequestGroupCheckRequest) { r.SkillKey, r.Dc = "", 99 }, connect.CodeInvalidArgument, skillMsg},
		{"an ID that is no UUID", func(r *playv1.RequestGroupCheckRequest) { r.CharacterIds = []string{"x"} }, connect.CodeNotFound, groupNoCharacterMessage},
		{"a character twice", func(r *playv1.RequestGroupCheckRequest) {
			r.CharacterIds = []string{a.toren.GetId(), a.toren.GetId()}
		}, connect.CodeInvalidArgument, twiceMsg},
		{"a character twice and a DC wrong", func(r *playv1.RequestGroupCheckRequest) {
			r.CharacterIds, r.Dc = []string{a.toren.GetId(), a.toren.GetId()}, 99
		}, connect.CodeInvalidArgument, twiceMsg},
		{"a DC above 40", func(r *playv1.RequestGroupCheckRequest) { r.Dc = 41 }, connect.CodeInvalidArgument, dcMsg},
		{"a DC below 0", func(r *playv1.RequestGroupCheckRequest) { r.Dc = -1 }, connect.CodeInvalidArgument, dcMsg},
		{"an NPC", func(r *playv1.RequestGroupCheckRequest) { r.CharacterIds = []string{a.goblin.GetId()} }, connect.CodeInvalidArgument, whoMsg},
		{"a character of nowhere", func(r *playv1.RequestGroupCheckRequest) { r.CharacterIds = []string{newKey()} }, connect.CodeInvalidArgument, whoMsg},
		{"a group of one", func(r *playv1.RequestGroupCheckRequest) {
			r.CharacterIds, r.Group = []string{a.toren.GetId()}, new(true)
		}, connect.CodeInvalidArgument, "a group check needs at least two characters; ask one without group"},
	}
	for _, c := range cases {
		_, err := a.requestGroup(t, a.master, c.edit)
		wantRefusal(t, c.name, err, c.code, c.msg)
	}
	if v := a.groupView(t, a.master); v != nil {
		t.Fatalf("a refused request left %v", v)
	}
	// A group of one that is not said to be a group is a plain roll; "group: false" for everyone
	// is each judged alone.
	if _, err := a.askOnly(t, stealthKey, nil, a.toren); err != nil {
		t.Fatalf("asking one character error = %v", err)
	}
	_, err := a.requestGroup(t, a.master, nil)
	wantGroupBlocked(t, "a second check", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_OPEN, "a group check is open already")
}

// TestGroupCheckRequestOfAnUnaskablePartyAndAnIdempotencyConflict: with every player character dead
// there is nobody to ask; a key reused for another change says so with its message.
func TestGroupCheckRequestOfAnUnaskablePartyAndAnIdempotencyConflict(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, nil)
	reuse := &playv1.CloseGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: asked.GetId()}
	if _, err := a.master.contests.CloseGroupCheck(t.Context(), connect.NewRequest(reuse)); err != nil {
		t.Fatalf("CloseGroupCheck() error = %v", err)
	}
	// The same key was used to ask, not to close: another kind of change.
	_, err := a.master.contests.RequestGroupCheck(t.Context(), connect.NewRequest(&playv1.RequestGroupCheckRequest{
		CampaignId: a.campaignID, IdempotencyKey: reuse.GetIdempotencyKey(), SkillKey: stealthKey, Dc: 12,
	}))
	wantRefusal(t, "a key used for another kind of change", err, connect.CodeInvalidArgument, "idempotency_key was already used for another change")
	// Another player's key is not this player's retry either.
	roll := &playv1.RollGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: newKey(), Roll: rollIn()}
	_, err = a.master.contests.CloseGroupCheck(t.Context(), connect.NewRequest(&playv1.CloseGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: roll.GetIdempotencyKey(), GroupCheckId: asked.GetId()}))
	wantContestBlocked(t, "closing twice with a new key", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED)

	for _, c := range []*charactersv1.Character{a.toren, a.pens, a.bri} {
		if _, err := a.master.characters.MarkCharacterDead(t.Context(), connect.NewRequest(&charactersv1.MarkCharacterDeadRequest{CampaignId: a.campaignID, CharacterId: c.GetId()})); err != nil {
			t.Fatalf("MarkCharacterDead() error = %v", err)
		}
	}
	_, err = a.requestGroup(t, a.master, nil)
	wantRefusal(t, "a party of the dead", err, connect.CodeInvalidArgument, "the campaign has no living player character to ask")
}

// TestGroupCheckRollRefusalsAndTheirOrder freezes the refusals of RollGroupCheck, RollForPlayer
// and CloseGroupCheck that come before the database is read: the key, the IDs and the roll, in that order.
func TestGroupCheckRollRefusalsAndTheirOrder(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, nil)
	id := asked.GetId()
	badInputs := map[string]*playv1.CheckRollInput{
		"no roll":            {},
		"roll_in_app false":  {Roll: &playv1.CheckRollInput_RollInApp{RollInApp: false}},
		"three faces":        faces(1, 2, 3),
		"a face of 21":       faces(21),
		"no faces":           faces(),
		"a face of 0 second": faces(5, 0),
	}
	wantMsg := map[string]string{
		"no roll": groupNoRollMessage, "roll_in_app false": "roll_in_app must be true", "three faces": "d20_faces must have one or two faces",
		"a face of 21": "each d20 face must be 1 to 20", "no faces": "d20_faces must have one or two faces", "a face of 0 second": "each d20 face must be 1 to 20",
	}
	for name, in := range badInputs {
		_, err := a.rollGroupWith(t, a.caio, id, in)
		wantRefusal(t, "RollGroupCheck with "+name, err, connect.CodeInvalidArgument, wantMsg[name])
		_, err = a.master.contests.RollForPlayer(t.Context(), connect.NewRequest(&playv1.RollForPlayerRequest{
			CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: id, CharacterId: a.bri.GetId(), Roll: in,
		}))
		wantRefusal(t, "RollForPlayer with "+name, err, connect.CodeInvalidArgument, wantMsg[name])
	}

	roll := func(key, check string, in *playv1.CheckRollInput) error {
		_, err := a.caio.contests.RollGroupCheck(t.Context(), connect.NewRequest(&playv1.RollGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: key, GroupCheckId: check, Roll: in}))
		return err
	}
	wantRefusal(t, "RollGroupCheck, bad key", roll("x", id, rollIn()), connect.CodeInvalidArgument, groupBadKeyMessage)
	wantRefusal(t, "RollGroupCheck, bad key and bad ID", roll("x", "y", nil), connect.CodeInvalidArgument, groupBadKeyMessage)
	wantRefusal(t, "RollGroupCheck, bad ID", roll(newKey(), "y", rollIn()), connect.CodeNotFound, groupNoCheckMessage)
	wantRefusal(t, "RollGroupCheck, bad ID and bad roll", roll(newKey(), "y", nil), connect.CodeNotFound, groupNoCheckMessage)
	wantRefusal(t, "RollGroupCheck, a check of nowhere", roll(newKey(), newKey(), rollIn()), connect.CodeNotFound, groupNoCheckMessage)

	forPlayer := func(key, check, character string, in *playv1.CheckRollInput) error {
		_, err := a.master.contests.RollForPlayer(t.Context(), connect.NewRequest(&playv1.RollForPlayerRequest{CampaignId: a.campaignID, IdempotencyKey: key, GroupCheckId: check, CharacterId: character, Roll: in}))
		return err
	}
	bri := a.bri.GetId()
	wantRefusal(t, "RollForPlayer, bad key", forPlayer("x", id, bri, rollIn()), connect.CodeInvalidArgument, groupBadKeyMessage)
	wantRefusal(t, "RollForPlayer, bad ID", forPlayer(newKey(), "y", bri, rollIn()), connect.CodeNotFound, groupNoCheckMessage)
	wantRefusal(t, "RollForPlayer, bad character", forPlayer(newKey(), id, "y", rollIn()), connect.CodeNotFound, groupNoCharacterMessage)
	wantRefusal(t, "RollForPlayer, bad character and bad roll", forPlayer(newKey(), id, "y", nil), connect.CodeNotFound, groupNoCharacterMessage)
	wantRefusal(t, "RollForPlayer, a check of nowhere", forPlayer(newKey(), newKey(), bri, rollIn()), connect.CodeNotFound, groupNoCheckMessage)

	closeWith := func(key, check string) error {
		_, err := a.master.contests.CloseGroupCheck(t.Context(), connect.NewRequest(&playv1.CloseGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: key, GroupCheckId: check}))
		return err
	}
	wantRefusal(t, "CloseGroupCheck, bad key", closeWith("x", id), connect.CodeInvalidArgument, groupBadKeyMessage)
	wantRefusal(t, "CloseGroupCheck, bad ID", closeWith(newKey(), "y"), connect.CodeNotFound, groupNoCheckMessage)
	wantRefusal(t, "CloseGroupCheck, a check of nowhere", closeWith(newKey(), newKey()), connect.CodeNotFound, groupNoCheckMessage)
	if v := a.groupView(t, a.master); !v.GetOpen() || memberOf(v, a.toren.GetId()).GetAnswered() {
		t.Errorf("refused calls changed the check: %v", v)
	}
}

// TestGroupCheckRefusalMessagesOfTheMembers freezes the messages of the refusals that name the
// member: not asked, already answered, closed, and a player who has no character in the check.
func TestGroupCheckRefusalMessagesOfTheMembers(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked, err := a.askOnly(t, stealthKey, nil, a.toren)
	if err != nil {
		t.Fatalf("asking Toren error = %v", err)
	}
	id := asked.GetGroupCheck().GetId()
	const (
		notInMsg = "the character is not in this check"
		doneMsg  = "the character answered already"
		shutMsg  = "the group check is closed"
	)
	notIn := playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_IN_GROUP_CHECK
	// Pensantus's player is no member of this check, but has a living character.
	_, err = a.rollGroup(t, a.ana, id, 10)
	wantGroupBlocked(t, "a character that was not asked rolls", err, notIn, notInMsg)
	_, err = a.rollForPlayer(t, a.master, id, a.pens.GetId(), 10)
	wantGroupBlocked(t, "the master rolls for a character that was not asked", err, notIn, notInMsg)
	// A player who has no character at all.
	lone := a.h.newUser("Sem Personagem")
	a.h.join(a.master, a.campaignID, lone)
	_, err = a.rollGroup(t, lone, id, 10)
	wantGroupBlocked(t, "a player with no character rolls", err, notIn, "you have no living character in this check")

	a.mustRollGroup(t, a.caio, id, 10)
	_, err = a.rollGroup(t, a.caio, id, 10)
	wantGroupBlocked(t, "Toren answers twice", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_ALREADY_ANSWERED, doneMsg)
	_, err = a.rollForPlayer(t, a.master, id, a.toren.GetId(), 10)
	wantGroupBlocked(t, "the master rolls for Toren after he answered", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_ALREADY_ANSWERED, doneMsg)
	a.mustCloseGroup(t, id)
	closed := playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED
	_, err = a.rollGroup(t, a.caio, id, 10)
	wantGroupBlocked(t, "a roll after the close", err, closed, shutMsg)
	_, err = a.rollForPlayer(t, a.master, id, a.toren.GetId(), 10)
	wantGroupBlocked(t, "a roll for a player after the close", err, closed, shutMsg)
	_, err = a.closeGroup(t, a.master, id)
	wantGroupBlocked(t, "a second close", err, closed, shutMsg)
	// A closed check does not hold the next request.
	if _, err := a.requestGroup(t, a.master, nil); err != nil {
		t.Errorf("RequestGroupCheck() after the close error = %v", err)
	}
}

// TestGroupCheckCallsWithNoOpenSession: every call of the group check is refused with
// NO_OPEN_SESSION once the session has ended (the player's roll and the master's calls alike).
func TestGroupCheckCallsWithNoOpenSession(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, nil)
	a.master.end(t, a.master.liveSession(t, a.campaignID).GetGameSession())
	const noSession = playv1.GameSessionBlockedReason_GAME_SESSION_BLOCKED_REASON_NO_OPEN_SESSION
	_, err := a.requestGroup(t, a.master, nil)
	wantBlocked(t, "RequestGroupCheck", err, noSession)
	wantRefusal(t, "RequestGroupCheck", err, connect.CodeFailedPrecondition, "the campaign has no open game session")
	_, err = a.rollGroup(t, a.caio, asked.GetId(), 10)
	wantBlocked(t, "RollGroupCheck", err, noSession)
	_, err = a.rollForPlayer(t, a.master, asked.GetId(), a.toren.GetId(), 10)
	wantBlocked(t, "RollForPlayer", err, noSession)
	_, err = a.closeGroup(t, a.master, asked.GetId())
	wantBlocked(t, "CloseGroupCheck", err, noSession)
	for who, u := range map[string]*user{"master": a.master, "player": a.caio} {
		_, err := u.contests.GetGroupCheck(t.Context(), connect.NewRequest(&playv1.GetGroupCheckRequest{CampaignId: a.campaignID}))
		wantBlocked(t, "GetGroupCheck as the "+who, err, noSession)
	}
}

// groupViewDigest is what a viewer reads of a group check, on one line with no IDs or times.
func groupViewDigest(v *playv1.GroupCheckView) string {
	if v == nil {
		return "none"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %q open=%t dc=%d show=%t group=%t save=%t asked=%d count=%d needed=%d verdict=%t/%t youRoll=%t",
		v.GetSkillKey(), v.GetSkillNamePt(), v.GetOpen(), v.GetDc(), v.GetShowDc(), v.GetGroup(), v.GetSave(), v.GetAskedCount(),
		v.GetPassedCount(), v.GetNeeded(), v.GetVerdictKnown(), v.GetGroupPassed(), v.GetYouRoll())
	if o := v.GetYourOption(); o != nil {
		fmt.Fprintf(&b, " option=%+d/%t/%s", o.GetModifier(), o.GetKnown(), o.GetMode())
	}
	// The master reads the members in the order of the query (by ID, so not stable): sort by name.
	members := slices.SortedFunc(slices.Values(v.GetMembers()), func(x, y *playv1.GroupCheckMemberView) int { return strings.Compare(x.GetName(), y.GetName()) })
	for _, m := range members {
		fmt.Fprintf(&b, " | %s answered=%t", m.GetName(), m.GetAnswered())
		if r := m.GetRoll(); r != nil {
			fmt.Fprintf(&b, " faces=%v mod=%d total=%d phys=%t mode=%s known=%t master=%t", r.GetFaces(), r.GetModifier(), r.GetTotal(), r.GetPhysical(), r.GetMode(), r.GetBonusKnown(), r.GetRolledByMaster())
		}
		fmt.Fprintf(&b, " passed=%t/%t", m.GetPassedKnown(), m.GetPassed())
	}
	return b.String()
}

// wantGroupDigests checks what each viewer reads of the current group check.
func (a *armed) wantGroupDigests(t *testing.T, step string, want map[string]string) {
	t.Helper()
	viewers := map[string]*user{"master": a.master, "Toren's player": a.caio, "Pensantus's player": a.ana, "Brisa's player": a.bia}
	for who, w := range want {
		if got := groupViewDigest(a.groupView(t, viewers[who])); got != w {
			t.Errorf("%s, %s reads\n got  %s\n want %s", step, who, got, w)
		}
	}
}

// Finding: while a group check is still open, the master already reads a verdict (verdict_known with
// group_passed from the answers so far, combat_groupcheck.go buildGroupCheckView) whenever the check has a DC;
// the field comment says "once closed". Frozen as it is: the master's screen reads it as a running tally.
//
// TestGroupCheckViewsGolden freezes what the master, each asked player and a player who was not
// asked read, at each step of a group check: DC hidden, then shown, then asked one by one.
func TestGroupCheckViewsGolden(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, nil) // Stealth, DC 12, hidden
	a.wantGroupDigests(t, "asked", map[string]string{
		"master":             `skill:stealth "Furtividade" open=true dc=12 show=false group=true save=false asked=3 count=0 needed=2 verdict=true/false youRoll=false | Brisa answered=false passed=false/false | Pensantus answered=false passed=false/false | Toren answered=false passed=false/false`,
		"Toren's player":     `skill:stealth "Furtividade" open=true dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=true option=+2/true/ROLL_MODE_KIND_NORMAL | Toren answered=false passed=false/false`,
		"Pensantus's player": `skill:stealth "Furtividade" open=true dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=true option=+2/true/ROLL_MODE_KIND_NORMAL | Pensantus answered=false passed=false/false`,
		"Brisa's player":     `skill:stealth "Furtividade" open=true dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=true option=+3/true/ROLL_MODE_KIND_NORMAL | Brisa answered=false passed=false/false`,
	})
	a.mustRollGroup(t, a.caio, asked.GetId(), 10) // 12
	a.mustRollGroup(t, a.ana, asked.GetId(), 4)   // 6
	a.wantGroupDigests(t, "two answered", map[string]string{
		"master":             `skill:stealth "Furtividade" open=true dc=12 show=false group=true save=false asked=3 count=1 needed=2 verdict=true/false youRoll=false | Brisa answered=false passed=false/false | Pensantus answered=true faces=[4] mod=2 total=6 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/false | Toren answered=true faces=[10] mod=2 total=12 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/true`,
		"Toren's player":     `skill:stealth "Furtividade" open=true dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=false | Toren answered=true faces=[10] mod=2 total=12 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=false/false`,
		"Pensantus's player": `skill:stealth "Furtividade" open=true dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=false | Pensantus answered=true faces=[4] mod=2 total=6 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=false/false`,
		"Brisa's player":     `skill:stealth "Furtividade" open=true dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=true option=+3/true/ROLL_MODE_KIND_NORMAL | Brisa answered=false passed=false/false`,
	})
	a.mustCloseGroup(t, asked.GetId())
	a.wantGroupDigests(t, "closed, DC hidden", map[string]string{
		"master":             `skill:stealth "Furtividade" open=false dc=12 show=false group=true save=false asked=3 count=1 needed=2 verdict=true/false youRoll=false | Brisa answered=false passed=false/false | Pensantus answered=true faces=[4] mod=2 total=6 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/false | Toren answered=true faces=[10] mod=2 total=12 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/true`,
		"Toren's player":     `skill:stealth "Furtividade" open=false dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=false | Toren answered=true faces=[10] mod=2 total=12 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=false/false`,
		"Pensantus's player": `skill:stealth "Furtividade" open=false dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=false | Pensantus answered=true faces=[4] mod=2 total=6 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=false/false`,
		"Brisa's player":     `skill:stealth "Furtividade" open=false dc=0 show=false group=true save=false asked=0 count=0 needed=0 verdict=false/false youRoll=false | Brisa answered=false passed=false/false`,
	})

	shown := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.ShowDc, r.SkillKey = true, "save:con" })
	a.mustRollGroup(t, a.bia, shown.GetId(), 20)
	if _, err := a.rollForPlayer(t, a.master, shown.GetId(), a.pens.GetId(), 1); err != nil {
		t.Fatalf("RollForPlayer() error = %v", err)
	}
	a.wantGroupDigests(t, "save, DC shown, open", map[string]string{
		"master":             `save:con "Teste de resistência de Constituição" open=true dc=12 show=true group=true save=true asked=3 count=1 needed=2 verdict=true/false youRoll=false | Brisa answered=true faces=[20] mod=4 total=24 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/true | Pensantus answered=true faces=[1] mod=1 total=2 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=true passed=true/false | Toren answered=false passed=false/false`,
		"Toren's player":     `save:con "Teste de resistência de Constituição" open=true dc=0 show=true group=true save=true asked=0 count=0 needed=0 verdict=false/false youRoll=true option=+4/true/ROLL_MODE_KIND_NORMAL | Toren answered=false passed=false/false`,
		"Pensantus's player": `save:con "Teste de resistência de Constituição" open=true dc=0 show=true group=true save=true asked=0 count=0 needed=0 verdict=false/false youRoll=false | Pensantus answered=true faces=[1] mod=1 total=2 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=true passed=true/false`,
		"Brisa's player":     `save:con "Teste de resistência de Constituição" open=true dc=0 show=true group=true save=true asked=0 count=0 needed=0 verdict=false/false youRoll=false | Brisa answered=true faces=[20] mod=4 total=24 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/true`,
	})
	a.mustCloseGroup(t, shown.GetId())
	a.wantGroupDigests(t, "save, DC shown, closed", map[string]string{
		"master":             `save:con "Teste de resistência de Constituição" open=false dc=12 show=true group=true save=true asked=3 count=1 needed=2 verdict=true/false youRoll=false | Brisa answered=true faces=[20] mod=4 total=24 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/true | Pensantus answered=true faces=[1] mod=1 total=2 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=true passed=true/false | Toren answered=false passed=false/false`,
		"Toren's player":     `save:con "Teste de resistência de Constituição" open=false dc=0 show=true group=true save=true asked=0 count=0 needed=0 verdict=true/false youRoll=false | Toren answered=false passed=false/false`,
		"Pensantus's player": `save:con "Teste de resistência de Constituição" open=false dc=0 show=true group=true save=true asked=0 count=0 needed=0 verdict=true/false youRoll=false | Pensantus answered=true faces=[1] mod=1 total=2 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=true passed=true/false`,
		"Brisa's player":     `save:con "Teste de resistência de Constituição" open=false dc=0 show=true group=true save=true asked=0 count=0 needed=0 verdict=true/false youRoll=false | Brisa answered=true faces=[20] mod=4 total=24 phys=false mode=ROLL_MODE_KIND_NORMAL known=true master=false passed=true/true`,
	})

	if _, err := a.askOnly(t, "ability:str", func(r *playv1.RequestGroupCheckRequest) { r.ShowDc = true }, a.toren, a.bri); err != nil {
		t.Fatalf("asking two error = %v", err)
	}
	a.wantGroupDigests(t, "two asked, each alone", map[string]string{
		"master":             `ability:str "Teste de Força" open=true dc=12 show=true group=false save=false asked=2 count=0 needed=0 verdict=false/false youRoll=false | Brisa answered=false passed=false/false | Toren answered=false passed=false/false`,
		"Toren's player":     `ability:str "Teste de Força" open=true dc=0 show=true group=false save=false asked=0 count=0 needed=0 verdict=false/false youRoll=true option=+3/true/ROLL_MODE_KIND_NORMAL | Toren answered=false passed=false/false`,
		"Pensantus's player": `none`,
		"Brisa's player":     `ability:str "Teste de Força" open=true dc=0 show=true group=false save=false asked=0 count=0 needed=0 verdict=false/false youRoll=true option=+0/true/ROLL_MODE_KIND_NORMAL | Brisa answered=false passed=false/false`,
	})
}
