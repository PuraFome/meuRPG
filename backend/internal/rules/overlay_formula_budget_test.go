package rules

import (
	"errors"
	"strings"
	"testing"
)

// The distinct formulas of a table are bounded in their text too: 500 formulas
// of about 950 bytes cost 310 ms and 106 MB of one With, so the sum of the
// distinct texts has a budget (MaxOverlayFormulaBytes).
func TestFormulaBytesAreBudgeted(t *testing.T) {
	t.Parallel()
	srd := loadForTest(t)

	within := bigFormulaOverlay(100)
	if _, err := srd.With(within); err != nil {
		t.Errorf("100 distinct formulas of about 950 bytes (%d bytes): %v, want accepted", MaxOverlayFormulaBytes*95/100, err)
	}

	over := bigFormulaOverlay(MaxOverlayFormulas)
	_, err := srd.With(over)
	var oe *OverlayError
	if !errors.As(err, &oe) || oe.Reason != ReasonLimit || !strings.Contains(oe.Message, "bytes of distinct formulas") {
		t.Errorf("500 distinct formulas of about 950 bytes: err = %v, want the limit of bytes of distinct formulas", err)
	}
}
