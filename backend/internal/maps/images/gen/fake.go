package gen

import (
	"bytes"
	"context"
	"errors"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FakeModel is the model name the Fake reports.
const FakeModel = "fake-image"

// The markers that make the Fake misbehave, so local runs and tests can see
// every outcome without a model: put one anywhere in the text.
const (
	MarkerRefuse = "[recusa]" // a refusal (RefusedError)
	MarkerEmpty  = "[vazio]"  // an answer without an image (ErrNoImage)
	MarkerError  = "[erro]"   // the service failing (ErrUnavailable)
	MarkerSlow   = "[lento]"  // a slow answer: waits SlowDelay (or the context)
)

// Fake is a Generator that needs no network: for the same Request it returns
// the same valid PNG, of the requested ratio. It records what it was asked,
// so a test can read what would have gone to Google.
type Fake struct {
	// SlowDelay is how long MarkerSlow waits. Zero means five seconds.
	SlowDelay time.Duration
	// Hook, when set, runs first in every call (after recording it); a test
	// blocks in it to hold a request "in flight". A returned error is the call's.
	Hook func(ctx context.Context, req Request) error

	mu    sync.Mutex
	calls []Call
}

// Call is one request the Fake received.
type Call struct {
	Request Request
	// Body is the JSON the Gemini generator would have sent for it.
	Body []byte
}

var _ Generator = (*Fake)(nil)

// Model implements Generator.
func (*Fake) Model() string { return FakeModel }

// Calls returns the requests received so far, oldest first.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

// Generate implements Generator.
func (f *Fake) Generate(ctx context.Context, req Request) (Image, error) {
	var body bytes.Buffer
	if err := writeBody(&body, FakeModel, req); err != nil {
		return Image{}, err
	}
	f.mu.Lock()
	f.calls = append(f.calls, Call{Request: req, Body: body.Bytes()})
	f.mu.Unlock()
	if f.Hook != nil {
		if err := f.Hook(ctx, req); err != nil {
			return Image{}, err
		}
		if ctx.Err() != nil { // a call cut off while the hook held it
			return Image{}, errors.Join(ErrUnavailable, ctx.Err())
		}
	}
	text := req.Text()
	switch {
	case strings.Contains(text, MarkerRefuse):
		return Image{}, &RefusedError{Reason: "safety"}
	case strings.Contains(text, MarkerEmpty):
		return Image{}, ErrNoImage
	case strings.Contains(text, MarkerError):
		return Image{}, ErrUnavailable
	case strings.Contains(text, MarkerSlow):
		d := f.SlowDelay
		if d == 0 {
			d = 5 * time.Second
		}
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return Image{}, errors.Join(ErrUnavailable, ctx.Err())
		}
	}
	return fakePNG(req.AspectRatio, text)
}

// fakePNG draws a flat color from the text's hash with a diagonal band, long
// side 256 px, in the ratio asked: always the same bytes for the same text.
func fakePNG(ratio, text string) (Image, error) {
	w, h := 256, 256
	if a, b, ok := strings.Cut(ratio, ":"); ok {
		x, _ := strconv.Atoi(a)
		y, _ := strconv.Atoi(b)
		switch {
		case x <= 0 || y <= 0:
		case x >= y:
			h = 256 * y / x
		default:
			w = 256 * x / y
		}
	}
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(text))
	v := sum.Sum32()
	base := color.RGBA{R: uint8(v & 0xff), G: uint8(v >> 8 & 0xff), B: uint8(v >> 16 & 0xff), A: 255}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := base
			if (x+y)/16%2 == 0 {
				c = color.RGBA{R: base.R / 2, G: base.G / 2, B: base.B / 2, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return Image{}, err
	}
	return Image{MimeType: "image/png", Data: buf.Bytes()}, nil
}
