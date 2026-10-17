package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The weapon attack waits in inspiration_holds since E4-6 and waited in roll_holds before: the holds the release before
// left open there (made by it, answered by this one) are still read, answered and counted by the same path. Nothing
// writes a roll_holds row any more, so these tests build the row the release before wrote (heldLegacyMove) and
// answer it through the RPCs that exist.

// heldLegacyMove moves the open weapon attacks of a combat from inspiration_holds to roll_holds, as the release before
// E4-6 kept them: the same id, request and answers; the d20 that counts as the face; the key of a hold a reaction
// window made with the suffix that said so.
func heldLegacyMove(t *testing.T, h *harness, encounterID string) int {
	t.Helper()
	ctx := t.Context()
	rows, err := playdb.New(h.pool).ListOpenAttackHolds(ctx, &encounterID)
	if err != nil {
		t.Fatalf("list the held attacks: %v", err)
	}
	for _, row := range rows {
		held := heldOfInspirationHold(row)
		key := held.Key
		if held.Window {
			key += windowHoldSuffix
		}
		tx, err := h.pool.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO roll_holds
			(id, encounter_id, combatant_id, idempotency_key, request, face, modifier, round, answer_key, created_at, lucky_index, lucky_answer_key, lucky_to)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			held.ID, held.EncounterID, held.CombatantID, key, held.Request, held.Faces[held.Counted], held.Modifier, row.Round,
			held.AnswerKey, row.CreatedAt, held.LuckyIndex, held.LuckyAnswerKey, held.LuckyTo); err != nil {
			t.Fatalf("keep the attack in roll_holds: %v", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM inspiration_holds WHERE id = $1`, row.ID); err != nil {
			t.Fatalf("forget the attack in inspiration_holds: %v", err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	return len(rows)
}

const countHeldAttacksInInspirationHolds = `SELECT count(*) FROM inspiration_holds WHERE encounter_id = $1 AND kind = 'weapon_attack'`

// heldLegacyRig is attackRig with the weapon attack kept in roll_holds, as the release before E4-6 kept it: every roll
// that holds is moved there at once, so the rest of the story (the retry by the key, the answers, the settled roll)
// reads and writes the legacy table.
func heldLegacyRig(t *testing.T, die bool) heldRig {
	t.Helper()
	a := newLuckyTable(t)
	var e *playv1.Encounter
	if die {
		e = a.luckyInspired(t)
	} else {
		e = a.luckyFight(t)
	}
	return heldRig{
		roller: a.h.roller,
		// As the weapon attack always did: the request made again once the roll is written is refused.
		refusesTheRollReplay: true,
		roll: func(t *testing.T, key string) (heldView, error) {
			t.Helper()
			res, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", func(r *playv1.RollAttackRequest) { inAppRoll(r); r.IdempotencyKey = key })
			if err != nil {
				return heldView{}, err
			}
			heldLegacyMove(t, a.h, e.GetId())
			return attackView(res), nil
		},
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
		holds: func(t *testing.T) int { t.Helper(); return countRows(t, a.h, countRollHolds, e.GetId()) },
	}
}

// TestAWeaponAttackKeptInRollHoldsRetriesAndAnswersTheSame: the five flows of
// TestEveryKindRetriesTheRollAndTheAnswerToTheSameResult, for a weapon attack the release before left in roll_holds:
// the roll again finds its question by the key, the Lucky and Bardic Inspiration answers settle it, a repeated answer
// gives the same result and another is refused. It is what the rollback target of this release relies on.
func TestAWeaponAttackKeptInRollHoldsRetriesAndAnswersTheSame(t *testing.T) {
	t.Parallel()
	for _, flow := range heldFlows {
		t.Run(flow.name, func(t *testing.T) {
			t.Parallel()
			walkHeldFlow(t, heldLegacyRig(t, flow.die), flow.face, flow.answers)
		})
	}
}

// legacyFight is Pipo's turn with Orla's die on her, and the attack that waits for the answer about it, moved to
// roll_holds.
func legacyFight(t *testing.T) (*armed, *playv1.Encounter, *playv1.RollAttackResponse) {
	t.Helper()
	a := newLuckyTable(t)
	e := a.luckyInspired(t)
	held, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", d20(14))
	if err != nil || held.GetInspirationOffer().GetHoldId() == "" {
		t.Fatalf("Pipo's attack = %v, %v; want the question about the die", held, err)
	}
	if n := heldLegacyMove(t, a.h, e.GetId()); n != 1 {
		t.Fatalf("moved %d held attacks, want 1", n)
	}
	return a, e, held
}

// TestAWeaponAttackKeptInRollHoldsIsInTheCombatView: the question is on its combatant for its player and for the
// master, and for nobody else (RN-10).
func TestAWeaponAttackKeptInRollHoldsIsInTheCombatView(t *testing.T) {
	t.Parallel()
	a, e, held := legacyFight(t)
	want := held.GetInspirationOffer().GetHoldId()
	for name, u := range map[string]*user{"its player": a.caio, "the master": a.master} {
		if got := byLabel(t, a.get(t, u), "Pipo").GetInspirationOffer().GetHoldId(); got != want {
			t.Errorf("%s reads the question %q on Pipo, want %q", name, got, want)
		}
	}
	if got := byLabel(t, a.get(t, a.ana), "Pipo").GetInspirationOffer(); got != nil {
		t.Errorf("another player reads %v on Pipo, want none", got)
	}
	if n := countRows(t, a.h, countRollHolds, e.GetId()); n != 1 {
		t.Errorf("%d held attacks in roll_holds, want 1", n)
	}
}

// TestAWeaponAttackKeptInRollHoldsRefusesTheNextRoll: one question at a time reads the legacy table too, and the
// refused attack writes nothing in the new one.
func TestAWeaponAttackKeptInRollHoldsRefusesTheNextRoll(t *testing.T) {
	t.Parallel()
	a, e, _ := legacyFight(t)
	a.h.roller.queue(9)
	_, err := a.attack(t, a.caio, e, "Pipo", battleaxe, "Goblin", inAppRoll)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_INSPIRATION_PENDING)
	if n := a.h.roller.pending(); n != 1 {
		t.Errorf("the refused attack took %d faces from the roller, want none", 1-n)
	}
	a.h.roller.drain()
	if n := countRows(t, a.h, countHeldAttacksInInspirationHolds, e.GetId()); n != 0 {
		t.Errorf("%d held attacks in inspiration_holds, want none", n)
	}
}

// TestAWeaponAttackKeptInRollHoldsIsAnsweredByTheCombatRPCs: the RPC of the rolls out of a combat does not find it,
// another player may not answer it, and the master may, as for any attack: the roll settles on its d20, the row is
// closed in the table it is in and nothing is written in the other.
func TestAWeaponAttackKeptInRollHoldsIsAnsweredByTheCombatRPCs(t *testing.T) {
	t.Parallel()
	a, e, held := legacyFight(t)
	hold := held.GetInspirationOffer().GetHoldId()
	if _, err := a.answerOutside(t, a.caio, hold, false, false, 0); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("AnswerOutsideInspiration() on an attack = %v, want not_found", err)
	}
	if _, err := a.answer(t, a.ana, e, hold, keepTheDie); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("another player's answer = %v, want permission_denied", err)
	}
	a.h.afterThePlayersChance()
	res, err := a.answer(t, a.master, e, hold, keepTheDie)
	if err != nil {
		t.Fatalf("the master's answer: error = %v", err)
	}
	if got := res.GetAttack().GetRoll().GetD20().GetFaces(); len(got) != 1 || got[0] != 14 {
		t.Errorf("the settled attack d20 = %v, want 14", got)
	}
	var answered bool
	if err := a.h.pool.QueryRow(t.Context(), `SELECT answer_key IS NOT NULL FROM roll_holds WHERE id = $1`, hold).Scan(&answered); err != nil || !answered {
		t.Errorf("the hold answered = %v, %v; want it closed in roll_holds", answered, err)
	}
	if n := countRows(t, a.h, countHeldAttacksInInspirationHolds, e.GetId()); n != 0 {
		t.Errorf("%d rows in inspiration_holds, want none", n)
	}
}

// TestAWeaponAttackKeptInRollHoldsAfterAWindowIsAnswered: the attack a reaction window held, whose Lucky question
// the release before kept in roll_holds under the request's key with the window's suffix, is answered by this one:
// the attack is resolved once, under the answer's key.
func TestAWeaponAttackKeptInRollHoldsAfterAWindowIsAnswered(t *testing.T) {
	t.Parallel()
	var raider *charactersv1.Character
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		a.pens = loreBard(t, a)
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3, abilities(10, 16, 14, 16, 8), []string{maceKey}, []string{sacredFlame}, nil, []string{cureWounds})
		raider = halflingRaider(t, a)
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: raider.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Toren": 18, "Reator": 10, "Brisa": 1},
		reveal:   []string{"Salteador"},
		at:       map[string][2]int32{"Toren": {3, 3}, "Salteador": {4, 3}, "Reator": {4, 4}, "Brisa": {8, 8}},
	})
	e = a.passTo(t, e, "Salteador")
	key := newKey()
	attack := func(r *playv1.RollAttackRequest) { d20(1)(r); r.IdempotencyKey = key }
	a.mustAttack(t, a.master, e, "Salteador", battleaxe, "Toren", attack)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CUTTING_WORDS)
	if w == nil {
		t.Fatal("the bard has no window")
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), func(r *playv1.AnswerReactionRequest) {
		r.Answer, r.Roll = playv1.ReactionChoice_REACTION_CHOICE_USE, &playv1.AnswerReactionRequest_Typed{Typed: 3}
	})
	if n := heldLegacyMove(t, a.h, e.GetId()); n != 1 {
		t.Fatalf("moved %d held attacks, want the window's question", n)
	}
	var holdKey string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT idempotency_key FROM roll_holds WHERE encounter_id = $1`, e.GetId()).Scan(&holdKey); err != nil || holdKey != key+windowHoldSuffix {
		t.Fatalf("the legacy hold's key = %q (%v), want the request's with %q", holdKey, err, windowHoldSuffix)
	}
	offer := byLabel(t, a.get(t, a.master), "Salteador").GetLuckyOffer()
	if offer == nil {
		t.Fatal("the raider's Lucky question is not in the combat view")
	}
	res, err := a.answerLucky(t, a.master, e, offer.GetHoldId(), rerollTyped(15))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	roll := res.GetAttack().GetRoll()
	if roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT || roll.GetD20().GetFaces()[0] != 15 {
		t.Errorf("the attack = %v, want a hit with the new 15", roll)
	}
	if n := countRows(t, a.h, countHeldAttacksInInspirationHolds, e.GetId()); n != 0 {
		t.Errorf("%d rows in inspiration_holds, want none", n)
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
