package play

import (
	"testing"

	"connectrpc.com/connect"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
)

// The table's roll_minimum on the damage dice (#757, damage_floor.go): a die of damage that shows less than the minimum
// counts as the minimum, a die smaller than the minimum counts as its own highest face, every die the app rolled is
// counted whether its part counts toward the total or not, the reroll runs on the face that came up and the floor on the
// die that stays, and the faces stay as they came up. These tests need the database (MEURPG_TEST_DATABASE_URL).

// minimumDamage is a roll_minimum of damage dice.
func minimumDamage(value string, damageType string) *rulesv1.TableEffect {
	return &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"damage"}, Value: value, DamageType: damageType}
}

// newMinimumTable is the reroll table (Toren, a Fighter, against the Capitão Goblin) with the weapon given, and the effects
// in one feat Toren has.
func newMinimumTable(t *testing.T, weapon string, effects ...*rulesv1.TableEffect) *rerollTable {
	t.Helper()
	r := &rerollTable{}
	r.armed = newArmedWith(t, func(a *armed) {
		r.feat = a.rerollFeat(t, effects...)
		a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 2, ""), abilities(16, 13, 14, 10, 8), []string{weapon}, nil, nil)
		a.giveFeat(t, a.toren, r.feat)
	})
	r.start(t, plan{
		npcs: []*playv1.Participant{{CharacterId: r.capitao.GetId()}}, npcRolls: []int{3}, reveal: []string{"Capitão Goblin"},
		players: map[string]int32{"Toren": 18},
		at:      map[string][2]int32{"Toren": {3, 3}, "Capitão Goblin": {4, 3}},
	})
	return r
}

// namedFeat is rerollFeat with the name of the feat, to make two of them.
func (a *armed) namedFeat(t *testing.T, name string, effects ...*rulesv1.TableEffect) string {
	t.Helper()
	res, err := a.master.table.CreateTableEntry(t.Context(), connect.NewRequest(&rulesv1.CreateTableEntryRequest{
		CampaignId: a.campaignID, Body: &rulesv1.CreateTableEntryRequest_TableFeat{TableFeat: &rulesv1.TableFeat{
			NamePt: name, DescPt: []string{"Um talento da mesa."}, Effects: effects,
		}},
	}))
	if err != nil {
		t.Fatalf("CreateTableEntry(%s) error = %v", name, err)
	}
	return res.Msg.GetEntry().GetKey()
}

// strike makes Toren hit with the weapon (d20 as given) and rolls the damage in the app with the dice queued.
func (r *rerollTable) strike(t *testing.T, weapon string, d20Face int32, roll func(*playv1.RollDamageRequest), faces ...int) *playv1.PendingDamage {
	t.Helper()
	e := r.turnOf(t, "Toren")
	res := r.mustAttack(t, r.caio, e, "Toren", weapon, "Capitão Goblin", d20(d20Face))
	if res.GetPendingDamage() == nil {
		t.Fatalf("the attack = %v, want a hit", res.GetRoll())
	}
	r.h.roller.queue(faces...)
	return r.mustDamage(t, r.caio, e, res.GetPendingDamage().GetId(), roll).GetPendingDamage()
}

// wantCounted fails unless the dice are exactly these: index, the face that came up and the number it counted as, all from source.
func wantCountedDice(t *testing.T, what string, got []*playv1.CountedDie, source string, want ...[3]int32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s counted dice = %v, want %v", what, got, want)
	}
	for i, w := range want {
		if got[i].GetIndex() != w[0] || got[i].GetFace() != w[1] || got[i].GetCountedAs() != w[2] || got[i].GetSourceKey() != source {
			t.Fatalf("%s counted dice = %v, want %v from %s", what, got, want, source)
		}
	}
}

// damageLine is the last damage line of the log as u reads it that carries what the test looks at.
func (a *armed) damageLine(t *testing.T, u *user, e *playv1.Encounter) *playv1.CombatLogDamage {
	t.Helper()
	var out *playv1.CombatLogDamage
	for _, en := range logEntries(a.log(t, u, e)) {
		if en.GetDamage().GetRoll() != nil || len(en.GetDamage().GetParts()) > 0 {
			out = en.GetDamage()
		}
	}
	if out == nil {
		t.Fatal("the log has no damage line with dice")
	}
	return out
}

// spellDamageLine is the damage of the last spell-target line of the log as u reads it.
func (a *armed) spellDamageLine(t *testing.T, u *user, e *playv1.Encounter) *playv1.CombatLogDamage {
	t.Helper()
	var out *playv1.CombatLogDamage
	for _, en := range logEntries(a.log(t, u, e)) {
		for _, tg := range en.GetSpell().GetTargets() {
			if tg.GetDamage() != nil {
				out = tg.GetDamage()
			}
		}
	}
	if out == nil {
		t.Fatal("the log has no damage line of a cast")
	}
	return out
}

// A damage with no parts: the d8 shows a 2 and counts as the minimum 4. The faces stay, the total follows what counted,
// the master and the roller read the same in the log, and a die at or above the minimum is never marked.
func TestDamageDieBelowTheMinimumCountsAsTheMinimum(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("4", ""))
	dmg := r.strike(t, battleaxe, 17, inAppDamage, 2)
	wantFaces(t, dmg, 2)
	wantCountedDice(t, "the roll", dmg.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 2, 4})
	if dmg.GetRoll().GetTotal() != 7 {
		t.Errorf("the roll total = %d, want 7 (the 4 it counted as and the +3 of Strength)", dmg.GetRoll().GetTotal())
	}
	r.wantDamage(t, dmg, 7)
	e := r.get(t, r.master)
	for who, u := range map[string]*user{"the master": r.master, "Caio": r.caio} {
		line := r.damageLine(t, u, e)
		wantCountedDice(t, who+"'s log", line.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 2, 4})
		if f := line.GetRoll().GetFaces(); len(f) != 1 || f[0] != 2 {
			t.Errorf("%s reads the faces %v, want [2] as it came up", who, f)
		}
	}
	// A 4 or more is not marked.
	again := r.strike(t, battleaxe, 17, inAppDamage, 6)
	if got := again.GetRoll().GetCountedDice(); len(got) != 0 {
		t.Errorf("a 6 counted dice = %v, want none", got)
	}
	r.wantDamage(t, again, 9)
}

// The minimum never goes past the die: a d4 under the minimum 6 counts as its own highest face, 4.
func TestDamageDieSmallerThanTheMinimumCountsAsItsHighestFace(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, dagger, minimumDamage("6", ""))
	dmg := r.strike(t, dagger, 17, inAppDamage, 2)
	wantCountedDice(t, "the roll", dmg.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 2, 4})
	r.wantDamage(t, dmg, 7) // 4 and the +3 of Strength
	full := r.strike(t, dagger, 17, inAppDamage, 4)
	if got := full.GetRoll().GetCountedDice(); len(got) != 0 {
		t.Errorf("a 4 on a d4 counted dice = %v, want none: it is the highest face already", got)
	}
}

// A critical hit rolls the dice doubled and every one of them is counted.
func TestDamageMinimumCountsTheExtraDiceOfACriticalHit(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("4", ""))
	dmg := r.strike(t, battleaxe, 20, inAppDamage, 1, 6, 3)
	wantFaces(t, dmg, 1, 6)
	wantCountedDice(t, "the roll", dmg.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 1, 4})
	r.wantDamage(t, dmg, 13) // 4 and 6 and the +3 of Strength
}

// The same request sent again gives the stored roll: the same faces, the same counted dice, and no die rolled twice.
func TestDamageMinimumRetryGivesTheSameCountedDice(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("4", ""))
	e := r.turnOf(t, "Toren")
	hit := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	r.h.roller.queue(1, 8)
	before, key := r.h.roller.taken(), newKey()
	send := func() *playv1.PendingDamage {
		req := &playv1.RollDamageRequest{CampaignId: r.campaignID, EncounterId: e.GetId(), PendingDamageId: hit.GetPendingDamage().GetId(), IdempotencyKey: key}
		inAppDamage(req)
		res, err := r.caio.combat.RollDamage(t.Context(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("RollDamage() error = %v", err)
		}
		return res.Msg.GetPendingDamage()
	}
	first, second := send(), send()
	for i, p := range []*playv1.PendingDamage{first, second} {
		wantFaces(t, p, 1)
		wantCountedDice(t, "the roll", p.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 1, 4})
		if p.GetRoll().GetTotal() != 7 {
			t.Errorf("answer %d total = %d, want 7", i+1, p.GetRoll().GetTotal())
		}
	}
	if got := r.h.roller.taken() - before; got != 1 {
		t.Errorf("dice rolled by the two requests = %d, want 1", got)
	}
}

// The undo of the roll and a new roll: the second roll shows its own counted dice (none), not the first one's.
func TestUndoOfTheDamageClearsTheCountedDice(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("4", ""))
	e := r.turnOf(t, "Toren")
	hit := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	id := hit.GetPendingDamage().GetId()
	r.h.roller.queue(1, 6)
	first := r.mustDamage(t, r.caio, e, id, inAppDamage).GetPendingDamage()
	wantCountedDice(t, "the first roll", first.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 1, 4})
	r.undoLast(t, e)
	second := r.mustDamage(t, r.caio, e, id, inAppDamage).GetPendingDamage()
	wantFaces(t, second, 6)
	if got := second.GetRoll().GetCountedDice(); len(got) != 0 {
		t.Errorf("the second roll counted dice = %v, want none", got)
	}
	r.wantDamage(t, second, 9)
}

// A sum typed from physical dice has no faces: nothing is counted.
func TestTypedDamageIsNotCountedByTheMinimum(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("6", ""))
	dmg := r.strike(t, battleaxe, 17, typedDamage(1))
	if got := dmg.GetRoll().GetCountedDice(); len(got) != 0 {
		t.Errorf("a typed damage counted dice = %v, want none", got)
	}
	r.wantDamage(t, dmg, 4)
}

// The reroll runs on the face that came up and the floor on the die that stays: the 1 is rolled again (to a 2) because
// the rule covers a 1, and the 2 then counts as the minimum 4. Floored first, the 1 would count 4 and never be rerolled.
func TestDamageRerollIsFlooredAfterTheReroll(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t, minimumDamage("4", ""))
	dmg := r.hit(t, r.turnOf(t, "Toren"), 17, inAppDamage, 1, 2)
	wantFaces(t, dmg, 2)
	wantCountedDice(t, "the roll", dmg.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 2, 4})
	r.wantDamage(t, dmg, 7)
	if got := r.left(t); got != 1 {
		t.Errorf("uses left = %d, want 1: the 1 was rolled again", got)
	}
}

// A hit with parts: the weapon's d8 is rolled again, then counted, and the part keeps the replaced face and the counted die.
func TestDamageRerollIsFlooredAfterTheRerollInAPart(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t, minimumDamage("4", ""), &rulesv1.TableEffect{Type: "extra_damage", Applies: "any_target", Value: "3", TextPt: "Dano extra."})
	e := r.turnOf(t, "Toren")
	res := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	part := partOf(res.GetPendingDamage().GetParts(), r.feat)
	if part == nil {
		t.Fatalf("the offered parts = %v, want the feat's extra", res.GetPendingDamage().GetParts())
	}
	r.h.roller.queue(1, 2)
	p := r.mustDamage(t, r.caio, e, res.GetPendingDamage().GetId(), marking(part.GetKey())).GetPendingDamage()
	weapon := diceOf(t, p)
	if len(weapon.GetFaces()) != 1 || weapon.GetFaces()[0] != 2 || len(weapon.GetRerolled()) != 1 || weapon.GetRerolled()[0].GetFrom() != 1 {
		t.Fatalf("the weapon part = %v, want the 1 rolled again to a 2", weapon)
	}
	wantCountedDice(t, "the weapon part", weapon.GetCountedDice(), r.feat, [3]int32{0, 2, 4})
	if weapon.GetSum() != 4 {
		t.Errorf("the weapon part sum = %d, want 4", weapon.GetSum())
	}
	r.wantDamage(t, p, 10) // 4, the +3 of Strength and the extra 3
}

// A minimum limited to a damage type counts the dice of that type only: the fire extra is counted, the slashing weapon is not.
func TestDamageMinimumWithADamageTypeCountsOnlyThatType(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("5", fireType),
		&rulesv1.TableEffect{Type: "extra_damage", Applies: "any_target", Dice: "1", DiceSides: 6, DamageType: fireType, TextPt: "Brasas."})
	e := r.turnOf(t, "Toren")
	res := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	part := partOf(res.GetPendingDamage().GetParts(), r.feat)
	if part == nil {
		t.Fatalf("the offered parts = %v, want the feat's extra", res.GetPendingDamage().GetParts())
	}
	r.h.roller.queue(2, 1)
	p := r.mustDamage(t, r.caio, e, res.GetPendingDamage().GetId(), marking(part.GetKey())).GetPendingDamage()
	var weapon, fire *playv1.DamagePartRoll
	for _, pr := range p.GetPartRolls() {
		if pr.GetPartKey() == "weapon" {
			weapon = pr
		} else if len(pr.GetFaces()) > 0 {
			fire = pr
		}
	}
	if weapon == nil || fire == nil {
		t.Fatalf("the part rolls = %v, want the weapon and the fire extra", p.GetPartRolls())
	}
	if len(weapon.GetCountedDice()) != 0 || weapon.GetSum() != 2 {
		t.Errorf("the slashing weapon = %v, want its 2 as it came up", weapon)
	}
	wantCountedDice(t, "the fire extra", fire.GetCountedDice(), r.feat, [3]int32{0, 1, 5})
	r.wantDamage(t, p, 10) // 2, the +3 of Strength and the fire die counted 5
}

// The same minimum without a damage type counts the dice of every part.
func TestDamageMinimumWithoutADamageTypeCountsEveryPart(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("5", ""),
		&rulesv1.TableEffect{Type: "extra_damage", Applies: "any_target", Dice: "1", DiceSides: 6, DamageType: fireType, TextPt: "Brasas."})
	e := r.turnOf(t, "Toren")
	res := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	part := partOf(res.GetPendingDamage().GetParts(), r.feat)
	r.h.roller.queue(2, 1)
	p := r.mustDamage(t, r.caio, e, res.GetPendingDamage().GetId(), marking(part.GetKey())).GetPendingDamage()
	for _, pr := range p.GetPartRolls() {
		if len(pr.GetFaces()) > 0 && len(pr.GetCountedDice()) != 1 {
			t.Errorf("the part %s counted dice = %v, want one", pr.GetPartKey(), pr.GetCountedDice())
		}
	}
	r.wantDamage(t, p, 13) // 5, the +3 of Strength and 5
}

// A damage-type minimum floors the dice of a spell's damage with no parts: Fire Bolt's d10 is fire, so the 2 counts 5,
// and a minimum of another type leaves it.
func TestDamageMinimumOfADamageTypeCountsASpellsDie(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, kind string
		counted    bool
	}{{"the spell's type", fireType, true}, {"another type", "damage-type:cold", false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := fighterTable(t, "")
			feat := a.rerollFeat(t, minimumDamage("5", tc.kind))
			a.giveFeat(t, a.pens, feat)
			e := a.closeFight(t)
			a.passTo(t, e, "Pensantus")
			hit := a.mustAttack(t, a.ana, e, "Pensantus", fireBolt, "Goblin", d20(15))
			a.h.roller.queue(2)
			p := a.mustDamage(t, a.ana, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
			wantFaces(t, p, 2)
			if tc.counted {
				wantCountedDice(t, "the spell", p.GetRoll().GetCountedDice(), feat, [3]int32{0, 2, 5})
				a.wantDamage(t, p, 5)
				return
			}
			if got := p.GetRoll().GetCountedDice(); len(got) != 0 {
				t.Errorf("the spell counted dice = %v, want none", got)
			}
			a.wantDamage(t, p, 2)
		})
	}
}

// The extra die of a Divine Smite counts against an undead or a fiend only, and the minimum counts it all the same: the
// die is floored, the total does not add it against a bandit.
func TestDamageMinimumCountsADieThatDoesNotCountInTheTotal(t *testing.T) {
	t.Parallel()
	a, _ := smiteTable(t)
	feat := a.rerollFeat(t, minimumDamage("5", ""))
	a.giveFeat(t, a.toren, feat)
	res := a.mustAttack(t, a.caio, a.get(t, a.caio), "Toren", battleaxe, "Bandido", d20(15))
	if res.GetPendingDamage() == nil {
		t.Fatalf("the attack = %v, want a hit", res.GetRoll())
	}
	a.h.roller.queue(1, 1, 1, 1) // weapon, the smite's two dice, the extra die
	p := a.mustDamage(t, a.caio, a.get(t, a.caio), res.GetPendingDamage().GetId(), func(r *playv1.RollDamageRequest) {
		inAppDamage(r)
		r.ExtrasChosen = true
		r.SelectedExtras = []*playv1.SelectedExtra{{Key: "divine-smite", SlotLevel: 1}}
	}).GetPendingDamage()
	var extra *playv1.DamagePartRoll
	for _, pr := range p.GetPartRolls() {
		if pr.GetPartKey() == "divine-smite-extra" {
			extra = pr
		}
		if len(pr.GetFaces()) > 0 && len(pr.GetCountedDice()) != len(pr.GetFaces()) {
			t.Errorf("the part %s counted dice = %v for the faces %v, want every die counted 5", pr.GetPartKey(), pr.GetCountedDice(), pr.GetFaces())
		}
	}
	if extra == nil {
		t.Fatalf("the part rolls = %v, want the smite's extra die", p.GetPartRolls())
	}
	wantCountedDice(t, "the extra die", extra.GetCountedDice(), feat, [3]int32{0, 1, 5})
	if extra.GetSum() != 5 {
		t.Errorf("the extra die sum = %d, want 5", extra.GetSum())
	}
	// The bandit is no undead: the total adds the weapon (5, +3 of Strength) and the smite's two dice (5 and 5).
	if status, amount := a.pendingOf(t, p.GetId()); (status != "rolled" && status != "applied") || amount != 18 {
		t.Errorf("the pending damage = %s for %d, want 18 without the extra die", status, amount)
	}
	// The master reads that the extra die did not count; the player reads it as counted, as before.
	counted := func(u *user) bool {
		for _, en := range logEntries(a.log(t, u, a.get(t, a.master))) {
			for _, pr := range en.GetDamage().GetParts() {
				if pr.GetPartKey() == "divine-smite-extra" {
					return pr.GetCounted()
				}
			}
		}
		t.Fatal("no extra die in the log")
		return false
	}
	if counted(a.master) || !counted(a.caio) {
		t.Errorf("the extra die counted: master %v, Caio %v, want false and true", counted(a.master), counted(a.caio))
	}
}

// Two entries with the minimum: the higher wins, and a tie keeps the entry that comes first, every time.
func TestDamageMinimumKeepsTheHigherAndTheEarlierOnATie(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		first, other string
		wantFirst    bool
		counted      int32
	}{{"a tie keeps the first", "4", "4", true, 4}, {"the higher wins", "4", "6", false, 6}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var first, other string
			a := newArmedWith(t, func(a *armed) {
				a.toren = a.caio.heroWith(t, a.campaignID, "Toren", classLevel("class:fighter", 2, ""), abilities(16, 13, 14, 10, 8), []string{battleaxe}, nil, nil)
				first, other = a.namedFeat(t, "Primeiro Piso", minimumDamage(tc.first, "")), a.namedFeat(t, "Segundo Piso", minimumDamage(tc.other, ""))
				a.giveFeat(t, a.toren, first)
				a.giveFeat(t, a.toren, other)
			})
			a.start(t, plan{
				npcs: []*playv1.Participant{{CharacterId: a.capitao.GetId()}}, npcRolls: []int{3}, reveal: []string{"Capitão Goblin"},
				players: map[string]int32{"Toren": 18},
				at:      map[string][2]int32{"Toren": {3, 3}, "Capitão Goblin": {4, 3}},
			})
			e := a.turnOf(t, "Toren")
			hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
			a.h.roller.queue(1)
			p := a.mustDamage(t, a.caio, e, hit.GetPendingDamage().GetId(), inAppDamage).GetPendingDamage()
			want := other
			if tc.wantFirst {
				want = first
			}
			wantCountedDice(t, "the roll", p.GetRoll().GetCountedDice(), want, [3]int32{0, 1, tc.counted})
		})
	}
}

// A damage held for a reaction is counted before the windows decide, and lands as it was kept.
func TestDamageHeldForAReactionIsCounted(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("4", ""))
	r.setRules(t, enemyReactionsAlways)
	e := r.turnOf(t, "Toren")
	held := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	if !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("the attack = %v, want it held for the master's check", held.GetRoll())
	}
	answerChecks(t, r.armed, e)
	pending := r.firstPending(t)
	r.h.roller.queue(1)
	r.mustDamage(t, r.caio, e, pending, inAppDamage)
	answerChecks(t, r.armed, e)
	r.wantDamage(t, &playv1.PendingDamage{Id: pending}, 7)
	line := r.damageLine(t, r.master, r.get(t, r.master))
	wantCountedDice(t, "the master's log", line.GetRoll().GetCountedDice(), r.feat, [3]int32{0, 1, 4})
}

// Both sides of the table's character sheets stay readable: a character with no minimum rolls as before.
func TestDamageWithoutAMinimumIsNotMarked(t *testing.T) {
	t.Parallel()
	r := newRerollTable(t)
	dmg := r.hit(t, r.turnOf(t, "Toren"), 17, inAppDamage, 3)
	if got := dmg.GetRoll().GetCountedDice(); len(got) != 0 {
		t.Errorf("counted dice = %v, want none", got)
	}
}

// A held damage with parts keeps its counted dice through the hold: the windows decide on the counted total and the
// replay lands the part as it was kept.
func TestHeldDamageWithPartsKeepsTheCountedDice(t *testing.T) {
	t.Parallel()
	r := newMinimumTable(t, battleaxe, minimumDamage("4", ""), &rulesv1.TableEffect{Type: "extra_damage", Applies: "any_target", Value: "3", TextPt: "Dano extra."})
	r.setRules(t, enemyReactionsAlways)
	e := r.turnOf(t, "Toren")
	held := r.mustAttack(t, r.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(17))
	if !held.GetRoll().GetHeldForReaction() {
		t.Fatalf("the attack = %v, want it held for the master's check", held.GetRoll())
	}
	answerChecks(t, r.armed, e)
	pending := r.firstPending(t)
	r.h.roller.queue(1)
	r.mustDamage(t, r.caio, e, pending, marking(r.feat))
	answerChecks(t, r.armed, e)
	r.wantDamage(t, &playv1.PendingDamage{Id: pending}, 10) // 4, the +3 of Strength and the extra 3
	for who, u := range map[string]*user{"the master": r.master, "Caio": r.caio} {
		var weapon *playv1.DamagePartRoll
		for _, pr := range r.damageLine(t, u, r.get(t, r.master)).GetParts() {
			if pr.GetPartKey() == "weapon" {
				weapon = pr
			}
		}
		if weapon == nil || weapon.GetSum() != 4 {
			t.Fatalf("%s reads the weapon part %v, want its 1 counted as 4", who, weapon)
		}
		wantCountedDice(t, who+"'s weapon part", weapon.GetCountedDice(), r.feat, [3]int32{0, 1, 4})
	}
}

// The fire of a Hellish Rebuke is damage the app rolls for the warlock: its dice count the warlock's minimum, and the
// fire type decides which minimum covers it. The stored roll keeps the faces and the counted dice, and the same answer
// sent again gives the same.
func TestHellishRebukeFireCountsTheMinimum(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, kind string
		want       int32
	}{{"fire minimum", fireType, 19}, {"cold minimum", "damage-type:cold", 12}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var feat string
			a, e := reactorFight(t, func(t *testing.T, a *armed) *charactersv1.Character {
				t.Helper()
				warlock := pactWarlock(t, a)
				feat = a.rerollFeat(t, minimumDamage("5", tc.kind))
				a.giveFeat(t, warlock, feat)
				return warlock
			})
			w := hurtReator(t, a, e)
			a.mustAnswer(t, a.ana, e, w.GetId(), useWith(func(r *playv1.AnswerReactionRequest) { r.Slot = &playv1.SpellSlot{Level: 2, Pact: true} }))
			save := a.windowOf(t, a.master, playv1.ReactionKind_REACTION_KIND_HELLISH_REBUKE)
			a.h.roller.queue(2, 9, 1) // the fire: 3d10
			key := newKey()
			answer := func() *playv1.HellishRebukeResult {
				done := a.mustAnswer(t, a.master, e, save.GetId(), func(r *playv1.AnswerReactionRequest) {
					typedAnswer(2)(r)
					r.IdempotencyKey = key
				})
				return done.GetResult().GetHellishRebuke()
			}
			first := answer()
			answer() // sent again: the stored answer, no second roll
			if first.GetSaved() || first.GetDamage() != tc.want {
				t.Fatalf("the result = %v, want the failed save and %d fire", first, tc.want)
			}
			var raw []byte
			var faces []int32
			var rows int
			if err := a.h.pool.QueryRow(t.Context(), `SELECT count(*) FROM pending_damages WHERE attack_key = $1`, spellHellishRebuke).Scan(&rows); err != nil || rows != 1 {
				t.Fatalf("fire pending damages = %d (%v), want 1", rows, err)
			}
			if err := a.h.pool.QueryRow(t.Context(), `SELECT counted_dice, faces FROM pending_damages WHERE attack_key = $1`, spellHellishRebuke).Scan(&raw, &faces); err != nil {
				t.Fatalf("read the fire's pending damage: %v", err)
			}
			got := readCountedDice(raw)
			if len(faces) != 3 || faces[0] != 2 || faces[1] != 9 || faces[2] != 1 {
				t.Errorf("faces = %v, want 2, 9, 1 as they came up", faces)
			}
			if tc.kind != fireType {
				if len(got) != 0 {
					t.Errorf("counted dice = %v, want none for another type", got)
				}
				return
			}
			if len(got) != 2 || got[0].Face != 2 || got[0].To != 5 || got[1].Index != 2 || got[1].To != 5 || got[0].SourceKey != feat {
				t.Errorf("counted dice = %v, want the 2 and the 1 counted as 5 by %s", got, feat)
			}
		})
	}
}

// A cast's damage (Sacred Flame, no parts) is rolled for the whole cast and logged by target: the caster's player and the
// master read the counted die with the faces, another player reads no dice.
func TestSpellDamageLogShowsTheCountedDice(t *testing.T) {
	t.Parallel()
	a := fighterTable(t, "")
	feat := a.rerollFeat(t, minimumDamage("6", "damage-type:radiant"))
	a.giveFeat(t, a.bri, feat)
	e := a.closeFight(t)
	a.passTo(t, e, "Brisa")
	flame := a.mustCast(t, a.bia, e, "Brisa", sacredFlame, nil, a.at(t, "Goblin"), noCastRoll)
	a.h.roller.queue(2)
	p := a.mustDamage(t, a.bia, e, flame.GetCast().GetPendingDamages()[0].GetId(), inAppDamage).GetPendingDamage()
	wantCountedDice(t, "the answer", p.GetRoll().GetCountedDice(), feat, [3]int32{0, 2, 6})
	for who, u := range map[string]*user{"the master": a.master, "Bia": a.bia} {
		wantCountedDice(t, who+"'s spell log", a.spellDamageLine(t, u, e).GetRoll().GetCountedDice(), feat, [3]int32{0, 2, 6})
	}
	if got := a.spellDamageLine(t, a.caio, e).GetRoll(); got != nil {
		t.Errorf("Caio reads the dice %v of another player's spell, want none", got)
	}
}
