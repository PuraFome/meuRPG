package play

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// featureRoster answers every summon check as the summon of a table feature.
type featureRoster struct{ noRoster }

func (featureRoster) CheckSummon(context.Context, pgx.Tx, string, string, string, int, int, []string) (link.SummonSpell, error) {
	return link.SummonSpell{Feature: true}, nil
}

// The summon of a feature is cast through the feature's grant with no slot: every other way of reaching the check (a
// slot, a ritual, a spell known) is refused before anything is made. The sheet's own validation keeps the hidden spell
// out of the lists, so this is the second guard, checked on its own.
func TestTheSummonCheckRefusesAFeatureCastWithoutItsGrant(t *testing.T) {
	t.Parallel()
	s := &Service{roster: featureRoster{}}
	if _, err := s.checkSummonChoice(t.Context(), nil, "c", "p", "spell:feature-chamado@mesa", 1, nil, false); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("checkSummonChoice(no grant) error = %v, want invalid_argument", err)
	}
	chk, err := s.checkSummonChoice(t.Context(), nil, "c", "p", "spell:feature-chamado@mesa", 1, nil, true)
	if err != nil || !chk.Feature {
		t.Errorf("checkSummonChoice(through the grant) = %+v, %v; want the feature's check accepted", chk, err)
	}
}
