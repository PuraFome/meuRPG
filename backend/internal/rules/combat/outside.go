package combat

import (
	"cmp"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What a character can cast outside a fight (SRD 5.1, "Spellcasting"): no turn
// and no action economy, so the only limits are the spell being on the sheet, a
// slot being free and, for a ritual, the class being one that casts rituals.

// ReasonNotCastable says the spell can be cast neither with a slot nor as a
// ritual: it is not prepared and no ritual feature reaches it. The sheet never
// lists such a spell, so it is not shown; the code is for a request that names one.
const ReasonNotCastable = "NOT_CASTABLE"

// OutsideSpell is a spell the character may cast outside a combat, with every
// way to cast it.
type OutsideSpell struct {
	Spell rules.SpellEntry
	// Standing is what the sheet says about the spell.
	Standing rules.SpellStanding
	// Slots are the slot levels it can be cast with: at least the spell's level and
	// with a free slot. Empty for a cantrip.
	Slots []SlotChoice
	// CanCast says the spell can be cast the normal way: a cantrip, or a prepared
	// spell with a free slot.
	CanCast bool
	// CanRitual says it can be cast as a ritual (no slot, 10 minutes more).
	CanRitual bool
	// Minutes is how long the normal cast takes (0 for an action, a bonus action
	// or a reaction), and RitualMinutes how long the ritual takes.
	Minutes, RitualMinutes int
	// Reason is set when neither way works: ReasonNoSlot (with MinLevel) or
	// ReasonNotCastable.
	Reason *Reason
}

// Enabled says some way of casting the spell works now.
func (o OutsideSpell) Enabled() bool { return o.CanCast || o.CanRitual }

// OutsideSpells lists the spells of the sheet the character can cast outside a
// fight: the prepared ones and the ones only a ritual reaches (a wizard's
// spellbook). A spell with neither a free slot nor a ritual stays in the list with
// its reason, so the screen can say why. Cantrips first, then by the spell's
// level and Portuguese name.
func OutsideSpells(d rules.Derived, u Usage) []OutsideSpell {
	var out []OutsideSpell
	for _, cs := range d.Spells {
		st := d.SpellStanding(cs.Spell.Key, cs.Spell.Ritual)
		if !cs.Prepared && !st.CanRitual {
			continue // on the book but not castable now
		}
		sp := cs.Spell
		o := OutsideSpell{
			Spell: sp, Standing: st, CanRitual: st.CanRitual,
			Minutes: rules.CastMinutes(sp.CastingTime, false), RitualMinutes: rules.CastMinutes(sp.CastingTime, true),
		}
		if cs.Prepared {
			o.Slots = slotChoices(d, u, sp.Level)
			o.CanCast = sp.Level == 0 || len(o.Slots) > 0
		}
		switch {
		case o.Enabled():
		case cs.Prepared:
			o.Reason = &Reason{Code: ReasonNoSlot, MinLevel: sp.Level}
		default:
			o.Reason = &Reason{Code: ReasonNotCastable}
		}
		out = append(out, o)
	}
	slices.SortStableFunc(out, func(a, b OutsideSpell) int {
		if c := cmp.Compare(a.Spell.Level, b.Spell.Level); c != 0 {
			return c
		}
		return strings.Compare(sortName(a.Spell), sortName(b.Spell))
	})
	return out
}

// slotChoices are the free slots a spell of the level can be cast with: the
// slots of the level and above, and the pact slot when it reaches the level.
func slotChoices(d rules.Derived, u Usage, level int) []SlotChoice {
	if level < 1 {
		return nil
	}
	var out []SlotChoice
	for l := level; l <= 9; l++ {
		if free := SlotsFree(d, u, l); free > 0 {
			out = append(out, SlotChoice{Level: l, Free: free})
		}
	}
	if p := d.PactMagic; p != nil && p.SlotLevel >= level {
		if free := PactSlotsFree(d, u); free > 0 {
			out = append(out, SlotChoice{Level: p.SlotLevel, Pact: true, Free: free})
		}
	}
	return out
}
