package rules

import (
	"fmt"
	"io/fs"
	"slices"
)

// Action economies, as in the grant_action effect.
const (
	EconomyAction      = "action"
	EconomyBonusAction = "bonus_action"
	EconomyReaction    = "reaction"
	EconomyFree        = "free"
	EconomyMovement    = "movement"
)

// Resource recharges, as in the resource effect.
const (
	RechargeShortRest = "short_rest"
	RechargeLongRest  = "long_rest"
	RechargeDawn      = "dawn"
	RechargeNone      = "none"
)

// Resource is a use-limited feature at the character's level: Second Wind
// (1 use), Ki points, Rage. The session spends and recovers it (RN-02).
type Resource struct {
	// Key is the effect's resource name, such as "second_wind".
	Key string
	// NamePT is the Portuguese name ("Retomar o Fôlego").
	NamePT string
	// Max is the number of uses. Unlimited uses (a barbarian's Rage at 20, a druid's Wild Shape at 20) are 99; the screens say "ilimitado".
	Max int
	// Recharge is a Recharge* constant.
	Recharge string
	// Source is the feature or trait key that gives it.
	Source string
}

// Action is something a character does on their turn: a standard action
// (Dash) or one a feature grants (Second Wind).
type Action struct {
	// Key is the feature key for a feature action ("feature:second-wind"),
	// or "standard:dash" for a standard one.
	Key    string
	NamePT string
	// Economy is an Economy* constant.
	Economy string
	// Resource is the key of the Resource each use spends, or "".
	Resource string
	// Source is the feature or trait key that grants it, "" for standard
	// actions.
	Source string
}

// standardAction is one entry of effects/standard_actions.json: the actions
// every character has, which the SRD only says in prose.
type standardAction struct {
	Key     string `json:"key"`
	NamePT  string `json:"name_pt"`
	Economy string `json:"economy"`
}

// loadStandardActions reads effects/standard_actions.json.
func (c *content) loadStandardActions(fsys fs.FS) error {
	var f struct {
		Actions []standardAction `json:"actions"`
	}
	if err := readJSON(fsys, "effects/standard_actions.json", &f); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, a := range f.Actions {
		switch {
		case a.Key == "" || a.NamePT == "":
			return fmt.Errorf("effects/standard_actions.json: every action needs a key and a name_pt")
		case seen[a.Key]:
			return fmt.Errorf("effects/standard_actions.json: duplicate key %q", a.Key)
		case !slices.Contains(economies, a.Economy):
			return fmt.Errorf("effects/standard_actions.json: %s: unknown economy %q", a.Key, a.Economy)
		}
		seen[a.Key] = true
		c.standardActions = append(c.standardActions, Action{Key: "standard:" + a.Key, NamePT: a.NamePT, Economy: a.Economy})
	}
	return nil
}

// resourcesAndActions fills Derived.Resources and Derived.Actions from the
// resource and grant_action effects of what the character has,
// StandardActions from the hand-written list, and AttacksPerAction from the
// extra_attack effects.
func (x *deriver) resourcesAndActions() {
	x.d.StandardActions = slices.Clone(x.c.standardActions)
	x.d.AttacksPerAction = 1
	for _, a := range x.active {
		if a.effect.Type == "extra_attack" && x.applies(a) {
			x.d.AttacksPerAction = max(x.d.AttacksPerAction, a.effect.Count)
		}
	}
	// Two classes may share a resource name (Channel Divinity of the cleric and
	// the paladin); the session counts them as one pool, which has the larger
	// Max: a second class gives new effects but no extra use (SRD Multiclassing).
	at := map[string]int{}
	for _, a := range x.active {
		e := a.effect
		if e.Type != "resource" || !x.applies(a) {
			continue
		}
		n, err := e.max.Int(x.env)
		if err != nil {
			x.issue(IssueFormula, "", "Um efeito de %s foi ignorado: a fórmula falhou.", x.c.namePT(a.owner))
			continue
		}
		if n <= 0 {
			continue
		}
		recharge := e.Recharge
		if e.rechargeIf != nil {
			switch ok, err := e.rechargeIf.Bool(x.env); {
			case err != nil:
				x.issue(IssueFormula, "", "Um efeito de %s foi ignorado: a condição falhou.", x.c.namePT(a.owner))
				continue
			case ok:
				recharge = e.RechargeThen
			}
		}
		name := x.c.namesPT["resource:"+e.Resource]
		if name == "" {
			name = x.c.namePT(a.owner)
		}
		r := Resource{Key: e.Resource, NamePT: name, Max: n, Recharge: recharge, Source: a.owner}
		if i, shared := at[e.Resource]; shared {
			if n > x.d.Resources[i].Max {
				x.d.Resources[i] = r
			}
			continue
		}
		at[e.Resource] = len(x.d.Resources)
		x.d.Resources = append(x.d.Resources, r)
	}
	for _, a := range x.active {
		if a.effect.Type != "grant_action" || !x.applies(a) {
			continue
		}
		act := Action{Key: a.owner, NamePT: x.c.namePT(a.owner), Economy: a.effect.Economy, Source: a.owner}
		// The resource of the same feature, when it has one at this level. The
		// action takes the resource's name: the SRD names some features per
		// level ("Surto de Ação (1 uso)"), and the screen shows the uses apart.
		for _, r := range x.d.Resources {
			if r.Source == a.owner {
				act.Resource = r.Key
				act.NamePT = r.NamePT
				break
			}
		}
		if !slices.ContainsFunc(x.d.Actions, func(o Action) bool { return o.Key == act.Key && o.Economy == act.Economy }) {
			x.d.Actions = append(x.d.Actions, act)
		}
	}
}
