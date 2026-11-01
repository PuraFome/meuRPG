package characters

import (
	"reflect"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The rerolls of a sheet reach the combat's traits as they are.
func TestRerollRulesOfCopiesTheDerivedRules(t *testing.T) {
	t.Parallel()
	d := rules.Derived{
		Rerolls: []rules.RerollRule{
			{Applies: []string{rules.RollAppliesAttack, rules.RollAppliesSave}, AtMost: 2, Resource: "sorte", Cost: 2, Source: "feature:a@mesa"},
			{Applies: []string{rules.RollDamage}, AtMost: 1, Cost: 1, Source: "feature:b@mesa"},
		},
	}
	rerolls := rerollRulesOf(d)
	wantR := []link.RerollRule{
		{Applies: []string{"attack", "save"}, AtMost: 2, Resource: "sorte", Cost: 2, Source: "feature:a@mesa"},
		{Applies: []string{"damage"}, AtMost: 1, Cost: 1, Source: "feature:b@mesa"},
	}
	if !reflect.DeepEqual(rerolls, wantR) {
		t.Errorf("rerollRulesOf = %+v; want %+v", rerolls, wantR)
	}
	if r := rerollRulesOf(rules.Derived{}); len(r) != 0 {
		t.Errorf("rerollRulesOf of nothing = %+v; want empty", r)
	}
}
