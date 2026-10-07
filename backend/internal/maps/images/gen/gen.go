// Package gen asks an image model for pictures (MR-039, RN-28, ADR-0019).
//
// Generator is the small interface the maps module uses; Gemini is the real
// one (the Gemini API over plain net/http, no SDK), Fake the deterministic one
// for local runs, CI and tests. The package knows nothing about campaigns,
// people or the database: a Request holds only what may go to the model (the
// master's text, a style, a ratio and gallery images), so there is no field a
// name or an e-mail could travel in. What the module adds to a request is
// listed in docs/architecture.md.
package gen

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DefaultModel is the image model used when GEMINI_IMAGE_MODEL is not set.
const DefaultModel = "gemini-3.1-flash-image"

// The most reference images one request may carry: the model takes up to 10
// objects and 4 characters (Google's image generation guide, 05/10/2026).
const (
	MaxObjectReferences    = 10
	MaxCharacterReferences = 4
)

// Ratios are the aspect ratios the model returns (Google's image generation
// guide, 05/10/2026), and the only ones a Request may ask for.
var Ratios = []string{"1:1", "3:2", "2:3", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"}

// ValidRatio reports whether ratio is one of Ratios.
func ValidRatio(ratio string) bool {
	for _, r := range Ratios {
		if r == ratio {
			return true
		}
	}
	return false
}

// Image is a picture's bytes with its type.
type Image struct {
	MimeType string
	Data     []byte
}

// Reference is a gallery image sent as a reference. Character says it shows a
// character (a portrait), which the model counts apart from objects and places.
type Reference struct {
	Image
	Character bool
}

// Edit asks to change a picture that was generated before. The model keeps no
// state: the previous picture, the text that made it and the adjustments
// already made go again with the new instruction.
type Edit struct {
	Previous    Image
	Original    string   // the first text, as the master wrote it
	Adjustments []string // the instructions applied since, oldest first
	Instruction string   // the new one
}

// The layouts of a request made from a map (MR-039): how the model reads the
// Drawing.
const (
	// LayoutScene: a painting of the place the drawing shows, from what the
	// players' characters see.
	LayoutScene = "scene"
	// LayoutIsometric: the same place seen from an isometric angle.
	LayoutIsometric = "isometric"
	// LayoutTexture: the drawing painted over as a top-down battle map.
	LayoutTexture = "texture"
)

// ValidLayout reports whether layout is "" or one of the layouts.
func ValidLayout(layout string) bool {
	switch layout {
	case "", LayoutScene, LayoutIsometric, LayoutTexture:
		return true
	}
	return false
}

// MaxRooms is the most rooms a textured map's request lists.
const MaxRooms = 60

// Request is everything that goes to the model.
type Request struct {
	// Prompt is the master's text, as written. For an edit it is empty.
	Prompt string
	// Style is a phrase for the style ("oil painting"), "" for none.
	Style string
	// AspectRatio is one of Ratios.
	AspectRatio string
	// References are gallery images, at most MaxObjectReferences objects and
	// MaxCharacterReferences characters.
	References []Reference
	// Edit, when set, makes this an edit.
	Edit *Edit
	// Drawing is the server's own plan of a map (floors and walls, no names): the
	// first image after an edit's previous one, ahead of the References. Layout
	// says how to read it. Both are set for a request made from a map, and neither
	// for any other.
	Drawing *Image
	Layout  string
	// Rooms is the rooms of a generated dungeon, one line each ("Sala 3: 7 x 5
	// squares"): sent only with LayoutTexture.
	Rooms []string
}

// Generator makes one picture from a Request.
type Generator interface {
	// Generate returns the picture, or an error. ErrNoImage, a *RefusedError
	// and ErrUnavailable are the typed ones; the module reads them with
	// errors.Is and errors.As and never shows another message to the master.
	// It retries what is worth retrying itself (see Gemini).
	Generate(ctx context.Context, req Request) (Image, error)
	// Model names the model, for the request's record.
	Model() string
}

var (
	// ErrNoImage is the service answering without a picture.
	ErrNoImage = errors.New("the image service did not return an image")
	// ErrUnavailable is the service being unreachable, timing out or failing
	// (after the one retry a 429 or a 5xx gets).
	ErrUnavailable = errors.New("the image service is unavailable")
	// ErrNotAuthorized is the service refusing our key (a 401 or 403: a blocked
	// or invalid key, a restricted API). It is the operator's problem, not the
	// master's text: the master hears "not available", the log has the reason.
	ErrNotAuthorized = errors.New("the image service did not accept the key")
)

// MaxBodyBytes is the most a call's body may be, base64 included. Bigger
// requests are refused before a slot is reserved (maps checks BodySize).
const MaxBodyBytes = 8 << 20

// BodySize estimates the JSON body this Request makes, in bytes: the text, and
// every image in base64 (4 bytes for each 3) with its small wrapper.
func (r Request) BodySize() int {
	n := len(r.Text()) + 512
	add := func(img Image) { n += (len(img.Data)+2)/3*4 + len(img.MimeType) + 48 }
	if r.Edit != nil {
		add(r.Edit.Previous)
	}
	if r.Drawing != nil {
		add(*r.Drawing)
	}
	for _, ref := range r.References {
		add(ref.Image)
	}
	return n
}

// RefusedError is the service refusing a prompt, or stopping for safety. Reason
// is a short code for the log ("safety", "blocked", "prohibited"); it carries
// nothing the master wrote.
type RefusedError struct{ Reason string }

func (e *RefusedError) Error() string { return "the image service refused the request: " + e.Reason }

// Validate checks a Request's shape, before anything is sent.
func (r Request) Validate() error {
	if !ValidRatio(r.AspectRatio) {
		return fmt.Errorf("gen: aspect ratio %q is not one the model returns", r.AspectRatio)
	}
	if r.Edit == nil && strings.TrimSpace(r.Prompt) == "" {
		return errors.New("gen: the prompt is empty")
	}
	if r.Edit != nil && (strings.TrimSpace(r.Edit.Instruction) == "" || len(r.Edit.Previous.Data) == 0) {
		return errors.New("gen: an edit needs an instruction and the previous image")
	}
	if !ValidLayout(r.Layout) || (r.Layout == "") != (r.Drawing == nil) || (r.Layout != "" && r.Edit != nil) {
		return errors.New("gen: a drawing goes with a layout, and a layout with a drawing, in a request that is not an edit")
	}
	if len(r.Rooms) > 0 && r.Layout != LayoutTexture || len(r.Rooms) > MaxRooms {
		return errors.New("gen: rooms go only with the textured map, and at most MaxRooms of them")
	}
	objects, characters := 0, 0
	for _, ref := range r.References {
		if ref.Character {
			characters++
		} else {
			objects++
		}
	}
	if objects > MaxObjectReferences || characters > MaxCharacterReferences {
		return fmt.Errorf("gen: at most %d objects and %d characters as references", MaxObjectReferences, MaxCharacterReferences)
	}
	return nil
}

// Text builds the text the model gets: the master's words as written, with
// the style and the framing around them. It is the same for every Generator,
// so the fake sees what Gemini would.
func (r Request) Text() string {
	var b strings.Builder
	if e := r.Edit; e != nil {
		b.WriteString("Edit the attached image (the first one). Keep everything that the new instruction does not change.\n")
		b.WriteString("The picture was made from this description: ")
		b.WriteString(e.Original)
		b.WriteString("\n")
		for _, a := range e.Adjustments {
			b.WriteString("Adjustment already applied: ")
			b.WriteString(a)
			b.WriteString("\n")
		}
		b.WriteString("New instruction: ")
		b.WriteString(e.Instruction)
	} else {
		switch r.Layout {
		case LayoutScene:
			b.WriteString("Paint a scene of the place that the first attached image shows. The first image is a plan seen from above: light squares are floor, dark hatched squares are walls and rock, and solid black is not visible and must not appear in the picture. Colored discs mark where creatures stand: draw a creature there, never the disc.\n")
		case LayoutIsometric:
			b.WriteString("Paint an isometric view of the place that the first attached image shows, seen from above at an angle, like a game board. The first image is a plan seen from above: light squares are floor, dark hatched squares are walls and rock, and solid black is not visible and must not appear in the picture. Colored discs mark where creatures stand: draw a creature there, never the disc.\n")
		case LayoutTexture:
			b.WriteString("Paint the plan in the first attached image as a textured top-down battle map, in the same proportions. Keep every wall, floor and passage exactly where the plan has them: light areas are floor, dark hatched areas are solid rock and walls. Do not draw a grid, text, numbers, doors or characters.\n")
			if len(r.Rooms) > 0 {
				b.WriteString("The rooms of the plan:\n")
				for _, room := range r.Rooms {
					b.WriteString("- ")
					b.WriteString(room)
					b.WriteString("\n")
				}
			}
		}
		b.WriteString(r.Prompt)
	}
	if r.Style != "" {
		b.WriteString("\nStyle: ")
		b.WriteString(r.Style)
	}
	if len(r.References) > 0 {
		b.WriteString("\nThe other attached images are references for the scene.")
	}
	return b.String()
}
