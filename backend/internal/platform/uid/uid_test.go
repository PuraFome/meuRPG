package uid

import "testing"

func TestCanonical(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
		ok             bool
	}{
		{"canonical", "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174000", true},
		{"upper case", "123E4567-E89B-12D3-A456-426614174000", "123e4567-e89b-12d3-a456-426614174000", true},
		{"braced", "{123e4567-e89b-12d3-a456-426614174000}", "123e4567-e89b-12d3-a456-426614174000", true},
		{"empty", "", "", false},
		{"not a uuid", "nope", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Canonical(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Errorf("Canonical(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}
