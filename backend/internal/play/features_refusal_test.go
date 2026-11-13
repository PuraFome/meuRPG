package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A disabled row whose code UseFeature does not map is refused all the same: nothing disabled ever succeeds (the call fails
// closed). The master's economy override is the one way past a disabled row, and only for the economy's codes.
func TestFeatureRefusalFailsClosed(t *testing.T) {
	t.Parallel()
	code := func(c rulesv1.DisabledReasonCode) *rulesv1.DisabledReason { return &rulesv1.DisabledReason{Code: c} }
	for _, c := range []rulesv1.DisabledReasonCode{
		rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_SLOT,
		rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_ONLY,
		rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_CASTING_TIME_TOO_LONG,
		rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ARMOR_NOT_PROFICIENT,
		rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NOT_YOUR_TURN,
	} {
		for _, master := range []bool{false, true} {
			if featureRefusal(code(c), master) == nil {
				t.Errorf("a row disabled by %v (master %v) is let through", c, master)
			}
		}
	}
	err := featureRefusal(code(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_SLOT), false)
	if b := encounterBlocked(t, err); b.GetReason() != playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_SLOT {
		t.Errorf("NO_SLOT is %v", b.GetReason())
	}
	if got := connect.CodeOf(featureRefusal(code(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_REACTION_ONLY), false)); got != connect.CodeFailedPrecondition {
		t.Errorf("an unmapped code is %v, want failed_precondition", got)
	}
	// The economy is the master's to override; the uses are not.
	if err := featureRefusal(code(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED), true); err != nil {
		t.Errorf("the master with the action used = %v, want it allowed", err)
	}
	if featureRefusal(code(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_ACTION_USED), false) == nil {
		t.Error("a player with the action used is let through")
	}
	if featureRefusal(code(rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_USES), true) == nil {
		t.Error("the master past the uses is let through")
	}
	if featureRefusal(nil, false) != nil {
		t.Error("an enabled feature is refused")
	}
}
