package play

import (
	"testing"

	"connectrpc.com/connect"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Cutting Words on an NPC's ability check outside a combat (SRD 5.1, College of Lore): the master rolls the creature's d20, the bards
// that may cut it are asked one at a time, and the check settles when the last one answers or the master goes on without them.

const deception = "skill:deception"

// npcTable is a scene with no map and no combat: the master, a fighter, and two Lore bards (Brisa and Lia) with the given setting.
type npcTable struct {
	*armed
	lia   *charactersv1.Character
	liaU  *user
	npcID string
}

func loreSheet(t *testing.T, u *user, campaignID, name string, ask charactersv1.CuttingWordsAsk, cha int32) *charactersv1.Character {
	t.Helper()
	sheet := &charactersv1.CharacterSheet{Content: &charactersv1.CharacterSheet_Full{Full: &charactersv1.FullSheet{
		BaseScores: abilities(10, 14, 12, 10, cha), RaceKey: "race:human", Classes: []*charactersv1.ClassLevel{classLevel("class:bard", 3, "subclass:lore")},
		WeaponKeys: []string{rapier}, CuttingWordsAsk: ask,
	}}}
	res, err := u.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
		CampaignId: campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: name, Sheet: sheet,
	}))
	if err != nil {
		t.Fatalf("CreateCharacter(%s) error = %v", name, err)
	}
	return res.Msg.GetCharacter()
}

func newNpcTable(t *testing.T, ask charactersv1.CuttingWordsAsk) *npcTable {
	t.Helper()
	h := newHarness(t)
	a := &armed{h: h, master: h.newUser("Samuel"), caio: h.newUser("Caio"), ana: h.newUser("Ana"), bia: h.newUser("Bia")}
	a.campaignID = h.newCampaign(a.master, "Mirathel", a.caio, a.ana, a.bia)
	a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores16(), []string{battleaxe}, nil)
	a.bri = loreSheet(t, a.bia, a.campaignID, "Brisa", ask, 16)
	n := &npcTable{armed: a, liaU: a.ana}
	n.lia = loreSheet(t, a.ana, a.campaignID, "Lia", ask, 14)
	n.npcID = a.master.npc(t, a.campaignID, "Barão", 20, 12).GetId()
	a.master.start(t, a.campaignID)
	return n
}

func (n *npcTable) roll(t *testing.T, face int32, sees bool) (*playv1.RollNpcCheckResponse, error) {
	t.Helper()
	return n.rollWith(t, face, sees, newKey())
}

// rollWith is roll under the given key.
func (n *npcTable) rollWith(t *testing.T, face int32, sees bool, key string) (*playv1.RollNpcCheckResponse, error) {
	t.Helper()
	res, err := n.master.play.RollNpcCheck(t.Context(), connect.NewRequest(&playv1.RollNpcCheckRequest{
		CampaignId: n.campaignID, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: n.npcID}, CheckKey: deception,
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: face}, BardSees: sees, IdempotencyKey: key,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (n *npcTable) answer(t *testing.T, u *user, holdID string, cut bool, face int32) (*playv1.CuttingWordsAnswered, error) {
	t.Helper()
	a := &playv1.CuttingWordsAnswer{Cut: cut}
	if cut {
		a.Roll = &playv1.CuttingWordsAnswer_TypedFace{TypedFace: face}
	}
	res, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(&playv1.AnswerHeldRollRequest{
		CampaignId: n.campaignID, HoldId: holdID, IdempotencyKey: newKey(), Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: a},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCuttingWords(), nil
}

// TestNpcCheckAsksTheBardsInOrderAndSettlesTheTotal: both bards see the Barão; Brisa (sheet order) is asked first and cuts a 4, Lia
// is asked next and lets it pass; the check settles at 15 - 4.
func TestNpcCheckAsksTheBardsInOrderAndSettlesTheTotal(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	if !res.GetHeld() || res.GetCheck().GetD20().GetFaces()[0] != 15 || len(res.GetCheck().GetAsks()) != 2 {
		t.Fatalf("RollNpcCheck() = %v, want a held check of 15 with two asks", res)
	}
	hold := res.GetCheck().GetHoldId()
	first, second := n.bia, n.ana
	if res.GetCheck().GetAsks()[0].GetCharacterName() != "Brisa" {
		first, second = n.ana, n.bia
	}
	if got := len(n.listHeldRolls(t, second)); got != 0 {
		t.Fatalf("the second bard reads %d offers before the turn, want 0", got)
	}
	if got := n.listHeldRolls(t, first); len(got) != 1 || got[0].GetD20() != nil {
		t.Fatalf("the first bard reads %v, want one question with no d20", got)
	}
	_, err = n.answer(t, second, hold, true, 3)
	wantCode(t, "the bard out of turn cuts the check", err, connect.CodeNotFound)
	if a, err := n.answer(t, first, hold, true, 4); err != nil || !a.GetCut() {
		t.Fatalf("first answer = %v, %v, want a cut", a, err)
	}
	if got := n.listHeldRolls(t, second); len(got) != 1 {
		t.Fatalf("the second bard reads %d offers after the first, want 1", len(got))
	}
	if _, err := n.answer(t, second, hold, false, 0); err != nil {
		t.Fatalf("second answer error = %v", err)
	}
	for _, o := range n.listHeldRolls(t, n.master) {
		t.Fatalf("the master still reads %v after the check settled", o)
	}
}

// resolved is the settled event of the table's check: the total and the die that came off.
func (n *npcTable) resolved(t *testing.T) (total, cut int32, count int) {
	t.Helper()
	rows, err := n.h.pool.Query(t.Context(), `SELECT (payload->>'total')::int, coalesce((payload->>'cut')::int, 0) FROM session_events WHERE kind = 'npc_check_resolved'`)
	if err != nil {
		t.Fatalf("read the resolved events: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&total, &cut); err != nil {
			t.Fatalf("scan: %v", err)
		}
		count++
	}
	return total, cut, count
}

// TestNpcCheckSettlesWithTheDieOffAndSpendsOneUse: one bard cuts a 4 off a 15: the check settles at 11 and the uses go down by one.
func TestNpcCheckSettlesWithTheDieOffAndSpendsOneUse(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	before := n.usesLeft(t)
	hold := res.GetCheck().GetHoldId()
	u := n.bia
	if res.GetCheck().GetAsks()[0].GetCharacterName() != "Brisa" {
		u = n.ana
	}
	if _, err := n.answer(t, u, hold, true, 4); err != nil {
		t.Fatalf("answer error = %v", err)
	}
	other := n.ana
	if u == n.ana {
		other = n.bia
	}
	if _, err := n.answer(t, other, hold, false, 0); err != nil {
		t.Fatalf("second answer error = %v", err)
	}
	if total, cut, _ := n.resolved(t); total != 11 || cut != 4 {
		t.Fatalf("resolved total, cut = %d, %d; want 11, 4", total, cut)
	}
	if u == n.bia {
		if after := n.usesLeft(t); after != before-1 {
			t.Fatalf("uses left %d -> %d, want one spent", before, after)
		}
	}
}

// TestNpcCheckAnswerRetryIsIdempotent: the same key answers once; another key for the closed ask is refused.
func TestNpcCheckAnswerRetryIsIdempotent(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold := res.GetCheck().GetHoldId()
	u := n.bia
	if res.GetCheck().GetAsks()[0].GetCharacterName() != "Brisa" {
		u = n.ana
	}
	pass := &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}}
	req := &playv1.AnswerHeldRollRequest{CampaignId: n.campaignID, HoldId: hold, IdempotencyKey: newKey(), Answer: pass}
	for i := range 2 {
		if _, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(req)); err != nil {
			t.Fatalf("answer #%d error = %v", i, err)
		}
	}
	req.IdempotencyKey = newKey()
	_, err = u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(req))
	wantCode(t, "answer again with another key", err, connect.CodeFailedPrecondition)
	if got := n.resolvedCount(t); got != 0 {
		t.Errorf("%d resolved events after a pass of the first bard, want 0: the second still waits", got)
	}
}

// TestNpcCheckWithNobodyToAskSettlesAtOnce: the bards do not see the creature, so nobody is asked.
func TestNpcCheckWithNobodyToAskSettlesAtOnce(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, false)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	if res.GetHeld() || !res.GetCheck().GetSettled() || res.GetCheck().GetTotal() != 15 {
		t.Fatalf("RollNpcCheck() = %v, want a settled 15", res)
	}
}

// TestNpcCheckSettingMatrix: only the setting "todos" asks for a check.
func TestNpcCheckSettingMatrix(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		ask  charactersv1.CuttingWordsAsk
		held bool
	}{
		"all":     {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, true},
		"attacks": {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ONLY_ATTACKS, false},
		"never":   {charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_NEVER, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, tc.ask)
			res, err := n.roll(t, 12, true)
			if err != nil || res.GetHeld() != tc.held {
				t.Fatalf("RollNpcCheck() = %v, %v; want held %v", res, err, tc.held)
			}
		})
	}
}

// TestNpcCheckSkipGoesOnWithoutTheBards: the master goes on; the check settles at the d20, the retry replays, the bard is refused.
func TestNpcCheckSkipGoesOnWithoutTheBards(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, key := res.GetCheck().GetHoldId(), newKey()
	skip := func(u *user) (*playv1.SkipNpcCheckAsksResponse, error) {
		r, err := u.play.SkipNpcCheckAsks(t.Context(), connect.NewRequest(&playv1.SkipNpcCheckAsksRequest{CampaignId: n.campaignID, HoldId: hold, IdempotencyKey: key}))
		if err != nil {
			return nil, err
		}
		return r.Msg, nil
	}
	_, err = skip(n.caio)
	wantCode(t, "a player skips", err, connect.CodePermissionDenied)
	for i := range 2 {
		out, err := skip(n.master)
		if err != nil || !out.GetCheck().GetSettled() || out.GetCheck().GetTotal() != 15 {
			t.Fatalf("skip #%d = %v, %v; want a settled 15", i, out, err)
		}
	}
	if _, _, count := n.resolved(t); count != 1 {
		t.Fatalf("%d resolved events, want 1", count)
	}
	key = newKey() // another skip of the settled check reads it as it settled and writes nothing
	if out, err := skip(n.master); err != nil || !out.GetCheck().GetSettled() || out.GetCheck().GetTotal() != 15 {
		t.Fatalf("a second skip = %v, %v; want the settled 15", out, err)
	}
	if _, _, count := n.resolved(t); count != 1 {
		t.Fatalf("%d resolved events after a second skip, want 1", count)
	}
	_, err = n.answer(t, n.bia, hold, true, 4)
	wantCode(t, "a bard answers after the skip", err, connect.CodeFailedPrecondition)
	if left := n.uses(t, n.bri); left != 3 {
		t.Errorf("Brisa has %d uses after the skip, want 3: nothing was spent", left)
	}
}

// TestNpcCheckIsMasterOnly: a player rolls, lists the picker or skips nothing.
func TestNpcCheckIsMasterOnly(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	_, err := n.bia.play.RollNpcCheck(t.Context(), connect.NewRequest(&playv1.RollNpcCheckRequest{
		CampaignId: n.campaignID, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: n.npcID}, CheckKey: deception,
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 10}, IdempotencyKey: newKey(),
	}))
	wantCode(t, "a player rolls", err, connect.CodePermissionDenied)
	_, err = n.bia.play.ListNpcCheckPicker(t.Context(), connect.NewRequest(&playv1.ListNpcCheckPickerRequest{CampaignId: n.campaignID}))
	wantCode(t, "a player lists the picker", err, connect.CodePermissionDenied)
}

// TestNpcCheckPickerListsTheCreature: the stage-less NPC is not listed, the SRD key asked is.
func TestNpcCheckPickerListsTheSRDKey(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.master.play.ListNpcCheckPicker(t.Context(), connect.NewRequest(&playv1.ListNpcCheckPickerRequest{CampaignId: n.campaignID, MonsterKey: "monster:wolf"}))
	if err != nil || len(res.Msg.GetCandidates()) == 0 {
		t.Fatalf("ListNpcCheckPicker() = %v, %v; want the wolf", res, err)
	}
}

// TestNpcCheckRefusedWhileACombatRuns: a check outside a combat is not rolled inside one.
func TestNpcCheckRefusedWhileACombatRuns(t *testing.T) {
	t.Parallel()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 2, scores16(), []string{battleaxe}, nil)
		a.bri = loreSheet(t, a.bia, a.campaignID, "Brisa", charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL, 16)
	})
	a.start(t, plan{npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3}, players: map[string]int32{"Toren": 10, "Brisa": 10}})
	n := &npcTable{armed: a, npcID: a.goblin.GetId()}
	_, err := n.roll(t, 12, true)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_COMBAT_OPEN)
}

// firstAsked is the user and the check of the bard asked first.
func (n *npcTable) firstAsked(res *playv1.RollNpcCheckResponse) *user {
	if res.GetCheck().GetAsks()[0].GetCharacterName() == "Brisa" {
		return n.bia
	}
	return n.ana
}

// TestNpcCheckPhysicalDiceTypesTheBardsFace: a table of real dice (the master's own d20 is typed or rolled either way); the bard types the face; the app's roll is refused, a typed face out
// of the die is refused, and the master's d20 is untouched by the bard's mode.
func TestNpcCheckPhysicalDiceTypesTheBardsFace(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	n.forceDice(t, campaignsv1.DiceMode_DICE_MODE_PHYSICAL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, u := res.GetCheck().GetHoldId(), n.firstAsked(res)
	cut := func(r *playv1.CuttingWordsAnswer) error {
		_, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(&playv1.AnswerHeldRollRequest{
			CampaignId: n.campaignID, HoldId: hold, IdempotencyKey: newKey(), Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: r},
		}))
		return err
	}
	wantEncounterBlocked(t, cut(&playv1.CuttingWordsAnswer{Cut: true, Roll: &playv1.CuttingWordsAnswer_RollInApp{RollInApp: true}}), blockedWrongDice)
	wantCode(t, "a face of 9 on a d6", cut(&playv1.CuttingWordsAnswer{Cut: true, Roll: &playv1.CuttingWordsAnswer_TypedFace{TypedFace: 9}}), connect.CodeInvalidArgument)
	if row, ok := n.hold(t, hold); !ok || row.answered {
		t.Errorf("the hold = %+v, %v after the refused faces, want it still held", row, ok)
	}
	if err := cut(&playv1.CuttingWordsAnswer{Cut: true, Roll: &playv1.CuttingWordsAnswer_TypedFace{TypedFace: 5}}); err != nil {
		t.Fatalf("a typed face of 5 error = %v", err)
	}
}

// TestNpcCheckAnswerLosesTheRaceWithTheSkip: the master goes on without the bards while the first one answers; one of them wins, the
// check settles once and the loser reads a neutral refusal or the same answer.
func TestNpcCheckAnswerLosesTheRaceWithTheSkip(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, u := res.GetCheck().GetHoldId(), n.firstAsked(res)
	done := make(chan error, 2)
	go func() {
		_, err := n.master.play.SkipNpcCheckAsks(t.Context(), connect.NewRequest(&playv1.SkipNpcCheckAsksRequest{CampaignId: n.campaignID, HoldId: hold, IdempotencyKey: newKey()}))
		done <- err
	}()
	go func() {
		_, err := n.answer(t, u, hold, true, 3)
		done <- err
	}()
	for range 2 {
		if err := <-done; err != nil && connect.CodeOf(err) != connect.CodeNotFound && connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("a racer failed with %v, want success or a neutral refusal", err)
		}
	}
	if _, _, count := n.resolved(t); count > 1 {
		t.Fatalf("%d resolved events, want at most 1", count)
	}
}
