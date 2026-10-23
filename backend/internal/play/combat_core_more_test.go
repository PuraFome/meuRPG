package play

import (
	"slices"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// More of the core, branch by branch: the creatures of a party that leaves or starts a combat,
// the defaults of an add of monsters, a death confirmed before the combat runs, and the pure
// helpers the handlers lean on.

// TestCoreRemovingAPlayerInSetupTakesItsCreaturesOut: a player's character that leaves a combat
// that has not begun takes its creatures with it, and they stay on its list.
func TestCoreRemovingAPlayerInSetupTakesItsCreaturesOut(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	a.give(t, a.toren, "monster:wolf", "Presa")
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{2},
		players: map[string]int32{"Pensantus": 20, "Sálvia": 15, "Toren": 12}, setup: true,
	})
	if !slices.Contains(labels(e), "Presa") {
		t.Fatalf("the combat has %v, want Presa with Toren", labels(e))
	}
	res, err := a.master.combat.RemoveCombatant(t.Context(), connect.NewRequest(&playv1.RemoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: byLabel(t, e, "Toren").GetId(), IdempotencyKey: newKey(),
	}))
	if err != nil {
		t.Fatalf("RemoveCombatant(Toren) error = %v", err)
	}
	got := labels(res.Msg.GetEncounter())
	if slices.Contains(got, "Toren") || slices.Contains(got, "Presa") {
		t.Errorf("the combat has %v after Toren left, want neither Toren nor Presa", got)
	}
	if n := len(a.mustCreatures(t, a.caio, a.toren)); n != 1 {
		t.Errorf("Toren's creatures = %d, want the gift still his", n)
	}
}

// TestCoreAStartCountsThePartysCreatures: the creatures of the party take room in the 40 of a
// start with monsters, and a start without monsters does not count them.
func TestCoreAStartCountsThePartysCreatures(t *testing.T) {
	t.Parallel()
	a := newSummoners(t)
	a.mustCastSummon(t, a.ana, a.pens, findFamiliar, nil, 0, []string{"monster:owl"}, "Nanquim")
	start := func(monsters int32) (*playv1.StartEncounterResponse, error) {
		res, err := a.master.combat.StartEncounter(t.Context(), connect.NewRequest(&playv1.StartEncounterRequest{
			CampaignId: a.campaignID, IdempotencyKey: newKey(), Name: "Cerco",
			Monsters: []*playv1.MonsterGroup{{CreatureKey: bandit, Count: monsters}},
		}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	// Three heroes and the owl are four combatants: 37 monsters make 41.
	_, err := start(37)
	wantBlockedBy(t, "37 monsters and four of the party", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TOO_MANY_COMBATANTS)
	wantRefusal(t, "37 monsters and four of the party", err, connect.CodeFailedPrecondition, "a combat has at most 40 combatants")
	res, err := start(36)
	if err != nil {
		t.Fatalf("StartEncounter(36 monsters) error = %v", err)
	}
	if n := len(res.GetEncounter().GetCombatants()); n != 40 {
		t.Errorf("the combat has %d combatants, want 40", n)
	}
}

// TestCoreAddMonstersTakesOneWhenNoCountIsGiven freezes the default count and that a monster
// without a name takes its creature's.
func TestCoreAddMonstersTakesOneWhenNoCountIsGiven(t *testing.T) {
	t.Parallel()
	a := newArmed(t)
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 10, "Pensantus": 9, "Brisa": 8}, setup: true,
	})
	res, err := a.addMonsters(t, e, newKey(), func(r *playv1.AddMonstersRequest) { r.Count = 0 })
	if err != nil {
		t.Fatalf("AddMonsters() error = %v", err)
	}
	if len(res.GetCombatantIds()) != 1 {
		t.Fatalf("AddMonsters(count 0) added %d monsters, want 1", len(res.GetCombatantIds()))
	}
	added := coreCombatantByID(t, res.GetEncounter(), res.GetCombatantIds()[0])
	if !added.GetHidden() {
		t.Error("a monster added without saying so starts visible, want hidden like any NPC")
	}
	if added.GetLabel() != "Bandido" {
		t.Errorf("the monster is %q, want the creature's name, Bandido", added.GetLabel())
	}
}

func coreCombatantByID(t *testing.T, e *playv1.Encounter, id string) *playv1.Combatant {
	t.Helper()
	for _, c := range e.GetCombatants() {
		if c.GetId() == id {
			return c
		}
	}
	t.Fatalf("no combatant %s", id)
	return nil
}

// TestCoreConfirmDeathBeforeTheCombatRuns freezes a death confirmed in SETUP: the character is
// defeated and the combat stays in SETUP.
func TestCoreConfirmDeathBeforeTheCombatRuns(t *testing.T) {
	t.Parallel()
	x := newCoreFixture(t)
	id := x.label(t, "Toren")
	if _, err := x.h.pool.Exec(t.Context(), `UPDATE combatants SET death_failures = 3 WHERE id = $1`, id); err != nil {
		t.Fatalf("set the failures: %v", err)
	}
	res, err := x.master.combat.ConfirmDeath(t.Context(), connect.NewRequest(&playv1.ConfirmDeathRequest{
		CampaignId: x.campaignID, EncounterId: x.e.GetId(), CombatantId: id, IdempotencyKey: newKey(),
	}))
	if err != nil {
		t.Fatalf("ConfirmDeath() error = %v", err)
	}
	e := res.Msg.GetEncounter()
	if e.GetStatus() != playv1.EncounterStatus_ENCOUNTER_STATUS_SETUP {
		t.Errorf("the status = %v, want SETUP", e.GetStatus())
	}
	if c := byLabel(t, e, "Toren"); c.GetState() != playv1.CombatantState_COMBATANT_STATE_DEAD {
		t.Errorf("Toren's state = %v, want DEAD", c.GetState())
	}
}

// TestCoreSurpriseSuggestionLeavesTheDefeatedOut freezes that a defeated creature gets no line.
func TestCoreSurpriseSuggestionLeavesTheDefeatedOut(t *testing.T) {
	t.Parallel()
	a := newSneaks(t)
	c := a.arena(t, arenaPlan{setup: true})
	before, err := c.suggestions(t, a.master)
	if err != nil {
		t.Fatalf("GetSurpriseSuggestion() error = %v", err)
	}
	if _, err := a.adjustHP(t, c.refresh(t), "Goblin", damageHP(50)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	after, err := c.suggestions(t, a.master)
	if err != nil {
		t.Fatalf("GetSurpriseSuggestion() error = %v", err)
	}
	if got, want := len(after.GetSuggestions()), len(before.GetSuggestions())-1; got != want {
		t.Errorf("the suggestions after the Goblin fell = %d, want %d", got, want)
	}
	if slices.ContainsFunc(after.GetSuggestions(), func(s *playv1.SurpriseSuggestion) bool { return s.GetCombatantId() == c.id(t, "Goblin") }) {
		t.Error("a defeated creature still gets a suggestion")
	}
}

// TestCoreHelpersOfTheSurpriseAndTheDeath freezes the pure parts: which combatant is on turn when none is
// or the one on turn left, and who is still surprised.
func TestCoreHelpersOfTheSurpriseAndTheDeath(t *testing.T) {
	t.Parallel()
	cs := []playdb.Combatant{{ID: "a", OrderIndex: 0}, {ID: "b", OrderIndex: 1}}
	if got := currentOrder(playdb.Encounter{}, cs); got != -1 {
		t.Errorf("currentOrder(no turn) = %d, want -1", got)
	}
	gone := "gone"
	if got := currentOrder(playdb.Encounter{CurrentCombatantID: &gone}, cs); got != -1 {
		t.Errorf("currentOrder(a combatant that left) = %d, want -1", got)
	}
	b := "b"
	if got := currentOrder(playdb.Encounter{CurrentCombatantID: &b}, cs); got != 1 {
		t.Errorf("currentOrder(b) = %d, want 1", got)
	}
	if stillSurprised(playdb.Encounter{Status: statusEnded}, 0, cs[0]) {
		t.Error("a creature is still surprised in a combat that ended")
	}
	if stillSurprised(playdb.Encounter{Status: statusSetup}, -1, playdb.Combatant{Defeated: true}) {
		t.Error("a defeated creature is still surprised")
	}
	if !stillSurprised(playdb.Encounter{Status: statusSetup}, -1, cs[0]) {
		t.Error("a creature is not surprised before the combat begins")
	}
}
