package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// A held roll that spent the Inspiration and is dropped before it is answered never happens, so the Inspiration goes back: one way to
// drop a hold (hold_forget.go), and every path that ends a hold runs through it. These tests run the real paths (the gate, the removal
// of a combatant, the end of a combat, the orphan sweep of game time), each with a hold that spent the Inspiration and has no answer.

// newHalflingFavorFight is the favor fight with a halfling party: a 1 on a d20 asks the Lucky question, which needs no die.
func newHalflingFavorFight(t *testing.T) *favorFight {
	t.Helper()
	a := newCastersOf(t, 3, "race:halfling")
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {4, 9}},
	})
	return favorFightOf(t, a, e)
}

// holdExists says the hold is still in the table.
func (f *favorParty) holdExists(t *testing.T, id string) bool {
	t.Helper()
	var n int
	if err := f.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM inspiration_holds WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count the hold %s: %v", id, err)
	}
	return n == 1
}

// spentOnDie is Toren's attack made with the Inspiration while he holds a Bardic Inspiration die: the roll waits for the die's question,
// with the Inspiration spent. It returns the hold.
func (f *favorFight) spentOnDie(t *testing.T) string {
	t.Helper()
	f.giveCombatantDie(t, "Toren", f.e)
	f.give(t, f.toren)
	f.h.roller.queue(4, 15)
	held, err := favorPaths[0].send(t, f, f.caio, newKey(), true, nil)
	if err != nil || held.holdID == "" || f.favorHolds(t) || !f.holdFavorOf(t, held.holdID) {
		t.Fatalf("the attack with the Inspiration and a die = %+v, %v (Inspiration there %v), want the die's question with the Inspiration spent", held, err, f.favorHolds(t))
	}
	return held.holdID
}

// spentOnLucky is Toren's attack (a halfling's) made with the Inspiration that shows a 1 on both dice: the roll waits for the Lucky
// question, which needs no die. It returns the hold.
func (f *favorFight) spentOnLucky(t *testing.T) string {
	t.Helper()
	f.give(t, f.toren)
	f.h.roller.queue(1, 1)
	res, err := f.attack(t, f.caio, f.e, "Toren", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { inAppRoll(r); r.SpendFavor = true })
	id := res.GetHeldRollOffer().GetHoldId()
	if err != nil || id == "" || f.favorHolds(t) || !f.holdFavorOf(t, id) {
		t.Fatalf("the attack with the Inspiration that shows a 1 = %v, %v (Inspiration there %v), want the Lucky question with the Inspiration spent", res, err, f.favorHolds(t))
	}
	return id
}

func (f *favorFight) wantGivenBack(t *testing.T, what, id string) {
	t.Helper()
	if f.holdExists(t, id) {
		t.Errorf("%s: the hold %s is still there", what, id)
	}
	if !f.favorHolds(t) {
		t.Errorf("%s: the Inspiration did not come back (rows = %d)", what, f.rows(t))
	}
}

// The gate forgets a stale hold of the same roller when its next held roll comes (the round turned): the roll that spent the Inspiration
// never happened, so it goes back.
func TestFavorGoesBackWhenTheGateForgetsAStaleHold(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	id := f.spentOnDie(t)
	if _, err := f.endTurn(t, f.master, f.e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	f.e = f.passTo(t, f.e, "Toren") // the round turned
	if !f.holdExists(t, id) {
		t.Fatalf("the hold %s went before the next held roll: the test does not reach the gate", id)
	}
	f.h.roller.queue(7)
	if _, err := favorPaths[0].send(t, f, f.caio, newKey(), false, nil); err != nil {
		t.Fatalf("the next attack error = %v", err)
	}
	f.wantGivenBack(t, "the next held roll of the roller", id)
}

// The sweep of game time forgets the holds that wait for a die that ran out, and the Inspiration they spent goes back.
func TestFavorGoesBackWhenTheDieOfAHoldRunsOut(t *testing.T) {
	t.Parallel()
	e := newSceneCheckEnv(t)
	e.die(t)
	e.give(t, e.who)
	e.h.roller.queue(4, 15)
	held, err := e.send(t, newKey(), true)
	if err != nil || held.holdID == "" || e.favorHolds(t) {
		t.Fatalf("a scene check with a die and the spend = %+v, %v (Inspiration there %v), want the die's question with the Inspiration spent", held, err, e.favorHolds(t))
	}
	e.advanceGameTime(t, 700) // the die had 600 seconds
	if e.holdExists(t, held.holdID) || !e.favorHolds(t) {
		t.Errorf("after the die ran out the hold is there = %v, the Inspiration is there = %v, want the hold swept and the Inspiration back", e.holdExists(t, held.holdID), e.favorHolds(t))
	}
}

// The master removes a combatant that waits for a Lucky question (no die, so nothing else forgets it): the cascade of the foreign key
// would delete the hold and the Inspiration it spent with it.
func TestFavorGoesBackWhenACombatantWithALuckyHoldIsRemoved(t *testing.T) {
	t.Parallel()
	f := newHalflingFavorFight(t)
	id := f.spentOnLucky(t)
	// A player's combatant leaves only in the setup, so the combat goes back to it.
	if _, err := f.h.pool.Exec(t.Context(), `UPDATE encounters SET status = 'setup' WHERE id = $1`, f.e.GetId()); err != nil {
		t.Fatalf("back to the setup: %v", err)
	}
	if _, err := f.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: f.campaignID, EncounterId: f.e.GetId(), CombatantId: f.id(t, "Toren"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("RemoveCombatant() error = %v", err)
	}
	f.wantGivenBack(t, "the removal of the combatant", id)
}

// The combat ends while a Lucky question waits: no die is held, and the hold would stay (or go) with no Inspiration back.
func TestFavorGoesBackWhenTheCombatEndsOnALuckyHold(t *testing.T) {
	t.Parallel()
	f := newHalflingFavorFight(t)
	id := f.spentOnLucky(t)
	if _, err := f.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: f.campaignID, EncounterId: f.e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	f.wantGivenBack(t, "the end of the combat", id)
}

// The combatants that dropCombatants deletes (a creature that leaves, the creatures of a character that goes) take their holds with them
// through the same path: a hold with no answer that spent the Inspiration gives it back.
func TestFavorGoesBackWhenDropCombatantsDeletesAHolder(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	cs, err := f.h.svc.queries.ListCombatants(t.Context(), f.e.GetId())
	if err != nil {
		t.Fatalf("ListCombatants() error = %v", err)
	}
	gi := slices.IndexFunc(cs, func(c playdb.Combatant) bool { return c.Label == "Goblin" })
	if gi < 0 {
		t.Fatal("no goblin in the combat")
	}
	goblin := cs[gi]
	// A creature carries the character of its owner, which the hold needs; it spent the owner's Inspiration (the row is the hold's).
	var id string
	if err := f.h.pool.QueryRow(t.Context(), `INSERT INTO inspiration_holds (campaign_id, character_id, user_id, kind, idempotency_key, request, faces, modifier, total, counted, physical, created_at, round, combatant_id, encounter_id, favor_spent)
		VALUES ($1, $2, (SELECT user_id FROM campaign_members WHERE campaign_id = $1 LIMIT 1), 'weapon_attack', $3, '\x'::bytea, '{4,15}', 5, 20, 1, false, now(), 1, $4, $5, true) RETURNING id`,
		f.campaignID, f.toren.GetId(), newKey(), goblin.ID, f.e.GetId()).Scan(&id); err != nil {
		t.Fatalf("hold the creature's roll: %v", err)
	}
	if f.favorHolds(t) {
		t.Fatal("Toren has the Inspiration before it is given back")
	}
	err = f.h.svc.tx(t.Context(), f.sessionID, func(tx pgx.Tx, _ *hints) error {
		q := f.h.svc.queries.WithTx(tx)
		session, err := q.GetOpenGameSession(t.Context(), f.campaignID)
		if err != nil {
			return err
		}
		c := &combatTx{tx: tx, q: q, session: session, enc: playdb.Encounter{ID: f.e.GetId(), Round: f.e.GetRound()}, now: f.h.svc.now(), svc: f.h.svc}
		_, _, err = f.h.svc.dropCombatants(t.Context(), c, cs, []playdb.Combatant{goblin}, false)
		return err
	})
	if err != nil {
		t.Fatalf("dropCombatants() error = %v", err)
	}
	f.wantGivenBack(t, "dropCombatants", id)
}

// A hold that was answered has nothing to give back (its roll happened), and a grant that came after the spend is not stacked: the
// character keeps the one row.
func TestFavorForgottenHoldGivesNothingWhenAnsweredAndNeverStacks(t *testing.T) {
	t.Parallel()
	t.Run("answered", func(t *testing.T) {
		t.Parallel()
		f := newFavorFight(t)
		id := f.spentOnDie(t)
		if _, err := f.h.pool.Exec(t.Context(), `UPDATE inspiration_holds SET answer_key = $2 WHERE id = $1`, id, newKey()); err != nil {
			t.Fatalf("answer the hold: %v", err)
		}
		if _, err := f.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: f.campaignID, EncounterId: f.e.GetId(), IdempotencyKey: newKey()})); err != nil {
			t.Fatalf("EndEncounter() error = %v", err)
		}
		if f.holdExists(t, id) || f.favorHolds(t) {
			t.Errorf("the answered hold is there = %v, the Inspiration is there = %v, want the hold gone and nothing given back", f.holdExists(t, id), f.favorHolds(t))
		}
	})
	t.Run("a grant after the spend", func(t *testing.T) {
		t.Parallel()
		f := newFavorFight(t)
		id := f.spentOnDie(t)
		f.give(t, f.toren) // the master grants another one while the roll waits
		if _, err := f.endTurn(t, f.master, f.e, true); err != nil {
			t.Fatalf("EndTurn() error = %v", err)
		}
		f.e = f.passTo(t, f.e, "Toren")
		f.h.roller.queue(7)
		if _, err := favorPaths[0].send(t, f, f.caio, newKey(), false, nil); err != nil {
			t.Fatalf("the next attack error = %v", err)
		}
		if f.holdExists(t, id) || f.rows(t) != 1 {
			t.Errorf("the hold is there = %v, the Inspiration rows = %d, want the hold gone and one row", f.holdExists(t, id), f.rows(t))
		}
	})
}

// The held roll of a group check is found by the key AND its roller: member B, under the key member A's roll is held with, is not given
// A's hold (and A's Inspiration), and spends its own.
func TestFavorHoldIsScopedToItsRoller(t *testing.T) {
	t.Parallel()
	f := newFavorFight(t)
	key := newKey()
	f.give(t, f.toren)
	f.give(t, f.pens)
	if _, err := f.h.pool.Exec(t.Context(), `INSERT INTO inspiration_holds (campaign_id, character_id, user_id, kind, idempotency_key, request, faces, modifier, total, counted, physical, created_at, favor_spent)
		VALUES ($1, $2, (SELECT user_id FROM campaign_members WHERE campaign_id = $1 LIMIT 1), 'group_check', $3, '\x'::bytea, '{4,15}', 2, 17, 1, false, now(), true)`, f.campaignID, f.toren.GetId(), key); err != nil {
		t.Fatalf("hold Toren's roll: %v", err)
	}
	ask := favorAsk{Want: true, CampaignID: f.campaignID, CharacterID: f.pens.GetId(), Own: true, Key: key, Kind: heldGroupCheck, Rolls: true}
	env := f.h.svc.outsideGate(nil, playdb.New(f.h.pool))
	got, err := f.h.svc.useFavor(t.Context(), env, ask)
	if err != nil || !got.Spent || got.Used != true {
		t.Fatalf("Pensantus's spend under Toren's key = %+v, %v, want it spent by Pensantus (Toren's hold is not hers)", got, err)
	}
	var left []string
	rows, err := f.h.pool.Query(t.Context(), `SELECT character_id FROM character_favor WHERE campaign_id = $1`, f.campaignID)
	if err != nil {
		t.Fatalf("read the Inspiration rows: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		left = append(left, id)
	}
	if !slices.Equal(left, []string{f.toren.GetId()}) {
		t.Errorf("the Inspiration rows left = %v, want only Toren's: Pensantus spent her own", left)
	}
}
