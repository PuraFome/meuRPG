package play

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

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

// A reaction window holds a check of a combat before the d20 (Cutting Words): the hold only checks the Inspiration is there, with
// nothing spent, and refuses a spend the replay would refuse.
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

// The replay of a check a window held is the roll: it spends the Inspiration.
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

// An Inspiration gone when the replay comes drops the check, and nothing is spent.
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
	r.fight(t)
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

// typedSearch is a search with typed dice and a spend, as Caio sends it.
func (e *checkEnv) typedSearch(t *testing.T, skill playv1.TrapSearchSkill, edit func(*playv1.SearchForTrapsRequest)) (checkRolled, error) {
	t.Helper()
	req := &playv1.SearchForTrapsRequest{CampaignId: e.campaignID, IdempotencyKey: newKey(), Skill: skill, SpendFavor: true}
	edit(req)
	res, err := e.roller.play.SearchForTraps(t.Context(), connect.NewRequest(req))
	if err != nil {
		return checkRolled{}, err
	}
	return trapRolledOf(res.Msg), nil
}

// A Perception search with a real die and a spend: the spend counts against the search's mode, so the typed faces are the two of the
// advantage; the extra d20_face_2 of the dim light is not read under it (the advantage rolls both totals from the pair), and one face is
// refused as for any advantage, with the Inspiration left.
func TestFavorOnATypedPerceptionSearch(t *testing.T) {
	t.Parallel()
	e := newTrapSearchEnv(t)
	e.give(t, e.who)
	e.h.roller.queue(4, 15)
	seq := e.seq(t)
	_, err := e.typedSearch(t, playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_PERCEPTION, func(q *playv1.SearchForTrapsRequest) { q.D20Faces = []int32{9} })
	if err == nil {
		t.Fatal("one face under a spend was accepted, want it refused as for any advantage")
	}
	if c := connect.CodeOf(err); c != connect.CodeAborted && c != connect.CodeInvalidArgument {
		t.Errorf("one face under a spend error = %v, want the dice-count refusal", err)
	}
	e.untouched(t, "the one face", 2, seq)
	if !e.favorHolds(t) {
		t.Fatal("a refused search took the Inspiration")
	}
	got, err := e.typedSearch(t, playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_PERCEPTION, func(q *playv1.SearchForTrapsRequest) {
		q.D20Faces, q.D20Face_2 = []int32{3, 17}, proto.Int32(1)
	})
	if err != nil {
		t.Fatalf("two faces and the light's die error = %v", err)
	}
	if !slices.Equal(got.faces, []int32{3, 17}) || got.mode != "advantage" || !got.hasFavor() || e.favorHolds(t) {
		t.Errorf("the search = %+v (Inspiration there %v), want the pair 3 and 17, the advantage and the Inspiration spent", got, e.favorHolds(t))
	}
}

// d20_face_2 is the light's die of a Perception search only: on any other skill it is not read at all under a spend, even out of range.
func TestFavorOnATypedSearchOfAnotherSkillIgnoresTheLightsDie(t *testing.T) {
	t.Parallel()
	e := newTrapSearchEnv(t)
	e.give(t, e.who)
	got, err := e.typedSearch(t, playv1.TrapSearchSkill_TRAP_SEARCH_SKILL_INVESTIGATION, func(q *playv1.SearchForTrapsRequest) {
		q.D20Faces, q.D20Face_2 = []int32{3, 17}, proto.Int32(99)
	})
	if err != nil {
		t.Fatalf("an Investigation search with a stray d20_face_2 error = %v, want it ignored", err)
	}
	if !slices.Equal(got.faces, []int32{3, 17}) || got.mode != "advantage" || e.favorHolds(t) {
		t.Errorf("the search = %+v (Inspiration there %v), want the pair, the advantage and the Inspiration spent", got, e.favorHolds(t))
	}
}

// A window's replay of a check that spends the Inspiration and then holds a Bardic die keeps the die's question with favor_spent (only
// a weapon attack's hold carries the window's mark); a retry under the same key finds it: never HAS_NONE, and no second spend.
func TestFavorRetryAfterAWindowReplayThatHoldsADie(t *testing.T) {
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
			e.die(t)
			key := newKey()
			held, err := e.send(t, key, true)
			if err != nil || !held.heldByWindow {
				t.Fatalf("the spend on a check a window holds = %+v, %v, want it held", held, err)
			}
			e.h.roller.queue(3, 17)
			e.mustAnswer(t, e.master, e.fight.refresh(t), e.windowHeld(t).GetId(), passAnswer)
			if e.favorHolds(t) {
				t.Fatal("the replay did not spend the Inspiration")
			}
			var spent bool
			if err := e.h.pool.QueryRow(t.Context(), `SELECT favor_spent FROM inspiration_holds WHERE idempotency_key = $1`, key).Scan(&spent); err != nil || !spent {
				t.Fatalf("the replay's hold: favor_spent %v, %v, want the die's question kept with the Inspiration spent (a check's hold is never a window's mark)", spent, err)
			}
			again, err := e.send(t, key, true)
			if err != nil {
				t.Fatalf("the retry of the key whose replay holds a die error = %v, want no refusal (the Inspiration was spent by that key)", err)
			}
			if again.holdID == "" && !again.heldByWindow && len(again.faces) != 2 {
				t.Errorf("the retry = %+v, want the die's question, the held check, or the written roll with its two dice", again)
			}
			if e.favorHolds(t) {
				t.Error("the retry gave the Inspiration back")
			}
		})
	}
}

// The same for the attacks: the replay of a weapon attack held by a window spends the Inspiration and holds the die's question under the
// window's mark, which a client's retry of the key does not find as its own. The retry must not be told HAS_NONE nor spend again.
func TestFavorRetryAfterAWindowReplayOfAnAttackThatHoldsADie(t *testing.T) {
	t.Parallel()
	p := favorPaths[0]
	f := newFavorFight(t)
	f.setRules(t, enemyReactionsAlways)
	p.ready(t, f)
	u, who := p.roller(f)
	f.giveCombatantDie(t, p.actor, f.e)
	f.give(t, who)
	key := newKey()
	held, err := p.send(t, f, u, key, true, nil)
	if err != nil || !held.heldByWindow {
		t.Fatalf("the spend on a roll a window holds = %+v, %v, want it held", held, err)
	}
	f.h.roller.queue(3, 17)
	f.mustAnswer(t, f.master, f.e, f.windowOf(t, f.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK).GetId(), passAnswer)
	if f.favorHolds(t) {
		t.Fatal("the replay did not spend the Inspiration")
	}
	var window, spent bool
	if err := f.h.pool.QueryRow(t.Context(), `SELECT window_hold, favor_spent FROM inspiration_holds WHERE idempotency_key = $1`, key).Scan(&window, &spent); err != nil || !spent {
		t.Fatalf("the replay's hold = window %v, favor_spent %v, %v, want the die's question kept with the Inspiration spent", window, spent, err)
	}
	t.Logf("the replay's hold carries the window's mark: %v", window)
	f.give(t, who) // another one: the retry must leave it
	_, err = p.send(t, f, u, key, true, nil)
	if err != nil {
		t.Errorf("the retry of the key whose replay holds a die error = %v, want no refusal", err)
	}
	if !f.favorHolds(t) {
		t.Error("the retry spent the second Inspiration")
	}
}
