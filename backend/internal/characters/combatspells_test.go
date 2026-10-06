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
		"spell:bless": true, "spell:web": true, "spell:prayer-of-healing": true, "spell:mass-healing-word": true,
		"spell:hold-person": false, "spell:cure-wounds": false, "spell:healing-word": false, "spell:sacred-flame": false,
		"spell:guiding-bolt": false, "spell:magic-missile": false, "spell:shield": false, "spell:scorching-ray": false,
	} {
		det, ok := s.srd.SpellDetails(key)
		if !ok {
			t.Errorf("%s is not in the content", key)
			continue
		}
		if got := isArea(det); got != want {
			t.Errorf("isArea(%s) = %v, want %v", key, got, want)
		}
	}
	for key, want := range map[string]bool{"spell:hold-person": true, "spell:charm-person": true, "spell:bless": true, "spell:cure-wounds": false, "spell:shield": false} {
		det, _ := s.srd.SpellDetails(key)
		if got := extraTargetRE.MatchString(joinLines(det.HigherLevel)); got != want {
			t.Errorf("an additional target per level for %s = %v, want %v", key, got, want)
		}
	}
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + " "
	}
	return out
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
