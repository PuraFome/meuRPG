// Package reaction is the pure part of the reaction window (PM-04): the numbers
// of each reaction as SRD 5.1 gives them, the rule that closes a window by itself,
// the order windows are answered in and the sentence the combat shows while one
// waits. It reads no database; package play keeps the windows and calls it.
package reaction

import (
	"slices"
	"strings"
)

// Ranges and limits of the reactions, in feet and creatures.
const (
	// RangeFt is the 60 ft of Counterspell, Hellish Rebuke, Feather Fall and
	// Cutting Words (SRD): 18 m on the table.
	RangeFt = 60
	// FeatherFallMaxTargets is how many falling creatures Feather Fall slows (SRD).
	FeatherFallMaxTargets = 5
	// CounterspellLevel is the level of the Counterspell spell, the lowest slot that
	// casts it.
	CounterspellLevel = 3
	// HellishRebukeBaseLevel is the level Hellish Rebuke's 2d10 is written for, 1st.
	HellishRebukeBaseLevel = 1
	// InfernalLegacyLevel is the level the tiefling's trait casts Hellish Rebuke at
	// (SRD, Infernal Legacy): 2nd.
	InfernalLegacyLevel = 2
	// ShieldACBonus is the armor class Shield adds until the start of the caster's
	// next turn (SRD, Shield).
	ShieldACBonus = 5
	// MinConcentrationDC is the least a concentration save can be (SRD, Duration).
	MinConcentrationDC = 10
	// DeflectDie is the die Deflect Missiles rolls (1d10, SRD, Monk 3).
	DeflectDie = 10
	// DeflectNormalRangeFt and DeflectLongRangeFt are the range of the missile a
	// monk throws back (SRD, Monk 3).
	DeflectNormalRangeFt = 20
	DeflectLongRangeFt   = 60
)

// Counterspell says what a Counterspell cast with a slot does to a spell (SRD,
// Counterspell): a spell whose level is no more than the slot's fails with no
// check (the slot is at least the 3rd; the spell's own text covers a 3rd-level
// spell or lower with the base slot, and "no effect if its level is less than or
// equal to the level of the slot used" for a higher one); a higher spell needs an
// ability check using the caster's spellcasting ability against DC 10 + the
// spell's level.
func Counterspell(slotLevel, spellLevel int) (auto bool, dc int) {
	if spellLevel <= slotLevel {
		return true, 0
	}
	return false, 10 + spellLevel
}

// UncannyDodge is the damage left after Uncanny Dodge: half, rounded down (the
// SRD says "halve", and the Player's Handbook rounds down).
func UncannyDodge(damage int) int {
	return max(damage, 0) / 2
}

// DeflectMissiles is the damage left after Deflect Missiles: the damage minus
// 1d10 + the Dexterity modifier + the monk level, never below 0 (SRD, Monk 3).
func DeflectMissiles(damage, die, dexMod, monkLevel int) (left, reduction int) {
	reduction = max(die+dexMod+monkLevel, 0)
	return max(damage-reduction, 0), reduction
}

// HellishRebukeDice is how many d10 Hellish Rebuke rolls cast with a slot of the
// level: 2d10, and 1d10 more for each slot level above the 1st (SRD).
func HellishRebukeDice(slotLevel int) int {
	return 2 + max(slotLevel-HellishRebukeBaseLevel, 0)
}

// CuttingWordsDie is the Bardic Inspiration die a Cutting Words subtracts: d6 from
// level 1, d8 from 5, d10 from 10 and d12 from 15 (SRD, Bard).
func CuttingWordsDie(bardLevel int) int {
	switch {
	case bardLevel >= 15: //nolint:mnd // the Bardic Inspiration table
		return 12
	case bardLevel >= 10: //nolint:mnd // the Bardic Inspiration table
		return 10
	case bardLevel >= 5: //nolint:mnd // the Bardic Inspiration table
		return 8
	}
	return 6
}

// ConcentrationDC is the Constitution save DC to keep a concentration after damage:
// 10 or half the damage, whichever is higher (SRD, Duration).
func ConcentrationDC(damage int) int {
	return max(MinConcentrationDC, max(damage, 0)/2)
}

// Kind is the reaction a window asks.
type Kind string

// The kinds of window.
const (
	Shield            Kind = "shield"
	UncannyDodgeKind  Kind = "uncanny_dodge"
	HellishRebukeKind Kind = "hellish_rebuke"
	CounterspellKind  Kind = "counterspell"
	CuttingWords      Kind = "cutting_words"
	DeflectKind       Kind = "deflect_missiles"
	FeatherFall       Kind = "feather_fall"
	Concentration     Kind = "concentration_save"
	MasterCheck       Kind = "master_check"
)

// Reason is why a window closed by itself.
type Reason string

// The reasons.
const (
	ReasonNone                 Reason = ""
	ReasonReactionSpent        Reason = "reaction_spent"
	ReasonReactorIncapacitated Reason = "reactor_incapacitated"
	ReasonTriggerGone          Reason = "trigger_gone"
)

// Facts are what decides whether an open window still stands.
type Facts struct {
	// ReactionUsed: the reactor already spent its reaction this round.
	ReactionUsed bool
	// Incapacitated: the reactor is incapacitated, unconscious, down or dead.
	Incapacitated bool
	// TriggerGone: what triggered the window is not there any more.
	TriggerGone bool
}

// Closure says whether a window must close by itself, and why. A window that
// does not use the reactor's reaction (the concentration save, the master's check)
// ignores ReactionUsed. Incapacitated wins over a spent reaction, and both over a
// gone trigger: the reason a reactor reads is about itself first.
func Closure(kind Kind, f Facts) (Reason, bool) {
	usesReaction := kind != Concentration && kind != MasterCheck
	switch {
	case f.Incapacitated && kind != MasterCheck && kind != Concentration:
		return ReasonReactorIncapacitated, true
	case f.ReactionUsed && usesReaction:
		return ReasonReactionSpent, true
	case f.TriggerGone:
		return ReasonTriggerGone, true
	}
	return ReasonNone, false
}

// Window is what the order needs of a window.
type Window struct {
	ID    string
	Group string
	Seq   int64
	Open  bool
}

// AnswerNow is the window to answer now in its group: the open one with the least
// seq. The master answers windows of different groups in any order, but within a
// group the reactors answer in the order they were opened (the initiative order).
func AnswerNow(all []Window, id string) bool {
	var self Window
	for _, w := range all {
		if w.ID == id {
			self = w
		}
	}
	if !self.Open {
		return false
	}
	for _, w := range all {
		if w.Open && w.Group == self.Group && w.Seq < self.Seq {
			return false
		}
	}
	return true
}

// Wait is what the combat waits for, for one reader.
type Wait struct {
	// Master: the master (or an NPC, or a reactor the reader does not see) holds it.
	Master bool
	// Reactors are the labels of the player's characters the reader sees that react.
	Reactors []string
	// Savers are the labels of the player's characters the reader sees that owe a
	// concentration save.
	Savers []string
}

// Title is the line "Esperando ...": "Esperando o mestre", "Esperando a reação de
// Sálvia", "Esperando o mestre e a reação de Sálvia", "Esperando o teste de
// Constituição de Sálvia".
func (w Wait) Title() string {
	var parts []string
	if w.Master {
		parts = append(parts, "o mestre")
	}
	if len(w.Reactors) > 0 {
		parts = append(parts, "a reação de "+join(sorted(w.Reactors)))
	}
	if len(w.Savers) > 0 {
		parts = append(parts, "o teste de Constituição de "+join(sorted(w.Savers)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Esperando " + join(parts)
}

func sorted(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return slices.Compact(out)
}

// join writes a list with commas and a last "e".
func join(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " e " + items[len(items)-1]
}

// Holds is what the reader's own action is held for.
type Holds int

// The things a window holds, for a reader.
const (
	// HoldsTurn: nothing of the reader's: the turn goes on when it is answered.
	HoldsTurn Holds = iota
	// HoldsYourAttack: the reader's attack (or damage) result.
	HoldsYourAttack
	// HoldsYourSpell: the reader's casting.
	HoldsYourSpell
	// HoldsFall: the fall of a trap.
	HoldsFall
)

// Detail is the line under the title: what waits, with no number and no NPC. The
// master is the one that answers when Wait.Master is set ("ele" is the master);
// when only a visible player answers it says "quando responder" (no pronoun for
// a person). trap names the trap of a fall.
func Detail(h Holds, w Wait, trap string) string {
	then := "quando responder."
	if w.Master {
		then = "quando ele responder."
	}
	switch h {
	case HoldsYourAttack:
		return "O resultado do seu ataque sai " + then
	case HoldsYourSpell:
		return "A sua conjuração se resolve " + then
	case HoldsFall:
		if trap == "" {
			trap = "queda"
		}
		return "A queda no " + trap + " só é resolvida " + then
	}
	return "O turno continua " + then
}
