package rules

import (
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// The state gate: the one place where an effect's `while` is read and checked.
//
// An effect with `while` holds only while the named state of its entry is on, and a
// condition (`when`) may read the same states with active("name"). A name belongs
// to the entry the effect is in, as a resource name does: the state effect that
// defines it is in the same class, subclass, race, subrace, background or feat.
//
// To gate another effect type:
//
//  1. add the type to gatedEffectTypes and "while" to its ownFields
//     (overlay_features.go);
//  2. add a `while` field of kind "state_ref" to its menu type (tablemenu.go);
//  3. if its Derive step asks x.applies(a), nothing else is needed (applies reads
//     the gate); a step that does not (a resistance is read by Content.Resistances)
//     asks x.gateOpen(a) itself.
//
// Validation is shared too: checkEffect calls checkGate for every effect, and
// bindGate resolves the names once the entry's states are known.

// gatedEffectTypes are the effect types that read `while`.
var gatedEffectTypes = []string{"resistance", "roll_mode", "extra_damage", "weapon_rule", "pseudo_weapon"}

// checkGate checks the `while` of an effect against the states of its entry. A
// resistance may also name "rage", the barbarian's state; no other type may,
// because the sheet does not know whether the character rages. A resistance
// needs its `while`; the other types are gated only when they carry one.
func (b *overlayBuilder) checkGate(site effectSite, entry string, e *Effect) error {
	if !slices.Contains(gatedEffectTypes, e.Type) || e.While == "" {
		return nil
	}
	if e.Type == "resistance" && e.While == ResistanceWhileRage {
		return nil
	}
	if b.stateKeys[entry][e.While] == "" {
		return site.fail(".while", ReasonReference, "while names %q, which no state effect of this entry defines", e.While)
	}
	return nil
}

// checkStateReads checks what an effect reads of the states of its entry: its `while` and
// the states its condition reads with active().
func (b *overlayBuilder) checkStateReads(site effectSite, entry string, e *Effect) error {
	if err := b.checkGate(site, entry, e); err != nil {
		return err
	}
	return b.checkWhenStates(site, entry, e)
}

// checkWhenStates checks the states a `when` formula reads with active(): each
// must be defined by a state effect of the effect's entry. A name that cannot be a
// state name is left to the compiler, which refuses it as a formula.
func (b *overlayBuilder) checkWhenStates(site effectSite, entry string, e *Effect) error {
	for _, name := range formula.ParseActiveNames(e.When) {
		if b.stateKeys[entry][name] == "" {
			return site.fail(".when", ReasonReference, "the condition reads active(%q), which no state effect of this entry defines", name)
		}
	}
	return nil
}

// bindGate resolves, on the copy of the effect that is compiled, what the gate
// needs at Derive: the entry, the key of the state `while` names and the keys of
// the states the `when` formula reads.
func (b *overlayBuilder) bindGate(entry string, e *Effect) {
	e.entry = entry
	if slices.Contains(gatedEffectTypes, e.Type) && e.While != ResistanceWhileRage {
		e.gateKey = b.stateKeys[entry][e.While]
	}
	for _, name := range formula.ParseActiveNames(e.When) {
		if e.whenStates == nil {
			e.whenStates = map[string]string{}
		}
		e.whenStates[name] = b.stateKeys[entry][name]
	}
}

// stateOn says whether the state with this key is on for the character.
func (x *deriver) stateOn(key string) bool { return key != "" && x.activeKeys[key] }

// gateOpen says whether the effect's `while` lets it hold: it carries none, it
// names the barbarian's rage (play reads that one), or the state it names is on.
func (x *deriver) gateOpen(a activeEffect) bool {
	e := a.effect
	if e.While == "" || e.While == ResistanceWhileRage {
		return true
	}
	return x.stateOn(e.gateKey)
}

// collectActiveStates marks which of the character's states are on: the keys of
// Build.ActiveStates that a state effect of the sheet defines. A key nothing
// defines (a state of content that changed) is ignored.
func (x *deriver) collectActiveStates() {
	x.activeKeys = map[string]bool{}
	if len(x.b.ActiveStates) == 0 {
		return
	}
	for _, a := range x.active {
		if a.effect.Type == "state" && slices.Contains(x.b.ActiveStates, a.effect.stateKey) {
			x.activeKeys[a.effect.stateKey] = true
		}
	}
}
