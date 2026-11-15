package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
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
