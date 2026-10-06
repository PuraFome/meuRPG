package characters

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dbtest"
	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
)

// The table's rules outside combat (MR-025, RN-24): the hit points of a level-up
// and the ability scores of a new sheet. The rules are saved through the real
// CampaignService and read through the real ContentSource wiring (probeSource
// reads them with campaigns.Service, in the caller's transaction).

// setRules saves the table's rules as master: the defaults, changed by change.
func setRules(t *testing.T, master *user, campaignID string, change func(*campaignsv1.TableRules)) {
	t.Helper()
	got, err := master.campaigns.GetTableRules(t.Context(), connect.NewRequest(&campaignsv1.GetTableRulesRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("GetTableRules() error = %v", err)
	}
	r := proto.CloneOf(got.Msg.GetRules())
	change(r)
	if _, err := master.campaigns.SetTableRules(t.Context(), connect.NewRequest(&campaignsv1.SetTableRulesRequest{CampaignId: campaignID, Rules: r})); err != nil {
		t.Fatalf("SetTableRules() error = %v", err)
	}
}

// methods allows only the listed ways of making ability scores.
func methods(std, buy, roll, typed bool) func(*campaignsv1.TableRules) {
	return func(r *campaignsv1.TableRules) {
		r.AbilityMethods = &campaignsv1.AbilityMethods{StandardArray: std, PointBuy: buy, Rolled_4D6: roll, Typed: typed}
	}
}

func hitPointsRule(rule campaignsv1.HitPointsRule) func(*campaignsv1.TableRules) {
	return func(r *campaignsv1.TableRules) { r.HitPoints = rule }
}

// TestRN24_TheLevelUpFollowsTheHitPointsRule: with "roll" the average is refused
// and the roll goes through; with "average" every roll is refused and the average
// goes through; GetLevelUpOptions says which methods are allowed.
func TestRN24_TheLevelUpFollowsTheHitPointsRule(t *testing.T) {
	t.Parallel()
	const (
		avg  = charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_AVERAGE
		app  = charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_IN_APP
		phys = charactersv1.LevelUpHitPointsMethod_LEVEL_UP_HIT_POINTS_METHOD_ROLLED_PHYSICAL
	)
	withMethod := func(m charactersv1.LevelUpHitPointsMethod, value int32) *charactersv1.LevelUpChoices {
		c := pensantusLevelUp()
		c.HitPoints = &charactersv1.LevelUpHitPoints{Method: m, Value: value}
		return c
	}
	wantRule := func(t *testing.T, call string, err error) {
		t.Helper()
		r := refusal(t, call, err)
		if r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS_RULE || r.GetField() != "choices.hit_points.method" {
			t.Errorf("%s refusal = %v, want HIT_POINTS_RULE on choices.hit_points.method", call, r)
		}
	}

	t.Run("the default lets the player choose", func(t *testing.T) {
		tb := newLevelUpTable(t, 2700, 5)
		o, err := tb.options(tb.owner, tb.pc)
		if err != nil || o.GetHitPointsRule() != charactersv1.LevelUpHitPointsRule_LEVEL_UP_HIT_POINTS_RULE_PLAYER_CHOOSES {
			t.Fatalf("options = %v, %v; want PLAYER_CHOOSES", o.GetHitPointsRule(), err)
		}
		if _, err := tb.roll(tb.owner, tb.pc); err != nil {
			t.Errorf("RollLevelUpHitPoints() error = %v", err)
		}
	})

	t.Run("roll", func(t *testing.T) {
		tb := newLevelUpTable(t, 2700, 5)
		setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_ROLL))
		o, err := tb.options(tb.owner, tb.pc)
		if err != nil || o.GetHitPointsRule() != charactersv1.LevelUpHitPointsRule_LEVEL_UP_HIT_POINTS_RULE_ROLL_ONLY {
			t.Fatalf("options = %v, %v; want ROLL_ONLY", o.GetHitPointsRule(), err)
		}
		_, err = tb.levelUp(tb.owner, tb.pc, withMethod(avg, 0))
		wantRule(t, "LevelUpCharacter(average)", err)
		p, err := tb.preview(tb.owner, tb.pc, withMethod(avg, 0))
		if err != nil || p.GetRefusal().GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS_RULE {
			t.Errorf("PreviewLevelUp(average) refusal = %v, %v; want HIT_POINTS_RULE", p.GetRefusal(), err)
		}
		// A preview that has not chosen yet is not refused.
		undecided := pensantusLevelUp()
		undecided.HitPoints = nil
		p, err = tb.preview(tb.owner, tb.pc, undecided)
		if err != nil || p.GetRefusal() != nil {
			t.Errorf("PreviewLevelUp(no method) refusal = %v, %v; want none", p.GetRefusal(), err)
		}
		if r, err := tb.roll(tb.owner, tb.pc); err != nil || r.GetValue() != 5 {
			t.Fatalf("RollLevelUpHitPoints() = %v, %v; want 5", r, err)
		}
		if _, err := tb.levelUp(tb.owner, tb.pc, withMethod(app, 0)); err != nil {
			t.Errorf("LevelUpCharacter(rolled in the app) error = %v", err)
		}
	})

	t.Run("roll, with a physical die", func(t *testing.T) {
		tb := newLevelUpTable(t, 2700)
		setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_ROLL))
		if _, err := tb.levelUp(tb.owner, tb.pc, withMethod(phys, 3)); err != nil {
			t.Errorf("LevelUpCharacter(typed die) error = %v", err)
		}
	})

	t.Run("average", func(t *testing.T) {
		tb := newLevelUpTable(t, 2700)
		setRules(t, tb.master, tb.campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_AVERAGE))
		o, err := tb.options(tb.owner, tb.pc)
		if err != nil || o.GetHitPointsRule() != charactersv1.LevelUpHitPointsRule_LEVEL_UP_HIT_POINTS_RULE_AVERAGE_ONLY {
			t.Fatalf("options = %v, %v; want AVERAGE_ONLY", o.GetHitPointsRule(), err)
		}
		_, err = tb.roll(tb.owner, tb.pc)
		if b := blocked(t, "RollLevelUpHitPoints()", err); b.GetReason() != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_HIT_POINTS_AVERAGE_ONLY {
			t.Errorf("RollLevelUpHitPoints() reason = %v, want HIT_POINTS_AVERAGE_ONLY", b.GetReason())
		}
		_, err = tb.levelUp(tb.owner, tb.pc, withMethod(app, 0))
		wantRule(t, "LevelUpCharacter(rolled in the app)", err)
		_, err = tb.levelUp(tb.owner, tb.pc, withMethod(phys, 4))
		wantRule(t, "LevelUpCharacter(typed die)", err)
		if _, err := tb.levelUp(tb.owner, tb.pc, withMethod(avg, 0)); err != nil {
			t.Errorf("LevelUpCharacter(average) error = %v", err)
		}
	})
}

// scoresSheet is Pensantus with other base scores, in the order of the sheet.
func scoresSheet(scores ...int32) *charactersv1.CharacterSheet {
	s := pensantusSheet()
	s.GetFull().BaseScores = &rulesv1.AbilityScores{
		Strength: scores[0], Dexterity: scores[1], Constitution: scores[2], Intelligence: scores[3], Wisdom: scores[4], Charisma: scores[5],
	}
	return s
}

func (u *user) createWith(campaignID string, method charactersv1.AbilityMethod, sheet *charactersv1.CharacterSheet) (*charactersv1.Character, error) {
	res, err := u.api.CreateCharacter(context.Background(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Pensantus", Sheet: sheet, AbilityMethod: method,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCharacter(), nil
}

func abilityRefusal(t *testing.T, call string, err error) *charactersv1.AbilityScoresRefusal {
	t.Helper()
	wantCode(t, call, err, connect.CodeFailedPrecondition)
	ce, _ := errors.AsType[*connect.Error](err)
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if r, ok := v.(*charactersv1.AbilityScoresRefusal); ok {
				return r
			}
		}
	}
	t.Fatalf("%s error %v has no AbilityScoresRefusal detail", call, err)
	return nil
}

func (u *user) rollScores(campaignID string, typed ...[]int32) (*charactersv1.RollAbilityScoresResponse, error) {
	req := &charactersv1.RollAbilityScoresRequest{CampaignId: campaignID}
	for _, d := range typed {
		req.TypedDice = append(req.TypedDice, &charactersv1.AbilityRollSet{Dice: d})
	}
	res, err := u.api.RollAbilityScores(context.Background(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (u *user) storedScores(t *testing.T, campaignID string) *charactersv1.AbilityRolls {
	t.Helper()
	res, err := u.api.GetAbilityRolls(t.Context(), connect.NewRequest(&charactersv1.GetAbilityRollsRequest{CampaignId: campaignID}))
	if err != nil {
		t.Fatalf("GetAbilityRolls() error = %v", err)
	}
	return res.Msg.GetRolls()
}

func totalsOf(r *charactersv1.AbilityRolls) []int32 {
	var out []int32
	for _, s := range r.GetSets() {
		out = append(out, s.GetTotal())
	}
	return out
}

const (
	mStd   = charactersv1.AbilityMethod_ABILITY_METHOD_STANDARD_ARRAY
	mBuy   = charactersv1.AbilityMethod_ABILITY_METHOD_POINT_BUY
	mRoll  = charactersv1.AbilityMethod_ABILITY_METHOD_ROLLED_4D6
	mTyped = charactersv1.AbilityMethod_ABILITY_METHOD_TYPED
)

// The dice of the E10-03 drawing: 16, 14, 13, 12, 10 and 8.
var drawingDice = []int{6, 5, 5, 2, 5, 5, 4, 1, 5, 4, 4, 3, 4, 4, 4, 2, 4, 3, 3, 2, 3, 3, 2, 1}

// abilityTable is a campaign with a master and four players, and a server whose
// dice give the drawing's sets first and six 18s next.
func newAbilityTable(t *testing.T) (*harness, *user, []*user, string, *testDiceRules) {
	t.Helper()
	sixes := slices.Repeat([]int{6}, 24)
	faces := slices.Concat(drawingDice, sixes, sixes) // three batches of six sets
	rule := &testDiceRules{}
	rule.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_PLAYER_CHOOSES))
	h := newHarnessWith(t, func(c *Config) { c.Dice, c.Roller = rule, &dice.Fixed{Faces: faces} })
	master := h.newUser("Samuel")
	players := []*user{h.newUser("Ana"), h.newUser("Bia"), h.newUser("Caio"), h.newUser("Duda")}
	return h, master, players, h.newCampaign(master, "Mirathel", players...), rule
}

// TestRN24_TheScoresFollowTheMethod: each way of making the scores, accepted and
// refused; the way a table switched off; a client that does not say its way.
func TestRN24_TheScoresFollowTheMethod(t *testing.T) {
	t.Parallel()
	h, master, players, campaign, _ := newAbilityTable(t)
	_ = h
	ana, bia, caio, duda := players[0], players[1], players[2], players[3]
	type refused = charactersv1.AbilityScoresRefusalReason
	const (
		notStd  = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_NOT_STANDARD_ARRAY
		badBuy  = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_BAD_POINT_BUY
		noRolls = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_NO_ROLLS_STORED
		notRoll = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_NOT_THE_ROLLS
		range3  = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_TYPED_OUT_OF_RANGE
		notOK   = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_METHOD_NOT_ALLOWED
	)
	wantRefusal := func(call string, err error, reason refused, method charactersv1.AbilityMethod) {
		t.Helper()
		if r := abilityRefusal(t, call, err); r.GetReason() != reason || r.GetMethod() != method {
			t.Errorf("%s refusal = %v, want %v for %v", call, r, reason, method)
		}
	}

	// The standard array: a permutation is accepted, anything else is refused.
	_, err := ana.createWith(campaign, mStd, scoresSheet(15, 15, 13, 12, 10, 8))
	wantRefusal("the array with a 15 twice", err, notStd, mStd)
	_, err = ana.createWith(campaign, mStd, scoresSheet(16, 14, 13, 12, 10, 8))
	wantRefusal("the array with a 16", err, notStd, mStd)
	if c, err := ana.createWith(campaign, mStd, scoresSheet(8, 10, 12, 13, 14, 15)); err != nil || c.GetSheet().GetFull().GetBaseScores().GetStrength() != 8 {
		t.Fatalf("the array, permuted: %v, %v", c, err)
	}

	// The point buy: 25 of 27 points is fine, 28 is not, nor a 16.
	_, err = bia.createWith(campaign, mBuy, scoresSheet(15, 15, 15, 9, 8, 8))
	wantRefusal("28 points", err, badBuy, mBuy)
	_, err = bia.createWith(campaign, mBuy, scoresSheet(16, 8, 8, 8, 8, 8))
	wantRefusal("a bought 16", err, badBuy, mBuy)
	if _, err := bia.createWith(campaign, mBuy, scoresSheet(10, 14, 13, 8, 15, 10)); err != nil {
		t.Fatalf("the point buy, 25 points: %v", err)
	}

	// Typed: 3 to 18; 19 is refused, 18 and 3 are not.
	_, err = caio.createWith(campaign, mTyped, scoresSheet(19, 10, 10, 10, 10, 10))
	wantRefusal("a typed 19", err, range3, mTyped)
	_, err = caio.createWith(campaign, mTyped, scoresSheet(2, 10, 10, 10, 10, 10))
	wantRefusal("a typed 2", err, range3, mTyped)
	// A client that does not say its way is the one from before the rules: on a table
	// that allows every way, only the sheet's own ranges (1 to 30) apply.
	if _, err := caio.createWith(campaign, charactersv1.AbilityMethod_ABILITY_METHOD_UNSPECIFIED, scoresSheet(20, 3, 10, 10, 10, 10)); err != nil {
		t.Fatalf("an unspecified way, on a table that allows every way: %v", err)
	}

	// 4d6: no stored rolls, other results, then the stored ones in any order.
	_, err = duda.createWith(campaign, mRoll, scoresSheet(16, 14, 13, 12, 10, 8))
	wantRefusal("4d6 before any roll", err, noRolls, mRoll)
	rolled, err := duda.rollScores(campaign)
	if err != nil || rolled.GetAlreadyRolled() || len(rolled.GetRolls().GetSets()) != 6 {
		t.Fatalf("RollAbilityScores() = %v, %v", rolled, err)
	}
	if got := totalsOf(rolled.GetRolls()); !proto.Equal(&rulesv1.AbilityScores{Strength: got[0], Dexterity: got[1], Constitution: got[2], Intelligence: got[3], Wisdom: got[4], Charisma: got[5]},
		&rulesv1.AbilityScores{Strength: 16, Dexterity: 14, Constitution: 13, Intelligence: 12, Wisdom: 10, Charisma: 8}) {
		t.Fatalf("the sets' totals = %v, want 16 14 13 12 10 8", got)
	}
	_, err = duda.createWith(campaign, mRoll, scoresSheet(16, 16, 13, 12, 10, 8))
	wantRefusal("a result twice", err, notRoll, mRoll)
	_, err = duda.createWith(campaign, mRoll, scoresSheet(17, 14, 13, 12, 10, 8))
	wantRefusal("another result", err, notRoll, mRoll)
	if _, err := duda.createWith(campaign, mRoll, scoresSheet(12, 14, 13, 8, 16, 10)); err != nil {
		t.Fatalf("the stored rolls, assigned: %v", err)
	}

	// The NPCs of the master and the master's edits are free.
	npc := master.create(t, campaign, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Ogro", func() *charactersv1.CharacterSheet {
		s := scoresSheet(25, 1, 20, 3, 8, 7)
		return s
	}())
	if npc.GetSheet().GetFull().GetBaseScores().GetStrength() != 25 {
		t.Errorf("the master's NPC = %v", npc.GetSheet().GetFull().GetBaseScores())
	}
	pc := ana.get(t, campaign, ana.mustLiving(t, campaign))
	if _, err := master.update(t, pc, pc.GetName(), scoresSheet(30, 30, 1, 1, 9, 9)); err != nil {
		t.Errorf("the master's edit of a player's scores: %v", err)
	}
}

// mustLiving is the ID of the player's living character.
func (u *user) mustLiving(t *testing.T, campaignID string) string {
	t.Helper()
	res, err := u.api.ListCharacters(t.Context(), connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: campaignID}))
	if err != nil || len(res.Msg.GetCharacters()) == 0 {
		t.Fatalf("ListCharacters() = %v, %v", res, err)
	}
	return res.Msg.GetCharacters()[0].GetId()
}

// TestRN24_ATableCanSwitchMethodsOff: a method the master switched off is refused
// at creation, and 4d6 cannot even be rolled.
func TestRN24_ATableCanSwitchMethodsOff(t *testing.T) {
	t.Parallel()
	_, master, players, campaign, _ := newAbilityTable(t)
	ana := players[0]
	setRules(t, master, campaign, methods(true, false, false, false)) // only the array
	for name, tc := range map[string]struct {
		method charactersv1.AbilityMethod
		sheet  *charactersv1.CharacterSheet
	}{
		"point buy": {mBuy, scoresSheet(10, 14, 13, 8, 15, 10)},
		"typed":     {mTyped, scoresSheet(10, 14, 13, 8, 15, 10)},
		"4d6":       {mRoll, scoresSheet(16, 14, 13, 12, 10, 8)},
		"no method": {charactersv1.AbilityMethod_ABILITY_METHOD_UNSPECIFIED, scoresSheet(10, 14, 13, 8, 15, 10)},
	} {
		_, err := ana.createWith(campaign, tc.method, tc.sheet)
		if r := abilityRefusal(t, name, err); r.GetReason() != charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_METHOD_NOT_ALLOWED {
			t.Errorf("%s: refusal = %v, want METHOD_NOT_ALLOWED", name, r)
		}
	}
	_, err := ana.rollScores(campaign)
	if r := abilityRefusal(t, "RollAbilityScores()", err); r.GetReason() != charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_METHOD_NOT_ALLOWED {
		t.Errorf("RollAbilityScores() refusal = %v, want METHOD_NOT_ALLOWED", r)
	}
	if _, err := ana.createWith(campaign, mStd, scoresSheet(15, 14, 13, 12, 10, 8)); err != nil {
		t.Errorf("the one method that is allowed: %v", err)
	}
}

// TestRN24_TheServerKeepsTheFourD6: the same sets come back until a character is
// created with them, and then the next character gets new sets.
func TestRN24_TheServerKeepsTheFourD6(t *testing.T) {
	t.Parallel()
	_, master, players, campaign, _ := newAbilityTable(t)
	ana, bia := players[0], players[1]
	if got := ana.storedScores(t, campaign); got != nil {
		t.Fatalf("before any roll, GetAbilityRolls() = %v, want none", got)
	}
	first, err := ana.rollScores(campaign)
	if err != nil || first.GetAlreadyRolled() || first.GetRolls().GetTyped() || first.GetRolls().GetRolledAt() == nil {
		t.Fatalf("first RollAbilityScores() = %v, %v", first, err)
	}
	// Reloading never rerolls: not by asking again, not by the read.
	again, err := ana.rollScores(campaign)
	if err != nil || !again.GetAlreadyRolled() || !proto.Equal(again.GetRolls(), first.GetRolls()) {
		t.Fatalf("second RollAbilityScores() = %v, %v; want the same sets, already_rolled", again, err)
	}
	if got := ana.storedScores(t, campaign); !proto.Equal(got, first.GetRolls()) {
		t.Errorf("GetAbilityRolls() = %v, want the first sets", got)
	}
	// Each player has their own sets.
	if got := bia.storedScores(t, campaign); got != nil {
		t.Errorf("another player's GetAbilityRolls() = %v, want none", got)
	}
	bFirst, err := bia.rollScores(campaign)
	if err != nil || proto.Equal(bFirst.GetRolls(), first.GetRolls()) {
		t.Fatalf("another player's roll = %v, %v; want sets of their own", bFirst, err)
	}

	// A sheet that uses them consumes them: the next roll is new.
	if _, err := ana.createWith(campaign, mRoll, scoresSheet(16, 14, 13, 12, 10, 8)); err != nil {
		t.Fatalf("CreateCharacter(4d6) error = %v", err)
	}
	if got := ana.storedScores(t, campaign); got != nil {
		t.Errorf("after the sheet, GetAbilityRolls() = %v, want none", got)
	}
	next, err := ana.rollScores(campaign)
	if err != nil || next.GetAlreadyRolled() || proto.Equal(next.GetRolls(), first.GetRolls()) {
		t.Fatalf("the next roll = %v, %v; want new sets", next, err)
	}
	for _, total := range totalsOf(next.GetRolls()) {
		if total != 18 {
			t.Errorf("the next sets' totals = %v, want 18 (the server's dice gave 6s)", totalsOf(next.GetRolls()))
			break
		}
	}
	// A refused sheet does not consume them.
	if _, err := bia.createWith(campaign, mRoll, scoresSheet(3, 3, 3, 3, 3, 3)); err == nil {
		t.Fatal("a sheet that is not the rolls was accepted")
	}
	if got := bia.storedScores(t, campaign); !proto.Equal(got, bFirst.GetRolls()) {
		t.Errorf("a refused sheet changed the stored sets: %v", got)
	}
	// The master has no ability scores to roll.
	_, err = master.rollScores(campaign)
	wantCode(t, "the master's RollAbilityScores()", err, connect.CodePermissionDenied)
	_, err = master.api.GetAbilityRolls(t.Context(), connect.NewRequest(&charactersv1.GetAbilityRollsRequest{CampaignId: campaign}))
	wantCode(t, "the master's GetAbilityRolls()", err, connect.CodePermissionDenied)
}

// TestRN24_PhysicalDiceAreTypedOnce: with physical dice the player types the six
// sets, once; the dice rule says which of the two ways a campaign allows.
func TestRN24_PhysicalDiceAreTypedOnce(t *testing.T) {
	t.Parallel()
	_, _, players, campaign, rule := newAbilityTable(t)
	ana, bia, caio := players[0], players[1], players[2]
	typed := [][]int32{{6, 5, 5, 2}, {5, 5, 4, 1}, {5, 4, 4, 3}, {4, 4, 4, 2}, {4, 3, 3, 2}, {3, 3, 2, 1}}
	const (
		forcedApp  = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_DICE_FORCED_IN_APP
		forcedReal = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_DICE_FORCED_PHYSICAL
		already    = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_ROLLS_ALREADY_STORED
	)

	// Everybody rolls in the app: typed dice are refused.
	rule.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_IN_APP))
	_, err := ana.rollScores(campaign, typed...)
	if r := abilityRefusal(t, "typed dice, app forced", err); r.GetReason() != forcedApp {
		t.Errorf("refusal = %v, want DICE_FORCED_IN_APP", r)
	}
	// Everybody rolls real dice: the server's roll is refused, typed dice are not.
	rule.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_FORCED_PHYSICAL))
	_, err = ana.rollScores(campaign)
	if r := abilityRefusal(t, "a roll, physical forced", err); r.GetReason() != forcedReal {
		t.Errorf("refusal = %v, want DICE_FORCED_PHYSICAL", r)
	}
	res, err := ana.rollScores(campaign, typed...)
	if err != nil || res.GetAlreadyRolled() || !res.GetRolls().GetTyped() {
		t.Fatalf("typed dice = %v, %v", res, err)
	}
	if got := totalsOf(res.GetRolls()); len(got) != 6 || got[0] != 16 || got[5] != 8 {
		t.Errorf("the typed sets' totals = %v, want 16 ... 8 (the lowest die of each dropped)", got)
	}
	// Typed once: other dice are refused, the same dice are a retry, and no dice read them.
	other := append([][]int32{{1, 1, 1, 1}}, typed[1:]...)
	_, err = ana.rollScores(campaign, other...)
	if r := abilityRefusal(t, "typing again", err); r.GetReason() != already {
		t.Errorf("refusal = %v, want ROLLS_ALREADY_STORED", r)
	}
	if retry, err := ana.rollScores(campaign, typed...); err != nil || !retry.GetAlreadyRolled() || !proto.Equal(retry.GetRolls(), res.GetRolls()) {
		t.Errorf("typing the same dice again = %v, %v; want the stored sets", retry, err)
	}
	if got := ana.storedScores(t, campaign); !proto.Equal(got, res.GetRolls()) {
		t.Errorf("GetAbilityRolls() = %v, want the typed sets", got)
	}
	// And the sheet is made from them, like any stored sets.
	if _, err := ana.createWith(campaign, mRoll, scoresSheet(10, 12, 13, 14, 16, 8)); err != nil {
		t.Errorf("a sheet from the typed dice: %v", err)
	}

	// Players choose: either way is fine.
	rule.rule.Store(int32(charactersv1.LevelUpDiceRule_LEVEL_UP_DICE_RULE_PLAYER_CHOOSES))
	if _, err := bia.rollScores(campaign, typed...); err != nil {
		t.Errorf("typed dice, players choose: %v", err)
	}
	if _, err := caio.rollScores(campaign); err != nil {
		t.Errorf("a roll, players choose: %v", err)
	}

	// Dice that are not six sets of four 1-to-6 dice are invalid.
	for name, bad := range map[string][][]int32{
		"five sets":  typed[:5],
		"three dice": {{1, 2, 3}, {1, 2, 3}, {1, 2, 3}, {1, 2, 3}, {1, 2, 3}, {1, 2, 3}},
		"a die of 7": {{7, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}},
		"a die of 0": {{0, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}},
	} {
		_, err := players[3].rollScores(campaign, bad...)
		wantCode(t, name, err, connect.CodeInvalidArgument)
	}
}

// TestRN24_ThePendingMemberMakesTheirScoresToo: a pending member (RN-15) creates
// their one character with the same rules.
func TestRN24_ThePendingMemberMakesTheirScoresToo(t *testing.T) {
	t.Parallel()
	h, master, _, campaign, _ := newAbilityTable(t)
	pending := h.newUser("Pendente")
	h.joinPending(master, campaign, pending)
	rolled, err := pending.rollScores(campaign)
	if err != nil || len(rolled.GetRolls().GetSets()) != 6 {
		t.Fatalf("a pending member's RollAbilityScores() = %v, %v", rolled, err)
	}
	_, err = pending.createWith(campaign, mStd, scoresSheet(10, 14, 13, 8, 15, 10))
	abilityRefusal(t, "a pending member's wrong array", err)
	if c, err := pending.createWith(campaign, mRoll, scoresSheet(12, 14, 13, 8, 16, 10)); err != nil || c.GetState() != charactersv1.CharacterState_CHARACTER_STATE_PENDING {
		t.Errorf("a pending member's sheet = %v, %v", c, err)
	}
}

// TestRN24_TheTableRulesAreReadInTheCallersTransaction (PR #121): a write that
// reads the table's rules passes the one-connection pool, because the read goes
// through the transaction it holds. The test pool has one connection and an
// Acquire tracer, so a read through the pool in the transaction would fail here.
func TestRN24_TheTableRulesAreReadInTheCallersTransaction(t *testing.T) {
	t.Parallel()
	var rec *recordingSource
	h := newHarnessWith(t, func(c *Config) { rec = &recordingSource{inner: c.Content}; c.Content = rec })
	master, owner := h.newUser("Samuel"), h.newUser("Dona")
	campaign := h.newCampaign(master, "Mirathel", owner)
	setRules(t, master, campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_AVERAGE))
	rec.take()

	// A creation, which reads the rules inside its transaction.
	if _, err := owner.createWith(campaign, mTyped, scoresSheet(12, 14, 13, 8, 16, 10)); err != nil {
		t.Fatalf("CreateCharacter() error = %v", err)
	}
	inTx := false
	for _, a := range rec.take() {
		inTx = inTx || a.inTx
	}
	if !inTx {
		t.Error("CreateCharacter never read the table's rules through its transaction")
	}
	// And a read of the rules inside a transaction, directly.
	err := db.InTx(t.Context(), h.pool, func(tx pgx.Tx) error {
		r, err := h.svc.TableRulesFor(t.Context(), tx, campaign)
		if err != nil || r.HitPoints != HitPointsAverage {
			t.Errorf("TableRulesFor() = %+v, %v; want the average rule", r, err)
		}
		return err
	})
	if err != nil {
		t.Fatalf("InTx() error = %v", err)
	}
}

// TestRN24_ADraftEditFollowsTheRecordedMethod: the server records how the scores
// were made, and while the sheet is the player's draft their edits of the base
// scores must still follow it; the master's edits are free, the client cannot
// rewrite the record, and a sheet with no record (a client from before the rules)
// is not checked.
func TestRN24_ADraftEditFollowsTheRecordedMethod(t *testing.T) {
	t.Parallel()
	h, master, players, campaign, _ := newAbilityTable(t)
	ana, bia, caio, duda := players[0], players[1], players[2], players[3]
	edit := func(u *user, c *charactersv1.Character, scores ...int32) (*charactersv1.Character, error) {
		return u.update(t, c, c.GetName(), scoresSheet(scores...))
	}
	wantRefusal := func(call string, err error, reason charactersv1.AbilityScoresRefusalReason, method charactersv1.AbilityMethod) {
		t.Helper()
		if r := abilityRefusal(t, call, err); r.GetReason() != reason || r.GetMethod() != method {
			t.Errorf("%s refusal = %v, want %v for %v", call, r, reason, method)
		}
	}
	const (
		notStd  = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_NOT_STANDARD_ARRAY
		badBuy  = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_BAD_POINT_BUY
		notRoll = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_NOT_THE_ROLLS
		range3  = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_TYPED_OUT_OF_RANGE
	)

	// The array: another assignment is fine, other numbers are not.
	pcA, err := ana.createWith(campaign, mStd, scoresSheet(15, 14, 13, 12, 10, 8))
	if err != nil {
		t.Fatal(err)
	}
	if o := pcA.GetSheet().GetFull().GetAbilityOrigin(); o.GetMethod() != mStd {
		t.Fatalf("recorded origin = %v, want the standard array", o)
	}
	pcA, err = edit(ana, pcA, 8, 10, 12, 13, 14, 15)
	if err != nil {
		t.Fatalf("a permutation of the array: %v", err)
	}
	_, err = edit(ana, pcA, 18, 10, 12, 13, 14, 15)
	wantRefusal("the array, edited to a 18", err, notStd, mStd)
	// The client cannot rewrite the record to get past it.
	forged := scoresSheet(18, 18, 18, 18, 18, 18)
	forged.GetFull().AbilityOrigin = &charactersv1.AbilityOrigin{Method: mTyped}
	_, err = ana.update(t, pcA, pcA.GetName(), forged)
	wantRefusal("a forged origin", err, notStd, mStd)
	// Edits that leave the scores alone are not about them.
	same := pcA.GetSheet()
	same.GetFull().Languages = []string{"Élfico"}
	if pcA, err = ana.update(t, pcA, pcA.GetName(), same); err != nil || pcA.GetSheet().GetFull().GetAbilityOrigin().GetMethod() != mStd {
		t.Fatalf("an edit that leaves the scores: %v, %v", pcA, err)
	}
	// The master edits free, and the record stays.
	if pcA, err = edit(master, pcA, 30, 1, 9, 9, 9, 9); err != nil || pcA.GetSheet().GetFull().GetAbilityOrigin().GetMethod() != mStd {
		t.Fatalf("the master's edit: %v, %v", pcA, err)
	}

	// Point buy.
	pcB, err := bia.createWith(campaign, mBuy, scoresSheet(10, 14, 13, 8, 15, 10))
	if err != nil {
		t.Fatal(err)
	}
	if pcB, err = edit(bia, pcB, 15, 15, 8, 8, 15, 8); err != nil {
		t.Errorf("a purchase within 27: %v", err)
	}
	_, err = edit(bia, pcB, 15, 15, 15, 15, 8, 8)
	wantRefusal("the purchase, edited past 27", err, badBuy, mBuy)

	// Typed.
	pcC, err := caio.createWith(campaign, mTyped, scoresSheet(12, 12, 12, 12, 12, 12))
	if err != nil {
		t.Fatal(err)
	}
	if pcC, err = edit(caio, pcC, 18, 3, 12, 12, 12, 12); err != nil {
		t.Errorf("typed 18 and 3: %v", err)
	}
	_, err = edit(caio, pcC, 19, 3, 12, 12, 12, 12)
	wantRefusal("typed 19", err, range3, mTyped)

	// 4d6: the sheet keeps the six results, which the creation consumed.
	if _, err := duda.rollScores(campaign); err != nil {
		t.Fatal(err)
	}
	pcD, err := duda.createWith(campaign, mRoll, scoresSheet(16, 14, 13, 12, 10, 8))
	if err != nil {
		t.Fatal(err)
	}
	if got := duda.storedScores(t, campaign); got != nil {
		t.Fatalf("the stored rolls after the sheet: %v", got)
	}
	if pcD, err = edit(duda, pcD, 8, 10, 12, 13, 14, 16); err != nil {
		t.Errorf("the stored results, reassigned: %v", err)
	}
	_, err = edit(duda, pcD, 18, 10, 12, 13, 14, 16)
	wantRefusal("4d6, edited to a 18", err, notRoll, mRoll)

	// A sheet made by a client that did not say its way has no record, so no check.
	h2 := newHarness(t)
	m2, p2 := h2.newUser("Samuel"), h2.newUser("Eva")
	c2 := h2.newCampaign(m2, "Outra", p2)
	legacy, err := p2.createWith(c2, charactersv1.AbilityMethod_ABILITY_METHOD_UNSPECIFIED, scoresSheet(12, 12, 12, 12, 12, 12))
	if err != nil || legacy.GetSheet().GetFull().GetAbilityOrigin() != nil {
		t.Fatalf("a sheet with no way said = %v, %v; want no record", legacy, err)
	}
	if _, err := p2.update(t, legacy, legacy.GetName(), scoresSheet(25, 1, 12, 12, 12, 12)); err != nil {
		t.Errorf("a draft edit of a sheet with no record: %v", err)
	}
	// An NPC never gets one, even if the client sends it.
	npcSheet := scoresSheet(12, 12, 12, 12, 12, 12)
	npcSheet.GetFull().AbilityOrigin = &charactersv1.AbilityOrigin{Method: mStd}
	npc := m2.create(t, c2, charactersv1.CharacterKind_CHARACTER_KIND_ENEMY, "Ogro", npcSheet)
	if npc.GetSheet().GetFull().GetAbilityOrigin() != nil {
		t.Error("an NPC kept an ability origin the client sent")
	}
	_ = h
}

// TestRN24_TheManualBonusesCannotGetAroundTheMethod: the positive manual bonuses of a
// player's sheet may not add up to more than the race lets them place (plus 2 per
// Ability Score Improvement reached), at creation and on a draft edit; negative
// ones, the master and a sheet with no recorded way are free.
func TestRN24_TheManualBonusesCannotGetAroundTheMethod(t *testing.T) {
	t.Parallel()
	_, master, players, campaign, _ := newAbilityTable(t)
	ana, bia, caio := players[0], players[1], players[2]
	const extra = charactersv1.AbilityScoresRefusalReason_ABILITY_SCORES_REFUSAL_REASON_EXTRA_BONUSES
	withExtra := func(race string, bonus *rulesv1.AbilityScores) *charactersv1.CharacterSheet {
		s := scoresSheet(15, 14, 13, 12, 10, 8)
		s.GetFull().RaceKey, s.GetFull().SubraceKey, s.GetFull().ExtraAbilityBonuses = race, "", bonus
		return s
	}
	// A gnome places nothing: +10 in Strength is a score of 25 on an array table.
	_, err := ana.createWith(campaign, mStd, withExtra("race:gnome", &rulesv1.AbilityScores{Strength: 10}))
	if r := abilityRefusal(t, "a gnome with +10", err); r.GetReason() != extra {
		t.Errorf("refusal = %v, want EXTRA_BONUSES", r)
	}
	// Negative values are free.
	pc, err := ana.createWith(campaign, mStd, withExtra("race:gnome", &rulesv1.AbilityScores{Strength: -3, Charisma: -10}))
	if err != nil {
		t.Fatalf("negative bonuses: %v", err)
	}
	// The half-elf places two points: +1 and +1 is fine, +2 and +1 is not.
	if _, err := bia.createWith(campaign, mStd, withExtra("race:half-elf", &rulesv1.AbilityScores{Strength: 1, Dexterity: 1})); err != nil {
		t.Fatalf("the half-elf's two points: %v", err)
	}
	_, err = caio.createWith(campaign, mStd, withExtra("race:half-elf", &rulesv1.AbilityScores{Strength: 2, Dexterity: 1}))
	if r := abilityRefusal(t, "a half-elf with 3 points", err); r.GetReason() != extra {
		t.Errorf("refusal = %v, want EXTRA_BONUSES", r)
	}
	// A draft edit by the player is checked too; the master edits free.
	edited := pc.GetSheet()
	edited.GetFull().ExtraAbilityBonuses = &rulesv1.AbilityScores{Strength: 5}
	_, err = ana.update(t, pc, pc.GetName(), edited)
	if r := abilityRefusal(t, "a draft edit with +5", err); r.GetReason() != extra {
		t.Errorf("refusal = %v, want EXTRA_BONUSES", r)
	}
	if _, err := master.update(t, pc, pc.GetName(), edited); err != nil {
		t.Errorf("the master's edit: %v", err)
	}
	// A sheet with no recorded way (a client from before the rules) is not checked.
	legacy, err := players[3].createWith(campaign, charactersv1.AbilityMethod_ABILITY_METHOD_UNSPECIFIED, withExtra("race:gnome", &rulesv1.AbilityScores{Strength: 10}))
	if err != nil || legacy == nil {
		t.Errorf("a sheet with no way said: %v", err)
	}
}

// TestRN24_ACreationIsRefusedWhatItsInputsBreak: an unknown method is invalid_argument,
// and the hit points a player stores per level follow the table's rule.
func TestRN24_ACreationIsRefusedWhatItsInputsBreak(t *testing.T) {
	t.Parallel()
	_, master, players, campaign, _ := newAbilityTable(t)
	ana, bia, caio := players[0], players[1], players[2]
	_, err := ana.createWith(campaign, charactersv1.AbilityMethod(99), scoresSheet(15, 14, 13, 12, 10, 8))
	wantCode(t, "an unknown method", err, connect.CodeInvalidArgument)

	// Pensantus is level 3: his hit points per level are decided at creation.
	rolled := func() *charactersv1.CharacterSheet {
		s := scoresSheet(15, 14, 13, 12, 10, 8)
		s.GetFull().HitPoints = &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_ROLLED, Rolls: []int32{3, 4}}
		return s
	}
	average := func() *charactersv1.CharacterSheet {
		s := scoresSheet(15, 14, 13, 12, 10, 8)
		s.GetFull().HitPoints = &charactersv1.HitPoints{Method: charactersv1.HitPointsMethod_HIT_POINTS_METHOD_AVERAGE}
		return s
	}
	wantRule := func(call string, err error) {
		t.Helper()
		if r := refusal(t, call, err); r.GetReason() != charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_HIT_POINTS_RULE {
			t.Errorf("%s refusal = %v, want HIT_POINTS_RULE", call, r)
		}
	}
	setRules(t, master, campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_ROLL))
	_, err = ana.createWith(campaign, mStd, average())
	wantRule("the average where everybody rolls", err)
	short := rolled()
	short.GetFull().HitPoints.Rolls = []int32{3}
	_, err = ana.createWith(campaign, mStd, short)
	wantRule("a level without a roll", err)
	if _, err := ana.createWith(campaign, mStd, rolled()); err != nil {
		t.Errorf("rolls where everybody rolls: %v", err)
	}
	setRules(t, master, campaign, hitPointsRule(campaignsv1.HitPointsRule_HIT_POINTS_RULE_AVERAGE))
	_, err = bia.createWith(campaign, mStd, rolled())
	wantRule("rolls where everybody takes the average", err)
	if _, err := bia.createWith(campaign, mStd, average()); err != nil {
		t.Errorf("the average where everybody takes it: %v", err)
	}
	_ = caio
}

// TestRN24_TwoRollsOfTheSameCharacterStoreOneSet: two RollAbilityScores that race
// both answer with the same sets.
func TestRN24_TwoRollsOfTheSameCharacterStoreOneSet(t *testing.T) {
	t.Parallel()
	dbtest.PoolSize(t, 2) // the racers must overlap: one connection would run them one by one
	_, _, players, campaign, _ := newAbilityTable(t)
	ana := players[0]
	var wg sync.WaitGroup
	results := make([]*charactersv1.RollAbilityScoresResponse, 4)
	errs := make([]error, 4)
	start := make(chan struct{})
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i], errs[i] = ana.rollScores(campaign)
		}()
	}
	close(start)
	wg.Wait()
	rolled := 0
	for i, res := range results {
		if errs[i] != nil {
			t.Fatalf("RollAbilityScores() #%d error = %v", i, errs[i])
		}
		if !res.GetAlreadyRolled() {
			rolled++
		}
		if !proto.Equal(res.GetRolls(), results[0].GetRolls()) {
			t.Errorf("call #%d got other sets than call #0", i)
		}
	}
	if rolled != 1 {
		t.Errorf("%d calls rolled the sets, want exactly one", rolled)
	}
	if got := ana.storedScores(t, campaign); !proto.Equal(got, results[0].GetRolls()) {
		t.Errorf("the stored sets are not the ones answered")
	}
}
