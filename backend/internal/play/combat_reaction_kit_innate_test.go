package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

func npcKit(st link.ReactionStats, used string) kit {
	return kit{who: playdb.Combatant{Kind: kindNPC, SlotsUsed: []byte(used)}, st: st}
}

var (
	shieldInnate = link.InnateUse{Spell: spellShield, Resource: "innate_3_day_shield", Max: 3}
	innateOnly   = link.ReactionStats{StatBlock: true, Innate: []link.InnateUse{shieldInnate}, ResourceMax: map[string]int{shieldInnate.Resource: 3}}
	slotOnly     = link.ReactionStats{StatBlock: true, Spells: []string{spellShield}, SlotsTotal: [9]int{4}}
)

func wantInvalid(t *testing.T, what string, err error) {
	t.Helper()
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("%s: error = %v, want invalid_argument", what, err)
	}
}

// TestPickSlotRefusesWhatTheKitDoesNotOffer: an answer must match the offered choice by level, pact
// and resource, so a made-up resource, a slot level for an innate-only creature, a resource without
// its uses and a resource-less answer against an innate choice are all refused.
func TestPickSlotRefusesWhatTheKitDoesNotOffer(t *testing.T) {
	t.Parallel()
	mage := npcKit(slotOnly, `{}`).slotsFor(spellShield)
	_, err := pickSlot(&playv1.SpellSlot{Level: 1, Resource: "innate_3_day_shield"}, mage)
	wantInvalid(t, "a mage answering with a made-up resource", err)
	if got, err := pickSlot(&playv1.SpellSlot{Level: 1}, mage); err != nil || got.Resource != "" {
		t.Errorf("a mage answering with a slot = %+v, %v; want the slot", got, err)
	}

	couatl := npcKit(innateOnly, `{}`).slotsFor(spellShield)
	_, err = pickSlot(&playv1.SpellSlot{Level: 1}, couatl)
	wantInvalid(t, "a slot level for an innate-only creature", err)
	_, err = pickSlot(&playv1.SpellSlot{Level: 1, Resource: "innate_1_day_other"}, couatl)
	wantInvalid(t, "a wrong resource", err)
	got, err := pickSlot(&playv1.SpellSlot{Level: 1, Resource: shieldInnate.Resource}, couatl)
	if err != nil || got.Resource != shieldInnate.Resource || got.Level != 1 {
		t.Errorf("the resource answer = %+v, %v; want the innate use", got, err)
	}

	spent := npcKit(innateOnly, `{"res:innate_3_day_shield": 3}`).slotsFor(spellShield)
	if len(spent) != 0 {
		t.Fatalf("choices with the uses spent = %v, want none", spent)
	}
	_, err = pickSlot(&playv1.SpellSlot{Level: 1, Resource: shieldInnate.Resource}, spent)
	wantInvalid(t, "the resource answer with the uses spent", err)
}

// TestSlotsForListsSlotsAndInnateUsesApart: a stat block with both gets both kinds of choice, and the
// answer picks the one its resource names; a player's kit offers its options' slots alone.
func TestSlotsForListsSlotsAndInnateUsesApart(t *testing.T) {
	t.Parallel()
	both := slotOnly
	both.Innate, both.ResourceMax = innateOnly.Innate, innateOnly.ResourceMax
	choices := npcKit(both, `{"res:innate_3_day_shield": 1}`).slotsFor(spellShield)
	var innate *rulesv1.SlotChoice
	slots := 0
	for _, c := range choices {
		if c.GetResource() == "" {
			slots++
		} else {
			innate = c
		}
	}
	if slots != 1 || innate == nil || innate.GetFree() != 2 || innate.GetTotal() != 3 || innate.GetLevel() != 1 {
		t.Fatalf("choices = %v, want the 1st-level slot and the innate use, 2 of 3", choices)
	}
	if got, err := pickSlot(&playv1.SpellSlot{Level: 1}, choices); err != nil || got.Resource != "" {
		t.Errorf("slot answer = %+v, %v; want the slot", got, err)
	}
	if got, err := pickSlot(&playv1.SpellSlot{Level: 1, Resource: shieldInnate.Resource}, choices); err != nil || got.Resource != shieldInnate.Resource {
		t.Errorf("resource answer = %+v, %v; want the innate use", got, err)
	}

	player := kit{who: playdb.Combatant{Kind: kindPlayer}, opts: &rulesv1.TurnOptions{Spells: []*rulesv1.SpellOption{
		{Spell: &rulesv1.Spell{Key: spellShield}, Slots: []*rulesv1.SlotChoice{{Level: 1, Free: 2, Total: 4}}},
	}}}
	got := player.slotsFor(spellShield)
	if len(player.st.Innate) != 0 || len(got) != 1 || got[0].GetResource() != "" || got[0].GetFree() != 2 {
		t.Errorf("a player's choices = %v (innate %v), want its slot alone", got, player.st.Innate)
	}
}
