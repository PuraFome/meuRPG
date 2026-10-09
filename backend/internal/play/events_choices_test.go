package play

import (
	"slices"
	"testing"
)

// TestTheChoicesCompletedEventIsAppendable: package characters writes it through
// AppendEvent while a session is open, and AppendEvent refuses a kind that is not
// on this list (RN-32).
func TestTheChoicesCompletedEventIsAppendable(t *testing.T) {
	t.Parallel()
	if !slices.Contains(appendableKinds, eventCharacterChoicesCompleted) {
		t.Fatalf("%q is not in appendableKinds", eventCharacterChoicesCompleted)
	}
}
