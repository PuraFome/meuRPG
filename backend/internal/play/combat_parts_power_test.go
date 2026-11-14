package play

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The damage part of a power attack (combat_attack_power.go, hitParts): a flat automatic part of the weapon's type, listed
// when the attack made the trade and hit. These tests need no database.

const (
	powerFeature = "feature:golpe-pesado@mesa"
	powerName    = "Golpe Pesado"
	powerBonus   = 10
)

func powerTrade(hidden bool) *powerAttackEvent {
	return &powerAttackEvent{Key: powerFeature, Name: powerName, Penalty: 5, DamageBonus: powerBonus, Hidden: hidden}
}

// powerFacts is a hit of a player's character with a slashing weapon, with the trade the attack made.
func powerFacts(trade *powerAttackEvent) hitFacts {
	return hitFacts{
		attacker:    playdb.Combatant{ID: "a", Kind: kindPlayer, FeatureState: []byte(`{}`)},
		target:      playdb.Combatant{ID: "t"},
		attack:      link.Attack{Key: "e", Name: "Espada", Weapon: true, Melee: true, DiceCount: 2, DiceSides: 6, DamageType: "damage-type:slashing"},
		weaponBonus: 3,
		names:       func(string) string { return "" },
		power:       trade,
	}
}

func TestAHitThatMadeAPowerAttackListsItsDamageAsAnAutomaticPart(t *testing.T) {
	t.Parallel()
	parts := powerFacts(powerTrade(false)).buildParts()
	if len(parts) != 2 || parts[0].Key != partWeapon || parts[1].Key != partPowerAttack {
		t.Fatalf("parts = %+v, want the weapon then the power attack", parts)
	}
	p := parts[1]
	if p.Kind != partAuto || p.Choosable || !p.Selected || !p.Available || p.Flat != powerBonus || p.Count != 0 || p.Sides != 0 || p.Label != powerName ||
		p.DamageType != "damage-type:slashing" || p.Source != powerFeature || p.Table != tablePowerAttack || p.Hide || p.NoCrit {
		t.Errorf("part = %+v, want an automatic flat %d of the weapon's type, from the feature, never chosen", p, powerBonus)
	}
}

func TestAHitThatMadeNoPowerAttackHasNoPowerPart(t *testing.T) {
	t.Parallel()
	if parts := powerFacts(nil).buildParts(); len(parts) != 0 {
		t.Errorf("parts = %+v, want none: nothing else adds a line to this hit", parts)
	}
	h := powerFacts(nil)
	h.sheet = link.Sheet{Traits: link.Traits{PowerAttacks: []link.PowerAttack{{Key: powerFeature, NamePT: powerName, AttackPenalty: 5, DamageBonus: powerBonus, Weapon: "melee"}}}}
	if parts := h.buildParts(); len(parts) != 0 {
		t.Errorf("parts = %+v, want none: having the feature is not making the trade", parts)
	}
}

func TestAHiddenPowerAttackKeepsItsFlagOnThePart(t *testing.T) {
	t.Parallel()
	parts := powerFacts(powerTrade(true)).buildParts()
	if len(parts) != 2 || !parts[1].Hide || parts[1].Label != powerName {
		t.Errorf("parts = %+v, want the part to say it is hidden and keep the real name for its owner", parts)
	}
}

// A critical hit doubles the dice of the other parts and never the flat number, and the dice of the part are none.
func TestACriticalHitNeverDoublesThePowerAttackBonus(t *testing.T) {
	t.Parallel()
	pd := playdb.PendingDamage{Parts: mustJSON(powerFacts(powerTrade(false)).buildParts()), Critical: true}
	for _, rule := range []combat.CriticalRule{combat.CriticalDoubledDice, combat.CriticalMaxPlusRoll} {
		parts := partsProtoFor(pd, rule, dtPT, true)
		if len(parts) != 2 {
			t.Fatalf("parts = %v, want two", parts)
		}
		if w := parts[0]; w.GetDiceCount() == 2 && rule == combat.CriticalDoubledDice {
			t.Errorf("the weapon's dice = %d on a critical hit, want them doubled", w.GetDiceCount())
		}
		if p := parts[1]; p.GetFlat() != powerBonus || p.GetDiceCount() != 0 || p.GetDoubled() {
			t.Errorf("rule %v: the power part = flat %d, dice %d, doubled %v; want the flat %d once and no dice", rule, p.GetFlat(), p.GetDiceCount(), p.GetDoubled(), powerBonus)
		}
	}
}

// The master and the attacker's player read the feature and the number; another player reads neither the number nor, for a
// feature the master retired, the name (RN-10).
func TestThePowerAttackPartForEachReader(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		hidden     bool
		reads      bool
		label      string
		flat       int32
		source     string
		reasonSeen bool
	}{
		{"the owner, a visible feature", false, true, powerName, powerBonus, powerFeature, true},
		{"the owner, a hidden feature", true, true, powerName, powerBonus, powerFeature, true},
		{"another player, a visible feature", false, false, powerName, 0, "", false},
		{"another player, a hidden feature", true, false, otherSourceTx, 0, "", false},
	} {
		parts := powerFacts(powerTrade(tc.hidden)).buildParts()
		pd := playdb.PendingDamage{Parts: mustJSON(parts)}
		got := partsProtoFor(pd, combat.CriticalDoubledDice, dtPT, tc.reads)[1]
		if got.GetKey() != partPowerAttack || got.GetLabelPt() != tc.label || got.GetFlat() != tc.flat || got.GetSourceKey() != tc.source || (got.GetReasonPt() != "") != tc.reasonSeen {
			t.Errorf("%s: part = key %q, label %q, flat %d, source %q, reason %q; want label %q, flat %d, source %q", tc.name, got.GetKey(), got.GetLabelPt(), got.GetFlat(), got.GetSourceKey(), got.GetReasonPt(), tc.label, tc.flat, tc.source)
		}
		if !got.GetAuto() || got.GetChoosable() {
			t.Errorf("%s: part = %v, want it automatic and not chosen by anyone", tc.name, got)
		}
	}
}

// What the part rolled is read the same way: the roll of the part is the number, for the owner and the master alone.
func TestThePowerAttackRollForEachReader(t *testing.T) {
	t.Parallel()
	roll := partRoll{Key: partPowerAttack, Label: powerName, Flat: powerBonus, DamageType: "damage-type:slashing", Counted: true, Hide: true}
	pd := playdb.PendingDamage{PartRolls: mustJSON([]partRoll{roll})}
	for _, tc := range []struct {
		name          string
		master, reads bool
		label         string
		flat          int32
	}{
		{"the master", true, true, powerName, powerBonus},
		{"the owner", false, true, powerName, powerBonus},
		{"another player", false, false, otherSourceTx, 0},
	} {
		got := partRollsProto(pd, tc.master, tc.reads, dtPT)
		if len(got) != 1 || got[0].GetLabelPt() != tc.label || got[0].GetFlat() != tc.flat || got[0].GetPartKey() != partPowerAttack {
			t.Errorf("%s: roll = %v, want label %q and flat %d", tc.name, got, tc.label, tc.flat)
		}
	}
	roll.Hide = false
	pd = playdb.PendingDamage{PartRolls: mustJSON([]partRoll{roll})}
	if got := partRollsProto(pd, false, false, dtPT); len(got) != 1 || got[0].GetLabelPt() != powerName || got[0].GetFlat() != 0 {
		t.Errorf("another player, a visible feature: roll = %v, want the name and no number", got)
	}
}

// The attack's marker in the log: the owner and the master read all of it, another player the name (or "Outra fonte") alone.
func TestThePowerAttackMarkerOfTheLog(t *testing.T) {
	t.Parallel()
	owner := powerTrade(false).logView(true)
	if owner.GetKey() != powerFeature || owner.GetNamePt() != powerName || owner.GetPenalty() != 5 || owner.GetDamageBonus() != powerBonus || owner.GetHidden() {
		t.Errorf("the owner reads %v, want the feature, its name and both numbers", owner)
	}
	if hiddenOwner := powerTrade(true).logView(true); hiddenOwner.GetNamePt() != powerName || hiddenOwner.GetKey() != powerFeature || hiddenOwner.GetPenalty() != 5 || hiddenOwner.GetHidden() {
		t.Errorf("the owner of a hidden feature reads %v, want the name and the numbers", hiddenOwner)
	}
	other := powerTrade(false).logView(false)
	if other.GetKey() != "" || other.GetNamePt() != powerName || other.GetPenalty() != 0 || other.GetDamageBonus() != 0 || other.GetHidden() {
		t.Errorf("another player reads %v, want the name and no key or number", other)
	}
	hidden := powerTrade(true).logView(false)
	if hidden.GetKey() != "" || hidden.GetNamePt() != otherSourceTx || hidden.GetPenalty() != 0 || hidden.GetDamageBonus() != 0 || !hidden.GetHidden() {
		t.Errorf("another player reads %v of a hidden feature, want %q, hidden and no key or number", hidden, otherSourceTx)
	}
	var none *powerAttackEvent
	if none.logView(true) != nil || none.logView(false) != nil {
		t.Error("an attack that made no trade has a marker")
	}
}
