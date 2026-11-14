package play

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The weapon-hit-damage spells of #577, second part (combat_hit_effects_test.go is the first): who reads what of the
// pending effect (RN-10), what a cast refuses, how the two blocks of an effect split between the caster and its
// targets, the conditions of a failed save and their length, and the cases around the hit (an Extra Attack, two
// spells, another caster's row, a thrown weapon, a spell attack, a recast, a mark whose target died).

const (
	frightenedKey = "condition:frightened"
	sealKey       = "spell:selo-de-nanquim@mesa"
)

// dropConcentration is the caster ending its concentration.
func (a *armed) dropConcentration(t *testing.T, e *playv1.Encounter) {
	t.Helper()
	if _, err := a.ana.combat.EndConcentration(t.Context(), connect.NewRequest(&playv1.EndConcentrationRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), CombatantId: a.id(t, "Pensantus"), IdempotencyKey: newKey(),
	})); err != nil {
		t.Fatalf("EndConcentration() error = %v", err)
	}
}

// conditioned says the combatant has the condition, as the master reads it.
func (a *armed) conditioned(t *testing.T, label, cond string) bool {
	t.Helper()
	return slices.Contains(byLabel(t, a.get(t, a.master), label).GetConditions(), cond)
}

// pendingOfPensantus are the pending damages of Pensantus that u reads in his turn options.
func (a *armed) pendingOfPensantus(t *testing.T, u *user, e *playv1.Encounter) []*playv1.PendingDamage {
	t.Helper()
	opts, err := a.options(t, u, e, "Pensantus")
	if err != nil {
		t.Fatalf("GetTurnOptions(Pensantus) error = %v", err)
	}
	return opts.GetPendingDamages()
}

// effectPartOf is the automatic part of a pending damage that is not the weapon's: the spell's, whatever it is called.
func effectPartOf(p *playv1.PendingDamage) *playv1.DamagePart {
	for _, d := range p.GetParts() {
		if d.GetAuto() && d.GetKey() != extraWeaponDamage && !d.GetChoosable() {
			return d
		}
	}
	return nil
}

// hitRiderLines are the lines of the log that are an answered rider, as u reads them.
func (a *armed) hitRiderLines(t *testing.T, u *user, e *playv1.Encounter) []*playv1.CombatLogHitRider {
	t.Helper()
	var out []*playv1.CombatLogHitRider
	for _, r := range a.log(t, u, e).GetRounds() {
		for _, en := range r.GetEntries() {
			if en.GetKind() == playv1.CombatLogKind_COMBAT_LOG_KIND_HIT_RIDER {
				out = append(out, en.GetHitRider())
			}
		}
	}
	return out
}

// RN-10: the dice, the die, the trigger and the save of the pending effect are the caster's player's and the master's.
// A hit with the effect pending is made and read as every other player: its part (the spell's name, or "Outra fonte"
// when the effect is not theirs), its dice, the pending damage, the log and the turn options. The canary is a d12 of 7
// dice (a 13 cap), which nothing else of the fight shows.
func TestRN10_APendingHitEffectReachesOnlyItsCastersPlayerAndTheMaster(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		visibility rulesv1.TableEffectAudience
		label      string
	}{
		{"a public effect", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, "Marca de Nanquim"},
		{"an effect of its owner", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER, otherSourceTx},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, func(s *rulesv1.TableSpell) {
				markShape(s)
				s.Effect.Visibility = tc.visibility
			}, hitMod(trigMarked, func(m *tableMod) {
				m.Die, m.Dice, m.MaxDice, m.Save, m.OnFailConditions = 12, 7, 13, "wis", []string{frightenedKey}
			}))
			e := a.hitFight(t)
			a.castMark(t, e, 1, "Goblin")
			hit := a.strike(t, e, battleaxe, "Goblin", d20(19))
			if _, err := a.options(t, a.caio, e, "Pensantus"); connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Errorf("another player's turn options of the caster: error = %v, want permission_denied", err)
			}
			canaries := []*regexp.Regexp{
				regexp.MustCompile(`"diceSides":\s*12`), regexp.MustCompile(`"diceCount":\s*7\b`), regexp.MustCompile(`(?i)marked_target|next_hit|each_hit|next_melee_hit`),
				regexp.MustCompile(`Depois do dano, o alvo faz um teste de resistência de Sabedoria`), // the rider text of the part
			}
			// The rider text is on the part before the roll: the caster's player and the master read it (the positive
			// control), and the others read no word of it, before the roll or after.
			for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
				if text := a.readsOf(t, u, e, "Pensantus"); !canaries[3].MatchString(text) {
					t.Errorf("%s does not read the rider text of the part: the canary is unreachable", name)
				}
			}
			for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
				if text := a.readsOf(t, u, e, "Pensantus"); canaries[3].MatchString(text) {
					t.Errorf("%s reads the rider text of the part before the roll", name)
				}
			}
			a.rollDamage(t, hit, 1, 1, 2, 3, 4, 5, 6, 7, 2) // the weapon, the seven dice, the save (a 2: it fails)
			for _, p := range []struct {
				name string
				u    *user
			}{{"Caio", a.caio}, {"Bia", a.bia}} {
				text := a.readsOf(t, p.u, e, "Pensantus")
				for _, re := range canaries {
					if re.MatchString(text) {
						t.Errorf("%s reads %s of the pending effect", p.name, re)
					}
				}
				if lines := a.hitRiderLines(t, p.u, e); tc.label == otherSourceTx && len(lines) != 0 {
					t.Errorf("%s reads %d save lines of an effect that is not theirs", p.name, len(lines))
				}
			}
			// The positive controls: the caster's player and the master read the card with the die and the spell's name.
			for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
				text := a.readsOf(t, u, e, "Pensantus")
				if !canaries[0].MatchString(text) || !canaries[1].MatchString(text) || !strings.Contains(text, "Marca de Nanquim") {
					t.Errorf("%s does not read the dice and the name of the effect: the canary is unreachable", name)
				}
			}
		})
	}
}

// The part of a hit that is read before the roll, by the caster's player and the master: the spell's name and its dice.
func TestRN10_TheCastersPlayerAndTheMasterReadThePartWhole(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) { m.Die, m.Dice, m.MaxDice = 12, 7, 13 }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.strike(t, e, battleaxe, "Goblin", d20(19))
	for name, u := range map[string]*user{"Ana": a.ana, "the master": a.master} {
		part := effectPartOf(a.pendingOfPensantus(t, u, e)[0])
		if part == nil || part.GetLabelPt() != "Marca de Nanquim" || part.GetDiceCount() != 7 || part.GetDiceSides() != 12 || part.GetKey() != tableMark {
			t.Errorf("%s reads the part %v, want Marca de Nanquim, 7d12", name, part)
		}
	}
}

// A marked_target spell needs exactly one target that is not the caster, and refuses the cast before anything is spent:
// the slot and the bonus action are still there for the cast that follows.
func TestAHitSpellMarkedTargetRefusesTheWrongTargetsBeforeSpendingAnything(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	slotsBefore := a.slotsLeft(t, e)
	for _, tc := range []struct {
		name    string
		targets []string
	}{{"two creatures", []string{"Goblin", "Capitão Goblin"}}, {"only the caster", []string{"Pensantus"}}} {
		_, err := a.cast(t, a.ana, e, "Pensantus", tableMark, slotOfLevel(1), a.at(t, tc.targets...), noCastRoll)
		wantCode(t, tc.name, err, connect.CodeInvalidArgument)
		if all, _ := a.rows(t); all != 0 {
			t.Errorf("%s: %d rows after the refusal, want none", tc.name, all)
		}
	}
	if got := a.slotsLeft(t, e); got != slotsBefore {
		t.Errorf("slots left = %d after the refusals, want %d: nothing is spent", got, slotsBefore)
	}
	a.castMark(t, e, 1, "Goblin") // the bonus action was not spent
	if all, _ := a.rows(t); all != 2 {
		t.Errorf("after the cast: %d rows, want the caster's and the mark", all)
	}
}

// slotsLeft is the 1st-level slots Pensantus has free for the spell.
func (a *armed) slotsLeft(t *testing.T, e *playv1.Encounter) int32 {
	t.Helper()
	opt := spellOption(a.mustOptions(t, a.ana, e, "Pensantus"), tableMark)
	if opt == nil || len(opt.GetSlots()) == 0 {
		t.Fatalf("the spell's option = %v, want it listed with its slots", opt)
	}
	return opt.GetSlots()[0].GetFree()
}

// The mark follows the visibility of the effect: a public one is the spell's name on the target for every player (and
// nothing else: no modifier, no dice), an owner's one is hidden from the other players.
func TestAHitSpellTheMarkFollowsTheVisibilityOfTheEffect(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		visibility rulesv1.TableEffectAudience
		label      string
		seen       bool
	}{
		{"public", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_PUBLIC, "Marca de Nanquim", true},
		{"owner", rulesv1.TableEffectAudience_TABLE_EFFECT_AUDIENCE_OWNER, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newHitCasters(t, func(s *rulesv1.TableSpell) {
				markShape(s)
				s.Effect.Visibility = tc.visibility
			}, hitMod(trigMarked, func(m *tableMod) { m.Die, m.Dice, m.MaxDice = 12, 7, 13 }))
			e := a.hitFight(t)
			a.castMark(t, e, 1, "Goblin")
			goblin := byLabel(t, a.get(t, a.caio), "Goblin")
			var seen string
			for _, l := range goblin.GetEffectLabels() {
				seen = l.GetTextPt()
			}
			if (tc.seen && seen != tc.label) || (!tc.seen && seen != "") {
				t.Errorf("Caio reads the label %q on the goblin, want %q", seen, tc.label)
			}
			if text := jsonOf(a.get(t, a.caio)) + jsonOf(a.log(t, a.caio, e)); regexp.MustCompile(`"diceSides":\s*12|(?i)marked_target`).MatchString(text) {
				t.Errorf("Caio reads the dice or the trigger of the mark")
			}
			if byLabel(t, a.get(t, a.master), "Goblin") == nil || len(byLabel(t, a.get(t, a.master), "Goblin").GetEffects()) == 0 {
				t.Errorf("the master does not read the mark on the goblin")
			}
			for _, ef := range byLabel(t, a.get(t, a.master), "Goblin").GetEffects() {
				if len(ef.GetModifiers()) != 0 {
					t.Errorf("the mark carries the modifiers %v, want none", ef.GetModifiers())
				}
			}
		})
	}
}

// The save of a spell is written like the save of a monk's rider: the master reads the d20, the DC and the result, a
// passed save is a line too, and another player of the table reads the result and no number.
func TestAHitSpellTheSaveIsInTheLogAsARiderLine(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1, 19) // passes
	e = a.nextTurn(t, e)
	hit = a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1, 4) // fails
	dc := conv.Int32(a.count(t, `SELECT save_dc FROM combatant_states WHERE source_key = $1 AND save_dc IS NOT NULL`, tableMark))
	lines := a.hitRiderLines(t, a.master, e)
	if len(lines) != 2 {
		t.Fatalf("the master reads %d rider lines, want 2 (the save that passed and the one that failed)", len(lines))
	}
	for _, r := range lines {
		passed := r.GetResult() == playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_SAVED
		if (passed && r.GetD20() != 19) || (!passed && (r.GetD20() != 4 || r.GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED)) || !r.GetNumbers() || r.GetDc() != dc || dc == 0 {
			t.Errorf("the line %v, want saved with d20 19 or failed with d20 4, both against DC %d", r, dc)
		}
	}
	// The line says the spell has a save: the master's, the attacker's player's and the target's player's alone (RN-10).
	for name, u := range map[string]*user{"the attacker's player": a.ana} {
		if n := len(a.hitRiderLines(t, u, e)); n != 2 {
			t.Errorf("%s reads %d save lines of an NPC's save, want 2", name, n)
		}
	}
	for name, u := range map[string]*user{"Caio": a.caio, "Bia": a.bia} {
		if lines := a.hitRiderLines(t, u, e); len(lines) != 0 {
			t.Errorf("%s, a player of neither side, reads %d save lines: %v", name, len(lines), lines)
		}
	}
	// A save of a player's character: its own player and the attacker's read it, the third player reads nothing.
	e = a.nextTurn(t, e)
	hit = a.strike(t, e, battleaxe, "Toren", d20(19))
	a.rollDamage(t, hit, 1, 1, 6)
	for name, want := range map[string]int{"the master": 3, "the attacker's player": 3, "the target's player": 1} {
		u := map[string]*user{"the master": a.master, "the attacker's player": a.ana, "the target's player": a.caio}[name]
		if n := len(a.hitRiderLines(t, u, e)); n != want {
			t.Errorf("%s reads %d save lines, want %d", name, n, want)
		}
	}
	if lines := a.hitRiderLines(t, a.bia, e); len(lines) != 0 {
		t.Errorf("Bia, a player of neither side, reads %d save lines: %v", len(lines), lines)
	}
}

// The target's own player reads the d20 of its save (the DC when the table shows it), as for a monk's rider.
func TestAHitSpellTheTargetsPlayerReadsItsOwnSave(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Toren", d20(19))
	a.rollDamage(t, hit, 1, 1, 6)
	lines := a.hitRiderLines(t, a.caio, e)
	if len(lines) != 1 || !lines[0].GetNumbers() || lines[0].GetD20() != 6 || lines[0].GetResult() != playv1.HitRiderLogResult_HIT_RIDER_LOG_RESULT_FAILED {
		t.Errorf("Toren's player reads %v, want his own d20 (6) and the failure", lines)
	}
}

// The concentration that ends between the hit and the roll takes the effect with it: the roll has no part of the spell,
// and no save.
func TestAHitSpellConcentrationDroppedBetweenTheHitAndTheRoll(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	wantHitPart(t, "the hit", hit, true)
	a.dropConcentration(t, e)
	dmg := a.rollDamage(t, hit, 5)
	if r := hitRoll(dmg); r != nil {
		t.Errorf("the roll has the spell's part %v after the concentration ended", r)
	}
	if lines := a.hitRiderLines(t, a.master, e); len(lines) != 0 || a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("a save was made (%d lines) or a condition given after the concentration ended", len(lines))
	}
}

// An Extra Attack with two pending hits on one next_hit: both are offered the part, the first roll uses the effect and
// the second finds it gone (its part is out, and the damage rolls as a plain weapon hit).
func TestAHitSpellTwoPendingHitsOnOneNextHit(t *testing.T) {
	t.Parallel()
	a := newHitParty(t, hitParty{class: "class:paladin", level: 5, cha: 16, spells: []hitSpellOf{{name: "Marca de Nanquim", key: tableMark, mods: []*tableMod{hitMod(trigNext)}}}})
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	first := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	second := a.strike(t, e, battleaxe, "Capitão Goblin", d20(18))
	wantHitPart(t, "the first hit", first, true)
	wantHitPart(t, "the second hit", second, true)
	if r := hitRoll(a.rollDamage(t, first, 3, 4)); r == nil {
		t.Fatalf("the first roll has no part of the spell")
	}
	dmg := a.rollDamage(t, second, 5)
	if r := hitRoll(dmg); r != nil {
		t.Errorf("the second roll has the part %v of an effect that was already used", r)
	}
	if p := hitPart(dmg); p != nil && p.GetAvailable() {
		t.Errorf("the second hit still offers the part %v", p)
	}
	if _, spent := a.rows(t); spent != 1 {
		t.Errorf("spent = %d, want 1", spent)
	}
}

// Two spells at once (two rows on the caster) both add their damage to the hit and both are spent by it.
func TestAHitSpellTwoSimultaneousRowsAddUp(t *testing.T) {
	t.Parallel()
	noConc := func(s *rulesv1.TableSpell) { s.Concentration = false }
	a := newHitParty(t, hitParty{spells: []hitSpellOf{
		{name: "Marca de Nanquim", key: tableMark, shape: noConc, mods: []*tableMod{hitMod(trigNext, func(m *tableMod) { m.Dice = 2 })}},
		{name: "Selo de Nanquim", key: sealKey, shape: noConc, mods: []*tableMod{hitMod(trigNext, func(m *tableMod) { m.Die, m.DamageType = 6, "damage-type:fire" })}},
	}})
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	e = a.nextTurn(t, e)
	a.mustCast(t, a.ana, e, "Pensantus", sealKey, slotOfLevel(1), a.at(t), noCastRoll)
	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	var autos []*playv1.DamagePart
	for _, d := range hit.GetParts() {
		if d.GetAuto() && d.GetKey() != extraWeaponDamage {
			autos = append(autos, d)
		}
	}
	if len(autos) != 2 {
		t.Fatalf("the hit offers %d automatic parts of spells, want 2: %v", len(autos), hit.GetParts())
	}
	dmg := a.rollDamage(t, hit, 1, 2, 3, 4) // the weapon, then Marca (2d8: 2 and 3), then Selo (1d6: 4)
	sum := map[string]int32{}
	for _, r := range dmg.GetPartRolls() {
		sum[r.GetPartKey()] = r.GetSum()
	}
	if sum[tableMark] != 5 || sum[sealKey] != 4 {
		t.Errorf("the parts rolled %v, want Marca 5 and Selo 4", sum)
	}
	if n := a.count(t, `SELECT count(*) FROM combatant_states WHERE source_key IN ($1, $2) AND spent_at IS NOT NULL`, tableMark, sealKey); n != 2 {
		t.Errorf("%d rows spent, want both", n)
	}
}

// Another caster's row is not used by a hit of someone else: Toren hits with a row of Pensantus on the field.
func TestAHitSpellAnotherCastersRowIsNotUsed(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	e = a.nextTurn(t, e)
	e = a.passTo(t, e, "Toren")
	hit := a.mustAttack(t, a.caio, e, "Toren", battleaxe, "Capitão Goblin", d20(19)).GetPendingDamage()
	if effectPartOf(hit) != nil {
		t.Errorf("Toren's hit offers the part %v of Pensantus's spell", effectPartOf(hit))
	}
	a.h.roller.queue(3)
	a.mustDamage(t, a.caio, a.get(t, a.master), hit.GetId(), inAppDamage)
	if _, spent := a.rows(t); spent != 0 {
		t.Errorf("Toren's hit used the effect of Pensantus")
	}
}

// A next_melee_hit is not used by a handaxe thrown at range (a ranged attack): no part, nothing spent.
func TestAHitSpellNextMeleeHitIsNotUsedByAThrownWeaponAtRange(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigMelee))
	e := a.start(t, plan{
		npcs:     []*playv1.Participant{{CharacterId: a.capitao.GetId()}, {CharacterId: a.goblin.GetId()}},
		npcRolls: []int{1, 1},
		players:  map[string]int32{"Pensantus": 20, "Toren": 10, "Brisa": 5},
		reveal:   []string{"Capitão Goblin", "Goblin"},
		at:       map[string][2]int32{"Pensantus": {3, 3}, "Capitão Goblin": {8, 3}, "Goblin": {4, 3}, "Toren": {3, 4}, "Brisa": {3, 5}},
	})
	a.castMark(t, e, 1)
	thrown := a.strike(t, e, handaxeKey, "Capitão Goblin", func(r *playv1.RollAttackRequest) { r.Roll, r.D20Faces = nil, []int32{19, 19} }) // at long range: disadvantage
	wantOffHitPart(t, "the thrown handaxe at 25 ft", thrown)
	a.rollDamage(t, thrown, 3)
	if _, spent := a.rows(t); spent != 0 {
		t.Errorf("a thrown handaxe at range used a next_melee_hit")
	}
	e = a.nextTurn(t, e)
	near := a.strike(t, e, handaxeKey, "Goblin", d20(19)) // the same weapon at 5 ft is a melee hit
	wantHitPart(t, "the handaxe swung at 5 ft", near, true)
}

// A spell attack is not a weapon hit: the part is not offered and the effect is not used.
func TestAHitSpellASpellAttackDoesNotOfferThePart(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	bolt := a.strike(t, e, fireBolt, "Capitão Goblin", d20(19))
	if bolt.GetId() == "" {
		t.Fatalf("the Fire Bolt did not hit")
	}
	if p := hitPart(bolt); p != nil {
		t.Errorf("a spell attack offers the part %v", p)
	}
	a.rollDamage(t, bolt, 4)
	if _, spent := a.rows(t); spent != 0 {
		t.Errorf("a spell attack spent the effect")
	}
}

// The undo of the damage whose save failed takes the condition away with the spent mark: the target is clear and the
// effect is back, and the damage rolled again with a save that passes leaves it clear.
func TestAHitSpellUndoAfterAFailedSaveTakesTheConditionAway(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	hit := a.strike(t, e, battleaxe, "Capitão Goblin", d20(19))
	a.rollDamage(t, hit, 1, 1, 2)
	if !a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Fatalf("the failed save gave no condition")
	}
	a.undoLast(t, e)
	if a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("the target is still frightened after the undo of the damage")
	}
	if n := a.count(t, `SELECT count(*) FROM combatant_states WHERE source_key = $1 AND cardinality(condition_keys) > 0`, tableMark); n != 0 {
		t.Errorf("%d condition rows left after the undo", n)
	}
	if lines := a.hitRiderLines(t, a.master, e); len(lines) != 0 {
		t.Errorf("the save's line stays in the log after the undo (%d)", len(lines))
	}
	if _, spent := a.rows(t); spent != 0 {
		t.Errorf("the effect is still spent after the undo")
	}
	a.rollDamage(t, hit, 1, 1, 19)
	if a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("the target is frightened by a save that passed")
	}
}

// A second failed save of the casting replaces the conditions of the first; the master's undo of the second damage takes
// the new conditions away and puts the first hit's back, with their own row.
func TestAHitSpellUndoOfASecondFailedSaveRestoresTheFirstHitsCondition(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) {
		m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd
	}))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 2)
	const conditionRow = `SELECT coalesce(string_agg(s.id::text, ','), '') FROM combatant_states s JOIN combatants c ON c.id = s.combatant_id
		WHERE s.source_key = $1 AND c.label = 'Capitão Goblin' AND cardinality(s.condition_keys) > 0`
	var first string
	if err := a.h.pool.QueryRow(t.Context(), conditionRow, tableMark).Scan(&first); err != nil || first == "" || strings.Contains(first, ",") {
		t.Fatalf("after the first failed save the condition rows are %q (%v), want one", first, err)
	}
	e = a.nextTurn(t, e)
	a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 3)
	var second string
	if err := a.h.pool.QueryRow(t.Context(), conditionRow, tableMark).Scan(&second); err != nil || second == "" || second == first || strings.Contains(second, ",") {
		t.Fatalf("after the second failed save the condition rows are %q (%v), want the replacing one, not %q", second, err, first)
	}
	a.undoLast(t, e)
	var after string
	if err := a.h.pool.QueryRow(t.Context(), conditionRow, tableMark).Scan(&after); err != nil || after != first {
		t.Errorf("after the undo of the second damage the condition rows are %q (%v), want the first hit's %q back", after, err, first)
	}
	if !a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("the target lost the first hit's condition with the undo of the second damage")
	}
}

// The master cannot take the spell's part out of a rolled damage (it is an automatic line, not an extra): the effect
// stays spent and the save stays made, and the way back is the undo of the damage.
func TestAHitSpellItsPartCannotBeRemovedFromARolledDamage(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	rolled := a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 2)
	_, err := a.master.combat.RemoveDamagePart(t.Context(), connect.NewRequest(&playv1.RemoveDamagePartRequest{
		CampaignId: a.campaignID, EncounterId: e.GetId(), PendingDamageId: rolled.GetId(), PartKey: tableMark, Reason: "motivo", IdempotencyKey: newKey(),
	}))
	wantBlockedBy(t, "RemoveDamagePart(the spell's part)", err, playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_DAMAGE_PART_NOT_REMOVABLE)
	if _, spent := a.rows(t); spent != 1 || !a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("the refused removal changed the effect: spent %d, frightened %v", spent, a.conditioned(t, "Capitão Goblin", frightenedKey))
	}
	a.undoLast(t, e)
	if _, spent := a.rows(t); spent != 0 || a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("the undo left the effect spent (%d) or the target frightened", spent)
	}
}

// A recast of the spell ends the old casting: with condition_duration spell_end the condition of the old casting goes with it.
func TestAHitSpellRecastEndsTheConditionOfTheOldCasting(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigEach, func(m *tableMod) {
		m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd
	}))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 2)
	if !a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Fatalf("the failed save gave no condition")
	}
	e = a.nextTurn(t, e)
	if !a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Fatalf("the condition ended with the target's turn, want it to last as the spell")
	}
	a.castMark(t, e, 2)
	if a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("the condition of the old casting outlived the recast")
	}
	if all, _ := a.rows(t); all != 1 {
		t.Errorf("after the recast: %d rows, want the new casting's one", all)
	}
}

// The marked creature dies: the spell stands on the caster, a hit on another creature carries no part, and the mark moves
// with a recast.
func TestAHitSpellTheMarkedTargetDies(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, markShape, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	if _, err := a.adjustHP(t, e, "Goblin", damageHP(100)); err != nil {
		t.Fatalf("AdjustCombatantHitPoints() error = %v", err)
	}
	wantHitPart(t, "a hit on another creature", a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), false)
	e = a.nextTurn(t, e)
	a.castMark(t, e, 1, "Capitão Goblin")
	wantHitPart(t, "a hit on the new mark", a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), true)
}

// An effect with both blocks, what it does to its targets (a +2 to the attack rolls) and the damage it adds to the
// caster's hits: the target gets its effect and not the hit modifier, the caster's row has the hit modifier alone, one
// concentration ends both, and the undo of the cast takes both away.
func TestAHitSpellBothBlocksSplitBetweenTheTargetsAndTheCaster(t *testing.T) {
	t.Parallel()
	bonus := &tableMod{
		Kind: rulesv1.TableEffectModifierKind_TABLE_EFFECT_MODIFIER_KIND_ROLL_BONUS, Value: 2,
		AppliesTo: []rulesv1.TableEffectRollKind{rulesv1.TableEffectRollKind_TABLE_EFFECT_ROLL_KIND_ATTACK},
	}
	a := newHitCasters(t, markShape, bonus, hitMod(trigMarked))
	e := a.hitFight(t)
	a.castMark(t, e, 1, "Goblin")
	if all, _ := a.rows(t); all != 2 || a.rowsOn(t, "Pensantus") != 1 || a.rowsOn(t, "Goblin") != 1 {
		t.Fatalf("after the cast: %d rows (caster %d, goblin %d), want one on each", all, a.rowsOn(t, "Pensantus"), a.rowsOn(t, "Goblin"))
	}
	kinds := func(label string) []string {
		rows, err := a.h.pool.Query(t.Context(), `SELECT m->>'kind' FROM combatant_states s JOIN combatants c ON c.id = s.combatant_id, jsonb_array_elements(s.modifiers) m WHERE s.source_key = $1 AND c.label = $2 ORDER BY 1`, tableMark, label)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				t.Fatalf("scan: %v", err)
			}
			out = append(out, k)
		}
		return out
	}
	if got := kinds("Goblin"); !slices.Equal(got, []string{rules.ModifierRollBonus}) {
		t.Errorf("the goblin's modifiers = %v, want the roll bonus alone", got)
	}
	if got := kinds("Pensantus"); !slices.Equal(got, []string{rules.ModifierWeaponHitDamage}) {
		t.Errorf("the caster's modifiers = %v, want the hit damage alone", got)
	}
	wantHitPart(t, "a hit on the marked goblin", a.strike(t, e, battleaxe, "Goblin", d20(19)), true)
	a.dropConcentration(t, e)
	if all, _ := a.rows(t); all != 0 {
		t.Errorf("after the concentration: %d rows, want none", all)
	}
	e = a.nextTurn(t, e)
	a.castMark(t, e, 1, "Goblin")
	a.undoLast(t, e)
	if all, _ := a.rows(t); all != 0 {
		t.Errorf("after the undo of the cast: %d rows, want none", all)
	}
}

// The conditions of a failed save last until the end of the target's next turn: a target that has not acted this round
// loses them at the end of its turn of this round; one that has acted keeps them through the next round's turn.
func TestAHitSpellConditionLastsUntilTheEndOfTheTargetsNextTurn(t *testing.T) {
	t.Parallel()
	t.Run("a target that has not acted yet", func(t *testing.T) {
		t.Parallel()
		a := newHitCasters(t, nil, hitMod(trigNext, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }))
		e := a.hitFight(t)
		a.castMark(t, e, 1)
		a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 2)
		e = a.passTo(t, e, "Capitão Goblin")
		if !a.conditioned(t, "Capitão Goblin", frightenedKey) {
			t.Fatalf("the condition ended before the end of the target's turn")
		}
		a.passTo(t, e, "Pensantus") // the target's turn (the NPCs' together) ended, and the round is the next
		if a.conditioned(t, "Capitão Goblin", frightenedKey) || a.get(t, a.master).GetRound() != 2 {
			t.Errorf("the condition outlived the end of the target's turn of this round")
		}
	})
	t.Run("a target that has already acted", func(t *testing.T) {
		t.Parallel()
		a := newHitCasters(t, nil, hitMod(trigNext, func(m *tableMod) { m.Save, m.OnFailConditions = "wis", []string{frightenedKey} }))
		e := a.start(t, plan{
			npcs: []*playv1.Participant{{CharacterId: a.capitao.GetId()}, {CharacterId: a.goblin.GetId()}}, npcRolls: []int{18, 17},
			players: map[string]int32{"Pensantus": 5, "Toren": 4, "Brisa": 3}, reveal: []string{"Capitão Goblin", "Goblin"}, theatre: true,
		})
		e = a.passTo(t, e, "Pensantus")
		a.castMark(t, e, 1)
		a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 2)
		e = a.passTo(t, e, "Capitão Goblin") // the next round's turn
		if !a.conditioned(t, "Capitão Goblin", frightenedKey) {
			t.Fatalf("the condition ended with the round: the target has not had its next turn")
		}
		_ = a.passTo(t, e, "Pensantus")
		if a.conditioned(t, "Capitão Goblin", frightenedKey) || a.get(t, a.master).GetRound() != 2 {
			t.Errorf("the condition outlived the end of the target's next turn")
		}
	})
}

// With condition_duration spell_end the conditions last as the spell does: through the target's turns, until the
// concentration ends.
func TestAHitSpellConditionSpellEndLastsWithTheConcentration(t *testing.T) {
	t.Parallel()
	a := newHitCasters(t, nil, hitMod(trigNext, func(m *tableMod) {
		m.Save, m.OnFailConditions, m.ConditionDuration = "wis", []string{frightenedKey}, spellEnd
	}))
	e := a.hitFight(t)
	a.castMark(t, e, 1)
	a.rollDamage(t, a.strike(t, e, battleaxe, "Capitão Goblin", d20(19)), 1, 1, 2)
	e = a.nextTurn(t, e) // the target's turn went by
	if !a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Fatalf("the condition ended with the target's turn, want it to last as the spell")
	}
	a.dropConcentration(t, e)
	if a.conditioned(t, "Capitão Goblin", frightenedKey) {
		t.Errorf("the condition outlived the concentration")
	}
}

// ---- what no database is needed for: the effect of an SRD spell goes through the same functions ----

// An effect built like an SRD one (a smite: the next melee hit, concentration, a save with a condition) and a table's
// both go through the same functions: its part is the caster's row's, its other block is the targets'.
func TestAHitEffectOfTheSRDShapeSplitsAndOffersItsPart(t *testing.T) {
	t.Parallel()
	hit := rules.EffectModifier{
		Kind: rules.ModifierWeaponHitDamage, Trigger: rules.HitTriggerNextMeleeHit, Die: 6, Dice: 2, PerSlotDice: 1, MaxDice: 6,
		DamageType: "damage-type:fire", Save: "con", OnFailConditions: []string{"condition:blinded"},
	}
	bonus := rules.EffectModifier{Kind: rules.ModifierRollBonus, Value: 1, AppliesTo: []string{rules.RollAppliesAttack}}
	def := &rules.EffectDef{Key: "spell:searing-smite", NamePT: "Golpe Ardente", Kind: "spell", Concentration: true, Modifiers: []rules.EffectModifier{bonus, hit}}
	def.TagsPT = rules.EffectTagsPT(def.Modifiers)
	side := targetSideOf(def)
	if side == nil || len(side.Modifiers) != 1 || side.Modifiers[0].Kind != rules.ModifierRollBonus {
		t.Fatalf("the target side = %+v, want the roll bonus alone", side)
	}
	if only := (&rules.EffectDef{Modifiers: []rules.EffectModifier{hit}}); targetSideOf(only) != nil {
		t.Errorf("an effect that is only the hit modifier has a target side")
	}
	if got := hitModifiersAtSlot(onlyHitModifiers(def.Modifiers), 2); len(got) != 1 || got[0].Dice != 4 || got[0].PerSlotDice != 0 {
		t.Errorf("the caster's modifiers at 2 levels above = %+v, want 4d6 with no per-slot dice", got)
	}
	if tags := tagsOfRow(def, []rules.EffectModifier{bonus}); slices.ContainsFunc(tags, func(s string) bool { return strings.Contains(s, "golpe") }) {
		t.Errorf("a target reads the tags %q of the damage the caster adds", tags)
	}
	if tags := tagsOfRow(def, []rules.EffectModifier{hit}); len(tags) != 1 || !strings.Contains(tags[0], "golpe corpo a corpo") {
		t.Errorf("the caster's row reads the tags %q, want the hit damage's alone", tags)
	}
	row := playdb.CombatantState{ID: "row-1", SourceKey: new("spell:searing-smite"), PlayerVisible: false, Audience: "all"}
	mods := hitModifiersAtSlot(onlyHitModifiers(def.Modifiers), 0)
	hf := hitFacts{
		attacker: playdb.Combatant{ID: "a"}, target: playdb.Combatant{ID: "t"}, attack: linkMeleeAttack(),
		states: map[string][]playdb.CombatantState{"a": {withMods(row, mods)}}, names: func(string) string { return "Golpe Ardente" },
	}
	parts := hf.hitEffectParts()
	if len(parts) != 1 || parts[0].Count != 2 || parts[0].Sides != 6 || !parts[0].Hide || parts[0].Label != "Golpe Ardente" || parts[0].Effect != "row-1" {
		t.Errorf("the SRD-shaped effect's part = %+v, want 2d6, hidden from the others, named", parts)
	}
	if k, label, src := hitPartForOthers(parts[0], parts[0].Label, parts[0].Source); k != "row-1" || label != otherSourceTx || src != "" {
		t.Errorf("the others read %q %q %q, want the row's id and Outra fonte", k, label, src)
	}
}

func linkMeleeAttack() link.Attack { return link.Attack{Weapon: true, Melee: true, RangeFt: 5} }

// withMods is the row with the modifiers an effect row keeps.
func withMods(row playdb.CombatantState, mods []rules.EffectModifier) playdb.CombatantState {
	body, _ := json.Marshal(mods)
	row.Modifiers, row.Kind = body, stateEffect
	return row
}

// The part of a spell's hit as the players who are not the attacker's read it: no dice, and "Outra fonte" with no key
// of the spell when the effect is not theirs; the attacker's player and the master read it whole.
func TestAHitPartAsOthersReadIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		hide         bool
		label, key   string
		wantOthers   string
		wantKeyOther string
	}{
		{"a public effect", false, "Marca de Nanquim", tableMark, "Marca de Nanquim", tableMark},
		{"a hidden effect", true, "Marca de Nanquim", tableMark, otherSourceTx, "row-1"},
	} {
		part := partRecord{Key: tc.key, Label: tc.label, Kind: partAuto, Count: 7, Sides: 12, DamageType: "damage-type:fire", Source: tc.key, Available: true, Table: tableHitEffect, Effect: "row-1", Hide: tc.hide}
		roll := partRoll{Key: tc.key, Label: tc.label, Count: 7, Sides: 12, Faces: []int32{1, 2, 3, 4, 5, 6, 7}, Sum: 28, Counted: true, Effect: "row-1", Hide: tc.hide}
		p := playdb.PendingDamage{Parts: mustJSON([]partRecord{part}), PartRolls: mustJSON([]partRoll{roll})}
		whole, others := partsProtoFor(p, combat.CriticalDoubledDice, dtPT, true)[0], partsProtoFor(p, combat.CriticalDoubledDice, dtPT, false)[0]
		if whole.GetLabelPt() != tc.label || whole.GetDiceCount() != 7 || whole.GetDiceSides() != 12 || whole.GetKey() != tc.key {
			t.Errorf("%s: the owner reads %v, want it whole", tc.name, whole)
		}
		if others.GetLabelPt() != tc.wantOthers || others.GetDiceCount() != 0 || others.GetDiceSides() != 0 || others.GetKey() != tc.wantKeyOther || (tc.hide && others.GetSourceKey() != "") {
			t.Errorf("%s: the others read %v", tc.name, others)
		}
		seen := partRollsProto(p, false, false, dtPT)[0]
		if seen.GetLabelPt() != tc.wantOthers || seen.GetDiceCount() != 0 || len(seen.GetFaces()) != 0 || seen.GetSum() != 28 || seen.GetPartKey() != tc.wantKeyOther {
			t.Errorf("%s: the others read the roll %v", tc.name, seen)
		}
		if mine := partRollsProto(p, true, true, dtPT)[0]; mine.GetDiceCount() != 7 || len(mine.GetFaces()) != 7 || mine.GetLabelPt() != tc.label {
			t.Errorf("%s: the attacker's side reads the roll %v, want it whole", tc.name, mine)
		}
	}
}

// The flag of a hit effect on a part's roll follows the part's key (RN-10): it is true only for a reader who reads the
// spell's key (the attacker's player, the master, and the other players when the effect is public). A reader whose key is
// the row's id reads false, so the flag never says a spell is behind the part. The weapon's own part never has it.
func TestRN10_TheHitEffectFlagFollowsTheKeyTheReaderReads(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		hide            bool
		othersKey       string
		othersHitEffect bool
		ownersHitEffect bool
	}{
		{"a public effect", false, tableMark, true, true},
		{"a hidden effect", true, "row-1", false, true},
	} {
		weapon := partRoll{Key: partWeapon, Label: "Machado", Count: 1, Sides: 8, Faces: []int32{5}, Sum: 5, Counted: true}
		hit := partRoll{Key: tableMark, Label: "Marca de Nanquim", Count: 7, Sides: 12, Faces: []int32{1, 2, 3, 4, 5, 6, 7}, Sum: 28, Counted: true, Effect: "row-1", Hide: tc.hide}
		p := playdb.PendingDamage{PartRolls: mustJSON([]partRoll{weapon, hit}), Parts: mustJSON([]partRecord{{Key: partWeapon, Kind: partWeapon}, {Key: tableMark, Kind: partAuto, Table: tableHitEffect, Effect: "row-1", Hide: tc.hide}})}
		for _, r := range []struct {
			who           string
			master, reads bool
			wantKey       string
			want          bool
		}{
			{"the attacker's player", false, true, tableMark, tc.ownersHitEffect},
			{"the master", true, true, tableMark, tc.ownersHitEffect},
			{"another player", false, false, tc.othersKey, tc.othersHitEffect},
		} {
			rolls := partRollsProto(p, r.master, r.reads, dtPT)
			if len(rolls) != 2 {
				t.Fatalf("%s, %s: %d rolls, want 2", tc.name, r.who, len(rolls))
			}
			if got := rolls[1]; got.GetPartKey() != r.wantKey || got.GetHitEffect() != r.want {
				t.Errorf("%s, %s: reads key %q hit_effect %v, want %q and %v", tc.name, r.who, got.GetPartKey(), got.GetHitEffect(), r.wantKey, r.want)
			}
			if rolls[0].GetHitEffect() {
				t.Errorf("%s, %s: the weapon's roll reads hit_effect", tc.name, r.who)
			}
		}
		if got := partRollProto(hit, dtPT); !got.GetHitEffect() { // the log's damage parts, read by the master and the attacker's player
			t.Errorf("%s: the log's part of the effect reads hit_effect false", tc.name)
		}
		if got := partRollProto(weapon, dtPT); got.GetHitEffect() {
			t.Errorf("%s: the log's weapon part reads hit_effect", tc.name)
		}
	}
}
