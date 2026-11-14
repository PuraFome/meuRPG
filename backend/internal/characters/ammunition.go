package characters

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
	"github.com/PuraFome/meuRPG/backend/internal/platform/uid"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// Ammunition (SRD 5.1, "Weapons": the ammunition property): what a character holds of each kind lives in
// character_ammunition, one row per kind. A read fills FullSheet.ammunition for the owner and the master; a sheet's
// save never touches the table. Package play spends a piece in the transaction of a ranged attack and calls
// RecoverAmmunition after the combat; the methods below never ask who the caller is.

// MaxAmmunition is the most pieces of one kind a character holds.
const MaxAmmunition = 9999

// The refusals are package link's, so package play tells them apart without importing this one.
var (
	// ErrNoAmmunition: no piece of the kind is left (or the character never held any).
	ErrNoAmmunition = link.ErrNoAmmunition
	// ErrAmmunitionRecovered: the pieces of the character's last combat were already taken back.
	ErrAmmunitionRecovered = link.ErrAmmunitionRecovered
	// ErrNothingToRecover: the character fired nothing in that combat.
	ErrNothingToRecover = link.ErrNothingToRecover
)

// fillAmmunition fills FullSheet.ammunition from the stored rows: one entry per kind with pieces or fired in the
// last combat, sorted by key. The caller is the character's player or the master, so the counts reach nobody else
// (RN-10).
func (s *Service) fillAmmunition(ctx context.Context, content *rules.Content, c *charactersv1.Character) error {
	full := c.GetSheet().GetFull()
	if full == nil {
		return nil
	}
	rows, err := s.queries.ListAmmunition(ctx, c.GetId())
	if err != nil {
		return wrap("read ammunition", err)
	}
	full.Ammunition = nil
	for _, r := range rows {
		full.Ammunition = append(full.Ammunition, &charactersv1.SheetAmmunition{
			Key: r.AmmoKey, NamePt: content.AmmunitionNamePT(r.AmmoKey), Quantity: r.Quantity,
			SpentLastCombat: r.SpentLastCombat, Recovered: r.Recovered,
		})
	}
	return nil
}

// ammunitionCharacter checks that the character is in the campaign and returns its canonical id.
func (s *Service) ammunitionCharacter(ctx context.Context, q *charactersdb.Queries, campaignID, characterID string) (string, error) {
	id, ok := uid.Canonical(characterID)
	if !ok {
		return "", errCharacterNotFound()
	}
	if _, err := q.GetCharacter(ctx, charactersdb.GetCharacterParams{CampaignID: campaignID, ID: id}); errors.Is(err, pgx.ErrNoRows) {
		return "", errCharacterNotFound()
	} else if err != nil {
		return "", wrap("read the character", err)
	}
	return id, nil
}

// SetAmmunition sets the pieces a character holds of a kind (the key must be a catalog ammunition, the count 0 to
// MaxAmmunition) and returns the kind's state. What was fired in the last combat stays. Who may is play's rule.
func (s *Service) SetAmmunition(ctx context.Context, tx pgx.Tx, campaignID, characterID, ammoKey string, quantity int32) (*charactersv1.SheetAmmunition, error) {
	q := s.queries.WithTx(tx)
	id, err := s.ammunitionCharacter(ctx, q, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, wrap("read rules content", err)
	}
	if !slices.Contains(content.AmmunitionKinds(), ammoKey) {
		return nil, invalidArgument(fieldErr("ammo_key", "is not a kind of ammunition"))
	}
	if quantity < 0 || quantity > MaxAmmunition {
		return nil, invalidArgument(fieldErr("quantity", "must be 0 to %d", MaxAmmunition))
	}
	row, err := q.SetAmmunitionQuantity(ctx, charactersdb.SetAmmunitionQuantityParams{CharacterID: id, AmmoKey: ammoKey, Quantity: quantity, Now: s.now()})
	if err != nil {
		return nil, wrap("set ammunition", err)
	}
	return &charactersv1.SheetAmmunition{
		Key: ammoKey, NamePt: content.AmmunitionNamePT(ammoKey), Quantity: row.Quantity, SpentLastCombat: row.SpentLastCombat, Recovered: row.Recovered,
	}, nil
}

// SpendAmmunition takes one piece of a kind for a shot in the encounter and returns the pieces left. A combat other
// than the one the row last counted starts the count over (spent 0, not recovered) before the shot. ErrNoAmmunition
// when none is left or the character never held the kind; nothing is written then. The row is locked, so two
// shots at the last piece never both fire. invalid_argument for a key that is no kind of ammunition.
func (s *Service) SpendAmmunition(ctx context.Context, tx pgx.Tx, campaignID, characterID, ammoKey, encounterID string) (left int32, err error) {
	q := s.queries.WithTx(tx)
	id, err := s.ammunitionCharacter(ctx, q, campaignID, characterID)
	if err != nil {
		return 0, err
	}
	enc, ok := uid.Canonical(encounterID)
	if !ok {
		return 0, invalidArgument(fieldErr("encounter_id", "is not an id"))
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return 0, wrap("read rules content", err)
	}
	if !slices.Contains(content.AmmunitionKinds(), ammoKey) {
		return 0, invalidArgument(fieldErr("ammo_key", "is not a kind of ammunition"))
	}
	row, err := q.GetAmmunitionForUpdate(ctx, charactersdb.GetAmmunitionForUpdateParams{CharacterID: id, AmmoKey: ammoKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoAmmunition
	}
	if err != nil {
		return 0, wrap("lock ammunition", err)
	}
	if row.Quantity <= 0 {
		return 0, ErrNoAmmunition
	}
	spent, recovered := row.SpentLastCombat, row.Recovered
	if row.LastEncounterID == nil || *row.LastEncounterID != enc {
		spent, recovered = 0, false
	}
	err = q.WriteAmmunitionSpent(ctx, charactersdb.WriteAmmunitionSpentParams{
		CharacterID: id, AmmoKey: ammoKey, Quantity: row.Quantity - 1, SpentLastCombat: spent + 1, LastEncounterID: enc, Recovered: recovered, Now: s.now(),
	})
	if err != nil {
		return 0, wrap("spend ammunition", err)
	}
	return row.Quantity - 1, nil
}

// AmmunitionLeft is the pieces a character of the campaign holds of a kind, 0 when it never held any. `not_found` for
// a character that is not in the campaign. A nil tx reads outside a transaction (the turn's options).
func (s *Service) AmmunitionLeft(ctx context.Context, tx pgx.Tx, campaignID, characterID, ammoKey string) (int32, error) {
	q := s.queriesIn(tx)
	id, err := s.ammunitionCharacter(ctx, q, campaignID, characterID)
	if err != nil {
		return 0, err
	}
	n, err := q.GetAmmunitionQuantity(ctx, charactersdb.GetAmmunitionQuantityParams{CharacterID: id, AmmoKey: ammoKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, wrap("read ammunition", err)
	}
	return n, nil
}

// RecoverAmmunition takes back half (rounded down) of what the character fired in the encounter, for each kind, and
// marks the kinds recovered, also those that give nothing back. It returns the kinds it changed. ErrNothingToRecover
// when the character fired nothing in that encounter, ErrAmmunitionRecovered when every kind of it was taken back.
func (s *Service) RecoverAmmunition(ctx context.Context, tx pgx.Tx, campaignID, characterID, encounterID string) ([]*charactersv1.SheetAmmunition, error) {
	q := s.queries.WithTx(tx)
	id, err := s.ammunitionCharacter(ctx, q, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	enc, ok := uid.Canonical(encounterID)
	if !ok {
		return nil, invalidArgument(fieldErr("encounter_id", "is not an id"))
	}
	rows, err := q.ListAmmunitionOfEncounterForUpdate(ctx, charactersdb.ListAmmunitionOfEncounterForUpdateParams{CharacterID: id, EncounterID: enc})
	if err != nil {
		return nil, wrap("lock ammunition", err)
	}
	if len(rows) == 0 {
		return nil, ErrNothingToRecover
	}
	content, err := s.contentFor(ctx, tx, campaignID)
	if err != nil {
		return nil, wrap("read rules content", err)
	}
	var out []*charactersv1.SheetAmmunition
	for _, r := range rows {
		if r.Recovered {
			continue
		}
		quantity := min(r.Quantity+r.SpentLastCombat/2, MaxAmmunition)
		if err := q.WriteAmmunitionRecovered(ctx, charactersdb.WriteAmmunitionRecoveredParams{CharacterID: id, AmmoKey: r.AmmoKey, Quantity: quantity, Now: s.now()}); err != nil {
			return nil, wrap("recover ammunition", err)
		}
		out = append(out, &charactersv1.SheetAmmunition{
			Key: r.AmmoKey, NamePt: content.AmmunitionNamePT(r.AmmoKey), Quantity: quantity, SpentLastCombat: r.SpentLastCombat, Recovered: true,
		})
	}
	if len(out) == 0 {
		return nil, ErrAmmunitionRecovered
	}
	return out, nil
}
