package voice

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"
)

func TestForNPCIsStableAndSpreads(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for range 200 {
		id := uuid.New().String()
		v := ForNPC(id)
		if v != ForNPC(id) {
			t.Fatalf("ForNPC(%s) changed between two calls", id)
		}
		if v == Narrator {
			t.Errorf("an NPC got the narrator's voice %q", v)
		}
		seen[v] = true
	}
	if len(seen) != len(npcVoices) {
		t.Errorf("200 NPCs used %d of the %d voices of the list", len(seen), len(npcVoices))
	}
}

func TestFakeMakesAShortSilentWAV(t *testing.T) {
	t.Parallel()
	f := &Fake{}
	a, err := f.Speak(t.Context(), "olá", Narrator)
	if err != nil || a.ContentType != ContentType {
		t.Fatalf("Speak() = %v, %v", a.ContentType, err)
	}
	if string(a.Data[:4]) != "RIFF" || string(a.Data[8:12]) != "WAVE" || f.Calls() != 1 {
		t.Errorf("not a WAV file, or calls = %d", f.Calls())
	}
	if n := binary.LittleEndian.Uint32(a.Data[40:]); int(n) != len(a.Data)-44 {
		t.Errorf("data chunk says %d bytes, the file holds %d", n, len(a.Data)-44)
	}
	for _, b := range a.Data[44:] {
		if b != 0 {
			t.Fatal("the fake audio is not silent")
		}
	}
}

// answerOf is what the model answers: one part with the PCM in base64.
func answerOf(mime string, pcm []byte) string {
	return `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"` + mime + `","data":"` + base64.StdEncoding.EncodeToString(pcm) + `"}}]}}]}`
}

func TestGeminiAsksForAudioWithTheVoiceAndWrapsThePCM(t *testing.T) {
	t.Parallel()
	var gotPath, gotKey, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("x-goog-api-key")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(answerOf("audio/L16;codec=pcm;rate=16000", []byte{1, 0, 2, 0})))
	}))
	t.Cleanup(srv.Close)
	g := &Gemini{Key: "secret-key", ModelName: "a-model", URL: srv.URL + "/models/"}
	a, err := g.Speak(t.Context(), "Bem-vindos", "Puck")
	if err != nil {
		t.Fatalf("Speak() error = %v", err)
	}
	if gotPath != "/models/a-model:generateContent" || gotKey != "secret-key" {
		t.Errorf("path %q, key %q", gotPath, gotKey)
	}
	for _, want := range []string{`"text":"Bem-vindos"`, `"responseModalities":["AUDIO"]`, `"voiceName":"Puck"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("the body %s lacks %s", gotBody, want)
		}
	}
	if rate := binary.LittleEndian.Uint32(a.Data[24:]); rate != 16000 || len(a.Data) != 48 {
		t.Errorf("rate %d, %d bytes; want the mime type's 16000 and 44 + 4", rate, len(a.Data))
	}
}

func TestGeminiFailuresAreTypedAndNeverQuoteTheTextOrTheKey(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		status int
		body   string
		want   error
	}{
		"a server error":  {500, `{"error":{"message":"boom"}}`, ErrUnavailable},
		"not json":        {200, `<html>`, ErrUnavailable},
		"no audio":        {200, `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`, ErrNoAudio},
		"not base64":      {200, `{"candidates":[{"content":{"parts":[{"inlineData":{"data":"!!"}}]}}]}`, ErrNoAudio},
		"a refused text":  {200, `{"promptFeedback":{"blockReason":"SAFETY"}}`, ErrRefused},
		"no candidates":   {200, `{}`, ErrNoAudio},
		"an empty answer": {200, ``, ErrUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			var log strings.Builder
			g := &Gemini{Key: "secret-key", ModelName: "m", URL: srv.URL + "/", Logger: slog.New(slog.NewTextHandler(&log, nil))}
			_, err := g.Speak(t.Context(), "TEXTO-SECRETO", "Kore")
			if err != tc.want { //nolint:errorlint // the sentinel itself
				t.Errorf("error = %v, want %v", err, tc.want)
			}
			if strings.Contains(log.String(), "secret-key") || strings.Contains(log.String(), "TEXTO-SECRETO") {
				t.Errorf("the log holds the key or the text: %s", log.String())
			}
		})
	}
}

func TestGeminiWithoutAModelOrTextAsksNothing(t *testing.T) {
	t.Parallel()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	t.Cleanup(srv.Close)
	for _, g := range []*Gemini{{Key: "k", URL: srv.URL + "/"}, {Key: "k", ModelName: "m", URL: srv.URL + "/"}} {
		text := "x"
		if g.ModelName != "" {
			text = "  "
		}
		if _, err := g.Speak(context.Background(), text, "Kore"); err != ErrNoAudio { //nolint:errorlint // the sentinel itself
			t.Errorf("error = %v, want ErrNoAudio", err)
		}
	}
	if called {
		t.Error("a call without a model or text reached the service")
	}
}

func TestSampleRateIsBoundedAndFallsBackToTheDefault(t *testing.T) {
	t.Parallel()
	for mime, want := range map[string]uint32{
		"audio/L16;codec=pcm;rate=24000": 24000,
		"audio/L16;rate=16000":           16000,
		"audio/L16":                      defaultRate,
		"audio/L16;rate=0":               defaultRate,
		"audio/L16;rate=99999999999":     defaultRate,
		"audio/L16;rate=4294967295":      defaultRate,
		"audio/L16;rate=7999":            defaultRate,
		"audio/L16;rate=192001":          defaultRate,
		"audio/L16;rate=192000":          192000,
	} {
		if got := sampleRate(mime); got != want {
			t.Errorf("sampleRate(%q) = %d, want %d", mime, got, want)
		}
	}
}
