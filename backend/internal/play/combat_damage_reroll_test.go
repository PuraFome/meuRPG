package play

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The damage dice a table feat lets the character roll again (#573, combat_damage_reroll.go): one use of the
// feat's resource covers one damage roll, every die that shows the limit or lower is rolled once more and the new
// face is used. These tests need the database (MEURPG_TEST_DATABASE_URL).

const rerollUses = "sorte_do_dano"

// rerollEffects are the effects of the usual feat: a pool of two uses and a reroll of a damage die of at most 2 that
// costs one.
func rerollEffects() []*rulesv1.TableEffect {
	return []*rulesv1.TableEffect{
		{Type: "resource", Resource: rerollUses, Max: "2", Recharge: "short_rest"},
		rerollRule(2, rerollUses, "1"),
	}
}

func rerollRule(atMost int32, resource, cost string) *rulesv1.TableEffect {
	return &rulesv1.TableEffect{Type: "reroll", Rolls: []string{"damage"}, AtMost: atMost, Resource: resource, Cost: cost}
}

// rerollFeat makes a table feat with the effects, returning its key.
func (a *armed) rerollFeat(t *testing.T, effects ...*rulesv1.TableEffect) string {
	t.Helper()
	res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
		CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
			NamePt: "Dado Sortudo", DescPt: []string{"Um talento da mesa."}, Effects: effects,
		}},
	}))
	if err != nil {
		t.Fatalf("CreateTableEntry() error = %v", err)
	}
	return res.Msg.GetEntry().GetKey()
}

// rerollTable is a table where Toren, a Fighter, has a feat that rolls damage dice again.
type rerollTable struct {
	*armed
	feat string
}

// newRerollTable gives Toren the usual feat plus the extra effects.
func newRerollTable(t *testing.T, more ...*rulesv1.TableEffect) *rerollTable {
	t.Helper()
	return newRerollTableWith(t, append(rerollEffects(), more...))
}

// newRerollTableWith gives Toren a feat with exactly these effects, on a theatre-less combat with the Capitão Goblin.
func newRerollTableWith(t *testing.T, effects []*rulesv1.TableEffect) *rerollTable {
	t.Helper()
	r := &rerollTable{}
	r.armed = newArmedWith(t, func(a *armed) {
		r.feat = a.rerollFeat(t, effects...)
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 2, ""), abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil, nil)
		a.giveFeat(t, a.toren, r.feat)
	})
	r.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: r.capitao.GetId()}}, npcRolls: []int{3}, reveal: []string{"Capitão Goblin"},
		players: map[string]int32{"Toren": 18},
		at:      map[string][2]int32{"Toren": {3, 3}, "Capitão Goblin": {4, 3}},
	})
	return r
}

func (r *rerollTable) left(t *testing.T) int32 {
	t.Helper()
	left, _ := poolOf(r.vitals(t, r.toren), rerollUses)
	return left
}

// hit makes Toren hit the Capitão Goblin (d20 as given) and rolls the damage in the app with the dice queued.
func (r *rerollTable) hit(t *testing.T, e *playv1.Encounter, d20Face int32, roll func(*playv1.RollDamageRequest), faces ...int) *playv1.PendingDamage {
	t.Helper()
	res := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(d20Face))
	if res.GetPendingDamage() == nil {
		t.Fatalf("the attack = %v, want a hit", res.GetRoll())
	}
	r.h.roller.queue(faces...)
	return r.mustDamage(t, r.caio, e, res.GetPendingDamage().GetId(), roll).GetPendingDamage()
}

// wantFaces fails unless the pending damage's roll shows exactly these faces.
func wantFaces(t *testing.T, p *playv1.PendingDamage, want ...int32) {
	t.Helper()
	got := p.GetRoll().GetFaces()
	if len(got) != len(want) {
		t.Fatalf("faces = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("faces = %v, want %v", got, want)
		}
	}
}

// wantDamage fails unless the pending damage is rolled for this amount.
func (a *armed) wantDamage(t *testing.T, p *playv1.PendingDamage, amount int32) {
	t.Helper()
	if status, got := a.pendingOf(t, p.GetId()); (status != "rolled" && status != "applied") || got != amount {
		t.Errorf("the pending damage = %s for %d, want it rolled (or applied) for %d", status, got, amount)
	}
}

func TestDamageDieOfTwoOrLowerIsRolledAgainAndSpendsAUse(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	if got := r.left(t); got != 2 {
		t.Fatalf("uses at the start = %d, want 2", got)
	}
	e := r.turnOf(t, "Toren")
	dmg := r.hit(t, e, 17, inAppDamage, 1, 6) // the d8 shows 1, the new one 6
	wantFaces(t, dmg, 6)
	r.wantDamage(t, dmg, 9) // 6 and the +3 of Strength: the total follows the new face
	if got := r.left(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

func TestDamageRerollWithNoUsesLeftChangesNothing(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	r.hit(t, r.turnOf(t, "Toren"), 17, inAppDamage, 1, 6) // spends the first use
	r.hit(t, r.turnOf(t, "Toren"), 17, inAppDamage, 2, 7) // the second
	if got := r.left(t); got != 0 {
		t.Fatalf("uses left = %d, want 0", got)
	}
	dmg := r.hit(t, r.turnOf(t, "Toren"), 17, inAppDamage, 1) // no use left: the 1 stays
	wantFaces(t, dmg, 1)
	r.wantDamage(t, dmg, 4)
	if got := r.left(t); got != 0 {
		t.Errorf("uses left = %d, want 0", got)
	}
}

func TestUndoOfTheDamageGivesTheRerollUseBack(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	e := r.turnOf(t, "Toren")
	r.hit(t, e, 17, inAppDamage, 2, 5)
	if got := r.left(t); got != 1 {
		t.Fatalf("uses left after the reroll = %d, want 1", got)
	}
	r.undoLast(t, e) // the damage roll
	if got := r.left(t); got != 2 {
		t.Errorf("uses left after the undo = %d, want 2", got)
	}
}

func TestDamageDieAboveTheLimitIsNotRolledAgainAndSpendsNothing(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	dmg := r.hit(t, r.turnOf(t, "Toren"), 17, inAppDamage, 3) // 3 is above at_most 2
	wantFaces(t, dmg, 3)
	if got := r.left(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// A critical hit doubles the dice: only the ones that show the limit or lower are rolled again, in one use.
func TestDamageRerollOfACriticalHitRollsOnlyTheLowDice(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	dmg := r.hit(t, r.turnOf(t, "Toren"), 20, inAppDamage, 1, 5, 6) // 2d8: 1 and 5, the 1 becomes 6
	wantFaces(t, dmg, 6, 5)
	r.wantDamage(t, dmg, 14)
	if got := r.left(t); got != 1 {
		t.Errorf("uses left = %d, want 1 (one use for the whole roll)", got)
	}
}

// A rule that costs 2 spends 2 and the undo gives 2 back.
func TestDamageRerollThatCostsTwoSpendsTwoAndTheUndoGivesTwoBack(t *testing.T) {
	t.Parallel()
	r := newRerollTableWith(t, []*rulesv1.TableEffect{
		{Type: "resource", Resource: rerollUses, Max: "3", Recharge: "short_rest"},
		rerollRule(2, rerollUses, "2"),
	})
	e := r.turnOf(t, "Toren")
	dmg := r.hit(t, e, 17, inAppDamage, 1, 6)
	wantFaces(t, dmg, 6)
	if got := r.left(t); got != 1 {
		t.Fatalf("uses left = %d, want 1 (3 less 2)", got)
	}
	r.undoLast(t, e)
	if got := r.left(t); got != 3 {
		t.Errorf("uses left after the undo = %d, want 3", got)
	}
}

// A reroll with no resource is free: it rolls again, spends nothing and the undo of the damage still works.
func TestFreeDamageRerollSpendsNothing(t *testing.T) {
	t.Parallel()
	r := newRerollTableWith(t, []*rulesv1.TableEffect{{Type: "reroll", Rolls: []string{"damage"}, AtMost: 2}})
	e := r.turnOf(t, "Toren")
	dmg := r.hit(t, e, 17, inAppDamage, 1, 6)
	wantFaces(t, dmg, 6)
	r.wantDamage(t, dmg, 9)
	r.undoLast(t, e)
	if status, _ := r.pendingOf(t, dmg.GetId()); status != "awaiting_roll" {
		t.Errorf("the pending damage after the undo = %s, want awaiting_roll", status)
	}
}

// A sum typed after rolling real dice has no faces: nothing is rolled again and nothing is spent.
func TestTypedDamageIsNotRolledAgain(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	dmg := r.hit(t, r.turnOf(t, "Toren"), 17, typedDamage(1))
	r.wantDamage(t, dmg, 4)
	if got := r.left(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// Two rules: the first (a limit of 1) finds no die to roll again and keeps its use; the second serves the roll.
func TestDamageRerollOfARuleThatChangesNothingLeavesTheNextOne(t *testing.T) {
	t.Parallel()
	r := newRerollTableWith(t, []*rulesv1.TableEffect{
		{Type: "resource", Resource: rerollUses, Max: "3", Recharge: "short_rest"},
		rerollRule(1, rerollUses, "2"),
		rerollRule(3, rerollUses, "1"),
	})
	dmg := r.hit(t, r.turnOf(t, "Toren"), 17, inAppDamage, 3, 6) // 3 is above the first limit, within the second
	wantFaces(t, dmg, 6)
	if got := r.left(t); got != 2 {
		t.Errorf("uses left = %d, want 2 (only the second rule spent, 1 of 3)", got)
	}
}

// A damage held for a reaction window (here the master's check) is not rerolled, and its replay is not: nothing is
// spent and the dice land as rolled.
func TestDamageHeldForAReactionIsNotRerolled(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	r.setRules(t, enemyReactionsAlways)
	e := r.turnOf(t, "Toren")
	held := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	if !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("the attack = %v, want it held for the master's check", held.GetRoll())
	}
	answerChecks(t, r.armed, e)
	pending := r.firstPending(t)
	r.h.roller.queue(1, 6)
	r.mustDamage(t, r.caio, e, pending, inAppDamage)
	answerChecks(t, r.armed, e) // the damage waits for the master too; the answer lands it
	if status, amount := r.pendingOf(t, pending); (status != "rolled" && status != "applied") || amount != 4 {
		t.Errorf("the pending damage = %s for %d, want rolled for 4 (the 1 stays)", status, amount)
	}
	if got := r.left(t); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// A hit with parts (an extra the feat adds) rolls its dice the same way, keeps the face it replaced in the part's
// rerolls with the feat as source, the total follows, and the undo gives the use back.
func TestDamageRerollOfAHitWithPartsKeepsTheReplacedFace(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t, &rulesv1.TableEffect{Type: "extra_damage", Applies: "any_target", Value: "3", TextPt: "Dano extra."})
	e := r.turnOf(t, "Toren")
	res := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	part := partOf(res.GetPendingDamage().GetParts(), r.feat)
	if part == nil {
		t.Fatalf("the offered parts = %v, want the feat's extra", res.GetPendingDamage().GetParts())
	}
	r.h.roller.queue(2, 5)
	p := r.mustDamage(t, r.caio, e, res.GetPendingDamage().GetId(), marking(part.GetKey())).GetPendingDamage()
	weapon := diceOf(t, p)
	if weapon.GetFaces()[0] != 5 || len(weapon.GetRerolled()) != 1 || weapon.GetRerolled()[0].GetFrom() != 2 || weapon.GetRerolled()[0].GetSourceKey() != r.feat {
		t.Fatalf("the weapon part = %v, want the face 5 after a 2, rerolled by %s", weapon, r.feat)
	}
	r.wantDamage(t, p, 11) // 5, the +3 of Strength and the extra 3
	if got := r.left(t); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
	r.undoLast(t, e)
	if got := r.left(t); got != 2 {
		t.Errorf("uses left after the undo = %d, want 2", got)
	}
}

// diceOf is the part of the roll that rolled dice.
func diceOf(t *testing.T, p *playv1.PendingDamage) *playv1.DamagePartRoll {
	t.Helper()
	for _, pr := range p.GetPartRolls() {
		if len(pr.GetFaces()) > 0 {
			return pr
		}
	}
	t.Fatalf("the hit has no part with dice, want the parts path: %v", p)
	return nil
}

// The extra of another damage type goes through the target's resistance and vulnerability: the totals by type
// follow the new face. The awakened shrub resists piercing and is vulnerable to fire.
func TestDamageRerollKeepsTheTotalsByTypeWithResistance(t *testing.T) {
	t.Parallel()
	var feat string
	a, e, shrub := partsFight(t, func(a *armed) {
		feat = a.rerollFeat(t, append(rerollEffects(), &rulesv1.TableEffect{
			Type: "extra_damage", Applies: "any_target", Value: "3", DamageType: "damage-type:fire", TextPt: "Brasas.",
		})...)
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 2, ""), abilities(16, 12, 14, 10, 8), []string{rapier}, nil, nil)
		a.giveFeat(t, a.toren, feat)
	}, "monster:awakened-shrub")
	hit := a.mustAttack(t, a.caio, e, "Toren", rapier, shrub, d20(15))
	part := partOf(hit.GetPendingDamage().GetParts(), feat)
	if part == nil {
		t.Fatalf("the offered parts = %v, want the feat's extra", hit.GetPendingDamage().GetParts())
	}
	a.h.roller.queue(1, 6) // the d8 shows 1, the new one 6
	p := a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), marking(part.GetKey())).GetPendingDamage()
	if f := diceOf(t, p).GetFaces(); len(f) != 1 || f[0] != 6 {
		t.Fatalf("faces = %v, want [6]", f)
	}
	// Piercing: 6 and 3 of Strength = 9, halved to 4. Fire: 3, doubled to 6. The reroll moved the piercing total.
	a.wantDamage(t, p, 10)
}

// Great Weapon Fighting rolls a die once; the table's reroll does not roll that die again, and a rule whose only
// low die was rolled already changes nothing and spends nothing.
func TestTableRerollDoesNotRollAgainADieGreatWeaponFightingRolled(t *testing.T) {
	t.Parallel()
	a, e, shrub := greatWeaponFight(t, 2)
	hit := a.mustAttack(t, a.caio, e, "Toren", "equipment:greatsword", shrub, d20(15))
	a.h.roller.queue(5, 1, 2) // the dice 5 and 1; Great Weapon Fighting rolls the 1 again: 2, which stays
	p := a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	w := diceOf(t, p)
	if f := w.GetFaces(); len(f) != 2 || f[0] != 5 || f[1] != 2 || len(w.GetRerolled()) != 1 || w.GetRerolled()[0].GetSourceKey() != "" {
		t.Errorf("the weapon part = %v, want faces 5 and 2 with the one reroll of Great Weapon Fighting", w)
	}
	if got, _ := poolOf(a.vitals(t, a.toren), rerollUses); got != 2 {
		t.Errorf("uses left = %d, want 2", got)
	}
}

// With a limit of 3 the table rolls the 3 again, the die Great Weapon Fighting rolled keeps its face, and each reroll
// names its source.
func TestTableRerollAndGreatWeaponFightingRollDifferentDice(t *testing.T) {
	t.Parallel()
	a, e, shrub := greatWeaponFight(t, 3)
	hit := a.mustAttack(t, a.caio, e, "Toren", "equipment:greatsword", shrub, d20(15))
	a.h.roller.queue(3, 1, 2, 6) // 3 and 1; the 1 becomes 2 (Great Weapon Fighting); the 3 becomes 6 (the feat)
	p := a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
	w := diceOf(t, p)
	if f := w.GetFaces(); len(f) != 2 || f[0] != 6 || f[1] != 2 || len(w.GetRerolled()) != 2 {
		t.Fatalf("the weapon part = %v, want faces 6 and 2 and two rerolls", w)
	}
	for _, rr := range w.GetRerolled() {
		want := ""
		if rr.GetIndex() == 0 {
			want = a.rerollFeatKey
		}
		if rr.GetSourceKey() != want {
			t.Errorf("the reroll of die %d names %q, want %q", rr.GetIndex(), rr.GetSourceKey(), want)
		}
	}
	if got, _ := poolOf(a.vitals(t, a.toren), rerollUses); got != 1 {
		t.Errorf("uses left = %d, want 1", got)
	}
}

// greatWeaponFight is Toren with a greatsword, Great Weapon Fighting and the usual feat with this limit, beside a shrub.
func greatWeaponFight(t *testing.T, atMost int32) (*rerollArmed, *playv1.Encounter, string) {
	t.Helper()
	var feat string
	a, e, shrub := partsFight(t, func(a *armed) {
		feat = a.rerollFeat(t, &rulesv1.TableEffect{Type: "resource", Resource: rerollUses, Max: "2", Recharge: "short_rest"}, rerollRule(atMost, rerollUses, "1"))
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 2, ""), abilities(16, 12, 14, 10, 8), []string{"equipment:greatsword"}, nil,
			[]string{"feature:fighter-fighting-style-great-weapon-fighting"})
		a.giveFeat(t, a.toren, feat)
	}, "monster:awakened-shrub")
	return &rerollArmed{armed: a, rerollFeatKey: feat}, e, shrub
}

// rerollArmed is an armed table with the key of the reroll feat Toren has.
type rerollArmed struct {
	*armed
	rerollFeatKey string
}

// A Divine Smite's extra die counts against an undead or a fiend only, and a player reads it as counted whatever the
// target is. The reroll therefore rolls it again against every target: the player reads the same lines and the same
// reroll entries for the undead and for the bandit, and only the total differs, by the extra die.
func TestTheRerollRollsTheSmiteDieAgainEveryTargetAndTheTotalCountsItForTheUndeadOnly(t *testing.T) {
	t.Parallel()
	a, _ := smiteTable(t)
	a.giveFeat(t, a.toren, a.rerollFeat(t, rerollEffects()...))
	smite := func(target string) *playv1.PendingDamage {
		t.Helper()
		res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", battleaxe, target, d20(15))
		if res.GetPendingDamage() == nil {
			t.Fatalf("the attack on %s = %v, want a hit", target, res.GetRoll())
		}
		a.h.roller.queue(1, 5, 6, 1, 7, 4) // weapon, the smite's two dice, the extra die; then the new faces of the 1s
		out := a.mustDamage(t, a.caio, a.get(t, a.caio), res.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
			inAppDamage(r)
			r.ExtrasChosen = true
			r.SelectedExtras = []*playv1.SelectedExtra{{Key: "divine-smite", SlotLevel: 1}}
		})
		return out.GetPendingDamage()
	}
	lines := func(p *playv1.PendingDamage) []string {
		var out []string
		for _, r := range p.GetPartRolls() {
			s := r.GetPartKey()
			for _, f := range r.GetFaces() {
				s += fmt.Sprint(",", f)
			}
			for _, rr := range r.GetRerolled() {
				s += fmt.Sprintf(";%d>%d@%s", rr.GetFrom(), rr.GetTo(), rr.GetSourceKey())
			}
			if !r.GetCounted() {
				s += ";NOT-COUNTED"
			}
			out = append(out, s)
		}
		return out
	}
	undead := smite("Esqueleto")
	a.mustEndTurn(t, a.caio, a.get(t, a.caio))
	a.passTo(t, a.get(t, a.master), "Toren")
	human := smite("Bandido")
	if u, h := lines(undead), lines(human); !slices.Equal(u, h) {
		t.Fatalf("the player reads %v for the undead and %v for the bandit; want the same lines and reroll entries", u, h)
	}
	if l := lines(undead); !slices.ContainsFunc(l, func(s string) bool { return strings.HasPrefix(s, "divine-smite-extra,4;1>4@") }) {
		t.Errorf("the lines = %v, want the extra die rolled again from 1 to 4", l)
	}
	if undead.GetAmount() != human.GetAmount() {
		t.Errorf("the player reads the amounts %d and %d, want the same", undead.GetAmount(), human.GetAmount())
	}
	// The master reads which extra die counted: the rolled-again die is the one that does against the undead only.
	a.mustEndTurn(t, a.caio, a.get(t, a.caio))
	for _, r := range a.log(t, a.master, a.get(t, a.master)).GetRounds() {
		for _, en := range r.GetEntries() {
			for _, pr := range en.GetDamage().GetParts() {
				if want := en.GetTargetLabel() == "Esqueleto"; pr.GetPartKey() == "divine-smite-extra" && pr.GetCounted() != want {
					t.Errorf("the master reads the extra die on %s as counted = %v, want %v", en.GetTargetLabel(), pr.GetCounted(), want)
				}
			}
		}
	}
}
