package rules

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// checkOptionPrerequisites raises IssueOptionPrerequisite for each option the sheet picked and owns whose prerequisite
// it no longer meets (a lost class level, Pact Boon, spell or option). The pick stays on the sheet; the issue says what
// is missing, naming nothing a player may not read.
func (x *deriver) checkOptionPrerequisites() {
	if len(x.c.optionPrereq) == 0 {
		return
	}
	owns := ownedBy(x.b, *x.d)
	for i, key := range x.b.FeatureChoices {
		p, ok := x.c.optionPrereq[key]
		if !ok || !owns(key) {
			continue
		}
		unmet := x.c.unmetPicked(p, x.b, *x.d)
		if len(unmet) == 0 {
			continue
		}
		x.issue(IssueOptionPrerequisite, fmt.Sprintf("full.feature_choice_keys[%d]", i),
			"O personagem não cumpre mais o pré-requisito de %s: %s", x.c.namePT(key), lowerFirst(strings.Join(unmet, " ")))
	}
}

// unmetPicked are the sentences of what a picked option's prerequisite lacks now.
func (c *content) unmetPicked(p TableOptionPrerequisite, b Build, d Derived) []string {
	asked := FeatPrerequisite{ClassKey: p.ClassKey, ClassLevel: p.ClassLevel, RequiresPact: p.RequiresPact, RequiresSpell: p.RequiresSpell, RequiresOptions: p.RequiresOptions}
	var out []string
	for _, u := range c.unmetKnowledge(asked, b, d) {
		// The sheet stores the text of its issues (known_issues), so it is the same for the master and the player:
		// an entry hidden from the players is never named in it, whoever reads.
		if c.hidden(u.Key) && !c.sheetHas(b, u.Key) {
			out = append(out, hiddenUnmetPT)
			continue
		}
		out = append(out, c.knowledgeUnmetPT(u, b))
	}
	return out
}

// lostOptions are the options the build picked and owns whose prerequisite the derived sheet no longer meets (as
// lostFeats does for feats): they stay listed and do not apply until the character meets it again.
func (c *content) lostOptions(b Build, d Derived) map[string]bool {
	var lost map[string]bool
	owns := ownedBy(b, d)
	for _, key := range b.FeatureChoices {
		p, ok := c.optionPrereq[key]
		if !ok || !owns(key) || len(c.unmetPicked(p, b, d)) == 0 {
			continue
		}
		if lost == nil {
			lost = map[string]bool{}
		}
		lost[key] = true
	}
	return lost
}

// lowerFirst makes the first letter lower case, for a sentence that joins another after a colon.
func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
