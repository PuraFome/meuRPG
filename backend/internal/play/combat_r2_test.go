package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// R2-10: an opportunity attack is a melee attack made right before the mover leaves reach, so its
// mode is read with the mover on the square it left, not where the move ended.
func TestR210_AnOpportunityAttackIsReadFromTheSquareTheMoverLeft(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	e := c.oppFight(t)
	if _, err := c.conditions(t, c.master, e, "Toren", []string{"condition:prone"}, true, false); err != nil {
		t.Fatalf("SetCombatantConditions(Toren prone) error = %v", err)
	}
	c.moveOffering(t, "Toren", 4, 6) // three squares from Goblin 1 now, beside it when it left reach
	offers := c.offersOf(t, c.master)
	if len(offers) != 1 {
		t.Fatalf("the master's offers = %v, want one", offers)
	}
	res, err := c.offerAttack(t, c.master, "Goblin 1", sword, "Toren", offers[0].GetId(), func(r *playv1.RollAttackRequest) { r.D20Faces = []int32{4, 17} })
	if err != nil {
		t.Fatalf("the offer's attack error = %v", err)
	}
	if got := res.GetRoll().GetSuggestedMode(); got != playv1.RollMode_ROLL_MODE_ADVANTAGE {
		t.Errorf("the suggested mode = %v (%v), want advantage: a prone target within 1.5 m of a melee attacker", got, res.GetRoll().GetSources())
	}
}

// R2-08: Rage's bonus is for melee weapon attacks made with Strength. A thrown melee weapon (a javelin)
// thrown at range is a ranged attack and gets none; used in melee, it still does.
func TestR208_RageDoesNotAddToAThrownWeaponThrownAtRange(t *testing.T) {
	t.Parallel()
	at := func(col, row int32) playdb.Combatant {
		return playdb.Combatant{ID: "x", GridCol: &col, GridRow: &row}
	}
	javelin := link.Attack{Key: "j", Name: "Azagaia", Weapon: true, Melee: true, Ability: "str", RangeFt: 30, LongRangeFt: 120}
	facts := func(target playdb.Combatant) hitFacts {
		attacker := at(0, 0)
		attacker.ID = "a"
		target.ID = "t"
		return hitFacts{
			attacker: attacker, target: target, attack: javelin,
			sheet:  link.Sheet{Traits: link.Traits{BarbarianLevel: 3}},
			states: map[string][]playdb.CombatantState{"a": {{Kind: stateRage}}},
		}
	}
	rage := func(h hitFacts) bool {
		return slices.ContainsFunc(automaticLines(h.scene()), func(e combat.Extra) bool { return e.Key == combat.AutoRage })
	}
	if rage(facts(at(6, 0))) { // 30 ft away: thrown
		t.Errorf("a javelin thrown from 30 ft got Rage's bonus")
	}
	if !rage(facts(at(1, 0))) { // adjacent: a stab
		t.Errorf("a javelin used in melee got no Rage bonus")
	}
	h := facts(at(6, 0))
	h.opportunity = true // an opportunity attack is melee, whatever the distance it is rolled from
	if !rage(h) {
		t.Errorf("an opportunity attack with a javelin got no Rage bonus")
	}
}

// R2-06: a reserved character (imported, no player yet) cannot fight. Asking for one is a
// failed_precondition that says so, never "character not found", and the default party leaves it out.
func TestR206_AReservedCharacterIsRefusedAsReservedNotMissing(t *testing.T) {
	t.Parallel()
	f := newFight(t)
	if _, err := f.h.pool.Exec(t.Context(), `UPDATE characters SET reserved = true, player_user_id = NULL WHERE id = $1`, f.tor.GetId()); err != nil {
		t.Fatalf("reserve the character: %v", err)
	}
	start := func(p ...*playv1.Participant) (*playv1.StartEncounterResponse, error) {
		res, err := f.master.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
			CampaignId: f.campaignID, IdempotencyKey: newKey(), Name: "Emboscada", Participants: p,
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	_, err := start(&playv1.Participant{CharacterId: f.tor.GetId()}, &playv1.Participant{CharacterId: f.goblin.GetId()})
	wantEncounterBlocked(t, err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CHARACTER_RESERVED)
	// Without a player character in the list, the whole party joins: the reserved one is not in it.
	res, err := start(&playv1.Participant{CharacterId: f.goblin.GetId()})
	if err != nil {
		t.Fatalf("StartEncounter(goblin) error = %v", err)
	}
	for _, c := range res.GetEncounter().GetCombatants() {
		if c.GetLabel() == "Toren" {
			t.Errorf("the reserved Toren joined the default party: %v", labels(res.GetEncounter()))
		}
	}
}
