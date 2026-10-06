package puzzle

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrAnswers means the accepted answers of a riddle are not valid (none, too many, too
// long, empty once folded, or the same twice).
var ErrAnswers = errors.New("puzzle: the answers are not valid")

// The limits of what the master writes for a riddle and of what a player types.
const (
	// MaxAnswers is how many answers a riddle accepts, and MaxAnswerLen the length
	// of each, in characters.
	MaxAnswers   = 10
	MaxAnswerLen = 80
	// MaxTyped is the longest text a player may send as an answer or a deciphered
	// message, in characters. A riddle's answer is short; a cipher's message may be long.
	MaxTyped = 600
)

// accentFolder takes the accents off the letters of Portuguese (and the Latin
// ones near it). The text is lower-cased first, so only lower-case forms are here.
var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "ā", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e", "ē", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "ī", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o", "ō", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u", "ū", "u",
	"ç", "c", "ñ", "n", "ý", "y", "ÿ", "y", "ß", "ss", "æ", "ae", "œ", "oe",
)

// Fold is the form two texts are compared in (RN-27): lower case, no accents,
// and only letters and digits, each run of anything else (spaces, punctuation)
// counting as one space, and no space at the ends. "  A Sombra! " and "a sombra"
// fold to the same text. The comparison is the player's convenience, not a secret:
// folding hides nothing from the master.
func Fold(s string) string {
	s = accentFolder.Replace(strings.ToLower(s))
	var b strings.Builder
	space := true // a space is never written at the start
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		case !space:
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// ValidateAnswers checks the accepted answers of a riddle: 1 to MaxAnswers, each
// 1 to MaxAnswerLen characters (after trimming) with something in it once folded,
// none twice once folded.
func ValidateAnswers(answers []string) error {
	if len(answers) < 1 || len(answers) > MaxAnswers {
		return fmt.Errorf("%w: %d answers, want 1 to %d", ErrAnswers, len(answers), MaxAnswers)
	}
	seen := map[string]bool{}
	for i, a := range answers {
		if n := utf8.RuneCountInString(strings.TrimSpace(a)); n < 1 || n > MaxAnswerLen {
			return fmt.Errorf("%w: answer %d has %d characters, want 1 to %d", ErrAnswers, i+1, n, MaxAnswerLen)
		}
		f := Fold(a)
		if f == "" {
			return fmt.Errorf("%w: answer %d has no letter or digit", ErrAnswers, i+1)
		}
		if seen[f] {
			return fmt.Errorf("%w: answer %d repeats another", ErrAnswers, i+1)
		}
		seen[f] = true
	}
	return nil
}

// Matches says whether the typed text is one of the accepted answers, compared as
// Fold does. A typed text with nothing in it never matches.
func Matches(answers []string, typed string) bool {
	f := Fold(typed)
	if f == "" {
		return false
	}
	for _, a := range answers {
		if Fold(a) == f {
			return true
		}
	}
	return false
}
