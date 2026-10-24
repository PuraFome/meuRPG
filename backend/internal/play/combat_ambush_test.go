package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

func ambushFacts(round int32, current string, surprised ...string) modeFacts {
	cs := []playdb.Combatant{{ID: "a", OrderIndex: 0}, {ID: "b", OrderIndex: 1}, {ID: "c", OrderIndex: 2}}
	return modeFacts{cs: cs, surprised: surprised, enc: playdb.Encounter{Status: statusActive, Round: round, CurrentCombatantID: &current}}
}

func TestAnAmbusherHasAdvantageOnACreatureThatHasNotActed(t *testing.T) {
	traits := link.Traits{UnactedAdvantage: "feature:golpe-de-emboscada@mesa"}
	var s combat.AttackScene
	ambushFacts(1, "a").ambush(&s, playdb.Combatant{ID: "c", OrderIndex: 2}, traits)
	if s.UnactedAdvantage != "feature:golpe-de-emboscada@mesa" {
		t.Errorf("a target later in the first round: advantage from %q, want the feature", s.UnactedAdvantage)
	}
	s = combat.AttackScene{}
	ambushFacts(1, "c").ambush(&s, playdb.Combatant{ID: "b", OrderIndex: 1}, traits)
	if s.UnactedAdvantage != "" {
		t.Errorf("a target that already acted: advantage from %q, want none", s.UnactedAdvantage)
	}
	s = combat.AttackScene{}
	ambushFacts(2, "a").ambush(&s, playdb.Combatant{ID: "c", OrderIndex: 2}, traits)
	if s.UnactedAdvantage != "" {
		t.Errorf("second round: advantage from %q, want none", s.UnactedAdvantage)
	}
	s = combat.AttackScene{}
	ambushFacts(1, "a").ambush(&s, playdb.Combatant{ID: "c", OrderIndex: 2}, link.Traits{})
	if s.UnactedAdvantage != "" {
		t.Errorf("without the feature: advantage from %q, want none", s.UnactedAdvantage)
	}
}

func TestAHitOnASurprisedCreatureIsCriticalOnlyWithTheFeature(t *testing.T) {
	var s combat.AttackScene
	ambushFacts(1, "a", "c").ambush(&s, playdb.Combatant{ID: "c", OrderIndex: 2}, link.Traits{SurprisedCritical: "feature:golpe-mortal@mesa"})
	if !s.SurprisedCritical {
		t.Error("a surprised target: the hit is not critical")
	}
	s = combat.AttackScene{}
	ambushFacts(1, "a", "c").ambush(&s, playdb.Combatant{ID: "b", OrderIndex: 1}, link.Traits{SurprisedCritical: "feature:golpe-mortal@mesa"})
	if s.SurprisedCritical {
		t.Error("a target that is not surprised: the hit is critical")
	}
	s = combat.AttackScene{}
	ambushFacts(1, "a", "c").ambush(&s, playdb.Combatant{ID: "c", OrderIndex: 2}, link.Traits{})
	if s.SurprisedCritical {
		t.Error("without the feature: the hit is critical")
	}
}
