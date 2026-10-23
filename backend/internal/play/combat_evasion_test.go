package play

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// Evasion (SRD 5.1, Monk 7 and Rogue 7): on a Dexterity saving throw that halves the damage on a
// success, the creature takes none on a success and half on a failure, unless it is incapacitated.
// These tests need the database (MEURPG_TEST_DATABASE_URL).

// newEvasionCave is the cone cave with Toren a character of class at level, Dexterity 16.
func newEvasionCave(t *testing.T, class string, level int32) *cave {
	t.Helper()
	spells := []string{fireball, lightningBolt, burningHands, thunderwave, sleepSpell}
	c := newCaveOf(t, 5, spells, func(a *armed) *charactersv1.Character {
		return a.caio.hero(t, a.campaignID, "Toren", class, "race:human", level, &rulesv1.AbilityScores{Strength: 10, Dexterity: 15, Constitution: 14, Intelligence: 10, Wisdom: 14, Charisma: 8}, nil, nil)
	})
	c.coneFight(t)
	return c
}

// torenDamage casts Burning Hands over the cone with every save rolling face and says whether Toren
// has pending damage, and whether it is the half of it.
func (c *cave) torenDamage(t *testing.T, face int) (pending, half bool) {
	t.Helper()
	c.h.roller.queue(face, face, face)
	res := c.mustCastArea(t, c.ana, "Pensantus", burningHands, slotOfLevel(1), nil, toward(1, 0))
	id := resultOf(t, res, c.id(t, "Toren")).GetPendingDamageId()
	if id == "" {
		return false, false
	}
	for _, p := range res.GetCast().GetPendingDamages() {
		if p.GetId() == id {
			return true, p.GetHalf()
		}
	}
	t.Fatalf("pending damage %s is not in the cast", id)
	return false, false
}

func TestEvasionTakesNoDamageOnASaveAndHalfOnAFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		class string
		level int32
		face  int
		want  string // "none", "half" or "full"
	}{
		{"monk 7 saves", "class:monk", 7, 20, "none"},
		{"monk 7 fails", "class:monk", 7, 1, "half"},
		{"rogue 7 saves", "class:rogue", 7, 20, "none"},
		{"rogue 7 fails", "class:rogue", 7, 1, "half"},
		{"monk 6 saves", "class:monk", 6, 20, "half"},
		{"monk 6 fails", "class:monk", 6, 1, "full"},
		{"rogue 6 saves", "class:rogue", 6, 20, "half"},
		{"fighter 7 saves", "class:fighter", 7, 20, "half"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newEvasionCave(t, tc.class, tc.level)
			pending, half := c.torenDamage(t, tc.face)
			got := "full"
			switch {
			case !pending:
				got = "none"
			case half:
				got = "half"
			}
			if got != tc.want {
				t.Errorf("Toren's damage = %s, want %s", got, tc.want)
			}
		})
	}
}

// An incapacitated creature cannot use Evasion: a pass still halves.
func TestEvasionIsLostWhileIncapacitated(t *testing.T) {
	t.Parallel()
	c := newEvasionCave(t, "class:monk", 7)
	c.setConditions(t, c.get(t, c.master), "Toren", "condition:incapacitated")
	pending, half := c.torenDamage(t, 20)
	if !pending || !half {
		t.Errorf("incapacitated Toren: pending = %v, half = %v, want half damage on a pass", pending, half)
	}
}
