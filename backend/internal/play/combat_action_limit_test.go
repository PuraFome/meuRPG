package play

import (
	"strings"
	"testing"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The limits of a stat block's actions are spent and refused (SRD 5.1, "Recharge", "Limited usage"): a Recharge
// action is spent after one use, an action with uses a day after its uses, one that recharges after a rest
// after one use. The state is kept in creature_spent and shown to the master alone (RN-20, RN-29). These tests
// need the database (MEURPG_TEST_DATABASE_URL).

const (
	spiderWeb         = "monster:giant-spider#web"
	healingTouch      = "monster:deva#healing-touch"
	fetidCloud        = "monster:dretch#fetid-cloud"
	inkCloud          = "monster:octopus#ink-cloud"
	breathSpent       = spentRechargePrefix + fireBreath
	recharging        = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CREATURE_ACTION_RECHARGING
	noUsesLeft        = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CREATURE_ACTION_NO_USES_LEFT
	healingTouchN     = 3
	unavailableAction = playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CREATURE_ACTION_UNAVAILABLE
)

// twoD20 is the roll of an attack made with disadvantage: both dice show the face (a ranged attack on an adjacent
// creature).
func twoD20(face int32) func(*playv1.RollAttackRequest) {
	return func(r *playv1.RollAttackRequest) { r.Roll, r.D20Faces = nil, []int32{face, face} }
}

// limitedOf is the limited action of the creature that has the key, as u reads it.
func (a *armed) limitedOf(t *testing.T, u *user, label, key string) *playv1.LimitedAction {
	t.Helper()
	for _, l := range byLabel(t, a.get(t, u), label).GetLimitedActions() {
		if l.GetActionKey() == key {
			return l
		}
	}
	return nil
}

// ownTurnAgain passes the turns until the creature is on turn again, in the next round.
func (a *armed) ownTurnAgain(t *testing.T, e *playv1.Encounter, label string) *playv1.Encounter {
	t.Helper()
	if _, err := a.endTurn(t, a.master, e, true); err != nil { // discards the damage still open
		t.Fatalf("EndTurn() error = %v", err)
	}
	return a.passTo(t, e, label)
}

// A Recharge action is spent by its use and refused on the creature's next turn; the refusal changes nothing
// (no event, the action not spent, the state as it was) and the master reads it spent.
func TestActionLimit_RechargeActionIsSpentAndRefused(t *testing.T) {
	t.Parallel()
	a, e, hound := houndFight(t, true)
	if l := a.limitedOf(t, a.master, hound, fireBreath); l == nil || !l.GetAvailable() || l.GetUsesLeft() != 1 || l.GetKind() != "recharge" || l.GetUsage() != "Recharge 5-6" {
		t.Fatalf("before the breath the master reads %v, want it available with 1 use", l)
	}
	a.h.roller.queue(18)
	a.mustUseAction(t, a.master, e, hound, fireBreath, []string{"Toren"})
	if got := a.spentOf(t, a.id(t, hound))[breathSpent]; got != 1 {
		t.Fatalf("after the breath %s = %d, want 1", breathSpent, got)
	}
	if l := a.limitedOf(t, a.master, hound, fireBreath); l.GetAvailable() || l.GetUsesLeft() != 0 || l.GetUsesAllowed() != 1 {
		t.Errorf("after the breath the master reads %v, want it spent", l)
	}

	a.h.roller.queue(1) // the Recharge roll at the turn start fails
	e = a.ownTurnAgain(t, e, hound)
	events := a.events(t)
	_, err := a.useAction(t, a.master, e, hound, fireBreath, []string{"Toren"})
	wantBlockedBy(t, "UseCreatureAction", err, recharging)
	if d := byLabel(t, a.get(t, a.master), hound); d.GetActionUsed() || a.events(t) != events || a.spentOf(t, a.id(t, hound))[breathSpent] != 1 {
		t.Errorf("the refusal changed something: action used %v, events %d (was %d), spent %v", d.GetActionUsed(), a.events(t), events, a.spentOf(t, a.id(t, hound)))
	}
}

// An action with uses a day is refused when the uses run out (3 a day), one that recharges after a rest after
// its one use; the master reads the uses left; a hand-made NPC has none of this.
func TestActionLimit_UsesADayAndRestRunOut(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, creature, action string
		uses                   int
		targets                []string
	}{
		{"three a day", "monster:deva", healingTouch, healingTouchN, nil},
		{"one a day with a save", "monster:dretch", fetidCloud, 1, []string{"Toren"}},
		{"rest", "monster:octopus", inkCloud, 1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e, who := creatureFight(t, true, tc.creature, false)
			for i := 1; i <= tc.uses; i++ {
				if len(tc.targets) > 0 {
					a.h.roller.queue(18)
				}
				a.mustUseAction(t, a.master, e, who, tc.action, tc.targets)
				l := a.limitedOf(t, a.master, who, tc.action)
				if got := a.spentOf(t, a.id(t, who))[spentUsePrefix+tc.action]; got != i || int(l.GetUsesLeft()) != tc.uses-i || l.GetAvailable() != (i < tc.uses) {
					t.Fatalf("after use %d of %d: spent %d, master reads %v", i, tc.uses, got, l)
				}
				e = a.ownTurnAgain(t, e, who)
			}
			events := a.events(t)
			_, err := a.useAction(t, a.master, e, who, tc.action, tc.targets)
			wantBlockedBy(t, "UseCreatureAction", err, noUsesLeft)
			if a.events(t) != events || byLabel(t, a.get(t, a.master), who).GetActionUsed() || a.spentOf(t, a.id(t, who))[spentUsePrefix+tc.action] != tc.uses {
				t.Errorf("the refusal changed something: events %d (was %d), spent %v", a.events(t), events, a.spentOf(t, a.id(t, who)))
			}
		})
	}
}

// The attack of a Recharge action (a spider's Web, a ranged attack: a creature of a character has it as an
// attack of its sheet) is spent by its roll and refused on the creature's next turn, and its undo gives the
// use back. The bite beside it is not limited. The player who owns the creature (Toren's) makes these attacks and
// reads the reason of the refusal, as the master does (RN-29).
func TestActionLimit_AttackRechargeActionIsSpentRefusedAndUndone(t *testing.T) {
	t.Parallel()
	c := newCave(t)
	c.give(t, c.toren, "monster:giant-spider", "Teia")
	e := c.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: c.goblins.GetId(), Count: 1}},
		npcRolls: []int{2},
		players:  map[string]int32{"Toren": 18, "Pensantus": 10, "Brisa": 1, "Teia": 15},
		reveal:   []string{"Goblin"},
		at:       map[string][2]int32{"Toren": {6, 7}, "Pensantus": {5, 8}, "Brisa": {4, 7}, "Teia": {7, 7}, "Goblin": {8, 7}},
	})
	e = c.mustEndTurn(t, c.caio, e)
	id, key := c.id(t, "Teia"), spentRechargePrefix+spiderWeb
	c.mustAttack(t, c.caio, e, "Teia", spiderWeb, "Goblin", twoD20(15))
	if got := c.spentOf(t, id)[key]; got != 1 {
		t.Fatalf("after the Web %s = %d, want 1", key, got)
	}
	c.armed.undoLast(t, e)
	if got := c.spentOf(t, id)[key]; got != 0 || byLabel(t, c.get(t, c.master), "Teia").GetActionUsed() {
		t.Fatalf("after the undo %s = %d, want 0 and the action back", key, got)
	}
	c.mustAttack(t, c.caio, e, "Teia", spiderWeb, "Goblin", twoD20(15))
	c.h.roller.queue(1) // the Recharge roll at the turn start fails
	e = c.ownTurnAgain(t, e, "Teia")
	events := c.events(t)
	_, err := c.attack(t, c.caio, e, "Teia", spiderWeb, "Goblin", twoD20(15))
	wantBlockedBy(t, "RollAttack by the owner", err, recharging)
	_, err = c.attack(t, c.master, e, "Teia", spiderWeb, "Goblin", twoD20(15))
	wantBlockedBy(t, "RollAttack by the master", err, recharging)
	if c.events(t) != events || c.spentOf(t, id)[key] != 1 || byLabel(t, c.get(t, c.master), "Teia").GetActionUsed() {
		t.Errorf("the refusal changed something: events %d (was %d), spent %v", c.events(t), events, c.spentOf(t, id))
	}
	if _, err := c.attack(t, c.caio, e, "Teia", "monster:giant-spider#bite", "Goblin", d20(15)); err != nil {
		t.Errorf("the bite, which is not limited, was refused: %v", err)
	}
}

// The undo of a creature's action gives the use back: a Recharge action is available again and an action with
// uses a day has the use back.
func TestActionLimit_UndoGivesTheUseBack(t *testing.T) {
	t.Parallel()
	a, e, hound := houndFight(t, true)
	a.h.roller.queue(18)
	a.mustUseAction(t, a.master, e, hound, fireBreath, []string{"Toren"})
	a.undoLast(t, e)
	if got := a.spentOf(t, a.id(t, hound)); len(got) != 0 {
		t.Errorf("after the undo the hound has spent %v, want nothing", got)
	}
	if l := a.limitedOf(t, a.master, hound, fireBreath); !l.GetAvailable() {
		t.Errorf("after the undo the master reads %v, want it available", l)
	}
	a.h.roller.queue(18)
	a.mustUseAction(t, a.master, e, hound, fireBreath, []string{"Toren"}) // usable again

	b, eb, deva := creatureFight(t, true, "monster:deva", false)
	b.mustUseAction(t, b.master, eb, deva, healingTouch, nil)
	eb = b.ownTurnAgain(t, eb, deva)
	b.mustUseAction(t, b.master, eb, deva, healingTouch, nil)
	b.undoLast(t, eb)
	if got := b.spentOf(t, b.id(t, deva))[spentUsePrefix+healingTouch]; got != 1 {
		t.Errorf("after the undo of the second use: spent %d, want 1", got)
	}
}

// What the creature spent is the master's alone: a player reads no limited actions (not even empty), no key of
// the stat block and no state, in the combat and in the log. The master reads them (the positive control).
func TestRN20_LimitedActionsAreTheMastersAlone(t *testing.T) {
	t.Parallel()
	a, e, hound := houndFight(t, true)
	a.h.roller.queue(18)
	a.mustUseAction(t, a.master, e, hound, fireBreath, []string{"Toren"})
	if l := a.limitedOf(t, a.master, hound, fireBreath); l == nil || l.GetAvailable() {
		t.Fatalf("the master reads %v, want the breath spent", l)
	}
	for who, u := range map[string]*user{"Caio": a.caio, "Ana": a.ana} {
		seen := a.get(t, u)
		if c := byLabel(t, seen, hound); len(c.GetLimitedActions()) != 0 {
			t.Errorf("%s reads %v, want no limited actions", who, c.GetLimitedActions())
		}
		if js := asJSON(t, seen) + asJSON(t, a.log(t, u, e)); strings.Contains(js, "imitedActions") || strings.Contains(js, "recharge") || strings.Contains(js, "fire-breath") {
			t.Errorf("%s: the combat or the log carries the limit:\n%s", who, js)
		}
	}
}

// A hand-made NPC (no stat block) has no limits: its attack spends and shows nothing.
func TestActionLimit_HandMadeNPCIsUnaffected(t *testing.T) {
	t.Parallel()
	a, e, hound := houndFight(t, true)
	e = a.passTo(t, e, "Goblin")
	a.mustAttack(t, a.master, e, "Goblin", "basic:0", "Toren", d20(15))
	if got := a.spentOf(t, a.id(t, "Goblin")); len(got) != 0 {
		t.Errorf("the goblin has spent %v, want nothing", got)
	}
	if c := byLabel(t, a.get(t, a.master), "Goblin"); len(c.GetLimitedActions()) != 0 {
		t.Errorf("the master reads %v on a hand-made NPC, want none", c.GetLimitedActions())
	}
	if len(byLabel(t, a.get(t, a.master), hound).GetLimitedActions()) != 1 {
		t.Error("the hound's breath is not listed (the positive control)")
	}
}

// A spend that does not fit in the creature's 4 KiB is refused as an action it cannot make, and nothing changes.
func TestActionLimit_ASpendThatDoesNotFitIsRefused(t *testing.T) {
	t.Parallel()
	a, e, hound := houndFight(t, true)
	id := a.id(t, hound)
	pad := `{"pad": "` + strings.Repeat("x", 4040) + `"}`
	if err := playdb.New(a.h.pool).SetCombatantCreatureSpent(t.Context(), playdb.SetCombatantCreatureSpentParams{ID: id, CreatureSpent: []byte(pad)}); err != nil {
		t.Fatalf("fill the column: %v", err)
	}
	events := a.events(t)
	a.h.roller.queue(18)
	_, err := a.useAction(t, a.master, e, hound, fireBreath, []string{"Toren"})
	wantBlockedBy(t, "UseCreatureAction", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_CREATURE_ACTION_UNAVAILABLE)
	if d := byLabel(t, a.get(t, a.master), hound); d.GetActionUsed() || a.events(t) != events || a.spentOf(t, id)[breathSpent] != 0 {
		t.Errorf("the refusal changed something: action used %v, events %d (was %d)", d.GetActionUsed(), a.events(t), events)
	}
}

// cutPlan is a stat block whose two attack actions share the name a basic sheet keeps; each is limited (Recharge 5-6) as
// the arguments say, and the second differs in its damage unless same.
func cutPlan(limitFirst, limitSecond, same bool) (*rules.MonsterPlan, link.Attack) {
	long := strings.Repeat("a", maxPlanAttackName)
	dmg := func(sides int) []rules.ActionDamage {
		return []rules.ActionDamage{{Dice: rules.DiceFormula{Count: 2, Sides: sides, Bonus: 3}, TypeKey: "damage-type:fire"}}
	}
	usage := func(on bool) rules.ActionUsage {
		if on {
			return rules.ActionUsage{Kind: rules.UsageRecharge, RechargeMin: 5}
		}
		return rules.ActionUsage{Kind: rules.UsageAtWill}
	}
	second := 8
	if same {
		second = 6
	}
	at := &rules.ActionAttack{Bonus: 7, Melee: true, ReachFt: 10}
	plan := &rules.MonsterPlan{Actions: []rules.ActionPlan{
		{Key: "x#one", NamePT: long + "b", Attack: at, Damage: dmg(6), Usage: usage(limitFirst)},
		{Key: "x#two", NamePT: long + "c", Attack: at, Damage: dmg(second), Usage: usage(limitSecond)},
		{Key: "x#bite", NamePT: "Mordida", Attack: at, Damage: dmg(6)},
		{Key: "x#web", NamePT: "Teia"},
	}}
	sheet := link.Attack{Name: long, ToHit: 7, DiceCount: 2, DiceSides: second, DiceBonus: 3, DamageType: "damage-type:fire", RangeFt: 10}
	return plan, sheet
}

// A name that only one action has is the action's, and an action with no attack roll matches none.
func TestPlanByCutNameFindsTheActionOfTheName(t *testing.T) {
	t.Parallel()
	plan, _ := cutPlan(false, false, false)
	if got, _, ok := planByCutName(plan, link.Attack{Name: "Mordida"}); !ok || got.Key != "x#bite" {
		t.Errorf("a name of one action matched %v, %v, want the bite", got.Key, ok)
	}
	if _, _, ok := planByCutName(plan, link.Attack{Name: "Teia"}); ok {
		t.Error("an action with no attack matched")
	}
}

// Two actions that share the cut name are told apart by the numbers of the sheet's attack, so the limit is taken
// from the right one: the sheet's attack (2d8+3) is the second action, which is the limited one, and it is
// refused once spent; the first action (2d6+3), unlimited, is never refused.
func TestPlanByCutNameTellsTwoActionsApartByTheirNumbers(t *testing.T) {
	t.Parallel()
	plan, sheet := cutPlan(false, true, false)
	got, ambiguous, ok := planByCutName(plan, sheet)
	if !ok || ambiguous || got.Key != "x#two" {
		t.Fatalf("the 2d8+3 attack matched %v, ambiguous %v, ok %v, want x#two", got.Key, ambiguous, ok)
	}
	spent := playdb.Combatant{CreatureSpent: []byte(`{"` + limitKeyOf(got) + `":1}`)}
	wantBlockedBy(t, "the spent action", checkActionLimit(got, spent), recharging)
	sheet.DiceSides = 6
	if got, _, ok = planByCutName(plan, sheet); !ok || got.Key != "x#one" {
		t.Errorf("the 2d6+3 attack matched %v, %v, want x#one", got.Key, ok)
	}
	for _, change := range []func(*link.Attack){
		func(a *link.Attack) { a.ToHit++ }, func(a *link.Attack) { a.DiceBonus++ },
		func(a *link.Attack) { a.DamageType = "damage-type:cold" }, func(a *link.Attack) { a.RangeFt = 0 },
	} {
		_, sheet := cutPlan(false, true, false)
		change(&sheet)
		if got, _, ok := planByCutName(plan, sheet); ok {
			t.Errorf("an attack with other numbers matched %v", got.Key)
		}
	}
}

// Two actions with the same numbers cannot be told apart: when one of them is limited the attack is refused as an
// action the creature cannot make, when none is it is no action (the attack is made as today).
func TestPlanByCutNameWithTheSameNumbers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		first, second bool
		wantAmbiguous bool
	}{
		{"one limited", false, true, true}, {"both limited", true, true, true}, {"none limited", false, false, false},
	} {
		plan, sheet := cutPlan(tc.first, tc.second, true)
		got, ok, err := planOfSheetAttack(plan, sheet)
		if ok {
			t.Errorf("%s: matched %v", tc.name, got.Key)
		}
		if tc.wantAmbiguous {
			wantBlockedBy(t, tc.name, err, unavailableAction)
		} else if err != nil {
			t.Errorf("%s: refused: %v", tc.name, err)
		}
	}
}

// The range a basic sheet writes: 0 for a melee attack at the normal reach, the reach of a longer one, the
// range of a thrown or ranged one.
func TestSheetRangeOfFollowsTheBasicSheet(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		at   rules.ActionAttack
		want int
	}{
		{rules.ActionAttack{Melee: true, ReachFt: 5}, 0},
		{rules.ActionAttack{Melee: true, ReachFt: 10}, 10},
		{rules.ActionAttack{Melee: true, ReachFt: 5, RangeFt: 20, LongRangeFt: 60}, 20},
		{rules.ActionAttack{RangeFt: 80, LongRangeFt: 320}, 80},
		{rules.ActionAttack{RangeFt: 900}, 600},
	} {
		if got := sheetRangeOf(&tc.at); got != tc.want {
			t.Errorf("%+v: range %d, want %d", tc.at, got, tc.want)
		}
	}
}
