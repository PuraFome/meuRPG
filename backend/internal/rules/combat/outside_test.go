package combat

import (
	"slices"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func castSpell(key string, level int, ritual bool, unit string, amount int) rules.CharacterSpell {
	return rules.CharacterSpell{Spell: rules.SpellEntry{
		Key: key, Name: key, NamePT: key, Level: level, Ritual: ritual, CastingTime: rules.CastingTime{Unit: unit, Amount: amount},
	}, Prepared: true}
}

func TestOutsideSpellsLists(t *testing.T) {
	t.Parallel()
	wizard := rules.Spellcasting{Class: "class:wizard", Ritual: true, PreparesSpells: true}
	d := rules.Derived{
		Spellcasting: []rules.Spellcasting{wizard},
		SpellSlots:   []int{2, 1, 0, 0, 0, 0, 0, 0, 0},
		Spells: []rules.CharacterSpell{
			castSpell("spell:magic-missile", 1, false, rules.CastAction, 1),
			castSpell("spell:light", 0, false, rules.CastAction, 1),
			{Spell: rules.SpellEntry{Key: "spell:detect-magic", Level: 1, Ritual: true, CastingTime: rules.CastingTime{Unit: rules.CastAction, Amount: 1}}}, // the spellbook, not prepared
			{Spell: rules.SpellEntry{Key: "spell:sleep", Level: 1, CastingTime: rules.CastingTime{Unit: rules.CastAction, Amount: 1}}},                      // the spellbook, not a ritual
			castSpell("spell:prayer-of-healing", 2, false, rules.CastMinute, 10),
			castSpell("spell:shield", 1, false, rules.CastReaction, 1),
		},
	}
	got := OutsideSpells(d, Usage{})
	keys := func(list []OutsideSpell) []string {
		var out []string
		for _, o := range list {
			out = append(out, o.Spell.Key)
		}
		return out
	}
	// Cantrips first, then by the spell's level, then by name; the spellbook spell that is not a ritual is out.
	want := []string{"spell:light", "spell:detect-magic", "spell:magic-missile", "spell:shield", "spell:prayer-of-healing"}
	if !slices.Equal(keys(got), want) {
		t.Fatalf("spells = %v, want %v", keys(got), want)
	}
	by := func(key string) OutsideSpell { return got[slices.Index(keys(got), key)] }

	// A cantrip needs no slot.
	if l := by("spell:light"); !l.CanCast || len(l.Slots) != 0 || l.CanRitual {
		t.Errorf("a cantrip = %+v, want castable with no slots and no ritual", l)
	}
	// A ritual in the spellbook can be cast as a ritual only: 10 minutes, no slot.
	if r := by("spell:detect-magic"); r.CanCast || !r.CanRitual || r.RitualMinutes != 10 || r.Minutes != 0 || !r.Enabled() {
		t.Errorf("a spellbook ritual = %+v, want a ritual of 10 minutes only", r)
	}
	// A prepared spell can use any free slot from its level up.
	if m := by("spell:magic-missile"); !m.CanCast || len(m.Slots) != 2 || m.Slots[0].Level != 1 || m.Slots[0].Free != 2 || m.Slots[1].Level != 2 {
		t.Errorf("Magic Missile = %+v, want the 1st (2 free) and 2nd slots", m)
	}
	// A reaction spell is cast outside a fight like any other; nothing triggers it, the table does.
	if s := by("spell:shield"); !s.CanCast || s.Minutes != 0 {
		t.Errorf("Shield = %+v, want castable at once", s)
	}
	// The casting time is read in minutes, the ritual's 10 more included.
	if p := by("spell:prayer-of-healing"); p.Minutes != 10 || p.CanRitual {
		t.Errorf("Prayer of Healing = %+v, want 10 minutes and no ritual tag", p)
	}
}

func TestOutsideSpellsWithoutSlots(t *testing.T) {
	t.Parallel()
	cleric := rules.Spellcasting{Class: "class:cleric", Ritual: true, PreparesSpells: true}
	d := rules.Derived{
		Spellcasting: []rules.Spellcasting{cleric}, SpellSlots: []int{1, 0, 0, 0, 0, 0, 0, 0, 0},
		Spells: []rules.CharacterSpell{
			castSpell("spell:cure-wounds", 1, false, rules.CastAction, 1),
			castSpell("spell:detect-magic", 1, true, rules.CastAction, 1),
			castSpell("spell:spiritual-weapon", 2, false, rules.CastBonusAction, 1),
		},
	}
	used := Usage{SlotsUsed: [9]int{1}}
	got := OutsideSpells(d, used)
	by := func(key string) OutsideSpell {
		i := slices.IndexFunc(got, func(o OutsideSpell) bool { return o.Spell.Key == key })
		if i < 0 {
			t.Fatalf("%s is missing from the list", key)
		}
		return got[i]
	}
	// Spent slots: Cure Wounds has no way left, and says which slot would do.
	if c := by("spell:cure-wounds"); c.Enabled() || c.Reason == nil || c.Reason.Code != ReasonNoSlot || c.Reason.MinLevel != 1 {
		t.Errorf("Cure Wounds with no free slot = %+v, want disabled with NO_SLOT at level 1", c)
	}
	// The ritual still works with the slots spent: that is the point of a ritual.
	if r := by("spell:detect-magic"); r.CanCast || !r.CanRitual || !r.Enabled() || r.Reason != nil {
		t.Errorf("Detect Magic with no free slot = %+v, want a ritual only", r)
	}
	// There was never a slot of the 2nd level.
	if s := by("spell:spiritual-weapon"); s.Enabled() || s.Reason == nil || s.Reason.MinLevel != 2 {
		t.Errorf("Spiritual Weapon = %+v, want disabled, minimum level 2", s)
	}
}

func TestOutsideSpellsPactSlot(t *testing.T) {
	t.Parallel()
	d := rules.Derived{
		PactMagic:    &rules.PactMagic{SlotLevel: 2, Slots: 2},
		Spellcasting: []rules.Spellcasting{{Class: "class:warlock"}},
		Spells:       []rules.CharacterSpell{castSpell("spell:hold-person", 2, false, rules.CastAction, 1), castSpell("spell:fireball", 3, false, rules.CastAction, 1)},
	}
	got := OutsideSpells(d, Usage{PactSlotsUsed: 1})
	if len(got) != 2 {
		t.Fatalf("spells = %d, want 2", len(got))
	}
	hold := got[0]
	if len(hold.Slots) != 1 || !hold.Slots[0].Pact || hold.Slots[0].Level != 2 || hold.Slots[0].Free != 1 {
		t.Errorf("Hold Person slots = %+v, want the pact slot of the 2nd level, 1 free", hold.Slots)
	}
	// A 3rd-level spell does not fit a pact slot of the 2nd level.
	if fb := got[1]; fb.Enabled() {
		t.Errorf("Fireball with only a 2nd-level pact slot = %+v, want disabled", fb)
	}
}
