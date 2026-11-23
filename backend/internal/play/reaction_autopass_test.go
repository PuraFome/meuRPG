package play

import "testing"

// The vector of reaction-autopass.spec.ts: the two sides derive the same key.
func TestAutoPassKeyIsTheWebs(t *testing.T) {
	id := "0f1e2d3c-4b5a-4987-a6b5-c4d3e2f1a0b9"
	want := "f0e1d2c3-b4a5-4678-a94a-3b2c1d0e5f46"
	if got := autoPassKeyOf(id); got != want {
		t.Fatalf("autoPassKeyOf = %s, want %s", got, want)
	}
	if got := autoPassKeyOf("0F1E2D3C-4B5A-4987-A6B5-C4D3E2F1A0B9"); got != want {
		t.Fatalf("upper case = %s, want %s", got, want)
	}
	if !isAutoPassKey(id, "F0E1D2C3-B4A5-4678-A94A-3B2C1D0E5F46") || isAutoPassKey(id, id) || isAutoPassKey(id, "") {
		t.Fatal("isAutoPassKey does not tell the auto-pass key from another")
	}
}
