package rules

import (
	"fmt"
	"strings"
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
			"O personagem não cumpre mais o pré-requisito de %s: %s", x.c.namePT(key), strings.Join(unmet, " "))
	}
}

// unmetPicked are the sentences of what a picked option's prerequisite lacks now.
func (c *content) unmetPicked(p TableOptionPrerequisite, b Build, d Derived) []string {
	asked := FeatPrerequisite{ClassKey: p.ClassKey, ClassLevel: p.ClassLevel, RequiresPact: p.RequiresPact, RequiresSpell: p.RequiresSpell, RequiresOptions: p.RequiresOptions}
	var out []string
	for _, u := range c.unmetKnowledge(asked, b, d) {
		out = append(out, c.knowledgeUnmetPT(u, b))
	}
	return out
}
