package characters

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Vitals (RN-02): a player character's current and temporary hit points,
// spell slots used, pact magic slots used and hit dice used, which last
// from one game session to the next (table character_vitals).
//
// The maximums are never stored: they come from rules.Derive, like every
// number on the sheet, and the stored values are clamped to them on every
// read. A character without a row is fresh: full hit points, nothing used.
//
// Package play serves the vitals (PlayService: GetLiveSession, the live
// stream, AdjustCharacterVitals) through its VitalsKeeper interface, which
// this Service implements with ListVitals, GetVitals and AdjustVitals. The
// messages are play's (playv1.CharacterVitals): play declares the interface
// and what goes through it, as with SheetLocker, and this package only
// fills them in. So this package imports play's generated API types
// (gen/meurpg/play/v1), never package play itself; cmd/api connects the
// two.

// MaxTemporaryHitPoints is the most temporary hit points a character can
// have. The rules set no maximum; this one only keeps typos out.
const MaxTemporaryHitPoints = 999

// maxSpellLevel is the highest spell level, the length of
// character_vitals.spell_slots_used.
const maxSpellLevel = 9

// vitalsRow is a character with its stored vitals: a row of ListVitals or
// GetVitals, which have the same columns. The vitals columns are nil when
// the character has no character_vitals row.
type vitalsRow = charactersdb.ListVitalsRow

// vitalsMax are the maximums, derived from the sheet.
type vitalsMax struct {
	hitPoints int
	// slots[k] is the number of slots of spell level k+1.
	slots        [maxSpellLevel]int
	pactLevel    int
	pactSlots    int
	hitDice      []rules.HitDice
	hitDiceTotal int
}

// maxima derives the maximums from a stored sheet. A player character
// always has a full sheet; anything else has no hit points, slots or dice.
func (s *Service) maxima(characterID string, doc []byte) (vitalsMax, error) {
	sheet, err := loadSheet(characterID, doc)
	if err != nil {
		return vitalsMax{}, err
	}
	full := sheet.GetFull()
	if full == nil {
		return vitalsMax{}, nil
	}
	d := rules.Derive(buildOf(full), s.rules)
	m := vitalsMax{hitPoints: max(d.HitPointsMax, 0), hitDice: d.HitDice}
	for i, n := range d.SpellSlots {
		if i < maxSpellLevel {
			m.slots[i] = max(n, 0)
		}
	}
	if d.PactMagic != nil {
		m.pactLevel, m.pactSlots = d.PactMagic.SlotLevel, max(d.PactMagic.Slots, 0)
	}
	for _, hd := range d.HitDice {
		m.hitDiceTotal += hd.Count
	}
	return m, nil
}

// vitalsToProto merges the stored values with the maximums, clamping each
// value to its maximum: a sheet that lost a level, or a class, never shows
// more than it has now.
func vitalsToProto(row vitalsRow, m vitalsMax) *playv1.CharacterVitals {
	v := &playv1.CharacterVitals{
		CharacterId:        row.ID,
		Name:               row.Name,
		PlayerUserId:       deref(row.PlayerUserID),
		HitPointsCurrent:   i32(m.hitPoints), // fresh: full hit points
		HitPointsMax:       i32(m.hitPoints),
		HitPointsTemporary: derefInt(row.HitPointsTemporary),
		HitDiceTotal:       i32(m.hitDiceTotal),
		HitDiceUsed:        min(derefInt(row.HitDiceUsed), i32(m.hitDiceTotal)),
		Revision:           derefInt(row.Revision),
		UpdatedAt:          timestamp(row.UpdatedAt),
	}
	if row.HitPointsCurrent != nil {
		v.HitPointsCurrent = min(*row.HitPointsCurrent, v.HitPointsMax)
	}
	for k, total := range m.slots {
		if total == 0 {
			continue
		}
		var used int32
		if k < len(row.SpellSlotsUsed) {
			used = min(row.SpellSlotsUsed[k], i32(total))
		}
		v.SpellSlots = append(v.SpellSlots, &playv1.SpellSlotUsage{Level: i32(k + 1), Total: i32(total), Used: used})
	}
	if m.pactSlots > 0 {
		v.PactSlots = &playv1.PactSlotUsage{
			SlotLevel: i32(m.pactLevel),
			Total:     i32(m.pactSlots),
			Used:      min(derefInt(row.PactSlotsUsed), i32(m.pactSlots)),
		}
	}
	for _, hd := range m.hitDice {
		v.HitDice = append(v.HitDice, &rulesv1.HitDice{Faces: i32(hd.Die), Count: i32(hd.Count)})
	}
	return v
}

// ListVitals returns the vitals of the campaign's living, active player
// characters, oldest first (RN-02): not NPCs, not the dead, not a
// character waiting for approval. Package play calls it after its own
// authorization check, and chooses which of them the caller may see.
func (s *Service) ListVitals(ctx context.Context, campaignID string) ([]*playv1.CharacterVitals, error) {
	rows, err := s.queries.ListVitals(ctx, campaignID)
	if err != nil {
		return nil, s.dbError(ctx, "list vitals", err)
	}
	out := make([]*playv1.CharacterVitals, 0, len(rows))
	for _, row := range rows {
		m, err := s.maxima(row.ID, row.Sheet)
		if err != nil {
			return nil, s.dbError(ctx, "list vitals", err)
		}
		out = append(out, vitalsToProto(row, m))
	}
	return out, nil
}

// GetVitals returns one living, active player character's vitals, or a
// `not_found` Connect error when characterID is not one in the campaign.
func (s *Service) GetVitals(ctx context.Context, campaignID, characterID string) (*playv1.CharacterVitals, error) {
	return s.getVitals(ctx, s.queries, campaignID, characterID)
}

// GetVitalsTx is GetVitals inside tx, so a change that computes from the
// vitals and writes them back (the master applying damage) reads what its own
// transaction will overwrite, never a stale copy.
func (s *Service) GetVitalsTx(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (*playv1.CharacterVitals, error) {
	return s.getVitals(ctx, s.queries.WithTx(tx), campaignID, characterID)
}

func (s *Service) getVitals(ctx context.Context, q *charactersdb.Queries, campaignID, characterID string) (*playv1.CharacterVitals, error) {
	id, ok := parseUUID(characterID)
	if !ok {
		return nil, errCharacterNotFound()
	}
	row, err := q.GetVitals(ctx, charactersdb.GetVitalsParams{CampaignID: campaignID, ID: id})
	if err != nil {
		return nil, s.dbError(ctx, "get vitals", err) // no row: not_found
	}
	m, err := s.maxima(row.ID, row.Sheet)
	if err != nil {
		return nil, s.dbError(ctx, "get vitals", err)
	}
	return vitalsToProto(vitalsRow(row), m), nil
}

// AdjustVitals applies the master's correction (RN-02) inside tx, and
// returns the vitals before and after it. Each value set in req replaces
// the current one; the rest stay. It reads only the vitals fields of req,
// and its errors are Connect errors to return as they are:
//   - `not_found`: characterID is not a living, active player character of
//     the campaign;
//   - `invalid_argument`: nothing to change, or a value outside 0 to its
//     maximum, naming the request's field.
//
// Package play calls it from AdjustCharacterVitals, after checking that
// the caller is the campaign's master and that a session is open, in the
// transaction that also writes the session event. It takes no caller on
// purpose, like LockSheets.
func (s *Service) AdjustVitals(ctx context.Context, tx pgx.Tx, campaignID, characterID string, req *playv1.AdjustCharacterVitalsRequest) (before, after *playv1.CharacterVitals, err error) {
	id, ok := parseUUID(characterID)
	if !ok {
		return nil, nil, errCharacterNotFound()
	}
	q := s.queries.WithTx(tx)
	row, err := q.GetVitals(ctx, charactersdb.GetVitalsParams{CampaignID: campaignID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, errCharacterNotFound()
	}
	if err != nil {
		return nil, nil, wrap("get vitals", err)
	}
	m, err := s.maxima(row.ID, row.Sheet)
	if err != nil {
		return nil, nil, err
	}
	before = vitalsToProto(vitalsRow(row), m)
	after = proto.CloneOf(before)
	if err := applyVitalsChange(after, req); err != nil {
		return nil, nil, invalidArgument(err)
	}

	used := make([]int32, maxSpellLevel)
	for _, slot := range after.GetSpellSlots() {
		used[slot.GetLevel()-1] = slot.GetUsed()
	}
	saved, err := q.UpsertVitals(ctx, charactersdb.UpsertVitalsParams{
		CharacterID:        row.ID,
		HitPointsCurrent:   after.GetHitPointsCurrent(),
		HitPointsTemporary: after.GetHitPointsTemporary(),
		SpellSlotsUsed:     used,
		PactSlotsUsed:      after.GetPactSlots().GetUsed(),
		HitDiceUsed:        after.GetHitDiceUsed(),
		Now:                s.now(),
	})
	if err != nil {
		return nil, nil, wrap("save vitals", err)
	}
	after.Revision = saved.Revision
	after.UpdatedAt = timestamppb.New(saved.UpdatedAt)
	return before, after, nil
}

// applyVitalsChange sets on v the values req sets, checking each against
// its maximum. v already has the maximums. Errors are fieldErrors, with the
// request's field names.
func applyVitalsChange(v *playv1.CharacterVitals, req *playv1.AdjustCharacterVitalsRequest) error {
	if req.HitPointsCurrent == nil && req.HitPointsTemporary == nil && len(req.GetSpellSlotsUsed()) == 0 &&
		req.PactSlotsUsed == nil && req.HitDiceUsed == nil {
		return fieldErr("request", "must set at least one value to change")
	}
	if req.HitPointsCurrent != nil {
		if err := inRange("hit_points_current", req.GetHitPointsCurrent(), v.GetHitPointsMax()); err != nil {
			return err
		}
		v.HitPointsCurrent = req.GetHitPointsCurrent()
	}
	if req.HitPointsTemporary != nil {
		if err := inRange("hit_points_temporary", req.GetHitPointsTemporary(), MaxTemporaryHitPoints); err != nil {
			return err
		}
		v.HitPointsTemporary = req.GetHitPointsTemporary()
	}
	seen := map[int32]bool{}
	for i, change := range req.GetSpellSlotsUsed() {
		field := fmt.Sprintf("spell_slots_used[%d]", i)
		level := change.GetLevel()
		if seen[level] {
			return fieldErr(field+".level", "repeats level %d", level)
		}
		seen[level] = true
		slot := slotOfLevel(v, level)
		if slot == nil {
			return fieldErr(field+".level", "must be a spell level the character has slots of")
		}
		if err := inRange(field+".used", change.GetUsed(), slot.GetTotal()); err != nil {
			return err
		}
		slot.Used = change.GetUsed()
	}
	if req.PactSlotsUsed != nil {
		if v.GetPactSlots() == nil {
			return fieldErr("pact_slots_used", "must be unset: the character has no pact magic slots")
		}
		if err := inRange("pact_slots_used", req.GetPactSlotsUsed(), v.GetPactSlots().GetTotal()); err != nil {
			return err
		}
		v.PactSlots.Used = req.GetPactSlotsUsed()
	}
	if req.HitDiceUsed != nil {
		if err := inRange("hit_dice_used", req.GetHitDiceUsed(), v.GetHitDiceTotal()); err != nil {
			return err
		}
		v.HitDiceUsed = req.GetHitDiceUsed()
	}
	return nil
}

func slotOfLevel(v *playv1.CharacterVitals, level int32) *playv1.SpellSlotUsage {
	for _, slot := range v.GetSpellSlots() {
		if slot.GetLevel() == level {
			return slot
		}
	}
	return nil
}

func inRange(field string, value, maximum int32) error {
	if value < 0 || value > maximum {
		return fieldErr(field, "must be 0 to %d", maximum)
	}
	return nil
}

func derefInt(n *int32) int32 {
	if n == nil {
		return 0
	}
	return *n
}
