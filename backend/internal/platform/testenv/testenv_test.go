package testenv

import (
	"testing"
)

// stopped is what the fake panics with, to stop DatabaseURL where the real
// testing.T would stop the test (Skip and Fatal both end the goroutine).
type stopped struct{ how string }

type fakeT struct {
	testing.TB
}

func (fakeT) Helper()               {}
func (fakeT) Skip(...any)           { panic(stopped{"skip"}) }
func (fakeT) Fatalf(string, ...any) { panic(stopped{"fatal"}) }

// outcome runs DatabaseURL and reports how it ended: "skip", "fatal" or
// "url:<value>".
func outcome(t *testing.T) string {
	t.Helper()
	var how string
	func() {
		defer func() {
			if r := recover(); r != nil {
				s, ok := r.(stopped)
				if !ok {
					panic(r)
				}
				how = s.how
			}
		}()
		how = "url:" + DatabaseURL(fakeT{t})
	}()
	return how
}

func TestDatabaseURL(t *testing.T) {
	tests := []struct {
		name, url, require, want string
	}{
		{"url set", "postgresql://x", "", "url:postgresql://x"},
		{"url set and required", "postgresql://x", "1", "url:postgresql://x"},
		{"missing skips", "", "", "skip"},
		{"missing with require 0 skips", "", "0", "skip"},
		{"missing with require 1 fails", "", "1", "fatal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(DatabaseEnv, tc.url)
			t.Setenv(RequireEnv, tc.require)
			if got := outcome(t); got != tc.want {
				t.Errorf("DatabaseURL() ended as %q, want %q", got, tc.want)
			}
		})
	}
}
