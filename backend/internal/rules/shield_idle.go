package rules

import (
	"fmt"
	"slices"
	"strings"
)

// IssueShieldIdle is the code of the notice "O Machado agora pede as duas mãos: com ele equipado, o escudo fica parado":
// an equipped shield does not count because an equipped weapon takes both hands (board 580-16, state 8). It is a notice
// (Derived.Notices), never an issue.
const IssueShieldIdle = "shield_idle"

// ShieldIdle is what the notice says: the weapon that takes both hands and the armor class the character has with the
// shield and without it. The sentence is built for each reader (SentencePT), as the character's name is not in the Build.
type ShieldIdle struct {
	// Key is the weapon's key: with the notice's code and field, what the notice is told apart by (Issue.ID).
	Key string
	// WeaponPT is the weapon's name.
	WeaponPT string
	// ACWith and ACWithout are the armor class with the shield counting and as the sheet has it, without.
	ACWith, ACWithout int
}

// idleIDPrefix marks, in a notice's identity, the weapon's key where an issue has its sentence.
const idleIDPrefix = "idle:"

// SentencePT is the sentence a reader gets: "O Machado de Lenhador agora pede as duas mãos: com ele equipado, o escudo de
// Tor fica parado (CA 18 → 16).". name is the character's (empty reads "o personagem").
func (s ShieldIdle) SentencePT(name string) string {
	of := "de " + name
	if name == "" {
		of = "do personagem"
	}
	article, pronoun, ending := "O", "ele", "o"
	if feminineWeapon(s.WeaponPT) {
		article, pronoun, ending = "A", "ela", "a"
	}
	return fmt.Sprintf("%s %s agora pede as duas mãos: com %s equipad%s, o escudo %s fica parado (CA %d → %d).",
		article, s.WeaponPT, pronoun, ending, of, s.ACWith, s.ACWithout)
}

// feminineWeaponNouns are the weapon names that end in a letter other than "a" and are feminine.
var feminineWeaponNouns = []string{"foice", "rede"}

// feminineWeapon says the weapon's name is feminine, read from its first word: the table does not give a gender, and a
// Portuguese weapon name ending in "a" is feminine (a espada, a lança, a besta).
func feminineWeapon(name string) bool {
	first, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(name)), " ")
	return strings.HasSuffix(first, "a") || strings.HasSuffix(first, "ã") || slices.Contains(feminineWeaponNouns, first)
}

// shieldIdleNotices are the notices of a sheet whose equipped shield does not count: one for each equipped table weapon
// that takes both hands (only the master's table weapons are edited, so an SRD weapon's notice would never be told),
// tied to the weapon's key, with the armor class the character would have with the shield counting.
func (c *content) shieldIdleNotices(b Build, d Derived) []Issue {
	if !d.ShieldIdle {
		return nil
	}
	var keys []string
	for _, key := range b.Weapons {
		if eq, ok := c.equipment[key]; ok && isTableKey(key) && !b.IsCarried(key) && eq.Weapon != nil && slices.Contains(eq.Weapon.Properties, twoHandedProperty) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	free := b
	free.Carried = append(slices.Clone(b.Carried), keys...)
	with := derive(free, c).ArmorClass
	x := &deriver{b: b, c: c}
	var out []Issue
	for i, key := range b.Weapons {
		if !slices.Contains(keys, key) {
			continue
		}
		field := fmt.Sprintf("full.weapon_keys[%d]", i)
		out = append(out, Issue{
			Code: IssueShieldIdle, Field: field, Keys: x.issueKeys(IssueShieldIdle, field),
			Idle: &ShieldIdle{Key: key, WeaponPT: c.namePT(key), ACWith: with, ACWithout: d.ArmorClass},
		})
		out[len(out)-1].Message = out[len(out)-1].Idle.SentencePT("")
	}
	return out
}

// AllIssues are the issues of the sheet and then its notices: what the sheet stores the identity of (known issues) and
// what a changed table entry is flagged by.
func (d Derived) AllIssues() []Issue {
	out := make([]Issue, 0, len(d.Issues)+len(d.Notices))
	return append(append(out, d.Issues...), d.Notices...)
}
