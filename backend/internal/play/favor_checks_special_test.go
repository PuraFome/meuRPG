package play

import (
	"testing"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The cases of the checks that are not the table's: the master's rolls for a player, the calls that roll no d20, the reaction windows that
// hold a check of a combat, the search for traps in a combat and the Perception search.

// Only the owner spends: the master who rolls a combat check for the player's character is told he is not the holder, and the Inspiration
// stays.
func TestFavorOfTheMasterOnACombatCheckIsNotTheHolders(t *testing.T) {
	t.Parallel()
	fire := map[string]func(t *testing.T, e *checkEnv) error{
		"a contest start": func(t *testing.T, e *checkEnv) error {
			t.Helper()
			_, err := e.fight.startContest(t, e.master, func(r *playv1.StartContestRequest) {
				r.InitiatorId, r.TargetId, r.Purpose, r.SpendFavor = e.fight.id(t, "Toren"), e.fight.id(t, "Goblin"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE, true
			})
			return err
		},
		"a hide": func(t *testing.T, e *checkEnv) error {
			t.Helper()
			_, err := e.master.contests.Hide(t.Context(), connect.NewRequest(&playv1.HideRequest{
				CampaignId: e.campaignID, EncounterId: e.fight.e.GetId(), IdempotencyKey: newKey(), CombatantId: e.fight.id(t, "Toren"), ActionKey: actionHide,
				Roll: rollIn(), SpendFavor: true,
			}))
			return err
		},
	}
	for _, mk := range []struct {
		name string
		env  func(*testing.T) *checkEnv
	}{{"a contest start", newContestStartEnv}, {"a hide", newHideEnv}} {
		t.Run(mk.name, func(t *testing.T) {
			t.Parallel()
			e := mk.env(t)
			e.give(t, e.who)
			e.h.roller.queue(4, 15)
			seq := e.seq(t)
			wantFavorBlocked(t, mk.name+" of the master", fire[mk.name](t, e), playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOT_THE_HOLDER)
			e.untouched(t, "the master's roll", 2, seq)
			if !e.favorHolds(t) {
				t.Error("the master spent the player's Inspiration")
			}
		})
	}
	t.Run("a contest answer", func(t *testing.T) {
		t.Parallel()
		e := newContestAnswerEnv(t)
		e.give(t, e.who)
		contestID := e.contestID(t)
		e.h.roller.queue(4, 15)
		seq := e.seq(t)
		_, err := e.master.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
			CampaignId: e.campaignID, EncounterId: e.fight.e.GetId(), IdempotencyKey: newKey(), ContestId: contestID,
			Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, Roll: rollIn(), SpendFavor: true,
		}))
		wantFavorBlocked(t, "the master's answer", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOT_THE_HOLDER)
		e.untouched(t, "the master's answer", 2, seq)
		if !e.favorHolds(t) {
			t.Error("the master spent the player's Inspiration")
		}
	})
}

// contestID is the contest of the Goblin's grapple that waits for Toren's answer.
func (e *checkEnv) contestID(t *testing.T) string {
	t.Helper()
	var id string
	if err := e.h.pool.QueryRow(t.Context(), `SELECT id FROM combat_contests ORDER BY created_at DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("find the contest: %v", err)
	}
	return id
}

// A call that rolls no d20 has nothing to spend the Inspiration on: the owner who leaves the answer to the master is told there is
// nothing to roll; a grapple that came from an attack was rolled already and is the master's (not the holder), and the row stays.
func TestFavorOnAContestThatRollsNothing(t *testing.T) {
	t.Parallel()
	t.Run("an answer left to the master", func(t *testing.T) {
		t.Parallel()
		e := newContestAnswerEnv(t)
		e.give(t, e.who)
		seq := e.seq(t)
		_, err := e.roller.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
			CampaignId: e.campaignID, EncounterId: e.fight.e.GetId(), IdempotencyKey: newKey(), ContestId: e.contestID(t),
			Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, DeferToMaster: true, SpendFavor: true,
		}))
		wantFavorBlocked(t, "the answer left to the master with a spend", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOTHING_TO_ROLL)
		if !e.favorHolds(t) || e.seq(t) != seq {
			t.Errorf("Inspiration there = %v, number %d -> %d, want the row kept and the number still", e.favorHolds(t), seq, e.seq(t))
		}
		// Without the spend the answer is left as always.
		if _, err := e.roller.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
			CampaignId: e.campaignID, EncounterId: e.fight.e.GetId(), IdempotencyKey: newKey(), ContestId: e.contestID(t),
			Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS, DeferToMaster: true,
		})); err != nil {
			t.Errorf("the answer left to the master error = %v", err)
		}
	})
	t.Run("a grapple that came from an attack", func(t *testing.T) {
		t.Parallel()
		e := newContestStartEnv(t)
		e.give(t, e.who)
		seq := e.seq(t)
		_, err := e.fight.startContest(t, e.master, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = e.fight.id(t, "Toren"), e.fight.id(t, "Goblin"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
			r.Kind, r.EscapeDc, r.Roll, r.SpendFavor = playv1.ContestKind_CONTEST_KIND_ESCAPE_DC, 12, nil, true
		})
		wantFavorBlocked(t, "the grapple of an attack with a spend", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_NOT_THE_HOLDER)
		if !e.favorHolds(t) || e.seq(t) != seq {
			t.Errorf("Inspiration there = %v, number %d -> %d, want the row kept and the number still", e.favorHolds(t), seq, e.seq(t))
		}
	})
}

// The master's roll for a player's group check has no field to spend with: it rolls as always and the player's Inspiration stays.
func TestFavorStaysWhenTheMasterRollsTheGroupCheckForAPlayer(t *testing.T) {
	t.Parallel()
	e := newGroupCheckEnv(t)
	e.give(t, e.who)
	var checkID string
	if err := e.h.pool.QueryRow(t.Context(), `SELECT id FROM group_checks ORDER BY created_at DESC LIMIT 1`).Scan(&checkID); err != nil {
		t.Fatalf("find the group check: %v", err)
	}
	e.h.roller.queue(8)
	res, err := e.master.contests.RollForPlayer(t.Context(), connect.NewRequest(&playv1.RollForPlayerRequest{
		CampaignId: e.campaignID, GroupCheckId: checkID, CharacterId: e.who.GetId(), IdempotencyKey: newKey(), Roll: rollIn(),
	}))
	if err != nil {
		t.Fatalf("RollForPlayer() error = %v", err)
	}
	if roll := memberOf(res.Msg.GetGroupCheck(), e.who.GetId()).GetRoll(); len(roll.GetFaces()) != 1 || roll.GetMode() != playv1.RollModeKind_ROLL_MODE_KIND_NORMAL {
		t.Errorf("the master's roll = %v, want one d20 in a normal roll", roll)
	}
	if !e.favorHolds(t) || e.stored(t) {
		t.Errorf("Inspiration there = %v, the roll says it was spent = %v, want it kept and the roll not to say so", e.favorHolds(t), e.stored(t))
	}
}

// windowHeld rolls a combat check of the table with the master's check window open before the d20.
func (e *checkEnv) windowHeld(t *testing.T) *playv1.ReactionWindow {
	t.Helper()
	w := e.windowOf(t, e.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil {
		t.Fatal("no master check window is open")
	}
	return w
}

// A reaction window holds a check of a combat before the d20 (Cutting Words): the hold only checks the Inspiration is there, with nothing
// spent; the replay is the roll and spends it; one the replay finds gone drops the check, and nothing is spent.
func TestFavorOnACombatCheckAWindowHolds(t *testing.T) {
	t.Parallel()
	for _, p := range checkEnvs {
		if p.name != "a contest start" && p.name != "a contest answer" && p.name != "a hide" {
			continue
		}
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			e := p.mk(t)
			e.setRules(t, enemyReactionsAlways)

			// With none, the hold itself is refused: the player does not wait for the windows to learn it.
			_, err := e.send(t, newKey(), true)
			wantFavorBlocked(t, "the spend with none on a check a window holds", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_HAS_NONE)
			if w := e.windowOf(t, e.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK); w != nil {
				t.Fatalf("a refused spend left the window %v", w)
			}
			// An advantage the Inspiration cannot add to is refused at the hold too.
			e.give(t, e.who)
			e.advantage(t)
			_, err = e.send(t, newKey(), true)
			wantFavorBlocked(t, "the spend that adds nothing on a check a window holds", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_ADDS_NOTHING)
			if !e.favorHolds(t) {
				t.Fatal("a refused hold took the Inspiration")
			}
		})
	}
}

func TestFavorOnTheReplayOfAHeldCombatCheck(t *testing.T) {
	t.Parallel()
	for _, p := range checkEnvs {
		if p.name != "a contest start" && p.name != "a contest answer" && p.name != "a hide" {
			continue
		}
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			e := p.mk(t)
			e.setRules(t, enemyReactionsAlways)
			e.give(t, e.who)
			held, err := e.send(t, newKey(), true)
			if err != nil || !held.heldByWindow {
				t.Fatalf("the spend on a check a window holds = %+v, %v, want it held", held, err)
			}
			if !e.favorHolds(t) {
				t.Fatal("the Inspiration was spent while the check waits for the window")
			}
			w := e.windowHeld(t)
			e.h.roller.queue(3, 17)
			e.mustAnswer(t, e.master, e.fight.refresh(t), w.GetId(), passAnswer)
			if e.favorHolds(t) || !e.stored(t) {
				t.Errorf("after the replay the Inspiration is there = %v and the roll says it was spent = %v, want it spent by the replay", e.favorHolds(t), e.stored(t))
			}
		})
	}
}

func TestFavorGoneAtTheReplayDropsTheCombatCheck(t *testing.T) {
	t.Parallel()
	for _, p := range checkEnvs {
		if p.name != "a contest start" && p.name != "a contest answer" && p.name != "a hide" {
			continue
		}
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			e := p.mk(t)
			e.setRules(t, enemyReactionsAlways)
			e.give(t, e.who)
			held, err := e.send(t, newKey(), true)
			if err != nil || !held.heldByWindow {
				t.Fatalf("the spend on a check a window holds = %+v, %v, want it held", held, err)
			}
			if _, err := e.h.pool.Exec(t.Context(), `DELETE FROM character_favor WHERE character_id = $1`, e.who.GetId()); err != nil { // given away meanwhile
				t.Fatalf("take the Inspiration away: %v", err)
			}
			w := e.windowHeld(t)
			e.mustAnswer(t, e.master, e.fight.refresh(t), w.GetId(), passAnswer)
			var state string
			if err := e.h.pool.QueryRow(t.Context(), `SELECT state FROM reaction_holds ORDER BY created_at DESC LIMIT 1`).Scan(&state); err != nil {
				t.Fatalf("read the hold: %v", err)
			}
			if state != "dropped" {
				t.Errorf("the hold is %q, want it dropped", state)
			}
			if e.favorHolds(t) {
				t.Error("the replay gave an Inspiration out of nothing")
			}
		})
	}
}

// A search for traps in a combat is the Search action of a combatant: the spend and the action go in the same transaction. A refusal keeps
// both the Inspiration and the action.
func TestFavorOnATrapSearchInACombat(t *testing.T) {
	t.Parallel()
	r := newTrapRig(t)
	r.trap(t, "Fosso Dourado", 7, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc = 20 })
	e := r.fight(t)
	f := favorPartyOf(t, r.armed)
	search := func(spend bool) (*playv1.SearchForTrapsResponse, error) {
		return r.trapSearchCall(t, r.caio, func(q *playv1.SearchForTrapsRequest) {
			q.Skill, q.Roll, q.SpendFavor = perception, &playv1.SearchForTrapsRequest_RollInApp{RollInApp: true}, spend
		})
	}
	r.h.roller.queue(4, 15)
	_, err := search(true)
	wantFavorBlocked(t, "the search with none", err, playv1.FavorBlockedReason_FAVOR_BLOCKED_REASON_HAS_NONE)
	if byLabel(t, r.get(t, r.master), "Toren").GetActionUsed() {
		t.Error("a refused search spent the action")
	}

	if err := f.grant(r.toren.GetId()); err != nil {
		t.Fatalf("GrantFavor() error = %v", err)
	}
	got, err := search(true)
	if err != nil {
		t.Fatalf("the search with the Inspiration error = %v", err)
	}
	if faces := got.GetRoll().GetFaces(); len(faces) != 2 || got.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || !(checkRolled{labels: sourceLabels(got.GetSources())}).hasFavor() || !got.GetSpentAction() {
		t.Errorf("the search = %v, want two d20, an advantage, the source Inspiração and the action spent", got)
	}
	if f.favorHolds(t) || !byLabel(t, r.get(t, r.master), "Toren").GetActionUsed() {
		t.Errorf("the Inspiration is there = %v, the action is used = %v, want both spent together", f.favorHolds(t), byLabel(t, r.get(t, r.master), "Toren").GetActionUsed())
	}
	if evs := r.trapSearchEvents(t); len(evs) != 1 || !evs[0].Favor {
		t.Errorf("the trap_searched events = %+v, want the one that says the Inspiration was spent", evs)
	}
	_ = e
}

// A Perception search rolls a second die for the dim light; under a spend the faces are the pair of the advantage, and the typed second
// die of the light is not read.
func TestFavorOnAPerceptionSearch(t *testing.T) {
	t.Parallel()
	e := newTrapSearchEnv(t)
	e.give(t, e.who)
	e.h.roller.queue(4, 15)
	r, err := e.roller.play.SearchForTraps(t.Context(), connect.NewRequest(&playv1.SearchForTrapsRequest{
		CampaignId: e.campaignID, IdempotencyKey: newKey(), Skill: perception, SpendFavor: true,
		Roll: &playv1.SearchForTrapsRequest_RollInApp{RollInApp: true},
	}))
	if err != nil {
		t.Fatalf("the Perception search with the Inspiration error = %v", err)
	}
	got := trapRolledOf(r.Msg)
	if len(got.faces) != 2 || got.mode != "advantage" || !got.hasFavor() || e.favorHolds(t) || !e.stored(t) {
		t.Errorf("the search = %+v (Inspiration there %v), want the pair, the advantage and the Inspiration spent", got, e.favorHolds(t))
	}
}
