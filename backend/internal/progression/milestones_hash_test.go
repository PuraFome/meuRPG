package progression

import (
	"testing"

	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
)

// The hash AddMilestone keeps with its idempotency key is the hash of the text
// the milestone is stored with (trimmed), so a retry that differs only by the
// whitespace around the text is the same request.
func TestAddMilestoneHashIgnoresSurroundingWhitespace(t *testing.T) {
	t.Parallel()
	hashOf := func(raw string) string {
		t.Helper()
		req := &progressionv1.AddMilestoneRequest{CampaignId: "c1", Text: raw, IdempotencyKey: "k"}
		text, err := cleanMilestoneText(raw)
		if err != nil {
			t.Fatalf("cleanMilestoneText(%q) error = %v", raw, err)
		}
		return *addMilestoneHash(req, text)
	}
	if a, b := hashOf("Chegar ao Vale Seco"), hashOf("  Chegar ao Vale Seco \t"); a != b {
		t.Errorf("hash with surrounding whitespace = %s, want the trimmed text's %s", b, a)
	}
	if a, b := hashOf("Chegar ao Vale Seco"), hashOf("Derrotar a Sombra"); a == b {
		t.Error("two different texts hash the same")
	}
}

// The caller's request is never changed by hashing it.
func TestAddMilestoneHashLeavesTheRequestAlone(t *testing.T) {
	t.Parallel()
	req := &progressionv1.AddMilestoneRequest{CampaignId: "c1", Text: "  x ", IdempotencyKey: "k"}
	addMilestoneHash(req, "x")
	if req.GetText() != "  x " {
		t.Errorf("request text = %q, want it untouched", req.GetText())
	}
}
