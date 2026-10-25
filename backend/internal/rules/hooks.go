package rules

// The events an on_event effect hooks (the owner's alone).
const (
	EventShortRest       = "short_rest"
	EventLongRest        = "long_rest"
	EventDeathSaveFailed = "death_save_failed"
	EventDropToZero      = "drop_to_zero"
)

// eventKinds is the closed list of events.
var eventKinds = []string{EventShortRest, EventLongRest, EventDeathSaveFailed, EventDropToZero}

// EventHook is an on_event effect of the character, resolved for its level.
type EventHook struct {
	// Event is an Event* constant and Source the feature, trait or feat that has it.
	Event, Source string
	// Heal is the dice regained (the ability modifier in Bonus), nil for none.
	Heal *DiceFormula `json:",omitempty"`
	// HealPercentMax is the share of the maximum hit points regained, 0 for none.
	HealPercentMax int `json:",omitempty"`
	// TempHP is the temporary hit points given, 0 for none.
	TempHP int `json:",omitempty"`
	// Restore is the resource whose uses come back, RestoreCount how many.
	Restore      string `json:",omitempty"`
	RestoreCount int    `json:",omitempty"`
	// Spends is the resource the hook costs, SpendCount how many uses: the hook runs
	// only when that many are left, and spends them.
	Spends     string `json:",omitempty"`
	SpendCount int    `json:",omitempty"`
}

// HooksFor lists the hooks of an event, in the order of the character's features.
func (d *Derived) HooksFor(event string) []EventHook { return nil }

// PercentOfMax is pct percent of the maximum hit points, rounded down, at least 1.
func PercentOfMax(pct, maxHP int) int { return -1 }

// RestoreUses is how many uses a hook that restores count uses really gives back
// when used are spent: never more than are spent, so a hook that runs again changes
// nothing.
func RestoreUses(used, count int) int { return -1 }
