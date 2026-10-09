package play

import (
	"connectrpc.com/connect"
	"testing"

	"google.golang.org/protobuf/proto"

	campaignsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/campaigns/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// "Reações dos inimigos" (PM-04): by default only an enemy that can react opens a
// window, and "Sempre" opens the master's one-tap check on every hit, cast and damage
// of a player against an enemy, so that waiting does not tell the table whether the
// enemy could.

func enemyReactionsAlways(r *campaignsv1.TableRules) {
	r.EnemyReactions = campaignsv1.EnemyReactionsRule_ENEMY_REACTIONS_RULE_ALWAYS
}

// TestTheMastersCheckWaitsOnEveryHitWhenTheTableAsksForIt: with "Sempre" Toren's hit on a
// Goblin that has no reaction at all still waits for the master, who answers with one tap.
// The players read what they read when a real reaction is pending.
func TestTheMastersCheckWaitsOnEveryHitWhenTheTableAsksForIt(t *testing.T) {
	t.Parallel()
	a := newCasters(t)
	e := a.threeAndAGoblin(t)
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(15))
	if hit.GetPendingDamage().GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_ROLL {
		t.Fatalf("by default the hit waits for %v, want the damage roll: the Goblin cannot react", hit.GetPendingDamage().GetStatus())
	}
	if got := a.get(t, a.caio).GetReactionWait(); got != nil {
		t.Errorf("Toren's player reads %v with no reaction pending", got)
	}

	// Second fight, same table, now with "Sempre".
	b := newCasters(t)
	b.setRules(t, enemyReactionsAlways)
	f := b.threeAndAGoblin(t)
	held := b.mustAttack(t, b.caio, f, "Toren", battleaxe, "Goblin", d20(15))
	if held.GetPendingDamage().GetStatus() != playv1.PendingDamageStatus_PENDING_DAMAGE_STATUS_AWAITING_REACTION {
		t.Fatalf("with \"Sempre\" the hit waits for %v, want the reaction", held.GetPendingDamage().GetStatus())
	}
	w := b.windowOf(t, b.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil || !w.GetAnswerNow() || w.GetReactorId() != "" {
		t.Fatalf("the master's window = %v, want the check with no reactor", w)
	}
	if got := b.get(t, b.caio).GetReactionWait().GetTitlePt(); got != "Esperando o mestre" {
		t.Errorf("Toren's player reads %q, want \"Esperando o mestre\"", got)
	}
	for _, u := range []*user{b.caio, b.ana, b.bia} {
		if n := len(b.get(t, u).GetReactionWindows()); n != 0 {
			t.Errorf("a player got %d windows: the check is the master's", n)
		}
	}
	// "Nada a reagir": one tap, and the hit goes on.
	b.mustAnswer(t, b.master, f, w.GetId(), passAnswer)
	if _, err := b.damage(t, b.caio, f, held.GetPendingDamage().GetId(), inAppDamage); err != nil {
		t.Errorf("RollDamage() after the check error = %v", err)
	}
}

// TestWhatThePlayersReadDoesNotTellWhetherTheEnemyCouldReact: "Sempre" on a Goblin and a
// real Shield on a Mago give the attacker the same words and the same answer (RN-10).
func TestWhatThePlayersReadDoesNotTellWhetherTheEnemyCouldReact(t *testing.T) {
	t.Parallel()
	real, e := mageFight(t)
	realHit := real.mustAttack(t, real.caio, e, "Toren", battleaxe, "Mago", d20(9))

	always := newCasters(t)
	always.setRules(t, enemyReactionsAlways)
	f := always.threeAndAGoblin(t)
	maskedHit := always.mustAttack(t, always.caio, f, "Toren", battleaxe, "Goblin", d20(15))

	strip := func(p *playv1.PendingDamage) *playv1.PendingDamage {
		q := proto.Clone(p).(*playv1.PendingDamage)
		q.Id, q.TargetId, q.AttackerId = "", "", ""
		return q
	}
	if !proto.Equal(strip(realHit.GetPendingDamage()), strip(maskedHit.GetPendingDamage())) {
		t.Errorf("the pending damage differs:\n real   %v\n masked %v", realHit.GetPendingDamage(), maskedHit.GetPendingDamage())
	}
	if !proto.Equal(real.get(t, real.caio).GetReactionWait(), always.get(t, always.caio).GetReactionWait()) {
		t.Errorf("the wait differs:\n real   %v\n masked %v", real.get(t, real.caio).GetReactionWait(), always.get(t, always.caio).GetReactionWait())
	}
	if !proto.Equal(real.get(t, real.ana).GetReactionWait(), always.get(t, always.ana).GetReactionWait()) {
		t.Errorf("another player's wait differs")
	}
}

// shieldedHit is Toren hitting the Mago, which holds the hit for its Shield.
func shieldedHit(t *testing.T) (*armed, *playv1.Encounter, *playv1.ReactionWindow) {
	t.Helper()
	a, e := mageFight(t)
	a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Mago", d20(9))
	w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD)
	if w == nil {
		t.Fatal("the Mago has no Shield window")
	}
	return a, e, w
}

// TestTwoAnswersToOneWindowSettleToOne: the master answers the same window twice at the
// same time; one wins and the other is told it came too late, and Shield is spent once.
func TestTwoAnswersToOneWindowSettleToOne(t *testing.T) {
	t.Parallel()
	a, e, w := shieldedHit(t)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := a.answerReaction(t, a.master, e, w.GetId(), useAnswer(1))
			errs <- err
		}()
	}
	var failed, won int
	for range 2 {
		if err := <-errs; err != nil {
			failed++
			wantBlockedBy(t, "the late answer", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NOT_YOUR_TURN_TO_ANSWER)
		} else {
			won++
		}
	}
	if won != 1 || failed != 1 {
		t.Fatalf("answers: %d won, %d failed; want one of each", won, failed)
	}
	var used string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT slots_used::STRING FROM combatants WHERE id = $1`, a.id(t, "Mago")).Scan(&used); err != nil || used != `{"1": 1}` {
		t.Errorf("the Mago's slots used = %q, %v; want one 1st-level slot", used, err)
	}
}

// TestTheEndOfTheCombatDiscardsTheWindows: nothing is left of an unanswered window.
func TestTheEndOfTheCombatDiscardsTheWindows(t *testing.T) {
	t.Parallel()
	a, e, w := shieldedHit(t)
	if _, err := a.master.combat.EndEncounter(t.Context(), connect.NewRequest(&playv1.EndEncounterRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), IdempotencyKey: newKey()})); err != nil {
		t.Fatalf("EndEncounter() error = %v", err)
	}
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM reaction_windows) + (SELECT count(*) FROM reaction_holds)`).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows left = %d, %v; want none", n, err)
	}
	if _, err := a.answerReaction(t, a.master, e, w.GetId(), passAnswer); err == nil {
		t.Error("a window of a finished combat was answered")
	}
}

// TestAWindowClosesByItselfWhenTheReactorIsIncapacitated: the master paralyzes the Mago
// while Toren's hit waits for it; the window closes, the hit goes on to its damage, and
// nobody is asked anything.
func TestAWindowClosesByItselfWhenTheReactorIsIncapacitated(t *testing.T) {
	t.Parallel()
	a, e, _ := shieldedHit(t)
	a.setConditions(t, e, "Mago", "condition:paralyzed")
	if w := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_SHIELD); w != nil {
		t.Fatalf("the paralyzed Mago still has a window: %v", w)
	}
	if got := a.get(t, a.caio).GetReactionWait(); got != nil {
		t.Errorf("Toren's player still waits: %v", got)
	}
	if _, err := a.damage(t, a.caio, e, a.firstPending(t), inAppDamage); err != nil {
		t.Errorf("RollDamage() after the window closed error = %v", err)
	}
}

// firstPending is the id of the only pending damage of the combat.
func (a *armed) firstPending(t *testing.T) string {
	t.Helper()
	var id string
	if err := a.h.pool.QueryRow(t.Context(), `SELECT id FROM pending_damages ORDER BY created_at DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("read the pending damage: %v", err)
	}
	return id
}

// TestAHeldCastAnsweredAgainWithItsKeyHoldsOnce: a client that sends the same cast again (its
// network dropped) gets the same answer and no second window or second hold.
func TestAHeldCastAnsweredAgainWithItsKeyHoldsOnce(t *testing.T) {
	t.Parallel()
	a, e := mageFight(t)
	a.pensantusTurn(t, e)
	key := newKey()
	targets := []*playv1.SpellTarget{darts(a, t, "Goblin", 3)}
	first, err := a.castKey(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), targets, noCastRoll, key)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	again, err := a.castKey(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), targets, noCastRoll, key)
	if err != nil {
		t.Fatalf("CastSpell(again) error = %v", err)
	}
	if !proto.Equal(first.GetCast(), again.GetCast()) || first.GetCast() != nil {
		t.Errorf("the second answer differs: %v / %v", first, again)
	}
	var holds, windows int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM reaction_holds), (SELECT count(*) FROM reaction_windows)`).Scan(&holds, &windows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if holds != 1 || windows < 1 {
		t.Errorf("holds %d, windows %d; want one hold", holds, windows)
	}
}
