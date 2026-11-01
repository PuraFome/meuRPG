package characters

import (
	"reflect"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The rerolls and the roll minimums of a sheet reach the combat's traits as they are.
func TestRollRulesOfCopiesTheDerivedRules(t *testing.T) {
	t.Parallel()
	d := rules.Derived{
		Rerolls: []rules.RerollRule{
			{Applies: []string{rules.RollAppliesAttack, rules.RollAppliesSave}, AtMost: 2, Resource: "sorte", Cost: 2, Source: "feature:a@mesa"},
			{Applies: []string{rules.RollDamage}, AtMost: 1, Cost: 1, Source: "feature:b@mesa"},
		},
		RollMinimums: []rules.RollMinimum{{Applies: []string{rules.RollAppliesCheck}, Value: 10, Source: "feature:c@mesa"}},
	}
	rerolls, minimums := rollRulesOf(d)
	wantR := []link.RerollRule{
		{Applies: []string{"attack", "save"}, AtMost: 2, Resource: "sorte", Cost: 2, Source: "feature:a@mesa"},
		{Applies: []string{"damage"}, AtMost: 1, Cost: 1, Source: "feature:b@mesa"},
	}
	wantM := []link.RollMinimum{{Applies: []string{"check"}, Value: 10, Source: "feature:c@mesa"}}
	if !reflect.DeepEqual(rerolls, wantR) || !reflect.DeepEqual(minimums, wantM) {
		t.Errorf("rollRulesOf = %+v, %+v; want %+v, %+v", rerolls, minimums, wantR, wantM)
	}
	if r, m := rollRulesOf(rules.Derived{}); len(r) != 0 || len(m) != 0 {
		t.Errorf("rollRulesOf of nothing = %+v, %+v; want empty", r, m)
	}
}
