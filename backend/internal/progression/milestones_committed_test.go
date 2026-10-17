package progression

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	progressionv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/progression/v1"
	"github.com/PuraFome/meuRPG/backend/internal/progression/progressiondb"
)

// A write that is committed answers with what it wrote even when the read of
// the list after it fails: an error would make the app retry a write that is
// already there.
func TestCommittedViewsLogsAFailedReadAndGivesNoList(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	s := &Service{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	got := s.committedViews(t.Context(), "add a milestone", func() ([]*progressionv1.Milestone, error) {
		return nil, errors.New("connection reset")
	})
	if got != nil {
		t.Errorf("views = %v, want none", got)
	}
	// The module prefix and the level are what the service tests and the
	// operator's alerts read.
	for _, want := range []string{`"level":"ERROR"`, `"msg":"progression: cannot read`, "add a milestone", "connection reset"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q does not name %q", logs.String(), want)
		}
	}
}

func TestCommittedViewsPassesTheReadOn(t *testing.T) {
	t.Parallel()
	s := &Service{logger: slog.New(slog.DiscardHandler)}
	list := []*progressionv1.Milestone{{Id: "a"}}
	got := s.committedViews(t.Context(), "add a milestone", func() ([]*progressionv1.Milestone, error) { return list, nil })
	if len(got) != 1 || got[0].GetId() != "a" {
		t.Errorf("views = %v, want the list read", got)
	}
}

func TestWrittenMilestone(t *testing.T) {
	t.Parallel()
	row := progressiondb.PlannedMilestone{ID: "m1", Text: "Chegar ao Vale Seco"}
	listed := &progressionv1.Milestone{Id: "m1", Text: "Chegar ao Vale Seco", HasHistory: true}
	if got := writtenMilestone([]*progressionv1.Milestone{{Id: "m0"}, listed}, row); got != listed {
		t.Errorf("milestone = %v, want the listed one", got)
	}
	got := writtenMilestone(nil, row)
	if got.GetId() != "m1" || got.GetText() != "Chegar ao Vale Seco" || got.GetReached() {
		t.Errorf("milestone without the list = %v, want the written row as a planned milestone", got)
	}
}
