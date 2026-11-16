package rules

import (
	"fmt"
	"strings"
	"testing"
)

// #580, board 580-16 state 8: a table weapon that takes both hands, equipped with a shield, leaves the shield idle. The
// sheet tells it as a notice: stored by its identity and told with a changed entry, but no issue, no validation error
// and no level-up refusal.

func shieldIdleContent(t *testing.T) *Content {
	t.Helper()
	ws := append(ruleWeapons(), tableWeapons()...)
	for i := range ws {
		ws[i].Revision = 1
	}
	return withOverlay(t, Overlay{Revision: 1, Weapons: ws})
}

func noticeOf(d Derived, code string) (Issue, bool) {
	for _, is := range d.Notices {
		if is.Code == code {
			return is, true
		}
	}
	return Issue{}, false
}

func withShieldOn(b Build) Build {
	b.Shield = true
	return b
}

func TestATwoHandedWeaponWithAShieldRaisesTheShieldIdleNotice(t *testing.T) {
	t.Parallel()
	c := shieldIdleContent(t)
	b := withShieldOn(ruleSheet(12, 14, wLight, wHeavy))
	d := Derive(b, c)
	n, ok := noticeOf(d, IssueShieldIdle)
	if !ok || len(d.Notices) != 1 {
		t.Fatalf("notices = %+v, want the one shield_idle", d.Notices)
	}
	if n.Field != "full.weapon_keys[1]" || len(n.Keys) != 1 || n.Keys[0] != wHeavy || n.Idle == nil || n.Idle.Key != wHeavy {
		t.Errorf("notice = %+v, want it at the weapon's field, tied to the weapon's key", n)
	}
	bare := b
	bare.Weapons = []string{wLight}
	if n.Idle.ACWith != Derive(bare, c).ArmorClass || n.Idle.ACWithout != d.ArmorClass || n.Idle.ACWith != n.Idle.ACWithout+2 {
		t.Errorf("AC with %d and without %d, want %d and %d", n.Idle.ACWith, n.Idle.ACWithout, Derive(bare, c).ArmorClass, d.ArmorClass)
	}
	for _, is := range d.Issues {
		if is.Code == IssueShieldIdle {
			t.Errorf("issues hold %+v: a notice is no issue", is)
		}
	}
	if !d.ShieldIdle {
		t.Error("ShieldIdle is off: the sheet still says the shield is idle")
	}
}

func TestTheShieldIdleSentenceIsTheBoardsPerReader(t *testing.T) {
	t.Parallel()
	c := shieldIdleContent(t)
	n, _ := noticeOf(Derive(withShieldOn(ruleSheet(12, 14, wHeavy)), c), IssueShieldIdle)
	if n.Idle == nil {
		t.Fatal("no notice")
	}
	with, without := n.Idle.ACWith, n.Idle.ACWithout
	cases := []struct {
		name, character, want string
	}{
		{"a named character", "Tor", "Maça de Guerra passou a exigir as duas mãos. Com Maça de Guerra em uso, o escudo de Tor não conta na CA (CA %d → %d)."},
		{"nobody named", "", "Maça de Guerra passou a exigir as duas mãos. Com Maça de Guerra em uso, o escudo do personagem não conta na CA (CA %d → %d)."},
	}
	for _, tc := range cases {
		if got, want := n.Idle.SentencePT(tc.character), fmt.Sprintf(tc.want, with, without); got != want {
			t.Errorf("%s: sentence = %q, want %q", tc.name, got, want)
		}
	}
	if got, want := (ShieldIdle{Key: "weapon:x", WeaponPT: "Machado de Lenhador", ACWith: 18, ACWithout: 16}).SentencePT("Tor"),
		"Machado de Lenhador passou a exigir as duas mãos. Com Machado de Lenhador em uso, o escudo de Tor não conta na CA (CA 18 → 16)."; got != want {
		t.Errorf("sentence = %q, want the board's %q", got, want)
	}
}

// The weapon's name stands alone: no article and no pronoun for it, whatever the name, since the table gives no gender.
func TestTheShieldIdleSentenceNeverGuessesAGender(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Foice Longa", "Rede de Pesca", "Besta Leve", "Tridente", "Cajado", "Florete Fino", "Maça"} {
		got := (ShieldIdle{WeaponPT: name, ACWith: 18, ACWithout: 16}).SentencePT("Tor")
		want := name + " passou a exigir as duas mãos. Com " + name + " em uso, o escudo de Tor não conta na CA (CA 18 → 16)."
		if got != want {
			t.Errorf("%s: sentence = %q, want %q", name, got, want)
		}
		for _, guess := range []string{" ela ", " ele ", " equipada", " equipado", "O " + name, "A " + name, "agora"} {
			if strings.Contains(got, guess) {
				t.Errorf("%s: sentence %q holds %q, a guess at the weapon's gender", name, got, guess)
			}
		}
	}
}

// The numbers are told only when they differ: with the same armor class before and after, the sentence says the shield
// does not count and gives no parenthesis.
func TestTheShieldIdleSentenceGivesNoNumbersWhenTheyAreEqual(t *testing.T) {
	t.Parallel()
	equal := ShieldIdle{WeaponPT: "Machado de Lenhador", ACWith: 16, ACWithout: 16}
	if got, want := equal.SentencePT("Tor"), "Machado de Lenhador passou a exigir as duas mãos. Com Machado de Lenhador em uso, o escudo de Tor não conta na CA."; got != want {
		t.Errorf("equal: sentence = %q, want %q", got, want)
	}
	if got, want := equal.SentencePT(""), "Machado de Lenhador passou a exigir as duas mãos. Com Machado de Lenhador em uso, o escudo do personagem não conta na CA."; got != want {
		t.Errorf("equal, nobody named: sentence = %q, want %q", got, want)
	}
	differ := ShieldIdle{WeaponPT: "Machado de Lenhador", ACWith: 18, ACWithout: 16}
	if got := differ.SentencePT("Tor"); !strings.HasSuffix(got, "não conta na CA (CA 18 → 16).") {
		t.Errorf("different: sentence = %q, want the numbers", got)
	}
}

// Only a weapon that takes both hands raises it: a versatile weapon with a shield is held in one hand, and a weapon or a
// shield that is only carried is not in use.
func TestNothingElseLeavesTheShieldIdle(t *testing.T) {
	t.Parallel()
	c := shieldIdleContent(t)
	carried := func(b Build, keys ...string) Build {
		b.Carried = keys
		return b
	}
	cases := []struct {
		name string
		b    Build
	}{
		{"a versatile weapon", withShieldOn(ruleSheet(12, 14, weaponHalberd))},
		{"a one-handed weapon", withShieldOn(ruleSheet(12, 14, wLight, wFinesse))},
		{"the two-handed weapon is carried", carried(withShieldOn(ruleSheet(12, 14, wHeavy)), wHeavy)},
		{"the shield is carried", carried(withShieldOn(ruleSheet(12, 14, wHeavy)), "equipment:shield")},
		{"no shield", ruleSheet(12, 14, wHeavy)},
	}
	for _, tc := range cases {
		d := Derive(tc.b, c)
		if len(d.Notices) != 0 || len(d.Issues) != 0 && hasCode(d, IssueShieldIdle) {
			t.Errorf("%s: notices %+v, issues %+v, want none", tc.name, d.Notices, d.Issues)
		}
	}
}

func hasCode(d Derived, code string) bool {
	for _, is := range d.Issues {
		if is.Code == code {
			return true
		}
	}
	return false
}

// The identity is the code, the field and the weapon's key, never a sentence: the same whatever the AC or the name.
func TestTheShieldIdleIdentityHoldsNoSentence(t *testing.T) {
	t.Parallel()
	c := shieldIdleContent(t)
	n, _ := noticeOf(Derive(withShieldOn(ruleSheet(12, 14, wHeavy)), c), IssueShieldIdle)
	id := n.ID()
	if !strings.HasPrefix(id, "shield_idle|full.weapon_keys|") || strings.Contains(id, "[") || !strings.HasSuffix(id, wHeavy) || strings.Contains(id, "mãos") || strings.Contains(id, "Maça") {
		t.Errorf("identity = %q, want code|list field without an index|…weapon key, no sentence", id)
	}
	again, _ := noticeOf(Derive(withShieldOn(ruleSheet(18, 14, wHeavy)), c), IssueShieldIdle)
	if again.ID() != id {
		t.Errorf("identity %q changed with the numbers to %q", id, again.ID())
	}
	moved, _ := noticeOf(Derive(withShieldOn(ruleSheet(12, 14, wLight, wHeavy)), c), IssueShieldIdle)
	if moved.ID() != id {
		t.Errorf("identity %q changed with the weapon's place in the list to %q", id, moved.ID())
	}
	if all := Derive(withShieldOn(ruleSheet(12, 14, wHeavy)), c).AllIssues(); len(all) == 0 || all[len(all)-1].ID() != id {
		t.Errorf("AllIssues = %+v, want the notice among them", all)
	}
}

// A notice is never a validation error and never refuses a level-up.
func TestTheShieldIdleNoticeBlocksNothing(t *testing.T) {
	t.Parallel()
	c := shieldIdleContent(t)
	before := withShieldOn(ruleSheet(12, 14, wHeavy))
	before.FeatureChoices = []string{"feature:fighter-fighting-style-defense"}
	if err := Validate(before, c); err != nil {
		t.Errorf("Validate() = %v, want none", err)
	}
	if _, ok := noticeOf(Derive(before, c), IssueShieldIdle); !ok {
		t.Fatal("the sheet has no notice: the case tests nothing")
	}
	after, err := ApplyLevelUp(before, LevelUpChoices{Class: "class:fighter", HitPoints: LevelUpHitPoints{Average: true}}, c)
	if err == nil {
		err = CheckLevelUp(before, after, c)
	}
	if err != nil {
		t.Errorf("the level-up of a sheet with a shield idle was refused: %v", err)
	}
}

func TestTheShieldIdleNumbersStowEveryTwoHandedWeaponTheSrdOnesToo(t *testing.T) {
	t.Parallel()
	c := shieldIdleContent(t)
	b := withShieldOn(ruleSheet(12, 14, "equipment:greatsword", wHeavy))
	d := Derive(b, c)
	n, ok := noticeOf(d, IssueShieldIdle)
	if !ok || n.Idle == nil {
		t.Fatalf("notices = %+v, want the one shield_idle", d.Notices)
	}
	bare := b
	bare.Weapons = []string{}
	if want := Derive(bare, c).ArmorClass; n.Idle.ACWith != want || n.Idle.ACWithout != d.ArmorClass || n.Idle.ACWith != n.Idle.ACWithout+2 {
		t.Errorf("AC with %d and without %d, want %d and %d (the shield counts once both two-handed weapons are stowed)", n.Idle.ACWith, n.Idle.ACWithout, want, d.ArmorClass)
	}
}

func TestTheShieldIdleNumbersCountAMagicShield(t *testing.T) {
	t.Parallel()
	c := shieldIdleContent(t)
	b := withShieldOn(ruleSheet(12, 14, wHeavy))
	b.ItemBonuses = []ItemBonus{{Key: ShieldKey, Bonus: 1}}
	d := Derive(b, c)
	n, ok := noticeOf(d, IssueShieldIdle)
	if !ok || n.Idle == nil {
		t.Fatalf("notices = %+v, want the one shield_idle", d.Notices)
	}
	if n.Idle.ACWith != n.Idle.ACWithout+3 {
		t.Errorf("AC with %d and without %d, want the +2 shield and its +1 bonus: 3 more", n.Idle.ACWith, n.Idle.ACWithout)
	}
}

func TestTwoTableTwoHandedWeaponsGiveTheSameNumbers(t *testing.T) {
	t.Parallel()
	const second = "weapon:machado-grande" + tableSuffix
	ws := append(ruleWeapons(), TableWeapon{TableEntry: TableEntry{Key: second, NamePT: "Machado Grande"}, Category: "martial", Damage: "1d12", DamageType: "damage-type:slashing", Properties: []string{"weapon-property:heavy", "weapon-property:two-handed"}})
	for i := range ws {
		ws[i].Revision = 1
	}
	c := withOverlay(t, Overlay{Revision: 1, Weapons: ws})
	d := Derive(withShieldOn(ruleSheet(12, 14, wHeavy, second)), c)
	if len(d.Notices) != 2 {
		t.Fatalf("notices = %+v, want one for each table two-handed weapon", d.Notices)
	}
	for _, n := range d.Notices {
		if n.Idle.ACWith != d.ArmorClass+2 || n.Idle.ACWithout != d.ArmorClass {
			t.Errorf("%s: AC with %d and without %d, want %d and %d", n.Idle.Key, n.Idle.ACWith, n.Idle.ACWithout, d.ArmorClass+2, d.ArmorClass)
		}
	}
}

// An effect that sets the armor class leaves it the same with the shield counting or not: the shield is no idle one in
// any number the sheet shows, and the notice would say "CA 15 → 15", so none is raised. The shield's hint stays.
func TestNoShieldIdleNoticeWhenTheShieldChangesNoNumber(t *testing.T) {
	t.Parallel()
	const fixed = "feat:ca-fixa" + tableSuffix
	ws := append(ruleWeapons(), tableWeapons()...)
	for i := range ws {
		ws[i].Revision = 1
	}
	feat := TableFeat{TableEntry: TableEntry{Key: fixed, NamePT: "CA Fixa", Revision: 1}, DescPT: []string{"Texto."}, Effects: []Effect{{Type: "modifier", Target: "ac", Mode: "set", Value: "15"}}}
	c := withOverlay(t, Overlay{Revision: 1, Weapons: ws, Feats: []TableFeat{feat}})
	b := withShieldOn(ruleSheet(12, 14, wHeavy))
	b.Feats = []string{fixed}
	d := Derive(b, c)
	if !d.ShieldIdle || d.ArmorClass != 15 {
		t.Fatalf("shield idle %v, AC %d, want the idle shield and the set AC 15: the case tests nothing", d.ShieldIdle, d.ArmorClass)
	}
	if len(d.Notices) != 0 {
		t.Errorf("notices = %+v, want none: the shield changes no number", d.Notices)
	}
}
