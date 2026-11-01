package play

import (
	"testing"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Spike Growth (SRD 5.1) keeps a 20-foot radius of difficult terrain; a creature that moves into or within it takes 2d4 piercing
// damage for every 5 feet it travels, with no saving throw. A square entered is 5 feet. These tests need the database
// (MEURPG_TEST_DATABASE_URL); the cave is the one of zones_test.go: Toren at (5,7), Pensantus (12,7), Brisa (3,7), the ogre (7,7).

const spikeGrowthKey = "spell:spike-growth"

func spikeCave(t *testing.T) *cave {
	t.Helper()
	return newCaveOf(t, 9, []string{thunderwave, spikeGrowthKey}, func(a *armed) *charactersv1.Character {
		return a.caio.caster(t, a.campaignID, "Toren", "class:druid", "race:human", 7, abilities(12, 12, 14, 16, 10),
			nil, nil, nil, []string{spikeGrowthKey})
	})
}

// castSpikes has Toren, on turn, put the spikes at the square.
func (c *cave) castSpikes(t *testing.T, col, row int32) *playv1.Encounter {
	t.Helper()
	e := c.zoneFight(t)
	c.mustCastArea(t, c.caio, "Toren", spikeGrowthKey, slotOfLevel(2), at(col, row), nil)
	return e
}

// pendingDiceOf are the dice counts of the damages of the spell waiting to be rolled for the creature.
func (c *cave) pendingDiceOf(t *testing.T, label, spell string) []int {
	t.Helper()
	rows, err := c.h.pool.Query(t.Context(), `SELECT dice_count FROM pending_damages WHERE target_id = $1 AND attack_key = $2 AND status = 'awaiting_roll' ORDER BY created_at, id`, c.id(t, label), spell)
	if err != nil {
		t.Fatalf("read the damages of %s: %v", spell, err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("read a damage of %s: %v", spell, err)
		}
		out = append(out, n)
	}
	return out
}

// spikeDamagesOf are the damages of the spikes waiting to be rolled for the creature: their dice counts.
func (c *cave) spikeDamagesOf(t *testing.T, label string) []int {
	t.Helper()
	return c.pendingDiceOf(t, label, spikeGrowthKey)
}

// walkOgre walks the ogre on its own turn.
func (c *cave) walkOgre(t *testing.T, col, row int32) {
	t.Helper()
	c.passTo(t, c.get(t, c.master), "Ogro")
	if _, err := c.move(t, c.master, "Ogro", col, row); err != nil {
		t.Fatalf("MoveCombatant(Ogro to %d,%d) error = %v", col, row, err)
	}
}

func TestSpikeGrowthIsATwentyFootDifficultCylinderThatAsksNobody(t *testing.T) {
	t.Parallel()
	c := spikeCave(t)
	c.castSpikes(t, 16, 7)
	z := c.oneZone(t, c.master, spikeGrowthKey)
	if far := farthest(z, 16, 7); far != 4 || !z.GetConcentration() || z.GetSaveAbility() != "" || z.GetDamageDice() != "2d4" || z.GetDamageTypeKey() != "damage-type:piercing" {
		t.Errorf("zone = %v, want a 20-foot cylinder (4 squares) held by concentration, 2d4 piercing, no save", z)
	}
	if !hasNote(z, "difficult_terrain") {
		t.Errorf("notes = %v, want difficult_terrain", z.GetNotes())
	}
	if got := c.spikeDamagesOf(t, "Pensantus"); len(got) != 0 {
		t.Errorf("the cast hurt Pensantus, standing inside: %v, want none: the spikes hurt who travels", got)
	}
}

// The ogre walks from (7,7) to (11,7): of the squares it enters, (10,7) and (11,7) are inside, so 2d4 twice.
func TestSpikeGrowthHurtsForEverySquareEnteredInside(t *testing.T) {
	t.Parallel()
	c := spikeCave(t)
	c.castSpikes(t, 14, 7)
	c.walkOgre(t, 11, 7)
	if got := c.spikeDamagesOf(t, "Ogro"); len(got) != 1 || got[0] != 4 {
		t.Fatalf("damages of the ogre = %v, want one of 4 dice (2d4 twice)", got)
	}
}

// A creature placed inside by the master is not hurt: it did not travel. Walking within the area is hurt for each square.
func TestSpikeGrowthHurtsWhoMovesWithinAndNotWhoIsPlaced(t *testing.T) {
	t.Parallel()
	c := spikeCave(t)
	c.castSpikes(t, 14, 7)
	c.mustMove(t, c.master, "Ogro", 11, 7)
	if got := c.spikeDamagesOf(t, "Ogro"); len(got) != 0 {
		t.Fatalf("a placement hurt the ogre: %v, want none", got)
	}
	c.walkOgre(t, 13, 7)
	if got := c.spikeDamagesOf(t, "Ogro"); len(got) != 1 || got[0] != 4 {
		t.Errorf("damages after two squares inside = %v, want one of 4 dice", got)
	}
}

// Leaving: only the squares entered inside count.
func TestSpikeGrowthDoesNotHurtTheSquaresOutside(t *testing.T) {
	t.Parallel()
	c := spikeCave(t)
	c.castSpikes(t, 14, 7)
	c.mustMove(t, c.master, "Ogro", 11, 7)
	c.walkOgre(t, 9, 7) // (10,7) is inside, (9,7) is out
	if got := c.spikeDamagesOf(t, "Ogro"); len(got) != 1 || got[0] != 2 {
		t.Errorf("damages after leaving = %v, want one of 2 dice: one square inside", got)
	}
}

// A push counts as travel: Thunderwave takes the ogre two squares west, into the spikes.
func TestSpikeGrowthHurtsWhoIsPushedThrough(t *testing.T) {
	t.Parallel()
	c := spikeCave(t)
	c.castSpikes(t, 6, 7) // columns 2 to 10
	c.mustMove(t, c.master, "Ogro", 11, 7)
	c.passTo(t, c.get(t, c.master), "Pensantus")
	c.h.roller.queue(1)
	if _, err := c.castArea(t, c.ana, "Pensantus", thunderwave, slotOfLevel(1), nil, toward(-1, 0)); err != nil {
		t.Fatalf("CastSpell(Thunderwave) error = %v", err)
	}
	if got := c.spikeDamagesOf(t, "Ogro"); len(got) != 1 || got[0] != 4 {
		t.Errorf("damages of the pushed ogre = %v, want one of 4 dice: pushed from 11 to 9, squares 10 and 9", got)
	}
}

// Whoever rolls the damage rolls the dice of every square, and the creature takes them as piercing damage.
func TestSpikeGrowthDamageIsRolledAsPiercing(t *testing.T) {
	t.Parallel()
	c := spikeCave(t)
	c.castSpikes(t, 14, 7)
	c.walkOgre(t, 11, 7)
	var id string
	if err := c.h.pool.QueryRow(t.Context(), `SELECT id FROM pending_damages WHERE target_id = $1 AND attack_key = $2`, c.id(t, "Ogro"), spikeGrowthKey).Scan(&id); err != nil {
		t.Fatalf("no damage waits for the ogre: %v", err)
	}
	c.h.roller.queue(1, 2, 3, 4)
	d := c.mustDamage(t, c.caio, c.get(t, c.master), id, inAppDamage).GetPendingDamage()
	if d.GetAmount() != 10 || d.GetDamageTypeKey() != "damage-type:piercing" {
		t.Errorf("damage = %d %s, want 10 piercing", d.GetAmount(), d.GetDamageTypeKey())
	}
}

// A jump crosses none of the squares under it: the creature that lands inside is not hurt.
func TestSpikeGrowthDoesNotHurtWhoJumpsIn(t *testing.T) {
	t.Parallel()
	c := spikeCave(t)
	c.castSpikes(t, 10, 7)
	z := c.oneZone(t, c.master, spikeGrowthKey)
	if !zoneHas(z, 6, 7) || zoneHas(z, 5, 7) {
		t.Fatalf("zone squares = %v, want (6,7) inside and Toren's (5,7) outside", z.GetSquares())
	}
	c.mustMove(t, c.caio, "Toren", 6, 7, jumpTo(playv1.JumpKind_JUMP_KIND_LONG))
	if got := c.spikeDamagesOf(t, "Toren"); len(got) != 0 {
		t.Errorf("the jump hurt Toren: %v, want none", got)
	}
}
