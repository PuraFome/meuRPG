package play

import (
	"encoding/json"
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

// rollJSONOf is the roll a check of a combat wrote, as the table keeps it.
func (e *checkEnv) rollJSONOf(t *testing.T, path string) string {
	t.Helper()
	query := map[string]string{
		"a contest start":  `SELECT initiator_roll::text FROM combat_contests ORDER BY created_at DESC LIMIT 1`,
		"a contest answer": `SELECT defender_roll::text FROM combat_contests ORDER BY created_at DESC LIMIT 1`,
		"a hide":           `SELECT roll::text FROM combat_hide_attempts ORDER BY created_at DESC LIMIT 1`,
	}[path]
	var got string
	if err := e.h.pool.QueryRow(t.Context(), query).Scan(&got); err != nil {
		t.Fatalf("read the roll of %s: %v", path, err)
	}
	return got
}

// wantRolledWithoutIt checks the replay of a check wrote the roll with nothing spent: the hold is released and not dropped, the roll does not
// say it was made with the Inspiração, and its notes (which only the master and the roller read) say why, in the words given.
func (e *checkEnv) wantRolledWithoutIt(t *testing.T, path, note string, dice int) {
	t.Helper()
	var state string
	if err := e.h.pool.QueryRow(t.Context(), `SELECT state FROM reaction_holds ORDER BY created_at DESC LIMIT 1`).Scan(&state); err != nil {
		t.Fatalf("read the hold: %v", err)
	}
	if state != "released" {
		t.Errorf("the hold is %q, want it released and the check rolled", state)
	}
	var roll contestRoll
	raw := e.rollJSONOf(t, path)
	if err := json.Unmarshal([]byte(raw), &roll); err != nil {
		t.Fatalf("read the roll %s: %v", raw, err)
	}
	if e.stored(t) || !slices.Contains(roll.Notes, rollNote{Kind: noteFavorLost, Label: note}) {
		t.Errorf("the roll = %s (made with the Inspiração: %v), want it made without and a note %q of kind favor_lost", raw, e.stored(t), note)
	}
	if len(roll.Faces) != dice {
		t.Errorf("the roll = %s, want %d d20", raw, dice)
	}
}

// B3: an Inspiração gone when the replay comes does not drop the check: it rolls without it, nothing is spent, and the notes say why.
func TestFavorGoneAtTheReplayRollsTheCombatCheckWithoutIt(t *testing.T) {
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
			e.h.roller.queue(12)
			e.mustAnswer(t, e.master, e.fight.refresh(t), w.GetId(), passAnswer)
			e.wantRolledWithoutIt(t, p.name, "Inspiração não usada: não estava mais com você", 1)
			if e.favorHolds(t) {
				t.Error("the replay gave an Inspiration out of nothing")
			}
		})
	}
}

// B3: an Inspiração that adds nothing when the replay comes (an advantage of its own came meanwhile) stays with the player and the check
// rolls without it.
func TestFavorThatAddsNothingAtTheReplayRollsTheCombatCheckWithoutIt(t *testing.T) {
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
			e.advantage(t)
			w := e.windowHeld(t)
			e.h.roller.queue(3, 17)
			e.mustAnswer(t, e.master, e.fight.refresh(t), w.GetId(), passAnswer)
			e.wantRolledWithoutIt(t, p.name, "Inspiração guardada: não mudaria mais a rolagem", 2)
			if !e.favorHolds(t) {
				t.Error("the Inspiração was taken though the check was rolled without it")
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

// holdRow is the row of the Bardic die's question a replay kept under the request's key.
type holdRow struct {
	id                   string
	window, spent, asked bool
}

func (f *favorParty) holdRowOf(t *testing.T, key string) holdRow {
	t.Helper()
	var r holdRow
	if err := f.h.pool.QueryRow(t.Context(), `SELECT id, window_hold, favor_spent, answer_key IS NULL FROM inspiration_holds WHERE idempotency_key = $1`, key).Scan(&r.id, &r.window, &r.spent, &r.asked); err != nil {
		t.Fatalf("read the die's question kept under the key: %v", err)
	}
	return r
}

// A window's replay of a check that spends the Inspiration and then holds a Bardic die keeps the die's question with favor_spent. What it
// guards: the question's row remembers the spend (a replay that forgot it would count the roll as made without the Inspiration: the retry
// and the answer would spend the next one or leave it out), a client's retry of the key is answered as the first call was with nothing
// written and nothing spent, and the answer to the die counts the Inspiration once and spends no other.
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
			row := e.holdRowOf(t, key)
			if !row.spent || row.window || !row.asked {
				t.Fatalf("the replay's hold = %+v, want the die's question open, with the Inspiration spent and no window's mark (a check's hold is never one)", row)
			}
			e.give(t, e.who) // another one: neither the retry nor the answer may take it
			events, seq := e.eventsAll(t), e.seq(t)
			again, err := e.send(t, key, true)
			if err != nil || !again.heldByWindow || e.eventsAll(t) != events || e.seq(t) != seq || !e.favorHolds(t) {
				t.Errorf("the retry = %+v, %v (events %d -> %d, number %d -> %d, Inspiration there %v), want the first answer, nothing written and nothing spent", again, err, events, e.eventsAll(t), seq, e.seq(t), e.favorHolds(t))
			}
			// What the answer writes for a window-replayed contest or hide is the same with or without the spend (it writes no
			// contest row here, spend or not); this test pins what the spend adds: nothing more is taken by the answer.
			if _, err := e.answerHeldRoll(t, e.roller, row.id, dieAnswer(false)); err != nil {
				t.Fatalf("the answer to the die's question error = %v", err)
			}
			if !e.favorHolds(t) {
				t.Error("the answer to the die's question took the second Inspiração, want it untouched")
			}
		})
	}
}

// eventsAll counts the events of the campaign's sessions: a retry that wrote one is found.
func (f *favorParty) eventsAll(t *testing.T) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM session_events WHERE game_session_id IN (SELECT id FROM game_sessions WHERE campaign_id = $1)`)
}

// The same for the attacks: the replay of a weapon attack held by a window spends the Inspiration and holds the die's question under the
// window's mark, which is why a retry of the key does not find the question as its own (held_store.go byKeyOf: a window's hold is the
// replay's, not the request's). What it guards: the row keeps window_hold and favor_spent, the retry is answered as the first call
// was (the event of the key) with nothing written and nothing spent, and the answer to the die counts the Inspiration once.
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
	row := f.holdRowOf(t, key)
	if !row.spent || !row.window || !row.asked {
		t.Fatalf("the replay's hold = %+v, want the die's question open, with the Inspiration spent and the window's mark", row)
	}
	f.give(t, who) // another one: neither the retry nor the answer may take it
	events, seq := f.eventsAll(t), f.seq(t)
	again, err := p.send(t, f, u, key, true, nil)
	if err != nil || !again.heldByWindow || f.eventsAll(t) != events || f.seq(t) != seq || !f.favorHolds(t) {
		t.Errorf("the retry = %+v, %v (events %d -> %d, number %d -> %d, Inspiration there %v), want the first answer, nothing written and nothing spent", again, err, events, f.eventsAll(t), seq, f.seq(t), f.favorHolds(t))
	}
	out, err := f.answerHeldRoll(t, u, row.id, dieAnswer(false))
	if err != nil {
		t.Fatalf("the answer to the die's question error = %v", err)
	}
	if got := p.answered(out); len(got.faces) != 2 || got.mode != playv1.RollMode_ROLL_MODE_ADVANTAGE || !got.hasFavorSource() || f.eventFavor(t, p.kind) != "true" || !f.favorHolds(t) {
		t.Errorf("the answered roll = %+v (the roll says %q, the new Inspiração there: %v), want the advantage once with the source of the Inspiração, and the second Inspiração untouched", got, f.eventFavor(t, p.kind), f.favorHolds(t))
	}
}
