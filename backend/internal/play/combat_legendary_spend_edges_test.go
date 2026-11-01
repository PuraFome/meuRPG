package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// The edges of spending legendary actions (SRD 5.1, "Legendary Actions"): what the stream tells, an attack
// that waits for a reaction, a request sent twice, and a move option that cannot be walked. These tests need
// the database (MEURPG_TEST_DATABASE_URL).

// RN-20: a text spend changes nothing a player sees, so the hints of the spend and of its undo go to the
// master alone and the players cannot count them. The master's stream is the positive control.
func TestRN20_ATextSpendAndItsUndoHintTheMasterAlone(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	mw, pw := a.master.watch(t, a.campaignID), a.caio.watch(t, a.campaignID)
	mw.ready(t)
	pw.ready(t)

	a.mustSpendLegendary(t, e, "Draco", "wing-attack")
	a.undoLast(t, e)
	a.master.markCurrentMap(t, a.campaignID, a.mapID, mw, pw)
	players, master := pw.beforeMarker(t), mw.beforeMarker(t)
	if len(master) == 0 {
		t.Fatal("the master's stream got nothing for the spend: the control is missing")
	}
	changed, logs := 0, 0
	for _, ev := range master {
		if ev.GetEncounterChanged() != nil {
			changed++
		}
		if ev.GetCombatLogChanged() != nil {
			logs++
		}
	}
	if changed != 2 || logs != 2 {
		t.Errorf("the master's stream has %d encounter_changed and %d combat_log_changed, want 2 and 2 (the spend and the undo)", changed, logs)
	}
	if len(players) != 0 {
		t.Errorf("Caio's stream got %d hints for a text spend and its undo, want none:\n%s", len(players), asJSONAll(t, players))
	}
}

// dragonProtectionFight is the party of the Protection tests (Toren at 3,3 with Reator next to him) and the
// adult red dragon Draco at 5,3, on Toren's turn: Reator may protect Toren from the dragon's attack.
func dragonProtectionFight(t *testing.T) (*armed, *playv1.Encounter) {
	t.Helper()
	a := newArmedWith(t, func(a *armed) {
		a.toren = a.caio.hero(t, a.campaignID, "Toren", "class:fighter", "race:human", 5, abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil)
		res, err := a.ana.characters.CreateCharacter(t.Context(), connect.NewRequest(&charactersv1.CreateCharacterRequest{
			CampaignId: a.campaignID, Kind: charactersv1.CharacterKind_CHARACTER_KIND_PLAYER, Name: "Reator", Sheet: protector().sheet(),
		}))
		if err != nil {
			t.Fatalf("CreateCharacter(Reator) error = %v", err)
		}
		a.pens = res.Msg.GetCharacter()
		a.bri = a.bia.caster(t, a.campaignID, "Brisa", "class:cleric", "race:human", 3, abilities(10, 16, 14, 16, 8), []string{maceKey}, []string{sacredFlame}, nil, []string{cureWounds})
	})
	e := a.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: a.goblin.GetId()}}, npcRolls: []int{3},
		players: map[string]int32{"Toren": 15, "Reator": 10, "Brisa": 1}, setup: true,
	})
	a.h.roller.queue(20)
	a.mustAddMonsters(t, e, func(r *playv1.AddMonstersRequest) {
		r.CreatureKey, r.Count, r.Name, r.Hidden = adultRedDragon, 1, "Draco", new(false)
	})
	e = a.begin(t, a.get(t, a.master))
	a.castStand(t, map[string][2]int32{"Toren": {3, 3}, "Reator": {4, 4}, "Draco": {5, 3}, "Goblin": {9, 9}, "Brisa": {8, 8}})
	e = a.passTo(t, e, "Toren")
	return a, e
}

// A legendary attack that Reator's Protection holds is not paid for while it waits: nothing is spent before
// the answer, the cost is spent once by the replay the answer releases, and the same request sent again
// afterwards gives the written attack and spends nothing more.
func TestLegendarySpend_AHeldAttackSpendsOnlyWhenTheAnswerReleasesIt(t *testing.T) {
	t.Parallel()
	a, e := dragonProtectionFight(t)
	key := newKey()
	send := func() (*playv1.RollAttackResponse, error) {
		return a.legendaryAttackAs(t, e, "Draco", "tail-attack", dragonTail, "Toren", 15, key)
	}
	held, err := send()
	if err != nil || !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("RollAttack(legendary) = %v, %v; want it held for Reator", held, err)
	}
	w := a.protectionWindow(t)
	if w == nil {
		t.Fatal("Reator reads no Protection window: the control is missing")
	}
	if got, l := a.spentOn(t, "Draco"), a.legendaryOf(t, a.master); got != 0 || l.GetActionsLeft() != 3 {
		t.Errorf("while the attack waits the dragon spent %d and the master reads %v left, want nothing spent", got, l.GetActionsLeft())
	}
	a.mustAnswer(t, a.ana, e, w.GetId(), passAnswer)
	if got, l := a.spentOn(t, "Draco"), a.legendaryOf(t, a.master); got != 1 || l.GetActionsLeft() != 2 {
		t.Errorf("after the answer the dragon spent %d and the master reads %d left, want 1 spent (the Tail Attack) and 2 left", got, l.GetActionsLeft())
	}
	if d := byLabel(t, a.get(t, a.master), "Draco"); d.GetActionUsed() || d.GetReactionUsed() {
		t.Errorf("the dragon's turn economy after the replay: action %v, reaction %v; want untouched", d.GetActionUsed(), d.GetReactionUsed())
	}
	events := a.events(t)
	if _, err := send(); err != nil {
		t.Fatalf("the request again: %v", err)
	}
	if got, n := a.spentOn(t, "Draco"), a.events(t); got != 1 || n != events {
		t.Errorf("the request again left spent = %d and %d events (were 1 and %d), want no second spend", got, n, events)
	}
}

// legendaryAttackAs is legendaryAttack under the idempotency key given.
func (a *armed) legendaryAttackAs(t *testing.T, e *playv1.Encounter, attacker, option, key, target string, face int32, idem string) (*playv1.RollAttackResponse, error) {
	t.Helper()
	return a.attack(t, a.master, e, attacker, key, target, func(r *playv1.RollAttackRequest) {
		d20(face)(r)
		r.LegendaryOption, r.IdempotencyKey = option, idem
	})
}

// A request sent twice with the same idempotency key spends once, writes one event and answers the same:
// the text option, the attack and the move.
func TestLegendarySpend_ARetryOfTheSameKeySpendsOnce(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	text := &playv1.SpendLegendaryActionRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Draco"), OptionKey: "wing-attack", IdempotencyKey: newKey()}
	send := func() *playv1.SpendLegendaryActionResponse {
		res, err := a.master.combat.SpendLegendaryAction(t.Context(), connect.NewRequest(text))
		if err != nil {
			t.Fatalf("SpendLegendaryAction() error = %v", err)
		}
		return res.Msg
	}
	first := send()
	events := a.events(t)
	again := send()
	if got, n := a.spentOn(t, "Draco"), a.events(t); got != 2 || n != events {
		t.Errorf("the text spend twice left spent = %d and %d events (were 2 and %d), want one spend", got, n, events)
	}
	if first.GetEncounter().GetRevision() != again.GetEncounter().GetRevision() {
		t.Errorf("the retry answered revision %d, the first %d", again.GetEncounter().GetRevision(), first.GetEncounter().GetRevision())
	}

	key := newKey()
	for range 2 {
		if _, err := a.legendaryAttackAs(t, e, "Draco", "tail-attack", dragonTail, "Toren", 20, key); err != nil {
			t.Fatalf("RollAttack(legendary) error = %v", err)
		}
	}
	if got := a.spentOn(t, "Draco"); got != 3 {
		t.Errorf("the attack twice under one key left spent = %d, want 3 (2 and one Tail Attack)", got)
	}
}

// The same for the move.
func TestLegendarySpend_ARetryOfTheSameMoveSpendsOnce(t *testing.T) {
	t.Parallel()
	a, e, vamp := vampireFight(t)
	e = a.passTo(t, e, "Toren")
	req := &playv1.MoveCombatantRequest{CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, vamp), IdempotencyKey: newKey(), Col: 10, Row: 8, LegendaryOption: "move"}
	for range 2 {
		if _, err := a.master.combat.MoveCombatant(t.Context(), connect.NewRequest(req)); err != nil {
			t.Fatalf("MoveCombatant(legendary) error = %v", err)
		}
	}
	if got := a.spentOn(t, vamp); got != 1 {
		t.Errorf("the move twice under one key left spent = %d, want 1", got)
	}
}

// A move option of a creature with no square (a combat without a map) cannot be walked: it is offered as text
// and the spend is counted, with the master's line and an undo; the log has no trace for the players.
func TestLegendarySpend_AMoveWithNoSquareIsSpentAsText(t *testing.T) {
	t.Parallel()
	a, e, vamp := creatureFight(t, true, vampireKey, false)
	e = a.passTo(t, e, "Toren")
	if l := byLabel(t, a.get(t, a.master), vamp).GetLegendary(); l == nil || !l.GetUsableNow() || l.GetActionsLeft() != 3 {
		t.Fatalf("the master reads %v for the vampire, want it usable now with 3 left", l)
	}
	a.mustSpendLegendary(t, e, vamp, "move")
	if got := a.spentOn(t, vamp); got != 1 {
		t.Fatalf("spent = %d after the Move as text, want 1", got)
	}
	var line *playv1.CombatLogEntry
	for _, l := range logEntries(a.log(t, a.master, e)) {
		if l.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_LEGENDARY_SPENT {
			line = l
		}
	}
	if line == nil || line.GetLegendarySpent().GetOptionName() != "Move" || line.GetLegendarySpent().GetCost() != 1 {
		t.Errorf("the master's line = %v, want the Move at cost 1", line)
	}
	for who, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		wantNoTrace(t, who, "the log", asJSON(t, a.log(t, u, e)))
	}
	a.undoLast(t, e)
	if got := a.spentOn(t, vamp); got != 0 {
		t.Errorf("after the undo spent = %d, want 0", got)
	}
	// The cost still limits it: three Moves spend the three, the fourth has nothing left.
	for range 3 {
		a.mustSpendLegendary(t, e, vamp, "move")
	}
	_, err := a.spendLegendary(t, a.master, e, vamp, "move")
	wantBlockedBy(t, "a fourth Move as text", err, blockedNoneLeft)
}

// A creature that stands on the map walks its move: as text it is refused, and nothing is spent.
func TestLegendarySpend_AMoveOnAMapIsNotText(t *testing.T) {
	t.Parallel()
	a, e, vamp := vampireFight(t)
	e = a.passTo(t, e, "Toren")
	_, err := a.spendLegendary(t, a.master, e, vamp, "move")
	wantBlockedBy(t, "a Move as text on a map", err, blockedWrongKind)
	if got := a.spentOn(t, vamp); got != 0 {
		t.Errorf("the refusal left spent = %d, want 0", got)
	}
}

// A creature that any condition incapacitates takes no legendary action (SRD 5.1, "Incapacitated"): each of
// the five is refused as LEGENDARY_UNAVAILABLE by the text option, the attack and the master's view, and
// spends nothing. Without the condition the same spend is allowed (the control).
func TestLegendarySpend_AnIncapacitatingConditionRefusesTheSpend(t *testing.T) {
	t.Parallel()
	a, e := onToren(t)
	for _, key := range []string{"condition:incapacitated", "condition:stunned", "condition:paralyzed", "condition:petrified", "condition:unconscious"} {
		var err error
		if e, err = a.conditions(t, a.master, e, "Draco", []string{key}, true, false); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
		_, err = a.spendLegendary(t, a.master, e, "Draco", "detect")
		wantBlockedBy(t, key+": a text spend", err, blockedUnusable)
		_, err = a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Toren", 20)
		wantBlockedBy(t, key+": an attack", err, blockedUnusable)
		if l := a.legendaryOf(t, a.master); l.GetUsableNow() || a.spentOn(t, "Draco") != 0 {
			t.Errorf("%s: usable now = %v, spent = %d; want false and 0", key, l.GetUsableNow(), a.spentOn(t, "Draco"))
		}
		if e, err = a.conditions(t, a.master, e, "Draco", nil, true, false); err != nil {
			t.Fatalf("clear %s: %v", key, err)
		}
	}
	a.mustSpendLegendary(t, e, "Draco", "detect")
}

// While a reaction window waits for the master, no legendary spend goes on: not the text option, not the
// attack, not the move (REACTION_PENDING), and none of them spends. Only the creature's turn and its action
// economy are replaced by the legendary rules; the table is held as for any other action.
func TestLegendarySpend_AnOpenWindowHoldsTheSpend(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, creature, label string
		spend                 func(a *armed, e *playv1.Encounter) error
	}{
		{"text", adultRedDragon, "Draco", func(a *armed, e *playv1.Encounter) error {
			_, err := a.spendLegendary(t, a.master, e, "Draco", "detect")
			return err
		}},
		{"attack", adultRedDragon, "Draco", func(a *armed, e *playv1.Encounter) error {
			_, err := a.legendaryAttack(t, e, "Draco", "tail-attack", dragonTail, "Pensantus", 20)
			return err
		}},
		{"move", vampireKey, "Vlad", func(a *armed, e *playv1.Encounter) error {
			_, err := a.legendaryMove(t, a.master, e, "Vlad", "move", 8, 6)
			return err
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, _ := resistFightWith(t, c.creature, "", c.label)
			a.h.roller.queue(1)
			if _, err := a.holdAt(t, c.label); err != nil {
				t.Fatalf("CastSpell() error = %v", err)
			}
			if a.windowOf(t, a.master, resistKind) == nil {
				t.Fatalf("the master's windows = %v, want the Legendary Resistance window", a.windows(t, a.master))
			}
			e := a.get(t, a.master)
			before := a.events(t)
			wantBlockedBy(t, "a legendary "+c.name+" with a window open", c.spend(a, e), playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_REACTION_PENDING)
			if got := a.spentOn(t, c.label); got != 0 || a.events(t) != before {
				t.Errorf("the refused %s spent %d and wrote %d events, want none", c.name, got, a.events(t)-before)
			}
		})
	}
}
