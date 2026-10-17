package maps

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"uuid"

	"connectrpc.com/connect"

	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/names"
)

// The checks of what the master sent with an image request.

// cleanKey checks an idempotency key: 1 to 64 printable characters.
func cleanKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if n := utf8.RuneCountInString(key); n < 1 || n > 64 || !utf8.ValidString(key) || strings.IndexFunc(key, unicode.IsControl) >= 0 {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency_key must be 1 to 64 characters"))
	}
	return key, nil
}

// cleanPrompt checks the master's text: 1 to 500 characters, valid UTF-8, no
// control characters but line breaks and tabs. It stays as written; the error
// says the rule, never the text.
func cleanPrompt(text, field string) (string, error) {
	text = strings.TrimSpace(text)
	bad := !utf8.ValidString(text) || strings.IndexFunc(text, func(r rune) bool {
		return unicode.IsControl(r) && r != '\n' && r != '\t'
	}) >= 0
	if n := utf8.RuneCountInString(text); n < 1 || n > maxPromptCharacters || bad {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s must be 1 to %d characters of text", field, maxPromptCharacters))
	}
	return text, nil
}

// imageNameOf is the name the generated image gets: the master's (checked like a rename), or
// the default when there is none ("" when the caller has none and fills it in later).
func imageNameOf(given, fallback string) (string, error) {
	if strings.TrimSpace(given) == "" {
		return fallback, nil
	}
	name, err := names.Clean(given, maxNameLength)
	if err != nil {
		// The message says the rule, never the name, which is free text.
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name %w", err))
	}
	return name, nil
}

// imageNameFor is the name the request records for its gallery image ("" only for a request
// without one, which the picture then names "Imagem n").
func imageNameFor(n newRequest) string { return n.name }

// datedName is a default name that says what the picture is and the day it was made, and nothing
// the master wrote: the text of a request or the name of a hidden map can hold a secret, and the
// players read the name of an image they are shown (RN-10). "Arte da cena · 06/10".
func datedName(way string, now time.Time) string {
	return way + " · " + now.In(brazil).Format("02/01")
}

// suffixedName is a name with a suffix, cut so that it still fits the name limit.
func suffixedName(name, suffix string) string {
	room := maxNameLength - utf8.RuneCountInString(suffix)
	if utf8.RuneCountInString(name) > room {
		name = strings.TrimSpace(string([]rune(name)[:room]))
	}
	return name + suffix
}

func styleAndRatio(style mapsv1.ImageStyle, ratio mapsv1.ImageAspectRatio) (styleKey, ratioName string, err error) {
	if _, ok := stylePhrases[style]; !ok && style != mapsv1.ImageStyle_IMAGE_STYLE_UNSPECIFIED {
		return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("style is not one of the styles"))
	}
	ratioName = defaultSceneRatio
	if ratio != mapsv1.ImageAspectRatio_IMAGE_ASPECT_RATIO_UNSPECIFIED {
		var ok bool
		if ratioName, ok = ratioNames[ratio]; !ok {
			return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("aspect_ratio is not one of the ratios"))
		}
	}
	return styleKeys[style], ratioName, nil
}

func cleanImageIDs(ids []string, most int, field string) ([]string, error) {
	if len(ids) > most {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s has at most %d images", field, most))
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		u, err := uuid.Parse(id)
		if err != nil {
			return nil, errImageNotFound()
		}
		out = append(out, u.String())
	}
	return out, nil
}

func hasDuplicates(ids []string) bool {
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}
