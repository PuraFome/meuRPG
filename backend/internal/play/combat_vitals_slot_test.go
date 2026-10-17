package play

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// slotVitalsKeeper stands in for the characters module: it serves one sheet and records the
// adjustment spendSlot asks for. No database is needed, spendSlot only passes the transaction on.
type slotVitalsKeeper struct {
	VitalsKeeper
	now       *playv1.CharacterVitals
	getErr    error
	adjustErr error
	adjusted  []*playv1.AdjustCharacterVitalsRequest
}

func (k *slotVitalsKeeper) GetVitalsTx(context.Context, pgx.Tx, string, string) (*playv1.CharacterVitals, error) {
	return k.now, k.getErr
}

func (k *slotVitalsKeeper) AdjustVitals(_ context.Context, _ pgx.Tx, _, _ string, req *playv1.AdjustCharacterVitalsRequest) (before, after *playv1.CharacterVitals, err error) {
	k.adjusted = append(k.adjusted, req)
	if k.adjustErr != nil {
		return nil, nil, k.adjustErr
	}
	return k.now, proto.Clone(k.now).(*playv1.CharacterVitals), nil
}

func slotSheet(spell []*playv1.SpellSlotUsage, pact *playv1.PactSlotUsage) *playv1.CharacterVitals {
	return &playv1.CharacterVitals{CharacterId: "ch1", SpellSlots: spell, PactSlots: pact}
}

func slotUsage(level, used, total int32) *playv1.SpellSlotUsage {
	return &playv1.SpellSlotUsage{Level: level, Used: used, Total: total}
}

const slotNoFreeMessage = "failed_precondition: there is no free spell slot of that level"

// spendSlot, frozen as it is: what it asks the keeper to write for a spend and for a give-back
// (an undo), the NO_SLOT refusal with the lowest level, and the quiet give-back of a slot the
// sheet no longer has. The refusal is the same for a spell slot and a pact slot.
func TestSpendSlotFrozen(t *testing.T) {
	t.Parallel()
	pact := func(level, used, total int32) *playv1.PactSlotUsage {
		return &playv1.PactSlotUsage{SlotLevel: level, Used: used, Total: total}
	}
	spell := func(level int32) slotRef { return slotRef{Level: level} }
	pactRef := func(level int32) slotRef { return slotRef{Level: level, Pact: true} }
	one := func(level, used int32) *playv1.AdjustCharacterVitalsRequest {
		return &playv1.AdjustCharacterVitalsRequest{SpellSlotsUsed: []*playv1.SpellSlotsUsed{{Level: level, Used: used}}}
	}
	pactUsed := func(used int32) *playv1.AdjustCharacterVitalsRequest {
		return &playv1.AdjustCharacterVitalsRequest{PactSlotsUsed: &used}
	}
	for _, c := range []struct {
		name      string
		sheet     *playv1.CharacterVitals
		slot      slotRef
		delta     int32
		want      *playv1.AdjustCharacterVitalsRequest // nil: nothing is written
		refused   int32                                // the min_level of NO_SLOT, 0 for no refusal
		lostWhat  string                               // the give-back found nothing: the warning's "what"
		wantSheet bool                                 // the result is the sheet as it was read
	}{
		{name: "spend a spell slot", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 1, 3), slotUsage(2, 0, 2)}, nil), slot: spell(2), delta: 1, want: one(2, 1)},
		{name: "spend the last spell slot", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 2, 3)}, nil), slot: spell(1), delta: 1, want: one(1, 3)},
		{name: "spend with every slot of the level used", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 3, 3)}, nil), slot: spell(1), delta: 1, refused: 1},
		{name: "spend a level the sheet has no slots of", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 0, 3)}, nil), slot: spell(3), delta: 1, refused: 3},
		{name: "spend with a spell slot ref when only a pact slot exists", sheet: slotSheet(nil, pact(2, 0, 1)), slot: spell(2), delta: 1, refused: 2},
		{name: "give a spell slot back", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 2, 3)}, nil), slot: spell(1), delta: -1, want: one(1, 1)},
		{name: "give back a slot that was not used", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 0, 3)}, nil), slot: spell(1), delta: -1, want: one(1, 0)},
		{name: "give back above the new total", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 5, 2)}, nil), slot: spell(1), delta: -1, want: one(1, 2)},
		{name: "give back a level the sheet lost", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 1, 3)}, nil), slot: spell(4), delta: -1, lostWhat: "spell slot", wantSheet: true},
		{name: "spend a pact slot", sheet: slotSheet(nil, pact(2, 0, 2)), slot: pactRef(2), delta: 1, want: pactUsed(1)},
		{name: "spend with the pact slots used", sheet: slotSheet(nil, pact(2, 2, 2)), slot: pactRef(2), delta: 1, refused: 2},
		{name: "spend a pact slot of another level", sheet: slotSheet(nil, pact(2, 0, 2)), slot: pactRef(3), delta: 1, refused: 3},
		{name: "spend a pact slot the sheet has not", sheet: slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 0, 3)}, nil), slot: pactRef(1), delta: 1, refused: 1},
		{name: "give a pact slot back", sheet: slotSheet(nil, pact(2, 1, 2)), slot: pactRef(2), delta: -1, want: pactUsed(0)},
		{name: "give back a pact slot that was not used", sheet: slotSheet(nil, pact(2, 0, 2)), slot: pactRef(2), delta: -1, want: pactUsed(0)},
		{name: "give back a pact slot above the new total", sheet: slotSheet(nil, pact(2, 4, 1)), slot: pactRef(2), delta: -1, want: pactUsed(1)},
		{name: "give back a pact slot of another level", sheet: slotSheet(nil, pact(3, 1, 2)), slot: pactRef(2), delta: -1, lostWhat: "pact slot", wantSheet: true},
		{name: "give back a pact slot the sheet lost", sheet: slotSheet(nil, nil), slot: pactRef(2), delta: -1, lostWhat: "pact slot", wantSheet: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			keeper := &slotVitalsKeeper{now: c.sheet}
			var logs bytes.Buffer
			s := &Service{vitals: keeper, logger: slog.New(slog.NewTextHandler(&logs, nil))}
			ct := &combatTx{}
			ct.session.CampaignID = "camp1"
			got, err := s.spendSlot(t.Context(), ct, "ch1", c.slot, c.delta)
			switch {
			case c.refused != 0:
				if got != nil || err == nil || connect.CodeOf(err) != connect.CodeFailedPrecondition || err.Error() != slotNoFreeMessage {
					t.Fatalf("spendSlot() = %v, %v; want the NO_SLOT refusal %q", got, err, slotNoFreeMessage)
				}
				if b := wantEncounterBlocked(t, err, blockedNoSlot); b.GetMinLevel() != c.refused {
					t.Errorf("NO_SLOT min_level = %d, want %d", b.GetMinLevel(), c.refused)
				}
			case err != nil:
				t.Fatalf("spendSlot() error = %v", err)
			}
			if (c.refused != 0 || c.lostWhat != "") && len(keeper.adjusted) != 0 {
				t.Errorf("the keeper was asked to write %v, want nothing", keeper.adjusted)
			}
			if c.want != nil {
				if len(keeper.adjusted) != 1 || !proto.Equal(keeper.adjusted[0], c.want) {
					t.Errorf("the keeper was asked %v, want [%v]", keeper.adjusted, c.want)
				}
				if got == nil || !proto.Equal(got, c.sheet) {
					t.Errorf("spendSlot() = %v, want the vitals after (%v)", got, c.sheet)
				}
			}
			if c.wantSheet && got != c.sheet {
				t.Errorf("spendSlot() = %v, want the sheet as it was read", got)
			}
			warned := strings.Contains(logs.String(), "play: an undo found nothing to give back")
			if warned != (c.lostWhat != "") || (c.lostWhat != "" && !strings.Contains(logs.String(), `what="`+c.lostWhat+`"`)) {
				t.Errorf("log = %q, want a warning about %q: %v", logs.String(), c.lostWhat, c.lostWhat != "")
			}
		})
	}
}

// The two errors spendSlot passes on: the sheet that cannot be read and the keeper that refuses the
// write; in both nothing else happens.
func TestSpendSlotFrozenKeeperErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	sheet := slotSheet([]*playv1.SpellSlotUsage{slotUsage(1, 0, 2)}, nil)
	logger := slog.New(slog.DiscardHandler)

	read := &slotVitalsKeeper{getErr: boom}
	if got, err := (&Service{vitals: read, logger: logger}).spendSlot(t.Context(), &combatTx{}, "ch1", slotRef{Level: 1}, 1); got != nil || !errors.Is(err, boom) || len(read.adjusted) != 0 {
		t.Errorf("a sheet that cannot be read: spendSlot() = %v, %v; want the keeper's error and no write", got, err)
	}
	write := &slotVitalsKeeper{now: sheet, adjustErr: boom}
	if got, err := (&Service{vitals: write, logger: logger}).spendSlot(t.Context(), &combatTx{}, "ch1", slotRef{Level: 1}, 1); got != nil || !errors.Is(err, boom) || len(write.adjusted) != 1 {
		t.Errorf("a write the keeper refuses: spendSlot() = %v, %v; want the keeper's error", got, err)
	}
}
