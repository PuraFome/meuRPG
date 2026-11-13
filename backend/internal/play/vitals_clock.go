package play

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// clockedVitals is the vitals keeper with the one thing the characters module does not know: how
// long a Wild Shape form has left. The clock is a lasting effect, which is play's (docs/architecture/
// play/lasting-effects.md), so play reads it and puts it on every WildShapeState that goes by.
// The characters module keeps the form; it never reads where the clock lives.
type clockedVitals struct {
	VitalsKeeper
	db playdb.DBTX
}

// withClock puts the time left on the vitals that carry a form. A form with no clock stays unset.
func withClock(ctx context.Context, db playdb.DBTX, vs ...*playv1.CharacterVitals) error {
	q := playdb.New(db)
	for _, v := range vs {
		if v == nil || v.GetWildShape() == nil {
			continue
		}
		left, err := q.FormClockSecondsLeft(ctx, v.GetCharacterId())
		if err != nil {
			return fmt.Errorf("read the form's clock: %w", err)
		}
		v.WildShape.SecondsLeft = nil
		if left >= 0 {
			v.WildShape.SecondsLeft = &left
		}
	}
	return nil
}

func (k clockedVitals) ListVitals(ctx context.Context, campaignID string) ([]*playv1.CharacterVitals, error) {
	vs, err := k.VitalsKeeper.ListVitals(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	return vs, withClock(ctx, k.db, vs...)
}

func (k clockedVitals) GetVitals(ctx context.Context, campaignID, characterID string) (*playv1.CharacterVitals, error) {
	v, err := k.VitalsKeeper.GetVitals(ctx, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	return v, withClock(ctx, k.db, v)
}

func (k clockedVitals) GetVitalsTx(ctx context.Context, tx pgx.Tx, campaignID, characterID string) (*playv1.CharacterVitals, error) {
	v, err := k.VitalsKeeper.GetVitalsTx(ctx, tx, campaignID, characterID)
	if err != nil {
		return nil, err
	}
	return v, withClock(ctx, tx, v)
}

// The changes answer with the vitals before and after: both read the clock inside the same transaction.

func (k clockedVitals) AdjustVitals(ctx context.Context, tx pgx.Tx, campaignID, characterID string, req *playv1.AdjustCharacterVitalsRequest) (before, after *playv1.CharacterVitals, err error) {
	before, after, err = k.VitalsKeeper.AdjustVitals(ctx, tx, campaignID, characterID, req)
	return before, after, clockedPair(ctx, tx, err, before, after)
}

func (k clockedVitals) SetExhaustion(ctx context.Context, tx pgx.Tx, campaignID, characterID string, level int32) (before, after *playv1.CharacterVitals, err error) {
	before, after, err = k.VitalsKeeper.SetExhaustion(ctx, tx, campaignID, characterID, level)
	return before, after, clockedPair(ctx, tx, err, before, after)
}

func (k clockedVitals) SetArmorBase(ctx context.Context, tx pgx.Tx, campaignID, characterID string, armorClass int32) (before, after *playv1.CharacterVitals, err error) {
	before, after, err = k.VitalsKeeper.SetArmorBase(ctx, tx, campaignID, characterID, armorClass)
	return before, after, clockedPair(ctx, tx, err, before, after)
}

func (k clockedVitals) SetActiveStates(ctx context.Context, tx pgx.Tx, campaignID, characterID string, states []*playv1.ActiveState) (before, after *playv1.CharacterVitals, err error) {
	before, after, err = k.VitalsKeeper.SetActiveStates(ctx, tx, campaignID, characterID, states)
	return before, after, clockedPair(ctx, tx, err, before, after)
}

func (k clockedVitals) SetHitPointsMaxBonus(ctx context.Context, tx pgx.Tx, campaignID, characterID string, bonus int32) (before, after *playv1.CharacterVitals, err error) {
	before, after, err = k.VitalsKeeper.SetHitPointsMaxBonus(ctx, tx, campaignID, characterID, bonus)
	return before, after, clockedPair(ctx, tx, err, before, after)
}

func (k clockedVitals) AssumeWildShape(ctx context.Context, tx pgx.Tx, campaignID, characterID, beast string) (before, after *playv1.CharacterVitals, body link.Character, err error) {
	before, after, body, err = k.VitalsKeeper.AssumeWildShape(ctx, tx, campaignID, characterID, beast)
	return before, after, body, clockedPair(ctx, tx, err, before, after)
}

func (k clockedVitals) SetWildShape(ctx context.Context, tx pgx.Tx, campaignID, characterID, beast string, hp int32) (after *playv1.CharacterVitals, body link.Character, err error) {
	after, body, err = k.VitalsKeeper.SetWildShape(ctx, tx, campaignID, characterID, beast, hp)
	return after, body, clockedPair(ctx, tx, err, after)
}

func (k clockedVitals) SetFamiliarSight(ctx context.Context, tx pgx.Tx, campaignID, characterID, creatureID string, inCombat bool, conditions []string) (*playv1.CharacterVitals, error) {
	v, err := k.VitalsKeeper.SetFamiliarSight(ctx, tx, campaignID, characterID, creatureID, inCombat, conditions)
	if err != nil {
		return v, err
	}
	return v, withClock(ctx, tx, v)
}

// clockedPair is the clock on the vitals a change returned, unless the change failed.
func clockedPair(ctx context.Context, db playdb.DBTX, err error, vs ...*playv1.CharacterVitals) error {
	if err != nil {
		return err
	}
	return withClock(ctx, db, vs...)
}

// clockedVitalsOf wraps the keeper, unless there is none (a test that builds the service bare).
func clockedVitalsOf(k VitalsKeeper, pool playdb.DBTX) VitalsKeeper {
	if k == nil {
		return nil
	}
	return clockedVitals{VitalsKeeper: k, db: pool}
}
