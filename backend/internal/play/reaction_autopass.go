package play

import "strings"

// The digits of a UUID that stay as they are when the key of an automatic pass is made: its version and variant ones.
const (
	uuidVersionAt = 14
	uuidVariantAt = 19
)

// autoPassKeyOf is the idempotency key of the automatic pass of a window: the window's id with every hex digit flipped
// (d -> 15-d), but the version and variant ones. It is the same derivation as autoPassKey in the web's reaction-autopass.ts.
func autoPassKeyOf(windowID string) string {
	id := strings.ToLower(windowID)
	b := []byte(id)
	for i, ch := range b {
		if i == uuidVersionAt || i == uuidVariantAt {
			continue
		}
		switch {
		case ch >= '0' && ch <= '9':
			b[i] = "0123456789abcdef"[15-int(ch-'0')]
		case ch >= 'a' && ch <= 'f':
			b[i] = "0123456789abcdef"[15-(int(ch-'a')+10)]
		}
	}
	return string(b)
}

// isAutoPassKey says the key is the automatic pass of the window (case apart): the master's screen passed it by itself.
func isAutoPassKey(windowID, key string) bool {
	return key != "" && strings.EqualFold(key, autoPassKeyOf(windowID))
}
