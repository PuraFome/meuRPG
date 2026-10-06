package play

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every combat transaction reads the table's rules (RN-24: the critical, the
// hidden death saves), so it is built only by writeOnce or openTx. A combatTx
// written by hand elsewhere would follow the SRD's defaults silently, as a
// puzzle's trap once did. This test reads the package's sources and fails on one.
func TestEveryCombatTxIsBuiltWithTheRules(t *testing.T) {
	t.Parallel()
	built := regexp.MustCompile(`combatTx\{`)
	allowed := map[string]bool{
		"combat_write.go":      true, // writeOnce
		"combat_tablerules.go": true, // openTx
		"vitals.go":            true, // a placeholder for applyBody, not a transaction
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || allowed[f] {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !built.MatchString(line) || strings.Contains(line, "openTx(ctx, combatTx{") {
				continue
			}
			if strings.Contains(line, "type combatTx struct") || strings.Contains(line, "func (c *combatTx)") {
				continue
			}
			t.Errorf("%s:%d builds a combatTx without the table's rules; use openTx: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}
