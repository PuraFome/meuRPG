package play

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	maplink "github.com/PuraFome/meuRPG/backend/internal/maps/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// The held NPC check after the first review: the keys of its answers, the errors of its answer, the rows a session leaves.

// TestNpcCheckLastAnswerUnderTheKeyOfAnotherEventIsAReusedKey: the last bard's answer settles the check under its own key, so a key that
// another event of the session already holds (the roll of an earlier check) is refused as every reused key is, and nothing is spent or
// settled; a cut under a key of its own then settles the check.
func TestNpcCheckLastAnswerUnderTheKeyOfAnotherEventIsAReusedKey(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	earlier := newKey()
	a, err := n.rollWith(t, 15, true, earlier)
	if err != nil || !a.GetHeld() {
		t.Fatalf("the first RollNpcCheck() = %v, %v; want a held check", a, err)
	}
	if _, err := n.skip(n.master, a.GetCheck().GetHoldId(), newKey()); err != nil {
		t.Fatalf("SkipNpcCheckAsks() error = %v", err)
	}
	b, err := n.roll(t, 12, true)
	if err != nil || !b.GetHeld() {
		t.Fatalf("the second RollNpcCheck() = %v, %v; want a held check", b, err)
	}
	hold, first := b.GetCheck().GetHoldId(), n.firstAsked(b)
	last := n.ana
	if first == n.ana {
		last = n.bia
	}
	if _, err := n.pass(first, hold, newKey()); err != nil {
		t.Fatalf("the first bard's pass error = %v", err)
	}
	before := n.uses(t, n.charOf(last))
	for name, a := range map[string]*playv1.CuttingWordsAnswer{"a pass": {Cut: false}, "a cut": typedCut(3)} {
		_, err := n.answerAs(last, hold, earlier, a)
		wantCode(t, "the last bard answers with the key of an earlier roll, "+name, err, connect.CodeInvalidArgument)
	}
	if got := n.uses(t, n.charOf(last)); got != before {
		t.Errorf("uses %d -> %d after refused answers, want none spent", before, got)
	}
	if row, ok := n.hold(t, hold); !ok || row.answered {
		t.Errorf("the hold = %+v, %v after refused answers; want it still waiting", row, ok)
	}
	if _, err := n.answerAs(last, hold, newKey(), typedCut(3)); err != nil {
		t.Fatalf("the last bard's cut under a key of its own error = %v", err)
	}
	if row, ok := n.hold(t, hold); !ok || !row.answered {
		t.Errorf("the hold = %+v, %v after the last answer; want it settled", row, ok)
	}
}

// TestNpcCheckAnswerWrapsTheErrorOfItsTransaction: a write of the answer's own transaction the database refuses (after the keep went
// well) reads as the store's error of every path, an internal error, not as the raw error of the database.
func TestNpcCheckAnswerWrapsTheErrorOfItsTransaction(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	first := n.firstAsked(res)
	hold := res.GetCheck().GetHoldId()
	name := "refuse_npc_answer_" + strings.ReplaceAll(newKey(), "-", "")
	run := func(ctx context.Context, sql string) {
		t.Helper()
		if _, err := n.h.pool.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// Only the answer's transaction writes an answer key on an ask of this check.
	run(t.Context(), `CREATE FUNCTION `+name+`() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	-- CockroachDB reads a field of NEW only inside parentheses.
	IF (NEW).answer_key IS NOT NULL AND (NEW).hold_id = '`+hold+`' THEN
		RAISE EXCEPTION 'the answer is refused';
	END IF;
	RETURN NEW;
END $$`)
	run(t.Context(), `CREATE TRIGGER `+name+` BEFORE UPDATE ON npc_check_asks FOR EACH ROW EXECUTE FUNCTION `+name+`()`)
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		run(ctx, `DROP TRIGGER `+name+` ON npc_check_asks`)
		run(ctx, `DROP FUNCTION `+name+`()`)
	})
	m := authz.Membership{CampaignID: n.campaignID, UserID: first.id, Role: authz.RolePlayer}
	req := &playv1.AnswerHeldRollRequest{
		CampaignId: n.campaignID, HoldId: hold, IdempotencyKey: newKey(),
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	}
	_, err = n.h.svc.answerNpcCheck(t.Context(), m, req, req.GetIdempotencyKey(), req.GetHoldId())
	wantCode(t, "the answer whose write the database refuses", err, connect.CodeInternal)
}

// TestNpcCheckRerunSettlesNothing: the rerun the registry holds for the kind refuses with an internal error and writes nothing: the check
// is still waiting, with no resolved event, after a path that reached it by mistake.
func TestNpcCheckRerunSettlesNothing(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil || !res.GetHeld() {
		t.Fatalf("RollNpcCheck() = %v, %v; want a held check", res, err)
	}
	hold := res.GetCheck().GetHoldId()
	def, ok := heldRegistry[heldNpcCheck]
	if !ok || def.rerun == nil {
		t.Fatal("the registry has no rerun for the NPC check")
	}
	out, err := def.rerun(t.Context(), n.h.svc, heldRoll{ID: hold, CampaignID: n.campaignID, Kind: heldNpcCheck}, newKey())
	wantCode(t, "the rerun of an NPC check", err, connect.CodeInternal)
	if out != nil {
		t.Errorf("the rerun answered %v, want nothing", out)
	}
	if row, ok := n.hold(t, hold); !ok || row.answered {
		t.Errorf("the hold after the rerun = %+v, %v; want it still waiting", row, ok)
	}
	if got := n.resolvedCount(t); got != 0 {
		t.Errorf("%d resolved events after the rerun, want none", got)
	}
}

// holdsOfTheCampaign counts the NPC check holds of the table and the asks of the holds given.
func (n *npcTable) holdsOfTheCampaign(t *testing.T, holds ...string) (rows, asks int) {
	t.Helper()
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM inspiration_holds WHERE campaign_id = $1 AND kind = 'npc_check'`, n.campaignID).Scan(&rows); err != nil {
		t.Fatalf("count the holds: %v", err)
	}
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM npc_check_asks WHERE hold_id = ANY($1::UUID[])`, holds).Scan(&asks); err != nil {
		t.Fatalf("count the asks: %v", err)
	}
	return rows, asks
}

// TestNpcCheckSettledChecksStayUntilTheSessionEndsAndGoWithIt: a settled check keeps its hold and its asks while the session is open (the
// master reads it again with GetNpcCheck), whichever way it settled (the bards' answers, the skip, the next roll); the session that ends
// deletes every NPC check hold of the campaign, the settled ones too, and their asks go with them.
func TestNpcCheckSettledChecksStayUntilTheSessionEndsAndGoWithIt(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	answered, err := n.roll(t, 15, true) // settled by the answers of both bards
	if err != nil || !answered.GetHeld() {
		t.Fatalf("the first RollNpcCheck() = %v, %v; want a held check", answered, err)
	}
	first, second := n.firstAsked(answered), n.bia
	if first == n.bia {
		second = n.ana
	}
	for _, u := range []*user{first, second} {
		if _, err := n.pass(u, answered.GetCheck().GetHoldId(), newKey()); err != nil {
			t.Fatalf("a bard's pass error = %v", err)
		}
	}
	skipped, err := n.roll(t, 14, true) // settled by the master's skip
	if err != nil || !skipped.GetHeld() {
		t.Fatalf("the second RollNpcCheck() = %v, %v; want a held check", skipped, err)
	}
	if _, err := n.skip(n.master, skipped.GetCheck().GetHoldId(), newKey()); err != nil {
		t.Fatalf("SkipNpcCheckAsks() error = %v", err)
	}
	next, err := n.roll(t, 13, true) // settled by the roll that follows it
	if err != nil || !next.GetHeld() {
		t.Fatalf("the third RollNpcCheck() = %v, %v; want a held check", next, err)
	}
	last, err := n.roll(t, 12, true)
	if err != nil || !last.GetHeld() {
		t.Fatalf("the fourth RollNpcCheck() = %v, %v; want a held check", last, err)
	}
	holds := []string{answered.GetCheck().GetHoldId(), skipped.GetCheck().GetHoldId(), next.GetCheck().GetHoldId()}
	for _, id := range holds {
		if row, ok := n.hold(t, id); !ok || !row.answered {
			t.Fatalf("the hold %s = %+v, %v before the session ends; want it kept, settled", id, row, ok)
		}
	}
	if rows, asks := n.holdsOfTheCampaign(t, holds...); rows != 4 || asks != 6 {
		t.Fatalf("before the end: %d holds and %d asks of the three settled checks, want 4 and 6", rows, asks)
	}
	n.master.end(t, n.session(t))
	if rows, asks := n.holdsOfTheCampaign(t, append(holds, last.GetCheck().GetHoldId())...); rows != 0 || asks != 0 {
		t.Errorf("after the end: %d holds and %d asks, want none", rows, asks)
	}
}

// TestNpcCheckAsksOnlyTheBardsOfThePlayersOfTheScene: with no token pair to measure, the master's box says the bards see the creature, and
// the bard's player must also be one the open scene is open to. A bard whose player is outside the scene has no ask, hears nothing of
// the check and reads no offer, whether the creature is on the stage or not; with the scene open to everyone both bards are asked.
// Brisa (Bia's) is asked before Lia (Ana's): the first asked hears the check and reads the offer.
func TestNpcCheckAsksOnlyTheBardsOfThePlayersOfTheScene(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		to    func(n *npcTable) []*user // nil: the scene is open to the whole table
		stage bool
		want  []string
	}{
		"open to everyone":              {nil, false, []string{"Brisa", "Lia"}},
		"open to Ana, off the stage":    {func(n *npcTable) []*user { return []*user{n.ana} }, false, []string{"Lia"}},
		"open to Ana, on the stage":     {func(n *npcTable) []*user { return []*user{n.ana} }, true, []string{"Lia"}},
		"open to Bia, off the stage":    {func(n *npcTable) []*user { return []*user{n.bia} }, false, []string{"Brisa"}},
		"open to Caio, off the stage":   {func(n *npcTable) []*user { return []*user{n.caio} }, false, nil},
		"open to Ana and Bia, on stage": {func(n *npcTable) []*user { return []*user{n.ana, n.bia} }, true, []string{"Brisa", "Lia"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			point, _ := n.h.newScene(n.h.newMap(n.campaignID, gridColumns), "A clareira", true, 1)
			if tc.to == nil {
				n.openScene(t, point)
			} else {
				n.openFor(t, point, tc.to(n)...)
			}
			if tc.stage {
				if _, err := n.master.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: n.campaignID, CharacterId: n.npcID})); err != nil {
					t.Fatalf("PutOnStage() error = %v", err)
				}
			}
			bards := map[string]*user{"Brisa": n.bia, "Lia": n.ana}
			listeners := map[string]heardBy{}
			for name, u := range bards {
				listeners[name] = listen(t, n.h, n.campaignID, u.id, false)
			}
			res, err := n.roll(t, 15, true)
			if err != nil {
				t.Fatalf("RollNpcCheck() error = %v", err)
			}
			var asked []string
			for _, a := range res.GetCheck().GetAsks() {
				asked = append(asked, a.GetCharacterName())
			}
			if res.GetHeld() != (len(tc.want) > 0) || !slices.Equal(asked, tc.want) {
				t.Fatalf("the bards asked = %v (held %v), want %v", asked, res.GetHeld(), tc.want)
			}
			if res.GetHeld() {
				if rows := n.asksOf(t, res.GetCheck().GetHoldId()); rows != len(tc.want) {
					t.Errorf("the check has %d ask rows, want %d", rows, len(tc.want))
				}
			}
			for name, u := range bards {
				first := len(tc.want) > 0 && tc.want[0] == name // only the bard asked now hears the check and reads its offer
				heard := false
				for _, ev := range listeners[name].heard(t, n.h) {
					heard = heard || ev.GetHeldRollsChanged() != nil
				}
				if heard != first {
					t.Errorf("%s's player heard the held rolls change = %v, want %v", name, heard, first)
				}
				if got := n.listHeldRolls(t, u); len(got) != map[bool]int{true: 1}[first] {
					t.Errorf("%s's player reads %d offers, want %v", name, len(got), map[bool]int{true: 1}[first])
				}
			}
		})
	}
}

// asksOf counts the ask rows of a hold.
func (n *npcTable) asksOf(t *testing.T, hold string) int {
	t.Helper()
	var rows int
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM npc_check_asks WHERE hold_id = $1`, hold).Scan(&rows); err != nil {
		t.Fatalf("count the asks: %v", err)
	}
	return rows
}

// rollMonster is roll for an SRD creature.
func (n *npcTable) rollMonster(t *testing.T, key string, face int32, sees bool) (*playv1.RollNpcCheckResponse, error) {
	t.Helper()
	res, err := n.master.play.RollNpcCheck(t.Context(), connect.NewRequest(&playv1.RollNpcCheckRequest{
		CampaignId: n.campaignID, Npc: &playv1.RollNpcCheckRequest_MonsterKey{MonsterKey: key}, CheckKey: deception,
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: face}, BardSees: sees, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// TestNpcCheckSaysWhyNobodyWasAsked: a check that settles at once tells the master why, the first reason that applies: no bard can cut a
// check at all (the setting is not "todos"); the creature is immune to being charmed; its token is hidden on the map; nobody sees it. The
// roll answers with it, GetNpcCheck reads it again as the roll stored it, and a check that asked a bard has none.
func TestNpcCheckSaysWhyNobodyWasAsked(t *testing.T) {
	t.Parallel()
	const (
		noBard  = playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_NO_BARD_CAN_CUT
		charmed = playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_CHARM_IMMUNE
		hidden  = playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_TOKEN_HIDDEN
		blind   = playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_NOBODY_SEES
		none    = playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_UNSPECIFIED
	)
	all, attacks := charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ONLY_ATTACKS
	for name, tc := range map[string]struct {
		ask     charactersv1.CuttingWordsAsk
		monster string
		hideTok bool
		sees    bool
		want    playv1.NpcCheckNobodyReason
	}{
		"the setting is not todos":               {attacks, "", false, true, noBard},
		"immune to being charmed":                {all, "monster:black-pudding", false, true, charmed},
		"the token is hidden":                    {all, "", true, true, hidden},
		"nobody sees it":                         {all, "", false, false, blind},
		"no bard and immune: the bard first":     {attacks, "monster:black-pudding", false, true, noBard},
		"a bard is asked: no reason":             {all, "", false, true, none},
		"a creature that is not immune is asked": {all, "monster:wolf", false, true, none},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, tc.ask)
			if tc.hideTok {
				mapID := n.mapWith(t, gridColumns)
				n.h.placeToken(mapID, n.npcID, 1750, 250)
				n.hideToken(t, mapID, n.npcID)
			}
			var res *playv1.RollNpcCheckResponse
			var err error
			if tc.monster != "" {
				res, err = n.rollMonster(t, tc.monster, 15, tc.sees)
			} else {
				res, err = n.roll(t, 15, tc.sees)
			}
			if err != nil {
				t.Fatalf("RollNpcCheck() error = %v", err)
			}
			if got := res.GetCheck().GetNobodyReason(); got != tc.want || res.GetHeld() != (tc.want == none) {
				t.Errorf("the roll says %v (held %v), want %v", got, res.GetHeld(), tc.want)
			}
			if got := n.mustLatest(t); got.GetNobodyReason() != tc.want || !proto.Equal(got, res.GetCheck()) && tc.want != none {
				t.Errorf("GetNpcCheck() says %v, want %v and the roll's own read", got.GetNobodyReason(), tc.want)
			}
		})
	}
}

// blindTo is the sight of a fog map where the players named see no square.
type blindTo struct {
	maplink.CombatSight
	blind map[string]bool
}

func (b blindTo) Sees(userID string, _ grid.Square) bool { return !b.blind[userID] }

// fogOf is a fog source that gives every map the sight given.
type fogOf struct {
	FogSource
	sight maplink.CombatSight
}

func (f fogOf) CombatSight(context.Context, string, string) (maplink.CombatSight, error) {
	return f.sight, nil
}

// TestNpcCheckOnAFogMapAsksOnlyTheBardsWhosePlayerSeesTheSquare: both bards stand within 60 feet of the creature on a map with fog, and the
// master's box says nothing the map does not (it is ticked, and is ignored): the bard whose player does not see the creature's square
// is not asked (no ask row, no hint, no offer); when neither player sees it, nobody is, and the master reads that nobody sees it.
func TestNpcCheckOnAFogMapAsksOnlyTheBardsWhosePlayerSeesTheSquare(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		blind []string // the users that do not see the square
		want  []string
	}{
		"both players see it":     {nil, []string{"Brisa", "Lia"}},
		"Ana's player does not":   {[]string{"ana"}, []string{"Brisa"}},
		"Bia's player does not":   {[]string{"bia"}, []string{"Lia"}},
		"neither of them sees it": {[]string{"ana", "bia"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			mapID := n.mapWith(t, gridColumns)
			n.h.placeToken(mapID, n.bri.GetId(), 250, 250)
			n.h.placeToken(mapID, n.lia.GetId(), 250, 750)
			n.h.placeToken(mapID, n.npcID, 3250, 250)
			users := map[string]*user{"ana": n.ana, "bia": n.bia}
			blind := map[string]bool{}
			for _, who := range tc.blind {
				blind[users[who].id] = true
			}
			n.h.svc.fog = fogOf{FogSource: n.h.svc.fog, sight: blindTo{blind: blind}}
			listeners := map[string]heardBy{"Brisa": listen(t, n.h, n.campaignID, n.bia.id, false), "Lia": listen(t, n.h, n.campaignID, n.ana.id, false)}
			res, err := n.roll(t, 15, true)
			if err != nil {
				t.Fatalf("RollNpcCheck() error = %v", err)
			}
			var asked []string
			for _, a := range res.GetCheck().GetAsks() {
				asked = append(asked, a.GetCharacterName())
			}
			if !slices.Equal(asked, tc.want) || res.GetHeld() != (len(tc.want) > 0) {
				t.Fatalf("the bards asked = %v (held %v), want %v", asked, res.GetHeld(), tc.want)
			}
			if len(tc.want) == 0 {
				if got := res.GetCheck().GetNobodyReason(); got != playv1.NpcCheckNobodyReason_NPC_CHECK_NOBODY_REASON_NOBODY_SEES {
					t.Errorf("the reason nobody was asked = %v, want nobody sees it", got)
				}
			}
			for name, u := range map[string]*user{"Brisa": n.bia, "Lia": n.ana} {
				first := len(tc.want) > 0 && tc.want[0] == name
				if n.changed(t, listeners[name]) != first || len(n.listHeldRolls(t, u)) != map[bool]int{true: 1}[first] {
					t.Errorf("%s's player is told and offered the question = %v, want %v: only the bard asked first is", name, !first, first)
				}
			}
		})
	}
}

// TestNpcCheckTwoAnswersOfOneBardUnderTwoKeysSpendOnce: the bard's cut is sent twice at once, each under a key of its own: one is
// taken, the other finds the question answered (failed_precondition), and exactly one use is spent and one die comes off the total.
func TestNpcCheckTwoAnswersOfOneBardUnderTwoKeysSpendOnce(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	hold, first, second := n.twoAsked(t)
	before := n.uses(t, n.charOf(first))
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, face := range []int32{3, 5} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = n.answerAs(first, hold, newKey(), typedCut(face))
		}()
	}
	wg.Wait()
	won, lost := 0, 0
	for _, err := range errs {
		switch connect.CodeOf(err) {
		case connect.CodeFailedPrecondition:
			lost++
		default:
			if err == nil {
				won++
			}
		}
	}
	if won != 1 || lost != 1 {
		t.Fatalf("the two answers = %v, %v; want one taken and one told the question was answered", errs[0], errs[1])
	}
	if after := n.uses(t, n.charOf(first)); after != before-1 {
		t.Errorf("uses %d -> %d, want exactly one spent", before, after)
	}
	if _, err := n.pass(second, hold, newKey()); err != nil {
		t.Fatalf("the second bard's pass error = %v", err)
	}
	if got := n.mustLatest(t); !got.GetSettled() || got.GetCut() != got.GetAsks()[0].GetFace() || got.GetCut() == 0 {
		t.Errorf("the settled check = %v, want the one die that was taken off, and only that", got)
	}
}

// TestNpcCheckAnswerAfterTheSceneClosedStillWorks: the check belongs to the session and was judged when it was rolled (the bard's player
// was in the scene then), so closing the scene takes nothing from the question: the bard still reads it, with the name it was asked
// with, and answers it.
func TestNpcCheckAnswerAfterTheSceneClosedStillWorks(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	point, _ := n.h.newScene(n.h.newMap(n.campaignID, gridColumns), "A clareira", true, 1)
	n.openFor(t, point, n.ana)
	if _, err := n.master.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: n.campaignID, CharacterId: n.npcID})); err != nil {
		t.Fatalf("PutOnStage() error = %v", err)
	}
	res, err := n.roll(t, 15, true)
	if err != nil || !res.GetHeld() || len(res.GetCheck().GetAsks()) != 1 {
		t.Fatalf("RollNpcCheck() = %v, %v; want one bard asked (Ana's)", res, err)
	}
	if _, err := n.master.play.CloseScene(t.Context(), connect.NewRequest(&playv1.CloseSceneRequest{CampaignId: n.campaignID})); err != nil {
		t.Fatalf("CloseScene() error = %v", err)
	}
	if got := n.listHeldRolls(t, n.ana); len(got) != 1 {
		t.Fatalf("the bard reads %d offers after the scene closed, want its question", len(got))
	}
	if got, err := n.answer(t, n.ana, res.GetCheck().GetHoldId(), true, 4); err != nil || !got.GetCut() {
		t.Fatalf("the bard's cut after the scene closed = %v, %v; want it taken", got, err)
	}
	if got := n.mustLatest(t); !got.GetSettled() || got.GetTotal() != 11 || got.GetCut() != 4 {
		t.Errorf("the check = %v, want it settled at 11", got)
	}
}

// TestNpcCheckOfADeletedCreatureIsNotSettled: the creature's character is deleted (the master's account goes, say) while its check waits
// after a cut. The hold goes with it (inspiration_holds.character_id cascades) and so do its asks: nobody is asked any more, the cut
// stays spent (it was the bard's die, rolled for that check), and the master reads the check as neither held nor settled: its total is not
// an outcome, with no way to skip what is gone; the next check the master rolls goes on as usual.
func TestNpcCheckOfADeletedCreatureIsNotSettled(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	hold, first, second := n.twoAsked(t)
	before := n.uses(t, n.charOf(first))
	if _, err := n.answerAs(first, hold, newKey(), typedCut(4)); err != nil {
		t.Fatalf("the first bard's cut error = %v", err)
	}
	if _, err := n.h.pool.Exec(t.Context(), `DELETE FROM characters WHERE id = $1`, n.npcID); err != nil {
		t.Fatalf("delete the creature: %v", err)
	}
	if after := n.uses(t, n.charOf(first)); after != before-1 {
		t.Errorf("the first bard's uses %d -> %d, want the cut spent and kept", before, after)
	}
	if got := n.listHeldRolls(t, second); len(got) != 0 {
		t.Errorf("the next bard reads %v, want nothing: the check is gone", got)
	}
	_, err := n.pass(second, hold, newKey())
	wantCode(t, "the next bard's answer to the check of a deleted creature", err, connect.CodeNotFound)
	_, err = n.skip(n.master, hold, newKey())
	wantCode(t, "the master's skip of the check of a deleted creature", err, connect.CodeNotFound)
	if got := n.mustLatest(t); got.GetSettled() || got.GetHoldId() != hold || got.GetD20().GetFaces()[0] != 15 {
		t.Errorf("GetNpcCheck() = %v, want the check as it was rolled, not settled", got)
	}
	if got := n.resolvedCount(t); got != 0 {
		t.Errorf("%d resolved events, want none: nothing settled it", got)
	}
}
