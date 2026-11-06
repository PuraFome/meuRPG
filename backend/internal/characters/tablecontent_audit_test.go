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
	if !strings.Contains(buf.String(), `"campaigns":1,"left_out":0,"failed":0`) {
		t.Errorf("the summary does not say 1 campaign and 0 left out:\n%s", buf.String())
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

// TestAuditTableContentLogsOnlyTheReasonOfACampaignThatDoesNotBuild: a campaign refused as a
// whole (over the entry limit) is counted and logged with its ID and a reason code, never the engine's text.
func TestAuditTableContentLogsOnlyTheReasonOfACampaignThatDoesNotBuild(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	_, err := f.h.pool.Exec(t.Context(), `INSERT INTO campaign_content (campaign_id, content_key, kind, name_pt, data, revision, created_at, updated_at)
		SELECT campaign_id, 'race:copia-' || i || '@mesa', kind, 'Copia ' || i, data, revision, created_at, updated_at
		FROM campaign_content, generate_series(1, 1000) AS i WHERE campaign_id = $1 AND content_key = $2`, f.campaign, f.race.GetKey())
	if err != nil {
		t.Fatalf("store 1000 more entries: %v", err)
	}
	var buf bytes.Buffer
	report, err := AuditTableContent(t.Context(), f.h.pool, f.h.svc.content.(*TableSource).srd, slog.New(slog.NewJSONHandler(&buf, nil)))
	if err != nil || report.Failed != 1 {
		t.Fatalf("AuditTableContent() = %+v, %v; want one failed campaign and no error", report, err)
	}
	out := buf.String()
	if !strings.Contains(out, f.campaign) || !strings.Contains(out, `"reason":`) {
		t.Errorf("the log lacks the campaign or a reason code:\n%s", out)
	}
	for _, text := range []string{"Gnomo de teste", "Copia ", "overlay:"} {
		if strings.Contains(out, text) {
			t.Errorf("the log holds %q, want the reason code only:\n%s", text, out)
		}
	}
}

// TestAuditTableContentChecksACampaignWithRowsAndNoRevision: a campaign whose revision row is gone reads as revision 0.
func TestAuditTableContentChecksACampaignWithRowsAndNoRevision(t *testing.T) {
	t.Parallel()
	f := newTolerantTable(t)
	if _, err := f.h.pool.Exec(t.Context(), `DELETE FROM campaign_content_state WHERE campaign_id = $1`, f.campaign); err != nil {
		t.Fatal(err)
	}
	report, err := AuditTableContent(t.Context(), f.h.pool, f.h.svc.content.(*TableSource).srd, slog.New(slog.DiscardHandler))
	if err != nil || report.Failed != 0 || report.Campaigns != 1 {
		t.Errorf("AuditTableContent() = %+v, %v; want the campaign checked", report, err)
	}
}
