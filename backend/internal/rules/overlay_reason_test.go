package rules

import (
	"errors"
	"strings"
	"testing"
)

// firstViolation runs With and returns the Reason and Message of the first violation.
func firstViolation(t *testing.T, srd *Content, o Overlay) (reason, msg string) {
	t.Helper()
	_, err := srd.With(o)
	if err == nil {
		return "", ""
	}
	var oe *OverlayError
	if !errors.As(err, &oe) {
		t.Fatalf("error is not an *OverlayError: %v", err)
	}
	v := oe.Violations()
	t.Logf("violations: %d; first: field=%q reason=%q msg=%q", len(v), v[0].Field, v[0].Reason, v[0].Message)
	return v[0].Reason, v[0].Message
}

func overlayGenClass(t *testing.T, o *Overlay) *TableClass {
	t.Helper()
	for i := range o.Classes {
		if o.Classes[i].Key == "class:gen-none"+tableSuffix {
			return &o.Classes[i]
		}
	}
	t.Fatal("no class gen-none")
	return nil
}

// A name or a paragraph over its length limit must carry the documented
// ReasonName / ReasonText, not the generic ReasonLimit.
func TestEntryReasonOfALongNameOrParagraph(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)
	cases := []struct {
		name string
		edit func(o *Overlay, c *TableClass)
		want string
	}{
		{"class name of 81", func(_ *Overlay, c *TableClass) { c.NamePT = strings.Repeat("a", 81) }, ReasonName},
		{"feature name of 81", func(_ *Overlay, c *TableClass) {
			c.Levels[0].Features[0].NamePT = strings.Repeat("a", 81)
		}, ReasonName},
		{"feature paragraph of 4001", func(_ *Overlay, c *TableClass) {
			c.Levels[0].Features[0].DescPT = []string{strings.Repeat("a", 4001)}
		}, ReasonText},
		{"feature with too many paragraphs", func(_ *Overlay, c *TableClass) {
			c.Levels[0].Features[0].DescPT = make([]string, maxTextParagraphs+1)
		}, ReasonLimit},
		{"spell paragraph of 4001", func(o *Overlay, _ *TableClass) { o.Spells[0].DescPT = []string{strings.Repeat("a", 4001)} }, ReasonText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := fullOverlay(t, srd)
			tc.edit(&o, overlayGenClass(t, &o))
			got, msg := firstViolation(t, srd, o)
			if got != tc.want {
				t.Errorf("Reason = %q (message %q), want %q", got, msg, tc.want)
			}
		})
	}
}
