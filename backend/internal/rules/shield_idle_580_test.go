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
		{"a feminine weapon and a named character", "Tor", "A Maça de Guerra agora pede as duas mãos: com ela equipada, o escudo de Tor fica parado (CA %d → %d)."},
		{"nobody named", "", "A Maça de Guerra agora pede as duas mãos: com ela equipada, o escudo do personagem fica parado (CA %d → %d)."},
	}
	for _, tc := range cases {
		if got, want := n.Idle.SentencePT(tc.character), fmt.Sprintf(tc.want, with, without); got != want {
			t.Errorf("%s: sentence = %q, want %q", tc.name, got, want)
		}
	}
	masc := ShieldIdle{Key: "weapon:x", WeaponPT: "Machado de Lenhador", ACWith: 18, ACWithout: 16}
	if got, want := masc.SentencePT("Tor"), "O Machado de Lenhador agora pede as duas mãos: com ele equipado, o escudo de Tor fica parado (CA 18 → 16)."; got != want {
		t.Errorf("sentence = %q, want the board's %q", got, want)
	}
	for name, want := range map[string]string{"Foice Longa": "A", "Rede de Pesca": "A", "Besta Leve": "A", "Tridente": "O", "Cajado": "O", "Florete Fino": "O"} {
		if got := (ShieldIdle{WeaponPT: name}).SentencePT("Tor"); !strings.HasPrefix(got, want+" "+name+" ") {
			t.Errorf("%s: sentence %q, want the article %q", name, got, want)
		}
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
