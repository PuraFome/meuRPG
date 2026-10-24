package rules

import "testing"

// Which attacks deal magical damage: a spell always, a creature's weapon attack
// when its stat block says its weapon attacks are magical, and no other.
func TestMonsterAttacksAreMagicalOnlyWithTheTrait(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	for key, want := range map[string]bool{
		"monster:unicorn": true, "monster:balor": true, "monster:goblin": false, "monster:wolf": false, "monster:ghost": false,
	} {
		d := monsterDerived(t, c, key)
		if len(d.Attacks) == 0 {
			t.Fatalf("%s has no attacks", key)
		}
		for _, a := range d.Attacks {
			if a.Magical != want {
				t.Errorf("%s: attack %q Magical = %v, want %v", key, a.Name, a.Magical, want)
			}
		}
	}
}

// A creature's resistance and immunity entries carry how the SRD limits them, and
// keep the SRD's words for display.
func TestCreatureDamageModsCarryTheQualifier(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	cases := []struct {
		key        string
		immunities bool
		qualifier  string
		note       string
	}{
		{"monster:ghost", false, "nonmagical", "from nonmagical weapons"},
		{"monster:gargoyle", false, "nonmagical-unless-adamantine", "from nonmagical weapons that aren't adamantine"},
		{"monster:werewolf-wolf", true, "nonmagical-unless-silvered", "from nonmagical weapons that aren't silvered"},
	}
	for _, tt := range cases {
		cr, ok := c.CreatureByKey(tt.key)
		if !ok {
			t.Fatalf("%s: not a creature", tt.key)
		}
		list := cr.Resistances
		if tt.immunities {
			list = cr.Immunities
		}
		var found bool
		for _, m := range list {
			if m.Note == tt.note {
				found = true
				if m.Qualifier != tt.qualifier || len(m.Types) != 3 {
					t.Errorf("%s: entry %+v, want qualifier %q on three types", tt.key, m, tt.qualifier)
				}
			}
		}
		if !found {
			t.Errorf("%s: no entry %q in %+v", tt.key, tt.note, list)
		}
	}
	// An entry with no damage type of its own, and a plain one, carry none.
	specter, _ := c.CreatureByKey("monster:specter")
	for _, m := range specter.Resistances {
		if m.Note == "" && m.Qualifier != "" {
			t.Errorf("a plain resistance carries qualifier %q", m.Qualifier)
		}
	}
}
