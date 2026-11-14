package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The spend of the Inspiration on the rolls that are no combat attack or save (favor_spend.go): the check of a scene, a group check, a
// contest (the start and the answer), a Hide, the search for traps and the try for a puzzle's hint. The paths share the table below; the
// ones a combat holds for a window, and the ones that have no d20, have their tests apart.

// checkRolled is what a path answers of its d20, as the roller reads it.
type checkRolled struct {
	faces []int32
	// mode is "advantage", "disadvantage" or "normal".
	mode string
	// labels are the sentences of the circumstances behind the mode (the sources, or the notes of a check).
	labels []string
	// holdID is the question the roll asks (Bardic Inspiration), and heldByWindow says a reaction window holds the roll: no d20 yet.
	holdID       string
	heldByWindow bool
}

func (r checkRolled) hasFavor() bool { return slices.Contains(r.labels, "Inspiração") }

func rollModeName(m playv1.RollMode) string {
	switch m {
	case playv1.RollMode_ROLL_MODE_ADVANTAGE:
		return "advantage"
	case playv1.RollMode_ROLL_MODE_DISADVANTAGE:
		return "disadvantage"
	case playv1.RollMode_ROLL_MODE_UNSPECIFIED, playv1.RollMode_ROLL_MODE_NORMAL:
	}
	return "normal"
}

func rollKindName(m playv1.RollModeKind) string {
	switch m {
	case playv1.RollModeKind_ROLL_MODE_KIND_ADVANTAGE:
		return "advantage"
	case playv1.RollModeKind_ROLL_MODE_KIND_DISADVANTAGE:
		return "disadvantage"
	case playv1.RollModeKind_ROLL_MODE_KIND_UNSPECIFIED, playv1.RollModeKind_ROLL_MODE_KIND_NORMAL:
	}
	return "normal"
}

func sourceLabels(src []*playv1.AdvantageSource) []string {
	var out []string
	for _, s := range src {
		out = append(out, s.GetTextPt())
	}
	return out
}

func noteLabels(notes []*playv1.RollNote) []string {
	var out []string
	for _, n := range notes {
		out = append(out, n.GetLabelPt())
	}
	return out
}

// checkRolledOf reads a CheckRoll (a contest's, a Hide's, a group check's).
func checkRolledOf(r *playv1.CheckRoll, holdID string) checkRolled {
	return checkRolled{faces: r.GetFaces(), mode: rollKindName(r.GetMode()), labels: noteLabels(r.GetNotes()), holdID: holdID}
}

// checkEnv is one path of the table: the world it rolls in, who rolls and how the roll is read back.
type checkEnv struct {
	name string
	*favorParty
	who    *charactersv1.Character
	roller *user
	// send rolls the check in the app under the key, with or without the spend. The test queues the faces.
	send func(t *testing.T, key string, spend bool) (checkRolled, error)
	// typed rolls it with the faces typed.
	typed func(t *testing.T, key string, spend bool, faces ...int32) (checkRolled, error)
	// stored says the roll the path wrote says it was made with the Inspiration.
	stored func(t *testing.T) bool
	// advantage gives the roller's next check an advantage that is not the Inspiration.
	advantage func(t *testing.T)
	// die gives the roller's character a Bardic Inspiration die, so that the roll waits for the player's answer.
	die func(t *testing.T)
	// answered reads the roll an answer to a held roll wrote.
	answered func(*playv1.AnswerHeldRollResponse) checkRolled
	// fresh makes another world of the same path.
	fresh func() *checkEnv
	// fight is the combat of the path, nil for the paths of no combat.
	fight *cx
}

// favorPartyOf is the Inspiration's view of a table in an open session.
func favorPartyOf(t *testing.T, a *armed) *favorParty {
	t.Helper()
	open, err := a.h.svc.queries.GetOpenGameSession(t.Context(), a.campaignID)
	if err != nil {
		t.Fatalf("GetOpenGameSession() error = %v", err)
	}
	return &favorParty{armed: a, sessionID: open.ID}
}

// storedFlag reads the favor flag of the newest row the query finds (a JSON field or a column, as text).
func (f *favorParty) storedFlag(t *testing.T, query string, args ...any) bool {
	t.Helper()
	var got *string
	if err := f.h.pool.QueryRow(t.Context(), query, args...).Scan(&got); err != nil {
		t.Fatalf("read the favor flag (%s): %v", query, err)
	}
	return got != nil && (*got == "true" || *got == "t")
}

// outsideAdvantage puts an advantage on every ability check of a character out of a combat.
func (f *favorParty) outsideAdvantage(t *testing.T, characterID string) {
	t.Helper()
	if _, err := f.h.pool.Exec(t.Context(), `INSERT INTO character_effects (campaign_id, character_id, group_id, source_key, source_kind, duration_kind, modifiers)
		VALUES ($1, $2, gen_random_uuid(), 'spell:enhance-ability', 'spell', 'until_dismissed', $3::jsonb)`,
		f.campaignID, characterID, `[{"kind":"check_advantage","abilities":["str","dex","con","int","wis","cha"]}]`); err != nil {
		t.Fatalf("give the advantage: %v", err)
	}
}

// --- the paths ---

func newSceneCheckEnv(t *testing.T) *checkEnv {
	t.Helper()
	a := newArmed(t)
	f := favorPartyOf(t, a)
	point, actions := a.h.newScene(a.mapID, "Ponte", true, 5, sceneSpec{key: "skill:athletics", dc: new(int32(15))})
	a.openScene(t, point)
	roll := func(t *testing.T, key string, spend bool, edit func(*playv1.RollSceneCheckRequest)) (checkRolled, error) {
		t.Helper()
		req := &playv1.RollSceneCheckRequest{CampaignId: a.campaignID, ActionId: actions[0], IdempotencyKey: key, SpendFavor: spend}
		edit(req)
		res, err := a.caio.play.RollSceneCheck(t.Context(), connect.NewRequest(req))
		if err != nil {
			return checkRolled{}, err
		}
		return sceneRolledOf(res.Msg), nil
	}
	return &checkEnv{
		name: "a scene check", favorParty: f, who: a.toren, roller: a.caio,
		send: func(t *testing.T, key string, spend bool) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, func(r *playv1.RollSceneCheckRequest) {
				r.Roll = &playv1.RollSceneCheckRequest_RollInApp{RollInApp: true}
			})
		},
		typed: func(t *testing.T, key string, spend bool, faces ...int32) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, func(r *playv1.RollSceneCheckRequest) { r.D20Faces = faces })
		},
		stored: func(t *testing.T) bool {
			t.Helper()
			return f.storedFlag(t, `SELECT payload->>'favor' FROM session_events WHERE kind = 'scene_check_rolled' AND game_session_id = $1 ORDER BY seq DESC LIMIT 1`, f.sessionID)
		},
		advantage: func(t *testing.T) {
			t.Helper()
			f.outsideAdvantage(t, a.toren.GetId())
		},
		die: func(t *testing.T) {
			t.Helper()
			giveDieByRow(t, a.h, a.campaignID, a.toren.GetId())
		},
		answered: func(r *playv1.AnswerHeldRollResponse) checkRolled { return sceneRolledOf(r.GetSceneCheck()) },
	}
}

func sceneRolledOf(r *playv1.RollSceneCheckResponse) checkRolled {
	roll := r.GetRoll()
	return checkRolled{
		faces: roll.GetRoll().GetFaces(), mode: rollModeName(roll.GetMode()), labels: sourceLabels(roll.GetSources()), holdID: r.GetHeldRollOffer().GetHoldId(),
	}
}

func newGroupCheckEnv(t *testing.T) *checkEnv {
	t.Helper()
	a := newArmed(t)
	f := favorPartyOf(t, a)
	view := a.mustRequestGroup(t, func(r *playv1.RequestGroupCheckRequest) { r.Dc = 15 })
	roll := func(t *testing.T, key string, spend bool, in *playv1.CheckRollInput) (checkRolled, error) {
		t.Helper()
		res, err := a.caio.contests.RollGroupCheck(t.Context(), connect.NewRequest(&playv1.RollGroupCheckRequest{
			CampaignId: a.campaignID, IdempotencyKey: key, GroupCheckId: view.GetId(), Roll: in, SpendFavor: spend,
		}))
		if err != nil {
			return checkRolled{}, err
		}
		return groupRolledOf(res.Msg, a.toren.GetId()), nil
	}
	return &checkEnv{
		name: "a group check", favorParty: f, who: a.toren, roller: a.caio,
		send: func(t *testing.T, key string, spend bool) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, rollIn())
		},
		typed: func(t *testing.T, key string, spend bool, fs ...int32) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, faces(fs...))
		},
		stored: func(t *testing.T) bool {
			t.Helper()
			return f.storedFlag(t, `SELECT roll->>'favor' FROM group_check_members WHERE character_id = $1`, a.toren.GetId())
		},
		advantage: func(t *testing.T) {
			t.Helper()
			f.outsideAdvantage(t, a.toren.GetId())
		},
		die: func(t *testing.T) {
			t.Helper()
			giveDieByRow(t, a.h, a.campaignID, a.toren.GetId())
		},
		answered: func(r *playv1.AnswerHeldRollResponse) checkRolled {
			return groupRolledOf(r.GetGroupCheck(), a.toren.GetId())
		},
	}
}

func groupRolledOf(r *playv1.RollGroupCheckResponse, characterID string) checkRolled {
	if r.GetGroupCheck() == nil {
		return checkRolled{holdID: r.GetHeldRollOffer().GetHoldId()}
	}
	return checkRolledOf(memberOf(r.GetGroupCheck(), characterID).GetRoll(), "")
}

func newTrapSearchEnv(t *testing.T) *checkEnv {
	t.Helper()
	r := newTrapRig(t)
	f := favorPartyOf(t, r.armed)
	r.trap(t, "Fosso Dourado", 9, 7, func(s *mapsv1.TrapSpec) { s.NoticeDc, s.FindDc = 0, 14 })
	r.place(t, r.toren.GetId(), 8, 7)
	roll := func(t *testing.T, key string, spend bool, edit func(*playv1.SearchForTrapsRequest)) (checkRolled, error) {
		t.Helper()
		req := &playv1.SearchForTrapsRequest{CampaignId: r.campaignID, IdempotencyKey: key, Skill: investigation, SpendFavor: spend}
		edit(req)
		res, err := r.caio.play.SearchForTraps(t.Context(), connect.NewRequest(req))
		if err != nil {
			return checkRolled{}, err
		}
		return trapRolledOf(res.Msg), nil
	}
	return &checkEnv{
		name: "a trap search", favorParty: f, who: r.toren, roller: r.caio,
		send: func(t *testing.T, key string, spend bool) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, func(q *playv1.SearchForTrapsRequest) {
				q.Roll = &playv1.SearchForTrapsRequest_RollInApp{RollInApp: true}
			})
		},
		typed: func(t *testing.T, key string, spend bool, fs ...int32) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, func(q *playv1.SearchForTrapsRequest) { q.D20Faces = fs })
		},
		stored: func(t *testing.T) bool {
			t.Helper()
			return f.storedFlag(t, `SELECT payload->>'favor' FROM session_events WHERE kind = 'trap_searched' AND game_session_id = $1 ORDER BY seq DESC LIMIT 1`, f.sessionID)
		},
		advantage: func(t *testing.T) {
			t.Helper()
			r.giveTrapSearchEffect(t, r.toren.GetId(), "spell:enhance-ability", `[{"kind":"check_advantage","abilities":["int","wis"]}]`)
		},
		die: func(t *testing.T) {
			t.Helper()
			giveDieByRow(t, r.h, r.campaignID, r.toren.GetId())
		},
		answered: func(r *playv1.AnswerHeldRollResponse) checkRolled { return trapRolledOf(r.GetTrapSearch()) },
	}
}

func trapRolledOf(r *playv1.SearchForTrapsResponse) checkRolled {
	return checkRolled{
		faces: r.GetRoll().GetFaces(), mode: rollModeName(r.GetMode()), labels: sourceLabels(r.GetSources()), holdID: r.GetHeldRollOffer().GetHoldId(),
	}
}

func newHintTryEnv(t *testing.T) *checkEnv {
	t.Helper()
	p := newPuzzleTable(t)
	f := favorPartyOf(t, p.armed)
	puz := p.riddle(t, "A porta da Cripta pergunta", hintCheck("skill:investigation", 15))
	p.show(t, puz.GetId())
	roll := func(t *testing.T, key string, spend bool, edit func(*playv1.TryPuzzleHintRequest)) (checkRolled, error) {
		t.Helper()
		req := &playv1.TryPuzzleHintRequest{CampaignId: p.campaignID, PuzzleId: puz.GetId(), IdempotencyKey: key, SpendFavor: spend}
		edit(req)
		res, err := p.pc(p.caio).TryPuzzleHint(t.Context(), connect.NewRequest(req))
		if err != nil {
			return checkRolled{}, err
		}
		return hintRolledOf(res.Msg), nil
	}
	return &checkEnv{
		name: hintTryName, favorParty: f, who: p.toren, roller: p.caio,
		send: func(t *testing.T, key string, spend bool) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, func(r *playv1.TryPuzzleHintRequest) { r.Roll = &playv1.TryPuzzleHintRequest_RollInApp{RollInApp: true} })
		},
		typed: func(t *testing.T, key string, spend bool, fs ...int32) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, func(r *playv1.TryPuzzleHintRequest) { r.D20Faces = fs })
		},
		stored: func(t *testing.T) bool {
			t.Helper()
			return f.storedFlag(t, `SELECT (roll_mode = 'advantage')::text FROM puzzle_hint_tries ORDER BY created_at DESC LIMIT 1`)
		},
		advantage: func(t *testing.T) {
			t.Helper()
			f.outsideAdvantage(t, p.toren.GetId())
		},
		die: func(t *testing.T) {
			t.Helper()
			giveDieByRow(t, p.h, p.campaignID, p.toren.GetId())
		},
		answered: func(r *playv1.AnswerHeldRollResponse) checkRolled { return hintRolledOf(r.GetHintTry()) },
	}
}

func hintRolledOf(r *playv1.TryPuzzleHintResponse) checkRolled {
	return checkRolled{
		faces: r.GetRoll().GetFaces(), mode: rollModeName(r.GetMode()), labels: sourceLabels(r.GetSources()), holdID: r.GetHeldRollOffer().GetHoldId(),
	}
}

// contestFight is the fight of the casters with the helper of its contests and Hide.
func contestFight(t *testing.T) (*favorFight, *cx) {
	t.Helper()
	f := newFavorFight(t)
	return f, &cx{armed: f.armed, e: f.e}
}

func newContestStartEnv(t *testing.T) *checkEnv {
	t.Helper()
	f, c := contestFight(t)
	roll := func(t *testing.T, key string, spend bool, in *playv1.CheckRollInput) (checkRolled, error) {
		t.Helper()
		res, err := c.startContest(t, f.caio, func(r *playv1.StartContestRequest) {
			r.InitiatorId, r.TargetId, r.Purpose = c.id(t, "Toren"), c.id(t, "Goblin"), playv1.ContestPurpose_CONTEST_PURPOSE_GRAPPLE
			r.IdempotencyKey, r.SpendFavor, r.Roll = key, spend, in
		})
		if err != nil {
			return checkRolled{}, err
		}
		out := checkRolledOf(res.GetContest().GetInitiatorRoll(), res.GetHeldRollOffer().GetHoldId())
		out.heldByWindow = res.GetContest() == nil && res.GetHeldRollOffer() == nil
		return out, nil
	}
	return &checkEnv{
		name: "a contest start", favorParty: f.favorParty, who: f.toren, roller: f.caio, fight: c,
		send: func(t *testing.T, key string, spend bool) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, rollIn())
		},
		typed: func(t *testing.T, key string, spend bool, fs ...int32) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, faces(fs...))
		},
		stored: func(t *testing.T) bool {
			t.Helper()
			return f.storedFlag(t, `SELECT initiator_roll->>'favor' FROM combat_contests ORDER BY created_at DESC LIMIT 1`)
		},
		advantage: func(t *testing.T) {
			t.Helper()
			c.addAdvantage(t, "str")
		},
		die: func(t *testing.T) {
			t.Helper()
			f.giveCombatantDie(t, "Toren", f.e)
		},
		answered: func(r *playv1.AnswerHeldRollResponse) checkRolled {
			return checkRolledOf(r.GetContest().GetContest().GetInitiatorRoll(), "")
		},
	}
}

// addAdvantage gives Toren an advantage on the checks of the ability: Enhance Ability from Pensantus.
func (c *cx) addAdvantage(t *testing.T, ability string) {
	t.Helper()
	c.mustAddEffect(t, c.refresh(t), enhanceAbilityKey, []string{"Toren"}, func(r *playv1.AddLastingEffectRequest) {
		r.CasterId, r.AbilityKey = c.id(t, "Pensantus"), ability
	})
}

// goblinGrapple makes the Goblin (the master's) grapple Toren and returns the contest that waits for Toren's answer.
func (c *cx) goblinGrapple(t *testing.T) string {
	t.Helper()
	c.advance(t, "Goblin")
	res, err := c.grapple(t, c.master, "Goblin", "Toren", 10)
	if err != nil {
		t.Fatalf("the Goblin's grapple error = %v", err)
	}
	return res.GetContest().GetId()
}

func newContestAnswerEnv(t *testing.T) *checkEnv {
	t.Helper()
	f, c := contestFight(t)
	contestID := c.goblinGrapple(t)
	roll := func(t *testing.T, u *user, key string, spend, leave bool, in *playv1.CheckRollInput) (*playv1.RespondContestResponse, error) {
		t.Helper()
		res, err := u.contests.RespondContest(t.Context(), connect.NewRequest(&playv1.RespondContestRequest{
			CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: key, ContestId: contestID, Skill: playv1.ContestSkill_CONTEST_SKILL_ATHLETICS,
			Roll: in, SpendFavor: spend, DeferToMaster: leave,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	answer := func(t *testing.T, key string, spend bool, in *playv1.CheckRollInput) (checkRolled, error) {
		t.Helper()
		res, err := roll(t, f.caio, key, spend, false, in)
		if err != nil {
			return checkRolled{}, err
		}
		out := checkRolledOf(res.GetContest().GetDefenderRoll(), res.GetHeldRollOffer().GetHoldId())
		out.heldByWindow = res.GetContest() == nil && res.GetHeldRollOffer() == nil
		return out, nil
	}
	return &checkEnv{
		name: "a contest answer", favorParty: f.favorParty, who: f.toren, roller: f.caio, fight: c,
		send: func(t *testing.T, key string, spend bool) (checkRolled, error) {
			t.Helper()
			return answer(t, key, spend, rollIn())
		},
		typed: func(t *testing.T, key string, spend bool, fs ...int32) (checkRolled, error) {
			t.Helper()
			return answer(t, key, spend, faces(fs...))
		},
		stored: func(t *testing.T) bool {
			t.Helper()
			return f.storedFlag(t, `SELECT defender_roll->>'favor' FROM combat_contests ORDER BY created_at DESC LIMIT 1`)
		},
		advantage: func(t *testing.T) {
			t.Helper()
			c.addAdvantage(t, "str")
		},
		die: func(t *testing.T) {
			t.Helper()
			f.giveCombatantDie(t, "Toren", f.e)
		},
		answered: func(r *playv1.AnswerHeldRollResponse) checkRolled {
			return checkRolledOf(r.GetContestAnswer().GetContest().GetDefenderRoll(), "")
		},
	}
}

func newHideEnv(t *testing.T) *checkEnv {
	t.Helper()
	f, c := contestFight(t)
	roll := func(t *testing.T, key string, spend bool, in *playv1.CheckRollInput) (checkRolled, error) {
		t.Helper()
		res, err := f.caio.contests.Hide(t.Context(), connect.NewRequest(&playv1.HideRequest{
			CampaignId: c.campaignID, EncounterId: c.e.GetId(), IdempotencyKey: key, CombatantId: c.id(t, "Toren"), ActionKey: actionHide, Roll: in, SpendFavor: spend,
		}))
		if err != nil {
			return checkRolled{}, err
		}
		out := checkRolledOf(res.Msg.GetAttempt().GetRoll(), res.Msg.GetHeldRollOffer().GetHoldId())
		out.heldByWindow = res.Msg.GetAttempt() == nil && res.Msg.GetHeldRollOffer() == nil
		return out, nil
	}
	return &checkEnv{
		name: "a hide", favorParty: f.favorParty, who: f.toren, roller: f.caio, fight: c,
		send: func(t *testing.T, key string, spend bool) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, rollIn())
		},
		typed: func(t *testing.T, key string, spend bool, fs ...int32) (checkRolled, error) {
			t.Helper()
			return roll(t, key, spend, faces(fs...))
		},
		stored: func(t *testing.T) bool {
			t.Helper()
			return f.storedFlag(t, `SELECT roll->>'favor' FROM combat_hide_attempts ORDER BY created_at DESC LIMIT 1`)
		},
		advantage: func(t *testing.T) {
			t.Helper()
			c.addAdvantage(t, "dex")
		},
		die: func(t *testing.T) {
			t.Helper()
			f.giveCombatantDie(t, "Toren", f.e)
		},
		answered: func(r *playv1.AnswerHeldRollResponse) checkRolled {
			return checkRolledOf(r.GetHide().GetAttempt().GetRoll(), "")
		},
	}
}

var checkEnvs = []struct {
	name string
	mk   func(*testing.T) *checkEnv
}{
	{"a scene check", newSceneCheckEnv},
	{"a group check", newGroupCheckEnv},
	{"a contest start", newContestStartEnv},
	{"a contest answer", newContestAnswerEnv},
	{"a hide", newHideEnv},
	{"a trap search", newTrapSearchEnv},
	{"a hint try", newHintTryEnv},
}

// forEachCheck runs the test on a world of its own for each path of the table. e.fresh makes another world of the same path.
func forEachCheck(t *testing.T, test func(t *testing.T, e *checkEnv)) {
	t.Helper()
	for _, p := range checkEnvs {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			e := p.mk(t)
			e.fresh = func() *checkEnv { return p.mk(t) }
			test(t, e)
		})
	}
}

// A spend on a check that has no advantage gives two dice and an advantage, the source in what the roller reads, the flag in what the
// path wrote, and takes the row: one roll, one step of the session's number, never more than the same roll without it.
func TestFavorSpendOnACheck(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		e.give(t, e.who)
		e.h.roller.queue(4, 15)
		got, err := e.send(t, newKey(), true)
		if err != nil {
			t.Fatalf("%s with the Inspiration error = %v", e.name, err)
		}
		if len(got.faces) != 2 || got.mode != "advantage" || !got.hasFavor() {
			t.Errorf("the roll = %+v, want two d20, an advantage and the source Inspiração", got)
		}
		if e.favorHolds(t) || !e.stored(t) {
			t.Errorf("the Inspiration is still there = %v, the roll says it was spent = %v, want it gone and the roll to say so", e.favorHolds(t), e.stored(t))
		}
	})
}

// rollSteps is how many steps of the session's number a plain roll of each path gives. The combat paths (a contest, a Hide) write through
// Service.tx and step it once; the scene, group, trap and hint paths write in their own transaction and step it 0 times, so for them the
// check below reads 0 == 0 on both sides: the spend adds no step of its own, and there is none to add.
var rollSteps = map[string]int64{
	"a scene check": 1, "a group check": 1, "a trap search": 1, "a hint try": 1,
	"a contest start": 1, "a contest answer": 1, "a hide": 1,
}

// A spend moves the session's number exactly as much as the same roll without it, and adds no step of its own.
func TestFavorSpendMovesTheNumberLikeTheRoll(t *testing.T) {
	t.Parallel()
	forEachCheck(t, func(t *testing.T, e *checkEnv) {
		t.Helper()
		e.give(t, e.who)
		before := e.seq(t)
		e.h.roller.queue(4, 15)
		if _, err := e.send(t, newKey(), true); err != nil {
			t.Fatalf("%s with the Inspiration error = %v", e.name, err)
		}
		if e.favorHolds(t) {
			t.Fatal("the Inspiration was not spent")
		}
		spent := e.seq(t) - before
		other := e.fresh()
		other.h.roller.queue(4)
		beforeOther := other.seq(t)
		if _, err := other.send(t, newKey(), false); err != nil {
			t.Fatalf("%s without it error = %v", e.name, err)
		}
		plain := other.seq(t) - beforeOther
		want := rollSteps[e.name]
		if plain != want || spent != want {
			t.Errorf("%s moved the number %d times with the Inspiration and %d without it, want %d for both", e.name, spent, plain, want)
		}
	})
}

// give grants the Inspiration to the character, as the master.
func (e *checkEnv) give(t *testing.T, c *charactersv1.Character) {
	t.Helper()
	if err := e.grant(c.GetId()); err != nil {
		t.Fatalf("GrantFavor(%s) error = %v", c.GetName(), err)
	}
}
