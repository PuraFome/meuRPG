package gen

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
)

// TestRealGemini makes one small picture with the real model. It runs only
// with MEURPG_TEST_GEMINI_API_KEY set (and optionally MEURPG_TEST_GEMINI_MODEL);
// the CI never sets it. It costs about one image (docs/operations.md). It is also
// where to confirm the answer's layout, which parseAnswer reads loosely.
func TestRealGemini(t *testing.T) {
	key := os.Getenv("MEURPG_TEST_GEMINI_API_KEY")
	if key == "" {
		t.Skip("MEURPG_TEST_GEMINI_API_KEY is not set")
	}
	g := &Gemini{Key: config.Secret(key), ModelName: os.Getenv("MEURPG_TEST_GEMINI_MODEL"), AttemptTimeout: 3 * time.Minute}
	img, err := g.Generate(t.Context(), Request{Prompt: "A small wooden tavern door at night, simple flat illustration", AspectRatio: "1:1"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(img.Data)); err != nil {
		t.Errorf("the picture is not a PNG or JPEG (%s): %v", img.MimeType, err)
	}
}
