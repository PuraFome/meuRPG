package rules

import "testing"

// The effects the monk's riders put on a target (Open Hand technique, Stunning Strike) are
// in the catalog and end with the monk's next turn.
func TestMonkRiderEffects(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	stun, ok := c.CombatEffect("effect:stunning-strike")
	if !ok || len(stun.Conditions) != 1 || stun.Conditions[0] != "condition:stunned" || stun.Duration.Kind != EffectDurationUntilEndOfTurnOf {
		t.Errorf("effect:stunning-strike = %+v, want stunned until the end of a turn", stun)
	}
	open, ok := c.CombatEffect("effect:open-hand-no-reactions")
	if !ok || len(open.Modifiers) != 1 || open.Modifiers[0].Kind != ModifierNoReaction || open.Duration.Kind != EffectDurationUntilEndOfTurnOf {
		t.Errorf("effect:open-hand-no-reactions = %+v, want no_reaction until the end of a turn", open)
	}
}
