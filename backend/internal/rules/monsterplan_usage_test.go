package rules

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// limitedActions is every limited action of the SRD's stat blocks as "creature|action|kind|n": n is the
// lowest d6 that recharges a Recharge action, the uses a day of a per-day action, and 1 for one that
// recharges after a rest. A wrong parse of an action's limit shows here by name.
const limitedActions = `
aboleth|Enslave|per_day|3
androsphinx|Roar|per_day|3
androsphinx|First Roar|per_day|3
androsphinx|Second Roar|per_day|3
androsphinx|Third Roar|per_day|3
ankheg|Acid Spray|recharge|6
giant-spider|Web|recharge|5
behir|Lightning Breath|recharge|5
gibbering-mouther|Blinding Spittle|recharge|5
night-hag|Nightmare Haunting|per_day|1
hell-hound|Fire Breath|recharge|5
blink-dog|Teleport|recharge|4
cloaker|Phantasms|rest|1
knight|Leadership|rest|1
deva|Healing Touch|per_day|3
chain-devil|Animate Chains|rest|1
adult-blue-dragon|Lightning Breath|recharge|5
ancient-blue-dragon|Lightning Breath|recharge|5
young-blue-dragon|Lightning Breath|recharge|5
adult-white-dragon|Cold Breath|recharge|5
ancient-white-dragon|Cold Breath|recharge|5
young-white-dragon|Cold Breath|recharge|5
adult-bronze-dragon|Breath Weapons|recharge|5
adult-bronze-dragon|Lightning Breath|recharge|5
adult-bronze-dragon|Repulsion Breath|recharge|5
ancient-bronze-dragon|Breath Weapons|recharge|5
ancient-bronze-dragon|Lightning Breath|recharge|5
ancient-bronze-dragon|Repulsion Breath|recharge|5
young-bronze-dragon|Breath Weapons|recharge|5
young-bronze-dragon|Lightning Breath|recharge|5
young-bronze-dragon|Repulsion Breath|recharge|5
adult-copper-dragon|Breath Weapons|recharge|5
adult-copper-dragon|Acid Breath|recharge|5
adult-copper-dragon|Slowing Breath|recharge|5
ancient-copper-dragon|Breath Weapons|recharge|5
ancient-copper-dragon|Acid Breath|recharge|5
ancient-copper-dragon|Slowing Breath|recharge|5
young-copper-dragon|Breath Weapons|recharge|5
young-copper-dragon|Acid Breath|recharge|5
young-copper-dragon|Slowing Breath|recharge|5
adult-brass-dragon|Breath Weapons|recharge|5
adult-brass-dragon|Fire Breath|recharge|5
adult-brass-dragon|Sleep Breath|recharge|5
ancient-brass-dragon|Breath Weapons|recharge|5
ancient-brass-dragon|Fire Breath|recharge|5
ancient-brass-dragon|Sleep Breath|recharge|5
young-brass-dragon|Breath Weapons|recharge|5
young-brass-dragon|Fire Breath|recharge|5
young-brass-dragon|Sleep Breath|recharge|5
adult-gold-dragon|Breath Weapons|recharge|5
adult-gold-dragon|Fire Breath|recharge|5
adult-gold-dragon|Weakening Breath|recharge|5
ancient-gold-dragon|Breath Weapons|recharge|5
ancient-gold-dragon|Fire Breath|recharge|5
ancient-gold-dragon|Weakening Breath|recharge|5
young-gold-dragon|Breath Weapons|recharge|5
young-gold-dragon|Fire Breath|recharge|5
young-gold-dragon|Weakening Breath|recharge|5
adult-silver-dragon|Breath Weapons|recharge|5
adult-silver-dragon|Cold Breath|recharge|5
adult-silver-dragon|Paralyzing Breath|recharge|5
ancient-silver-dragon|Breath Weapons|recharge|5
ancient-silver-dragon|Cold Breath|recharge|5
ancient-silver-dragon|Paralyzing Breath|recharge|5
young-silver-dragon|Breath Weapons|recharge|5
young-silver-dragon|Cold Breath|recharge|5
young-silver-dragon|Paralyzing Breath|recharge|5
adult-black-dragon|Acid Breath|recharge|5
ancient-black-dragon|Acid Breath|recharge|5
young-black-dragon|Acid Breath|recharge|5
adult-green-dragon|Poison Breath|recharge|5
ancient-green-dragon|Poison Breath|recharge|5
young-green-dragon|Poison Breath|recharge|5
adult-red-dragon|Fire Breath|recharge|5
ancient-red-dragon|Fire Breath|recharge|5
young-red-dragon|Fire Breath|recharge|5
dretch|Fetid Cloud|per_day|1
duergar|Enlarge|rest|1
duergar|Invisibility|rest|1
water-elemental|Whelm|recharge|4
air-elemental|Whirlwind|recharge|4
ettercap|Web|recharge|5
ghost|Possession|recharge|6
blue-dragon-wyrmling|Lightning Breath|recharge|5
white-dragon-wyrmling|Cold Breath|recharge|5
bronze-dragon-wyrmling|Breath Weapons|recharge|5
bronze-dragon-wyrmling|Lightning Breath|recharge|5
bronze-dragon-wyrmling|Repulsion Breath|recharge|5
copper-dragon-wyrmling|Breath Weapons|recharge|5
copper-dragon-wyrmling|Acid Breath|recharge|5
copper-dragon-wyrmling|Slowing Breath|recharge|5
brass-dragon-wyrmling|Breath Weapons|recharge|5
brass-dragon-wyrmling|Fire Breath|recharge|5
brass-dragon-wyrmling|Sleep Breath|recharge|5
gold-dragon-wyrmling|Breath Weapons|recharge|5
gold-dragon-wyrmling|Fire Breath|recharge|5
gold-dragon-wyrmling|Weakening Breath|recharge|5
silver-dragon-wyrmling|Breath Weapons|recharge|5
silver-dragon-wyrmling|Cold Breath|recharge|5
silver-dragon-wyrmling|Paralyzing Breath|recharge|5
black-dragon-wyrmling|Acid Breath|recharge|5
green-dragon-wyrmling|Poison Breath|recharge|5
red-dragon-wyrmling|Fire Breath|recharge|5
storm-giant|Lightning Strike|recharge|5
clay-golem|Haste|recharge|5
iron-golem|Poison Breath|recharge|5
stone-golem|Slow|recharge|5
gorgon|Petrifying Breath|recharge|5
winter-wolf|Cold Breath|recharge|5
darkmantle|Darkness Aura|per_day|1
ice-mephit|Frost Breath|recharge|6
magma-mephit|Fire Breath|recharge|6
dust-mephit|Blinding Breath|recharge|6
steam-mephit|Steam Breath|recharge|6
nalfeshnee|Horror Nimbus|recharge|5
planetar|Healing Touch|per_day|4
octopus|Ink Cloud|rest|1
giant-octopus|Ink Cloud|rest|1
quasit|Scare|per_day|1
chimera|Fire Breath|recharge|5
solar|Healing Touch|per_day|4
dragon-turtle|Steam Breath|recharge|5
treant|Animate Trees|per_day|1
unicorn|Healing Touch|per_day|3
unicorn|Teleport|per_day|1
vampire-bat|Children of the Night|per_day|1
vampire-vampire|Children of the Night|per_day|1
half-red-dragon-veteran|Fire Breath|recharge|5
vrock|Spores|recharge|6
vrock|Stunning Screech|per_day|1`

func TestActionUsageOfEverySRDMonsterAction(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	var got []string
	for _, e := range mustList(t, c, CreatureFilter{}) {
		p, _ := c.MonsterPlan(e.Key)
		for _, a := range p.Actions {
			u := a.Usage
			if !u.Limited() {
				if u.UsesAllowed() != 0 || u.RechargedBy(6) {
					t.Errorf("%s %s: an unlimited action has uses %d", e.Key, a.Name, u.UsesAllowed())
				}
				continue
			}
			n := u.UsesAllowed()
			if u.Kind == UsageRecharge {
				n = u.RechargeMin
			}
			got = append(got, fmt.Sprintf("%s|%s|%s|%d", strings.TrimPrefix(e.Key, "monster:"), a.Name, u.Kind, n))
		}
	}
	want := strings.Split(strings.TrimSpace(limitedActions), "\n")
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("limited actions differ (got %d, want %d)\n only in got: %q\n only in want: %q", len(got), len(want), diffOf(got, want), diffOf(want, got))
	}
}

// diffOf is what a has that b has not.
func diffOf(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func TestActionUsageLimitsAndRecharge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		text    string
		kind    string
		allowed int
		hits    []int
	}{
		{"Recharge 6", UsageRecharge, 1, []int{6}},
		{"Recharge 5-6", UsageRecharge, 1, []int{5, 6}},
		{"Recharge 4-6", UsageRecharge, 1, []int{4, 5, 6}},
		{"3/day", UsagePerDay, 3, nil},
		{"1/Day", UsagePerDay, 1, nil},
		{"Recharges after a short or long rest", UsageRest, 1, nil},
		{"", UsageAtWill, 0, nil},
	} {
		u := parseUsage(tc.text)
		if u.Kind != tc.kind || u.UsesAllowed() != tc.allowed || u.Limited() != (tc.kind != UsageAtWill) {
			t.Errorf("%q: kind %s, uses %d, limited %v", tc.text, u.Kind, u.UsesAllowed(), u.Limited())
		}
		for face := 1; face <= 6; face++ {
			if got, want := u.RechargedBy(face), slices.Contains(tc.hits, face); got != want {
				t.Errorf("%q: a %d recharges %v, want %v", tc.text, face, got, want)
			}
		}
	}
}
