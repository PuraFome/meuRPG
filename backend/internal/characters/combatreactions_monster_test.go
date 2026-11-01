package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// A stat block's reactions reach the combat through ReactionStats (RN-31): the Knight's
// Parry adds 2 to its armor class, the Marilith's 5; a goblin has none.
func TestCreatureReactionStatsHoldTheStatBlocksReactions(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	for key, bonus := range map[string]int{
		"monster:knight":   2,
		"monster:marilith": 5,
	} {
		got := creatureReactionStats(srd, key).MonsterReactions
		if len(got) != 1 || got[0] != (link.MonsterReaction{Kind: "parry", ACBonus: bonus}) {
			t.Errorf("%s reactions = %+v, want one Parry of +%d", key, got, bonus)
		}
	}
	if got := creatureReactionStats(srd, "monster:goblin").MonsterReactions; len(got) != 0 {
		t.Errorf("goblin reactions = %+v, want none", got)
	}
}
