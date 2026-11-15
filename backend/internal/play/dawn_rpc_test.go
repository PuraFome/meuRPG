package play

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The dawn of the magic items (SRD 5.1): the master's Amanhecer and the long rest that restores it too. The numbers of each
// item are the characters module's (characters/dawn_test.go); these tests are what play adds: who may ask, the dice the app
// rolls, the events, the key of a retry and who is told.

const (
	dawnEyes   = "item:eyes-of-charming"                       // 3 charges, all back at dawn
	dawnHammer = "item:hammer-of-thunderbolts"                 // 5 charges, 1d4+1 back at dawn
	dawnGoats  = "item:figurine-of-wondrous-power-ivory-goats" // 24 charges, all back on the seventh dawn after the last
)

func (c *charged) dawn(t *testing.T, u *user, edit ...func(*playv1.DawnRequest)) (*playv1.DawnResponse, error) {
	t.Helper()
	req := &playv1.DawnRequest{CampaignId: c.campaignID, IdempotencyKey: newKey()}
	for _, e := range edit {
		e(req)
	}
	res, err := u.charges.Dawn(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// restLong is a long rest of the master's, with what the test edits in the request.
func (c *charged) restLong(t *testing.T, edit ...func(*playv1.TakeRestRequest)) (*playv1.TakeRestResponse, error) {
	t.Helper()
	req := &playv1.TakeRestRequest{CampaignId: c.campaignID, Kind: playv1.RestKind_REST_KIND_LONG, IdempotencyKey: newKey()}
	for _, e := range edit {
		e(req)
	}
	res, err := c.master.resource.TakeRest(t.Context(), connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func withDawn(r *playv1.TakeRestRequest) { r.RestoreDawn = true }

// dawnEvents reads the payloads of the item_charges_restored events of the session, oldest first.
func (c *charged) dawnEvents(t *testing.T) []itemUseEvent {
	t.Helper()
	rows, err := c.h.pool.Query(t.Context(), `SELECT payload FROM session_events WHERE kind = $1 AND game_session_id IN (SELECT id FROM game_sessions WHERE campaign_id = $2) ORDER BY seq`, eventItemChargesRestored, c.campaignID)
	if err != nil {
		t.Fatalf("read the events: %v", err)
	}
	defer rows.Close()
	var out []itemUseEvent
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("scan: %v", err)
		}
		var ev itemUseEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out = append(out, ev)
	}
	return out
}

func (c *charged) leftOf(t *testing.T, key string) int32 {
	t.Helper()
	return c.itemOf(t, c.master, key).GetChargesLeft()
}

func TestTheDawnRestoresTheItemsThatRechargeAndRollsTheDice(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes, dawnHammer, chargeScarab)
	c.mustSpend(t, c.ana, dawnEyes, 3)
	c.mustSpend(t, c.ana, dawnHammer, 2) // 3 left
	c.mustSpend(t, c.ana, chargeScarab, 4)
	c.h.roller.queue(4) // 1d4+1: 5, but only 2 fit
	res, err := c.dawn(t, c.master)
	if err != nil {
		t.Fatalf("Dawn() error = %v", err)
	}
	if got := c.leftOf(t, dawnEyes); got != 3 {
		t.Errorf("the eyes have %d charges, want all 3", got)
	}
	if got := c.leftOf(t, dawnHammer); got != 5 {
		t.Errorf("the hammer has %d charges, want the 5 of its maximum", got)
	}
	if got := c.leftOf(t, chargeScarab); got != 8 {
		t.Errorf("the scarab never recharges: it has %d, want 8", got)
	}
	if len(res.GetRestored()) != 2 {
		t.Fatalf("Dawn() restored %v, want the eyes and the hammer", res.GetRestored())
	}
	byKey := map[string]*playv1.DawnRestored{}
	for _, r := range res.GetRestored() {
		byKey[r.GetItem().GetKey()] = r
		if r.GetCharacterId() != c.pens.GetId() {
			t.Errorf("a restored item of %s, want Pensantus", r.GetCharacterId())
		}
	}
	if byKey[dawnEyes].GetRegained() != 3 || byKey[dawnHammer].GetRegained() != 2 || byKey[dawnHammer].GetItem().GetChargesLeft() != 5 {
		t.Errorf("Dawn() answered %v", res.GetRestored())
	}
	events := c.dawnEvents(t)
	if len(events) != 2 {
		t.Fatalf("the dawn wrote %d item_charges_restored events, want one for each item that changed", len(events))
	}
	for _, ev := range events {
		if ev.Outcome != itemOutcomeRegained || ev.Amount < 1 || ev.Max < 1 {
			t.Errorf("event %+v does not say what the item regained", ev)
		}
	}
}

func TestTheDawnWritesNothingForItemsAtTheirMaximumAndRollsNothing(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes, dawnHammer)
	res, err := c.dawn(t, c.master)
	if err != nil {
		t.Fatalf("Dawn() error = %v", err)
	}
	if len(res.GetRestored()) != 0 || len(c.dawnEvents(t)) != 0 || c.h.roller.given != 0 {
		t.Errorf("a dawn of full items answered %v, wrote %d events and rolled %d dice", res.GetRestored(), len(c.dawnEvents(t)), c.h.roller.given)
	}
}

func TestOnlyTheMasterCallsTheDawn(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes)
	c.mustSpend(t, c.ana, dawnEyes, 1)
	for _, u := range []*user{c.ana, c.caio} {
		_, err := c.dawn(t, u)
		wantCode(t, "a player's Dawn", err, connect.CodePermissionDenied)
	}
	if got := c.leftOf(t, dawnEyes); got != 2 {
		t.Errorf("a refused dawn left the eyes at %d, want 2", got)
	}
}

func TestTheDawnIsIdempotent(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes, dawnHammer)
	c.mustSpend(t, c.ana, dawnEyes, 2)
	c.mustSpend(t, c.ana, dawnHammer, 4)
	c.h.roller.queue(2) // 1d4+1 = 3
	key := newKey()
	withKey := func(r *playv1.DawnRequest) { r.IdempotencyKey = key }
	first, err := c.dawn(t, c.master, withKey)
	if err != nil {
		t.Fatalf("Dawn() error = %v", err)
	}
	given := c.h.roller.given
	c.mustSpend(t, c.ana, dawnEyes, 1)
	again, err := c.dawn(t, c.master, withKey)
	if err != nil {
		t.Fatalf("Dawn() retried error = %v", err)
	}
	if c.h.roller.given != given || len(c.dawnEvents(t)) != 2 {
		t.Errorf("the retry rolled %d more dice and left %d events, want none more and 2", c.h.roller.given-given, len(c.dawnEvents(t)))
	}
	if got := c.leftOf(t, dawnEyes); got != 2 {
		t.Errorf("the retry restored the eyes again: %d left, want the 2 left after the spend", got)
	}
	if len(again.GetRestored()) != len(first.GetRestored()) {
		t.Errorf("the retry answered %v, the first call %v", again.GetRestored(), first.GetRestored())
	}
	_, err = c.master.charges.Dawn(t.Context(), connect.NewRequest(&playv1.DawnRequest{CampaignId: c.campaignID, IdempotencyKey: "nope"}))
	wantCode(t, "a key that is no UUID", err, connect.CodeInvalidArgument)
	_, err = c.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{CampaignId: c.campaignID, Kind: playv1.RestKind_REST_KIND_SHORT, IdempotencyKey: key}))
	wantCode(t, "the dawn's key reused for a rest", err, connect.CodeInvalidArgument)
}

func TestADawnWithNothingToRestoreStillAnswersItsRetryOnce(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnGoats)
	c.mustSpend(t, c.ana, dawnGoats, 24)
	key := newKey()
	for range 3 {
		if _, err := c.dawn(t, c.master, func(r *playv1.DawnRequest) { r.IdempotencyKey = key }); err != nil {
			t.Fatalf("Dawn() error = %v", err)
		}
	}
	// Three calls with one key are one dawn: six more with other keys make seven.
	for range 5 {
		if _, err := c.dawn(t, c.master); err != nil {
			t.Fatalf("Dawn() error = %v", err)
		}
	}
	if got := c.leftOf(t, dawnGoats); got != 0 {
		t.Fatalf("after six dawns the goats have %d charges, want none yet", got)
	}
	res, err := c.dawn(t, c.master)
	if err != nil || len(res.GetRestored()) != 1 || c.leftOf(t, dawnGoats) != 24 {
		t.Fatalf("the seventh dawn = %v, %v; the goats have %d charges, want 24", res, err, c.leftOf(t, dawnGoats))
	}
}

func TestTheDawnNeverRestoresALostItemOrOneWaitingForItsD20(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes, chargeWand)
	if _, err := c.correct(t, c.master, dawnEyes, 0, charactersv1.ItemLoss_ITEM_LOSS_GONE); err != nil {
		t.Fatalf("SetCharges() error = %v", err)
	}
	c.mustSpend(t, c.ana, chargeWand, 7) // the d20 of the last charge waits
	res, err := c.dawn(t, c.master)
	if err != nil || len(res.GetRestored()) != 0 {
		t.Fatalf("Dawn() = %v, %v, want nothing restored", res, err)
	}
	eyes, wand := c.itemOf(t, c.ana, dawnEyes), c.itemOf(t, c.ana, chargeWand)
	if eyes.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_GONE || eyes.GetChargesLeft() != 0 {
		t.Errorf("the lost eyes read %v", eyes)
	}
	if !wand.GetLastRollPending() || wand.GetChargesLeft() != 0 {
		t.Errorf("the wand waiting for its d20 reads %v", wand)
	}
}

func TestALongRestWithTheDawnRestoresInTheSameTransactionAndOnce(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes, dawnHammer)
	c.mustSpend(t, c.ana, dawnEyes, 3)
	c.mustSpend(t, c.ana, dawnHammer, 3)
	c.h.roller.queue(1) // 1d4+1 = 2
	key := newKey()
	withKey := func(r *playv1.TakeRestRequest) { r.IdempotencyKey = key }
	if _, err := c.restLong(t, withDawn, withKey); err != nil {
		t.Fatalf("TakeRest() error = %v", err)
	}
	if c.leftOf(t, dawnEyes) != 3 || c.leftOf(t, dawnHammer) != 4 {
		t.Fatalf("after the rest the eyes have %d and the hammer %d charges, want 3 and 4", c.leftOf(t, dawnEyes), c.leftOf(t, dawnHammer))
	}
	if n := len(c.dawnEvents(t)); n != 2 || c.countEvents(t, eventDawnTaken) != 0 {
		t.Errorf("the rest wrote %d item_charges_restored events and %d dawn_taken, want 2 and 0", n, c.countEvents(t, eventDawnTaken))
	}
	given := c.h.roller.given
	c.mustSpend(t, c.ana, dawnEyes, 1)
	if _, err := c.restLong(t, withDawn, withKey); err != nil {
		t.Fatalf("TakeRest() retried error = %v", err)
	}
	if c.leftOf(t, dawnEyes) != 2 || c.h.roller.given != given || len(c.dawnEvents(t)) != 2 {
		t.Errorf("the retried rest restored again: eyes %d, %d dice more, %d events", c.leftOf(t, dawnEyes), c.h.roller.given-given, len(c.dawnEvents(t)))
	}
}

// failRestEventOf makes the database refuse the rest_taken event of the campaign, the last write of a rest and the one that
// comes after the dawn: a real constraint of the database on a real write, not a stand-in for the code. The returned func
// lifts it.
func (c *charged) failRestEventOf(t *testing.T) (lift func()) {
	t.Helper()
	name := "refuse_rest_" + strings.ReplaceAll(newKey(), "-", "")
	run := func(ctx context.Context, sql string) {
		if _, err := c.h.pool.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	var sessionID string
	if err := c.h.pool.QueryRow(t.Context(), `SELECT id FROM game_sessions WHERE campaign_id = $1`, c.campaignID).Scan(&sessionID); err != nil {
		t.Fatalf("read the session: %v", err)
	}
	run(t.Context(), `CREATE FUNCTION `+name+`() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	-- CockroachDB reads a field of NEW only inside parentheses.
	IF (NEW).kind = '`+eventRestTaken+`' AND (NEW).game_session_id = '`+sessionID+`' THEN
		RAISE EXCEPTION 'the rest event is refused';
	END IF;
	RETURN NEW;
END $$`)
	run(t.Context(), `CREATE TRIGGER `+name+` BEFORE INSERT ON session_events FOR EACH ROW EXECUTE FUNCTION `+name+`()`)
	lifted := false
	lift = func() {
		if lifted {
			return
		}
		lifted = true
		ctx := context.WithoutCancel(t.Context())
		run(ctx, `DROP TRIGGER `+name+` ON session_events`)
		run(ctx, `DROP FUNCTION `+name+`()`)
	}
	t.Cleanup(lift)
	return lift
}

// The dawn of a long rest is part of its transaction: when a later write of the rest fails, the charges and the
// item_charges_restored events the dawn wrote are rolled back with it, and the same request taken again restores them once.
func TestALongRestThatFailsAfterTheDawnRollsTheDawnBack(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes, dawnHammer)
	c.mustSpend(t, c.ana, dawnEyes, 3)
	c.mustSpend(t, c.ana, dawnHammer, 3)
	lift := c.failRestEventOf(t)
	key := newKey()
	withKey := func(r *playv1.TakeRestRequest) { r.IdempotencyKey = key }
	c.h.roller.queue(1, 1) // 1d4+1 = 2, for each attempt
	given := c.h.roller.given
	if _, err := c.restLong(t, withDawn, withKey); err == nil {
		t.Fatal("TakeRest() succeeded although the database refused the rest event")
	}
	if c.h.roller.given == given {
		t.Fatal("the dawn rolled no die before the rest failed: the failure came before the step under test")
	}
	if eyes, hammer := c.leftOf(t, dawnEyes), c.leftOf(t, dawnHammer); eyes != 0 || hammer != 2 {
		t.Errorf("after the failed rest the eyes have %d and the hammer %d charges, want 0 and 2", eyes, hammer)
	}
	if n, rests := len(c.dawnEvents(t)), c.countEvents(t, eventRestTaken); n != 0 || rests != 0 {
		t.Errorf("the failed rest left %d item_charges_restored and %d rest_taken events, want none", n, rests)
	}
	lift()
	if _, err := c.restLong(t, withDawn, withKey); err != nil {
		t.Fatalf("TakeRest() again error = %v", err)
	}
	if eyes, hammer := c.leftOf(t, dawnEyes), c.leftOf(t, dawnHammer); eyes != 3 || hammer != 4 || len(c.dawnEvents(t)) != 2 {
		t.Errorf("the rest taken again left the eyes at %d and the hammer at %d with %d events, want 3, 4 and 2", eyes, hammer, len(c.dawnEvents(t)))
	}
}

func TestALongRestWithoutTheDawnAndAShortRestLeaveTheCharges(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes)
	c.mustSpend(t, c.ana, dawnEyes, 3)
	if _, err := c.restLong(t); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	if got := c.leftOf(t, dawnEyes); got != 0 {
		t.Errorf("a long rest without restore_dawn left the eyes at %d, want 0", got)
	}
	_, err := c.master.resource.TakeRest(t.Context(), connect.NewRequest(&playv1.TakeRestRequest{
		CampaignId: c.campaignID, Kind: playv1.RestKind_REST_KIND_SHORT, IdempotencyKey: newKey(), RestoreDawn: true,
	}))
	if err != nil {
		t.Fatalf("TakeRest(short) error = %v", err)
	}
	if got := c.leftOf(t, dawnEyes); got != 0 || len(c.dawnEvents(t)) != 0 {
		t.Errorf("a short rest with restore_dawn left the eyes at %d with %d events, want it ignored", got, len(c.dawnEvents(t)))
	}
}

// The charges and the lines of a dawn are the master's and the owner's: another player reads no word of them, and the stream
// tells the owner and the master that the sheet changed and nobody else.
func TestTheDawnTellsTheOwnerAndTheMasterAlone(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes)
	c.mustSpend(t, c.ana, dawnEyes, 2)
	master, ana, caio := c.master.watch(t, c.campaignID), c.ana.watch(t, c.campaignID), c.caio.watch(t, c.campaignID)
	for _, w := range []*watcher{master, ana, caio} {
		w.ready(t)
	}
	if _, err := c.dawn(t, c.master); err != nil {
		t.Fatalf("Dawn() error = %v", err)
	}
	c.master.markCurrentMap(t, c.campaignID, c.h.newMap(c.campaignID, 0), master, ana, caio)
	heard := func(w *watcher) bool {
		for _, ev := range w.beforeMarker(t) {
			if ev.GetVitalsChanged().GetVitals().GetCharacterId() == c.pens.GetId() {
				return true
			}
		}
		return false
	}
	if !heard(master) || !heard(ana) {
		t.Error("the master and the owner were not told that the sheet changed")
	}
	if evs := caio.beforeMarker(t); len(evs) != 0 {
		t.Errorf("another player received %v", evs)
	}
}

// A retried long rest with the dawn tells the owners of the items the dawn recharged, as the first call did, also when the rest
// itself changed nothing of their character (one at 0 hit points): the first call may have died before telling.
func TestARetriedLongRestWithTheDawnTellsTheOwnerOfAnItemTheRestDidNotChangeTheCharacterOf(t *testing.T) {
	t.Parallel()
	c := newCharged(t, dawnEyes)
	c.mustSpend(t, c.ana, dawnEyes, 2)
	c.hurt(t, c.pens, 0)
	master, ana, caio := c.master.watch(t, c.campaignID), c.ana.watch(t, c.campaignID), c.caio.watch(t, c.campaignID)
	for _, w := range []*watcher{master, ana, caio} {
		w.ready(t)
	}
	heardPens := func(w *watcher) bool {
		for _, ev := range w.beforeMarker(t) {
			if ev.GetVitalsChanged().GetVitals().GetCharacterId() == c.pens.GetId() {
				return true
			}
		}
		return false
	}
	key := newKey()
	withKey := func(r *playv1.TakeRestRequest) { r.IdempotencyKey = key }
	for _, call := range []string{"first call", "retry"} {
		if _, err := c.restLong(t, withDawn, withKey); err != nil {
			t.Fatalf("TakeRest(%s) error = %v", call, err)
		}
		mapID := c.h.newMap(c.campaignID, 0)
		c.master.markCurrentMap(t, c.campaignID, mapID, master, ana, caio)
		if got := c.leftOf(t, dawnEyes); got != 3 {
			t.Fatalf("%s: the eyes have %d charges, want 3", call, got)
		}
		toldMaster, toldAna := heardPens(master), heardPens(ana)
		if !toldMaster || !toldAna {
			t.Errorf("%s: the master (%v) or the owner of the eyes (%v) was not told that the sheet changed", call, toldMaster, toldAna)
		}
		if heardPens(caio) {
			t.Errorf("%s: another player was told of the sheet", call)
		}
	}
}
