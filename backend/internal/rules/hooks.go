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
func (d *Derived) HooksFor(event string) []EventHook {
	var out []EventHook
	for _, h := range d.Hooks {
		if h.Event == event {
			out = append(out, h)
		}
	}
	return out
}

// MaxHealPercent is the most a hook's heal_percent_max can be.
const MaxHealPercent = 100

// PercentOfMax is pct percent of the maximum hit points, rounded down, at least 1.
func PercentOfMax(pct, maxHP int) int {
	return max(maxHP*pct/MaxHealPercent, 1)
}

// RestoreUses is how many uses a hook that restores count uses really gives back
// when used are spent: never more than are spent, so a hook that runs again changes
// nothing.
func RestoreUses(used, count int) int {
	return max(min(used, count), 0)
}

// hooks fills Derived.Hooks from the on_event effects the character has, for its
// level and abilities. A hook that spends a resource the character does not have at
// this level is left out (it cannot be paid); one that restores such a resource
// keeps its other results.
func (x *deriver) hooks() {
	for _, a := range x.active {
		if a.effect.Type != "on_event" || !x.applies(a) {
			continue
		}
		if h, ok := x.hook(a); ok {
			x.d.Hooks = append(x.d.Hooks, h)
		}
	}
}

// hook is the EventHook of an on_event effect, false when nothing is left of it.
func (x *deriver) hook(a activeEffect) (EventHook, bool) {
	e := a.effect
	h := EventHook{Event: e.Event, Source: a.owner, HealPercentMax: e.HealPercentMax}
	if e.Resource != "" {
		if !x.hasResource(e.Resource) {
			return h, false
		}
		h.Spends, h.SpendCount = e.Resource, 1
		if e.cost != nil {
			n, ok := x.evalInt(e.cost, a)
			if !ok {
				return h, false
			}
			h.SpendCount = max(n, 1)
		}
	}
	x.hookResult(&h, a)
	return h, h.Heal != nil || h.HealPercentMax > 0 || h.TempHP > 0 || h.RestoreCount > 0
}

// hasResource says the character has the resource at its level.
func (x *deriver) hasResource(key string) bool { return slicesIndexResource(x.d.Resources, key) >= 0 }

// hookResult fills what a hook gives: the heal, the temporary hit points and the uses
// it restores (of a resource the character has).
func (x *deriver) hookResult(h *EventHook, a activeEffect) {
	e := a.effect
	if e.Heal != "" {
		if d, ok := parseHealDice(e.Heal); ok {
			d.Bonus += x.mods[Ability(e.Ability)]
			h.Heal = &d
		}
	}
	if e.tempHP != nil {
		if n, ok := x.evalInt(e.tempHP, a); ok {
			h.TempHP = max(n, 0)
		}
	}
	if e.Restore == "" || !x.hasResource(e.Restore) {
		return
	}
	h.Restore, h.RestoreCount = e.Restore, 1
	if e.value != nil {
		if n, ok := x.evalInt(e.value, a); ok {
			h.RestoreCount = max(n, 0)
		}
	}
}
