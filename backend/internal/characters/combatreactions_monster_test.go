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
	for key, want := range map[string]struct {
		bonus int
		text  string
	}{
		"monster:knight":   {2, "The knight adds 2 to its AC against one melee attack that would hit it. To do so, the knight must see the attacker and be wielding a melee weapon."},
		"monster:marilith": {5, "The marilith adds 5 to its AC against one melee attack that would hit it. To do so, the marilith must see the attacker and be wielding a melee weapon."},
	} {
		got := creatureReactionStats(srd, key).MonsterReactions
		if len(got) != 1 || got[0] != (link.MonsterReaction{Key: "parry", Name: "Parry", Text: want.text, Kind: "parry", ACBonus: want.bonus}) {
			t.Errorf("%s reactions = %+v, want one Parry of +%d", key, got, want.bonus)
		}
	}
	if got := creatureReactionStats(srd, "monster:goblin").MonsterReactions; len(got) != 0 {
		t.Errorf("goblin reactions = %+v, want none", got)
	}
}
