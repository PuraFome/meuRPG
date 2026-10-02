package combat

import (
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// TurnState is what the character has used in the current turn.
type TurnState struct {
	ActionUsed, BonusActionUsed, ReactionUsed bool
	// MovementUsedFt is the feet walked so far this turn.
	MovementUsedFt int
	// Dashed says the Dash action was taken: the speed counts twice.
	Dashed bool
}

// Reason codes for a disabled option. They are codes, never text: the web
// maps each to Portuguese copy ("Ação já usada", "Sem espaço de 2º círculo
// ou maior").
const (
	// ReasonActionUsed, ReasonBonusActionUsed and ReasonReactionUsed: the
	// economy the option needs is spent.
	ReasonActionUsed      = "ACTION_USED"
	ReasonBonusActionUsed = "BONUS_ACTION_USED"
	ReasonReactionUsed    = "REACTION_USED"
	// ReasonNoSlot: no free slot at the spell's level or above (MinLevel).
	ReasonNoSlot = "NO_SLOT"
	// ReasonNoUses: the feature's resource is spent (Recharge says when it
	// comes back).
	ReasonNoUses = "NO_USES"
	// ReasonReactionOnlyWhenHit: Shield, which is only cast when an attack
	// hits the caster, never on their own turn.
	ReasonReactionOnlyWhenHit = "REACTION_ONLY_WHEN_HIT"
	// ReasonReactionOnly: any other reaction spell, cast when its trigger
	// happens.
	ReasonReactionOnly = "REACTION_ONLY"
	// ReasonTooLong: the casting time is a minute or more, too long for a
	// fight.
	ReasonTooLong = "CASTING_TIME_TOO_LONG"
)

// Reason says why an option is disabled.
type Reason struct {
	Code string
	// MinLevel is set for ReasonNoSlot: the lowest slot level that would
	// do.
	MinLevel int
	// Recharge is set for ReasonNoUses: a rules.Recharge* constant.
	Recharge string
}

// Slot is the state of one part of the action economy.
type Slot struct {
	Used, Available bool
}

// Movement is the walking left in the turn.
type Movement struct {
	// SpeedFt is the speed for this turn: doubled after the Dash action.
	SpeedFt, UsedFt, LeftFt int
}

// Economy is the turn's action economy.
type Economy struct {
	Action, BonusAction, Reaction Slot
	Movement                      Movement
}

// Option is something the character may try. When Enabled is false, Reason
// says why.
type Option struct {
	Enabled bool
	Reason  *Reason
}

// AttackOption is a line of Derived.Attacks.
type AttackOption struct {
	Option
	Attack rules.Attack
}

// SlotChoice is a slot the spell can be cast with.
type SlotChoice struct {
	// Level is the slot level; Pact says it is a pact magic slot.
	Level int
	Pact  bool
	// Free is how many slots of this kind are free, so the app can warn
	// about the last one.
	Free int
}

// SpellOption is a spell the character can cast.
type SpellOption struct {
	Option
	Spell rules.SpellEntry
	// Economy is what casting costs: rules.EconomyAction, EconomyBonusAction
	// or EconomyReaction; "" for a casting time too long for a fight.
	Economy string
	// Slots are the slot levels it can be cast with: at least the spell's
	// level and with a free slot. Empty for a cantrip, and for a spell with
	// no slot (then the option is disabled with ReasonNoSlot).
	Slots []SlotChoice
}

// ActionOption is a standard action or a feature's action.
type ActionOption struct {
	Option
	Action rules.Action
	// UsesLeft is how many uses of the action's resource remain, when it
	// has one.
	UsesLeft int
}

// TurnOptions is everything "Sua vez" offers (MR-014).
type TurnOptions struct {
	Economy         Economy
	Attacks         []AttackOption
	Spells          []SpellOption
	StandardActions []ActionOption
	FeatureActions  []ActionOption
}

// shieldKey is the one reaction spell with its own reason.
const shieldKey = "spell:shield"

// Options works out what the character can do now, from their sheet, what
// they used this turn and what they spent since the last rest. It is
// computed on every read and never stored.
func Options(d rules.Derived, turn TurnState, u Usage) TurnOptions {
	speed := d.SpeedWalkFt
	if turn.Dashed {
		speed *= 2
	}
	out := TurnOptions{Economy: Economy{
		Action:      Slot{Used: turn.ActionUsed, Available: !turn.ActionUsed},
		BonusAction: Slot{Used: turn.BonusActionUsed, Available: !turn.BonusActionUsed},
		Reaction:    Slot{Used: turn.ReactionUsed, Available: !turn.ReactionUsed},
		Movement:    Movement{SpeedFt: speed, UsedFt: turn.MovementUsedFt, LeftFt: max(speed-turn.MovementUsedFt, 0)},
	}}

	// Attacks cost an action. The damaging cantrips are in Derived.Attacks
	// already, so they are left out of the spells below.
	cantripAttacks := map[string]bool{}
	for _, a := range d.Attacks {
		if a.Kind == "spell" {
			cantripAttacks[a.Key] = true
		}
		out.Attacks = append(out.Attacks, AttackOption{Option: economyOption(rules.EconomyAction, turn), Attack: a})
	}

	for _, cs := range d.Spells {
		if !cs.Prepared || cantripAttacks[cs.Spell.Key] {
			continue
		}
		out.Spells = append(out.Spells, spellOption(d, turn, u, cs.Spell))
	}

	for _, a := range d.StandardActions {
		out.StandardActions = append(out.StandardActions, actionOption(d, turn, u, a))
	}
	for _, a := range d.Actions {
		out.FeatureActions = append(out.FeatureActions, actionOption(d, turn, u, a))
	}
	return out
}

// economyOption is enabled while the economy is free. Free and movement
// actions are always free.
func economyOption(economy string, turn TurnState) Option {
	switch {
	case economy == rules.EconomyAction && turn.ActionUsed:
		return Option{Reason: &Reason{Code: ReasonActionUsed}}
	case economy == rules.EconomyBonusAction && turn.BonusActionUsed:
		return Option{Reason: &Reason{Code: ReasonBonusActionUsed}}
	case economy == rules.EconomyReaction && turn.ReactionUsed:
		return Option{Reason: &Reason{Code: ReasonReactionUsed}}
	}
	return Option{Enabled: true}
}

// spellEconomy maps a casting time to an economy, "" when it is too long.
func spellEconomy(ct rules.CastingTime) string {
	switch ct.Unit {
	case rules.CastAction:
		return rules.EconomyAction
	case rules.CastBonusAction:
		return rules.EconomyBonusAction
	case rules.CastReaction:
		return rules.EconomyReaction
	}
	return ""
}

func spellOption(d rules.Derived, turn TurnState, u Usage, sp rules.SpellEntry) SpellOption {
	o := SpellOption{Spell: sp, Economy: spellEconomy(sp.CastingTime)}

	if sp.Level > 0 {
		for l := sp.Level; l <= 9; l++ {
			if free := SlotsFree(d, u, l); free > 0 {
				o.Slots = append(o.Slots, SlotChoice{Level: l, Free: free})
			}
		}
		if p := d.PactMagic; p != nil && p.SlotLevel >= sp.Level {
			if free := PactSlotsFree(d, u); free > 0 {
				o.Slots = append(o.Slots, SlotChoice{Level: p.SlotLevel, Pact: true, Free: free})
			}
		}
	}

	// The first reason that applies wins: what the spell is, then the
	// economy, then the slots.
	switch {
	case o.Economy == "":
		o.Reason = &Reason{Code: ReasonTooLong}
	case o.Economy == rules.EconomyReaction && sp.Key == shieldKey:
		o.Reason = &Reason{Code: ReasonReactionOnlyWhenHit}
	case o.Economy == rules.EconomyReaction:
		o.Reason = &Reason{Code: ReasonReactionOnly}
	default:
		if eo := economyOption(o.Economy, turn); !eo.Enabled {
			o.Reason = eo.Reason
		} else if sp.Level > 0 && len(o.Slots) == 0 {
			o.Reason = &Reason{Code: ReasonNoSlot, MinLevel: sp.Level}
		}
	}
	o.Enabled = o.Reason == nil
	return o
}

func actionOption(d rules.Derived, turn TurnState, u Usage, a rules.Action) ActionOption {
	o := ActionOption{Action: a, Option: economyOption(a.Economy, turn)}
	if a.Resource == "" {
		return o
	}
	left, _ := ResourceLeft(d, u, a.Resource)
	o.UsesLeft = left
	if o.Enabled && left == 0 {
		o.Enabled = false
		o.Reason = &Reason{Code: ReasonNoUses, Recharge: rechargeOf(d, a.Resource)}
	}
	return o
}

func rechargeOf(d rules.Derived, key string) string {
	for _, r := range d.Resources {
		if r.Key == key {
			return r.Recharge
		}
	}
	return ""
}
