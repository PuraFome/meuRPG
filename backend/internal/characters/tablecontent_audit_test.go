package characters

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestAuditTableContentLogsWhatTheRulesWouldLeaveOut: the audit names each entry the rules
// refuse (campaign, key, kind, reason), counts them, changes nothing, and never prints text.
func TestAuditTableContentLogsWhatTheRulesWouldLeaveOut(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	f.h.refuseStored(f.campaign, f.class.GetKey(), f.sp.GetKey())
	before := f.h.countContentRows(t, f.campaign)

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	report, err := AuditTableContent(t.Context(), f.h.pool, f.h.svc.content.(*TableSource).srd, logger)
	if err != nil {
		t.Fatalf("AuditTableContent() error = %v, want nil", err)
	}
	if report.Failed != 0 || report.LeftOut != 3 || report.Campaigns != 1 {
		t.Errorf("report = %+v, want 1 campaign, no failure and its 3 left-out entries", report)
	}
	out := buf.String()
	for _, want := range []string{f.campaign, f.class.GetKey(), f.sub.GetKey(), f.sp.GetKey(), "bad_value", "dangling_reference", "table content audit done"} {
		if !strings.Contains(out, want) {
			t.Errorf("the audit log lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, f.race.GetKey()) || strings.Contains(out, "Guardião") || strings.Contains(out, "Raio de teste") {
		t.Errorf("the audit log names the healthy race or prints entry text:\n%s", out)
	}
	if after := f.h.countContentRows(t, f.campaign); after != before {
		t.Errorf("rows after the audit = %d, want %d: the audit only reads", after, before)
	}
}

// TestAuditTableContentOfAHealthyCampaignLogsNoEntry: the summary line says 0.
func TestAuditTableContentOfAHealthyCampaignLogsNoEntry(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	var buf bytes.Buffer
	if _, err := AuditTableContent(t.Context(), f.h.pool, f.h.svc.content.(*TableSource).srd, slog.New(slog.NewJSONHandler(&buf, nil))); err != nil {
		t.Fatalf("AuditTableContent() error = %v", err)
	}
	if strings.Contains(buf.String(), f.campaign) {
		t.Errorf("a healthy campaign is named in the log:\n%s", buf.String())
	}
}

func (h *harness) countContentRows(t *testing.T, campaign string) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(t.Context(), `SELECT count(*) FROM campaign_content WHERE campaign_id = $1`, campaign).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
