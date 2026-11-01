package play

import (
	"testing"

	"google.golang.org/protobuf/proto"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The table's roll_minimum on the held d20 rolls (#757).
// An attack, a saving throw and a death save count a d20 below the minimum as the minimum.
// Every rule that reads the die sees that number.
// The faces stay as they came up, and the roll says what it counted as and which feature did it.
// These tests need the database (MEURPG_TEST_DATABASE_URL), but the first ones.

// Reliable Talent and the table's minimums compete for the floor of a roll: the highest wins, a tie keeps Reliable Talent,
// a minimum of another kind or limited to a damage type is not a floor of the roll.
func TestTheFloorOfARollIsTheHighestApplicableMinimum(t *testing.T) {
	t.Parallel()
	traits := link.Traits{RollMinimums: []link.RollMinimum{
		{Applies: []string{rules.RollAppliesAttack}, Value: 8, Source: "feat:eight"},
		{Applies: []string{rules.RollAppliesAttack, rules.RollAppliesSave}, Value: 12, Source: "feat:twelve"},
		{Applies: []string{rules.RollAppliesCheck}, Value: 10, Source: "feat:ten"},
		{Applies: []string{rules.RollAppliesSave}, Value: 18, Source: "feat:fire", DamageType: "damage-type:fire"},
		{Applies: []string{rules.RollAppliesSave}, Value: 12, Source: "feat:twelve-again"},
	}}
	belowTalent := link.Traits{RollMinimums: []link.RollMinimum{{Applies: []string{rules.RollAppliesCheck}, Value: 4, Source: "feat:four"}}}
	for _, tc := range []struct {
		name     string
		traits   link.Traits
		kind     string
		reliable bool
		want     rollFloor
	}{
		{"no minimum, no talent", link.Traits{}, rules.RollAppliesAttack, false, rollFloor{}},
		{"talent alone", link.Traits{}, rules.RollAppliesCheck, true, rollFloor{Value: 10, Source: reliableTalentSource}},
		{"the highest of the kind", traits, rules.RollAppliesAttack, false, rollFloor{Value: 12, Source: "feat:twelve"}},
		{"a tie between minimums keeps the first", traits, rules.RollAppliesSave, false, rollFloor{Value: 12, Source: "feat:twelve"}},
		{"a minimum above the talent wins", traits, rules.RollAppliesAttack, true, rollFloor{Value: 12, Source: "feat:twelve"}},
		{"a tie keeps the talent", traits, rules.RollAppliesCheck, true, rollFloor{Value: 10, Source: reliableTalentSource}},
		{"a minimum below the talent loses", belowTalent, rules.RollAppliesCheck, true, rollFloor{Value: 10, Source: reliableTalentSource}},
		{"a damage type limits the minimum to the damage", link.Traits{RollMinimums: traits.RollMinimums[3:4]}, rules.RollAppliesSave, false, rollFloor{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := floorFor(tc.traits, tc.kind, tc.reliable); got != tc.want {
				t.Errorf("floorFor() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// The die that counts counts as the floor when it came up lower; the faces stay and the total uses the floor. The other die
// of a pair does not change the pick.
func TestAFloorCountsTheDieThatCountsAndKeepsTheFaces(t *testing.T) {
	t.Parallel()
	floor := rollFloor{Value: 10, Source: "feat:ten"}
	for _, tc := range []struct {
		name        string
		roll        d20Roll
		wantCounted int
		wantTotal   int
		wantValue   int
	}{
		{"a low die", d20Roll{Faces: []int{3}, Total: 8}, 10, 15, 10},
		{"a die at the floor", d20Roll{Faces: []int{10}, Total: 15}, 0, 15, 10},
		{"a die above the floor", d20Roll{Faces: []int{14}, Total: 19}, 0, 19, 14},
		{"the higher of a pair with advantage", d20Roll{Faces: []int{2, 7}, Index: 1, Total: 12}, 10, 15, 10},
		{"the lower of a pair with disadvantage", d20Roll{Faces: []int{18, 4}, Index: 1, Total: 9}, 10, 15, 10},
		{"a pair that counts above the floor", d20Roll{Faces: []int{12, 3}, Total: 17}, 0, 17, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			faces := append([]int(nil), tc.roll.Faces...)
			got := floorD20(tc.roll, 5, floor)
			if got.CountedValue != tc.wantCounted || got.Total != tc.wantTotal || got.Value() != tc.wantValue {
				t.Errorf("floorD20() = counted %d, total %d, value %d; want %d, %d, %d", got.CountedValue, got.Total, got.Value(), tc.wantCounted, tc.wantTotal, tc.wantValue)
			}
			if got.CountedValue != 0 && got.CountedBy != floor.Source {
				t.Errorf("counted by %q, want %q", got.CountedBy, floor.Source)
			}
			for i, f := range faces {
				if got.Faces[i] != f {
					t.Errorf("faces = %v, want %v as they came up", got.Faces, faces)
				}
			}
		})
	}
}

// minimumTable is the fighters' table (Toren a fighter of level 9, Pensantus a wizard, Brisa a cleric) with a feat that
// gives one of them a roll_minimum of the rolls; it shows the DC of a save.
func minimumTable(t *testing.T, who string, rolls []string, value string) (a *armed, feat string) {
	t.Helper()
	a = fighterTable(t, "")
	feat = a.rerollFeat(t, &rulesv1.TableEffect{Type: "roll_minimum", Rolls: rolls, Value: value})
	a.giveFeat(t, map[string]*charactersv1.Character{"Toren": a.toren, "Pensantus": a.pens, "Brisa": a.bri}[who], feat)
	a.setRules(t, showSaveDC)
	return a, feat
}

// wantCountedAs checks the roll shows the face as it came up, the number it counted as, the feature and the total.
func wantCountedAs(t *testing.T, what string, d *playv1.DiceRoll, face, counted, total int32, source string) {
	t.Helper()
	if got := d.GetFaces()[d.GetCountedIndex()]; got != face || d.GetTreatedAs() != counted || d.GetTreatedAsSource() != source || d.GetTotal() != total {
		t.Errorf("%s = %v; want the face %d counted as %d by %q, total %d", what, d, face, counted, source, total)
	}
}

// attackCallKeyed is RollAttack with the key given, to send the same request twice.
func attackCallKeyed(key string, roll func(*playv1.RollAttackRequest)) func(*playv1.RollAttackRequest) {
	return func(r *playv1.RollAttackRequest) {
		roll(r)
		r.IdempotencyKey = key
	}
}

// logEntryWith is the entry of the log the pick says is the one.
func (a *armed) logEntryWith(t *testing.T, u *user, e *playv1.Encounter, pick func(*playv1.CombatLogEntry) bool) *playv1.CombatLogEntry {
	t.Helper()
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if pick(en) {
				return en
			}
		}
	}
	return nil
}

// A weapon attack with a minimum of 10 whose die shows 1 is not the automatic miss of a natural 1: the die counts 10, the
// total is 17 and it hits the goblin (AC 15). The faces stay, the roll says what it counted as and which feature did it,
// the master and the attacker's player read it in the log and nobody else reads the roll; a retry gives the same answer
// and the undo leaves nothing to put back.
func TestAWeaponAttackWithAMinimumOf10OnAOneIsNotAnAutomaticMiss(t *testing.T) {
	t.Parallel()
	a, feat := minimumTable(t, "Toren", []string{"attack"}, "10")
	e := a.closeFight(t)
	a.undoes(t, "an attack with a minimum", func() {
		key := newKey()
		res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", attackCallKeyed(key, d20(1)))
		if got := res.GetRoll().GetOutcome(); got != hit {
			t.Fatalf("a natural 1 counted as 10 is %v, want a hit on 10 + 7 against AC 15", got)
		}
		wantCountedAs(t, "the d20", res.GetRoll().GetD20(), 1, 10, 17, feat)
		again := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", attackCallKeyed(key, d20(1)))
		if !proto.Equal(res.GetRoll(), again.GetRoll()) {
			t.Errorf("the retry = %v, want %v", again.GetRoll(), res.GetRoll())
		}
		pick := func(en *playv1.CombatLogEntry) bool { return en.GetAttackRoll() != nil }
		for who, u := range map[string]*user{"the master": a.master, "the attacker's player": a.caio} {
			if en := a.logEntryWith(t, u, e, pick); en == nil {
				t.Errorf("%s reads no roll in the log", who)
			} else {
				wantCountedAs(t, who+"'s log roll", en.GetAttackRoll(), 1, 10, 17, feat)
			}
		}
		if en := a.logEntryWith(t, a.ana, e, pick); en != nil {
			t.Errorf("another player reads the roll %v, want the outcome only", en.GetAttackRoll())
		}
	})
}

// A minimum of 2 turns a 1 into a 2, which is no natural 1 but misses on its total (2 + 7 against AC 15).
func TestAWeaponAttackWithAMinimumOf2OnAOneMissesOnItsTotal(t *testing.T) {
	t.Parallel()
	a, feat := minimumTable(t, "Toren", []string{"attack"}, "2")
	e := a.closeFight(t)
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if got := res.GetRoll().GetOutcome(); got != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Errorf("a 1 counted as 2 is %v, want a miss", got)
	}
	wantCountedAs(t, "the d20", res.GetRoll().GetD20(), 1, 2, 9, feat)
}

// A minimum of 20 makes any die count 20, so the attack hits as a critical hit (the table's range includes 20).
func TestAWeaponAttackWithAMinimumOf20IsACriticalHit(t *testing.T) {
	t.Parallel()
	a, feat := minimumTable(t, "Toren", []string{"attack"}, "20")
	e := a.closeFight(t)
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(3))
	if got := res.GetRoll().GetOutcome(); got != crit {
		t.Errorf("a 3 counted as 20 is %v, want a critical hit", got)
	}
	wantCountedAs(t, "the d20", res.GetRoll().GetD20(), 3, 20, 27, feat)
}

// A die that does not show less than the minimum is not marked: a 14 stays a 14.
func TestAMinimumLeavesAHigherDieAlone(t *testing.T) {
	t.Parallel()
	a, _ := minimumTable(t, "Toren", []string{"attack"}, "10")
	e := a.closeFight(t)
	d := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(14)).GetRoll().GetD20()
	if d.TreatedAs != nil || d.GetTreatedAsSource() != "" || d.GetTotal() != 21 {
		t.Errorf("the d20 = %v, want the 14 unmarked with total 21", d)
	}
}

// A minimum of another roll does not reach an attack.
func TestAMinimumOfSavesDoesNotReachAnAttack(t *testing.T) {
	t.Parallel()
	a, _ := minimumTable(t, "Toren", []string{"save"}, "10")
	e := a.closeFight(t)
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", d20(1))
	if d := res.GetRoll().GetD20(); d.TreatedAs != nil || d.GetTotal() != 8 || res.GetRoll().GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_MISS {
		t.Errorf("the attack = %v, want the plain 1 that misses", res.GetRoll())
	}
}

// With advantage the pick stays the mode's: the higher die of [1, 3] counts, and counts as 10.
func TestAMinimumCountsTheDieThatCountsOfAPairWithAdvantage(t *testing.T) {
	t.Parallel()
	a, feat := minimumTable(t, "Toren", []string{"attack"}, "10")
	e := a.closeFight(t)
	a.setConditions(t, e, "Goblin", "condition:restrained")
	res := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Goblin", func(q *playv1.RollAttackRequest) { q.Roll, q.D20Faces = nil, []int32{1, 3} })
	d := res.GetRoll().GetD20()
	if len(d.GetFaces()) != 2 || d.GetFaces()[0] != 1 || d.GetFaces()[1] != 3 || d.GetCountedIndex() != 1 || res.GetRoll().GetOutcome() != hit {
		t.Fatalf("the attack = %v, want the pair [1 3] with the 3 counting, a hit", res.GetRoll())
	}
	wantCountedAs(t, "the d20", d, 3, 10, 17, feat)
}

// A spell attack of a cast counts the minimum too: Guiding Bolt next to the goblin has disadvantage, the 1 of [1, 4]
// counts and counts as 10, so it hits on 10 + 5; a retry gives the same cast.
func TestASpellAttackOfACastCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a, feat := minimumTable(t, "Brisa", []string{"attack"}, "10")
	a.closeFight(t)
	e := a.turnOf(t, "Brisa")
	key := newKey()
	cast := func(r *playv1.CastSpellRequest) {
		d20Faces(1, 4)(r)
		r.IdempotencyKey = key
	}
	res, err := a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), cast)
	if err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	tg := res.GetCast().GetTargets()
	if len(tg) != 1 || tg[0].GetOutcome() != playv1.AttackOutcome_ATTACK_OUTCOME_HIT {
		t.Fatalf("the cast = %v, want a hit", res.GetCast())
	}
	d := tg[0].GetAttackRoll()
	if len(d.GetFaces()) != 2 || d.GetCountedIndex() != 0 {
		t.Fatalf("the attack roll = %v, want the pair [1 4] with the 1 counting", d)
	}
	wantCountedAs(t, "the attack roll", d, 1, 10, 15, feat)
	again, err := a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), cast)
	if err != nil || !proto.Equal(again.GetCast().GetTargets()[0].GetAttackRoll(), d) {
		t.Errorf("the retry = %v, %v; want the same roll", again.GetCast().GetTargets(), err)
	}
}

// The saving throw of an effect counts the minimum: a 3 counted as 10 saves against DC 10 (Toren's Wisdom save is +0),
// Indomitable is not asked about a save that passed, a retry gives the same result, and the log says what the die counted
// as to the master and the player who rolled it, and shows the others no roll.
func TestAnEffectSaveCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a, feat := minimumTable(t, "Toren", []string{"save"}, "10")
	e, w := a.holdOnToren(t)
	key := newKey()
	set := func(r *playv1.RollEffectSaveRequest) {
		effectSaveFace(3)(r)
		r.IdempotencyKey = key
	}
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), set)
	if err != nil || res.GetHeldRollOffer() != nil {
		t.Fatalf("RollEffectSave(3) = %v, %v; want it settled", res, err)
	}
	r := res.GetResult()
	if !r.GetSaved() || r.GetD20() != 3 || r.GetTotal() != 10 || r.GetTreatedAs() != 10 || r.GetTreatedAsSource() != feat {
		t.Errorf("the save = %v; want the 3 counted as 10 by the feat, total 10, saved", r)
	}
	again, err := a.rollEffectSave(t, a.caio, e, w.GetId(), set)
	if g := again.GetResult(); err != nil || g.GetD20() != r.GetD20() || g.GetTotal() != r.GetTotal() || g.GetSaved() != r.GetSaved() ||
		g.GetTreatedAs() != r.GetTreatedAs() || g.GetTreatedAsSource() != r.GetTreatedAsSource() {
		t.Errorf("the retry = %v, %v; want the roll of %v", g, err, r)
	}
	pick := func(en *playv1.CombatLogEntry) bool { return en.GetEffect().D20 != nil }
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"the target's player", a.caio}} {
		en := a.logEntryWith(t, who.u, e, pick)
		if en == nil || en.GetEffect().GetTreatedAs() != 10 || en.GetEffect().GetTreatedAsSource() != feat || en.GetEffect().GetD20() != 3 || en.GetEffect().GetTotal() != 10 {
			t.Errorf("%s reads %v, want the 3 counted as 10 by the feat", who.name, en.GetEffect())
		}
	}
	for _, en := range a.log(t, a.ana, e).GetRounds() {
		for _, line := range en.GetEntries() {
			if ef := line.GetEffect(); ef != nil && (ef.D20 != nil || ef.TreatedAs != nil) {
				t.Errorf("another player reads %v, want no roll", line.GetEffect())
			}
		}
	}
}

// The concentration save counts the minimum: Pensantus's Constitution save is +1, so a 3 counted as 10 makes 11 against DC
// 10 and keeps the concentration; the master's log line says both numbers, and a retry gives the same result.
func TestAConcentrationSaveCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a := multiclassTable(t)
	feat := a.rerollFeat(t, &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"save"}, Value: "10"})
	a.giveFeat(t, a.pens, feat)
	a, e := concentratingHitOn(t, a, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	key := newKey()
	set := func(r *playv1.ResolveConcentrationSaveRequest) {
		concentrationFace(3)(r)
		r.IdempotencyKey = key
	}
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), set)
	if err != nil || res.GetHeldRollOffer() != nil || !res.GetResult().GetKept() {
		t.Fatalf("ResolveConcentrationSave(3) = %v, %v; want the concentration kept", res, err)
	}
	wantCountedAs(t, "the save", res.GetResult().GetSave(), 3, 10, 11, feat)
	var line string
	for _, en := range logEntries(a.log(t, a.master, e)) {
		if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_REACTION_WINDOW {
			line = en.GetReactionTextPt()
		}
	}
	if want := "Pensantus manteve a concentração em Teia: 1d20 (3, conta como 10) + 1 = 11 contra CD 10."; line != want {
		t.Errorf("the master's line = %q, want %q", line, want)
	}
	again, err := a.resolveConcentration(t, a.ana, e, w.GetId(), set)
	if err != nil || !proto.Equal(again.GetResult(), res.GetResult()) {
		t.Errorf("the retry = %v, %v; want %v", again.GetResult(), err, res.GetResult())
	}
}

// dyingTable is the fighters' table with Toren at 0 hit points, a death save due on his turn and a minimum of the saves.
func dyingTable(t *testing.T, value string) (*armed, *playv1.Encounter, string) {
	t.Helper()
	a, feat := minimumTable(t, "Toren", []string{"save"}, value)
	e := a.castersFight(t, 1)
	a.correct(t, a.toren, hpIs(0))
	a.mustEndTurn(t, a.ana, e)
	return a, e, feat
}

// A death save counts the die that counts: a 1 that counts as 10 is an ordinary 10 (a success, not the two failures of a
// natural 1), a 9 counts 10, a minimum of 2 makes a 1 an ordinary 2 (one failure), and a die that counts 20 brings the
// character back. Indomitable is not asked about a save that passed on the die that counts.
func TestADeathSaveCountsTheMinimum(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                         string
		value                        string
		face                         int32
		outcome                      playv1.DeathSaveOutcome
		counted, total               int32
		successes, failures          int32
		asksIndomitable, wantTreated bool
	}{
		{"a 1 counted as 10", "10", 1, playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS, 10, 10, 1, 0, false, true},
		{"a 9 counted as 10", "10", 9, playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS, 10, 10, 1, 0, false, true},
		{"a 1 counted as 2", "2", 1, playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_FAILURE, 2, 2, 0, 1, true, true},
		{"a 5 counted as 20", "20", 5, playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_REVIVED, 20, 20, 0, 0, false, true},
		{"a 12 stays a 12", "10", 12, playv1.DeathSaveOutcome_DEATH_SAVE_OUTCOME_SUCCESS, 0, 12, 1, 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, e, feat := dyingTable(t, tc.value)
			key := newKey()
			set := func(r *playv1.RollDeathSaveRequest) {
				deathFace(tc.face)(r)
				r.IdempotencyKey = key
			}
			res, err := a.deathSave(t, a.caio, e, "Toren", set)
			if tc.asksIndomitable {
				if err != nil || res.GetHeldRollOffer().GetIndomitable() == nil {
					t.Fatalf("RollDeathSave(%d) = %v, %v; want Indomitable's question about the failure", tc.face, res, err)
				}
				out, err := a.answerHeldRoll(t, a.caio, res.GetHeldRollOffer().GetHoldId(), indomitableAnswer(false, 0))
				if err != nil {
					t.Fatalf("AnswerHeldRoll() error = %v", err)
				}
				res = &playv1.RollDeathSaveResponse{DeathSave: out.GetDeathSave().GetDeathSave()}
			} else if err != nil || res.GetHeldRollOffer() != nil {
				t.Fatalf("RollDeathSave(%d) = %v, %v; want it settled", tc.face, res, err)
			}
			ds := res.GetDeathSave()
			if ds.GetOutcome() != tc.outcome || ds.GetSuccesses() != tc.successes || ds.GetFailures() != tc.failures {
				t.Errorf("the death save = %v, want %v with %d successes and %d failures", ds, tc.outcome, tc.successes, tc.failures)
			}
			if tc.wantTreated {
				wantCountedAs(t, "the roll", ds.GetRoll(), tc.face, tc.counted, tc.total, feat)
			} else if r := ds.GetRoll(); r.TreatedAs != nil || r.GetTotal() != tc.total {
				t.Errorf("the roll = %v, want the %d unmarked", r, tc.face)
			}
			if tc.asksIndomitable {
				return
			}
			again, err := a.deathSave(t, a.caio, e, "Toren", set)
			if err != nil || !proto.Equal(again.GetDeathSave(), ds) {
				t.Errorf("the retry = %v, %v; want %v", again.GetDeathSave(), err, ds)
			}
			if tc.wantTreated {
				en := a.logEntryWith(t, a.master, e, func(en *playv1.CombatLogEntry) bool { return en.GetDeathSave() != nil })
				if en == nil {
					t.Fatalf("the master reads no death save in the log")
				}
				wantCountedAs(t, "the log roll", en.GetDeathSave().GetRoll(), tc.face, tc.counted, tc.total, feat)
			}
		})
	}
}

// The reroll is asked about the face as rolled and the minimum applies to the die that stays: a 2 is offered the reroll,
// the new 1 counts as 10 and hits, and the roll tells the die it replaced.
func TestTheRerollIsAskedAboutTheFaceAsRolledAndTheMinimumCountsTheNewDie(t *testing.T) {
	t.Parallel()
	effects := append(d20RerollEffects("2", 2, "1"), &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"attack"}, Value: "10"})
	r := d20RerollTable(t, true, effects)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	offer := res.GetHeldRollOffer()
	r.wantReroll(t, "the held attack", offer, playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 2)
	d := r.rolledAgain(t, r.caio, offer, 1).GetAttack().GetRoll()
	if d.GetOutcome() != hit {
		t.Errorf("the attack = %v, want a hit on the new 1 counted as 10", d)
	}
	rr := d.GetD20().GetRerolled()
	if len(rr) != 1 || rr[0].GetFrom() != 2 || rr[0].GetTo() != 1 {
		t.Errorf("rerolled = %v, want 2 -> 1", rr)
	}
	if got := d.GetD20(); got.GetTreatedAs() != 10 || got.GetTotal() != 16 || got.GetFaces()[0] != 1 {
		t.Errorf("the d20 = %v, want the new 1 counted as 10", got)
	}
}

// A die that is kept after the question still counts the minimum.
func TestKeepingTheRollAfterTheRerollQuestionStillCountsTheMinimum(t *testing.T) {
	t.Parallel()
	effects := append(d20RerollEffects("2", 2, "1"), &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"attack"}, Value: "10"})
	r := d20RerollTable(t, true, effects)
	e := r.closeFight(t)
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", d20(2))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	out, err := r.answerHeldRoll(t, r.caio, res.GetHeldRollOffer().GetHoldId(), rerollAnswer(false))
	if err != nil {
		t.Fatalf("AnswerHeldRoll() error = %v", err)
	}
	if d := out.GetAttack().GetRoll().GetD20(); d.GetTreatedAs() != 10 || d.GetTotal() != 16 || d.GetFaces()[0] != 2 {
		t.Errorf("the d20 = %v, want the 2 kept and counted as 10", d)
	}
}

// wantNoLeak checks a roll that hides its faces hides what it counted as too.
func wantNoLeak(t *testing.T, what string, d *playv1.DiceRoll) {
	t.Helper()
	if len(d.GetFaces()) == 0 && (d.GetTreatedAs() != 0 || d.GetTreatedAsSource() != "") {
		t.Errorf("%s = %v, want no faces and no counted number together", what, d)
	}
}

// Sacred Flame on Toren, whose saves have a minimum of 20: the 3 counts 20 and saves.
// The master and Toren's player read the face and what it counted as, the others read how the save ended.
func TestASpellSaveOfATargetCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a, e := luckySaveFight(t, "", "Brisa", false)
	feat := a.rerollFeat(t, &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"save"}, Value: "20"})
	a.giveFeat(t, a.toren, feat)
	a.h.roller.queue(3)
	res := a.flameAt(t, e, "Toren")
	if sv := res.GetCast().GetTargets()[0].GetSave(); sv.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED || sv.GetRoll() != nil {
		t.Errorf("the caster's player reads %v, want the outcome alone", sv)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Caio", a.caio}} {
		sv := a.castSaveOf(t, who.u, e, "Toren")
		roll := sv.GetRoll()
		if sv.GetOutcome() != playv1.SaveOutcome_SAVE_OUTCOME_SAVED {
			t.Errorf("%s reads %v, want the save passed", who.name, sv)
		}
		wantCountedAs(t, who.name+"'s roll", roll, 3, 20, 20+roll.GetModifier(), feat)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"Bia", a.bia}, {"Ana", a.ana}} {
		sv := a.castSaveOf(t, who.u, e, "Toren")
		wantNoLeak(t, who.name+"'s roll", sv.GetRoll())
		if sv.GetRoll() != nil {
			t.Errorf("%s reads the dice %v, want none", who.name, sv.GetRoll())
		}
	}
}

// A cast held for the master's Legendary Resistance rolls the saves of the other targets once: Toren's 1 counts 20 when the
// master answers and the cast goes on from the held rolls.
func TestAHeldCastReplaysTheMinimumOfATargetsSave(t *testing.T) {
	t.Parallel()
	a, e := resistFightWith(t, adultRedDragon, "", "Draco")
	feat := a.rerollFeat(t, &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"save"}, Value: "20"})
	a.giveFeat(t, a.toren, feat)
	a.put(t, "Toren", 7, 5)
	a.h.roller.queue(1, 3) // Toren's save is rolled first, then the dragon's
	if _, err := a.holdAt(t, "Draco", "Toren"); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	w := a.windowOf(t, a.master, resistKind)
	if w == nil {
		t.Fatalf("the master's windows = %v, want the Legendary Resistance window", a.windows(t, a.master))
	}
	asked := a.h.roller.taken()
	if _, err := a.resistUse(t, a.master, e, w, "Draco"); err != nil {
		t.Fatalf("AnswerReaction() error = %v", err)
	}
	if again := a.h.roller.taken(); again != asked {
		t.Errorf("the replay rolled %d dice more, want none", again-asked)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Caio", a.caio}} {
		roll := a.castSaveOf(t, who.u, a.get(t, a.master), "Toren").GetRoll()
		wantCountedAs(t, who.name+"'s roll", roll, 1, 20, 20+roll.GetModifier(), feat)
	}
	wantNoLeak(t, "Ana's roll", a.castSaveOf(t, a.ana, a.get(t, a.master), "Toren").GetRoll())
}

// A held attack that offers a reroll says what the die counts as: the offer, the list and a retry agree.
func TestTheOfferOfAHeldRollSaysWhatTheDieCountsAs(t *testing.T) {
	t.Parallel()
	effects := append(d20RerollEffects("2", 2, "1"), &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"attack"}, Value: "10"})
	r := d20RerollTable(t, true, effects)
	e := r.closeFight(t)
	key := newKey()
	res, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", attackCallKeyed(key, d20(2)))
	if err != nil {
		t.Fatalf("RollAttack(2) error = %v", err)
	}
	offer := res.GetHeldRollOffer()
	r.wantReroll(t, "the first offer", offer, playv1.HeldRollKind_HELD_ROLL_KIND_WEAPON_ATTACK, 2)
	d := offer.GetD20()
	wantCountedAs(t, "the first offer's d20", d, 2, 10, 16, r.feat)
	list := r.listHeldRolls(t, r.caio)
	if len(list) != 1 || !proto.Equal(list[0].GetD20(), d) {
		t.Errorf("the list = %v, want the offer's d20 %v", list, d)
	}
	again, err := r.attack(t, r.caio, e, "Toren", battleaxe, "Goblin", attackCallKeyed(key, d20(2)))
	if err != nil || !proto.Equal(again.GetHeldRollOffer().GetD20(), d) {
		t.Errorf("the retry = %v, %v; want the same d20", again.GetHeldRollOffer(), err)
	}
}

// The offer of a roll with only Reliable Talent behind it keeps what it always said: a single d20 shows no counted number,
// a pair with advantage shows it with Reliable Talent as the source, and a row from before the source was kept reads the same.
func TestTheOfferOfAHeldRollKeepsWhatReliableTalentAlwaysSaid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		held       heldRoll
		wantTreat  int32
		wantSource string
	}{
		{"a single d20", heldRoll{Faces: []int{3}, Modifier: 5, Total: 15}, 0, ""},
		{"a single d20 of Reliable Talent", heldRoll{Faces: []int{3}, Modifier: 5, Total: 15, CountedBy: reliableTalentSource}, 0, ""},
		{"a pair of a row without source", heldRoll{Faces: []int{3, 2}, Modifier: 5, Total: 15}, 10, reliableTalentSource},
		{"a pair of Reliable Talent", heldRoll{Faces: []int{3, 2}, Modifier: 5, Total: 15, CountedBy: reliableTalentSource}, 10, reliableTalentSource},
		{"a single d20 of a table minimum", heldRoll{Faces: []int{3}, Modifier: 5, Total: 15, CountedBy: "feat:ten"}, 10, "feat:ten"},
		{"a pair of a table minimum", heldRoll{Faces: []int{3, 2}, Modifier: 5, Total: 15, CountedBy: "feat:ten"}, 10, "feat:ten"},
		{"a pair above the floor", heldRoll{Faces: []int{12, 2}, Modifier: 5, Total: 17}, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := heldD20Of(tc.held)
			if d.GetTreatedAs() != tc.wantTreat || d.GetTreatedAsSource() != tc.wantSource || (tc.wantTreat == 0) != (d.TreatedAs == nil) {
				t.Errorf("heldD20Of() = %v, want counted as %d by %q", d, tc.wantTreat, tc.wantSource)
			}
		})
	}
}

// A save that fails on the number it counts as is asked about Indomitable: the question shows the face and the number, the
// new roll replaces the die and counts as it came up.
func TestIndomitableIsAskedAboutAFailedEffectSaveThatTheMinimumRaised(t *testing.T) {
	t.Parallel()
	a, feat := minimumTable(t, "Toren", []string{"save"}, "8")
	e, w := a.holdOnToren(t)
	res, err := a.rollEffectSave(t, a.caio, e, w.GetId(), effectSaveFace(2))
	offer := res.GetHeldRollOffer()
	if err != nil || offer.GetIndomitable() == nil || !offer.GetIndomitable().GetFailed() {
		t.Fatalf("RollEffectSave(2) = %v, %v; want Indomitable's question about the failed 8", res, err)
	}
	wantCountedAs(t, "the offer's d20", offer.GetD20(), 2, 8, 8, feat)
	out, err := a.answerHeldRoll(t, a.caio, offer.GetHoldId(), indomitableAnswer(true, 17))
	r := out.GetEffectSave().GetResult()
	if err != nil || !r.GetSaved() || r.GetD20() != 17 || r.GetTreatedAs() != 0 || len(r.GetRerolled()) != 1 || r.GetRerolled()[0].GetFrom() != 2 {
		t.Errorf("the save = %v, %v; want the new 17 to save, with no counted number, replacing the 2", r, err)
	}
}

// The same for the concentration save: a 2 counts 8 against DC 10 and +1 makes 9, so Indomitable is asked.
func TestIndomitableIsAskedAboutAFailedConcentrationSaveThatTheMinimumRaised(t *testing.T) {
	t.Parallel()
	a := multiclassTable(t)
	feat := a.rerollFeat(t, &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"save"}, Value: "8"})
	a.giveFeat(t, a.pens, feat)
	a, e := concentratingHitOn(t, a, 6)
	w := a.windowOf(t, a.ana, playv1.ReactionKind_REACTION_KIND_CONCENTRATION_SAVE)
	if w == nil {
		t.Fatalf("Pensantus's windows = %v, want a concentration save", a.windows(t, a.ana))
	}
	res, err := a.resolveConcentration(t, a.ana, e, w.GetId(), concentrationFace(2))
	offer := res.GetHeldRollOffer()
	if err != nil || offer.GetIndomitable() == nil || !offer.GetIndomitable().GetFailed() {
		t.Fatalf("ResolveConcentrationSave(2) = %v, %v; want Indomitable's question", res, err)
	}
	wantCountedAs(t, "the offer's d20", offer.GetD20(), 2, 8, 9, feat)
	out, err := a.answerHeldRoll(t, a.ana, offer.GetHoldId(), indomitableAnswer(true, 15))
	r := out.GetConcentrationSave().GetResult()
	if err != nil || !r.GetKept() || r.GetSave().GetFaces()[0] != 15 || r.GetSave().GetTreatedAs() != 0 {
		t.Errorf("the save = %v, %v; want the new 15 to keep the concentration", r, err)
	}
}

// A spell attack hides what it counted as wherever it hides its faces.
func TestASpellAttackHidesTheCountedNumberWithTheFaces(t *testing.T) {
	t.Parallel()
	a, _ := minimumTable(t, "Brisa", []string{"attack"}, "10")
	a.closeFight(t)
	e := a.turnOf(t, "Brisa")
	if _, err := a.cast(t, a.bia, e, "Brisa", guidingBolt, slotOfLevel(1), a.at(t, "Goblin"), d20Faces(1, 4)); err != nil {
		t.Fatalf("CastSpell() error = %v", err)
	}
	for _, who := range []struct {
		name string
		u    *user
	}{{"the master", a.master}, {"Bia", a.bia}, {"Caio", a.caio}, {"Ana", a.ana}} {
		for _, l := range logEntries(a.log(t, who.u, e)) {
			for _, tg := range l.GetSpell().GetTargets() {
				wantNoLeak(t, who.name+"'s attack roll", tg.GetAttackRoll())
			}
		}
	}
}

// The save of a creature's rider counts the target's minimum: the ghoul's claws ask DC 10 of Toren, whose saves have a
// minimum of 20, so his 2 counts 20 and he is not paralyzed.
// The master's line and Toren's player read it; another player reads the result alone.
func TestACreatureRiderSaveCountsTheMinimum(t *testing.T) {
	t.Parallel()
	a, e, ghoul := creatureFight(t, true, "monster:ghoul", false)
	feat := a.rerollFeat(t, &rulesv1.TableEffect{Type: "roll_minimum", Rolls: []string{"save"}, Value: "20"})
	a.giveFeat(t, a.toren, feat)
	res := a.mustAttack(t, a.master, e, ghoul, "basic:1", "Toren", d20(19))
	id := res.GetPendingDamage().GetId()
	a.mustDamage(t, a.master, e, id, typedDamage(5))
	if _, err := a.settle(t, a.master, e, id, true); err != nil {
		t.Fatalf("apply the claws: %v", err)
	}
	offers := ridersOfKind(a.get(t, a.master), playv1.HitRiderKind_HIT_RIDER_KIND_CREATURE)
	if len(offers) != 1 {
		t.Fatalf("creature offers = %v, want the claws' rider", offers)
	}
	a.h.roller.queue(2)
	after, err := a.useRider(t, a.master, e, offers[0].GetId(), playv1.HitRiderChoice_HIT_RIDER_CHOICE_APPLY)
	if err != nil {
		t.Fatalf("UseHitRider(apply) error = %v", err)
	}
	if hasCondition(after, t, "Toren", "condition:paralyzed") {
		t.Error("Toren is paralyzed, want the 2 to count 20 and save")
	}
	for _, who := range []struct {
		name string
		line *playv1.CombatLogHitRider
	}{{"the master", a.lastRiderLine(t)}, {"Toren's player", a.lastRiderLineOf(t, a.caio)}} {
		if l := who.line; l.GetD20() != 2 || l.GetTreatedAs() != 20 || l.GetTreatedAsSource() != feat || l.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED {
			t.Errorf("%s reads %v, want the 2 counted as 20 by the feat, saved", who.name, l)
		}
	}
	if other := a.lastRiderLineOf(t, a.ana); other.GetD20() != 0 || other.GetTreatedAs() != 0 || other.GetTreatedAsSource() != "" {
		t.Errorf("another player reads %v, want the result alone", other)
	}
}
