package play

import (
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// A character at 0 hit points is unconscious for the attack rules (SRD 5.1, Conditions):
// attacks against it have advantage, and a hit from within 5 feet is a critical hit, which
// is two death save failures. The combatant keeps no condition for it: the vitals say it.
// These tests need the database (MEURPG_TEST_DATABASE_URL).

// downedFight starts the sniper table's fight with the goblin first, next to Brisa, who is
// at 0 hit points; Pensantus stands about 50 ft from her.
func (s *sniperTable) downedFight(t *testing.T, first string) *playv1.Encounter {
	t.Helper()
	npc, players := []int{25}, map[string]int32{"Pensantus": 5, "Brisa": 4, "Toren": 3}
	if first == "Pensantus" {
		npc, players = []int{1}, map[string]int32{"Pensantus": 20, "Brisa": 4, "Toren": 3}
	}
	e := s.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: s.goblin.GetId()}}, npcRolls: npc, reveal: []string{"Goblin"},
		players: players,
		at:      map[string][2]int32{"Pensantus": {2, 10}, "Goblin": {13, 14}, "Brisa": {12, 14}, "Toren": {12, 6}},
	})
	s.correct(t, s.bri, hpIs(0))
	return e
}

func TestUnconsciousTargetWeaponHitFromWithin5FtIsCriticalAndTwoFailures(t *testing.T) {
	t.Parallel()
	s := newSniperTable(t)
	e := s.downedFight(t, "Goblin")

	opts := s.mustOptions(t, s.master, s.get(t, s.master), "Goblin")
	if tg := targetOf(opts, sword, "Brisa"); tg == nil || tg.GetRollMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || !tg.GetCriticalOnHit() {
		t.Errorf("the goblin's option against downed Brisa = %v, want advantage and a critical on a hit", tg)
	}

	s.h.roller.queue(15, 9, 3, 3) // two d20s (advantage), then the damage dice
	hit := s.mustAttack(t, s.master, e, "Goblin", sword, "Brisa", inAppRoll)
	if r := hit.GetRoll(); r.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || !r.GetCriticalOnHit() || !hit.GetPendingDamage().GetCritical() {
		t.Fatalf("the hit = mode %v, critical on hit %v, damage critical %v; want advantage and a critical", r.GetMode(), r.GetCriticalOnHit(), hit.GetPendingDamage().GetCritical())
	}
	id := hit.GetPendingDamage().GetId()
	s.mustDamage(t, s.master, e, id, inAppDamage)
	applied, err := s.settle(t, s.master, e, id, true)
	if err != nil || applied.GetDeathFailuresAdded() != 2 {
		t.Fatalf("the damage at 0 = %v, %v; want two death save failures", applied, err)
	}
}

func TestUnconsciousTargetSpellAttackFromAfarHasAdvantageOnly(t *testing.T) {
	t.Parallel()
	s := newSniperTable(t)
	e := s.downedFight(t, "Pensantus")

	s.h.roller.queue(15, 9, 3)
	hit := s.mustAttack(t, s.ana, e, "Pensantus", fireBolt, "Brisa", inAppRoll)
	if r := hit.GetRoll(); r.GetMode() != playv1.RollMode_ROLL_MODE_ADVANTAGE || r.GetCriticalOnHit() {
		t.Fatalf("the spell attack = mode %v, critical on hit %v; want advantage and no critical from afar", r.GetMode(), r.GetCriticalOnHit())
	}
	id := hit.GetPendingDamage().GetId()
	s.mustDamage(t, s.ana, e, id, inAppDamage)
	applied, err := s.settle(t, s.master, e, id, true)
	if err != nil || applied.GetDeathFailuresAdded() != 1 {
		t.Fatalf("the damage at 0 = %v, %v; want one death save failure", applied, err)
	}
}
