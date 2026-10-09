package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Group checks (SRD 5.1, "Working Together", Group Checks). These tests need the database
// (MEURPG_TEST_DATABASE_URL). The fixture is newArmed with an open session: Toren, Pensantus and
// Brisa, whose Stealth modifiers are +2, +2 and +3 (humans have +1 on every score; no skill proficiency).

const stealthKey = "skill:stealth"

// requestGroup calls RequestGroupCheck as u: Stealth, DC 12, the DC hidden, unless edit says other.
func (a *armed) requestGroup(t *testing.T, u *user, edit func(*playv1.RequestGroupCheckRequest)) (*playv1.RequestGroupCheckResponse, error) {
	t.Helper()
	req := &playv1.RequestGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), SkillKey: stealthKey, Dc: 12}
	if edit != nil {
		edit(req)
	}
	res, err := u.contests.RequestGroupCheck(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustRequestGroup(t *testing.T, edit func(*playv1.RequestGroupCheckRequest)) *playv1.GroupCheckView {
	t.Helper()
	res, err := a.requestGroup(t, a.master, edit)
	if err != nil {
		t.Fatalf("RequestGroupCheck() error = %v", err)
	}
	return res.GetGroupCheck()
}

// rollGroup calls RollGroupCheck as u, with the d20 the roller will give.
func (a *armed) rollGroup(t *testing.T, u *user, checkID string, face int) (*playv1.RollGroupCheckResponse, error) {
	t.Helper()
	a.h.roller.queue(face)
	return a.rollGroupWith(t, u, checkID, rollIn())
}

func (a *armed) rollGroupWith(t *testing.T, u *user, checkID string, in *playv1.CheckRollInput) (*playv1.RollGroupCheckResponse, error) {
	t.Helper()
	res, err := u.contests.RollGroupCheck(t.Context(), connect.NewRequest(&playv1.RollGroupCheckRequest{
		CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: checkID, Roll: in,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustRollGroup(t *testing.T, u *user, checkID string, face int) *playv1.GroupCheckView {
	t.Helper()
	res, err := a.rollGroup(t, u, checkID, face)
	if err != nil {
		t.Fatalf("RollGroupCheck() error = %v", err)
	}
	return res.GetGroupCheck()
}

// rollForPlayer calls RollForPlayer as u with the d20 the roller will give.
func (a *armed) rollForPlayer(t *testing.T, u *user, checkID, characterID string, face int) (*playv1.RollForPlayerResponse, error) {
	t.Helper()
	a.h.roller.queue(face)
	res, err := u.contests.RollForPlayer(t.Context(), connect.NewRequest(&playv1.RollForPlayerRequest{
		CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: checkID, CharacterId: characterID, Roll: rollIn(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) closeGroup(t *testing.T, u *user, checkID string) (*playv1.CloseGroupCheckResponse, error) {
	t.Helper()
	res, err := u.contests.CloseGroupCheck(t.Context(), connect.NewRequest(&playv1.CloseGroupCheckRequest{
		CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: checkID,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustCloseGroup(t *testing.T, checkID string) *playv1.GroupCheckView {
	t.Helper()
	res, err := a.closeGroup(t, a.master, checkID)
	if err != nil {
		t.Fatalf("CloseGroupCheck() error = %v", err)
	}
	return res.GetGroupCheck()
}

// groupView calls GetGroupCheck as u.
func (a *armed) groupView(t *testing.T, u *user) *playv1.GroupCheckView {
	t.Helper()
	res, err := u.contests.GetGroupCheck(t.Context(), connect.NewRequest(&playv1.GetGroupCheckRequest{CampaignId: a.campaignID}))
	if err != nil {
		t.Fatalf("GetGroupCheck() error = %v", err)
	}
	return res.Msg.GetGroupCheck()
}

// memberOf is the member of the view who is the character, or nil.
func memberOf(v *playv1.GroupCheckView, characterID string) *playv1.GroupCheckMemberView {
	for _, m := range v.GetMembers() {
		if m.GetCharacterId() == characterID {
			return m
		}
	}
	return nil
}

// TestGroupCheckAsksThePartyAndEachPlayerReadsOnlyTheirOwnRoll (SRD 5.1, "Group Checks"; RN-20):
// the master asks, each player rolls for their own character and reads their own roll, never
// another's, the DC, or whether it passed (the master did not show the DC); the master reads it all.
func TestGroupCheckAsksThePartyAndEachPlayerReadsOnlyTheirOwnRoll(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, nil)
	if !asked.GetOpen() || asked.GetSkillKey() != stealthKey || asked.GetSkillNamePt() == "" || asked.GetDc() != 12 || len(asked.GetMembers()) != 3 ||
		asked.GetNeeded() != 2 || asked.GetPassedCount() != 0 {
		t.Fatalf("the master's check = %v, want it open, DC 12, three characters, two needed", asked)
	}
	for _, m := range asked.GetMembers() {
		if m.GetAnswered() || m.GetName() == "" {
			t.Errorf("member %v, want a name and no answer yet", m)
		}
	}

	// Before rolling, a player reads that it is their turn to roll and what their roll adds.
	mine := a.groupView(t, a.caio)
	if !mine.GetYouRoll() || mine.GetYourOption().GetModifier() != 2 || !mine.GetYourOption().GetKnown() || mine.GetDc() != 0 || len(mine.GetMembers()) != 1 ||
		mine.GetMembers()[0].GetCharacterId() != a.toren.GetId() || mine.GetNeeded() != 0 || mine.GetPassedCount() != 0 {
		t.Fatalf("Toren's player reads %v, want only their own, +2, no DC and no count", mine)
	}

	if v := a.mustRollGroup(t, a.caio, asked.GetId(), 10); memberOf(v, a.toren.GetId()).GetRoll().GetTotal() != 12 || v.GetYouRoll() {
		t.Fatalf("Toren's answer = %v, want 12 and nothing more to roll", v)
	}
	a.mustRollGroup(t, a.ana, asked.GetId(), 9) // 11
	a.mustRollGroup(t, a.bia, asked.GetId(), 8) // 11
	all := a.groupView(t, a.master)
	if all.GetPassedCount() != 1 || !all.GetVerdictKnown() || all.GetGroupPassed() {
		t.Errorf("the master reads %v, want one of three passing and the group failing", all)
	}
	for _, id := range []string{a.toren.GetId(), a.pens.GetId(), a.bri.GetId()} {
		if m := memberOf(all, id); m == nil || !m.GetAnswered() || !m.GetPassedKnown() || m.GetRoll() == nil {
			t.Errorf("the master reads member %v, want the roll and the pass", m)
		}
	}

	// Every player reads only their own roll: not the others', the DC, the count or the verdict.
	for who, p := range map[string]struct {
		u    *user
		self string
		got  int32
	}{
		"Toren's player":     {a.caio, a.toren.GetId(), 12},
		"Pensantus's player": {a.ana, a.pens.GetId(), 11},
		"Brisa's player":     {a.bia, a.bri.GetId(), 11},
	} {
		v := a.groupView(t, p.u)
		if len(v.GetMembers()) != 1 || v.GetMembers()[0].GetCharacterId() != p.self || v.GetMembers()[0].GetRoll().GetTotal() != p.got {
			t.Errorf("%s reads %v, want only their own roll (%d)", who, v.GetMembers(), p.got)
		}
		if m := v.GetMembers()[0]; m.GetPassedKnown() || m.GetPassed() {
			t.Errorf("%s reads whether they passed (%v) but the master did not show the DC", who, m)
		}
		if v.GetDc() != 0 || v.GetPassedCount() != 0 || v.GetNeeded() != 0 || v.GetVerdictKnown() {
			t.Errorf("%s reads the DC, the count or the verdict: %v", who, v)
		}
	}

	// Closed, the players still read no verdict (the DC stays hidden).
	closed := a.mustCloseGroup(t, asked.GetId())
	if closed.GetOpen() || !closed.GetVerdictKnown() || closed.GetGroupPassed() {
		t.Errorf("the closed check = %v, want the verdict failed for the master", closed)
	}
	for who, u := range map[string]*user{"Toren's player": a.caio, "Pensantus's player": a.ana} {
		if v := a.groupView(t, u); v.GetVerdictKnown() || v.GetDc() != 0 || v.GetOpen() {
			t.Errorf("%s reads %v after the close, want it closed with no verdict", who, v)
		}
	}
}

// TestGroupCheckPassesWhenAtLeastHalfPassAndTheUnansweredFail (SRD 5.1, "Group Checks": if at least
// half the group succeeds, the whole group succeeds): with the DC shown the players read their own
// pass and the verdict. Stealth is +2, +2, +3, the DC 12.
func TestGroupCheckPassesWhenAtLeastHalfPassAndTheUnansweredFail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// faces by player: "Toren", "Pensantus", "Brisa"; a player missing from it never answers.
		faces map[string]int
		want  bool
		count int32
	}{
		{"two of three pass", map[string]int{"Toren": 10, "Pensantus": 10, "Brisa": 2}, true, 2},
		{"one of three passes", map[string]int{"Toren": 10, "Pensantus": 3, "Brisa": 5}, false, 1},
		{"all three pass", map[string]int{"Toren": 20, "Pensantus": 20, "Brisa": 20}, true, 3},
		{"two pass and one never answers", map[string]int{"Toren": 10, "Pensantus": 10}, true, 2},
		{"one passes and two never answer", map[string]int{"Toren": 10}, false, 1},
		{"nobody answers", map[string]int{}, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newArmed(t)
			asked := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.ShowDc = true })
			players := map[string]*user{"Toren": a.caio, "Pensantus": a.ana, "Brisa": a.bia}
			for name, face := range tc.faces {
				a.mustRollGroup(t, players[name], asked.GetId(), face)
			}
			closed := a.mustCloseGroup(t, asked.GetId())
			if closed.GetGroupPassed() != tc.want || !closed.GetVerdictKnown() || closed.GetPassedCount() != tc.count || closed.GetNeeded() != 2 {
				t.Fatalf("the master reads %v, want %d passing and the group %v", closed, tc.count, tc.want)
			}
			for name, u := range players {
				v := a.groupView(t, u)
				if !v.GetVerdictKnown() || v.GetGroupPassed() != tc.want {
					t.Errorf("%s's player reads the verdict %v/%v, want %v (the DC is shown)", name, v.GetVerdictKnown(), v.GetGroupPassed(), tc.want)
				}
				if v.GetDc() != 0 || v.GetPassedCount() != 0 {
					t.Errorf("%s's player reads the DC or the count: %v", name, v)
				}
				if _, answered := tc.faces[name]; answered && !v.GetMembers()[0].GetPassedKnown() {
					t.Errorf("%s's player does not read whether they passed, but the DC is shown", name)
				}
			}
		})
	}
}

// TestGroupCheckOfAnEvenPartyPassesWithExactlyHalf (SRD 5.1, "Group Checks": at least half): with four
// characters asked, two passing is enough and one is not.
func TestGroupCheckOfAnEvenPartyPassesWithExactlyHalf(t *testing.T) {
	t.Parallel()
	for name, passing := range map[string]int{"two of four": 2, "one of four": 1} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := newArmed(t)
			dani := a.h.newUser("Dani")
			a.h.join(a.master, a.campaignID, dani)
			fourth := dani.hero(t, a.campaignID, "Dara", "class:fighter", "race:human", 2, scores16(), []string{rapier}, nil)
			asked := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.ShowDc = true })
			if len(asked.GetMembers()) != 4 || asked.GetNeeded() != 2 {
				t.Fatalf("the check = %v, want four characters and two needed", asked)
			}
			players := []*user{a.caio, a.ana, a.bia, dani}
			for i, u := range players {
				face := 2 // a fail
				if i < passing {
					face = 20
				}
				a.mustRollGroup(t, u, asked.GetId(), face)
			}
			closed := a.mustCloseGroup(t, asked.GetId())
			if want := passing >= 2; closed.GetGroupPassed() != want || closed.GetPassedCount() != int32(passing) {
				t.Errorf("the master reads %v, want %d passing and the group %v", closed, passing, want)
			}
			if memberOf(closed, fourth.GetId()) == nil {
				t.Errorf("the fourth character is not in the check: %v", closed)
			}
		})
	}
}

// TestMasterRollsForAPlayerWhoDidNotAnswerAndClosesTheCheck: the master's roll for a character is
// marked as his, counts once, and the check closes for good.
func TestMasterRollsForAPlayerWhoDidNotAnswerAndClosesTheCheck(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.ShowDc = true })

	if _, err := a.rollForPlayer(t, a.caio, asked.GetId(), a.bri.GetId(), 10); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player rolling for another = %v, want permission_denied", err)
	}
	res, err := a.rollForPlayer(t, a.master, asked.GetId(), a.bri.GetId(), 10) // 10 + 3 = 13
	if err != nil {
		t.Fatalf("RollForPlayer() error = %v", err)
	}
	if m := memberOf(res.GetGroupCheck(), a.bri.GetId()); m == nil || !m.GetAnswered() || m.GetRoll().GetTotal() != 13 || !m.GetRoll().GetRolledByMaster() || !m.GetPassed() {
		t.Fatalf("Brisa's member = %v, want 13 rolled by the master and passing", m)
	}
	// Brisa reads that the master rolled for her; she answered once.
	if m := memberOf(a.groupView(t, a.bia), a.bri.GetId()); !m.GetRoll().GetRolledByMaster() || m.GetRoll().GetTotal() != 13 {
		t.Errorf("Brisa's player reads %v, want the master's roll for her", m)
	}
	if v := a.groupView(t, a.bia); v.GetYouRoll() {
		t.Error("Brisa is told to roll after the master rolled for her")
	}
	_, err = a.rollGroup(t, a.bia, asked.GetId(), 15)
	wantContestBlocked(t, "Brisa rolling after the master did", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_ALREADY_ANSWERED)
	_, err = a.rollForPlayer(t, a.master, asked.GetId(), a.bri.GetId(), 15)
	wantContestBlocked(t, "the master rolling twice for the same character", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_ALREADY_ANSWERED)
	_, err = a.rollForPlayer(t, a.master, asked.GetId(), a.goblin.GetId(), 15)
	wantContestBlocked(t, "the master rolling for a character nobody asked", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_NOT_IN_GROUP_CHECK)
	_, err = a.rollForPlayer(t, a.master, newKey(), a.toren.GetId(), 15)
	wantCode(t, "a group check that does not exist", err, connect.CodeNotFound)

	// A second check is refused while one is open; closing ends it for everybody.
	_, err = a.requestGroup(t, a.master, nil)
	wantContestBlocked(t, "a second group check while one is open", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_OPEN)
	closed := a.mustCloseGroup(t, asked.GetId())
	if closed.GetOpen() || closed.GetPassedCount() != 1 {
		t.Fatalf("the closed check = %v, want it closed with one passing", closed)
	}
	_, err = a.rollGroup(t, a.caio, asked.GetId(), 12)
	wantContestBlocked(t, "rolling after the close", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED)
	_, err = a.rollForPlayer(t, a.master, asked.GetId(), a.toren.GetId(), 12)
	wantContestBlocked(t, "the master rolling after the close", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED)
	_, err = a.closeGroup(t, a.master, asked.GetId())
	wantContestBlocked(t, "closing twice", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_GROUP_CHECK_CLOSED)
	if v := a.groupView(t, a.caio); v.GetId() != asked.GetId() || v.GetOpen() {
		t.Errorf("GetGroupCheck after the close = %v, want the latest check, closed", v)
	}
	// With the first closed, the master asks another.
	next := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.SkillKey, r.Dc = "ability:str", 0 })
	if !next.GetOpen() || next.GetId() == asked.GetId() || next.GetSkillKey() != "ability:str" {
		t.Errorf("the next check = %v, want another, open, on Strength", next)
	}
}

// TestGroupCheckRollsFollowTheCampaignsDiceAndAnswerOnce: the player rolls in the app or with the
// physical d20 the way the campaign has them roll, once, for their own character only.
func TestGroupCheckRollsFollowTheCampaignsDiceAndAnswerOnce(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, nil)

	// The master is no player of the table: he rolls for them, not as them.
	if _, err := a.rollGroup(t, a.master, asked.GetId(), 10); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("the master rolling as a player = %v, want permission_denied", err)
	}
	// A pending member has no character here and is not in the campaign yet.
	pending := a.h.newUser("Pendente")
	a.h.joinPending(a.master, a.campaignID, pending)
	if _, err := a.rollGroup(t, pending, asked.GetId(), 10); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a pending member rolling = %v, want not_found", err)
	}

	// The campaign has everyone roll physical dice: one face, 1 to 20.
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	_, err := a.rollGroupWith(t, a.caio, asked.GetId(), rollIn())
	wantBlockedBy(t, "an app roll in a campaign that rolls physical dice", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE)
	// The faces typed must match the roll the server works out: two for a normal roll are a mode that changed.
	if _, err := a.rollGroupWith(t, a.caio, asked.GetId(), faces(4, 9)); connect.CodeOf(err) != connect.CodeAborted {
		t.Errorf("two faces for a normal roll = %v, want aborted", err)
	}
	for name, in := range map[string]*playv1.CheckRollInput{"face 0": faces(0), "face 21": faces(21), "no faces": faces()} {
		if _, err := a.rollGroupWith(t, a.caio, asked.GetId(), in); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s = %v, want invalid_argument", name, err)
		}
	}
	res, err := a.rollGroupWith(t, a.caio, asked.GetId(), faces(14))
	if err != nil {
		t.Fatalf("RollGroupCheck(physical) error = %v", err)
	}
	if m := memberOf(res.GetGroupCheck(), a.toren.GetId()); !m.GetRoll().GetPhysical() || m.GetRoll().GetTotal() != 16 {
		t.Errorf("Toren's roll = %v, want the typed 14 plus 2, physical", m.GetRoll())
	}
	_, err = a.rollGroupWith(t, a.caio, asked.GetId(), faces(14))
	wantContestBlocked(t, "answering twice", err, playv1.ContestBlockedReason_CONTEST_BLOCKED_REASON_ALREADY_ANSWERED)

	// The campaign makes everyone roll in the app.
	a.forceDice(t, campaignsv1.DiceMode_DICE_MODE_APP)
	_, err = a.rollGroupWith(t, a.ana, asked.GetId(), faces(10))
	wantBlockedBy(t, "typed dice in a campaign that rolls in the app", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_WRONG_DICE_MODE)
	if v := a.mustRollGroup(t, a.ana, asked.GetId(), 10); memberOf(v, a.pens.GetId()).GetRoll().GetTotal() != 12 {
		t.Errorf("Pensantus's roll = %v, want 12", v)
	}
}

// TestGroupCheckRequestRefusesWhatIsNotACheckOrADC: the skill must be a skill or an ability check
// (never a save), the DC 1 to 40 or 0 for none (then there is no verdict, not even the master's).
func TestGroupCheckRequestRefusesWhatIsNotACheckOrADC(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	for name, edit := range map[string]func(*playv1.RequestGroupCheckRequest){
		"a saving throw":       func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = "save:dex" },
		"no skill":             func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = "" },
		"a skill that is none": func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = "skill:no-such-skill" },
		"a bare prefix":        func(r *playv1.RequestGroupCheckRequest) { r.SkillKey = "skill:" },
		"a DC of 41":           func(r *playv1.RequestGroupCheckRequest) { r.Dc = 41 },
		"a negative DC":        func(r *playv1.RequestGroupCheckRequest) { r.Dc = -1 },
	} {
		if _, err := a.requestGroup(t, a.master, edit); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s = %v, want invalid_argument", name, err)
		}
	}
	if v := a.groupView(t, a.master); v != nil {
		t.Fatalf("a refused request left %v", v)
	}
	if _, err := a.requestGroup(t, a.caio, nil); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("a player asking a group check = %v, want permission_denied", err)
	}
	none := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc, r.ShowDc = 0, true })
	for _, u := range []*user{a.caio, a.ana, a.bia} {
		a.mustRollGroup(t, u, none.GetId(), 12)
	}
	closed := a.mustCloseGroup(t, none.GetId())
	if closed.GetVerdictKnown() || closed.GetDc() != 0 {
		t.Errorf("a check with no DC = %v, want no verdict", closed)
	}
	if v := a.groupView(t, a.caio); v.GetVerdictKnown() || v.GetMembers()[0].GetPassedKnown() {
		t.Errorf("a player reads a verdict of a check with no DC: %v", v)
	}
}

// TestGroupCheckWritesAreIdempotent: the same key and request answers the same check and changes
// nothing more (nothing is rolled again), the same key with another request is refused.
func TestGroupCheckWritesAreIdempotent(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	ask := &playv1.RequestGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), SkillKey: stealthKey, Dc: 12}
	first, err := a.master.contests.RequestGroupCheck(t.Context(), connect.NewRequest(ask))
	if err != nil {
		t.Fatalf("RequestGroupCheck() error = %v", err)
	}
	again, err := a.master.contests.RequestGroupCheck(t.Context(), connect.NewRequest(ask))
	if err != nil || again.Msg.GetGroupCheck().GetId() != first.Msg.GetGroupCheck().GetId() {
		t.Fatalf("RequestGroupCheck(retry) = %v, %v, want the same check (not group_check_open)", again, err)
	}
	other := &playv1.RequestGroupCheckRequest{CampaignId: ask.GetCampaignId(), IdempotencyKey: ask.GetIdempotencyKey(), SkillKey: stealthKey, Dc: 15}
	_, err = a.master.contests.RequestGroupCheck(t.Context(), connect.NewRequest(other))
	wantCode(t, "the same key with another DC", err, connect.CodeInvalidArgument)
	id := first.Msg.GetGroupCheck().GetId()

	roll := &playv1.RollGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: id, Roll: rollIn()}
	a.h.roller.queue(10)
	if _, err := a.caio.contests.RollGroupCheck(t.Context(), connect.NewRequest(roll)); err != nil {
		t.Fatalf("RollGroupCheck() error = %v", err)
	}
	a.h.roller.queue(19) // a retry that rolled again would read 20
	retry, err := a.caio.contests.RollGroupCheck(t.Context(), connect.NewRequest(roll))
	if err != nil || memberOf(retry.Msg.GetGroupCheck(), a.toren.GetId()).GetRoll().GetTotal() != 12 {
		t.Errorf("RollGroupCheck(retry) = %v, %v, want the same 12 (not already_answered)", retry, err)
	}
	changed := &playv1.RollGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: roll.GetIdempotencyKey(), GroupCheckId: id, Roll: faces(3)}
	_, err = a.caio.contests.RollGroupCheck(t.Context(), connect.NewRequest(changed))
	wantCode(t, "the same key with another roll", err, connect.CodeInvalidArgument)

	forPlayer := &playv1.RollForPlayerRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: id, CharacterId: a.bri.GetId(), Roll: rollIn()}
	a.h.roller.queue(10)
	firstFor, err := a.master.contests.RollForPlayer(t.Context(), connect.NewRequest(forPlayer))
	if err != nil {
		t.Fatalf("RollForPlayer() error = %v", err)
	}
	total := memberOf(firstFor.Msg.GetGroupCheck(), a.bri.GetId()).GetRoll().GetTotal()
	a.h.roller.queue(18)
	retryFor, err := a.master.contests.RollForPlayer(t.Context(), connect.NewRequest(forPlayer))
	if err != nil || memberOf(retryFor.Msg.GetGroupCheck(), a.bri.GetId()).GetRoll().GetTotal() != total {
		t.Errorf("RollForPlayer(retry) = %v, %v, want the same %d (not already_answered, nothing rolled again)", retryFor, err, total)
	}
	changedFor := &playv1.RollForPlayerRequest{CampaignId: a.campaignID, IdempotencyKey: forPlayer.GetIdempotencyKey(), GroupCheckId: id, CharacterId: a.pens.GetId(), Roll: rollIn()}
	_, err = a.master.contests.RollForPlayer(t.Context(), connect.NewRequest(changedFor))
	wantCode(t, "the same key for another character", err, connect.CodeInvalidArgument)

	closeReq := &playv1.CloseGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: newKey(), GroupCheckId: id}
	if _, err := a.master.contests.CloseGroupCheck(t.Context(), connect.NewRequest(closeReq)); err != nil {
		t.Fatalf("CloseGroupCheck() error = %v", err)
	}
	if res, err := a.master.contests.CloseGroupCheck(t.Context(), connect.NewRequest(closeReq)); err != nil || res.Msg.GetGroupCheck().GetOpen() {
		t.Errorf("CloseGroupCheck(retry) = %v, %v, want the same closed check (not group_check_closed)", res, err)
	}
	otherClose := &playv1.CloseGroupCheckRequest{CampaignId: a.campaignID, IdempotencyKey: closeReq.GetIdempotencyKey(), GroupCheckId: newKey()}
	_, err = a.master.contests.CloseGroupCheck(t.Context(), connect.NewRequest(otherClose))
	wantCode(t, "the same key for another check", err, connect.CodeInvalidArgument)
}

// TestGroupCheckTellsTheTableWithoutContent: every change sends group_check_changed, a hint with no
// content: to everyone when the master asks, rolls for a player or closes; to the master and the
// roller alone when a player rolls.
func TestGroupCheckTellsTheTableWithoutContent(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	master, caio, ana := a.master.watch(t, a.campaignID), a.caio.watch(t, a.campaignID), a.ana.watch(t, a.campaignID)
	for _, w := range []*watcher{master, caio, ana} {
		w.ready(t)
	}
	mark := func() {
		a.master.markCurrentMap(t, a.campaignID, a.mapID, master, caio, ana)
	}
	changes := func(w *watcher) int {
		n := 0
		for _, ev := range w.beforeMarker(t) {
			if ev.GetGroupCheckChanged() != nil {
				n++
			} else {
				t.Errorf("event %v, want only group_check_changed", ev)
			}
		}
		return n
	}

	asked := a.mustRequestGroup(t, nil)
	mark()
	for who, w := range map[string]*watcher{"master": master, "Toren's player": caio, "Pensantus's player": ana} {
		if n := changes(w); n != 1 {
			t.Errorf("%s got %d changes for the request, want 1", who, n)
		}
	}
	a.mustRollGroup(t, a.caio, asked.GetId(), 10)
	mark()
	for who, want := range map[string]int{"master": 1, "Toren's player": 1, "Pensantus's player": 0} {
		got := changes(map[string]*watcher{"master": master, "Toren's player": caio, "Pensantus's player": ana}[who])
		if got != want {
			t.Errorf("%s got %d changes for Toren's roll, want %d (a player's roll is the master's and their own)", who, got, want)
		}
	}
	a.mustCloseGroup(t, asked.GetId())
	mark()
	for who, w := range map[string]*watcher{"master": master, "Toren's player": caio, "Pensantus's player": ana} {
		if n := changes(w); n != 1 {
			t.Errorf("%s got %d changes for the close, want 1", who, n)
		}
	}
}

// TestGroupCheckCallsRefuseWhoIsNotAllowed: the master alone asks, rolls for a player and closes; a
// stranger, a pending member and the master of another campaign are refused with not_found.
func TestGroupCheckCallsRefuseWhoIsNotAllowed(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	asked := a.mustRequestGroup(t, nil)
	if _, err := a.rollForPlayer(t, a.caio, asked.GetId(), a.bri.GetId(), 10); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("RollForPlayer as a player = %v, want permission_denied", err)
	}
	if _, err := a.closeGroup(t, a.caio, asked.GetId()); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("CloseGroupCheck as a player = %v, want permission_denied", err)
	}
	if v := a.groupView(t, a.master); !v.GetOpen() {
		t.Errorf("a refused close closed the check: %v", v)
	}
	o := a.outsiders(t)
	o.refusedHere(t, "GetGroupCheck", func(u *user, campaignID string) error {
		_, err := u.contests.GetGroupCheck(t.Context(), connect.NewRequest(&playv1.GetGroupCheckRequest{CampaignId: campaignID}))
		return err
	}, a.campaignID)
	o.refusedHere(t, "RequestGroupCheck", func(u *user, campaignID string) error {
		_, err := u.contests.RequestGroupCheck(t.Context(), connect.NewRequest(&playv1.RequestGroupCheckRequest{CampaignId: campaignID, IdempotencyKey: newKey(), SkillKey: stealthKey, Dc: 12}))
		return err
	}, a.campaignID)
	o.refusedHere(t, "RollGroupCheck", func(u *user, campaignID string) error {
		_, err := u.contests.RollGroupCheck(t.Context(), connect.NewRequest(&playv1.RollGroupCheckRequest{CampaignId: campaignID, IdempotencyKey: newKey(), GroupCheckId: asked.GetId(), Roll: rollIn()}))
		return err
	}, a.campaignID)
	o.refused(t, "RollForPlayer", func(u *user, campaignID string) error {
		_, err := u.contests.RollForPlayer(t.Context(), connect.NewRequest(&playv1.RollForPlayerRequest{
			CampaignId: campaignID, IdempotencyKey: newKey(), GroupCheckId: asked.GetId(), CharacterId: a.bri.GetId(), Roll: rollIn(),
		}))
		return err
	}, a.campaignID)
	o.refused(t, "CloseGroupCheck", func(u *user, campaignID string) error {
		_, err := u.contests.CloseGroupCheck(t.Context(), connect.NewRequest(&playv1.CloseGroupCheckRequest{CampaignId: campaignID, IdempotencyKey: newKey(), GroupCheckId: asked.GetId()}))
		return err
	}, a.campaignID)
}
