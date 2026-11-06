package play

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// namingRoster names two keys; "feature:retired" is one the master retired, so it has no name.
type namingRoster struct{ CombatRoster }

func (namingRoster) SourceNames(context.Context, pgx.Tx, string) (func(string) string, error) {
	return func(k string) string {
		if k == "feat:dado-sortudo" {
			return "Dado Sortudo da Mesa"
		}
		return ""
	}, nil
}

func TestNameSourcesFillsEveryKeyAResponseCarries(t *testing.T) {
	t.Parallel()
	s := &Service{roster: namingRoster{}, logger: slog.New(slog.DiscardHandler)}
	roll := &playv1.DiceRoll{
		TreatedAsSource: "feat:dado-sortudo",
		Rerolled:        []*playv1.Reroll{{SourceKey: "feat:dado-sortudo"}, {SourceKey: "feature:retired"}, {}},
	}
	offer := &playv1.HeldRollOffer{D20: roll, Question: &playv1.HeldRollOffer_Reroll{Reroll: &playv1.RerollQuestion{SourceKey: "feat:dado-sortudo"}}}
	s.nameSources(t.Context(), "campaign", offer)
	if roll.GetTreatedAsSourceName() != "Dado Sortudo da Mesa" || roll.GetRerolled()[0].GetSourceName() != "Dado Sortudo da Mesa" || offer.GetReroll().GetSourceName() != "Dado Sortudo da Mesa" {
		t.Errorf("names = %q %q %q, want the feature's name on the roll, its reroll and the question", roll.GetTreatedAsSourceName(), roll.GetRerolled()[0].GetSourceName(), offer.GetReroll().GetSourceName())
	}
	if roll.GetRerolled()[1].GetSourceName() != "" || roll.GetRerolled()[2].GetSourceName() != "" {
		t.Errorf("a retired entry or no key got a name: %v", roll.GetRerolled())
	}
}

func TestNameSourcesNamesNothingWithoutAKeyOrACampaign(t *testing.T) {
	t.Parallel()
	s := &Service{roster: namingRoster{}, logger: slog.New(slog.DiscardHandler)}
	roll := &playv1.DiceRoll{Rerolled: []*playv1.Reroll{{SourceKey: "feat:dado-sortudo"}}}
	s.nameSources(t.Context(), "", roll)
	if roll.GetRerolled()[0].GetSourceName() != "" {
		t.Errorf("a response with no campaign got a name")
	}
}

func TestRerollQuestionCarriesTheD20Sides(t *testing.T) {
	t.Parallel()
	q := heldOffer{Hold: heldRoll{RerollSource: "feat:x"}}.rerollOffer()
	if q.GetSides() != 20 {
		t.Errorf("sides = %d, want 20: the prompt draws the d20 from it", q.GetSides())
	}
	out := heldRollOfferOf(heldOffer{Asks: askReroll, Hold: heldRoll{Faces: []int{5}}}, offerRoller{})
	if out.GetD20().GetDiceSides() != 20 || out.GetReroll().GetSides() != 20 {
		t.Errorf("offer sides = %d and %d, want 20 on the d20 and on the question", out.GetD20().GetDiceSides(), out.GetReroll().GetSides())
	}
}
