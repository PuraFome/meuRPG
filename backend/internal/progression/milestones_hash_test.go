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

// The hash MoveMilestone keeps with its idempotency key is the request's minus the key: the same
// move hashes the same whatever key it came with, and another milestone or direction does not.
func TestMoveMilestoneHashIsTheRequestWithoutTheKey(t *testing.T) {
	t.Parallel()
	up, down := progressionv1.MilestoneDirection_MILESTONE_DIRECTION_UP, progressionv1.MilestoneDirection_MILESTONE_DIRECTION_DOWN
	hashOf := func(milestone string, dir progressionv1.MilestoneDirection, key string) string {
		return *moveMilestoneHash(&progressionv1.MoveMilestoneRequest{CampaignId: "c1", MilestoneId: milestone, Direction: dir, IdempotencyKey: key})
	}
	base := hashOf("m1", up, "k1")
	if got := hashOf("m1", up, "k2"); got != base {
		t.Errorf("the same move with another key hashes %s, want %s", got, base)
	}
	if got := hashOf("m1", down, "k1"); got == base {
		t.Error("another direction hashes the same")
	}
	if got := hashOf("m2", up, "k1"); got == base {
		t.Error("another milestone hashes the same")
	}
	req := &progressionv1.MoveMilestoneRequest{CampaignId: "c1", MilestoneId: "m1", Direction: up, IdempotencyKey: "k1"}
	moveMilestoneHash(req)
	if req.GetIdempotencyKey() != "k1" {
		t.Errorf("request key = %q, want it untouched", req.GetIdempotencyKey())
	}
}
