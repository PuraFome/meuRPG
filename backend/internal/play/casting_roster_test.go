package play

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// The reads of casting outside a combat, for the tests whose roster reads nothing.

func (noRoster) CastingOptions(context.Context, pgx.Tx, string, string) ([]*playv1.CastingSpell, error) {
	return nil, errors.New("not in this test")
}

func (noRoster) OutsideSpell(context.Context, pgx.Tx, string, string, string, int) (link.OutsideSpell, error) {
	return link.OutsideSpell{}, errors.New("not in this test")
}

func (noRoster) MageArmorAC(context.Context, pgx.Tx, string, string) (link.MageArmor, error) {
	return link.MageArmor{}, errors.New("not in this test")
}
