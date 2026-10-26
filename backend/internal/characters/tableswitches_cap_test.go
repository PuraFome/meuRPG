package characters

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The master can switch a whole campaign off in one call: every option of the
// SRD and every entry the table may hold.
func TestOptionSwitchesCapCoversAWholeCampaign(t *testing.T) {
	t.Parallel()
	srdOptions := len(optionEntries(loadRules(t), nil))
	if need := srdOptions + rules.MaxOverlayEntries; MaxOptionSwitches < need {
		t.Errorf("MaxOptionSwitches = %d, want at least %d (%d SRD options and %d table entries)", MaxOptionSwitches, need, srdOptions, rules.MaxOverlayEntries)
	}
}
