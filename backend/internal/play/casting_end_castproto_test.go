package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What the SpellCast of an answer (castProto) shows to each viewer that reads it: the caster's player and the
// master. The scenarios fill the gaps of testdata/cast_spell.golden, which reads every cast as the caster's
// player: a save rolled with two dice (only the master reads a save of a target he does not own), a target
// with two damages to roll, and a retry whose pending damage an undo took away. They need the database
// (MEURPG_TEST_DATABASE_URL). Run with UPDATE_GOLDEN=1 to write the file again, and read the diff.

const castEndProtoGolden = "testdata/cast_proto.golden"

// castEndIceStormTable is the casters' fight with Pensantus a level 7 wizard who knows Ice Storm (two damage types).
func castEndIceStormTable(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	const iceStorm = "spell:ice-storm"
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5,
			&rulesv1.AbilityScores{Strength: 16, Dexterity: 13, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{battleaxe}, nil)
		a.pens = a.ana.caster(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 7,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt},
			[]string{iceStorm}, []string{iceStorm})
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{maceKey}, []string{sacredFlame}, nil,
			[]string{cureWounds})
	})
	enc := a.castersFight(t, 1)
	return a, enc
}

// castEndHeightened casts a fireball with Heightened Spell on the Goblin as u, with the two d20 of its save queued.
func castEndHeightened(u func(a *armed) *user) func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
	return func(t *testing.T, a *armed, _ *playv1.Encounter) (*playv1.CastSpellResponse, error) {
		t.Helper()
		a.castStand(t, map[string][2]int32{"Nael": {4, 9}, "Tavo": {9, 3}, "Orla": {10, 3}, "Goblin": {4, 4}, "Capitão Goblin": {9, 9}})
		a.h.roller.queue(18, 2)
		return a.castWith(t, u(a), a.get(t, a.master), "Nael", fireballSpell, slotOfLevel(3), a.at(t, "Goblin"),
			&playv1.MetamagicChoice{Key: rules.MetamagicHeightened, HeightenedId: a.id(t, "Goblin")})
	}
}

// castEndIceStorm casts Ice Storm at the Goblin as u; the goblin fails its save.
func castEndIceStorm(u func(a *armed) *user) func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
	return func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
		t.Helper()
		a.h.roller.queue(1)
		return a.cast(t, u(a), e, "Pensantus", "spell:ice-storm", slotOfLevel(4), a.at(t, "Goblin"), noCastRoll)
	}
}

func castEndAsMaster(a *armed) *user { return a.master }

// castEndProtoScenarios are the scenarios of the golden.
func castEndProtoScenarios() []goldenScenario {
	heightened := sorcererTable(rules.MetamagicHeightened, rules.MetamagicSubtle)
	return []goldenScenario{
		castSteps("a heightened spell read by the caster", heightened, castEndHeightened(asAna)),
		castSteps("a heightened spell read by the master", heightened, castEndHeightened(castEndAsMaster)),
		castSteps("two damages read by the caster", castEndIceStormTable, castEndIceStorm(asAna)),
		castSteps("two damages read by the master", castEndIceStormTable, castEndIceStorm(castEndAsMaster)),
		castSteps("a retry after the pending damage was discarded", castersTable(1), func(t *testing.T, a *armed, e *playv1.Encounter) (*playv1.CastSpellResponse, error) {
			t.Helper()
			key := newKey()
			targets := []*playv1.SpellTarget{darts(a, t, "Goblin", 3)}
			first, err := a.castKey(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), targets, noCastRoll, key)
			if err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			if _, err := a.settle(t, a.master, e, first.GetCast().GetPendingDamages()[0].GetId(), false); err != nil {
				t.Fatalf("DiscardPendingDamage() error = %v", err)
			}
			return a.castKey(t, a.ana, e, "Pensantus", magicMissileSpell, slotOfLevel(1), targets, noCastRoll, key)
		}),
	}
}

// TestCastEndProtoIsFrozen runs every scenario on a fight of its own and compares what each viewer reads with
// the golden.
func TestCastEndProtoIsFrozen(t *testing.T) {
	t.Parallel()
	runGolden(t, castEndProtoGolden, castEndProtoScenarios())
}
