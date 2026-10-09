package play

import (
	"fmt"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

const greataxe = "equipment:greataxe"

// brutalFight is a fight with Ragna, a barbarian of the given level with a greataxe
// (d12) and a shortbow, next to the goblin; Pensantus and Brisa are there too.
func brutalFight(t *testing.T, level int32) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Ragna", "class:barbarian", "race:human", level,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{greataxe, shortbowKey}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.goblin.GetId()}},
		npcRolls: []int{3},
		players:  map[string]int32{"Ragna": 18, "Pensantus": 10, "Brisa": 1},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Ragna": {3, 3}, "Goblin": {4, 3}, "Pensantus": {10, 3}, "Brisa": {8, 8}},
	})
	return a, e
}

// TestBrutalCriticalAddsWeaponDiceToAMeleeCritical: a barbarian's melee critical hit
// rolls 1, 2 or 3 more weapon damage dice at levels 9, 13 and 17 (SRD 5.1, Barbarian:
// Brutal Critical), after the critical's own dice, named in the damage; a hit that is
// not a critical one, a ranged attack, a spell and a barbarian below level 9 get
// none.
func TestBrutalCriticalAddsWeaponDiceToAMeleeCritical(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		level int32
		extra int32
	}{{8, 0}, {9, 1}, {13, 2}, {17, 3}} {
		t.Run(fmt.Sprintf("level %d", tc.level), func(t *testing.T) {
			t.Parallel()
			a, e := brutalFight(t, tc.level)
			p := a.mustAttack(t, a.caio, e, "Ragna", greataxe, "Goblin", d20(20)).GetPendingDamage()
			if !p.GetCritical() || p.GetDiceCount() != 2 || p.GetDiceSides() != 12 || p.GetExtraDiceCount() != tc.extra {
				t.Fatalf("a level %d critical = %d dice of d%d and %d extra, want 2 of d12 and %d extra", tc.level, p.GetDiceCount(), p.GetDiceSides(), p.GetExtraDiceCount(), tc.extra)
			}
			if (tc.extra > 0) != (p.GetExtraDiceNamePt() == "Crítico Brutal") || (tc.extra == 0 && p.GetExtraDiceNamePt() != "") {
				t.Errorf("the extra dice are named %q for %d dice", p.GetExtraDiceNamePt(), tc.extra)
			}
			// The app rolls them all: the critical's own first, the feature's last.
			faces := []int32{7, 11, 4, 9, 2}[:2+tc.extra]
			a.h.roller.queue(intsOf(faces)...)
			rolled := a.mustDamage(t, a.caio, e, p.GetId(), inAppDamage).GetPendingDamage()
			var sum int32
			for _, f := range faces {
				sum += f
			}
			if got := rolled.GetRoll(); got.GetDiceCount() != 2+tc.extra || len(got.GetFaces()) != int(2+tc.extra) || got.GetTotal() != sum+got.GetModifier() {
				t.Errorf("the damage roll = %v, want %d dice with faces %v", got, 2+tc.extra, faces)
			}
			for i, f := range faces {
				if rolled.GetRoll().GetFaces()[i] != f {
					t.Errorf("faces = %v, want %v (the critical's own first)", rolled.GetRoll().GetFaces(), faces)
					break
				}
			}
			if rolled.GetExtraDiceCount() != tc.extra {
				t.Errorf("the rolled damage says %d extra dice, want %d", rolled.GetExtraDiceCount(), tc.extra)
			}
			// The log tells the same, to the master and to the table.
			for name, u := range map[string]*user{"master": a.master, "the roller": a.caio} {
				var d *playv1.CombatLogDamage
				for _, en := range a.log(t, u, e).GetRounds()[0].GetEntries() {
					if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetActorLabel() == "Ragna" {
						d = en.GetDamage()
					}
				}
				if d == nil || d.GetExtraDiceCount() != tc.extra || len(d.GetRoll().GetFaces()) != int(2+tc.extra) {
					t.Errorf("%s reads the damage line %v, want %d extra dice in a roll of %d", name, d, tc.extra, 2+tc.extra)
				}
			}
		})
	}
}

func intsOf(l []int32) []int {
	out := make([]int, len(l))
	for i, v := range l {
		out[i] = int(v)
	}
	return out
}

// TestBrutalCriticalIsOnlyForAMeleeCritical: a normal hit, a ranged attack and a
// spell attack roll their damage as they always did.
func TestBrutalCriticalIsOnlyForAMeleeCritical(t *testing.T) {
	t.Parallel()
	t.Run("a plain hit", func(t *testing.T) {
		t.Parallel()
		a, e := brutalFight(t, 9)
		if p := a.mustAttack(t, a.caio, e, "Ragna", greataxe, "Goblin", d20(15)).GetPendingDamage(); p.GetCritical() || p.GetExtraDiceCount() != 0 || p.GetDiceCount() != 1 {
			t.Errorf("a plain hit = critical %v, %d dice, %d extra; want one die and no extra", p.GetCritical(), p.GetDiceCount(), p.GetExtraDiceCount())
		}
	})
	t.Run("a ranged attack", func(t *testing.T) {
		t.Parallel()
		a, e := brutalFight(t, 9)
		if p := a.mustAttack(t, a.caio, e, "Ragna", shortbowKey, "Goblin", d20(20)).GetPendingDamage(); !p.GetCritical() || p.GetExtraDiceCount() != 0 || p.GetExtraDiceNamePt() != "" {
			t.Errorf("a ranged critical = %d extra dice named %q, want none", p.GetExtraDiceCount(), p.GetExtraDiceNamePt())
		}
	})
	t.Run("a spell attack", func(t *testing.T) {
		t.Parallel()
		a, e := brutalFight(t, 9)
		e = a.passTo(t, e, "Pensantus")
		if p := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Goblin", d20(20)).GetPendingDamage(); !p.GetCritical() || p.GetExtraDiceCount() != 0 {
			t.Errorf("a spell critical = %d extra dice, want none", p.GetExtraDiceCount())
		}
	})
}

// TestBrutalCriticalCountsInThePhysicalDiceSum: with real dice the player is told
// the dice of the critical and of the feature, and the sum typed is checked against
// all of them (3 to 36 for 3d12), whichever rule the table has for the critical's
// own dice: under "máximo mais uma rolagem" the maximum comes alone and the feature's
// die is still rolled.
func TestBrutalCriticalCountsInThePhysicalDiceSum(t *testing.T) {
	t.Parallel()
	a, e := brutalFight(t, 9)
	a.setPhysical(t, a.caio)
	p := a.mustAttack(t, a.caio, e, "Ragna", greataxe, "Goblin", d20(20)).GetPendingDamage()
	for _, sum := range []int32{2, 37} {
		if _, err := a.damage(t, a.caio, e, p.GetId(), typedDamage(sum)); err == nil {
			t.Errorf("typed_sum %d for 3d12 was accepted", sum)
		}
	}
	rolled := a.mustDamage(t, a.caio, e, p.GetId(), typedDamage(22)).GetPendingDamage()
	if !rolled.GetRoll().GetPhysical() || rolled.GetRoll().GetDiceCount() != 3 || rolled.GetExtraDiceCount() != 1 {
		t.Errorf("the typed damage = %v with %d extra, want a physical roll of 3 dice", rolled.GetRoll(), rolled.GetExtraDiceCount())
	}

	// The other rule: 1d12 for the critical's own dice (the maximum, 12, comes without
	// rolling) and 1d12 of the feature: 2 to 24.
	a2, e2 := brutalFight(t, 9)
	a2.setRules(t, criticalIs(maxRoll))
	a2.setPhysical(t, a2.caio)
	p2 := a2.mustAttack(t, a2.caio, e2, "Ragna", greataxe, "Goblin", d20(20)).GetPendingDamage()
	if p2.GetDiceCount() != 1 || p2.GetCriticalMax() != 12 || p2.GetExtraDiceCount() != 1 {
		t.Fatalf("the critical under the maximum rule = %d dice, %d kept, %d extra; want 1, 12 and 1", p2.GetDiceCount(), p2.GetCriticalMax(), p2.GetExtraDiceCount())
	}
	for _, sum := range []int32{1, 25} {
		if _, err := a2.damage(t, a2.caio, e2, p2.GetId(), typedDamage(sum)); err == nil {
			t.Errorf("typed_sum %d for 2d12 was accepted", sum)
		}
	}
	r2 := a2.mustDamage(t, a2.caio, e2, p2.GetId(), typedDamage(10)).GetPendingDamage()
	if r2.GetRoll().GetTotal() != 10+12+r2.GetBonus() {
		t.Errorf("the roll total = %d, want 10 + the 12 kept + %d", r2.GetRoll().GetTotal(), r2.GetBonus())
	}
}
