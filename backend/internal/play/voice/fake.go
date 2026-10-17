package voice

import (
	"context"
	"sync/atomic"
)

// Fake makes half a second of silence, with no network and no key. It is what the local stacks and
// CI use; the server refuses it on Cloud Run.
type Fake struct {
	calls atomic.Int32
}

var _ Generator = (*Fake)(nil)

// fakeRate and fakeSamples make 0.5 s at 8 kHz.
const (
	fakeRate    = 8000
	fakeSamples = fakeRate / 2
)

// Speak implements Generator.
func (f *Fake) Speak(context.Context, string, string) (Audio, error) {
	f.calls.Add(1)
	return Audio{Data: wav(make([]byte, fakeSamples*2), fakeRate), ContentType: ContentType}, nil
}

// Model implements Generator.
func (*Fake) Model() string { return "fake" }

// Calls is how many times Speak was called: a test asks it to prove a voice was made once.
func (f *Fake) Calls() int { return int(f.calls.Load()) }
