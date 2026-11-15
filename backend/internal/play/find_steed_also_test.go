package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// also_steed (SRD 5.1, Find Steed): a spell that targets only the caster also targets the steed the caster rides.

func (a *armed) spellOptionOf(t *testing.T, e *playv1.Encounter, key string) bool {
	t.Helper()
	for _, o := range a.mustOptions(t, a.ana, e, "Aurélio").GetOptions().GetSpells() {
		if o.GetSpell().GetKey() == key {
			return o.GetAlsoSteed()
		}
	}
	t.Fatalf("no spell option %s", key)
	return false
}

func (a *armed) castSelfWithSteed(t *testing.T, e *playv1.Encounter, spell string, also bool) (*playv1.CastSpellResponse, error) {
	t.Helper()
	return a.cast(t, a.ana, e, "Aurélio", spell, slotOfLevel(2), nil, func(r *playv1.CastSpellRequest) { r.AlsoSteed = also })
}

// TestFindSteedOffersTheSteedToSelfOnlySpellsWhileMounted: the option is set for a spell that reaches only the caster while the caster rides its
// bonded steed, and for no other spell, and not before the caster mounts.
func TestFindSteedOffersTheSteedToSelfOnlySpellsWhileMounted(t *testing.T) {
	t.Parallel()
	a, e := riderInFight(t)
	if !a.spellOptionOf(t, e, divineFavor) {
		t.Error("Divine Favor is not offered to the steed while Aurélio rides it")
	}
	if a.spellOptionOf(t, e, cureWounds) {
		t.Error("Cure Wounds (touch) is offered to the steed")
	}
}

// TestFindSteedAlsoSteedNeedsTheRideAndASelfOnlySpell: the refusals, with nothing spent.
func TestFindSteedAlsoSteedNeedsTheRideAndASelfOnlySpell(t *testing.T) {
	t.Parallel()
	a, e := riderInFight(t)
	events := a.events(t)
	if _, err := a.cast(t, a.ana, e, "Aurélio", cureWounds, slotOfLevel(2), a.at(t, "Trovão"), func(r *playv1.CastSpellRequest) { r.AlsoSteed = true }); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("also_steed on Cure Wounds = %v, want invalid_argument", err)
	}
	if a.events(t) != events {
		t.Error("the refusal wrote an event")
	}
}

// TestFindSteedAlsoSteedAsksForTheRide: on foot the caster is not offered the steed, and the cast that asks for it is refused with nothing spent.
func TestFindSteedAlsoSteedAsksForTheRide(t *testing.T) {
	t.Parallel()
	a, e := steedFight(t, false)
	if a.spellOptionOf(t, e, divineFavor) {
		t.Error("Divine Favor is offered to the steed while Aurélio is on foot")
	}
	events := a.events(t)
	if _, err := a.castSelfWithSteed(t, e, divineFavor, true); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("also_steed on foot = %v, want failed_precondition", err)
	}
	if a.events(t) != events || byLabel(t, a.get(t, a.master), "Aurélio").GetBonusActionUsed() {
		t.Error("the refusal changed something")
	}
}

// TestFindSteedAlsoSteedSaysWhatItDidToTheSteed: a cast with also_steed says so in the note, one without says nothing.
func TestFindSteedAlsoSteedSaysWhatItDidToTheSteed(t *testing.T) {
	t.Parallel()
	a, e := riderInFight(t)
	res, err := a.castSelfWithSteed(t, e, divineFavor, true)
	if err != nil {
		t.Fatalf("CastSpell(Divine Favor, also_steed) error = %v", err)
	}
	if res.GetCast().GetSteedNote() == "" {
		t.Error("the cast with also_steed has no note about the steed")
	}
}

// TestFindSteedOptionsCarryTheTextOnlyRules: the telepathic bond, where the steed appears and its language are lines of the spell's options,
// and no other summon spell has them.
func TestFindSteedOptionsCarryTheTextOnlyRules(t *testing.T) {
	t.Parallel()
	a := newRiders(t)
	for _, sp := range a.summonOptions(t, a.ana, a.pens).GetSpells() {
		if sp.GetSpellKey() == findSteed && len(sp.GetNotes()) != 3 {
			t.Errorf("Find Steed notes = %q, want the bond, the place it appears and the language", sp.GetNotes())
		}
		if sp.GetSpellKey() != findSteed && len(sp.GetNotes()) != 0 {
			t.Errorf("%s has notes %q", sp.GetSpellKey(), sp.GetNotes())
		}
	}
}

// TestFindSteedAlsoSteedPutsALastingEffectOnTheSteed: a table spell that targets only the caster and puts an effect on it (+2 armor class and 5
// temporary hit points), cast with also_steed by a rider, puts the effect on its steed too; without also_steed the steed gets none. No SRD
// spell of the paladin has such an effect, so the spell is the table's.
func TestFindSteedAlsoSteedPutsALastingEffectOnTheSteed(t *testing.T) {
	t.Parallel()
	for _, also := range []bool{true, false} {
		a := newArmedWith(t, func(a *armed) {
			mark := tableSpell("Marca de Nanquim", 1, "class:paladin", rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF, tableTarget(rulesv1.TableSpellTargetKind_TABLE_SPELL_TARGET_KIND_SELF))
			mark.Concentration = true
			mark.Duration = &rulesv1.TableSpellDuration{Kind: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED, Amount: 1, Unit: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE, UpTo: true}
			mark.Effect = &rulesv1.TableSpellEffect{Modifiers: []*tableMod{{Kind: modAC, Value: 2}, {Kind: modTemp, Value: 5}}, Visibility: rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC}
			if _, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
				CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableSpell{TableSpell: mark},
			})); err != nil {
				t.Fatalf("CreateTableEntry(Marca de Nanquim) error = %v", err)
			}
			a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
				&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
			a.pens = a.ana.caster(t, a.campaignID, "Aurélio", "class:paladin", "race:human", 9,
				&rulesv1.AbilityScores{Strength: 16, Dexterity: 10, Constitution: 14, Intelligence: 10, Wisdom: 12, Charisma: 16}, nil, nil,
				[]string{findSteed, tableMark}, []string{findSteed, tableMark})
		})
		a.mustCastSteed(t, warhorse, "celestial", "Trovão")
		e := a.start(t, plan{
			npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
			players: map[string]int32{"Aurélio": 18, "Toren": 10, "Trovão": 12},
			reveal:  []string{"Goblin"},
			at:      map[string][2]int32{"Aurélio": {3, 3}, "Toren": {6, 3}, "Goblin": {12, 3}, "Trovão": {4, 3}},
		})
		if _, err := a.ana.combat.MountCombatant(t.Context(), connect.NewRequest(&playv1.MountCombatantRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), RiderId: a.id(t, "Aurélio"), MountId: a.id(t, "Trovão"), IdempotencyKey: newKey(),
		})); err != nil {
			t.Fatalf("MountCombatant error = %v", err)
		}
		if _, err := a.castSelfWithSteed(t, a.get(t, a.master), tableMark, also); err != nil {
			t.Fatalf("CastSpell(Marca, also_steed=%v) error = %v", also, err)
		}
		after := a.get(t, a.master)
		for _, label := range []string{"Aurélio", "Trovão"} {
			c := byLabel(t, after, label)
			marked := c.GetArmorClassBonus() == 2 && c.GetHitPointsTemporary() == 5 && cardOf(t, c, tableMark) != nil
			if want := label == "Aurélio" || also; marked != want {
				t.Errorf("also_steed=%v: %s marked = %v (AC bonus %d, temp %d), want %v", also, label, marked, c.GetArmorClassBonus(), c.GetHitPointsTemporary(), want)
			}
		}
	}
}
