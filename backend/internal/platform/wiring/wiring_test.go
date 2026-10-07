package wiring_test

import (
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/platform/wiring"
)

func TestCheck(t *testing.T) {
	t.Parallel()
	if err := wiring.Check("play", wiring.Dep{Setter: "SetFog"}, wiring.Dep{Setter: "SetTraps"}); err != nil {
		t.Errorf("all wired: error = %v, want nil", err)
	}
	err := wiring.Check("play",
		wiring.Dep{Setter: "SetFog", Missing: true},
		wiring.Dep{Setter: "SetTraps"},
		wiring.Dep{Setter: "SetTerrain", Missing: true})
	if err == nil {
		t.Fatal("two setters missing: error = nil")
	}
	for _, want := range []string{"play", "SetFog", "SetTerrain"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "SetTraps") {
		t.Errorf("error %q names a setter that was called", err)
	}
}
