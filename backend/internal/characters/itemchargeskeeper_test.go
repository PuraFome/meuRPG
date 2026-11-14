package characters

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/db"
)

// inTx runs fn in a transaction of its own on the table's database.
func (et *editorTable) inTx(t *testing.T, fn func(tx pgx.Tx) error) error {
	t.Helper()
	return db.InTx(t.Context(), et.h.pool, fn)
}

type spendResult struct {
	before, after *charactersv1.SheetMagicItem
}

func (et *editorTable) spend(t *testing.T, pc *charactersv1.Character, key string, amount int32) (spendResult, error) {
	t.Helper()
	var r spendResult
	err := et.inTx(t, func(tx pgx.Tx) error {
		var err error
		r.before, r.after, err = et.h.svc.SpendItemCharges(t.Context(), tx, et.campaign, pc.GetId(), magicItemOf(pc, key).GetId(), amount)
		return err
	})
	return r, err
}

func refusedWith(err error) charactersv1.CharacterBlockedReason {
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition {
		return charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_UNSPECIFIED
	}
	for _, d := range ce.Details() {
		if v, derr := d.Value(); derr == nil {
			if b, ok := v.(*charactersv1.CharacterBlocked); ok {
				return b.GetReason()
			}
		}
	}
	return charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_UNSPECIFIED
}

func TestSpendItemChargesTakesTheCharges(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey)
	r, err := et.spend(t, pc, wandKey, 2)
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if r.before.GetChargesLeft() != 7 || r.after.GetChargesLeft() != 5 || r.after.GetChargesMax() != 7 || r.after.GetLastRollPending() {
		t.Errorf("before %+v after %+v", r.before, r.after)
	}
	if got := magicItemOf(et.read(t, et.master, pc.GetId()), wandKey); got.GetChargesLeft() != 5 {
		t.Errorf("read back: %+v", got)
	}
}

func TestSpendingTheLastChargeOfAWandWaitsForTheD20(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey, "item:staff-of-striking")
	for _, key := range []string{wandKey, "item:staff-of-striking"} {
		it := magicItemOf(pc, key)
		r, err := et.spend(t, pc, key, it.GetChargesMax()) // the whole item at once
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if !r.after.GetLastRollPending() || r.after.GetChargesLeft() != 0 || r.after.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED {
			t.Errorf("%s after: %+v", key, r.after)
		}
		got := magicItemOf(et.read(t, et.dona, pc.GetId()), key)
		if !got.GetLastRollPending() {
			t.Errorf("%s read back: %+v", key, got)
		}
		// Nothing can be spent while the roll waits.
		if _, err := et.spend(t, pc, key, 1); refusedWith(err) != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ITEM_NO_CHARGES {
			t.Errorf("%s spend again: %v", key, err)
		}
	}
}

func TestSpendingTheLastChargeOfAScarabLosesIt(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, "item:scarab-of-protection")
	et.setCharges(t, pc.GetId(), magicItemOf(pc, "item:scarab-of-protection").GetId(), 11, "")
	r, err := et.spend(t, pc, "item:scarab-of-protection", 1)
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if r.after.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_GONE || r.after.GetChargesLeft() != 0 || r.after.GetLastRollPending() {
		t.Errorf("after: %+v", r.after)
	}
	if _, err := et.spend(t, pc, "item:scarab-of-protection", 1); refusedWith(err) != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ITEM_DESTROYED {
		t.Errorf("a lost scarab: %v", err)
	}
}

func TestSpendItemChargesRefusals(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey, bagKey, "item:luck-blade", "item:staff-of-power")
	wand := magicItemOf(pc, wandKey).GetId()
	et.setCharges(t, pc.GetId(), magicItemOf(pc, "item:staff-of-power").GetId(), 20, "")
	spendEntry := func(character, entry string, amount int32) error {
		return et.inTx(t, func(tx pgx.Tx) error {
			_, _, err := et.h.svc.SpendItemCharges(t.Context(), tx, et.campaign, character, entry, amount)
			return err
		})
	}
	if _, err := et.spend(t, pc, wandKey, 8); refusedWith(err) != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_CHARGES_NOT_ENOUGH {
		t.Errorf("more than left: %v", err)
	}
	if _, err := et.spend(t, pc, "item:luck-blade", 1); refusedWith(err) != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ITEM_NO_CHARGES {
		t.Errorf("a blade with no count: %v", err)
	}
	if _, err := et.spend(t, pc, "item:staff-of-power", 1); refusedWith(err) != charactersv1.CharacterBlockedReason_CHARACTER_BLOCKED_REASON_ITEM_NO_CHARGES {
		t.Errorf("an empty staff: %v", err)
	}
	for name, err := range map[string]error{
		"zero":      spendEntry(pc.GetId(), wand, 0),
		"negative":  spendEntry(pc.GetId(), wand, -1),
		"a bag":     spendEntry(pc.GetId(), magicItemOf(pc, bagKey).GetId(), 1),
		"no entry":  spendEntry(pc.GetId(), "m99", 1),
		"no entry2": spendEntry(pc.GetId(), "", 1),
	} {
		if code := connect.CodeOf(err); code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := spendEntry("not-an-id", wand, 1); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a bad character id: %v", err)
	}
	if err := spendEntry("5b2f5d1c-0000-4000-8000-000000000000", wand, 1); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another character: %v", err)
	}
	if got := magicItemOf(et.read(t, et.master, pc.GetId()), wandKey); got.GetChargesLeft() != 7 {
		t.Errorf("a refusal wrote: %+v", got)
	}
}

func (et *editorTable) resolve(t *testing.T, pc *charactersv1.Character, key string, d20, extra int32) (spendResult, int32, error) {
	t.Helper()
	var r spendResult
	var regained int32
	err := et.inTx(t, func(tx pgx.Tx) error {
		var err error
		r.before, r.after, regained, err = et.h.svc.ResolveLastCharge(t.Context(), tx, et.campaign, pc.GetId(), magicItemOf(pc, key).GetId(), d20, extra)
		return err
	})
	return r, regained, err
}

func TestResolveLastCharge(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey, "item:staff-of-power", "item:staff-of-the-magi", "item:staff-of-striking")
	for _, key := range []string{wandKey, "item:staff-of-power", "item:staff-of-the-magi", "item:staff-of-striking"} {
		if _, err := et.spend(t, pc, key, magicItemOf(pc, key).GetChargesMax()); err != nil {
			t.Fatalf("empty %s: %v", key, err)
		}
	}
	// A 1 loses the item as its data says.
	r, regained, err := et.resolve(t, pc, wandKey, 1, 0)
	if err != nil || r.after.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_GONE || r.after.GetLastRollPending() || regained != 0 || !r.before.GetLastRollPending() {
		t.Errorf("a 1: %+v %d %v", r, regained, err)
	}
	if _, _, err := et.resolve(t, pc, wandKey, 10, 0); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("nothing waits any more: %v", err)
	}
	r, _, err = et.resolve(t, pc, "item:staff-of-striking", 1, 0)
	if err != nil || r.after.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_PLAIN_ITEM {
		t.Errorf("striking a 1: %+v %v", r, err)
	}
	// A 1 on an item that has no loss only ends the wait.
	r, _, err = et.resolve(t, pc, "item:staff-of-the-magi", 1, 0)
	if err != nil || r.after.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED || r.after.GetLastRollPending() {
		t.Errorf("magi a 1: %+v %v", r, err)
	}
	// A 20 regains what the caller rolled, up to the maximum.
	r, regained, err = et.resolve(t, pc, "item:staff-of-power", 20, 5)
	if err != nil || regained != 5 || r.after.GetChargesLeft() != 5 || r.after.GetLastRollPending() {
		t.Errorf("a 20: %+v %d %v", r, regained, err)
	}
	if got := magicItemOf(et.read(t, et.master, pc.GetId()), "item:staff-of-power"); got.GetChargesLeft() != 5 || got.GetLastRollPending() {
		t.Errorf("read back: %+v", got)
	}
}

func TestResolveLastChargeClampsAndRefuses(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey, "item:staff-of-power")
	for _, key := range []string{wandKey, "item:staff-of-power"} {
		if _, err := et.spend(t, pc, key, magicItemOf(pc, key).GetChargesMax()); err != nil {
			t.Fatalf("empty %s: %v", key, err)
		}
	}
	if _, _, err := et.resolve(t, pc, wandKey, 0, 0); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a d20 of 0: %v", err)
	}
	if _, _, err := et.resolve(t, pc, wandKey, 21, 0); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a d20 of 21: %v", err)
	}
	// A wand has no rule for a 20: it only ends the wait.
	r, regained, err := et.resolve(t, pc, wandKey, 20, 4)
	if err != nil || regained != 0 || r.after.GetChargesLeft() != 0 || r.after.GetLastRollPending() {
		t.Errorf("a wand 20: %+v %d %v", r, regained, err)
	}
	r, regained, err = et.resolve(t, pc, "item:staff-of-power", 20, 99)
	if err != nil || regained != 20 || r.after.GetChargesLeft() != 20 {
		t.Errorf("clamped: %+v %d %v", r, regained, err)
	}
	// An item that never waited for a roll refuses it.
	if _, _, err := et.resolve(t, pc, "item:staff-of-power", 5, 0); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("not pending: %v", err)
	}
}

func TestSetItemCharges(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey, bagKey, "item:luck-blade")
	set := func(key string, left int32, lost charactersv1.ItemLoss) (spendResult, error) {
		var r spendResult
		err := et.inTx(t, func(tx pgx.Tx) error {
			var err error
			r.before, r.after, err = et.h.svc.SetItemCharges(t.Context(), tx, et.campaign, pc.GetId(), magicItemOf(pc, key).GetId(), left, lost)
			return err
		})
		return r, err
	}
	// The luck blade leaves the "not set" state and keeps its row at full.
	r, err := set("item:luck-blade", 3, charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED)
	if err != nil || r.before.GetChargesLeft() != -1 || r.after.GetChargesLeft() != 3 {
		t.Fatalf("blade: %+v %v", r, err)
	}
	if rows, err := et.h.svc.queries.ListItemCharges(context.Background(), pc.GetId()); err != nil || len(rows) != 1 {
		t.Errorf("rows %v %v", rows, err)
	}
	if got := magicItemOf(et.read(t, et.master, pc.GetId()), "item:luck-blade"); got.GetChargesLeft() != 3 {
		t.Errorf("blade read: %+v", got)
	}
	// A full wand keeps no row.
	if _, err := et.spend(t, pc, wandKey, 7); err != nil {
		t.Fatalf("empty wand: %v", err)
	}
	r, err = set(wandKey, 7, charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED)
	if err != nil || r.after.GetChargesLeft() != 7 || r.after.GetLastRollPending() || !r.before.GetLastRollPending() {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if rows, _ := et.h.svc.queries.ListItemCharges(context.Background(), pc.GetId()); len(rows) != 1 {
		t.Errorf("rows after the restore: %v", rows)
	}
	// The master marks a loss, and clears it again.
	r, err = set(wandKey, 2, charactersv1.ItemLoss_ITEM_LOSS_PLAIN_ITEM)
	if err != nil || r.after.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_PLAIN_ITEM || r.after.GetChargesLeft() != 0 {
		t.Fatalf("lose: %+v %v", r, err)
	}
	r, err = set(wandKey, 4, charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED)
	if err != nil || r.after.GetLost() != charactersv1.ItemLoss_ITEM_LOSS_UNSPECIFIED || r.after.GetChargesLeft() != 4 {
		t.Fatalf("unlose: %+v %v", r, err)
	}
	for name, err := range map[string]error{
		"above max": errOf(set(wandKey, 8, 0)),
		"negative":  errOf(set(wandKey, -1, 0)),
		"a bag":     errOf(set(bagKey, 1, 0)),
		"bad loss":  errOf(set(wandKey, 1, charactersv1.ItemLoss(77))),
	} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func errOf(_ spendResult, err error) error { return err }

// A save never sets the pending roll, and keeps the one stored.
func TestSaveIgnoresTheLastRollPending(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey, "item:staff-of-striking")
	if _, err := et.spend(t, pc, wandKey, 7); err != nil {
		t.Fatalf("spend: %v", err)
	}
	req := editorSaveOf(et.read(t, et.master, pc.GetId()))
	req.Sheet = editorClone(req.GetSheet(), func(f *charactersv1.FullSheet) {
		for _, it := range f.GetMagicItems() {
			it.LastRollPending = it.GetKey() != wandKey
		}
	})
	saved, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !magicItemOf(saved, wandKey).GetLastRollPending() || magicItemOf(saved, "item:staff-of-striking").GetLastRollPending() {
		t.Errorf("after the save: %+v", saved.GetSheet().GetFull().GetMagicItems())
	}
}

// Another player and a pending member never read the pending flag.
func TestAnotherPlayerNeverReadsThePendingRoll(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, wandKey)
	if _, err := et.spend(t, pc, wandKey, 7); err != nil {
		t.Fatalf("spend: %v", err)
	}
	for _, u := range []*user{et.caio, et.pia, et.visitor} {
		_, err := u.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: et.campaign, CharacterId: pc.GetId()}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("another reader: %v", err)
		}
	}
}
