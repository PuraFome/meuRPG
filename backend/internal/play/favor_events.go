package play

// The session events of the Inspiration the master grants (favor_*.go). Each payload holds ids only, never a name or free text.
const (
	eventFavorGranted = "favor_granted"
	eventFavorSpent   = "favor_spent"
	eventFavorPassed  = "favor_passed"
)

// favorGrantedEvent is the payload of favor_granted: the character the master gave the Inspiration to.
type favorGrantedEvent struct {
	CharacterID string `json:"character_id"`
}

// favorSpentEvent is the payload of favor_spent: the character that spent the Inspiration. No writer uses it: a spend rides its roll's event (Favor).
type favorSpentEvent struct {
	CharacterID string `json:"character_id"`
}

// favorPassedEvent is the payload of favor_passed: the character that gave the Inspiration away and the one that took it. The
// pass itself is not built yet.
type favorPassedEvent struct {
	FromCharacterID string `json:"from_character_id"`
	ToCharacterID   string `json:"to_character_id"`
}
