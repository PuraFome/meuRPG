package rules

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

// SentencePT is the sentence a reader gets; name is the character's (empty reads "o personagem").
func (s ShieldIdle) SentencePT(name string) string { return "" }

// AllIssues are the issues of the sheet and then its notices: what the sheet stores the identity of (known issues) and
// what a changed table entry is flagged by.
func (d Derived) AllIssues() []Issue {
	out := make([]Issue, 0, len(d.Issues)+len(d.Notices))
	return append(append(out, d.Issues...), d.Notices...)
}
