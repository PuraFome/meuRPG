package voice

import (
	"os"
	"testing"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
)

// TestRealGeminiVoice makes one short voice with the real model. It runs only with
// MEURPG_TEST_GEMINI_API_KEY and MEURPG_TEST_GEMINI_TTS_MODEL set; the CI never sets them. It costs
// about one short line, and it is where to confirm the answer's layout and the voices' names.
func TestRealGeminiVoice(t *testing.T) {
	key, model := os.Getenv("MEURPG_TEST_GEMINI_API_KEY"), os.Getenv("MEURPG_TEST_GEMINI_TTS_MODEL")
	if key == "" || model == "" {
		t.Skip("MEURPG_TEST_GEMINI_API_KEY and MEURPG_TEST_GEMINI_TTS_MODEL are not set")
	}
	g := &Gemini{Key: config.Secret(key), ModelName: model, Timeout: 2 * time.Minute}
	for _, name := range []string{Narrator, ForNPC("00000000-0000-0000-0000-000000000001")} {
		audio, err := g.Speak(t.Context(), "A porta range, devagar.", name)
		if err != nil {
			t.Fatalf("Speak(%s) error = %v", name, err)
		}
		if len(audio.Data) < 44 || string(audio.Data[:4]) != "RIFF" || audio.ContentType != "audio/wav" {
			t.Errorf("Speak(%s) = %q, %d bytes; want a WAV", name, audio.ContentType, len(audio.Data))
		}
	}
}
