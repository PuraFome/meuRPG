// Package names checks the text that users type the same way in every
// module: short, one-line names, such as a campaign's name or a display name
// (Clean), and longer texts that may span several lines, such as a
// character's backstory (CleanText).
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

// CleanText trims spaces (and blank lines) at both ends of s and checks that
// the rest is a text of at most maxLength characters (Unicode code points,
// like Clean). Unlike Clean, it accepts an empty text, line breaks and tabs.
// Windows (\r\n) and old Mac (\r) line breaks become \n. It returns the
// cleaned text, or an error whose message can be shown to the user.
//
// It refuses every other control character and the invisible characters
// that change text direction, for the same reasons as Clean.
func CleanText(s string, maxLength int) (string, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) {
		return "", errors.New("must be valid UTF-8")
	}
	if n := utf8.RuneCountInString(s); n > maxLength {
		return "", fmt.Errorf("must be at most %d characters, got %d", maxLength, n)
	}
	for _, r := range s {
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return "", fmt.Errorf("must not contain the character %U", r)
		}
	}
	return s, nil
}
