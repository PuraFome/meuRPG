package play

import (
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// A table maneuver with no die: a grant_action option of the same feature that picks the golpes, which spends one use of
// the shared pool of dice and costs the economy it declares. Pensantus (Fighter 5) picked every golpe of the table.

// featureOf is the option of the feature action the key names, in Pensantus's turn options as his player reads them.
func (m *maneuverTable) featureOf(t *testing.T, e *playv1.Encounter, key string) *rulesv1.ActionOption {
	t.Helper()
	for _, f := range m.mustOptions(t, m.ana, e, "Pensantus").GetOptions().GetFeatureActions() {
		if f.GetAction().GetKey() == key {
			return f
		}
	}
	return nil
}

// useAction takes the feature action as Pensantus's player.
func (m *maneuverTable) useAction(t *testing.T, e *playv1.Encounter, key string) (*playv1.TakeActionResponse, error) {
	t.Helper()
	res, err := m.ana.combat.TakeAction(t.Context(), connect.NewRequest(&playv1.TakeActionRequest{
		CampaignId: m.campaignID, EncounterId: e.GetId(), CombatantId: m.id(t, "Pensantus"), ActionKey: key, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (m *maneuverTable) combatant(t *testing.T) *playv1.Combatant {
	t.Helper()
	return byLabel(t, m.get(t, m.master), "Pensantus")
}

// A golpe that is an action lists with its name and cost, spends one use of the pool the die golpes share, costs the
// bonus action it declares (the action stays), and the log names it for the master and every player.
func TestAPlainManeuverActionSpendsASharedUseAndItsEconomy(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "3")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")

	feint := m.featureOf(t, e, m.feint)
	if feint == nil || feint.GetAction().GetNamePt() != "Finta" || feint.GetAction().GetEconomy() != rulesv1.ActionEconomy_ACTION_ECONOMY_BONUS_ACTION || !feint.GetEnabled() || feint.GetUsesLeft() != 3 {
		t.Fatalf("the option = %v, want an enabled Finta (bonus action) with 3 uses left", feint)
	}
	if stance := m.featureOf(t, e, m.stance); stance == nil || stance.GetAction().GetEconomy() != rulesv1.ActionEconomy_ACTION_ECONOMY_ACTION {
		t.Errorf("the option = %v, want Postura Firme as an action", stance)
	}
	for _, die := range []string{m.heron, m.bull, m.grip, m.guard} { // a golpe that is a die lists no action: the die spends the use
		if got := m.featureOf(t, e, die); got != nil {
			t.Errorf("the die golpe %q lists the action %v", die, got)
		}
	}

	if _, err := m.useAction(t, e, m.feint); err != nil {
		t.Fatalf("TakeAction(Finta) error = %v", err)
	}
	if got := diceLeft(t, m); got != 2 {
		t.Errorf("dice left = %d, want 2", got)
	}
	if c := m.combatant(t); !c.GetBonusActionUsed() || c.GetActionUsed() {
		t.Errorf("Pensantus has the bonus action used = %v and the action used = %v, want the bonus action only", c.GetBonusActionUsed(), c.GetActionUsed())
	}
	for name, u := range map[string]*user{"the master": m.master, "the player": m.ana, "another player": m.caio} {
		line := m.logEntries(t, u, e)[0]
		if line.GetKind() != playv1.CombatLogKind_COMBAT_LOG_KIND_ACTION || line.GetKeyNamePt() != "Finta" || line.GetActorLabel() != "Pensantus" {
			t.Errorf("%s reads the line %v, want Pensantus's action named Finta", name, line)
		}
	}
	// The bonus action is spent: the same golpe is off, with the reason, and no use is spent trying.
	if again := m.featureOf(t, e, m.feint); again.GetEnabled() || again.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_BONUS_ACTION_USED {
		t.Errorf("the option after the use = enabled %v (%v), want off for the bonus action used", again.GetEnabled(), again.GetReason().GetCode())
	}
	if _, err := m.useAction(t, e, m.feint); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("a second Finta in the turn: %v, want failed_precondition", err)
	}
	if got := diceLeft(t, m); got != 2 {
		t.Errorf("dice left = %d after the refusal, want 2", got)
	}
}

// The dice are one pool: a golpe with a die and a golpe that is an action both lower what the other sees.
func TestAPlainManeuverActionAndADieGolpeShareThePool(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "3")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")

	pending := m.hit(t, e) // the attack of the Attack action
	m.h.roller.queue(5, 4)
	m.mustDamage(t, m.ana, e, pending.GetId(), marking(m.heron))
	if got := m.featureOf(t, e, m.feint).GetUsesLeft(); got != 2 {
		t.Fatalf("Finta sees %d uses after a die golpe, want 2", got)
	}
	if _, err := m.useAction(t, e, m.feint); err != nil {
		t.Fatalf("TakeAction(Finta) error = %v", err)
	}
	if got := diceLeft(t, m); got != 1 {
		t.Fatalf("dice left = %d, want 1 (one for the die golpe, one for Finta)", got)
	}
	// The die golpe sees the use the action spent: a second hit offers it as available with one use left, and spending
	// it empties the pool.
	second := m.hit(t, e)
	if heron := partOf(second.GetParts(), m.heron); heron == nil || !heron.GetAvailable() {
		t.Fatalf("the die golpe = %v, want it available with one use left", heron)
	}
	m.h.roller.queue(5, 4)
	m.mustDamage(t, m.ana, e, second.GetId(), marking(m.heron))
	if got := diceLeft(t, m); got != 0 {
		t.Errorf("dice left = %d, want 0", got)
	}
	if got := m.featureOf(t, e, m.feint).GetUsesLeft(); got != 0 {
		t.Errorf("Finta sees %d uses after the die golpe took the last one, want 0", got)
	}
}

// With no use left the action is refused with the reason and spends nothing, not even the economy; a short rest brings
// the dice back.
func TestAPlainManeuverActionIsRefusedWithNoDiceAndAShortRestBringsThemBack(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "1")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	if _, err := m.useAction(t, e, m.stance); err != nil {
		t.Fatalf("TakeAction(Postura Firme) error = %v", err)
	}
	if c := m.combatant(t); !c.GetActionUsed() || c.GetBonusActionUsed() {
		t.Errorf("Pensantus has the action used = %v and the bonus action used = %v, want the action only", c.GetActionUsed(), c.GetBonusActionUsed())
	}
	if off := m.featureOf(t, e, m.feint); off.GetEnabled() || off.GetReason().GetCode() != rulesv1.DisabledReasonCode_DISABLED_REASON_CODE_NO_USES {
		t.Errorf("Finta with the pool empty = enabled %v (%v), want off for NO_USES", off.GetEnabled(), off.GetReason().GetCode())
	}
	_, err := m.useAction(t, e, m.feint)
	wantBlockedBy(t, "Finta with no dice left", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_NO_USES)
	if c := m.combatant(t); c.GetBonusActionUsed() {
		t.Error("the refused Finta spent the bonus action")
	}
	if got := diceLeft(t, m); got != 0 {
		t.Fatalf("dice left = %d, want 0 before the rest", got)
	}
	m.endEncounter(t, e) // a rest waits for the end of the combat
	m.rest(t, playv1.RestKind_REST_KIND_SHORT)
	if got := diceLeft(t, m); got != 1 {
		t.Errorf("dice left after a short rest = %d, want 1", got)
	}
}

// The master's undo gives back the use and the economy of the action.
func TestUndoingAPlainManeuverActionGivesTheUseAndTheEconomyBack(t *testing.T) {
	t.Parallel()
	m := newManeuverTable(t, "3")
	e := m.passTo(t, m.castersFight(t, 1), "Pensantus")
	if _, err := m.useAction(t, e, m.feint); err != nil {
		t.Fatalf("TakeAction(Finta) error = %v", err)
	}
	m.undoLast(t, e)
	if got := diceLeft(t, m); got != 3 {
		t.Errorf("dice left after the undo = %d, want 3", got)
	}
	if c := m.combatant(t); c.GetBonusActionUsed() {
		t.Error("the undo left the bonus action spent")
	}
	if !m.featureOf(t, e, m.feint).GetEnabled() {
		t.Error("Finta is off after the undo")
	}
}
