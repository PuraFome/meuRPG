package play

import (
	"encoding/json"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A roll that waits (Lucky, Bardic Inspiration, a reaction window) keeps the trade it was made with: the answer settles on
// the penalty, the damage and the resource the roll was held with, whatever the master edits in the feature meanwhile.

// retune edits the heavy trade's feature in the table, as the master does between the hold and the answer.
func (g *powerTable) retune(t *testing.T, penalty, damage int32) {
	t.Helper()
	sub, _ := proto.Clone(g.entry.GetTableSubclass()).(*rulesv1.TableSubclass)
	for _, f := range sub.GetLevels()[0].GetFeatures() {
		if f.GetNamePt() != powerHeavyName {
			continue
		}
		for _, e := range f.GetEffects() {
			e.AttackPenalty, e.DamageBonus = penalty, damage
		}
	}
	res, err := g.master.table.UpdateTableEntry(t.Context(), connect.NewRequest(&rulesv1.UpdateTableEntryRequest{
		CampaignId: g.campaignID, Key: g.subKey, ExpectedRevision: g.entry.GetRevision(),
		Body: &rulesv1.UpdateTableEntryRequest_TableSubclass{TableSubclass: sub},
	}))
	if err != nil {
		t.Fatalf("UpdateTableEntry() error = %v", err)
	}
	g.entry = res.Msg.GetEntry()
}

// heavyOption is the heavy trade as the greatsword's option lists it now.
func (g *powerTable) heavyOption(t *testing.T, e *playv1.Encounter) *rulesv1.PowerAttackOption {
	t.Helper()
	for _, o := range g.mustOptions(t, g.caio, e, "Pipo").GetOptions().GetAttacks() {
		if o.GetAttack().GetKey() != greatsword {
			continue
		}
		for _, p := range o.GetPowerAttacks() {
			if p.GetName() == powerHeavyName {
				return p
			}
		}
	}
	t.Fatal("the greatsword lists no heavy trade")
	return nil
}

// lastAttackEvent is the stored event of the newest attack: its modifier and its trade.
func (g *powerTable) lastAttackEvent(t *testing.T) (modifier int32, trade *powerAttackEvent) {
	t.Helper()
	var payload string
	if err := g.h.pool.QueryRow(t.Context(), `SELECT payload::TEXT FROM session_events WHERE kind = 'attack_rolled' AND payload ? 'total' ORDER BY seq DESC LIMIT 1`).Scan(&payload); err != nil {
		t.Fatalf("read the attack's event: %v", err)
	}
	var ev struct {
		Modifier    int32             `json:"modifier"`
		PowerAttack *powerAttackEvent `json:"power_attack"`
	}
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		t.Fatalf("read the attack's event %s: %v", payload, err)
	}
	return ev.Modifier, ev.PowerAttack
}

// spendPool uses n of the trades' pool, as the table's rests and other attacks would.
func (g *powerTable) spendPool(t *testing.T, n int) {
	t.Helper()
	tag, err := g.h.pool.Exec(t.Context(), `INSERT INTO character_vitals (character_id, resources_used, revision, updated_at) VALUES ($1, jsonb_build_object($2::TEXT, $3::INT), 1, now())
		ON CONFLICT (character_id) DO UPDATE SET resources_used = character_vitals.resources_used || jsonb_build_object($2::TEXT, $3::INT)`, g.toren.GetId(), powerUses, n)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("spend the pool: %v, rows %d", err, tag.RowsAffected())
	}
}

func TestAHeldRollKeepsTheTradeItWasHeldWithWhenTheFeatureChangesBeforeTheAnswer(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	held, err := g.swing(t, e, g.heavyKey, 1)
	if err != nil || held.GetHeldRollOffer().GetLucky() == nil {
		t.Fatalf("RollAttack() = %v, %v; want Lucky's question", held, err)
	}
	g.retune(t, 2, 4)
	if p := g.heavyOption(t, e); p.GetPenalty() != 2 || p.GetDamageBonus() != 4 {
		t.Fatalf("the option lists %v after the edit, want 2 and 4: the edit did not reach the sheet", p)
	}
	res, err := g.answerLucky(t, g.caio, e, held.GetHeldRollOffer().GetHoldId(), rerollTyped(15))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	const bonus = pipoToHit - heavyPenalty
	roll := res.GetAttack().GetRoll()
	if d := roll.GetD20(); d.GetModifier() != bonus || d.GetTotal() != 15+bonus || roll.GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Errorf("the answer = modifier %d, total %d, outcome %v; want %d, %d and a hit: the penalty it was held with", d.GetModifier(), d.GetTotal(), roll.GetOutcome(), bonus, 15+bonus)
	}
	if modifier, trade := g.lastAttackEvent(t); modifier != bonus || trade == nil || trade.Penalty != heavyPenalty || trade.DamageBonus != heavyDamage || trade.Name != powerHeavyName {
		t.Errorf("the event = modifier %d, trade %+v; want %d and the numbers it was held with (%d, %d)", modifier, trade, bonus, heavyPenalty, heavyDamage)
	}
	pending := res.GetAttack().GetPendingDamage()
	wantPowerPart(t, "the damage the answer opened", pending.GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
	if rolled := g.rolled(t, e, pending.GetId(), 7); rolled.GetAmount() != 7+3+heavyDamage {
		t.Errorf("the damage = %d, want %d: the dice, the modifier and the 10 it was held with", rolled.GetAmount(), 7+3+heavyDamage)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

func TestAReplayAfterAReactionWindowKeepsTheTradeTheRequestWasHeldWith(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	g.setRules(t, enemyReactionsAlways)
	e := g.fight(t)
	held, err := g.swing(t, e, g.heavyKey, 14)
	if err != nil || !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("RollAttack() = %v, %v; want it held for the master's check", held, err)
	}
	w := g.windowOf(t, g.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK)
	if w == nil {
		t.Fatal("the master has no check")
	}
	g.retune(t, 2, 4)
	g.mustAnswer(t, g.master, e, w.GetId(), passAnswer)
	if w2 := g.windowOf(t, g.master, playv1.ReactionKind_REACTION_KIND_MASTER_CHECK); w2 != nil {
		g.mustAnswer(t, g.master, e, w2.GetId(), passAnswer)
	}
	const bonus = pipoToHit - heavyPenalty
	if modifier, trade := g.lastAttackEvent(t); modifier != bonus || trade == nil || trade.Penalty != heavyPenalty || trade.DamageBonus != heavyDamage {
		t.Errorf("the replayed event = modifier %d, trade %+v; want %d and the numbers the request was held with (%d, %d)", modifier, trade, bonus, heavyPenalty, heavyDamage)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

// Only the server pins a trade: a caller that sends the pin gets the sheet's numbers.
func TestACallerCannotPinItsOwnPowerAttack(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	res, err := g.swing(t, e, g.heavyKey, 14, func(r *playv1.RollAttackRequest) {
		r.PowerAttackPin = &playv1.PowerAttackPin{NamePt: "Outro", Penalty: 1, DamageBonus: 20}
	})
	if err != nil {
		t.Fatalf("RollAttack() error = %v", err)
	}
	if d := res.GetRoll().GetD20(); d.GetModifier() != pipoToHit-heavyPenalty {
		t.Errorf("the modifier = %d, want %d: the sheet's penalty", d.GetModifier(), pipoToHit-heavyPenalty)
	}
	wantPowerPart(t, "the damage", res.GetPendingDamage().GetParts(), powerHeavyName, g.heavyKey, heavyDamage)
}

func TestATradeWithNoUseLeftIsListedDisabled(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	if p := g.heavyOption(t, e); p.GetDisabled() || p.GetDisabledReasonPt() != "" {
		t.Fatalf("the heavy trade with uses left = %v, want it enabled", p)
	}
	g.spendPool(t, 2)
	p := g.heavyOption(t, e)
	if !p.GetDisabled() || p.GetDisabledReasonPt() == "" {
		t.Errorf("the heavy trade with no use left = %v, want it disabled with a reason", p)
	}
	for _, o := range g.mustOptions(t, g.caio, e, "Pipo").GetOptions().GetAttacks() {
		for _, q := range o.GetPowerAttacks() {
			if q.GetName() == powerFreeName && (q.GetDisabled() || q.GetDisabledReasonPt() != "") {
				t.Errorf("the free trade (no resource) = %v, want it enabled", q)
			}
		}
	}
	_, err := g.swing(t, e, g.heavyKey, 14)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT)
}

// A maneuver die and the trade on one pool spend two uses: with one left the attack is refused up front, writing and
// spending nothing; with two it goes and spends both.
func TestAManeuverDieAndATradeOnOnePoolNeedTwoUses(t *testing.T) {
	t.Parallel()
	g := newPowerTableWith(t, true)
	e := g.fight(t)
	dice := g.toren.GetDerived().GetRollDice()
	if len(dice) != 1 {
		t.Fatalf("Pipo's dice = %v, want the attack die", dice)
	}
	both := func(r *playv1.RollAttackRequest) { r.ManeuverKey, r.ExtraDieFaces = dice[0].GetKey(), []int32{3} }
	g.spendPool(t, 1)
	events := g.eventCount(t)
	_, err := g.swing(t, e, g.heavyKey, 14, both)
	wantResourceBlocked(t, err, playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT)
	if got, left := g.eventCount(t), g.usesLeft(t); got != events || left != 1 {
		t.Errorf("events = %d (want %d), uses left = %d (want 1): the refused attack wrote or spent", got, events, left)
	}
	if res, err := g.swing(t, e, g.heavyKey, 14); err != nil || res.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Fatalf("the trade alone with one use left = %v, %v; want it to go", res, err)
	}

	g2 := newPowerTableWith(t, true)
	e2 := g2.fight(t)
	if _, err := g2.swing(t, e2, g2.heavyKey, 14, both); err != nil {
		t.Fatalf("both with two uses: error = %v", err)
	}
	if got := g2.usesLeft(t); got != 0 {
		t.Errorf("uses left = %d, want 0: the die and the trade spent one each", got)
	}
}

// The client's retry of the request that made a power attack's roll wait (same key, no pin) is the request the answer wrote
// the attack from: it gives the written attack back and spends nothing more.
func TestARetryOfAHeldPowerAttackAfterItsAnswerGivesTheWrittenAttack(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	key := newKey()
	send := func() (*playv1.RollAttackResponse, error) {
		return g.swing(t, e, g.heavyKey, 1, func(r *playv1.RollAttackRequest) { r.IdempotencyKey = key })
	}
	held, err := send()
	if err != nil || held.GetHeldRollOffer().GetLucky() == nil {
		t.Fatalf("RollAttack() = %v, %v; want Lucky's question", held, err)
	}
	answer, err := g.answerLucky(t, g.caio, e, held.GetHeldRollOffer().GetHoldId(), rerollTyped(15))
	if err != nil {
		t.Fatalf("AnswerLucky() error = %v", err)
	}
	again, err := send()
	if err != nil || again.GetRoll().GetD20().GetTotal() != answer.GetAttack().GetRoll().GetD20().GetTotal() || again.GetRoll().GetD20().GetTotal() != 15+pipoToHit-heavyPenalty {
		t.Errorf("the request again = %v, %v; want the attack the answer wrote", again.GetRoll().GetD20(), err)
	}
	if got := g.usesLeft(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

// The feature itself is read from the sheet again on the answer: one removed while the roll waits refuses the answer, and
// nothing is written or spent.
func TestAHeldPowerAttackWhoseFeatureWasRemovedBeforeTheAnswerIsRefused(t *testing.T) {
	t.Parallel()
	g := newPowerTable(t)
	e := g.fight(t)
	held, err := g.swing(t, e, g.heavyKey, 1)
	if err != nil || held.GetHeldRollOffer().GetLucky() == nil {
		t.Fatalf("RollAttack() = %v, %v; want Lucky's question", held, err)
	}
	sub, _ := proto.Clone(g.entry.GetTableSubclass()).(*rulesv1.TableSubclass)
	level := sub.GetLevels()[0]
	level.Features = slices.DeleteFunc(level.Features, func(f *rulesv1.TableFeature) bool { return f.GetNamePt() == powerHeavyName })
	if _, err := g.master.table.UpdateTableEntry(t.Context(), connect.NewRequest(&rulesv1.UpdateTableEntryRequest{
		CampaignId: g.campaignID, Key: g.subKey, ExpectedRevision: g.entry.GetRevision(),
		Body: &rulesv1.UpdateTableEntryRequest_TableSubclass{TableSubclass: sub},
	})); err != nil {
		t.Fatalf("UpdateTableEntry() error = %v", err)
	}
	events := g.eventCount(t)
	_, err = g.answerLucky(t, g.caio, e, held.GetHeldRollOffer().GetHoldId(), rerollTyped(15))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("AnswerLucky() error = %v, want invalid_argument: the feature is gone", err)
	}
	if got := g.eventCount(t); got != events {
		t.Errorf("events = %d, want %d: the refused answer wrote one", got, events)
	}
	if got := g.usesLeft(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}
