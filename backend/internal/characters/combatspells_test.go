package characters

import (
	"errors"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// These tests need no database.

// TestSpellTargetingFollowsTheSRDText: the combat reads in the SRD's prose whether
// a spell hits an area and whether it gets an additional target for each slot
// level; a spell attack, Magic Missile and a single-target heal are never areas.
func TestSpellTargetingFollowsTheSRDText(t *testing.T) {
	t.Parallel()
	s := offlineService(t)
	for key, want := range map[string]bool{
		"spell:burning-hands": true, "spell:thunderwave": true, "spell:fireball": true, "spell:sleep": true, "spell:entangle": true,
		"spell:web": true,
		// The SRD says how many (hand-written, effects/spell_targets.json): a number, not any.
		"spell:bless": false, "spell:prayer-of-healing": false, "spell:mass-healing-word": false,
		"spell:hold-person": false, "spell:cure-wounds": false, "spell:healing-word": false, "spell:sacred-flame": false,
		"spell:guiding-bolt": false, "spell:magic-missile": false, "spell:shield": false, "spell:scorching-ray": false,
	} {
		det, ok := s.srd.SpellDetails(key)
		if !ok {
			t.Errorf("%s is not in the content", key)
			continue
		}
		if got := det.Target.AnyNumber(); got != want {
			t.Errorf("any number of targets for %s = %v, want %v", key, got, want)
		}
	}
	for key, want := range map[string]bool{"spell:hold-person": true, "spell:charm-person": true, "spell:bless": true, "spell:cure-wounds": false, "spell:shield": false} {
		det, _ := s.srd.SpellDetails(key)
		if got := det.Target.PerSlotLevel > 0; got != want {
			t.Errorf("an additional target per level for %s = %v, want %v", key, got, want)
		}
	}
}

// TestResourcesInVitals: the uses spent of each resource are cut to the sheet's
// total on every read, a stored key the sheet no longer has is ignored, and the
// master's correction checks the key and the total (RN-02).
func TestResourcesInVitals(t *testing.T) {
	t.Parallel()
	m := wizard3()
	m.resources = []rules.Resource{
		{Key: "second_wind", NamePT: "Retomar o Fôlego", Max: 1, Recharge: rules.RechargeShortRest},
		{Key: "rage", NamePT: "Fúria", Max: 3, Recharge: rules.RechargeLongRest},
	}
	v := vitalsToProto(vitalsRow{ID: "c1", ResourcesUsed: []byte(`{"second_wind": 5, "rage": 2, "ki": 1}`)}, m)
	if got := v.GetResources(); len(got) != 2 || got[0].GetKey() != "second_wind" || got[0].GetUsed() != 1 || got[0].GetTotal() != 1 ||
		got[0].GetRecharge() != rulesv1.Recharge_RECHARGE_SHORT_REST || got[1].GetUsed() != 2 || got[1].GetNamePt() != "Fúria" {
		t.Errorf("resources = %v, want second_wind 1 of 1 (cut) and rage 2 of 3, and no ki", got)
	}
	if v := vitalsToProto(vitalsRow{ID: "c1", ResourcesUsed: []byte(`not json`)}, m); v.GetResources()[0].GetUsed() != 0 {
		t.Errorf("a stored value that does not read = %v, want nothing spent", v.GetResources())
	}

	for name, tt := range map[string]struct {
		req   []*playv1.ResourceUsed
		field string // "" means accepted
	}{
		"a use brought back":   {[]*playv1.ResourceUsed{{Key: "rage", Used: 0}}, ""},
		"every use":            {[]*playv1.ResourceUsed{{Key: "rage", Used: 3}}, ""},
		"more than the total":  {[]*playv1.ResourceUsed{{Key: "rage", Used: 4}}, "resources_used[0].used"},
		"a resource not there": {[]*playv1.ResourceUsed{{Key: "ki", Used: 1}}, "resources_used[0].key"},
		"a resource twice":     {[]*playv1.ResourceUsed{{Key: "rage", Used: 1}, {Key: "rage", Used: 2}}, "resources_used[1].key"},
	} {
		v := vitalsToProto(vitalsRow{ID: "c1"}, m)
		err := applyVitalsChange(v, &playv1.AdjustCharacterVitalsRequest{ResourcesUsed: tt.req})
		fe, isField := errors.AsType[*fieldError](err)
		switch {
		case tt.field == "" && err != nil:
			t.Errorf("%s: error = %v, want accepted", name, err)
		case tt.field != "" && (!isField || fe.field != tt.field):
			t.Errorf("%s: error = %v, want a field error on %s", name, err, tt.field)
		}
	}
	v = vitalsToProto(vitalsRow{ID: "c1"}, m)
	if err := applyVitalsChange(v, &playv1.AdjustCharacterVitalsRequest{ResourcesUsed: []*playv1.ResourceUsed{{Key: "rage", Used: 2}}}); err != nil || v.GetResources()[1].GetUsed() != 2 || v.GetResources()[0].GetUsed() != 0 {
		t.Errorf("a correction of rage = %v, %v; want rage 2 and the rest as they were", v.GetResources(), err)
	}
}

// TestCasterForThirdCasterSubclass: a Fighter whose table subclass casts from the
// wizard's list, multiclassed with a Cleric listed first, casts a wizard spell
// with the subclass's ability and DC, not the Cleric's (the Cleric's list does
// not have it, and the first caster is no fallback when a caster has the list).
func TestCasterForThirdCasterSubclass(t *testing.T) {
	t.Parallel()
	srd, err := rules.LoadSRD()
	if err != nil {
		t.Fatal(err)
	}
	sub := rules.TableSubclass{
		Key: "subclass:cavaleiro-runico@mesa", NamePT: "Cavaleiro Rúnico", Class: "class:fighter",
		Casting: &rules.TableCasting{Kind: rules.CastingThird, Ability: rules.INT, Preparation: rules.PreparationKnown, ListFrom: "class:wizard"},
	}
	for lvl := 3; lvl <= rules.MaxLevel; lvl++ {
		sub.Levels = append(sub.Levels, rules.TableSubclassLevel{Level: lvl, CantripsKnown: 2, SpellsKnown: 3, Slots: [9]int{2}})
	}
	c, err := srd.With(rules.Overlay{Revision: 1, Subclasses: []rules.TableSubclass{sub}})
	if err != nil {
		t.Fatal(err)
	}
	b := rules.Build{
		BaseScores: map[rules.Ability]int{rules.STR: 10, rules.DEX: 10, rules.CON: 10, rules.INT: 18, rules.WIS: 14, rules.CHA: 10},
		Race:       "race:human", Background: "background:acolyte",
		Classes: []rules.ClassLevel{
			{Class: "class:cleric", Subclass: "subclass:life", Level: 3},
			{Class: "class:fighter", Subclass: sub.Key, Level: 5},
		},
	}
	d := rules.Derive(b, c)
	det, ok := c.SpellDetails("spell:fireball")
	if !ok {
		t.Fatal("no fireball")
	}
	sc := casterFor(d, det.Spell.Classes)
	if sc == nil || sc.Class != "class:fighter" || sc.Ability != rules.INT || sc.SpellList != "class:wizard" {
		t.Fatalf("caster = %+v", sc)
	}
	if want := 8 + d.ProficiencyBonus + 4; sc.SaveDC != want {
		t.Errorf("save DC = %d, want %d (INT 18)", sc.SaveDC, want)
	}
	// A cleric spell still goes to the cleric.
	cure, _ := c.SpellDetails("spell:cure-wounds")
	if sc := casterFor(d, cure.Spell.Classes); sc == nil || sc.Class != "class:cleric" {
		t.Errorf("cure wounds caster = %+v", sc)
	}
}
