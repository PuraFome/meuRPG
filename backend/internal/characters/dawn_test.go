package characters

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

const (
	eyesKey   = "item:eyes-of-charming"                       // 3 charges, all back at dawn
	hammerKey = "item:hammer-of-thunderbolts"                 // 5 charges, 1d4+1 back at dawn
	goatsKey  = "item:figurine-of-wondrous-power-ivory-goats" // 24, all back 7 dawns after the last
	scarabKey = "item:scarab-of-protection"                   // never recharges
	staffKey  = "item:staff-of-power"                         // d20 on the last charge, 2d8+4 at dawn
)

// dawnRolls is a roller that gives the totals in order and remembers what it was asked.
type dawnRolls struct {
	totals []int32
	asked  [][3]int
}

func (d *dawnRolls) roll(count, sides, bonus int) (int32, error) {
	d.asked = append(d.asked, [3]int{count, sides, bonus})
	if len(d.totals) == 0 {
		return 0, errors.New("no total scripted")
	}
	t := d.totals[0]
	d.totals = d.totals[1:]
	return t, nil
}

// row sets the whole stored state of an entry.
func (et *editorTable) row(t *testing.T, characterID, entryID string, used int32, lost string, dawns int32, pending bool) {
	t.Helper()
	err := et.h.svc.queries.SetItemCharges(context.Background(), charactersdb.SetItemChargesParams{
		CharacterID: characterID, EntryID: entryID, ChargesUsed: used, Lost: lost, DawnsWaited: dawns, LastRollPending: pending, Now: et.h.clock.Now(),
	})
	if err != nil {
		t.Fatalf("SetItemCharges: %v", err)
	}
}

// storedRow reads the stored row of an entry.
func (et *editorTable) storedRow(t *testing.T, characterID, entryID string) (charactersdb.ListItemChargesRow, bool) {
	t.Helper()
	rows, err := et.h.svc.queries.ListItemCharges(context.Background(), characterID)
	if err != nil {
		t.Fatalf("ListItemCharges: %v", err)
	}
	for _, r := range rows {
		if r.EntryID == entryID {
			return r, true
		}
	}
	return charactersdb.ListItemChargesRow{}, false
}

func (et *editorTable) dawn(t *testing.T, rolls *dawnRolls) []link.DawnItem {
	t.Helper()
	var out []link.DawnItem
	err := et.inTx(t, func(tx pgx.Tx) error {
		var err error
		out, err = et.h.svc.RestoreDawn(t.Context(), tx, et.campaign, rolls.roll)
		return err
	})
	if err != nil {
		t.Fatalf("RestoreDawn: %v", err)
	}
	return out
}

func (et *editorTable) leftOf(t *testing.T, pc *charactersv1.Character, key string) int32 {
	t.Helper()
	return magicItemOf(et.read(t, et.master, pc.GetId()), key).GetChargesLeft()
}

func TestDawnGivesBackAllTheChargesOfAnItemThatRegainsAll(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, eyesKey)
	entry := magicItemOf(pc, eyesKey).GetId()
	et.setCharges(t, pc.GetId(), entry, 3, "")
	rolls := &dawnRolls{}
	got := et.dawn(t, rolls)
	if len(got) != 1 || got[0].CharacterID != pc.GetId() || got[0].Regained != 3 || got[0].Item.GetId() != entry || got[0].Item.GetChargesLeft() != 3 {
		t.Fatalf("the dawn changed %+v, want the eyes with 3 charges back", got)
	}
	if len(rolls.asked) != 0 {
		t.Errorf("an item that regains all rolled %v", rolls.asked)
	}
	if left := et.leftOf(t, pc, eyesKey); left != 3 {
		t.Errorf("the eyes read %d charges after the dawn, want 3", left)
	}
	if _, set := et.storedRow(t, pc.GetId(), entry); set {
		t.Error("a full item keeps a row")
	}
}

func TestDawnRollsTheDiceAndStopsAtTheMaximum(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, hammerKey)
	entry := magicItemOf(pc, hammerKey).GetId()
	et.setCharges(t, pc.GetId(), entry, 4, "") // 1 left
	rolls := &dawnRolls{totals: []int32{3}}
	got := et.dawn(t, rolls)
	if len(rolls.asked) != 1 || rolls.asked[0] != [3]int{1, 4, 1} {
		t.Errorf("rolled %v, want 1d4+1 once", rolls.asked)
	}
	if len(got) != 1 || got[0].Regained != 3 || et.leftOf(t, pc, hammerKey) != 4 {
		t.Fatalf("the dawn gave %+v, want 3 back (4 left)", got)
	}
	// 3 left and a total of 5 stops at the maximum: 2 back.
	et.setCharges(t, pc.GetId(), entry, 2, "")
	got = et.dawn(t, &dawnRolls{totals: []int32{5}})
	if len(got) != 1 || got[0].Regained != 2 || got[0].Item.GetChargesLeft() != 5 {
		t.Fatalf("the dawn gave %+v, want 2 back (5 left)", got)
	}
}

func TestDawnLeavesAFullItemAloneAndRollsNothingForIt(t *testing.T) {
	t.Parallel()
	et, _ := attuneTable(t, hammerKey, eyesKey)
	rolls := &dawnRolls{}
	if got := et.dawn(t, rolls); len(got) != 0 || len(rolls.asked) != 0 {
		t.Errorf("the dawn of full items changed %+v and rolled %v", got, rolls.asked)
	}
}

func TestDawnNeverRestoresALostItemOrOneThatNeverRecharges(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, eyesKey, scarabKey)
	eyes, scarab := magicItemOf(pc, eyesKey).GetId(), magicItemOf(pc, scarabKey).GetId()
	et.row(t, pc.GetId(), eyes, 3, "gone", 0, false)
	et.setCharges(t, pc.GetId(), scarab, 5, "")
	rolls := &dawnRolls{}
	if got := et.dawn(t, rolls); len(got) != 0 {
		t.Fatalf("the dawn changed %+v, want nothing", got)
	}
	if r, _ := et.storedRow(t, pc.GetId(), eyes); r.Lost != "gone" || r.ChargesUsed != 3 {
		t.Errorf("the lost eyes are now %+v", r)
	}
	if r, _ := et.storedRow(t, pc.GetId(), scarab); r.ChargesUsed != 5 {
		t.Errorf("the scarab is now %+v", r)
	}
}

func TestDawnLeavesAnItemThatWaitsForItsD20AsItIs(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, staffKey)
	entry := magicItemOf(pc, staffKey).GetId()
	et.row(t, pc.GetId(), entry, 20, "", 0, true)
	rolls := &dawnRolls{}
	if got := et.dawn(t, rolls); len(got) != 0 || len(rolls.asked) != 0 {
		t.Fatalf("the dawn changed %+v and rolled %v", got, rolls.asked)
	}
	if r, _ := et.storedRow(t, pc.GetId(), entry); !r.LastRollPending || r.ChargesUsed != 20 {
		t.Errorf("the staff is now %+v, want it still waiting for its d20", r)
	}
}

// The goats regain all their charges the seventh dawn after the last one was spent: six dawns are not enough.
func TestTheIvoryGoatsCountSevenDawnsAfterTheLastCharge(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, goatsKey)
	entry := magicItemOf(pc, goatsKey).GetId()
	et.setCharges(t, pc.GetId(), entry, 24, "")
	for dawn := int32(1); dawn <= 6; dawn++ {
		if got := et.dawn(t, &dawnRolls{}); len(got) != 0 {
			t.Fatalf("dawn %d restored %+v", dawn, got)
		}
		if r, _ := et.storedRow(t, pc.GetId(), entry); r.DawnsWaited != dawn || r.ChargesUsed != 24 {
			t.Fatalf("after dawn %d the row is %+v", dawn, r)
		}
	}
	got := et.dawn(t, &dawnRolls{})
	if len(got) != 1 || got[0].Regained != 24 || got[0].Item.GetChargesLeft() != 24 {
		t.Fatalf("the seventh dawn gave %+v, want the 24 charges back", got)
	}
	if _, set := et.storedRow(t, pc.GetId(), entry); set {
		t.Error("the restored goats keep a row with a count of dawns")
	}
	// They start over: the charges spent again wait seven dawns again.
	et.setCharges(t, pc.GetId(), entry, 24, "")
	if got := et.dawn(t, &dawnRolls{}); len(got) != 0 {
		t.Fatalf("the first dawn after the second emptying restored %+v", got)
	}
	if r, _ := et.storedRow(t, pc.GetId(), entry); r.DawnsWaited != 1 {
		t.Errorf("the row is %+v, want one dawn counted", r)
	}
}

// Goats that still have charges do not wait: the count of dawns starts when the last one is spent.
func TestTheIvoryGoatsWithChargesLeftDoNotCountDawns(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, goatsKey)
	entry := magicItemOf(pc, goatsKey).GetId()
	et.row(t, pc.GetId(), entry, 20, "", 4, false) // some charges left, a stale count
	if got := et.dawn(t, &dawnRolls{}); len(got) != 0 {
		t.Fatalf("the dawn changed %+v, want nothing: the goats are not empty", got)
	}
	r, set := et.storedRow(t, pc.GetId(), entry)
	if !set || r.DawnsWaited != 0 || r.ChargesUsed != 20 {
		t.Errorf("the row is %+v, want the count of dawns at 0 and the 20 spent kept", r)
	}
}

func TestAFailedRollOfTheDawnLeavesTheRowsAsTheyWere(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t, hammerKey)
	entry := magicItemOf(pc, hammerKey).GetId()
	et.setCharges(t, pc.GetId(), entry, 4, "")
	err := et.inTx(t, func(tx pgx.Tx) error {
		_, err := et.h.svc.RestoreDawn(t.Context(), tx, et.campaign, (&dawnRolls{}).roll) // no total scripted: the roll fails
		return err
	})
	if err == nil {
		t.Fatal("RestoreDawn() = nil, want the roll's error")
	}
	if r, _ := et.storedRow(t, pc.GetId(), entry); r.ChargesUsed != 4 {
		t.Errorf("a failed dawn left %+v, want the 4 spent kept", r)
	}
}

// A dawn and a spend of the same item take turns: the character's row is locked, so the item ends as one of the two orders
// leaves it (2: the dawn first and the spend after, 3: the spend first and the dawn after), never with the dawn or the
// spend lost.
func TestADawnAndASpendOfTheSameItemTakeTurns(t *testing.T) {
	t.Parallel()
	for round := range 5 {
		et, pc := attuneTable(t, eyesKey)
		entry := magicItemOf(pc, eyesKey).GetId()
		et.setCharges(t, pc.GetId(), entry, 2, "") // 1 left
		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, errs[0] = et.spend(t, pc, eyesKey, 1)
		}()
		go func() {
			defer wg.Done()
			errs[1] = et.inTx(t, func(tx pgx.Tx) error {
				_, err := et.h.svc.RestoreDawn(t.Context(), tx, et.campaign, (&dawnRolls{}).roll)
				return err
			})
		}()
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("round %d: spend = %v, dawn = %v", round, errs[0], errs[1])
		}
		if left := et.leftOf(t, pc, eyesKey); left != 2 && left != 3 {
			t.Fatalf("round %d: the eyes have %d charges, want 2 or 3 (a lost update)", round, left)
		}
	}
}
