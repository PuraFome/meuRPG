package rules

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// TestEffectsCiteDocsThatExist checks that every docs/... page an effects
// file cites (a "source" string pointing at where the criteria are written)
// exists in the repository, so a moved or deleted page cannot leave a dead
// citation behind.
func TestEffectsCiteDocsThatExist(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(srd51.Files, "effects/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no effects files found: %v", err)
	}
	cite := regexp.MustCompile(`docs/[A-Za-z0-9_./-]+\.md`)
	cited := 0
	for _, f := range files {
		b, err := fs.ReadFile(srd51.Files, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, doc := range cite.FindAllString(string(b), -1) {
			cited++
			if _, err := os.Stat(filepath.Join("..", "..", "..", filepath.FromSlash(doc))); err != nil {
				t.Errorf("%s cites %s, which does not exist: %v", f, doc, err)
			}
		}
	}
	if cited == 0 {
		t.Error("no effects file cites a docs page: the check looks at nothing")
	}
}
