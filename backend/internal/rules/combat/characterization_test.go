package combat

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"hash"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

var updateCombatSweep = flag.Bool("update", false, "rewrite testdata/*.sha256 from the current code")

// checkCombatSweep compares the digest of what write prints with the one in
// testdata/<name>.sha256 (the full text is far too long to keep).
func checkCombatSweep(t *testing.T, name string, write func(w io.Writer)) {
	t.Helper()
	w := &combatDigest{h: sha256.New()}
	write(w)
	got := fmt.Sprintf("%s %d\n", hex.EncodeToString(w.h.Sum(nil)), w.lines)
	path := filepath.Join("testdata", name+".sha256")
	if *updateCombatSweep {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if got != string(want) {
		t.Errorf("the %s sweep changed: digest %q, want %q", name, strings.TrimSpace(got), strings.TrimSpace(string(want)))
	}
}

type combatDigest struct {
	h     hash.Hash
	lines int
}

func (d *combatDigest) Write(p []byte) (int, error) {
	d.lines += strings.Count(string(p), "\n")
	return d.h.Write(p)
}

// sweepConditions are every condition the modes read, and one they do not.
var sweepConditions = []string{
	conditionProne, conditionRestrained, conditionBlinded, conditionDeafened, conditionPoisoned,
	conditionFrightened, conditionInvisible, conditionStunned, conditionParalyzed, conditionUnconscious,
	conditionPetrified, conditionIncapacitated, conditionGrappled, "condition:charmed",
}

// randomCreature draws a creature with each condition, flag and list on or off
// at random, weighted so that most have few conditions and every
// combination of two or three comes up.
func randomCreature(r *rand.Rand) Creature {
	var c Creature
	for _, cond := range sweepConditions {
		if r.IntN(6) == 0 {
			c.Conditions = append(c.Conditions, cond)
		}
	}
	c.Dodging, c.Reckless, c.PackTactics = r.IntN(2) == 0, r.IntN(3) == 0, r.IntN(3) == 0
	c.DangerSense, c.Raging, c.Outlined = r.IntN(2) == 0, r.IntN(3) == 0, r.IntN(4) == 0
	c.Exhaustion = r.IntN(5)
	abilities := []string{"str", "dex", "con", "wis"}
	if r.IntN(3) == 0 {
		c.SaveAdvantage = []string{abilities[r.IntN(len(abilities))]}
	}
	if r.IntN(3) == 0 {
		c.CheckAdvantage = []string{abilities[r.IntN(len(abilities))]}
	}
	if r.IntN(4) == 0 {
		c.AttackDisadvantage = "spell:vicious-mockery"
	}
	return c
}

// TestAttackModeSweepCharacterization freezes AttackMode, the very sources in
// their order, for seeded scenes with every field drawn.
func TestAttackModeSweepCharacterization(t *testing.T) {
	t.Parallel()
	const scenes = 60000
	r := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed: the same scenes on every run
	distances := []int{0, 5, 10, 30, 80, 120, 200}
	checkCombatSweep(t, "attackmode_sweep", func(w io.Writer) {
		for range scenes {
			s := AttackScene{
				Attacker: randomCreature(r), Target: randomCreature(r),
				Ranged: r.IntN(2) == 0, StrengthMelee: r.IntN(2) == 0, OwnTurn: r.IntN(2) == 0,
				DistanceKnown: r.IntN(3) != 0, DistanceFt: distances[r.IntN(len(distances))],
				ReachFt: []int{0, 5, 10, 80}[r.IntN(4)], LongRangeFt: []int{0, 120, 320}[r.IntN(3)],
				AttackerUnseen: r.IntN(3) == 0, TargetUnseen: r.IntN(3) == 0, Helped: r.IntN(4) == 0,
				AllyNearTarget: r.IntN(2) == 0, HostileNearAttacker: r.IntN(3) == 0, NoLongRangeDisadvantage: r.IntN(4) == 0,
			}
			fmt.Fprintf(w, "%+v\n", AttackMode(s)) // the scenes are the same on every run: only the answer is written
		}
	})
}

// TestSaveModeSweepCharacterization freezes SaveMode, the very sources in their
// order, for seeded creatures, each ability, save and check.
func TestSaveModeSweepCharacterization(t *testing.T) {
	t.Parallel()
	const creatures = 6000
	r := rand.New(rand.NewPCG(3, 4)) //nolint:gosec // a fixed seed: the same scenes on every run
	checkCombatSweep(t, "savemode_sweep", func(w io.Writer) {
		for range creatures {
			c := randomCreature(r)
			for _, ability := range []string{"str", "dex", "con", "wis", ""} {
				for _, check := range []bool{false, true} {
					s := SaveScene{Creature: c, Ability: ability, Check: check, EffectVisible: r.IntN(2) == 0, StealthArmor: r.IntN(2) == 0}
					fmt.Fprintf(w, "%s %v %+v\n", ability, check, SaveMode(s))
				}
			}
		}
	})
}

// TestBonusAttackExhaustiveCharacterization freezes BonusAttack for every
// combination of the turn and of the two attacks.
func TestBonusAttackExhaustiveCharacterization(t *testing.T) {
	t.Parallel()
	traits := func(bits int) AttackTraits {
		return AttackTraits{Spell: bits&1 != 0, Melee: bits&2 != 0, Light: bits&4 != 0, Unarmed: bits&8 != 0, MartialArts: bits&16 != 0}
	}
	const traitCombos, turnCombos = 1 << 5, 1 << 4
	checkCombatSweep(t, "bonusattack_sweep", func(w io.Writer) {
		for tb := range turnCombos {
			for _, flurry := range []int{0, 1, 2} {
				for last := range traitCombos {
					for next := range traitCombos {
						turn := BonusAttackTurn{
							AttackAction: tb&1 != 0, BonusUsed: tb&2 != 0, NoSecondLight: tb&4 != 0, FrenzyReady: tb&8 != 0,
							FlurryLeft: flurry, Last: traits(last),
						}
						fmt.Fprintf(w, "%+v %+v => %d\n", turn, traits(next), BonusAttack(turn, traits(next)))
					}
				}
			}
		}
	})
}

// optionsSheet is a sheet with one of each kind of attack, spell and action
// Options reads: two light weapons, a heavy one, the unarmed strike of a
// monk, a cantrip of three beams; cantrips and spells of each economy, a
// Shield, a spell of a minute, an unprepared one and a cantrip that is also an
// attack; slots of the 1st and 2nd levels and a pact slot of the 2nd;
// Flurry of Blows, Action Surge and a feature with a resource.
func optionsSheet(perAction int, twoWeapon, pact bool) rules.Derived {
	ct := func(unit string) rules.CastingTime { return rules.CastingTime{Amount: 1, Unit: unit} }
	d := rules.Derived{
		SpeedWalkFt: 30, AttacksPerAction: perAction, TwoWeaponFighting: twoWeapon,
		SpellSlots: []int{2, 1},
		Attacks: []rules.Attack{
			{Key: "weapon:dagger", Kind: "weapon", Melee: true, Light: true, AbilityMod: 3, DamageDice: rules.DiceFormula{Count: 1, Sides: 4, Bonus: 3}, Damage: "1d4+3"},
			{Key: "weapon:shortsword", Kind: "weapon", Melee: true, Light: true, MartialArts: true, AbilityMod: -1, DamageDice: rules.DiceFormula{Count: 1, Sides: 6, Bonus: -1}, Damage: "1d6-1"},
			{Key: "weapon:greataxe", Kind: "weapon", Melee: true, AbilityMod: 2, DamageDice: rules.DiceFormula{Count: 1, Sides: 12, Bonus: 2}, Damage: "1d12+2"},
			{Key: rules.UnarmedStrikeKey, Kind: "weapon", Melee: true, MartialArts: true, DamageDice: rules.DiceFormula{Count: 1, Sides: 4}, Damage: "1d4"},
			{Key: "spell:eldritch-blast", Kind: "spell", Beams: 3, DamageDice: rules.DiceFormula{Count: 1, Sides: 10}, Damage: "1d10"},
			{Key: "spell:fire-bolt", Kind: "spell", Beams: 1, DamageDice: rules.DiceFormula{Count: 1, Sides: 10}, Damage: "1d10"},
		},
		Spells: []rules.CharacterSpell{
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:fire-bolt", NamePT: "Raio de Fogo", CastingTime: ct(rules.CastAction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:minor-illusion", NamePT: "Ilusão Menor", CastingTime: ct(rules.CastAction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:shillelagh", Name: "Shillelagh", CastingTime: ct(rules.CastBonusAction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:magic-missile", NamePT: "Mísseis Mágicos", Level: 1, CastingTime: ct(rules.CastAction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:healing-word", NamePT: "Palavra Curativa", Level: 1, CastingTime: ct(rules.CastBonusAction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: shieldKey, NamePT: "Escudo Arcano", Level: 1, CastingTime: ct(rules.CastReaction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:feather-fall", NamePT: "Queda Suave", Level: 1, CastingTime: ct(rules.CastReaction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:alarm", NamePT: "Alarme", Level: 1, CastingTime: ct(rules.CastMinute)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:misty-step", NamePT: "Passo Nebuloso", Level: 2, CastingTime: ct(rules.CastBonusAction)}},
			{Prepared: true, Spell: rules.SpellEntry{Key: "spell:fireball", NamePT: "Bola de Fogo", Level: 3, CastingTime: ct(rules.CastAction)}},
			{Prepared: false, Spell: rules.SpellEntry{Key: "spell:sleep", NamePT: "Sono", Level: 1, CastingTime: ct(rules.CastAction)}},
		},
		Resources: []rules.Resource{{Key: "ki", Max: 2, Recharge: "short_rest"}, {Key: actionSurgeResource, Max: 1, Recharge: "short_rest"}, {Key: "rage", Max: 2, Recharge: "long_rest"}},
		StandardActions: []rules.Action{
			{Key: "action:dash", Economy: rules.EconomyAction},
			{Key: "action:disengage", Economy: rules.EconomyBonusAction},
			{Key: "action:opportunity", Economy: rules.EconomyReaction},
			{Key: "action:drop", Economy: rules.EconomyFree},
		},
		Actions: []rules.Action{
			{Key: FlurryOfBlowsKey, Economy: rules.EconomyBonusAction, Resource: "ki"},
			{Key: "feature:action-surge", Economy: rules.EconomyFree, Resource: actionSurgeResource},
			{Key: "feature:rage", Economy: rules.EconomyBonusAction, Resource: "rage"},
			{Key: "feature:second-wind", Economy: rules.EconomyBonusAction, Resource: "second_wind"},
		},
	}
	if pact {
		d.PactMagic = &rules.PactMagic{SlotLevel: 2, Slots: 1}
	}
	return d
}

// TestOptionsSweepCharacterization freezes Options (so spellOption,
// bonusAttackOption, flurryOption and actionOption) for every combination of
// the turn's flags, attacks made, last attack and flurry left, on two pairs of
// a sheet and a usage (the Dash, which only doubles the speed, every fourth
// turn).
func TestOptionsSweepCharacterization(t *testing.T) {
	t.Parallel()
	const flagCombos = 1 << 7
	spent := Usage{SlotsUsed: [9]int{2, 1}, PactSlotsUsed: 1, ResourcesUsed: map[string]int{"ki": 2, actionSurgeResource: 1, "rage": 2}}
	cases := []struct {
		d rules.Derived
		u Usage
	}{{optionsSheet(1, false, false), spent}, {optionsSheet(2, true, true), Usage{}}}
	lastKeys := []string{"", "weapon:dagger", "weapon:shortsword", rules.UnarmedStrikeKey, "spell:eldritch-blast", ContestAttackKey, "weapon:lost"}
	checkCombatSweep(t, "options_sweep", func(w io.Writer) {
		n := 0
		for ci, c := range cases {
			for f := range flagCombos {
				for _, made := range []int{0, 1, 2} {
					for _, last := range lastKeys {
						for _, flurry := range []int{0, 2} {
							n++
							turn := TurnState{
								ActionUsed: f&1 != 0, BonusActionUsed: f&2 != 0, ReactionUsed: f&4 != 0, Dashed: n%4 == 0,
								SpellCast: f&8 != 0, BonusSpellCast: f&16 != 0, ActionSurged: f&32 != 0, FrenzyReady: f&64 != 0,
								MovementUsedFt: 15 * made, AttacksMade: made, LastAttackKey: last, FlurryLeft: flurry,
							}
							fmt.Fprintf(w, "c%d %+v => %s\n", ci, turn, optionsLine(Options(c.d, turn, c.u)))
						}
					}
				}
			}
		}
	})
}

// reasonText writes a reason, which is a pointer.
func reasonText(r *Reason) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%+v", *r)
}

// optionsLine writes what Options decides, in its order: the economy, then
// each attack, spell and action with its option and what goes with it.
func optionsLine(o TurnOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%+v|", o.Economy)
	for _, a := range o.Attacks {
		fmt.Fprintf(&b, "%s:%v:%s:%d:%d:%v:%d:%s:%+v;", a.Attack.Key, a.Enabled, reasonText(a.Reason), a.Bonus, a.FlurryLeft, a.DropsModifier, a.BeamsLeft, a.Attack.Damage, a.Attack.DamageDice)
	}
	b.WriteByte('|')
	for _, sp := range o.Spells {
		fmt.Fprintf(&b, "%s:%v:%s:%s:%+v;", sp.Spell.Key, sp.Enabled, reasonText(sp.Reason), sp.Economy, sp.Slots)
	}
	b.WriteByte('|')
	for _, a := range append(o.StandardActions, o.FeatureActions...) {
		fmt.Fprintf(&b, "%s:%v:%s:%d;", a.Action.Key, a.Enabled, reasonText(a.Reason), a.UsesLeft)
	}
	return b.String()
}
