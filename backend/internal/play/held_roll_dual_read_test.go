package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The weapon attack waits in roll_holds today, and a later release writes it in inspiration_holds
// (migration 00255 added the columns). This release reads both, so that the release after it can write the new
// table and still be rolled back to this one. Nothing writes a weapon_attack row yet: these tests build
// the row that release will write (heldDualMove) and answer it through the RPCs that exist, by the same paths.

// heldDualMove moves the open weapon attacks of a combat from roll_holds to inspiration_holds, as the
// release that writes them there will keep them: the same id, request and answers; the character and the user
// of the combatant; the combatant and the combat that own the hold.
func heldDualMove(t *testing.T, h *harness, encounterID string) int {
	t.Helper()
	ctx := t.Context()
	rows, err := playdb.New(h.pool).ListOpenRollHolds(ctx, encounterID)
	if err != nil {
		t.Fatalf("list the held attacks: %v", err)
	}
	for _, row := range rows {
		held := heldOfRollHold(row)
		var characterID, userID string
		if err := h.pool.QueryRow(ctx, `SELECT character_id, user_id FROM combatants WHERE id = $1`, row.CombatantID).Scan(&characterID, &userID); err != nil {
			t.Fatalf("read the attacker: %v", err)
		}
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO inspiration_holds
			(id, campaign_id, character_id, user_id, kind, idempotency_key, request, faces, modifier, total, counted, physical, answer_key,
			 created_at, round, lucky_index, lucky_answer_key, lucky_to, combatant_id, encounter_id, window_hold)
			VALUES ($1, $2, $3, $4, 'weapon_attack', $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
			held.ID, held.CampaignID, characterID, userID, held.Key, held.Request, faces32(held.Faces), held.Modifier, held.Total, held.Counted, held.Physical,
			held.AnswerKey, row.CreatedAt, row.Round, held.LuckyIndex, held.LuckyAnswerKey, held.LuckyTo, row.CombatantID, row.EncounterID, held.Window); err != nil {
			t.Fatalf("keep the attack in inspiration_holds: %v", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM roll_holds WHERE id = $1`, row.ID); err != nil {
			t.Fatalf("forget the attack in roll_holds: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	return len(rows)
}

const countHeldAttacksInInspirationHolds = `SELECT count(*) FROM inspiration_holds WHERE encounter_id = $1 AND kind = 'weapon_attack'`

// heldDualRig is attackRig with the weapon attack kept in inspiration_holds: every roll that holds is moved at once, so
// the rest of the story (the retry, the answers, the settled roll) reads and writes the new table.
func heldDualRig(t *testing.T, die bool) heldRig {
	t.Helper()
	a := newLuckyTable(t)
	var e *playv1.Encounter
	if die {
		e = a.luckyInspired(t)
	} else {
		e = a.luckyFight(t)
	}
	attack := func(t *testing.T, key string) (heldView, error) {
		t.Helper()
		res, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { inAppRoll(r); r.IdempotencyKey = key })
		if err != nil {
			return heldView{}, err
		}
		heldDualMove(t, a.h, e.GetId())
		return attackView(res), nil
	}
	return heldRig{
		roller: a.h.roller,
		// As the weapon attack in roll_holds: the request made again once the roll is written is refused.
		refusesTheRollReplay: true,
		roll:                 attack,
		answerLucky: func(t *testing.T, holdID, key string, reroll bool) (heldView, error) {
			t.Helper()
			res, err := a.answerLucky(t, a.caio, e, holdID, func(r *playv1.AnswerLuckyRequest) {
				if reroll {
					rerollInApp(r)
				} else {
					keepTheOne(r)
				}
				r.IdempotencyKey = key
			})
			if err != nil {
				return heldView{}, err
			}
			return attackView(res.GetAttack()), nil
		},
		answerDie: func(t *testing.T, holdID, key string, use bool) (heldView, error) {
			t.Helper()
			res, err := a.answer(t, a.caio, e, holdID, func(r *playv1.AnswerBardicInspirationRequest) {
				if use {
					useDieInApp(r)
				} else {
					keepTheDie(r)
				}
				r.IdempotencyKey = key
			})
			if err != nil {
				return heldView{}, err
			}
			return attackView(res.GetAttack()), nil
		},
		holds: func(t *testing.T) int {
			t.Helper()
			return countRows(t, a.h, countHeldAttacksInInspirationHolds, e.GetId())
		},
	}
}

// TestAWeaponAttackKeptInInspirationHoldsRetriesAndAnswersTheSame: the five flows of
// TestEveryKindRetriesTheRollAndTheAnswerToTheSameResult, for the weapon attack found in the new table: the roll
// again finds its question by the key, the Lucky and Bardic Inspiration answers settle it, a repeated answer gives the
// same result and another is refused.
func TestAWeaponAttackKeptInInspirationHoldsRetriesAndAnswersTheSame(t *testing.T) {
	t.Parallel()
	for _, flow := range heldFlows {
		t.Run(flow.name, func(t *testing.T) {
			t.Parallel()
			walkHeldFlow(t, heldDualRig(t, flow.die), flow.face, flow.answers)
		})
	}
}

// dualFight is Pipo's turn with Orla's die on her, and the attack that waits for the answer about it, moved to the new
// table.
func dualFight(t *testing.T) (*armed, *playv1.Encounter, *playv1.RollAttackResponse) {
	t.Helper()
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
	if err != nil || held.GetInspirationOffer().GetHoldId() == "" {
		t.Fatalf("Pipo's attack = %v, %v; want the question about the die", held, err)
	}
	if n := heldDualMove(t, a.h, e.GetId()); n != 1 {
		t.Fatalf("moved %d held attacks, want 1", n)
	}
	return a, e, held
}

// TestAWeaponAttackKeptInInspirationHoldsIsInTheCombatView: the question is on its combatant for its player and for
// the master, and for nobody else (RN-10).
func TestAWeaponAttackKeptInInspirationHoldsIsInTheCombatView(t *testing.T) {
	t.Parallel()
	a, e, held := dualFight(t)
	want := held.GetInspirationOffer().GetHoldId()
	for name, u := range map[string]*user{"its player": a.caio, "the master": a.master} {
		if got := byLabel(t, a.get(t, u), "Pipo").GetInspirationOffer().GetHoldId(); got != want {
			t.Errorf("%s reads the question %q on Pipo, want %q", name, got, want)
		}
	}
	if got := byLabel(t, a.get(t, a.ana), "Pipo").GetInspirationOffer(); got != nil {
		t.Errorf("another player reads %v on Pipo, want none", got)
	}
	if n := countRows(t, a.h, countHeldAttacksInInspirationHolds, e.GetId()); n != 1 {
		t.Errorf("%d held attacks in inspiration_holds, want 1", n)
	}
}

// TestAWeaponAttackKeptInInspirationHoldsRefusesTheNextRoll: one question at a time reads the new table too.
func TestAWeaponAttackKeptInInspirationHoldsRefusesTheNextRoll(t *testing.T) {
	t.Parallel()
	a, e, _ := dualFight(t)
	a.h.roller.queue(9)
	_, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", inAppRoll)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING)
	if n := a.h.roller.pending(); n != 1 {
		t.Errorf("the refused attack took %d faces from the roller, want none", 1-n)
	}
	a.h.roller.drain()
}

// TestAWeaponAttackKeptInInspirationHoldsIsAnsweredByTheCombatRPCs: the RPC of the rolls out of a combat does not find
// it, another player may not answer it, and the master may, as for any attack: the roll settles on its d20 and the
// row is closed in the table it is in.
func TestAWeaponAttackKeptInInspirationHoldsIsAnsweredByTheCombatRPCs(t *testing.T) {
	t.Parallel()
	a, e, held := dualFight(t)
	hold := held.GetInspirationOffer().GetHoldId()
	if _, err := a.answerOutside(t, a.caio, hold, false, false, 0); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("AnswerOutsideInspiration() on an attack = %v, want not_found", err)
	}
	if _, err := a.answer(t, a.ana, e, hold, keepTheDie); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player's answer = %v, want permission_denied", err)
	}
	res, err := a.answer(t, a.master, e, hold, keepTheDie)
	if err != nil {
		t.Fatalf("the master's answer: error = %v", err)
	}
	if got := res.GetAttack().GetRoll().GetD20().GetFaces(); len(got) != 1 || got[0] != 14 {
		t.Errorf("the settled attack d20 = %v, want 14", got)
	}
	var answered bool
	if err := a.h.pool.QueryRow(t.Context(), `SELECT answer_key IS NOT NULL FROM inspiration_holds WHERE id = $1`, hold).Scan(&answered); err != nil || !answered {
		t.Errorf("the hold answered = %v, %v; want it closed in inspiration_holds", answered, err)
	}
	if n := countRows(t, a.h, countRollHolds, e.GetId()); n != 0 {
		t.Errorf("%d rows in roll_holds, want none", n)
	}
}

// TestAHoldMadeInACombatNamesItsCombatant: a roll held in a combat out of the attack's path (a spell attack) is
// kept for its combatant and its combat, and the character's own list still shows it.
func TestAHoldMadeInACombatNamesItsCombatant(t *testing.T) {
	t.Parallel()
	a, e := brisaFight(t, true)
	a.h.roller.queue(9, 13)
	if _, err := a.castGuidingBolt(t, e); err != nil {
		t.Fatalf("Brisa's Guiding Bolt: error = %v", err)
	}
	var combatant, encounter *string
	var window bool
	if err := a.h.pool.QueryRow(t.Context(), `SELECT combatant_id::TEXT, encounter_id::TEXT, window_hold FROM inspiration_holds WHERE campaign_id = $1`, a.campaignID).
		Scan(&combatant, &encounter, &window); err != nil {
		t.Fatalf("read the hold: %v", err)
	}
	if deref(combatant) != a.id(t, "Brisa") || deref(encounter) != e.GetId() || window {
		t.Errorf("the hold names combatant %q, combat %q, window %v; want Brisa, her combat, no window", deref(combatant), deref(encounter), window)
	}
	if got := a.outsideState(t, a.bia).GetOffers(); len(got) != 1 {
		t.Errorf("Brisa's player reads %d questions, want 1", len(got))
	}
}

// TestAHoldMadeBeforeTheColumnsWereWrittenIsStillTheCharacters: a roll held in a combat by the release before this
// one names no combatant, and the character still owns it: it refuses the next roll and is on her list.
func TestAHoldMadeBeforeTheColumnsWereWrittenIsStillTheCharacters(t *testing.T) {
	t.Parallel()
	a, e := brisaFight(t, true)
	a.h.roller.queue(9, 13)
	if _, err := a.castGuidingBolt(t, e); err != nil {
		t.Fatalf("Brisa's Guiding Bolt: error = %v", err)
	}
	if _, err := a.h.pool.Exec(t.Context(), `UPDATE inspiration_holds SET combatant_id = NULL, encounter_id = NULL WHERE campaign_id = $1`, a.campaignID); err != nil {
		t.Fatalf("make the hold the old release's: %v", err)
	}
	a.h.roller.queue(9, 13)
	_, err := a.castGuidingBolt(t, e)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING)
	if got := a.outsideState(t, a.bia).GetOffers(); len(got) != 1 {
		t.Errorf("Brisa's player reads %d questions, want 1", len(got))
	}
	a.h.roller.drain()
}
