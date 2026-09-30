package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Shelter of the Faithful": "shelter-of-the-faithful",
		"Researcher":              "researcher",
		"  Two  Spaces! ":         "two-spaces",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSkillChoice(t *testing.T) {
	t.Parallel()
	var o optionSet
	raw := `{"choose": 2, "from": {"options": [
		{"option_type": "reference", "item": {"index": "skill-arcana"}},
		{"option_type": "reference", "item": {"index": "skill-history"}}]}}`
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	c, ok := skillChoice(&o)
	if !ok || c.Choose != 2 || strings.Join(c.From, ",") != "skill:arcana,skill:history" {
		t.Errorf("skillChoice = %+v, %v", c, ok)
	}

	// Instruments are not skills.
	raw = `{"choose": 3, "from": {"options": [{"option_type": "reference", "item": {"index": "lute"}}]}}`
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	if _, ok := skillChoice(&o); ok {
		t.Error("a choice of instruments is not a skill choice")
	}
}

// TestExpertisePicks: the rogue's first Expertise is "choose 1 of: 2
// skills, or 1 skill and thieves' tools", which is 2 picks.
func TestExpertisePicks(t *testing.T) {
	t.Parallel()
	var o optionSet
	raw := `{"choose": 1, "from": {"options": [
		{"option_type": "choice", "choice": {"choose": 2}},
		{"option_type": "multiple", "items": [{}, {}]}]}}`
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		t.Fatal(err)
	}
	if got := o.picks(); got != 2 {
		t.Errorf("picks = %d, want 2", got)
	}
}

// TestRefusesUnpinnedInput: a file that does not match inputHashes stops
// the import before anything is written.
func TestRefusesUnpinnedInput(t *testing.T) {
	t.Parallel()
	src, out := t.TempDir(), t.TempDir()
	for name := range inputHashes {
		if err := os.WriteFile(filepath.Join(src, name), []byte("[]"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err := run(src, out)
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("run = %v, want a sha256 mismatch", err)
	}
	if files, _ := os.ReadDir(out); len(files) != 0 {
		t.Errorf("wrote %d files after refusing the input", len(files))
	}
}

// TestRefusesMissingSource: -src must point at the files.
func TestRefusesMissingSource(t *testing.T) {
	t.Parallel()
	if err := run(filepath.Join(t.TempDir(), "nope"), t.TempDir()); err == nil {
		t.Error("run on a missing folder should fail")
	}
}
