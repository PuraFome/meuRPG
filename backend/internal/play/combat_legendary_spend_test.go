package play

import (
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Spending the legendary actions (SRD 5.1 "Legendary Actions", RN-20, RN-29): the master spends
// them on another creature's turn, one option at a time, at the option's cost, by three paths with one
// rule (a text option, an attack through RollAttack, a move through MoveCombatant). These tests need the
// database (MEURPG_TEST_DATABASE_URL).

const (
	// dragonTail is the Tail attack of the adult red dragon on the sheet the combat makes of its stat block.
	dragonTail = "basic:2"
	dragonBite = "basic:0"
	vampireKey = "monster:vampire-vampire"
)

var (
	blockedOwnTurn    = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_OWN_TURN
	blockedNoneLeft   = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_NONE_LEFT
	blockedUnknownOpt = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_OPTION_UNKNOWN
	blockedWrongKind  = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_WRONG_KIND
	blockedUnusable   = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_LEGENDARY_UNAVAILABLE
)

// spendLegendary calls SpendLegendaryAction as u.
func (a *armed) spendLegendary(t *testing.T, u *user, e *playv1.Encounter, label, option string) (*playv1.SpendLegendaryActionResponse, error) {
	t.Helper()
	res, err := u.combat.SpendLegendaryAction(t.Context(), connect.NewRequest(&playv1.SpendLegendaryActionRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), OptionKey: option, IdempotencyKey: newKey(),
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (a *armed) mustSpendLegendary(t *testing.T, e *playv1.Encounter, label, option string) *playv1.Encounter {
	t.Helper()
	res, err := a.spendLegendary(t, a.master, e, label, option)
	if err != nil {
		t.Fatalf("SpendLegendaryAction(%s) error = %v", option, err)
	}
	return res.GetEncounter()
}

// legendaryAttack is RollAttack as the master with a legendary option.
func (a *armed) legendaryAttack(t *testing.T, e *playv1.Encounter, attacker, option, key, target string, face int32) (*playv1.RollAttackResponse, error) {
	t.Helper()
	return a.attack(t, a.master, e, attacker, key, target, func(r *playv1.RollAttackRequest) {
		d20(face)(r)
		r.LegendaryOption = option
	})
}

// legendaryMove is MoveCombatant as the master with a legendary option.
func (a *armed) legendaryMove(t *testing.T, u *user, e *playv1.Encounter, label, option string, col, row int32) (*playv1.MoveCombatantResponse, error) {
	t.Helper()
	res, err := u.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, label), IdempotencyKey: newKey(), Col: col, Row: row, LegendaryOption: option,
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

// spentOn is the legendary actions the dragon has spent.
func (a *armed) spentOn(t *testing.T, label string) int {
	t.Helper()
	return a.spentOf(t, a.id(t, label))[spentLegendaryActions]
}

// attacksOf is the attacks the combatant made of its turn's Attack action.
func (a *armed) attacksOf(t *testing.T, label string) int {
	t.Helper()
	var n int
	if err := a.h.pool.QueryRow(t.Context(), `SELECT attacks_made FROM combatants WHERE id = $1`, a.id(t, label)).Scan(&n); err != nil {
		t.Fatalf("read the attacks made: %v", err)
	}
	return n
}

// onToren puts the dragon's first turn behind it: Toren is on turn, and the dragon may spend.
func onToren(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a, e, _ := dragonFight(t)
	e = a.passTo(t, e, "Toren")
	return a, e
}

// Within the budget: 1 and then 2 on a creature with 3, each one counted, the master reading what is left
// and that a spend is usable now; a spend beyond what is left is refused and changes nothing.
func TestLegendarySpend_WithinBudgetAndBeyond(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	if l := a.legendaryOf(t, a.master); !l.GetUsableNow() || l.GetActionsLeft() != 3 {
		t.Fatalf("on Toren's turn the master reads %v, want usable now with 3 left", l)
	}
	a.mustSpendLegendary(t, e, "Draco", "detect")
	if got := a.spentOn(t, "Draco"); got != 1 {
		t.Fatalf("after Detect the dragon spent %d, want 1", got)
	}
	a.mustSpendLegendary(t, e, "Draco", "wing-attack")
	l := a.legendaryOf(t, a.master)
	if got := a.spentOn(t, "Draco"); got != 3 || l.GetActionsLeft() != 0 || !l.GetUsableNow() {
		t.Fatalf("after Detect and Wing Attack the dragon spent %d and the master reads %v, want 3 spent, 0 left, still usable now", got, l)
	}
	for _, o := range l.GetOptions() {
		if o.GetAffordable() {
			t.Errorf("option %s is affordable with nothing left", o.GetKey())
		}
	}
	before := a.events(t)
	_, err := a.spendLegendary(t, a.master, e, "Draco", "detect")
	wantBlockedBy(t, "a spend with nothing left", err, blockedNoneLeft)
	if got, n := a.spentOn(t, "Draco"), a.events(t); got != 3 || n != before {
		t.Errorf("a refused spend left spent = %d and %d events (were 3 and %d)", got, n, before)
	}
}

// A cost of 2 with 1 left is refused, and the 1 left stays.
func TestLegendarySpend_ACostOverWhatIsLeftIsRefused(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	a.mustSpendLegendary(t, e, "Draco", "detect")
	a.mustSpendLegendary(t, e, "Draco", "detect")
	_, err := a.spendLegendary(t, a.master, e, "Draco", "wing-attack")
	wantBlockedBy(t, "Wing Attack (2) with 1 left", err, blockedNoneLeft)
	if got := a.spentOn(t, "Draco"); got != 2 {
		t.Errorf("spent = %d after the refusal, want 2", got)
	}
	a.mustSpendLegendary(t, e, "Draco", "detect") // the 1 left is still there
}

// The creature spends nothing on its own turn, and the reasons that block it are told apart.
func TestLegendarySpend_RefusalsSpendNothing(t *testing.T) {
	t.Parallel()
	a, e, _ := dragonFight(t) // the dragon has the turn
	_, err := a.spendLegendary(t, a.master, e, "Draco", "detect")
	wantBlockedBy(t, "a spend on its own turn", err, blockedOwnTurn)
	if l := a.legendaryOf(t, a.master); l.GetUsableNow() {
		t.Errorf("on its own turn the master reads usable_now = true")
	}
	e = a.passTo(t, e, "Toren")

	_, err = a.spendLegendary(t, a.caio, e, "Draco", "detect")
	wantCode(t, "a player's spend", err, connect.CodePermissionDenied)
	_, err = a.spendLegendary(t, a.master, e, "Draco", "no-such-option")
	wantBlockedBy(t, "an unknown option", err, blockedUnknownOpt)
	_, err = a.spendLegendary(t, a.master, e, "Draco", "tail-attack")
	wantBlockedBy(t, "an attack option as text", err, blockedWrongKind)
	_, err = a.spendLegendary(t, a.master, e, "Goblin", "detect")
	wantBlockedBy(t, "a goblin has none", err, blockedUnusable)

	e, err = a.conditions(t, a.master, e, "Draco", []string{"condition:stunned"}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.spendLegendary(t, a.master, e, "Draco", "detect")
	wantBlockedBy(t, "a stunned dragon", err, blockedUnusable)
	if l := a.legendaryOf(t, a.master); l.GetUsableNow() {
		t.Errorf("a stunned dragon reads usable_now = true")
	}
	e, err = a.conditions(t, a.master, e, "Draco", nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	e, err = a.adjustHP(t, e, "Draco", damageHP(1000))
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.spendLegendary(t, a.master, e, "Draco", "detect")
	wantBlockedBy(t, "a defeated dragon", err, blockedUnusable)
	if got := a.spentOn(t, "Draco"); got != 0 {
		t.Errorf("the refusals left spent = %d, want 0", got)
	}
}

// The actions come back when the dragon's own turn starts, not before.
func TestLegendarySpend_ComeBackAtTheOwnTurn(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	a.mustSpendLegendary(t, e, "Draco", "wing-attack")
	a.mustSpendLegendary(t, e, "Draco", "detect")
	e = a.passTo(t, e, "Pensantus")
	if got := a.spentOn(t, "Draco"); got != 3 {
		t.Fatalf("on Pensantus's turn the dragon spent %d, want 3 still", got)
	}
	e = a.passTo(t, e, "Draco")
	if got, l := a.spentOn(t, "Draco"), a.legendaryOf(t, a.master); got != 0 || l.GetActionsLeft() != 3 || l.GetUsableNow() {
		t.Errorf("at its turn the dragon spent %d and the master reads %v, want 0 spent, 3 left, not usable on its own turn", got, l)
	}
	a.passTo(t, e, "Toren")
	if l := a.legendaryOf(t, a.master); !l.GetUsableNow() {
		t.Errorf("after its turn the dragon is not usable now")
	}
}

// A legendary attack spends the cost and nothing of the dragon's turn: not its action, not its attacks, not
// its reaction. One attack for each spend. The damage is the ordinary pending damage.
func TestLegendarySpend_AttackSpendsTheCostNotTheAction(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	res, err := a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Toren", 20)
	if err != nil {
		t.Fatalf("RollAttack(legendary Tail Attack) error = %v", err)
	}
	if res.GetPendingDamage() == nil || res.GetRoll().GetAttackKey() != dragonTail {
		t.Errorf("a legendary attack answered %v, want the attack with its pending damage", res)
	}
	if got := a.spentOn(t, "Draco"); got != 1 {
		t.Errorf("spent = %d, want 1 (the cost of the Tail Attack)", got)
	}
	d := byLabel(t, a.get(t, a.master), "Draco")
	if d.GetActionUsed() || a.attacksOf(t, "Draco") != 0 || d.GetReactionUsed() {
		t.Errorf("the dragon's turn economy after a legendary attack: action %v, attacks %d, reaction %v; want all untouched", d.GetActionUsed(), a.attacksOf(t, "Draco"), d.GetReactionUsed())
	}
	// One attack for each spend: three attacks cost three, the fourth has nothing left.
	a.mustDamage(t, a.master, e, res.GetPendingDamage().GetId(), typedDamage(5))
	for range 2 {
		if _, err := a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Toren", 2); err != nil {
			t.Fatalf("RollAttack(legendary) error = %v", err)
		}
	}
	_, err = a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Toren", 2)
	wantBlockedBy(t, "a fourth legendary attack", err, blockedNoneLeft)
	if got := a.spentOn(t, "Draco"); got != 3 {
		t.Errorf("spent = %d, want 3", got)
	}
}

// The attack of a legendary option is refused on the dragon's own turn (nothing rolls, nothing is spent), for
// an attack the option is not, for an option of another kind, and for one that does not exist.
func TestLegendarySpend_AttackRefusals(t *testing.T) {
	t.Parallel()
	a, e, _ := dragonFight(t)
	_, err := a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Toren", 20)
	wantBlockedBy(t, "a legendary attack on its own turn", err, blockedOwnTurn)
	d := byLabel(t, a.get(t, a.master), "Draco")
	if d.GetActionUsed() || a.spentOn(t, "Draco") != 0 {
		t.Errorf("the refused attack spent: action %v, legendary %d", d.GetActionUsed(), a.spentOn(t, "Draco"))
	}
	e = a.passTo(t, e, "Toren")
	_, err = a.legendaryAttack(t, e, "Draco", "tail-attack", dragonBite, "Toren", 20)
	wantBlockedBy(t, "the Bite as the Tail Attack", err, blockedWrongKind)
	_, err = a.legendaryAttack(t, e, "Draco", "detect", dragonTail, "Toren", 20)
	wantBlockedBy(t, "a text option as an attack", err, blockedWrongKind)
	_, err = a.legendaryAttack(t, e, "Draco", "nothing", dragonTail, "Toren", 20)
	wantBlockedBy(t, "an unknown option as an attack", err, blockedUnknownOpt)
	// Toren is his own player's and has the turn: only the master-only rule refuses him (a creature
	// without legendary actions would otherwise be LEGENDARY_UNAVAILABLE).
	_, err = a.attack(t, a.caio, e, "Toren", battleaxe, "Draco", func(r *playv1.RollAttackRequest) { d20(10)(r); r.LegendaryOption = "tail-attack" })
	wantCode(t, "a player's legendary attack", err, connect.CodePermissionDenied)
	if got := a.spentOn(t, "Draco"); got != 0 {
		t.Errorf("the refusals left spent = %d, want 0", got)
	}
}

// vampireFight is a combat on a map with the party and a vampire at 10,5 that has the first turn.
func vampireFight(t *testing.T) (*armed, *playv1.Encounter, string) {
	t.Helper()
	return creatureFight(t, false, vampireKey, false)
}

// The Move option walks by the usual rules with a budget of its own: a vampire (30 ft) that has used all of
// its turn's movement still moves its full speed, and its turn's movement is not spent by it. It leaves
// the reach of an enemy and offers no opportunity attack, where the same step on its own turn does.
func TestLegendarySpend_MoveHasABudgetOfItsOwn(t *testing.T) {
	t.Parallel()
	a, e, vamp := vampireFight(t)
	a.put(t, "Toren", 10, 6)                                     // beside the vampire at 10,5
	own, err := a.legendaryMove(t, a.master, e, vamp, "", 16, 5) // 30 ft on its own turn: the whole speed, out of Toren's reach
	if err != nil || !own.GetProvoked() {
		t.Fatalf("the vampire's own move = provoked %v, %v; want it to provoke Toren (the control)", own.GetProvoked(), err)
	}
	for _, o := range own.GetEncounter().GetOpportunityOffers() {
		if _, err := a.master.combat.SkipOpportunity(t.Context(), connect.NewRequest(&playv1.SkipOpportunityRequest{
			CampaignId: a.campaignID, EncounterId: e.GetId(), OpportunityOfferId: o.GetId(), IdempotencyKey: newKey(),
		})); err != nil {
			t.Fatalf("SkipOpportunity() error = %v", err)
		}
	}
	used := a.moveRowOf(t, vamp)
	if used.UsedDft != 300 {
		t.Fatalf("the vampire walked %d dft on its turn, want 300", used.UsedDft)
	}
	e = a.passTo(t, e, "Toren")
	a.put(t, "Toren", 16, 6) // beside the vampire at 16,5 again

	res, err := a.legendaryMove(t, a.master, e, vamp, "move", 10, 5)
	if err != nil {
		t.Fatalf("MoveCombatant(legendary Move, 30 ft) error = %v", err)
	}
	after := a.moveRowOf(t, vamp)
	if after.Col != 10 || after.Row != 5 || after.UsedDft != used.UsedDft || after.LastDft != used.LastDft {
		t.Errorf("after the legendary move the row is %v, want (10, 5) with the turn's movement as it was (%v)", after, used)
	}
	if got := a.spentOn(t, vamp); got != 1 || res.GetProvoked() || len(res.GetEncounter().GetOpportunityOffers()) != 0 {
		t.Errorf("spent = %d, provoked = %v, offers = %v; want 1 and no opportunity offer", got, res.GetProvoked(), res.GetEncounter().GetOpportunityOffers())
	}
	_, err = a.legendaryMove(t, a.master, e, vamp, "move", 17, 5) // 35 ft from (10, 5): more than the speed
	wantBlockedBy(t, "a legendary move beyond the speed", err, reasonTooFar)
	if got := a.spentOn(t, vamp); got != 1 {
		t.Errorf("the refused move left spent = %d, want 1", got)
	}
}

// A move refused for other reasons: its own turn, a text option, a player, a jump.
func TestLegendarySpend_MoveRefusals(t *testing.T) {
	t.Parallel()
	a, e, vamp := vampireFight(t)
	_, err := a.legendaryMove(t, a.master, e, vamp, "move", 10, 6)
	wantBlockedBy(t, "a legendary move on its own turn", err, blockedOwnTurn)
	e = a.passTo(t, e, "Toren")
	_, err = a.legendaryMove(t, a.master, e, vamp, "bite", 10, 6)
	wantBlockedBy(t, "a bite as a move", err, blockedWrongKind)
	_, err = a.legendaryMove(t, a.caio, e, "Toren", "move", 8, 6)
	wantCode(t, "a player's legendary move", err, connect.CodePermissionDenied)
	_, err = a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(&playv1.MoveCombatantRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, vamp), IdempotencyKey: newKey(), Col: 10, Row: 6, LegendaryOption: "move", Forced: true,
	}))
	wantCode(t, "a forced legendary move", err, connect.CodeInvalidArgument)
	if got := a.spentOn(t, vamp); got != 0 {
		t.Errorf("the refusals left spent = %d, want 0", got)
	}
}

// The undo of each of the three gives the actions back, and only the last action can be undone: a turn that
// passed in between closes the chain, so an undo cannot cross the reset at the creature's own turn.
func TestLegendarySpend_UndoGivesTheActionsBack(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	a.mustSpendLegendary(t, e, "Draco", "wing-attack")
	if got := a.spentOn(t, "Draco"); got != 2 {
		t.Fatalf("spent = %d, want 2", got)
	}
	a.undoLast(t, e)
	if got := a.spentOn(t, "Draco"); got != 0 {
		t.Errorf("after the undo of the text spend: spent = %d, want 0", got)
	}

	if _, err := a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Toren", 20); err != nil {
		t.Fatalf("legendary attack: %v", err)
	}
	a.undoLast(t, e)
	d := byLabel(t, a.get(t, a.master), "Draco")
	if got := a.spentOn(t, "Draco"); got != 0 || d.GetActionUsed() || a.attacksOf(t, "Draco") != 0 {
		t.Errorf("after the undo of the attack: spent = %d, action %v, attacks %d; want 0, false, 0", got, d.GetActionUsed(), a.attacksOf(t, "Draco"))
	}

	// A turn that passed closes the chain: the spend cannot be undone, and its reset is not crossed.
	a.mustSpendLegendary(t, e, "Draco", "detect")
	e = a.passTo(t, e, "Pensantus")
	if id := a.log(t, a.master, e).GetUndoableEventId(); id != "" {
		t.Errorf("after the turn passed the master may undo %q, want nothing", id)
	}
}

// The undo of a legendary move gives the actions back and puts the vampire where it stood.
func TestLegendarySpend_UndoOfAMove(t *testing.T) {
	t.Parallel()
	a, e, vamp := vampireFight(t)
	e = a.passTo(t, e, "Toren")
	if _, err := a.legendaryMove(t, a.master, e, vamp, "move", 10, 8); err != nil {
		t.Fatalf("legendary move: %v", err)
	}
	if got := a.spentOn(t, vamp); got != 1 {
		t.Fatalf("spent = %d, want 1", got)
	}
	a.undoLast(t, e)
	if row := a.moveRowOf(t, vamp); row.Col != 10 || row.Row != 5 || a.spentOn(t, vamp) != 0 {
		t.Errorf("after the undo the vampire is at %v with %d spent, want (10, 5) and 0", row, a.spentOn(t, vamp))
	}
}

// The master's line names the creature, the option and the cost; no player's log has it.
func TestLegendarySpend_TheMastersLineAlone(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	a.mustSpendLegendary(t, e, "Draco", "wing-attack")
	var found *playv1.CombatLogEntry
	for _, r := range a.log(t, a.master, e).GetRounds() {
		for _, l := range r.GetEntries() {
			if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_LEGENDARY_SPENT {
				found = l
			}
		}
	}
	if found == nil || found.GetActorLabel() != "Draco" || found.GetLegendarySpent().GetOptionName() != "Wing Attack" || found.GetLegendarySpent().GetCost() != 2 {
		t.Fatalf("the master's line = %v, want Draco, Wing Attack, cost 2", found)
	}
	for who, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		js := asJSON(t, a.log(t, u, e))
		if strings.Contains(js, "LEGENDARY") || strings.Contains(js, "Wing") || strings.Contains(js, "legendary") {
			t.Errorf("%s's log has a trace of the spend:\n%s", who, js)
		}
	}
}

// legendaryTraces are the words no player-visible payload has of a spend: the word, the option names of the
// stat blocks, and the cost phrase.
var legendaryTraces = []string{"legendary", "Legendary", "lendária", "LEGENDARY", "Wing Attack", "wing-attack", "Tail Attack", "tail-attack", "spend", "Spend"}

// wantNoTrace fails when the text has one of the words.
func wantNoTrace(t *testing.T, who, what, text string) {
	t.Helper()
	for _, w := range legendaryTraces {
		if strings.Contains(text, w) {
			t.Errorf("%s: %s has %q:\n%s", who, what, w, text)
		}
	}
}

// RN-20, RN-29: the three spends leave players no trace. A text option is a line and an event for the master
// alone (the positive control: he reads the option, the cost and what is left); an attack and a move are the
// ordinary lines every player reads of any creature, in the log and in the stream, with nothing of how they
// were paid.
func TestRN20_LegendarySpendsLeaveNoTraceForPlayers(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	w := a.caio.watch(t, a.campaignID)
	w.ready(t)

	a.mustSpendLegendary(t, e, "Draco", "wing-attack")
	hit, err := a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Toren", 20)
	if err != nil {
		t.Fatalf("legendary attack: %v", err)
	}
	a.mustDamage(t, a.master, e, hit.GetPendingDamage().GetId(), typedDamage(5))
	a.master.markCurrentMap(t, a.campaignID, a.mapID, w)
	hints := w.beforeMarker(t)
	if len(hints) == 0 {
		t.Fatal("Caio's stream got nothing for the attack: the control is missing")
	}
	wantNoTrace(t, "Caio's stream", "the hints", asJSONAll(t, hints))

	// The log: the master reads the option and its cost; the attack is a line every player reads.
	masterLog := asJSON(t, a.log(t, a.master, e))
	if !strings.Contains(masterLog, "Wing Attack") || !strings.Contains(masterLog, "COMBAT_LOG_KIND_LEGENDARY_SPENT") {
		t.Errorf("the master's log lacks the spend line:\n%s", masterLog)
	}
	for who, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		js := asJSON(t, a.log(t, u, e))
		wantNoTrace(t, who, "the log", js)
		if !strings.Contains(js, "COMBAT_LOG_KIND_ATTACK") {
			t.Errorf("%s's log has no attack line: the control is missing:\n%s", who, js)
		}
		seen := a.get(t, u)
		wantNoTrace(t, who, "the combat", asJSON(t, seen))
		if c := byLabel(t, seen, "Draco"); c.GetLegendary() != nil || c.GetActionUsed() {
			t.Errorf("%s reads the dragon with legendary %v, action used %v", who, c.GetLegendary(), c.GetActionUsed())
		}
	}
	if l := a.legendaryOf(t, a.master); l.GetActionsLeft() != 0 {
		t.Errorf("the master reads %v, want the 3 spent (Wing Attack 2 and Tail Attack 1)", l)
	}
}

// The same for a move: the players read the vampire's step as any other, and the stream and the combat say
// nothing of the spend.
func TestRN20_ALegendaryMoveLeavesNoTraceForPlayers(t *testing.T) {
	t.Parallel()
	a, e, vamp := vampireFight(t)
	e = a.passTo(t, e, "Toren")
	w := a.caio.watch(t, a.campaignID)
	w.ready(t)
	if _, err := a.legendaryMove(t, a.master, e, vamp, "move", 10, 8); err != nil {
		t.Fatalf("legendary move: %v", err)
	}
	a.master.markCurrentMap(t, a.campaignID, a.mapID, w)
	hints := w.beforeMarker(t)
	if len(hints) == 0 {
		t.Fatal("Caio's stream got nothing for the move: the control is missing")
	}
	wantNoTrace(t, "Caio's stream", "the hints", asJSONAll(t, hints))
	for who, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		js := asJSON(t, a.log(t, u, e))
		wantNoTrace(t, who, "the log", js)
		if !strings.Contains(js, "COMBAT_LOG_KIND_MOVED") {
			t.Errorf("%s's log has no move line: the control is missing:\n%s", who, js)
		}
		wantNoTrace(t, who, "the combat", asJSON(t, a.get(t, u)))
	}
	if l := byLabel(t, a.get(t, a.master), vamp).GetLegendary(); l == nil || l.GetActionsLeft() != 2 {
		t.Errorf("the master reads %v for the vampire, want 2 left", l)
	}
}
