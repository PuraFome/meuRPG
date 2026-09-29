package names

import (
	"errors"
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"plain", "Mirathel", "Mirathel", false},
		{"trims spaces at both ends", "  Mirathel \u00a0", "Mirathel", false},
		{"keeps inner spaces", "A Maldição de Strahd", "A Maldição de Strahd", false},
		{"emoji", "Mesa de quinta 🐉", "Mesa de quinta 🐉", false},
		{"exactly the limit", strings.Repeat("é", 20), strings.Repeat("é", 20), false},
		{"empty", "", "", true},
		{"only spaces", " \t\n ", "", true},
		{"one character over", strings.Repeat("é", 21), "", true},
		{"line break", "Mira\nthel", "", true},
		{"tab", "Mira\tthel", "", true},
		{"NUL", "Mira\x00thel", "", true},
		{"right-to-left override", "Mira\u202ethel", "", true},
		{"invalid UTF-8", "Mira\xffthel", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Clean(tt.input, 20)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Clean(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	if _, err := Clean("   ", 10); !errors.Is(err, ErrEmpty) {
		t.Errorf("Clean(spaces) error = %v, want ErrEmpty", err)
	}
}

func TestCleanText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty is fine", "", "", false},
		{"only spaces become empty", " \t\n ", "", false},
		{"keeps line breaks and tabs inside", "Nasceu em\n\tMirathel.", "Nasceu em\n\tMirathel.", false},
		{"trims blank lines at both ends", "\n\nNasceu.\n\n", "Nasceu.", false},
		{"Windows line breaks", "linha 1\r\nlinha 2", "linha 1\nlinha 2", false},
		{"old Mac line breaks", "linha 1\rlinha 2", "linha 1\nlinha 2", false},
		{"exactly the limit", strings.Repeat("é", 20), strings.Repeat("é", 20), false},
		{"one character over", strings.Repeat("é", 21), "", true},
		{"NUL", "Mira\x00thel", "", true},
		{"escape", "Mira\x1bthel", "", true},
		{"right-to-left override", "Mira\u202ethel", "", true},
		{"invalid UTF-8", "Mira\xffthel", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CleanText(tt.input, 20)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CleanText(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("CleanText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
