// Package names checks the short, one-line names that users type, such as a
// campaign's name or a display name, the same way in every module.
package names

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrEmpty means the name is empty, or only spaces.
var ErrEmpty = errors.New("must not be empty")

// Clean trims spaces at both ends of s and checks that the rest is a
// one-line name of 1 to maxLength characters (Unicode code points, the same
// unit as char_length in SQL, so the database CHECKs agree). It returns the
// trimmed name, or an error whose message can be shown to the user.
//
// It refuses control characters (line breaks, tabs, NUL...) and the
// invisible Unicode characters that change text direction, which could make
// a name look like another one on screen. Accents, emoji and any script are
// fine.
func Clean(s string, maxLength int) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrEmpty
	}
	if !utf8.ValidString(s) {
		return "", errors.New("must be valid UTF-8")
	}
	if n := utf8.RuneCountInString(s); n > maxLength {
		return "", fmt.Errorf("must be at most %d characters, got %d", maxLength, n)
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return "", fmt.Errorf("must not contain the character %U", r)
		}
	}
	return s, nil
}
