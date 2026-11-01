package play

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
)

// featureRoster answers every summon check as the summon of a table feature, and spellRoster as a spell's.
type (
	featureRoster struct{ noRoster }
	spellRoster   struct{ noRoster }
)

func (spellRoster) CheckSummon(context.Context, pgx.Tx, string, string, string, int, int, []string) (link.SummonSpell, error) {
	return link.SummonSpell{}, nil
}

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

// The other way: a summoning spell that a grant gives (not a feature's own) is cast with the classes, never through
// granted_by, as in a combat.
func TestTheSummonCheckRefusesASpellSummonCastThroughAGrant(t *testing.T) {
	t.Parallel()
	s := &Service{roster: spellRoster{}}
	if _, err := s.checkSummonChoice(t.Context(), nil, "c", "p", "spell:conjure-animals", 3, nil, true); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("checkSummonChoice(a spell through a grant) error = %v, want invalid_argument", err)
	}
	if _, err := s.checkSummonChoice(t.Context(), nil, "c", "p", "spell:conjure-animals", 3, nil, false); err != nil {
		t.Errorf("checkSummonChoice(a spell with a slot) error = %v, want it accepted", err)
	}
}
