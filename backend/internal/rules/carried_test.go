package rules

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func hintTargets(d Derived, target string) int {
	n := 0
	for _, h := range d.Hints {
		if h.Target == target {
			n++
		}
	}
	return n
}

// A carried armor counts as no armor: the base is 10 + DEX or an unarmored base, with
// no proficiency issue and no Stealth hint.
func TestCarriedArmorCountsAsNone(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	tests := []struct {
		name      string
		class     string
		armor     string
		carried   bool
		mageArmor bool
		wantAC    int
		wantStlth int
		wantIssue bool
	}{
		{"equipped half plate", "class:wizard", "equipment:half-plate-armor", false, false, 17, 1, true},
		{"carried half plate", "class:wizard", "equipment:half-plate-armor", true, false, 12, 0, false},
		{"carried chain mail, mage armor", "class:wizard", "equipment:chain-mail", true, true, 15, 0, false},
		{"equipped chain mail, mage armor", "class:wizard", "equipment:chain-mail", false, true, 16, 1, true},
		{"carried leather, monk", "class:monk", "equipment:leather-armor", true, false, 14, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := standard(tt.class, 1)
			b.BaseScores = map[Ability]int{STR: 15, DEX: 14, CON: 13, INT: 12, WIS: 14, CHA: 8}
			b.Armor, b.MageArmor = tt.armor, tt.mageArmor
			if tt.carried {
				b.Carried = []string{tt.armor}
			}
			d := Derive(b, c)
			if d.ArmorClass != tt.wantAC {
				t.Errorf("AC = %d, want %d", d.ArmorClass, tt.wantAC)
			}
			if got := hintTargets(d, "skill:stealth"); got != tt.wantStlth {
				t.Errorf("Stealth hints = %d, want %d", got, tt.wantStlth)
			}
			if has := slicesHasIssue(d, IssueArmorProficiency); has != tt.wantIssue {
				t.Errorf("armor proficiency issue = %v, want %v", has, tt.wantIssue)
			}
			if tt.carried && (d.ArmorCategory != "none" || d.ArmorUntrained.Any()) {
				t.Errorf("category %q untrained %+v, want none and trained", d.ArmorCategory, d.ArmorUntrained)
			}
		})
	}
}

func slicesHasIssue(d Derived, code string) bool {
	for _, s := range issueCodes(d) {
		if len(s) >= len(code) && s[:len(code)] == code {
			return true
		}
	}
	return false
}

// Unarmored Defense applies when the armor is carried.
func TestCarriedArmorLetsUnarmoredDefenseApply(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	free := monkBuild(1, 10, 16)
	worn := monkBuild(1, 10, 16)
	worn.Armor = "equipment:chain-mail"
	carried := worn
	carried.Carried = []string{"equipment:chain-mail"}
	if got, want := Derive(carried, c).ArmorClass, Derive(free, c).ArmorClass; got != want {
		t.Errorf("monk with carried armor AC = %d, want %d (unarmored)", got, want)
	}
	if Derive(worn, c).ArmorClass == Derive(free, c).ArmorClass {
		t.Errorf("equipped chain mail should change the monk's AC")
	}
}

// A carried shield adds nothing, raises no proficiency issue and leaves Martial Arts on.
func TestCarriedShield(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	base := Derive(standard("class:wizard", 1), c).ArmorClass
	b := standard("class:wizard", 1)
	b.Shield = true
	if got := Derive(b, c).ArmorClass; got != base+shieldACBonus {
		t.Fatalf("equipped shield AC = %d, want %d", got, base+shieldACBonus)
	}
	b.Carried = []string{shieldKey}
	d := Derive(b, c)
	if d.ArmorClass != base || d.ArmorUntrained.Shield || slicesHasIssue(d, IssueArmorProficiency) || d.ShieldIdle {
		t.Errorf("carried shield: AC %d (want %d), untrained %+v, issues %v", d.ArmorClass, base, d.ArmorUntrained, issueCodes(d))
	}

	m := monkBuild(5, 9, 18)
	m.Shield = true
	m.Carried = []string{shieldKey}
	dagger, _ := attackOf(Derive(m, c), "equipment:dagger")
	if dagger.Damage != "1d6+4" {
		t.Errorf("monk with a carried shield dagger damage = %q, want 1d6+4 (Martial Arts)", dagger.Damage)
	}
}

// A carried weapon has no attack line and does not idle the shield; the unarmed strike stays.
func TestCarriedWeapon(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := shieldFighter("equipment:longsword", "equipment:greatsword")
	b.Carried = []string{"equipment:greatsword"}
	d := Derive(b, c)
	if _, ok := attackOf(d, "equipment:greatsword"); ok {
		t.Errorf("carried greatsword has an attack line")
	}
	if _, ok := attackOf(d, "equipment:longsword"); !ok {
		t.Errorf("equipped longsword lost its attack line")
	}
	if _, ok := attackOf(d, UnarmedStrikeKey); !ok {
		t.Errorf("unarmed strike is missing")
	}
	if d.ShieldIdle || d.ArmorClass != 18 {
		t.Errorf("AC %d idle %v, want 18 and a shield in use", d.ArmorClass, d.ShieldIdle)
	}
	b.Carried = []string{"equipment:longsword", "equipment:greatsword"}
	d = Derive(b, c)
	if len(d.Attacks) != 1 || d.Attacks[0].Key != UnarmedStrikeKey {
		t.Errorf("attacks = %d, want only the unarmed strike", len(d.Attacks))
	}
}

func TestValidateCarried(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	mk := func(shield bool, carried ...string) Build {
		b := standard("class:fighter", 1)
		b.Armor, b.Shield = "equipment:chain-mail", shield
		b.Weapons = []string{"equipment:longsword"}
		b.Carried = carried
		return b
	}
	tests := []struct {
		name  string
		b     Build
		field string
	}{
		{"ok", mk(true, "equipment:chain-mail", shieldKey, "equipment:longsword"), ""},
		{"unlisted key", mk(true, "equipment:dagger"), "full.carried_keys[0]"},
		{"repeat", mk(true, "equipment:longsword", "equipment:longsword"), "full.carried_keys[1]"},
		{"shield without shield", mk(false, shieldKey), "full.carried_keys[0]"},
		{"empty key", mk(true, ""), "full.carried_keys[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tt.b, c)
			var ve *ValidationError
			switch {
			case tt.field == "" && err != nil:
				t.Errorf("Validate = %v, want nil", err)
			case tt.field != "" && !errors.As(err, &ve):
				t.Errorf("Validate = %v, want a refusal on %s", err, tt.field)
			case tt.field != "" && ve.Field != tt.field:
				t.Errorf("field = %q, want %q", ve.Field, tt.field)
			}
		})
	}
	b := mk(true)
	b.Carried = make([]string, MaxCarried+1)
	if err := Validate(b, c); err == nil {
		t.Errorf("more than %d carried keys accepted", MaxCarried)
	}
}

// A sheet without carried_keys derives as it always did: everything it lists counts.
func TestNoCarriedKeysDerivesAsBefore(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	tests := []struct {
		name        string
		weapons     []string
		wantAC      int
		wantIdle    bool
		wantAttacks []string
	}{
		{"one-handed weapon", []string{"equipment:longsword"}, 18, false, []string{"equipment:longsword", UnarmedStrikeKey}},
		{"two-handed weapon", []string{"equipment:greatsword"}, 16, true, []string{"equipment:greatsword", UnarmedStrikeKey}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := shieldFighter(tt.weapons...)
			d := Derive(b, c)
			var keys []string
			for _, a := range d.Attacks {
				keys = append(keys, a.Key)
			}
			if d.ArmorClass != tt.wantAC || d.ShieldIdle != tt.wantIdle || !slices.Equal(keys, tt.wantAttacks) || d.ArmorCategory != "heavy" {
				t.Errorf("AC %d idle %v attacks %v category %q, want %d %v %v heavy", d.ArmorClass, d.ShieldIdle, keys, d.ArmorCategory, tt.wantAC, tt.wantIdle, tt.wantAttacks)
			}
			b.Carried = []string{}
			if !reflect.DeepEqual(d, Derive(b, c)) {
				t.Errorf("empty Carried derives differently from nil")
			}
		})
	}
}

// A carried armor key is not resolved, so an unknown one is silent until it is equipped.
func TestCarriedUnknownArmorIsSilent(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := standard("class:fighter", 1)
	b.Armor = "equipment:nope"
	if len(issueCodes(Derive(b, c))) == 0 {
		t.Fatalf("an equipped unknown armor raises no issue")
	}
	b.Carried = []string{"equipment:nope"}
	for _, code := range issueCodes(Derive(b, c)) {
		if strings.HasPrefix(code, IssueUnknownKey) {
			t.Errorf("carried unknown armor raised %q", code)
		}
	}
}

// The level-up refuses a change of carried_keys, and clone copies the slice.
func TestCarriedIsLockedInTheLevelUp(t *testing.T) {
	t.Parallel()
	before := shieldFighter("equipment:longsword")
	before.Carried = []string{"equipment:longsword"}
	for name, edit := range map[string]func(*Build){
		"an entry dropped": func(b *Build) { b.Carried = nil },
		"an entry added":   func(b *Build) { b.Carried = append(b.Carried, shieldKey) },
	} {
		after := before.clone()
		edit(&after)
		if err := checkLocked(before, after); err == nil || err.Field != "full.carried_keys" {
			t.Errorf("%s: checkLocked = %v", name, err)
		}
	}
	if err := checkLocked(before, before.clone()); err != nil {
		t.Errorf("a copy: %v", err)
	}
	after := before.clone()
	after.Carried[0] = shieldKey
	if before.Carried[0] != "equipment:longsword" {
		t.Errorf("clone shares the Carried slice")
	}
}

// ShieldCounts, which the owner environment and the summon count read, is false for a
// carried shield and for an idle one.
func TestShieldCounts(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	b := shieldFighter("equipment:longsword")
	if !b.ShieldCounts(Derive(b, c)) {
		t.Errorf("equipped shield does not count")
	}
	b.Carried = []string{shieldKey}
	if b.ShieldCounts(Derive(b, c)) {
		t.Errorf("carried shield counts")
	}
	b = shieldFighter("equipment:greatsword")
	if b.ShieldCounts(Derive(b, c)) {
		t.Errorf("idle shield counts")
	}
}
