package combat

import (
	"testing"

	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

func TestBonusAttack(t *testing.T) {
	t.Parallel()
	shortsword := AttackTraits{Melee: true, Light: true}
	longsword := AttackTraits{Melee: true}
	unarmed := AttackTraits{Melee: true, Unarmed: true, MartialArts: true}
	staff := AttackTraits{Melee: true, MartialArts: true}
	cantrip := AttackTraits{Spell: true}
	after := func(last AttackTraits) BonusAttackTurn { return BonusAttackTurn{AttackAction: true, Last: last} }

	for _, tc := range []struct {
		name string
		turn BonusAttackTurn
		next AttackTraits
		want BonusKind
	}{
		{"light weapon after a light weapon", after(shortsword), shortsword, BonusTwoWeapon},
		{"a single light weapon has no off hand", BonusAttackTurn{AttackAction: true, Last: shortsword, NoSecondLight: true}, shortsword, BonusNone},
		{"a heavier weapon off hand", after(shortsword), longsword, BonusNone},
		{"light weapon after a heavier one", after(longsword), shortsword, BonusNone},
		{"no Attack action yet", BonusAttackTurn{Last: shortsword}, shortsword, BonusNone},
		{"the bonus action is spent", BonusAttackTurn{AttackAction: true, BonusUsed: true, Last: shortsword}, shortsword, BonusNone},
		{"unarmed after an unarmed strike", after(unarmed), unarmed, BonusMartialArts},
		{"unarmed after a monk weapon", after(staff), unarmed, BonusMartialArts},
		{"unarmed after another weapon", after(longsword), unarmed, BonusNone},
		{"a monk weapon is no Martial Arts strike", after(unarmed), staff, BonusNone},
		{"flurry strike with the bonus action spent", BonusAttackTurn{AttackAction: true, BonusUsed: true, FlurryLeft: 2, Last: longsword}, unarmed, BonusFlurry},
		{"flurry allows only unarmed strikes", BonusAttackTurn{AttackAction: true, BonusUsed: true, FlurryLeft: 1, Last: longsword}, longsword, BonusNone},
		{"a cantrip is no bonus action attack", after(shortsword), cantrip, BonusNone},
		{"after a cantrip", after(cantrip), shortsword, BonusNone},
	} {
		if got := BonusAttack(tc.turn, tc.next); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestOffHandBonus(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		bonus, mod int
		style      bool
		want       int
	}{
		{"positive modifier is dropped", 5, 3, false, 2},
		{"the style keeps it", 5, 3, true, 5},
		{"a negative modifier stays", 0, -1, false, 0},
		{"a zero modifier", 4, 0, false, 4},
	} {
		if got := OffHandBonus(tc.bonus, tc.mod, tc.style); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

// bonusSheet is a sheet with the attacks the bonus action rules tell apart.
func bonusSheet(perAction int, style bool) rules.Derived {
	dice := func(bonus int) (rules.DiceFormula, string) {
		f := rules.DiceFormula{Count: 1, Sides: 4, Bonus: bonus}
		return f, diceText(f)
	}
	dagger, daggerText := dice(3)
	strike, strikeText := dice(3)
	shortsword := rules.DiceFormula{Count: 1, Sides: 6, Bonus: 3}
	shortswordText := diceText(shortsword)
	return rules.Derived{
		AttacksPerAction: perAction, TwoWeaponFighting: style,
		Attacks: []rules.Attack{
			{Key: "equipment:dagger", Kind: "weapon", Melee: true, Light: true, AbilityMod: 3, DamageDice: dagger, Damage: daggerText},
			{Key: "equipment:longsword", Kind: "weapon", Melee: true},
			// A second light melee weapon: two-weapon fighting needs one in each hand (SRD 5.1).
			{Key: "equipment:shortsword", Kind: "weapon", Melee: true, Light: true, AbilityMod: 3, DamageDice: shortsword, Damage: shortswordText},
			{Key: rules.UnarmedStrikeKey, Kind: "weapon", Melee: true, MartialArts: true, AbilityMod: 3, DamageDice: strike, Damage: strikeText},
			{Key: "spell:fire-bolt", Kind: "spell", Beams: 1},
			{Key: "spell:eldritch-blast", Kind: "spell", Beams: 2},
		},
		Actions:   []rules.Action{{Key: FlurryOfBlowsKey, Economy: rules.EconomyBonusAction, Resource: "ki"}},
		Resources: []rules.Resource{{Key: "ki", Max: 3}},
	}
}

func attackOptionOf(t *testing.T, o TurnOptions, key string) AttackOption {
	t.Helper()
	for _, a := range o.Attacks {
		if a.Attack.Key == key {
			return a
		}
	}
	t.Fatalf("no attack option %s", key)
	return AttackOption{}
}

func TestOptionsOffHandAttackAfterTheAttackAction(t *testing.T) {
	t.Parallel()
	const dagger = "equipment:dagger"
	after := TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: dagger}

	got := attackOptionOf(t, Options(bonusSheet(1, false), after, Usage{}), dagger)
	if !got.Enabled || got.Bonus != BonusTwoWeapon {
		t.Errorf("off hand = enabled %v, rule %d, want enabled with the two weapon rule", got.Enabled, got.Bonus)
	}
	if got.Attack.Damage != "1d4" || got.Attack.DamageDice.Bonus != 0 || !got.DropsModifier {
		t.Errorf("off hand damage = %s, want 1d4 (no ability modifier)", got.Attack.Damage)
	}
	if got := attackOptionOf(t, Options(bonusSheet(1, true), after, Usage{}), dagger); got.Attack.Damage != "1d4+3" || got.DropsModifier {
		t.Errorf("off hand damage with the style = %s, want 1d4+3", got.Attack.Damage)
	}
	if got := attackOptionOf(t, Options(bonusSheet(1, false), after, Usage{}), "equipment:longsword"); got.Enabled || got.Bonus != BonusNone {
		t.Errorf("a longsword off hand = enabled %v, rule %d, want a plain disabled attack", got.Enabled, got.Bonus)
	}

	spent := after
	spent.BonusActionUsed = true
	got = attackOptionOf(t, Options(bonusSheet(1, false), spent, Usage{}), dagger)
	if got.Enabled || got.Reason.Code != ReasonBonusActionUsed {
		t.Errorf("off hand with the bonus action spent = enabled %v, reason %v, want %s", got.Enabled, got.Reason, ReasonBonusActionUsed)
	}
	if first := attackOptionOf(t, Options(bonusSheet(1, false), TurnState{}, Usage{}), dagger); first.Bonus != BonusNone || first.Attack.Damage != "1d4+3" {
		t.Errorf("first attack = rule %d, damage %s, want the action's own attack with the whole damage", first.Bonus, first.Attack.Damage)
	}
}

func TestOptionsExtraAttackComesBeforeTheBonusAttack(t *testing.T) {
	t.Parallel()
	const dagger = "equipment:dagger"
	sheet := bonusSheet(2, false)

	got := attackOptionOf(t, Options(sheet, TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: dagger}, Usage{}), dagger)
	if !got.Enabled || got.Bonus != BonusNone || got.Attack.Damage != "1d4+3" {
		t.Errorf("second attack = enabled %v, rule %d, damage %s, want the Attack action's with the whole damage", got.Enabled, got.Bonus, got.Attack.Damage)
	}
	got = attackOptionOf(t, Options(sheet, TurnState{ActionUsed: true, AttacksMade: 2, LastAttackKey: dagger}, Usage{}), dagger)
	if !got.Enabled || got.Bonus != BonusTwoWeapon {
		t.Errorf("attack after both = enabled %v, rule %d, want the off hand", got.Enabled, got.Bonus)
	}
}

func TestOptionsMartialArtsStrikeAfterTheAttackAction(t *testing.T) {
	t.Parallel()
	after := TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: rules.UnarmedStrikeKey}
	got := attackOptionOf(t, Options(bonusSheet(1, false), after, Usage{}), rules.UnarmedStrikeKey)
	if !got.Enabled || got.Bonus != BonusMartialArts || got.Attack.Damage != "1d4+3" {
		t.Errorf("Martial Arts strike = enabled %v, rule %d, damage %s, want enabled with the modifier kept", got.Enabled, got.Bonus, got.Attack.Damage)
	}
	after.BonusActionUsed = true
	if got := attackOptionOf(t, Options(bonusSheet(1, false), after, Usage{}), rules.UnarmedStrikeKey); got.Enabled || got.Reason.Code != ReasonBonusActionUsed {
		t.Errorf("Martial Arts strike with the bonus action spent = enabled %v, reason %v", got.Enabled, got.Reason)
	}
	// The strike follows an attack with an unarmed strike or a monk weapon only.
	afterSword := TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: "equipment:longsword"}
	if got := attackOptionOf(t, Options(bonusSheet(1, false), afterSword, Usage{}), rules.UnarmedStrikeKey); got.Enabled {
		t.Error("an unarmed strike after a longsword is enabled")
	}
	// A cantrip is the whole action: nothing follows it.
	afterCantrip := TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: "spell:fire-bolt"}
	if got := attackOptionOf(t, Options(bonusSheet(1, false), afterCantrip, Usage{}), rules.UnarmedStrikeKey); got.Enabled {
		t.Error("an unarmed strike after a cantrip is enabled")
	}
}

func TestOptionsFlurryStrikesCountDownWithTheBonusActionSpent(t *testing.T) {
	t.Parallel()
	for _, left := range []int{2, 1} {
		turn := TurnState{ActionUsed: true, BonusActionUsed: true, AttacksMade: 1, LastAttackKey: "equipment:longsword", FlurryLeft: left}
		o := Options(bonusSheet(1, false), turn, Usage{})
		got := attackOptionOf(t, o, rules.UnarmedStrikeKey)
		if !got.Enabled || got.Bonus != BonusFlurry || got.FlurryLeft != left {
			t.Errorf("strike with %d left = enabled %v, rule %d, left %d", left, got.Enabled, got.Bonus, got.FlurryLeft)
		}
		if got := attackOptionOf(t, o, "equipment:longsword"); got.Enabled {
			t.Errorf("a sword attack during Flurry of Blows is enabled (%d left)", left)
		}
	}
	turn := TurnState{ActionUsed: true, BonusActionUsed: true, AttacksMade: 1, LastAttackKey: rules.UnarmedStrikeKey}
	if got := attackOptionOf(t, Options(bonusSheet(1, false), turn, Usage{}), rules.UnarmedStrikeKey); got.Enabled || got.Reason.Code != ReasonBonusActionUsed {
		t.Errorf("strike with none left = enabled %v, reason %v, want the bonus action's", got.Enabled, got.Reason)
	}
}

func TestOptionsFlurryOfBlowsNeedsTheAttackAction(t *testing.T) {
	t.Parallel()
	flurry := func(turn TurnState) ActionOption {
		for _, a := range Options(bonusSheet(1, false), turn, Usage{}).FeatureActions {
			if a.Action.Key == FlurryOfBlowsKey {
				return a
			}
		}
		t.Fatal("no Flurry of Blows option")
		return ActionOption{}
	}
	for _, tc := range []struct {
		name string
		turn TurnState
		want string
	}{
		{"before the Attack action", TurnState{}, ReasonAttackActionFirst},
		{"after another action", TurnState{ActionUsed: true}, ReasonAttackActionFirst},
		{"after a cantrip", TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: "spell:fire-bolt"}, ReasonAttackActionFirst},
		{"with the bonus action spent", TurnState{ActionUsed: true, BonusActionUsed: true, AttacksMade: 1, LastAttackKey: rules.UnarmedStrikeKey}, ReasonBonusActionUsed},
		{"after the Attack action", TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: rules.UnarmedStrikeKey}, ""},
	} {
		got := flurry(tc.turn)
		switch {
		case tc.want == "" && !got.Enabled:
			t.Errorf("%s: disabled with %v, want enabled", tc.name, got.Reason)
		case tc.want != "" && (got.Enabled || got.Reason == nil || got.Reason.Code != tc.want):
			t.Errorf("%s: enabled %v, reason %v, want %s", tc.name, got.Enabled, got.Reason, tc.want)
		}
	}
}

func TestOptionsACantripSpendsTheAttackAction(t *testing.T) {
	t.Parallel()
	// Eldritch Blast cast with the action counts a beam as an attack, which must
	// not leave an attack of Extra Attack behind.
	turn := TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: "spell:eldritch-blast"}
	o := Options(bonusSheet(2, false), turn, Usage{})
	if o.Economy.AttacksLeft != 0 {
		t.Errorf("attacks left after a cantrip = %d, want 0", o.Economy.AttacksLeft)
	}
	for _, key := range []string{"equipment:longsword", "equipment:dagger", rules.UnarmedStrikeKey, "spell:fire-bolt"} {
		got := attackOptionOf(t, o, key)
		if got.Enabled || got.Reason.Code != ReasonActionUsed || got.Bonus != BonusNone {
			t.Errorf("%s after a cantrip = enabled %v, reason %v, rule %d, want disabled with the action's reason", key, got.Enabled, got.Reason, got.Bonus)
		}
	}
	// A weapon Attack action still leaves Extra Attack's second attack.
	weapon := TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: "equipment:longsword"}
	if got := attackOptionOf(t, Options(bonusSheet(2, false), weapon, Usage{}), "equipment:longsword"); !got.Enabled {
		t.Error("the second attack of the Attack action is disabled")
	}
}

func TestOptionsEldritchBlastKeepsItsBeamsAfterTheFirst(t *testing.T) {
	t.Parallel()
	const blast = "spell:eldritch-blast"
	sheet := bonusSheet(1, false)
	for _, tc := range []struct {
		name      string
		turn      TurnState
		enabled   bool
		beamsLeft int
	}{
		{"before the cast", TurnState{}, true, 0},
		{"after the first beam", TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: blast}, true, 1},
		{"after the last beam", TurnState{ActionUsed: true, AttacksMade: 2, LastAttackKey: blast}, false, 0},
	} {
		o := Options(sheet, tc.turn, Usage{})
		got := attackOptionOf(t, o, blast)
		if got.Enabled != tc.enabled || got.BeamsLeft != tc.beamsLeft {
			t.Errorf("%s: enabled %v, beams left %d, want %v and %d", tc.name, got.Enabled, got.BeamsLeft, tc.enabled, tc.beamsLeft)
		}
		if tc.turn.ActionUsed && !tc.enabled && got.Reason.Code != ReasonActionUsed {
			t.Errorf("%s: reason %v, want the action's", tc.name, got.Reason)
		}
		// Every other attack is off, as the action's, once the action is spent.
		if tc.turn.ActionUsed {
			for _, key := range []string{"equipment:longsword", "spell:fire-bolt"} {
				if other := attackOptionOf(t, o, key); other.Enabled || other.Reason.Code != ReasonActionUsed {
					t.Errorf("%s: %s = enabled %v, reason %v, want the action's", tc.name, key, other.Enabled, other.Reason)
				}
			}
		}
	}
	// A beam of another cantrip's cast is not this one's.
	if got := attackOptionOf(t, Options(sheet, TurnState{ActionUsed: true, AttacksMade: 1, LastAttackKey: "spell:fire-bolt"}, Usage{}), blast); got.Enabled {
		t.Error("Eldritch Blast is enabled after Fire Bolt spent the action")
	}
}
