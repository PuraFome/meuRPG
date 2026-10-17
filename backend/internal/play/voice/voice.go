// Package voice makes the spoken audio of a line of the scene's conversation
// (MR-051, "Ouvir"). It only talks to the text-to-speech model: it knows no
// campaign, person or database, and the text it gets is the only thing that
// leaves the server (docs/privacy.md). Gemini calls the real model, Fake makes
// a short silence for local runs and CI.
package voice

import (
	"context"
	"errors"
	"hash/fnv"
)

// Generator makes the audio of a text with one of the voices of this package.
type Generator interface {
	// Speak returns the text read aloud by the named voice, as a WAV file.
	Speak(ctx context.Context, text, voice string) (Audio, error)
	// Model names the model, for the log.
	Model() string
}

// Audio is a spoken text.
type Audio struct {
	// Data is a complete WAV file (16-bit PCM, mono).
	Data []byte
	// ContentType is "audio/wav".
	ContentType string
}

// ContentType is the type of every Audio this package makes.
const ContentType = "audio/wav"

var (
	// ErrUnavailable is any failure of the service that is not the text's fault: the network, a
	// status that is not a success, an answer that cannot be read.
	ErrUnavailable = errors.New("the voice service is unavailable")
	// ErrNoAudio is an answer that holds no audio.
	ErrNoAudio = errors.New("the voice service did not return audio")
	// ErrRefused is a text the service refused to read.
	ErrRefused = errors.New("the voice service refused the text")
)

// Narrator is the one voice of the narrator.
const Narrator = "Charon"

// npcVoices is the short list the NPCs' voices are picked from. The model's voices speak the
// language of the text they are given, so these were picked for how different they sound from each
// other; change the list here and nowhere else. Adding to its end keeps every NPC's voice, taking
// from it or reordering it changes them.
var npcVoices = []string{"Puck", "Kore", "Fenrir", "Leda", "Orus", "Aoede", "Algieba", "Gacrux"}

// ForNPC is the voice of an NPC: always the same for the same character ID.
func ForNPC(characterID string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(characterID)) // a hash.Hash never fails
	return npcVoices[int(h.Sum32())%len(npcVoices)]
}
