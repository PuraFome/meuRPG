package maps

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

// The read deadline of the upload surfaces as os.ErrDeadlineExceeded from the
// body; the answer is a 400 about a slow upload, not a 500.
func TestFormErrorForASlowUpload(t *testing.T) {
	t.Parallel()
	err := formError(fmt.Errorf("read: %w", os.ErrDeadlineExceeded), "cannot read the file")
	he, ok := errors.AsType[*httpError](err)
	if !ok || he.reason != ReasonMalformedRequest || he.status != 400 {
		t.Fatalf("formError() = %#v, want a 400 MALFORMED_REQUEST", err)
	}
}
