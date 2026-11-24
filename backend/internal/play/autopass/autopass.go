// Package autopass tells the idempotency key of a reaction window's automatic pass: the key the master's screen sends with
// "Deixar passar" when the window's wait ends (web/src/app/core/combat/reaction-autopass.ts, autoPassKey).
package autopass

import "strings"

// The digits of a UUID that stay as they are when the key is made: its version and variant ones.
const (
	versionAt = 14
	variantAt = 19
)

const hexDigits = "0123456789abcdef"

// Key is the idempotency key of the automatic pass of a window: the window's id with every hex digit flipped (d -> 15-d),
// but the version and variant ones. It is the derivation of autoPassKey on the web.
func Key(windowID string) string {
	b := []byte(strings.ToLower(windowID))
	for i, ch := range b {
		if i == versionAt || i == variantAt {
			continue
		}
		if d := strings.IndexByte(hexDigits, ch); d >= 0 {
			b[i] = hexDigits[15-d]
		}
	}
	return string(b)
}

// Is says the key is the automatic pass of the window, case apart.
func Is(windowID, key string) bool {
	return key != "" && strings.EqualFold(key, Key(windowID))
}
