package rules

import (
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// The scopes a feature use's limit counts in (FeatureUse.LimitScope).
const (
	UseScopeTurn   = "turn"
	UseScopeRound  = "round"
	UseScopeCombat = "combat"
)

// Whom a feature use may be aimed at (FeatureTargets.Who).
const (
	FeatureWhoSelf  = "self"
	FeatureWhoAlly  = "ally"
	FeatureWhoEnemy = "enemy"
	FeatureWhoAny   = "any"
)

// The limits of what a feature use declares.
const (
	maxFeatureTargets      = 20
	maxFeatureRangeFt      = 300
	maxFeatureChoices      = 8
	maxChoiceKeyLen        = 40
	maxChoiceTextRunes     = 80
	maxFeatureEffectRounds = 14400
)

var (
	useScopes   = []string{UseScopeTurn, UseScopeRound, UseScopeCombat}
	featureWhos = []string{FeatureWhoSelf, FeatureWhoAlly, FeatureWhoEnemy, FeatureWhoAny}
)

// FeatureUse is what a table feature's grant_action declares to be used through the UseFeature call, not
// TakeAction (TableEffect.use). LimitScope with LimitUses limits the uses in a turn, a round or a combat
// (LimitUses is an Int formula); Targets says whom the use is aimed at, Choices the options the player picks
// one of, Applies what the use does besides spending.
type FeatureUse struct {
	LimitScope string          `json:"limit_scope,omitempty"`
	LimitUses  string          `json:"limit_uses,omitempty"`
	Targets    *FeatureTargets `json:"targets,omitempty"`
	Choices    []FeatureChoice `json:"choices,omitempty"`
	Applies    *FeatureApplies `json:"applies,omitempty"`
}

// FeatureTargets are the creatures a use is aimed at: Min to Max of Who, within RangeFt feet (0: no limit).
type FeatureTargets struct {
	Min     int    `json:"min,omitempty"`
	Max     int    `json:"max,omitempty"`
	Who     string `json:"who,omitempty"`
	RangeFt int    `json:"range_ft,omitempty"`
}

// FeatureChoice is one option of a use: a key the server checks and the label the player reads.
type FeatureChoice struct {
	Key    string `json:"key"`
	TextPT string `json:"text_pt"`
}

// FeatureApplies is what a use does to its targets (the user when it has none): Heal is dice (as the heal of a
// grant_action), TempHP an Int formula, Effect a lasting effect that lasts EffectRounds rounds (its Applies is
// ignored: it is ALL).
type FeatureApplies struct {
	Heal         string            `json:"heal,omitempty"`
	TempHP       string            `json:"temp_hp,omitempty"`
	Effect       *TableSpellEffect `json:"effect,omitempty"`
	EffectRounds int               `json:"effect_rounds,omitempty"`
}

// empty says the targets ask for nothing.
func (t *FeatureTargets) empty() bool { return t == nil || *t == FeatureTargets{} }

// empty says the applies do nothing.
func (a *FeatureApplies) empty() bool {
	return a == nil || (a.Heal == "" && a.TempHP == "" && a.Effect == nil && a.EffectRounds == 0)
}

// declaresNothing says the use is an empty message.
func (u *FeatureUse) declaresNothing() bool {
	return u.LimitScope == "" && u.LimitUses == "" && u.Targets.empty() && len(u.Choices) == 0 && u.Applies.empty()
}

// ActionUse is what Derive works out of the use a grant_action declares, with the character's formulas:
// Action.Use, nil for an action that is taken with TakeAction.
type ActionUse struct {
	// Scope is a UseScope* constant and Uses the uses allowed in it at the character's level (0: none at this
	// level). Scope "" has no limit of this kind (the resource may still limit the action).
	Scope string `json:",omitempty"`
	Uses  int    `json:",omitempty"`
	// Targets is what the use asks for (all zero when it takes none).
	Targets FeatureTargets `json:",omitzero"`
	// Choices are the options the player picks one of.
	Choices []FeatureChoice `json:",omitempty"`
	// Result is what the use heals and gives to each target: the dice and the temporary hit points.
	Result ActionResult `json:",omitzero"`
	// HasEffect says the use puts a lasting effect on its targets for EffectRounds rounds; the effect is
	// Content.FeatureUseEffect(action key).
	HasEffect    bool `json:",omitempty"`
	EffectRounds int  `json:",omitempty"`
}

// Limited says the use counts against a limit in a scope.
func (u *ActionUse) Limited() bool { return u != nil && u.Scope != "" }

// Left is how many uses remain in the scope when spent have been used in it. It is the seam the play module
// fills: the uses spent in the turn, round or combat are the combatant's state (combatants.feature_state), which
// the rules never read. Without a limit it is -1 (no limit of this kind).
func (u *ActionUse) Left(spent int) int {
	if !u.Limited() {
		return -1
	}
	return max(u.Uses-max(spent, 0), 0)
}

// ViaUseFeature says the action is taken with UseFeature, not TakeAction: the feature declares a use. TakeAction
// refuses its key.
func (a Action) ViaUseFeature() bool { return a.Use != nil }

// FeatureUseEffect is the lasting effect a feature use puts on its targets, by the feature's action key, and
// whether the feature has one. The result is shared and must not be modified.
func (c *Content) FeatureUseEffect(actionKey string) (*EffectDef, bool) {
	return c.CombatSpellEffect(actionKey)
}

// useOf works the declared use out for the character; it reports false (the action is left out, an Issue is
// recorded) when a formula fails, so the action is never taken the old way by mistake.
func (x *deriver) useOf(act *Action, a activeEffect) bool {
	u := a.effect.Use
	if u == nil {
		return true
	}
	out := &ActionUse{Scope: u.LimitScope}
	if a.effect.useLimit != nil {
		n, ok := x.evalInt(a.effect.useLimit, a)
		if !ok {
			return false
		}
		out.Uses = max(n, 0)
	}
	if t := u.Targets; t != nil {
		out.Targets = *t
	}
	out.Choices = slices.Clone(u.Choices)
	if ap := u.Applies; ap != nil {
		if d, ok := parseHealDice(ap.Heal); ok {
			out.Result.Heal = &d
		}
		if a.effect.useTempHP != nil {
			n, ok := x.evalInt(a.effect.useTempHP, a)
			if !ok {
				return false
			}
			out.Result.TempHP = max(n, 0)
		}
		out.HasEffect, out.EffectRounds = ap.Effect != nil, ap.EffectRounds
	}
	act.Use = out
	return true
}

// compileUse compiles the formulas of the use of a grant_action.
func (x effectCompiler) compileUse() error {
	u := x.e.Use
	if u == nil {
		return nil
	}
	var err error
	if x.e.useLimit, err = x.compile(u.LimitUses, formula.Int); err != nil {
		return err
	}
	if u.Applies != nil {
		x.e.useTempHP, err = x.compile(u.Applies.TempHP, formula.Int)
	}
	return err
}

// checkUse checks what a grant_action declares of its use. Every field that is wrong is reported at its path.
func (b *overlayBuilder) checkUse(site effectSite, e *Effect) error {
	u := e.Use
	if u == nil {
		return nil
	}
	var c entryErrors
	at := func(attr, reason, format string, args ...any) {
		c.at(site.owner, site.path, ".use"+attr, reason, format, args...)
	}
	if u.declaresNothing() {
		at("", ReasonValue, "a use that declares nothing: give it a limit, targets, choices or what it applies")
		return c.err()
	}
	checkUseLimit(u, at)
	checkUseTargets(u.Targets, at)
	checkUseChoices(u.Choices, at)
	b.checkUseApplies(&c, site, u.Applies)
	if err := c.err(); err != nil {
		return err
	}
	if u.Applies != nil && u.Applies.Effect != nil {
		b.registerUseEffect(site, u.Applies.Effect)
	}
	return nil
}

type useRefuse func(attr, reason, format string, args ...any)

func checkUseLimit(u *FeatureUse, at useRefuse) {
	switch {
	case u.LimitScope != "" && !slices.Contains(useScopes, u.LimitScope):
		at(".limit_scope", ReasonValue, "the scope is turn, round or combat")
	case u.LimitScope != "" && u.LimitUses == "":
		at(".limit_uses", ReasonValue, "limit_scope needs limit_uses: the uses allowed in the scope")
	case u.LimitScope == "" && u.LimitUses != "":
		at(".limit_scope", ReasonValue, "limit_uses needs limit_scope: the turn, the round or the combat")
	}
	if n, err := strconv.Atoi(u.LimitUses); err == nil && n < 1 {
		at(".limit_uses", ReasonValue, "limit_uses must be at least 1")
	}
}

func checkUseTargets(t *FeatureTargets, at useRefuse) {
	if t.empty() {
		return
	}
	switch {
	case t.Min < 0 || t.Min > maxFeatureTargets:
		at(".targets.min", ReasonValue, "min is 0 to %d", maxFeatureTargets)
	case t.Max > maxFeatureTargets || t.Max < t.Min:
		at(".targets.max", ReasonValue, "max is from min to %d", maxFeatureTargets)
	}
	switch {
	case t.Who != "" && !slices.Contains(featureWhos, t.Who):
		at(".targets.who", ReasonValue, "who is self, ally, enemy or any")
	case t.Max > 0 && t.Who == "":
		at(".targets.who", ReasonValue, "who is needed when the use takes targets")
	case t.Who == FeatureWhoSelf && t.Max > 1:
		at(".targets.max", ReasonValue, "a use aimed at self takes at most 1 target")
	}
	switch {
	case t.RangeFt < 0 || t.RangeFt > maxFeatureRangeFt:
		at(".targets.range_ft", ReasonValue, "range_ft is 0 to %d", maxFeatureRangeFt)
	case t.RangeFt > 0 && t.Who == FeatureWhoSelf:
		at(".targets.range_ft", ReasonValue, "a use aimed at self has no range")
	case t.RangeFt > 0 && t.Max == 0:
		at(".targets.range_ft", ReasonValue, "a range needs targets")
	}
}

func checkUseChoices(choices []FeatureChoice, at useRefuse) {
	if len(choices) > maxFeatureChoices {
		at(".choices", ReasonLimit, "a use offers at most %d choices", maxFeatureChoices)
	}
	for i, ch := range choices {
		idx := "[" + strconv.Itoa(i) + "]"
		switch {
		case !validResourceName(ch.Key):
			at(".choices"+idx+".key", ReasonValue, "the key is 1 to %d characters of a-z, 0-9 and _", maxChoiceKeyLen)
		case slices.ContainsFunc(choices[:i], func(o FeatureChoice) bool { return o.Key == ch.Key }):
			at(".choices"+idx+".key", ReasonValue, "the key %q is listed twice", ch.Key)
		}
		if n := utf8.RuneCountInString(ch.TextPT); n < 1 || n > maxChoiceTextRunes {
			at(".choices"+idx+".text_pt", ReasonValue, "the text is 1 to %d characters", maxChoiceTextRunes)
		}
	}
}

// checkUseApplies checks what the use applies; the lasting effect goes through the checks of a table spell's.
func (b *overlayBuilder) checkUseApplies(c *entryErrors, site effectSite, ap *FeatureApplies) {
	if ap == nil {
		return
	}
	path := site.path + ".use.applies"
	if ap.Heal != "" {
		if _, ok := parseHealDice(ap.Heal); !ok {
			c.at(site.owner, path, ".heal", ReasonValue, "heal is dice such as 1d6 or 2d4+2, with at least one die")
		}
	}
	if ap.EffectRounds != 0 && ap.Effect == nil {
		c.at(site.owner, path, ".effect_rounds", ReasonValue, "effect_rounds needs an effect")
	}
	if ap.Effect == nil {
		return
	}
	if ap.EffectRounds < 1 || ap.EffectRounds > maxFeatureEffectRounds {
		c.at(site.owner, path, ".effect_rounds", ReasonValue, "effect_rounds is 1 to %d: how long the effect lasts", maxFeatureEffectRounds)
	}
	if a := ap.Effect.Applies; a != "" && a != "all" {
		c.at(site.owner, path, ".effect.applies", ReasonValue, "a feature's effect applies to all its targets: it has no save or attack")
	}
	// The spell checks run on the effect without its applies, with a timed duration: the effect lasts as the use says.
	ef := *ap.Effect
	ef.Applies = ""
	ts := &TableSpell{TableEntry: TableEntry{Key: site.owner}, Effect: &ef, Duration: TableDuration{Kind: DurationTimed}}
	b.checkSpellEffect(c, site.owner, path, ts, SpellTarget{Kind: TargetCreature})
}

// registerUseEffect keeps the lasting effect of a feature's use under the feature's key, as a table spell's.
func (b *overlayBuilder) registerUseEffect(site effectSite, eff *TableSpellEffect) {
	if b.n.tableSpellEffects == nil {
		return
	}
	if _, ok := b.n.tableSpellEffects[site.owner]; ok {
		return
	}
	ef := *eff
	ef.Applies = ""
	def := tableEffectDef(&TableSpell{TableEntry: TableEntry{Key: site.owner, NamePT: b.n.namePT(site.owner)}, Effect: &ef})
	def.Kind = "feature"
	b.n.tableSpellEffects[site.owner] = def
}
