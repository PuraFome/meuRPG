package characters

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/characters/charactersdb"
)

// A stat block that is missing or does not read is told in the log, with the creature's id and the reason, and with
// none of its content; the creature still reads as the entry is (nil: no block of its own).
func TestMR578_AStatBlockThatDoesNotReadIsLoggedWithoutItsContent(t *testing.T) {
	t.Parallel()
	const hiddenName = "Lobo-Secreto-Do-Mestre"
	for _, tt := range []struct {
		name   string
		stored []byte
		reason string
	}{
		{"none stored", nil, "missing"},
		{"not json", []byte(hiddenName + " {"), "unreadable"},
		{"another creature's block", []byte(`{"key":"creature:outro@mesa","name_pt":"` + hiddenName + `"}`), "unreadable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			s := &Service{logger: slog.New(slog.NewTextHandler(&buf, nil))}
			v := creatureView{CharacterCreature: charactersdb.CharacterCreature{ID: "c0ffee00-0000-4000-8000-000000000001", MonsterKey: "creature:lobo@mesa", StatBlock: tt.stored}}
			if got := s.resolvedCreatureOf(t.Context(), nil, v); got != nil {
				t.Fatalf("resolvedCreatureOf() = %v, want nil", got)
			}
			out := buf.String()
			if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "reason="+tt.reason) || !strings.Contains(out, v.ID) {
				t.Errorf("log = %q, want a warning with the creature's id and reason %s", out, tt.reason)
			}
			if strings.Contains(out, hiddenName) || strings.Contains(out, "lobo@mesa") {
				t.Errorf("log carries the content of the block: %q", out)
			}
		})
	}
}
