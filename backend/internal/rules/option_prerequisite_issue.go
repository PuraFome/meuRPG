package rules

import (
	"fmt"
)

// checkOptionPrerequisites raises IssueOptionPrerequisite for each option the sheet picked and owns whose prerequisite
// it no longer meets (a lost class level, Pact Boon, spell or option). The pick stays on the sheet; the issue says what
// is missing (LostPrerequisite), and its stored message names no hidden entry.
func (x *deriver) checkOptionPrerequisites() {
	if len(x.c.optionPrereq) == 0 {
		return
	}
	owns := ownedBy(x.b, *x.d)
	for i, key := range x.b.FeatureChoices {
		if !owns(key) {
			continue
		}
		l := x.lost[key] // what it lacked with the options in place, when it was judged lost
		if l == nil {
			l = x.c.lostOption(key, x.b, *x.d)
		}
		if l != nil {
			x.lostIssue(IssueOptionPrerequisite, fmt.Sprintf("full.feature_choice_keys[%d]", i), l)
		}
	}
}

// lostOption is what the option key's prerequisite lacks on the sheet d, or nil when it has a prerequisite it meets or none.
func (c *content) lostOption(key string, b Build, d Derived) *LostPrerequisite {
	p, ok := c.optionPrereq[key]
	if !ok {
		return nil
	}
	asked := FeatPrerequisite{ClassKey: p.ClassKey, ClassLevel: p.ClassLevel, RequiresPact: p.RequiresPact, RequiresSpell: p.RequiresSpell, RequiresOptions: p.RequiresOptions}
	unmet := c.unmetKnowledge(asked, b, d)
	if len(unmet) == 0 {
		return nil
	}
	return c.lost(key, true, unmet, b, d)
}

// lostOptions are the options the build picked and owns whose prerequisite the derived sheet no longer meets (as
// lostFeats does for feats): they stay listed and do not apply until the character meets it again.
func (c *content) lostOptions(b Build, d Derived) map[string]*LostPrerequisite {
	var lost map[string]*LostPrerequisite
	owns := ownedBy(b, d)
	for _, key := range b.FeatureChoices {
		if !owns(key) {
			continue
		}
		if l := c.lostOption(key, b, d); l != nil {
			if lost == nil {
				lost = map[string]*LostPrerequisite{}
			}
			lost[key] = l
		}
	}
	return lost
}
