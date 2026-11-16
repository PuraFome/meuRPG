package play

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The held NPC check where it meets the rest of the play package: the rows it leaves, the combat that opens, the session that ends, the
// vitals of the bard that cuts and the answers that are told apart by their key.

// The questions of a table of two bards, Brisa (Bia) and Lia (Ana), are asked in this order; the NPC is the Barão.
func (n *npcTable) charOf(u *user) *charactersv1.Character {
	if u == n.bia {
		return n.bri
	}
	return n.lia
}

// session is the table's open session.
func (n *npcTable) session(t *testing.T) *playv1.GameSession {
	t.Helper()
	return n.master.liveSession(t, n.campaignID).GetGameSession()
}

// uses are the uses of Bardic Inspiration a bard has left.
func (n *npcTable) uses(t *testing.T, c *charactersv1.Character) int32 {
	t.Helper()
	left, _ := poolOf(n.vitalsOfCharacter(t, c), rules.BardicInspirationKey)
	return left
}

// heldRow is what the table keeps of a hold.
type heldRow struct {
	kind     string
	answered bool
	faces    []int32
	total    int32
}

// hold reads a hold row, false when it is gone.
func (n *npcTable) hold(t *testing.T, id string) (heldRow, bool) {
	t.Helper()
	var r heldRow
	rows, err := n.h.pool.Query(t.Context(), `SELECT kind, answer_key IS NOT NULL, faces, total FROM inspiration_holds WHERE id = $1`, id)
	if err != nil {
		t.Fatalf("read the hold: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return r, false
	}
	if err := rows.Scan(&r.kind, &r.answered, &r.faces, &r.total); err != nil {
		t.Fatalf("scan the hold: %v", err)
	}
	return r, true
}

// plant writes a hold row the way the code that made it would have left it, with no request to make it: another kind, or a check whose
// session is over. The asks are the bards it asks, in order.
func (n *npcTable) plant(t *testing.T, campaignID, characterID, userID, kind string, at time.Time, asks ...*charactersv1.Character) string {
	t.Helper()
	row, err := n.h.svc.queries.InsertInspirationHold(t.Context(), playdb.InsertInspirationHoldParams{
		CampaignID: campaignID, CharacterID: characterID, UserID: userID, Kind: kind, IdempotencyKey: newKey(),
		Request: []byte{}, Faces: []int32{15}, CreatedAt: at,
	})
	if err != nil {
		t.Fatalf("plant a %s hold: %v", kind, err)
	}
	for i, b := range asks {
		if _, err := n.h.svc.queries.InsertNpcCheckAsk(t.Context(), playdb.InsertNpcCheckAskParams{
			HoldID: row.ID, BardCharacterID: b.GetId(), Position: int32(i), Sides: 6, CreatedAt: at,
		}); err != nil {
			t.Fatalf("plant an ask: %v", err)
		}
	}
	return row.ID
}

// pass is the answer of a bard that lets the check go, as the given user and with the given key.
func (n *npcTable) pass(u *user, holdID, key string) (*playv1.AnswerHeldRollResponse, error) {
	res, err := u.resource.AnswerHeldRoll(n.h.t.Context(), connect.NewRequest(&playv1.AnswerHeldRollRequest{
		CampaignId: n.campaignID, HoldId: holdID, IdempotencyKey: key,
		Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: &playv1.CuttingWordsAnswer{Cut: false}},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// answerAs is a bard's answer with the key given.
func (n *npcTable) answerAs(u *user, holdID, key string, a *playv1.CuttingWordsAnswer) (*playv1.AnswerHeldRollResponse, error) {
	res, err := u.resource.AnswerHeldRoll(n.h.t.Context(), connect.NewRequest(&playv1.AnswerHeldRollRequest{
		CampaignId: n.campaignID, HoldId: holdID, IdempotencyKey: key, Answer: &playv1.AnswerHeldRollRequest_CuttingWords{CuttingWords: a},
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func typedCut(face int32) *playv1.CuttingWordsAnswer {
	return &playv1.CuttingWordsAnswer{Cut: true, Roll: &playv1.CuttingWordsAnswer_TypedFace{TypedFace: face}}
}

// skip is the master's skip of the asks.
func (n *npcTable) skip(u *user, holdID, key string) (*playv1.NpcCheckRead, error) {
	res, err := u.play.SkipNpcCheckAsks(n.h.t.Context(), connect.NewRequest(&playv1.SkipNpcCheckAsksRequest{CampaignId: n.campaignID, HoldId: holdID, IdempotencyKey: key}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetCheck(), nil
}

// resolvedEvent is a npc_check_resolved event as the session keeps it.
type resolvedEvent struct {
	actor, character string
	total, cut       int32
	skipped          bool
}

func (n *npcTable) resolvedEvents(t *testing.T) []resolvedEvent {
	t.Helper()
	rows, err := n.h.pool.Query(t.Context(), `SELECT coalesce(actor_user_id::TEXT, ''), coalesce(character_id::TEXT, ''), (payload->>'total')::INT,
		coalesce((payload->>'cut')::INT, 0), coalesce((payload->>'skipped')::BOOL, false) FROM session_events WHERE kind = 'npc_check_resolved' ORDER BY seq`)
	if err != nil {
		t.Fatalf("read the resolved events: %v", err)
	}
	defer rows.Close()
	var out []resolvedEvent
	for rows.Next() {
		var e resolvedEvent
		if err := rows.Scan(&e.actor, &e.character, &e.total, &e.cut, &e.skipped); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, e)
	}
	return out
}

// TestNpcCheckForgetNeedsTheCheckItsSessionAndItsPeople: a check whose session is over is forgotten by an answer that finds it so, but
// only a check, only one of this campaign, and only for the master and the bards it asks: anyone else reads "not found" and nothing is
// deleted (the hold of another kind, of another member or of another campaign survives).
func TestNpcCheckForgetNeedsTheCheckItsSessionAndItsPeople(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	other := n.h.newUser("Zed")
	otherCampaign := n.h.newCampaign(other, "Outra mesa")
	foreign := other.npc(t, otherCampaign, "Forasteiro", 9, 11)
	stale := time.Now().Add(-48 * time.Hour)
	now := n.h.clock.Now()
	mine := n.plant(t, n.campaignID, n.toren.GetId(), n.caio.id, "scene_check", now)
	theirs := n.plant(t, n.campaignID, n.bri.GetId(), n.bia.id, "scene_check", now)
	foreigners := n.plant(t, otherCampaign, foreign.GetId(), other.id, "npc_check", now, foreign)
	leftover := func() string { return n.plant(t, n.campaignID, n.npcID, n.master.id, "npc_check", now, n.bri, n.lia) }
	survives := func(what, id string) {
		t.Helper()
		if _, ok := n.hold(t, id); !ok {
			t.Errorf("%s: the hold is gone, want it to stay", what)
		}
	}

	// The session is over: the check of the earlier session is nobody's to answer.
	n.master.end(t, n.session(t))
	for what, id := range map[string]string{"their own hold of another kind": mine, "somebody else's hold of another kind": theirs, "a check of another campaign": foreigners} {
		_, err := n.pass(n.caio, id, newKey())
		wantCode(t, "Caio answers "+what, err, connect.CodeNotFound)
		survives("Caio answers "+what, id)
	}
	one, two, three := leftover(), leftover(), leftover()
	_, err := n.pass(n.caio, one, newKey())
	wantCode(t, "a player that is not asked answers a check of a session that is over", err, connect.CodeNotFound)
	survives("Caio answers a check that does not ask Caio", one)
	_, err = n.pass(n.bia, two, newKey())
	wantCode(t, "an asked bard answers a check of a session that is over", err, connect.CodeNotFound)
	if _, ok := n.hold(t, two); ok {
		t.Error("an asked bard answered a check of a session that is over: the hold stays, want it forgotten")
	}
	_, err = n.pass(n.master, three, newKey())
	wantCode(t, "the master answers a check of a session that is over", err, connect.CodeNotFound)
	if _, ok := n.hold(t, three); ok {
		t.Error("the master answered a check of a session that is over: the hold stays, want it forgotten")
	}
	survives("the check of the other campaign", foreigners)

	// A new session: the check of the old one is stale. Only the master and the bards asked drop it.
	n.master.start(t, n.campaignID)
	old := n.plant(t, n.campaignID, n.npcID, n.master.id, "npc_check", stale, n.bri, n.lia)
	_, err = n.pass(n.caio, old, newKey())
	wantCode(t, "a player that is not asked answers a stale check", err, connect.CodeNotFound)
	survives("Caio answers a stale check", old)
	_, err = n.pass(n.bia, old, newKey())
	wantCode(t, "an asked bard answers a stale check", err, connect.CodeNotFound)
	if _, ok := n.hold(t, old); ok {
		t.Error("an asked bard answered a stale check: the hold stays, want it forgotten")
	}
	other1, other2 := n.plant(t, n.campaignID, n.npcID, n.master.id, "npc_check", stale, n.bri), n.plant(t, n.campaignID, n.toren.GetId(), n.caio.id, "scene_check", stale)
	_, err = n.skip(n.master, other2, newKey())
	wantCode(t, "the master skips a hold of another kind", err, connect.CodeNotFound)
	survives("the master skips a hold of another kind", other2)
	if _, err = n.skip(n.master, other1, newKey()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("the master skips a stale check: %v, want not_found", err)
	}
	if _, ok := n.hold(t, other1); ok {
		t.Error("the master skipped a stale check: the hold stays, want it forgotten")
	}
}

// TestNpcCheckIsSettledAsASkipWhenACombatOpens: the master rolls a check the bards are asked about and opens a combat before they
// answer: the check settles at the d20 (the history says it was skipped), the asks close, the master and the bard asked are told, and
// the question is gone. No check is rolled while the combat is set up.
func TestNpcCheckIsSettledAsASkipWhenACombatOpens(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil || !res.GetHeld() {
		t.Fatalf("RollNpcCheck() = %v, %v; want a held check", res, err)
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	master, bard := listen(t, n.h, n.campaignID, n.master.id, true), listen(t, n.h, n.campaignID, first.id, false)
	other := n.ana
	if first == n.ana {
		other = n.bia
	}
	elsewhere := listen(t, n.h, n.campaignID, n.caio.id, false)
	_, err = n.master.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
		CampaignId: n.campaignID, IdempotencyKey: newKey(), Name: "Emboscada", Participants: []*playv1.Participant{{CharacterId: n.npcID}},
		Mode: playv1.EncounterMode_ENCOUNTER_MODE_THEATRE,
	}))
	if err != nil {
		t.Fatalf("StartEncounter() error = %v", err)
	}
	if row, ok := n.hold(t, hold); !ok || !row.answered {
		t.Errorf("the hold after the combat opened = %+v, %v; want it settled", row, ok)
	}
	want := []resolvedEvent{{actor: n.master.id, character: n.npcID, total: 15, skipped: true}}
	if got := n.resolvedEvents(t); len(got) != 1 || got[0] != want[0] {
		t.Errorf("the resolved events = %+v, want %+v: one skip, written for the creature by the master", got, want)
	}
	for _, u := range []*user{n.master, first, other} {
		if got := n.listHeldRolls(t, u); len(got) != 0 {
			t.Errorf("%s still reads %v after the combat opened", u.id, got)
		}
	}
	wantHeard := func(who string, h heardBy, want bool) {
		t.Helper()
		heard := false
		for _, ev := range h.heard(t, n.h) {
			heard = heard || ev.GetHeldRollsChanged() != nil
		}
		if heard != want {
			t.Errorf("%s heard the held rolls change = %v, want %v", who, heard, want)
		}
	}
	wantHeard("the master", master, true)
	wantHeard("the bard asked", bard, true)
	wantHeard("a player that was not asked", elsewhere, false)
	_, err = n.pass(first, hold, newKey())
	wantCode(t, "the bard answers after the combat opened", err, connect.CodeFailedPrecondition)
	_, err = n.roll(t, 12, true)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_COMBAT_OPEN)
	if left := n.uses(t, n.charOf(first)); left != 3 {
		t.Errorf("the bard's uses after the combat opened = %d, want 3: nothing was spent", left)
	}
}

// npcFight is a combat of the armed table in which the goblin is a character-backed NPC with a hold of the master's check waiting.
func npcFight(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{20},
		players: map[string]int32{"Toren": 1, "Pensantus": 1, "Brisa": 1}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	})
	hold, err := a.h.svc.queries.InsertInspirationHold(t.Context(), playdb.InsertInspirationHoldParams{
		CampaignID: a.campaignID, CharacterID: a.goblin.GetId(), UserID: a.master.id, Kind: "npc_check", IdempotencyKey: newKey(),
		Request: []byte{}, Faces: []int32{9}, CreatedAt: a.h.clock.Now(),
	})
	if err != nil {
		t.Fatalf("plant the check: %v", err)
	}
	return a, e, hold.ID
}

func (a *armed) holdAlive(t *testing.T, id string) bool {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM inspiration_holds WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count the hold: %v", err)
	}
	return n == 1
}

// TestNpcCheckHoldNeverStopsTheNpcsOtherRolls: the master's check of an NPC waits in a hold of the NPC's character; the NPC's own
// attack in a combat, with a Bardic Inspiration die in hand so that the gate looks at the holds, is not refused for it (a question
// about a roll the bards cannot answer is no question about the NPC's roll).
func TestNpcCheckHoldNeverStopsTheNpcsOtherRolls(t *testing.T) {
	t.Parallel()
	a, e, hold := npcFight(t)
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE combatants SET inspiration_sides = 6, inspiration_expires_round = 9 WHERE id = $1`, a.id(t, "Goblin")); err != nil {
		t.Fatalf("give the goblin a die: %v", err)
	}
	a.h.roller.queue(12)
	if _, err := a.attack(t, a.master, e, "Goblin", "basic:0", "Toren", inAppRoll); err != nil {
		t.Fatalf("the goblin's attack with a check held = %v, want no refusal", err)
	}
	if !a.holdAlive(t, hold) {
		t.Error("the goblin's attack deleted the check held for the bards")
	}
}

// TestNpcCheckHoldSurvivesTheNpcLeavingACombat: a combatant that leaves, or a combat that ends, forgets the rolls held for it; the master's
// check of the NPC is no roll of the combatant's, so it is not forgotten.
func TestNpcCheckHoldSurvivesTheNpcLeavingACombat(t *testing.T) {
	t.Parallel()
	a, e, hold := npcFight(t)
	if _, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Goblin"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant() error = %v", err)
	}
	if !a.holdAlive(t, hold) {
		t.Error("removing the NPC's combatant deleted the check held for the bards")
	}
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	if !a.holdAlive(t, hold) {
		t.Error("ending the combat deleted the check held for the bards")
	}
}

// TestNpcCheckCutTellsTheBardsVitals: the use a bard spends by cutting is pushed to the bard's own screen at once, as every other spend
// of Bardic Inspiration is; a pass spends nothing and pushes nothing of the vitals.
func TestNpcCheckCutTellsTheBardsVitals(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	own := n.charOf(first)
	vitalsOf := func(h heardBy) []*playv1.CharacterVitals {
		var out []*playv1.CharacterVitals
		for _, ev := range h.heard(t, n.h) {
			if v := ev.GetVitalsChanged().GetVitals(); v != nil {
				out = append(out, v)
			}
		}
		return out
	}
	bard, master, other := listen(t, n.h, n.campaignID, first.id, false), listen(t, n.h, n.campaignID, n.master.id, true), listen(t, n.h, n.campaignID, n.caio.id, false)
	if _, err := n.answer(t, first, hold, true, 4); err != nil {
		t.Fatalf("the cut error = %v", err)
	}
	for who, h := range map[string]heardBy{"the bard": bard, "the master": master} {
		got := vitalsOf(h)
		if len(got) != 1 || got[0].GetCharacterId() != own.GetId() {
			t.Fatalf("%s heard the vitals %v after the cut, want the bard's, once", who, got)
		}
		if left, _ := poolOf(got[0], rules.BardicInspirationKey); left != 2 {
			t.Errorf("%s heard %d uses left, want 2", who, left)
		}
	}
	if got := vitalsOf(other); len(got) != 0 {
		t.Errorf("a player that owns neither heard the vitals %v", got)
	}
}

// TestNpcCheckCutNeedsABardAwakeWithTheSetting: the question asks a bard that may cut; by the time it is answered the bard may be at 0
// hit points or have changed the setting: the bard cannot cut then (the use is not spent), but may pass.
func TestNpcCheckCutNeedsABardAwakeWithTheSetting(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		change func(t *testing.T, n *npcTable, bard *charactersv1.Character)
	}{
		"at 0 hit points": {func(t *testing.T, n *npcTable, bard *charactersv1.Character) {
			t.Helper()
			if _, err := n.h.pool.Exec(t.Context(), `INSERT INTO character_vitals (character_id, hit_points_current, updated_at) VALUES ($1, 0, now())
				ON CONFLICT (character_id) DO UPDATE SET hit_points_current = 0`, bard.GetId()); err != nil {
				t.Fatalf("knock the bard out: %v", err)
			}
		}},
		"with the setting for attacks only": {func(t *testing.T, n *npcTable, bard *charactersv1.Character) {
			t.Helper()
			sheet := proto.Clone(bard.GetSheet()).(*charactersv1.CharacterSheet)
			sheet.GetFull().CuttingWordsAsk = charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ONLY_ATTACKS
			if _, err := n.master.characters.UpdateCharacter(t.Context(), connect.NewRequest(&charactersv1.UpdateCharacterRequest{CampaignId: n.campaignID, CharacterId: bard.GetId(), Revision: bard.GetRevision(), Name: bard.GetName(), Sheet: sheet})); err != nil {
				t.Fatalf("change the setting: %v", err)
			}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			res, err := n.roll(t, 15, true)
			if err != nil {
				t.Fatalf("RollNpcCheck() error = %v", err)
			}
			hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
			own := n.charOf(first)
			tc.change(t, n, own)
			_, err = n.answer(t, first, hold, true, 4)
			wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NOT_AVAILABLE)
			if left := n.uses(t, own); left != 3 {
				t.Errorf("the refused cut left %d uses, want 3: nothing is spent", left)
			}
			if got, err := n.answer(t, first, hold, false, 0); err != nil || got.GetCut() || got.GetUsesLeft() != 3 {
				t.Errorf("the pass = %v, %v; want it taken, no cut and 3 uses", got, err)
			}
		})
	}
}

// TestNpcCheckSurvivesTheOrphanSweep: the sweep of the holds that wait for a die that ran out (it runs when game time moves on) leaves
// the master's check alone: it waits for the bards' dice, never for the NPC's.
func TestNpcCheckSurvivesTheOrphanSweep(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold := res.GetCheck().GetHoldId()
	if _, err := n.master.lasting.AdvanceGameTime(t.Context(), connect.NewRequest(&playv1.AdvanceGameTimeRequest{CampaignId: n.campaignID, IdempotencyKey: newKey(), Seconds: 60})); err != nil {
		t.Fatalf("AdvanceGameTime() error = %v", err)
	}
	if row, ok := n.hold(t, hold); !ok || row.answered {
		t.Fatalf("the hold after the sweep = %+v, %v; want it kept, waiting", row, ok)
	}
	if got := n.listHeldRolls(t, n.firstAsked(res)); len(got) != 1 {
		t.Errorf("the bard reads %v after the sweep, want the bard's question", got)
	}
}

// TestNpcCheckIsForgottenWhenItsSessionEnds: a check still held when the session ends can never be answered: its hold and its asks go in
// the transaction that ends the session, and the history says it was skipped.
func TestNpcCheckIsForgottenWhenItsSessionEnds(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	settled, err := n.roll(t, 9, false) // a check nobody is asked about is settled from the start: no hold to forget
	if err != nil || settled.GetHeld() {
		t.Fatalf("the check with nobody to ask = %v, %v", settled, err)
	}
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold := res.GetCheck().GetHoldId()
	n.master.end(t, n.session(t))
	if _, ok := n.hold(t, hold); ok {
		t.Error("the hold of the check is still there after the session ended")
	}
	var asks int
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM npc_check_asks WHERE hold_id = $1`, hold).Scan(&asks); err != nil || asks != 0 {
		t.Errorf("the asks of the check = %d, %v; want none", asks, err)
	}
	got := n.resolvedEvents(t)
	if len(got) != 2 || !got[1].skipped || got[1].total != 15 || got[1].actor != n.master.id || got[1].character != n.npcID {
		t.Errorf("the resolved events = %+v, want the settled 9 and then the skip of the held 15, by the master, for the NPC", got)
	}
}

// TestNpcCheckLastBardSettlesInTheMastersName: the check settles by the last answer the same way it does by the skip: the event is the
// creature's, written by the master (the bard's answer is the bard's own, and is in the ask).
func TestNpcCheckLastBardSettlesInTheMastersName(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	second := n.ana
	if first == n.ana {
		second = n.bia
	}
	if _, err := n.answer(t, first, hold, true, 4); err != nil {
		t.Fatalf("the first answer error = %v", err)
	}
	if _, err := n.answer(t, second, hold, false, 0); err != nil {
		t.Fatalf("the last answer error = %v", err)
	}
	want := resolvedEvent{actor: n.master.id, character: n.npcID, total: 11, cut: 4}
	if got := n.resolvedEvents(t); len(got) != 1 || got[0] != want {
		t.Errorf("the resolved events = %+v, want %+v", got, want)
	}
}

// TestNpcCheckAnswerKeyIsForOneAnswerOnly: the key of a bard's answer stands for that answer: sent again with another body (a pass
// for a cut, another face) it is a reused key; the same body answers as the first did.
func TestNpcCheckAnswerKeyIsForOneAnswerOnly(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	key := newKey()
	if _, err := n.answerAs(first, hold, key, typedCut(4)); err != nil {
		t.Fatalf("the cut error = %v", err)
	}
	for name, a := range map[string]*playv1.CuttingWordsAnswer{
		"a pass":         {Cut: false},
		"another face":   typedCut(5),
		"the app's roll": {Cut: true, Roll: &playv1.CuttingWordsAnswer_RollInApp{RollInApp: true}},
	} {
		_, err := n.answerAs(first, hold, key, a)
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "idempotency_key") {
			t.Errorf("the key of the cut sent with %s = %v, want the reused key's refusal", name, err)
		}
	}
	if got, err := n.answerAs(first, hold, key, typedCut(4)); err != nil || got.GetCuttingWords().GetDie().GetFaces()[0] != 4 {
		t.Errorf("the same cut again = %v, %v; want the first answer", got, err)
	}
	if left := n.uses(t, n.charOf(first)); left != 2 {
		t.Errorf("uses left = %d, want 2: one spent, whatever was sent again", left)
	}
}

// TestNpcCheckRetryAfterTheSettleIsNotHeld: the retry of the roll that asked the bards, once the check settled, reads the check as it
// settled and is not told "held"; a retry before it is held, changes nothing and reads the same.
func TestNpcCheckRetryAfterTheSettleIsNotHeld(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	req := &playv1.RollNpcCheckRequest{
		CampaignId: n.campaignID, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: n.npcID}, CheckKey: deception,
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 15}, BardSees: true, IdempotencyKey: newKey(),
	}
	call := func() *playv1.RollNpcCheckResponse {
		t.Helper()
		res, err := n.master.play.RollNpcCheck(t.Context(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("RollNpcCheck() error = %v", err)
		}
		return res.Msg
	}
	first, again := call(), call()
	if !proto.Equal(first, again) || !first.GetHeld() {
		t.Fatalf("the retry = %v, want the first answer, %v", again, first)
	}
	var holds int
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM inspiration_holds WHERE kind = 'npc_check'`).Scan(&holds); err != nil || holds != 1 {
		t.Fatalf("holds = %d, %v; want the one", holds, err)
	}
	hold, firstAsked := first.GetCheck().GetHoldId(), n.firstAsked(first)
	second := n.ana
	if firstAsked == n.ana {
		second = n.bia
	}
	if _, err := n.answer(t, firstAsked, hold, true, 4); err != nil {
		t.Fatalf("the first answer error = %v", err)
	}
	if _, err := n.answer(t, second, hold, false, 0); err != nil {
		t.Fatalf("the last answer error = %v", err)
	}
	settled := call()
	if settled.GetHeld() || !settled.GetCheck().GetSettled() || settled.GetCheck().GetTotal() != 11 || settled.GetCheck().GetCut() != 4 {
		t.Errorf("the retry after the settle = %v, want a settled 11 that is not held", settled)
	}
}

// TestNpcCheckOfADeletedHoldIsNotSettled: a check whose hold is gone reads as the event kept it; the event of a check still held keeps
// the held total, so it does not read as a settled one.
func TestNpcCheckOfADeletedHoldIsNotSettled(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	if _, err := n.h.pool.Exec(t.Context(), `DELETE FROM inspiration_holds WHERE id = $1`, res.GetCheck().GetHoldId()); err != nil {
		t.Fatalf("delete the hold: %v", err)
	}
	var key string
	if err := n.h.pool.QueryRow(t.Context(), `SELECT idempotency_key::TEXT FROM session_events WHERE kind = 'npc_check_held'`).Scan(&key); err != nil {
		t.Fatalf("read the key: %v", err)
	}
	got, err := n.master.play.RollNpcCheck(t.Context(), connect.NewRequest(&playv1.RollNpcCheckRequest{
		CampaignId: n.campaignID, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: n.npcID}, CheckKey: deception,
		Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: 15}, BardSees: true, IdempotencyKey: key,
	}))
	if err != nil {
		t.Fatalf("the retry error = %v", err)
	}
	if got.Msg.GetHeld() || got.Msg.GetCheck().GetSettled() {
		t.Errorf("the retry of a check whose hold is gone = %v, want neither held nor settled", got.Msg)
	}
}

// TestNpcCheckHoldIsFoundByNoOtherAnswer: the hold of a check is answered by the cutting_words arm alone. Every other arm of
// AnswerHeldRoll, and the RPCs of the outside inspiration, the outside lucky, the attack roll's inspiration and lucky, read "not found",
// for the bard whose turn it is and for the master; the check is still held and the bards still answer it.
func TestNpcCheckHoldIsFoundByNoOtherAnswer(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	encounter := newKey() // any UUID: the attack roll's RPCs look for a hold of an attack
	held := func(a any) *playv1.AnswerHeldRollRequest {
		r := &playv1.AnswerHeldRollRequest{CampaignId: n.campaignID, HoldId: hold, IdempotencyKey: newKey()}
		switch a := a.(type) {
		case *playv1.LuckyAnswer:
			r.Answer = &playv1.AnswerHeldRollRequest_Lucky{Lucky: a}
		case *playv1.DieAnswer:
			r.Answer = &playv1.AnswerHeldRollRequest_Die{Die: a}
		case *playv1.IndomitableAnswer:
			r.Answer = &playv1.AnswerHeldRollRequest_Indomitable{Indomitable: a}
		case *playv1.StrokeOfLuckAnswer:
			r.Answer = &playv1.AnswerHeldRollRequest_StrokeOfLuck{StrokeOfLuck: a}
		case *playv1.RerollAnswer:
			r.Answer = &playv1.AnswerHeldRollRequest_Reroll{Reroll: a}
		}
		return r
	}
	for _, u := range []*user{first, n.master} {
		calls := map[string]func() error{
			"AnswerHeldRoll lucky": func() error {
				_, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(held(&playv1.LuckyAnswer{})))
				return err
			},
			"AnswerHeldRoll die": func() error {
				_, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(held(&playv1.DieAnswer{})))
				return err
			},
			"AnswerHeldRoll indomitable": func() error {
				_, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(held(&playv1.IndomitableAnswer{})))
				return err
			},
			"AnswerHeldRoll stroke": func() error {
				_, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(held(&playv1.StrokeOfLuckAnswer{})))
				return err
			},
			"AnswerHeldRoll reroll": func() error {
				_, err := u.resource.AnswerHeldRoll(t.Context(), connect.NewRequest(held(&playv1.RerollAnswer{})))
				return err
			},
			"AnswerOutsideInspiration": func() error { _, err := n.answerOutside(t, u, hold, false, false, 0); return err },
			"AnswerOutsideLucky": func() error {
				_, err := answerOutsideLuckyKey(t, u, n.campaignID, hold, newKey(), func(*playv1.AnswerOutsideLuckyRequest) {})
				return err
			},
			"AnswerBardicInspiration": func() error {
				_, err := n.armed.answer(t, u, &playv1.Encounter{Id: encounter}, hold, func(*playv1.AnswerBardicInspirationRequest) {})
				return err
			},
			"AnswerLucky": func() error {
				_, err := n.answerLucky(t, u, &playv1.Encounter{Id: encounter}, hold, func(*playv1.AnswerLuckyRequest) {})
				return err
			},
		}
		for name, call := range calls {
			want := connect.CodeNotFound
			if u == n.master && strings.HasPrefix(name, "AnswerOutside") {
				want = connect.CodePermissionDenied // the outside answers are the player's alone: the master is refused before any hold is read
			}
			wantCode(t, name, call(), want)
		}
	}
	if row, ok := n.hold(t, hold); !ok || row.answered || row.kind != "npc_check" {
		t.Fatalf("the hold = %+v, %v after the refused answers, want it still held", row, ok)
	}
	if _, err := n.answer(t, first, hold, false, 0); err != nil {
		t.Fatalf("the bard's own answer error = %v", err)
	}
}

// TestNpcCheckSettledAnswerReplays: the retry of the answer that settled the check (the last bard's) reads what the first one read and
// writes nothing: one resolved event, one use spent.
func TestNpcCheckSettledAnswerReplays(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	res, err := n.roll(t, 15, true)
	if err != nil {
		t.Fatalf("RollNpcCheck() error = %v", err)
	}
	hold, first := res.GetCheck().GetHoldId(), n.firstAsked(res)
	last := n.ana
	if first == n.ana {
		last = n.bia
	}
	if _, err := n.answer(t, first, hold, false, 0); err != nil {
		t.Fatalf("the first answer error = %v", err)
	}
	key, before := newKey(), n.uses(t, n.charOf(last))
	settled, err := n.answerAs(last, hold, key, typedCut(4))
	if err != nil || !settled.GetCuttingWords().GetCut() {
		t.Fatalf("the settling answer = %v, %v; want a cut", settled, err)
	}
	again, err := n.answerAs(last, hold, key, typedCut(4))
	if err != nil || !proto.Equal(settled, again) {
		t.Errorf("the retry = %v, %v; want %v", again, err, settled)
	}
	if got := n.resolvedEvents(t); len(got) != 1 || got[0].total != 11 || got[0].cut != 4 {
		t.Errorf("the resolved events = %+v, want the one, 11 less 4", got)
	}
	if left := n.uses(t, n.charOf(last)); left != before-1 {
		t.Errorf("uses left = %d, want %d: one spent, the retry spent nothing", left, before-1)
	}
}

// TestNpcCheckRollKeyIsForOneRollOnly: the key of a roll stands for that roll: sent again with another face, another check or another
// creature it is a reused key, and no second check is held.
func TestNpcCheckRollKeyIsForOneRollOnly(t *testing.T) {
	t.Parallel()
	n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
	key := newKey()
	roll := func(face int32, check string) error {
		_, err := n.master.play.RollNpcCheck(t.Context(), connect.NewRequest(&playv1.RollNpcCheckRequest{
			CampaignId: n.campaignID, Npc: &playv1.RollNpcCheckRequest_NpcCharacterId{NpcCharacterId: n.npcID}, CheckKey: check,
			Roll: &playv1.RollNpcCheckRequest_D20Face{D20Face: face}, BardSees: true, IdempotencyKey: key,
		}))
		return err
	}
	if err := roll(15, deception); err != nil {
		t.Fatalf("the roll error = %v", err)
	}
	for name, err := range map[string]error{"another face": roll(9, deception), "another check": roll(15, "skill:insight")} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "idempotency_key") {
			t.Errorf("the key of the roll sent with %s = %v, want the reused key's refusal", name, err)
		}
	}
	if err := roll(15, deception); err != nil {
		t.Errorf("the same roll again error = %v", err)
	}
	var holds int
	if err := n.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM inspiration_holds WHERE kind = 'npc_check'`).Scan(&holds); err != nil || holds != 1 {
		t.Errorf("holds = %d, %v; want the one", holds, err)
	}
}

// TestNpcCheckQuestionNamesTheCreatureWhereTheBardSeesIt: the bard's question says who the creature is only when the bard's player sees
// it: its token in the bard's sight on the map, or, with no map, its place on the stage of a scene the player is in. The master's box
// alone asks the bard, and calls the creature "uma criatura". A token out of the 60 feet asks nobody, whatever the box says.
func TestNpcCheckQuestionNamesTheCreatureWhereTheBardSeesIt(t *testing.T) {
	t.Parallel()
	label := func(n *npcTable, res *playv1.RollNpcCheckResponse) string {
		t.Helper()
		offers := n.listHeldRolls(t, n.firstAsked(res))
		if len(offers) != 1 {
			t.Fatalf("the bard asked reads %d offers, want 1", len(offers))
		}
		return offers[0].GetRollerLabel()
	}
	onMap := func(n *npcTable, col int) {
		t.Helper()
		mapID := n.h.newMap(n.campaignID, gridColumns)
		if _, err := n.master.play.SetCurrentMap(t.Context(), connect.NewRequest(&playv1.SetCurrentMapRequest{CampaignId: n.campaignID, MapId: mapID})); err != nil {
			t.Fatalf("SetCurrentMap() error = %v", err)
		}
		n.h.placeToken(mapID, n.bri.GetId(), 250, 250)
		n.h.placeToken(mapID, n.lia.GetId(), 250, 750)
		n.h.placeToken(mapID, n.npcID, col*500+250, 250)
	}
	stage := func(n *npcTable) {
		t.Helper()
		point, _ := n.h.newScene(n.h.newMap(n.campaignID, gridColumns), "A clareira", true, 1)
		n.openScene(t, point)
		if _, err := n.master.play.PutOnStage(t.Context(), connect.NewRequest(&playv1.PutOnStageRequest{CampaignId: n.campaignID, CharacterId: n.npcID})); err != nil {
			t.Fatalf("PutOnStage() error = %v", err)
		}
	}
	for name, tc := range map[string]struct {
		setup func(*npcTable)
		box   bool
		want  string // the label, "" when nobody is asked
	}{
		"on the map within sight, the box off": {func(n *npcTable) { onMap(n, 6) }, false, "Barão"},
		"on the map out of the 60 feet":        {func(n *npcTable) { onMap(n, 15) }, true, ""},
		"on the stage, the box on":             {stage, true, "Barão"},
		"on the stage, the box off":            {stage, false, ""},
		"not on the stage, the box on":         {func(*npcTable) {}, true, unnamedCreature},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := newNpcTable(t, charactersv1.CuttingWordsAsk_CUTTING_WORDS_ASK_ALL)
			tc.setup(n)
			res, err := n.roll(t, 15, tc.box)
			if err != nil {
				t.Fatalf("RollNpcCheck() error = %v", err)
			}
			if tc.want == "" {
				if res.GetHeld() {
					t.Fatalf("the check was held, want nobody asked: %v", res)
				}
				return
			}
			if !res.GetHeld() {
				t.Fatalf("the check was not held, want the bards asked: %v", res)
			}
			if got := label(n, res); got != tc.want {
				t.Errorf("the bard's question calls the creature %q, want %q", got, tc.want)
			}
		})
	}
}

const unnamedCreature = "uma criatura"

// resolvedCount is how many checks of the table settled.
func (n *npcTable) resolvedCount(t *testing.T) int {
	t.Helper()
	return len(n.resolvedEvents(t))
}
