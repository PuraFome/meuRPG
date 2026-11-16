package rules

import (
	"fmt"
	"slices"
)

// IssueShieldIdle is the code of the notice "Machado passou a exigir as duas mãos. Com Machado em uso, o escudo não conta na CA":
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

// SentencePT is the sentence a reader gets: "Machado de Lenhador passou a exigir as duas mãos. Com Machado de Lenhador
// em uso, o escudo de Tor não conta na CA (CA 18 → 16).". name is the character's (empty reads "o personagem"). The
// weapon's name stands alone, with no article and no pronoun, as the table gives it no gender. The armor classes are
// told only when they differ.
func (s ShieldIdle) SentencePT(name string) string {
	of := "de " + name
	if name == "" {
		of = "do personagem"
	}
	numbers := ""
	if s.ACWith != s.ACWithout {
		numbers = fmt.Sprintf(" (CA %d → %d)", s.ACWith, s.ACWithout)
	}
	return fmt.Sprintf("%s passou a exigir as duas mãos. Com %s em uso, o escudo %s não conta na CA%s.",
		s.WeaponPT, s.WeaponPT, of, numbers)
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
