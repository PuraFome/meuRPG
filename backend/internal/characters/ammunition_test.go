package characters

import (
	"errors"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/prototext"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
)

const (
	arrowKey = "equipment:arrow"
	boltKey  = "equipment:crossbow-bolt"
)

func (et *editorTable) setAmmo(t *testing.T, pc *charactersv1.Character, key string, quantity int32) (*charactersv1.SheetAmmunition, error) {
	t.Helper()
	var got *charactersv1.SheetAmmunition
	err := et.inTx(t, func(tx pgx.Tx) error {
		var err error
		got, err = et.h.svc.SetAmmunition(t.Context(), tx, et.campaign, pc.GetId(), key, quantity)
		return err
	})
	return got, err
}

func (et *editorTable) shoot(t *testing.T, pc *charactersv1.Character, key, encounter string) (int32, error) {
	t.Helper()
	var left int32
	err := et.inTx(t, func(tx pgx.Tx) error {
		var err error
		left, err = et.h.svc.SpendAmmunition(t.Context(), tx, et.campaign, pc.GetId(), key, encounter)
		return err
	})
	return left, err
}

func (et *editorTable) recover(t *testing.T, pc *charactersv1.Character, encounter string) ([]*charactersv1.SheetAmmunition, error) {
	t.Helper()
	var got []*charactersv1.SheetAmmunition
	err := et.inTx(t, func(tx pgx.Tx) error {
		var err error
		got, err = et.h.svc.RecoverAmmunition(t.Context(), tx, et.campaign, pc.GetId(), encounter)
		return err
	})
	return got, err
}

func ammoOf(c *charactersv1.Character, key string) *charactersv1.SheetAmmunition {
	for _, a := range c.GetSheet().GetFull().GetAmmunition() {
		if a.GetKey() == key {
			return a
		}
	}
	return nil
}

func TestSetAmmunition(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	got, err := et.setAmmo(t, pc, arrowKey, 20)
	if err != nil || got.GetKey() != arrowKey || got.GetQuantity() != 20 || got.GetNamePt() == "" {
		t.Fatalf("set: %+v %v", got, err)
	}
	if _, err := et.setAmmo(t, pc, boltKey, 5); err != nil {
		t.Fatalf("set bolts: %v", err)
	}
	if _, err := et.setAmmo(t, pc, arrowKey, 0); err != nil {
		t.Fatalf("set to none: %v", err)
	}
	for _, reader := range []*user{et.master, et.dona} {
		read := et.read(t, reader, pc.GetId())
		if a := ammoOf(read, boltKey); a.GetQuantity() != 5 || a.GetNamePt() == "" {
			t.Errorf("bolts: %+v", read.GetSheet().GetFull().GetAmmunition())
		}
		if ammoOf(read, arrowKey) != nil {
			t.Errorf("an empty kind that fired nothing is listed: %+v", read.GetSheet().GetFull().GetAmmunition())
		}
	}
	for name, err := range map[string]error{
		"not ammunition": errOf2(et.setAmmo(t, pc, "equipment:longbow", 1)),
		"unknown":        errOf2(et.setAmmo(t, pc, "", 1)),
		"negative":       errOf2(et.setAmmo(t, pc, arrowKey, -1)),
		"too many":       errOf2(et.setAmmo(t, pc, arrowKey, 10000)),
	} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := et.setAmmo(t, &charactersv1.Character{Id: uuid.NewString()}, arrowKey, 1); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another character: %v", err)
	}
}

func errOf2(_ *charactersv1.SheetAmmunition, err error) error { return err }

func TestReadListsTheKindsInOrderWithTheirFiredPieces(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	enc := uuid.NewString()
	for _, k := range []string{boltKey, arrowKey} {
		if _, err := et.setAmmo(t, pc, k, 1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := et.shoot(t, pc, boltKey, enc); err != nil { // 0 pieces left, 1 fired: still listed
		t.Fatal(err)
	}
	got := et.read(t, et.master, pc.GetId()).GetSheet().GetFull().GetAmmunition()
	if len(got) != 2 || got[0].GetKey() != arrowKey || got[1].GetKey() != boltKey || got[1].GetQuantity() != 0 || got[1].GetSpentLastCombat() != 1 || got[1].GetRecovered() {
		t.Errorf("ammunition: %+v", got)
	}
}

func TestSaveIgnoresAndKeepsTheAmmunition(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	if _, err := et.setAmmo(t, pc, arrowKey, 12); err != nil {
		t.Fatal(err)
	}
	req := editorSaveOf(et.read(t, et.master, pc.GetId()))
	req.Sheet = editorClone(req.GetSheet(), func(f *charactersv1.FullSheet) {
		f.Ammunition = []*charactersv1.SheetAmmunition{{Key: arrowKey, Quantity: 5000}, {Key: boltKey, Quantity: 7}}
	})
	saved, err := editorUpdate(et.master, req)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := saved.GetSheet().GetFull().GetAmmunition(); len(got) != 1 || got[0].GetKey() != arrowKey || got[0].GetQuantity() != 12 {
		t.Errorf("after the save: %+v", got)
	}
	// The stored document never holds it either.
	var doc string
	if err := et.h.pool.QueryRow(t.Context(), "SELECT sheet::text FROM characters WHERE id = $1", pc.GetId()).Scan(&doc); err != nil {
		t.Fatal(err)
	}
	if containsAny(doc, "ammunition", "5000") {
		t.Errorf("the document holds ammunition: %s", doc)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}

func TestSpendAmmunition(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	encA, encB := uuid.NewString(), uuid.NewString()
	if _, err := et.shoot(t, pc, arrowKey, encA); !errors.Is(err, ErrNoAmmunition) {
		t.Fatalf("a kind never held: %v", err)
	}
	if _, err := et.setAmmo(t, pc, arrowKey, 3); err != nil {
		t.Fatal(err)
	}
	for want := int32(2); want >= 1; want-- {
		if left, err := et.shoot(t, pc, arrowKey, encA); err != nil || left != want {
			t.Fatalf("shot: %d %v, want %d", left, err, want)
		}
	}
	if a := ammoOf(et.read(t, et.master, pc.GetId()), arrowKey); a.GetQuantity() != 1 || a.GetSpentLastCombat() != 2 {
		t.Errorf("after two shots: %+v", a)
	}
	// Another combat starts the count over.
	if left, err := et.shoot(t, pc, arrowKey, encB); err != nil || left != 0 {
		t.Fatalf("shot in B: %d %v", left, err)
	}
	if a := ammoOf(et.read(t, et.master, pc.GetId()), arrowKey); a.GetSpentLastCombat() != 1 || a.GetRecovered() {
		t.Errorf("after the new combat: %+v", a)
	}
	// None left: refused, nothing written.
	if _, err := et.shoot(t, pc, arrowKey, encB); !errors.Is(err, ErrNoAmmunition) {
		t.Fatalf("empty: %v", err)
	}
	if a := ammoOf(et.read(t, et.master, pc.GetId()), arrowKey); a.GetSpentLastCombat() != 1 || a.GetQuantity() != 0 {
		t.Errorf("after the refusal: %+v", a)
	}
	if _, err := et.shoot(t, pc, arrowKey, "nope"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a bad encounter id: %v", err)
	}
	if _, err := et.shoot(t, &charactersv1.Character{Id: uuid.NewString()}, arrowKey, encA); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("another character: %v", err)
	}
}

func TestAmmunitionLeft(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	left := func(key string) int32 {
		var n int32
		if err := et.inTx(t, func(tx pgx.Tx) error {
			var err error
			n, err = et.h.svc.AmmunitionLeft(t.Context(), tx, pc.GetId(), key)
			return err
		}); err != nil {
			t.Fatalf("AmmunitionLeft: %v", err)
		}
		return n
	}
	if n := left(arrowKey); n != 0 {
		t.Errorf("no row: %d", n)
	}
	if _, err := et.setAmmo(t, pc, arrowKey, 9); err != nil {
		t.Fatal(err)
	}
	if n := left(arrowKey); n != 9 {
		t.Errorf("held: %d", n)
	}
}

func TestRecoverAmmunition(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	enc, other := uuid.NewString(), uuid.NewString()
	if _, err := et.recover(t, pc, enc); !errors.Is(err, ErrNothingToRecover) {
		t.Fatalf("nothing fired: %v", err)
	}
	for k, n := range map[string]int32{arrowKey: 10, boltKey: 10} {
		if _, err := et.setAmmo(t, pc, k, n); err != nil {
			t.Fatal(err)
		}
	}
	for range 5 {
		if _, err := et.shoot(t, pc, arrowKey, enc); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := et.shoot(t, pc, boltKey, enc); err != nil { // one piece: nothing comes back, still marked
		t.Fatal(err)
	}
	if _, err := et.recover(t, pc, other); !errors.Is(err, ErrNothingToRecover) {
		t.Fatalf("another combat: %v", err)
	}
	got, err := et.recover(t, pc, enc)
	if err != nil || len(got) != 2 {
		t.Fatalf("recover: %+v %v", got, err)
	}
	read := et.read(t, et.master, pc.GetId())
	if a := ammoOf(read, arrowKey); a.GetQuantity() != 7 || !a.GetRecovered() || a.GetSpentLastCombat() != 5 {
		t.Errorf("arrows: %+v", a)
	}
	if a := ammoOf(read, boltKey); a.GetQuantity() != 9 || !a.GetRecovered() {
		t.Errorf("bolts: %+v", a)
	}
	if _, err := et.recover(t, pc, enc); !errors.Is(err, ErrAmmunitionRecovered) {
		t.Errorf("twice: %v", err)
	}
	if a := ammoOf(et.read(t, et.master, pc.GetId()), arrowKey); a.GetQuantity() != 7 {
		t.Errorf("the second try gave pieces: %+v", a)
	}
}

func TestAmmunitionGoesWithTheCharacter(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	if _, err := et.setAmmo(t, pc, arrowKey, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := et.h.pool.Exec(t.Context(), "DELETE FROM characters WHERE id = $1", pc.GetId()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := et.h.pool.QueryRow(t.Context(), "SELECT count(*) FROM character_ammunition WHERE character_id = $1", pc.GetId()).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows left: %d %v", n, err)
	}
}

// Two shots at the last piece in parallel transactions: exactly one fires.
func TestTwoShotsAtTheLastPieceFireOnce(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	if _, err := et.setAmmo(t, pc, arrowKey, 1); err != nil {
		t.Fatal(err)
	}
	enc := uuid.NewString()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() { _, errs[i] = et.shoot(t, pc, arrowKey, enc) })
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, ErrNoAmmunition):
			t.Errorf("unexpected: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d shots fired", ok)
	}
	if a := ammoOf(et.read(t, et.master, pc.GetId()), arrowKey); a.GetQuantity() != 0 || a.GetSpentLastCombat() != 1 {
		t.Errorf("state: %+v", a)
	}
}

// Another player, a pending member and a stranger never read the ammunition: the character is not found.
func TestAnotherPlayerNeverReadsTheAmmunition(t *testing.T) {
	t.Parallel()
	et, pc := attuneTable(t)
	if _, err := et.setAmmo(t, pc, arrowKey, 4); err != nil {
		t.Fatal(err)
	}
	for _, u := range []*user{et.caio, et.pia, et.visitor} {
		_, err := u.api.GetCharacter(t.Context(), connect.NewRequest(&charactersv1.GetCharacterRequest{CampaignId: et.campaign, CharacterId: pc.GetId()}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("another reader: %v", err)
		}
		res, err := u.api.ListCharacters(t.Context(), connect.NewRequest(&charactersv1.ListCharactersRequest{CampaignId: et.campaign}))
		if err == nil && containsAny(prototext.Format(res.Msg), arrowKey, "ammunition") {
			t.Errorf("the list carries ammunition")
		}
	}
}
