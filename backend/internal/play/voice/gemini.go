package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuraFome/meuRPG/backend/internal/platform/config"
)

// GeminiBase is the Gemini API's endpoint of the models; the model's name and ":generateContent" follow.
// generateContent keeps no interaction on our account (docs/privacy.md). The request and answer
// follow https://ai.google.dev/gemini-api/docs/speech-generation: the audio comes as base64 16-bit
// PCM, mono, 24 kHz, in the first part of the first candidate.
const GeminiBase = "https://generativelanguage.googleapis.com/v1beta/models/"

// maxResponseBytes bounds one answer: a 2,000-character line is under three minutes of speech, about
// 8 MiB of PCM, 11 MiB in base64.
const maxResponseBytes = 16 << 20

// defaultRate is the sample rate the model answers with when its mime type names none.
const defaultRate = 24000

// Gemini is the Generator that calls the Gemini API with a key.
type Gemini struct {
	// Key is the API key, sent only in the x-goog-api-key header. It is a config.Secret: it prints as
	// [REDACTED], and nothing here formats it.
	Key config.Secret
	// ModelName is the text-to-speech model (GEMINI_TTS_MODEL). It has no default: the model's
	// name changes, so the operator sets the one in Google's current documentation.
	ModelName string
	// URL is the endpoint up to the model's name; empty means GeminiBase. Tests point it at httptest.
	URL string
	// Client does the calls; nil means a client of its own. Redirects are always refused, so the key
	// header never follows one.
	Client *http.Client
	// Timeout bounds one call; zero means two minutes.
	Timeout time.Duration
	// Logger gets one line per failed call: the status or the kind, never the text.
	Logger *slog.Logger
}

var _ Generator = (*Gemini)(nil)

// Model implements Generator.
func (g *Gemini) Model() string { return g.ModelName }

type speakRequest struct {
	Contents         []content        `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	ResponseModalities []string     `json:"responseModalities"`
	SpeechConfig       speechConfig `json:"speechConfig"`
}

type speechConfig struct {
	VoiceConfig struct {
		PrebuiltVoiceConfig struct {
			VoiceName string `json:"voiceName"`
		} `json:"prebuiltVoiceConfig"`
	} `json:"voiceConfig"`
}

// Speak implements Generator.
func (g *Gemini) Speak(ctx context.Context, text, voice string) (Audio, error) {
	if g.ModelName == "" || strings.TrimSpace(text) == "" {
		return Audio{}, ErrNoAudio
	}
	var req speakRequest
	req.Contents = []content{{Parts: []part{{Text: text}}}}
	req.GenerationConfig.ResponseModalities = []string{"AUDIO"}
	req.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName = voice
	body, err := json.Marshal(req)
	if err != nil {
		return Audio{}, fmt.Errorf("encode the request: %w", err)
	}
	base := g.URL
	if base == "" {
		base = GeminiBase
	}
	timeout := g.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+url.PathEscape(g.ModelName)+":generateContent", bytes.NewReader(body))
	if err != nil {
		return Audio{}, fmt.Errorf("build the request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", string(g.Key))
	res, err := g.client().Do(httpReq)
	if err != nil {
		g.log(ctx, "the call failed")
		return Audio{}, fmt.Errorf("%w", ErrUnavailable)
	}
	defer func() { _ = res.Body.Close() }()
	return g.parse(ctx, res.StatusCode, io.LimitReader(res.Body, maxResponseBytes))
}

func (g *Gemini) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (g *Gemini) log(ctx context.Context, what string, attrs ...any) {
	if g.Logger != nil {
		g.Logger.WarnContext(ctx, "voice: "+what, attrs...)
	}
}

type speakAnswer struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				InlineData *struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

var rateOf = regexp.MustCompile(`rate=(\d{4,6})`)

// parse reads an answer: the audio of the first candidate, or the reason there is none.
func (g *Gemini) parse(ctx context.Context, status int, r io.Reader) (Audio, error) {
	if status != http.StatusOK {
		g.log(ctx, "the service answered an error", "status", status)
		return Audio{}, ErrUnavailable
	}
	var ans speakAnswer
	if err := json.NewDecoder(r).Decode(&ans); err != nil {
		g.log(ctx, "the answer cannot be read")
		return Audio{}, ErrUnavailable
	}
	if ans.PromptFeedback.BlockReason != "" {
		g.log(ctx, "the text was refused", "reason", ans.PromptFeedback.BlockReason)
		return Audio{}, ErrRefused
	}
	for _, c := range ans.Candidates {
		for _, p := range c.Content.Parts {
			if p.InlineData == nil || p.InlineData.Data == "" {
				continue
			}
			pcm, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil || len(pcm) < 2 {
				return Audio{}, ErrNoAudio
			}
			return Audio{Data: wav(pcm[:len(pcm)&^1], sampleRate(p.InlineData.MimeType)), ContentType: ContentType}, nil
		}
	}
	g.log(ctx, "the answer holds no audio")
	return Audio{}, ErrNoAudio
}

// sampleRate is the rate a mime type such as "audio/L16;codec=pcm;rate=24000" names, or the default.
func sampleRate(mime string) int {
	if m := rateOf.FindStringSubmatch(mime); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return defaultRate
}
