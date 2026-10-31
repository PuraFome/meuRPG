package characters

import (
	"slices"
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

func npcSpellOf(t *testing.T, o combat.TurnOptions, key string) combat.SpellOption {
	t.Helper()
	i := slices.IndexFunc(o.Spells, func(s combat.SpellOption) bool { return s.Spell.Key == key })
	if i < 0 {
		t.Fatalf("no option for %s among %d spells", key, len(o.Spells))
	}
	return o.Spells[i]
}

// An NPC made from a spellcaster casts what its stat block lists (MR-042): the Mage's prepared spells
// with its slots, the Couatl's innate ones at will and with the uses a day, none for a Goblin.
func TestNPCMadeFromASpellcasterHasItsOwnSpells(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	mage := combat.Options(basicDerived(srd, &charactersv1.BasicSheet{MonsterKey: "monster:mage"}), combat.TurnState{}, combat.Usage{})
	if m := npcSpellOf(t, mage, "spell:magic-missile"); !m.Enabled || len(m.Slots) != 5 || m.Slots[0].Free != 4 { // the Mage has slots of the 1st to the 5th level, four of the 1st
		t.Errorf("magic missile = %+v, want enabled with five slot levels, four free of the 1st", m)
	}
	if c := npcSpellOf(t, mage, "spell:fire-bolt"); !c.Enabled || len(c.Slots) != 0 {
		t.Errorf("fire bolt = %+v, want an enabled cantrip", c)
	}
	couatl := combat.Options(basicDerived(srd, &charactersv1.BasicSheet{MonsterKey: "monster:couatl"}), combat.TurnState{}, combat.Usage{})
	if b := npcSpellOf(t, couatl, "spell:bless"); !b.Enabled || b.Grant == nil || b.Grant.AtWill || b.Grant.UsesLeft != 3 || b.Grant.SaveDC != 14 {
		t.Errorf("bless = %+v, want 3 uses with the stat block's DC 14", b)
	}
	if d := npcSpellOf(t, couatl, "spell:detect-magic"); d.Grant == nil || !d.Grant.AtWill {
		t.Errorf("detect magic = %+v, want at will", d)
	}
	goblin := combat.Options(basicDerived(srd, &charactersv1.BasicSheet{MonsterKey: "monster:goblin"}), combat.TurnState{}, combat.Usage{})
	if len(goblin.Spells) != 0 { // the control
		t.Errorf("a goblin lists %d spells, want none", len(goblin.Spells))
	}
}

// The numbers of the cast are the stat block's: the Mage's spell attack bonus, the Lich's save DC.
func TestNPCSpellNumbersAreTheStatBlocks(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	for _, tc := range []struct {
		monster, spell string
		toHit, dc      int
	}{{"monster:mage", "spell:fire-bolt", 6, 0}, {"monster:lich", "spell:thunderwave", 0, 20}} {
		det, ok := srd.SpellDetails(tc.spell)
		if !ok {
			t.Fatalf("no details for %s", tc.spell)
		}
		d := basicDerived(srd, &charactersv1.BasicSheet{MonsterKey: tc.monster})
		got := spellBasics(d, det)
		spellCasterNumbers(&got, d, det, nil)
		if got.ToHit != tc.toHit || got.SaveDC != tc.dc {
			t.Errorf("%s casts %s with +%d to hit and DC %d, want +%d and DC %d", tc.monster, tc.spell, got.ToHit, got.SaveDC, tc.toHit, tc.dc)
		}
	}
}

// A stat-block caster's cantrips scale with its caster level, the "Nth-level spellcaster" of its Spellcasting trait:
// the 9th-level Mage's Fire Bolt rolls 2d10 and the 1st-level Acolyte's Sacred Flame 1d8, not nothing.
func TestNPCCantripsRollTheirDiceAtTheStatBlocksCasterLevel(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatalf("LoadSRD: %v", err)
	}
	for _, tc := range []struct {
		monster, spell string
		count, sides   int
	}{{"monster:mage", "spell:fire-bolt", 2, 10}, {"monster:acolyte", "spell:sacred-flame", 1, 8}} {
		det, ok := srd.SpellDetails(tc.spell)
		if !ok {
			t.Fatalf("no details for %s", tc.spell)
		}
		d := basicDerived(srd, &charactersv1.BasicSheet{MonsterKey: tc.monster})
		var got link.Spell
		spellRolls(&got, d, det, 0, "", 0)
		if len(got.Damages) != 1 || got.Damages[0].Count != tc.count || got.Damages[0].Sides != tc.sides {
			t.Errorf("%s casts %s for %v, want %dd%d", tc.monster, tc.spell, got.Damages, tc.count, tc.sides)
		}
	}
}
