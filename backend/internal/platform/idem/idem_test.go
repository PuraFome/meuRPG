package idem

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	notesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/notes/v1"
)

func TestClean(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"none is allowed", "", "", false},
		{"spaces only is none", "  ", "", false},
		{"a UUID", "6b0b9e5e-2b0e-4f0e-9d6f-1c1b6c0f3a11", "6b0b9e5e-2b0e-4f0e-9d6f-1c1b6c0f3a11", false},
		{"trimmed", " abc ", "abc", false},
		{"exactly 64", strings.Repeat("é", 64), strings.Repeat("é", 64), false},
		{"65", strings.Repeat("a", 65), "", true},
		{"control character", "a\nb", "", true},
		{"invalid UTF-8", "a\xffb", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Clean(tt.in)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("Clean(%q) = %q, %v; want %q, error %v", tt.in, got, err, tt.want, tt.wantErr)
			}
			if err != nil && connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("code = %v, want invalid_argument", connect.CodeOf(err))
			}
		})
	}
}

func TestScope(t *testing.T) {
	t.Parallel()
	if Scope("c1", "") != nil {
		t.Error("no key must scope to nil")
	}
	if got := Scope("c1", "k"); got == nil || *got != "c1:k" {
		t.Errorf("Scope = %v, want c1:k", got)
	}
}

func TestHashIgnoresTheKeyAndNothingElse(t *testing.T) {
	t.Parallel()
	a := &notesv1.CreateNoteRequest{CampaignId: "c", Text: "x", IdempotencyKey: "one"}
	b := &notesv1.CreateNoteRequest{CampaignId: "c", Text: "x", IdempotencyKey: "two"}
	c := &notesv1.CreateNoteRequest{CampaignId: "c", Text: "y", IdempotencyKey: "one"}
	if *Hash(a) != *Hash(b) {
		t.Error("the same request with another key must hash the same")
	}
	if *Hash(a) == *Hash(c) {
		t.Error("another request must hash differently")
	}
	if a.GetIdempotencyKey() != "one" {
		t.Error("Hash must not change the request it is given")
	}
	if err := SameRequest(Hash(a), Hash(c)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("SameRequest(other) = %v, want invalid_argument", err)
	}
	if err := SameRequest(Hash(a), Hash(b)); err != nil {
		t.Errorf("SameRequest(same) = %v", err)
	}
	if err := SameRequest(nil, Hash(a)); err == nil {
		t.Error("a row with no hash is another request")
	}
}
