package play

// The session events of the Inspiration the master grants (favor_*.go). Each payload holds ids only, never a name or free text.
const (
	eventFavorGranted = "favor_granted"
	eventFavorSpent   = "favor_spent"
	eventFavorPassed  = "favor_passed"

	eventFavorPassAsked      = "favor_pass_asked"
	eventFavorAskRefused     = "favor_pass_refused"
	eventFavorPassCanceled   = "favor_pass_canceled"
	eventFavorRefundPrompted = "favor_refund_prompted"
	eventFavorRefunded       = "favor_refunded"
	eventFavorRefundKept     = "favor_refund_kept"
)

// favorGrantedEvent is the payload of favor_granted: the character the master gave the Inspiration to.
type favorGrantedEvent struct {
	CharacterID string `json:"character_id"`
}

// favorSpentEvent is the payload of favor_spent: the character that spent the Inspiration. No writer uses it: a spend rides its roll's event (Favor).
type favorSpentEvent struct {
	CharacterID string `json:"character_id"`
}

// favorPassedEvent is the payload of favor_passed: the master accepted the pass, so the character that gave the Inspiration away
// lost it and the one that took it has it now.
type favorPassedEvent struct {
	PassID          string `json:"pass_id"`
	FromCharacterID string `json:"from_character_id"`
	ToCharacterID   string `json:"to_character_id"`
}

// favorPassEvent is the payload of favor_pass_asked, favor_pass_refused and favor_pass_canceled: the pass and its two characters.
type favorPassEvent struct {
	PassID          string `json:"pass_id"`
	FromCharacterID string `json:"from_character_id"`
	ToCharacterID   string `json:"to_character_id"`
}

// favorRefundEvent is the payload of favor_refund_prompted, favor_refunded and favor_refund_kept: the prompt and the character that
// spent the Inspiration.
type favorRefundEvent struct {
	PromptID    string `json:"prompt_id"`
	CharacterID string `json:"character_id"`
}
