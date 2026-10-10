package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	mapsv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/maps/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

const relentless = "relentless_endurance"

// raceFight is a fight with Ragna, a player's character of the given race and class and level with
// a greataxe (d12) and a shortbow, next to the goblin; Pensantus and Brisa are there too.
func raceFight(t *testing.T, race, class string, level int32) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Ragna", class, race, level,
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

// TestSavageAttacksAddsOneWeaponDieToAMeleeWeaponCritical: a half-orc's critical hit with a
// melee weapon attack rolls one more of the weapon's damage dice, named "Ataques Selvagens"
// (SRD 5.1, Half-Orc); it stacks with a barbarian's Brutal Critical; an unarmed strike (no
// weapon die), a ranged attack, a plain hit and a human get none.
func TestSavageAttacksAddsOneWeaponDieToAMeleeWeaponCritical(t *testing.T) {
	t.Parallel()
	t.Run("a greataxe critical", func(t *testing.T) {
		t.Parallel()
		a, e := raceFight(t, "race:half-orc", "class:fighter", 1)
		p := a.mustAttack(t, a.caio, e, "Ragna", greataxe, "Goblin", d20(20)).GetPendingDamage()
		if !p.GetCritical() || p.GetDiceCount() != 2 || p.GetDiceSides() != 12 || p.GetExtraDiceCount() != 1 || p.GetExtraDiceNamePt() != "Ataques Selvagens" {
			t.Fatalf("a half-orc's critical = %d dice of d%d, %d extra named %q; want 2 of d12 and 1 extra named Ataques Selvagens",
				p.GetDiceCount(), p.GetDiceSides(), p.GetExtraDiceCount(), p.GetExtraDiceNamePt())
		}
		a.h.roller.queue(7, 11, 4)
		rolled := a.mustDamage(t, a.caio, e, p.GetId(), inAppDamage).GetPendingDamage()
		if got := rolled.GetRoll(); got.GetDiceCount() != 3 || len(got.GetFaces()) != 3 || got.GetFaces()[2] != 4 || rolled.GetExtraDiceNamePt() != "Ataques Selvagens" {
			t.Errorf("the damage roll = %v named %q, want 3 dice with the extra die (4) last", got, rolled.GetExtraDiceNamePt())
		}
		for name, u := range map[string]*user{"master": a.master, "the roller": a.caio} {
			var d *playv1.CombatLogDamage
			for _, en := range a.log(t, u, e).GetRounds()[0].GetEntries() {
				if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetActorLabel() == "Ragna" {
					d = en.GetDamage()
				}
			}
			if d == nil || d.GetExtraDiceCount() != 1 || d.GetExtraDiceNamePt() != "Ataques Selvagens" {
				t.Errorf("%s reads the damage line %v, want one extra die named Ataques Selvagens", name, d)
			}
		}
	})
	t.Run("with Brutal Critical they stack", func(t *testing.T) {
		t.Parallel()
		a, e := raceFight(t, "race:half-orc", "class:barbarian", 9)
		p := a.mustAttack(t, a.caio, e, "Ragna", greataxe, "Goblin", d20(20)).GetPendingDamage()
		if p.GetExtraDiceCount() != 2 || p.GetExtraDiceNamePt() != "Crítico Brutal e Ataques Selvagens" {
			t.Errorf("a half-orc barbarian 9's critical = %d extra named %q, want 2 named both", p.GetExtraDiceCount(), p.GetExtraDiceNamePt())
		}
	})
	t.Run("a plain hit", func(t *testing.T) {
		t.Parallel()
		a, e := raceFight(t, "race:half-orc", "class:fighter", 1)
		if p := a.mustAttack(t, a.caio, e, "Ragna", greataxe, "Goblin", d20(15)).GetPendingDamage(); p.GetCritical() || p.GetExtraDiceCount() != 0 {
			t.Errorf("a plain hit = critical %v with %d extra, want none", p.GetCritical(), p.GetExtraDiceCount())
		}
	})
	t.Run("an unarmed strike", func(t *testing.T) {
		t.Parallel()
		a, e := raceFight(t, "race:half-orc", "class:fighter", 1)
		if p := a.mustAttack(t, a.caio, e, "Ragna", unarmed, "Goblin", d20(20)).GetPendingDamage(); !p.GetCritical() || p.GetExtraDiceCount() != 0 || p.GetExtraDiceNamePt() != "" {
			t.Errorf("an unarmed critical = %d extra named %q, want none (no weapon die)", p.GetExtraDiceCount(), p.GetExtraDiceNamePt())
		}
	})
	t.Run("a ranged attack", func(t *testing.T) {
		t.Parallel()
		a, e := raceFight(t, "race:half-orc", "class:fighter", 1)
		if p := a.mustAttack(t, a.caio, e, "Ragna", shortbowKey, "Goblin", disadvantage(20)).GetPendingDamage(); !p.GetCritical() || p.GetExtraDiceCount() != 0 {
			t.Errorf("a ranged critical = %d extra, want none", p.GetExtraDiceCount())
		}
	})
	t.Run("a human", func(t *testing.T) {
		t.Parallel()
		a, e := raceFight(t, "race:human", "class:fighter", 1)
		if p := a.mustAttack(t, a.caio, e, "Ragna", greataxe, "Goblin", d20(20)).GetPendingDamage(); !p.GetCritical() || p.GetExtraDiceCount() != 0 {
			t.Errorf("a human's critical = %d extra, want none", p.GetExtraDiceCount())
		}
	})
}

// goblinHitsRagna has the goblin hit Ragna for damage points (the master rolls 1d6 + 2, so
// damage is 3 to 8) and returns the pending damage, rolled and waiting for the master.
func goblinHitsRagna(t *testing.T, a *armed, e *playv1.Encounter, d6 int) string {
	t.Helper()
	a.h.roller.queue(15, d6)
	hit := a.mustAttack(t, a.master, e, "Goblin", sword, "Ragna", inAppRoll)
	id := hit.GetPendingDamage().GetId()
	a.mustDamage(t, a.master, e, id, inAppDamage)
	return id
}

// relentlessLine is the log line of the damage the goblin did to Ragna.
func relentlessLine(t *testing.T, a *armed, u *user, e *playv1.Encounter) bool {
	t.Helper()
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_ATTACK && en.GetActorLabel() == "Goblin" && en.GetDamage().GetRelentlessEndurance() {
				return true
			}
		}
	}
	return false
}

// TestRelentlessEnduranceKeepsTheHalfOrcOnItsFeetOnce: damage that would drop a half-orc
// to 0 hit points but does not kill it outright leaves it at 1 and spends the use
// (SRD 5.1, Half-Orc); the log says so; with the use spent, the next one drops it.
func TestRelentlessEnduranceKeepsTheHalfOrcOnItsFeetOnce(t *testing.T) {
	t.Parallel()
	a, e := raceFight(t, "race:half-orc", "class:fighter", 1)
	if got := usedOf(a.vitals(t, a.toren), relentless); got != 0 {
		t.Fatalf("Relentless Endurance used = %d before the fight, want 0 (the half-orc has the resource)", got)
	}
	a.hurt(t, a.toren, 5)
	e = a.passTo(t, e, "Goblin")
	id := goblinHitsRagna(t, a, e, 3) // 3 + 2 = 5: exactly to 0
	if _, err := a.settle(t, a.master, e, id, true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	v := a.vitals(t, a.toren)
	if v.GetHitPointsCurrent() != 1 || usedOf(v, relentless) != 1 {
		t.Fatalf("after the blow Ragna has %d hit points and %d uses spent; want 1 and 1", v.GetHitPointsCurrent(), usedOf(v, relentless))
	}
	if got := combatantState(a.get(t, a.caio), "Ragna"); got == playv1.CombatantState_COMBATANT_STATE_DOWN {
		t.Error("Ragna is Caído after Relentless Endurance")
	}
	for name, u := range map[string]*user{"master": a.master, "Ragna's player": a.caio} {
		if !relentlessLine(t, a, u, e) {
			t.Errorf("%s does not read Relentless Endurance in the damage line", name)
		}
	}

	// The use is spent: the next blow that reaches 0 drops Ragna.
	if _, err := a.endTurn(t, a.master, e, true); err != nil {
		t.Fatalf("EndTurn() error = %v", err)
	}
	e = a.passTo(t, e, "Goblin")
	id = goblinHitsRagna(t, a, e, 3)
	if _, err := a.settle(t, a.master, e, id, true); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	if v := a.vitals(t, a.toren); v.GetHitPointsCurrent() != 0 {
		t.Errorf("after the second blow Ragna has %d hit points, want 0: the use was spent", v.GetHitPointsCurrent())
	}
}

// TestRelentlessEnduranceDoesNotSaveFromInstantDeath: damage left over after reaching 0 that
// equals or exceeds the hit point maximum kills outright (SRD 5.1, Instant Death), so the
// trait does not apply and the use stays.
func TestRelentlessEnduranceDoesNotSaveFromInstantDeath(t *testing.T) {
	t.Parallel()
	a, e := raceFight(t, "race:half-orc", "class:fighter", 1)
	a.hurt(t, a.toren, 5)
	hpMax := a.vitals(t, a.toren).GetHitPointsMax()
	e = a.passTo(t, e, "Goblin")
	id := goblinHitsRagna(t, a, e, 3)
	if _, err := a.master.combat.ApplyPendingDamage(t.Context(), connect.NewRequest(&playv1.ApplyPendingDamageRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: id, IdempotencyKey: newKey(), Amount: proto.Int32(5 + hpMax),
	})); err != nil {
		t.Fatalf("ApplyPendingDamage() error = %v", err)
	}
	v := a.vitals(t, a.toren)
	if v.GetHitPointsCurrent() != 0 || usedOf(v, relentless) != 0 {
		t.Errorf("after massive damage Ragna has %d hit points and %d uses spent; want 0 and 0", v.GetHitPointsCurrent(), usedOf(v, relentless))
	}
	if relentlessLine(t, a, a.master, e) {
		t.Error("the log says Relentless Endurance saved Ragna from massive damage")
	}
}

// TestALongRestGivesBackRelentlessEndurance: the use is back after a long rest.
func TestALongRestGivesBackRelentlessEndurance(t *testing.T) {
	t.Parallel()
	r := newRestTable(t)
	orc := r.ana.hero(t, r.campaign, "Gruk", "class:fighter", "race:half-orc", 1, abilities(16, 13, 14, 10, 10), []string{battleaxe}, nil)
	r.use(t, orc, func(q *playv1.AdjustCharacterVitalsRequest) {
		q.ResourcesUsed = []*playv1.ResourceUsed{{Key: relentless, Used: 1}}
	})
	if got := usedOf(r.vitals(t, r.master, orc), relentless); got != 1 {
		t.Fatalf("Relentless Endurance used = %d, want 1", got)
	}
	if _, err := r.rest(t, playv1.RestKind_REST_KIND_LONG); err != nil {
		t.Fatalf("TakeRest(long) error = %v", err)
	}
	if got := usedOf(r.vitals(t, r.master, orc), relentless); got != 0 {
		t.Errorf("Relentless Endurance used = %d after a long rest, want 0", got)
	}
}

// monkFall is the cave with a pit of 6d6 on (9, 7), Toren a human monk of level 4, and the
// combat begun: Toren walks into the pit (the dice roll 5 each, 30 points) and the fall waits
// for the wizard's Feather Fall.
func monkFall(t *testing.T) (*trapRig, *playv1.Encounter) {
	t.Helper()
	c := newCaveOf(t, 3, []string{burningHands, magicMissileSpell, shieldSpell, featherFallSpell}, func(a *armed) *charactersv1.Character {
		return a.caio.hero(t, a.campaignID, "Toren", "class:monk", "race:human", 4, abilities(12, 16, 14, 14, 8), nil, nil)
	})
	r := newTrapRigOn(t, c)
	r.trap(t, "Fosso", 9, 7, pit("6d6"), func(s *mapsv1.TrapSpec) { s.PresetKey = "trap:simple-pit" })
	e := r.fight(t)
	r.h.roller.queue(5, 5, 5, 5, 5, 5)
	if _, err := r.moveResult(t, r.caio, "Toren", 12, 7); err != nil {
		t.Fatalf("MoveCombatant() error = %v", err)
	}
	return r, e
}

// TestSlowFallTakesFiveTimesTheMonkLevelOffAPitFall: the monk's reaction reduces the fall damage
// by 5 x its monk level (SRD 5.1, Monk 4): 30 points become 10 at level 4, and the reaction is spent.
func TestSlowFallTakesFiveTimesTheMonkLevelOffAPitFall(t *testing.T) {
	t.Parallel()
	r, e := monkFall(t)
	if status, amount := r.fallOf(t, "Toren"); status != "awaiting_reaction" || amount != 30 {
		t.Fatalf("Toren's fall = %s for %d, want it waiting for the windows (30)", status, amount)
	}
	e, err := r.action(t, r.caio, e, "Toren", "feature:slow-fall")
	if err != nil {
		t.Fatalf("TakeAction(Slow Fall) error = %v", err)
	}
	if _, amount := r.fallOf(t, "Toren"); amount != 10 {
		t.Errorf("the fall damage = %d after Slow Fall, want 10 (30 - 5 x 4)", amount)
	}
	if !byLabel(t, e, "Toren").GetReactionUsed() {
		t.Error("Slow Fall did not spend the reaction")
	}
	w := r.windowOf(t, r.ana, playv1.ReactionKind_REACTION_KIND_FEATHER_FALL)
	if w == nil {
		t.Fatal("no Feather Fall window")
	}
	r.mustAnswer(t, r.ana, e, w.GetId(), passAnswer)
	if status, amount := r.fallOf(t, "Toren"); status != "rolled" || amount != 10 {
		t.Errorf("Toren's fall = %s for %d, want it rolled for the master (10)", status, amount)
	}
	if _, err := r.action(t, r.caio, e, "Toren", "feature:slow-fall"); err == nil {
		t.Error("a second Slow Fall was accepted with the reaction spent")
	}
}

// TestSlowFallNeedsAFallToReduce: without fall damage waiting, the button is refused and the
// reaction is kept.
func TestSlowFallNeedsAFallToReduce(t *testing.T) {
	t.Parallel()
	c := newCaveOf(t, 3, []string{burningHands, magicMissileSpell, shieldSpell, featherFallSpell}, func(a *armed) *charactersv1.Character {
		return a.caio.hero(t, a.campaignID, "Toren", "class:monk", "race:human", 4, abilities(12, 16, 14, 14, 8), nil, nil)
	})
	e := c.fight(t)
	if _, err := c.action(t, c.caio, e, "Toren", "feature:slow-fall"); err == nil {
		t.Fatal("Slow Fall was accepted with nothing falling")
	}
	if byLabel(t, c.get(t, c.caio), "Toren").GetReactionUsed() {
		t.Error("the refused Slow Fall spent the reaction")
	}
}

// nimbleGoblin is a fight with Ragna, a goblin fighter of the table's content whose Fuga
// Ágil gives "Desengajar ou Esconder como ação bônus".
func nimbleGoblin(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
			CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableRace{TableRace: &rulesv1.TableRace{
				NamePt: "Goblin (Multiverso)", Size: "Small", SpeedFt: 30, AbilityBonuses: &rulesv1.AbilityScores{Dexterity: 2, Constitution: 1}, DarkvisionFt: 60,
				Languages: []string{"language:common"},
				Traits: []*rulesv1.TableFeature{{
					NamePt: "Fuga Ágil", DescPt: []string{"Você pode usar uma ação bônus para realizar a ação Desengajar ou Esconder."},
					Effects: []*rulesv1.TableEffect{{Type: "grant_action", Economy: "bonus_action", TextPt: "Fuga Ágil: Desengajar ou Esconder como ação bônus."}},
				}},
			}},
		}))
		if err != nil {
			t.Fatalf("CreateTableEntry(race) error = %v", err)
		}
		a.toren = a.caio.hero(t, a.campaignID, "Ragna", "class:fighter", res.Msg.GetEntry().GetKey(), 1,
			&rulesv1.AbilityScores{Strength: 14, Dexterity: 14, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{greataxe}, nil)
		a.pens = a.ana.hero(t, a.campaignID, "Pensantus", "class:wizard", "race:gnome", 1,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 14, Constitution: 12, Intelligence: 16, Wisdom: 10, Charisma: 8}, nil, []string{fireBolt})
		a.bri = a.bia.hero(t, a.campaignID, "Brisa", "class:fighter", "race:human", 2,
			&rulesv1.AbilityScores{Strength: 10, Dexterity: 16, Constitution: 14, Intelligence: 10, Wisdom: 10, Charisma: 8}, []string{rapier}, nil)
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Ragna": 18, "Pensantus": 10, "Brisa": 1}, reveal: []string{"Goblin"},
		at: map[string][2]int32{"Ragna": {3, 3}, "Goblin": {10, 3}, "Pensantus": {10, 5}, "Brisa": {8, 8}},
	})
	return a, e
}

// nimbleKey is the key of the Fuga Ágil action that performs the standard action (":disengage" or ":hide").
func nimbleKey(t *testing.T, a *armed, e *playv1.Encounter, suffix string) string {
	t.Helper()
	for _, f := range a.mustOptions(t, a.caio, e, "Ragna").GetOptions().GetFeatureActions() {
		if k := f.GetAction().GetKey(); strings.HasPrefix(k, "trait:") && strings.HasSuffix(k, suffix) {
			return k
		}
	}
	t.Fatalf("no Fuga Ágil action ending in %q among the feature actions", suffix)
	return ""
}

// TestNimbleEscapeDisengagesOrHidesAsABonusAction: the goblin's Fuga Ágil is offered as two actions,
// Desengajar and Esconder, each a bonus action: the first ends the opportunity attacks for the turn,
// the second is the Hide check (ContestService.Hide) and spends the bonus action.
func TestNimbleEscapeDisengagesOrHidesAsABonusAction(t *testing.T) {
	t.Parallel()
	t.Run("disengage", func(t *testing.T) {
		t.Parallel()
		a, e := nimbleGoblin(t)
		key := nimbleKey(t, a, e, ":disengage")
		nimbleKey(t, a, e, ":hide") // both choices are offered
		if _, err := a.action(t, a.caio, e, "Ragna", key); err != nil {
			t.Fatalf("TakeAction(%s) error = %v", key, err)
		}
		var disengaged, bonus bool
		if err := a.h.pool.QueryRow(t.Context(), `SELECT disengaged, bonus_action_used FROM combatants WHERE id = $1`, a.id(t, "Ragna")).Scan(&disengaged, &bonus); err != nil {
			t.Fatalf("read the combatant: %v", err)
		}
		if !disengaged || !bonus {
			t.Errorf("after Fuga Ágil (Desengajar) disengaged = %v and bonus action used = %v, want both true", disengaged, bonus)
		}
	})
	t.Run("hide", func(t *testing.T) {
		t.Parallel()
		a, e := nimbleGoblin(t)
		key := nimbleKey(t, a, e, ":hide")
		if _, err := a.caio.contests.Hide(t.Context(), connect.NewRequest(&playv1.HideRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Ragna"), ActionKey: key, IdempotencyKey: newKey(), Roll: faces(10),
		})); err != nil {
			t.Fatalf("Hide(%s) error = %v", key, err)
		}
		var bonus, action bool
		if err := a.h.pool.QueryRow(t.Context(), `SELECT bonus_action_used, action_used FROM combatants WHERE id = $1`, a.id(t, "Ragna")).Scan(&bonus, &action); err != nil {
			t.Fatalf("read the combatant: %v", err)
		}
		if !bonus || action {
			t.Errorf("after Fuga Ágil (Esconder) bonus action used = %v and action used = %v, want true and false", bonus, action)
		}
	})
}
