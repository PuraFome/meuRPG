package rules

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The picks of a feat (RN-23, MR-025).
// A `choice` effect with a Name is a pick the player makes: an ability or a damage type.
// A later effect of the same feat writes "@name" where the pick goes.
// Examples are proficiency "save.@ability", damage_type "damage-type:@type" and mod("@ability").
// The token stands for the local part of the pick, the text after the first ":".
// The pick "ability:dex" is "dex", and "damage-type:fire" is "fire".
//
// This file is the one place that says which fields carry a token (pickFields).
// It checks the tokens at import (checkPicks) and replaces them (substitutePick).
// Derive reads the sheet's picks (pickValues) and applies a copy of each effect with its tokens replaced (resolvePicks).
// An effect whose pick is missing or not offered does not apply.
// The choice engine asks the picks as choices keyed by PickChoiceKey (feat_choices.go).

// The kinds of choice a token may refer to.
const (
	pickAbility    = "ability"
	pickDamageType = "damage_type"
)

// pickToken is "@name" inside a field.
var pickToken = regexp.MustCompile(`@([a-z0-9_]+)`)

// reservedPickName is the word after the "@" of a table key ("feature:x@mesa").
// A pick cannot be called that, or a table key would read as a token.
var reservedPickName = strings.TrimPrefix(tableSuffix, "@")

// validPickName says whether s can name a pick.
// It follows the rules of a state's name and is not the table suffix.
func validPickName(s string) bool { return formula.ValidStateName(s) && s != reservedPickName }

// pickField is a field of an Effect where a token may stand.
// attr is its name in the effect.
// kind is the kind of pick that fits it.
// formula says the field is a formula, where the token is mod("@x") or score("@x").
// strings reads the field, as pointers so the caller may replace it.
type pickField struct {
	attr    string
	kind    string
	formula bool
	strings func(e *Effect) []*string
	program func(e *Effect) **formula.Program
}

// keyField, keyListField and formulaKeyField make the rows of pickFields.
func keyField(attr, kind string, p func(e *Effect) *string) pickField {
	return pickField{attr: attr, kind: kind, strings: func(e *Effect) []*string { return []*string{p(e)} }}
}

func formulaKeyField(attr string, p func(e *Effect) *string, prog func(e *Effect) **formula.Program) pickField {
	f := keyField(attr, pickAbility, p)
	f.formula, f.program = true, prog
	return f
}

func keyListField(attr, kind string, list func(e *Effect) []string) pickField {
	return pickField{attr: attr, kind: kind, strings: func(e *Effect) []*string { return slicePtrs(list(e)) }}
}

// pickFields are the fields of an effect that may carry "@name".
// The validator, the sampler and the engine read this one table.
// A formula field takes ability picks only.
var pickFields = []pickField{
	keyField("proficiency", pickAbility, func(e *Effect) *string { return &e.Proficiency }),
	keyField("target", pickAbility, func(e *Effect) *string { return &e.Target }),
	keyListField("targets", pickAbility, func(e *Effect) []string { return e.Targets }),
	keyField("ability", pickAbility, func(e *Effect) *string { return &e.Ability }),
	keyField("attack_ability", pickAbility, func(e *Effect) *string { return &e.AttackAbility }),
	keyField("damage_ability", pickAbility, func(e *Effect) *string { return &e.DamageAbility }),
	keyField("damage_type", pickDamageType, func(e *Effect) *string { return &e.DamageType }),
	keyListField("damage_types", pickDamageType, func(e *Effect) []string { return e.DamageTypes }),
	keyField("except", pickDamageType, func(e *Effect) *string { return &e.Except }),
	formulaKeyField("value", func(e *Effect) *string { return &e.Value }, func(e *Effect) **formula.Program { return &e.value }),
	formulaKeyField("when", func(e *Effect) *string { return &e.When }, func(e *Effect) **formula.Program { return &e.when }),
	formulaKeyField("max", func(e *Effect) *string { return &e.Max }, func(e *Effect) **formula.Program { return &e.max }),
	formulaKeyField("prepared_max", func(e *Effect) *string { return &e.PreparedMax }, func(e *Effect) **formula.Program { return &e.preparedMax }),
	formulaKeyField("recharge_if", func(e *Effect) *string { return &e.RechargeIf }, func(e *Effect) **formula.Program { return &e.rechargeIf }),
	formulaKeyField("dice", func(e *Effect) *string { return &e.Dice }, func(e *Effect) **formula.Program { return &e.dice }),
	formulaKeyField("duration_rounds", func(e *Effect) *string { return &e.DurationRounds }, func(e *Effect) **formula.Program { return &e.durationRounds }),
	formulaKeyField("from_level", func(e *Effect) *string { return &e.FromLevel }, func(e *Effect) **formula.Program { return &e.fromLevel }),
	formulaKeyField("cost", func(e *Effect) *string { return &e.Cost }, func(e *Effect) **formula.Program { return &e.cost }),
	formulaKeyField("cost_max", func(e *Effect) *string { return &e.CostMax }, func(e *Effect) **formula.Program { return &e.costMax }),
	formulaKeyField("temp_hp", func(e *Effect) *string { return &e.TempHP }, func(e *Effect) **formula.Program { return &e.tempHP }),
	formulaKeyField("value_per_point", func(e *Effect) *string { return &e.ValuePerPoint }, func(e *Effect) **formula.Program { return &e.perPointFlat }),
	formulaKeyField("max_cr_from", func(e *Effect) *string { return &e.MaxCRFrom }, func(e *Effect) **formula.Program { return &e.maxCRFrom }),
	formulaKeyField("form_cost", func(e *Effect) *string { return &e.FormCost }, func(e *Effect) **formula.Program { return &e.formCost }),
	formulaKeyField("gives_slot", func(e *Effect) *string { return &e.GivesSlot }, func(e *Effect) **formula.Program { return &e.givesSlot }),
	formulaKeyField("recover_max_level", func(e *Effect) *string { return &e.RecoverMaxLevel }, func(e *Effect) **formula.Program { return &e.recoverMaxLevel }),
	formulaKeyField("recover_levels", func(e *Effect) *string { return &e.RecoverLevels }, func(e *Effect) **formula.Program { return &e.recoverLevels }),
}

func slicePtrs(s []string) []*string {
	out := make([]*string, len(s))
	for i := range s {
		out[i] = &s[i]
	}
	return out
}

// pickNames are the names the tokens in s write, in order, once each. The table suffix
// is not a token.
func pickNames(s string) []string {
	var out []string
	for _, m := range pickToken.FindAllStringSubmatch(s, -1) {
		if m[1] != reservedPickName && !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// pickLocal is the local part of a pick: the text after the first ":".
// A pick with no ":" is its own local part.
func pickLocal(pick string) string {
	if _, after, ok := strings.Cut(pick, ":"); ok {
		return after
	}
	return pick
}

// substitutePick replaces every token "@name" in field by local, the local part of a pick.
// So "save.@ability" with "dex" is "save.dex".
// The token of another name and the table suffix are left alone.
func substitutePick(field, name, local string) string {
	if name == reservedPickName || !strings.Contains(field, formula.PickPrefix+name) {
		return field
	}
	return pickToken.ReplaceAllStringFunc(field, func(m string) string {
		if m[len(formula.PickPrefix):] == name {
			return local
		}
		return m
	})
}

// checkPicks checks the name a choice declares and the tokens an effect writes.
// declared maps the names of the owner's earlier choices to their kind.
// A token must name an earlier choice of the same feat, of the kind its field takes.
// Every problem is at the field's own path.
// It also sets Effect.picks on an effect that reads a token, for the compile and for Derive.
func (b *overlayBuilder) checkPicks(site effectSite, e *Effect, declared map[string]string) error {
	if err := checkPickName(site, e, declared); err != nil {
		return err
	}
	used := false
	for _, f := range pickFields {
		for _, p := range f.strings(e) {
			for _, name := range pickNames(*p) {
				if err := checkPickToken(site, "."+f.attr, f.kind, name, declared); err != nil {
					return err
				}
				used = true
			}
		}
	}
	if used {
		e.picks = maps.Clone(declared)
	}
	if e.Name != "" {
		declared[e.Name] = e.Choice
	}
	return nil
}

// checkPickName checks the name a choice gives.
// Only a choice of a feat has one, it is valid, picks one, and is not taken.
func checkPickName(site effectSite, e *Effect, declared map[string]string) error {
	if e.Name == "" {
		return nil
	}
	switch {
	case e.Type != "choice":
		return site.fail(".name", ReasonValue, "only a choice has a name")
	case !strings.HasPrefix(site.owner, "feat:"):
		return site.fail(".name", ReasonValue, "only a choice of a feat has a name: only a feat reads its picks")
	case !validPickName(e.Name):
		return site.fail(".name", ReasonValue, "a name has 1 to %d characters of a-z, 0-9 and _, and is not %q", formula.MaxStateNameBytes, reservedPickName)
	}
	if e.Count != 1 {
		return site.fail(".count", ReasonValue, "a named choice picks one: @%s stands for a single pick", e.Name)
	}
	if _, taken := declared[e.Name]; taken {
		return site.fail(".name", ReasonValue, "two choices of this feat are named %q", e.Name)
	}
	return nil
}

// checkPickToken checks one "@name" in the field attr, which takes picks of the given kind.
func checkPickToken(site effectSite, attr, kind, name string, declared map[string]string) error {
	if !strings.HasPrefix(site.owner, "feat:") {
		return site.fail(attr, ReasonValue, "@%s: only the effects of a feat itself read the picks of its choices", name)
	}
	got, ok := declared[name]
	if !ok {
		return site.fail(attr, ReasonReference, "@%s is not the name of a choice earlier in this feat", name)
	}
	if got != kind {
		return site.fail(attr, ReasonValue, "@%s is a pick of %q; this field takes a pick of %q", name, got, kind)
	}
	return nil
}

// samplePicks puts a valid sample in place of every token of the key fields of e.
// The sample is the first ability, or a damage type of the content.
// So the checks of the effect judge the rest of the key: "score.@ability" is "score.str".
// It returns the function that puts the tokens back.
// The formulas are not touched: they compile with their tokens.
func (c *content) samplePicks(e *Effect) (restore func()) {
	if len(e.picks) == 0 {
		return func() {}
	}
	type saved struct {
		p *string
		s string
	}
	var undo []saved
	// The lists are replaced by copies while sampled: the caller's slices are never written.
	targets, damageTypes := e.Targets, e.DamageTypes
	e.Targets, e.DamageTypes = slices.Clone(targets), slices.Clone(damageTypes)
	for _, f := range pickFields {
		if f.formula {
			continue
		}
		for _, p := range f.strings(e) {
			orig := *p
			for _, name := range pickNames(orig) {
				*p = substitutePick(*p, name, c.pickSample(f.kind))
			}
			if *p != orig {
				undo = append(undo, saved{p, orig})
			}
		}
	}
	return func() {
		for _, u := range undo {
			*u.p = u.s
		}
		e.Targets, e.DamageTypes = targets, damageTypes
	}
}

// pickSample is a valid local part of a pick of the kind.
func (c *content) pickSample(kind string) string {
	if kind != pickDamageType {
		return string(STR)
	}
	for _, k := range sortedKeys(c.named) {
		if c.isLiveDamageType(k) {
			return pickLocal(k)
		}
	}
	return "fire"
}

// pickKeyExists says whether k is a key a choice of the kind may list in `from`.
// It is an ability, a damage type, or any key of the content for the other kinds.
func (c *content) pickKeyExists(kind, k string) bool {
	switch kind {
	case pickAbility:
		_, ok := abilityIndex[Ability(k)]
		return ok
	case pickDamageType:
		return c.isLiveDamageType(k)
	}
	return c.exists(k)
}

// PickChoiceKey is the key of the choice a feat's named choice asks.
// A pick of it is stored as "<feat key>#<name>=<value>".
func PickChoiceKey(owner, name string) string { return owner + textSeparator + name }

// namedPickChoices are the named choices of an ability or a damage type that owner asks.
func (c *content) namedPickChoices(owner string) []*Effect {
	var out []*Effect
	for _, e := range c.effects[owner] {
		if e.Type == "choice" && e.Name != "" && (e.Choice == pickAbility || e.Choice == pickDamageType) {
			out = append(out, e)
		}
	}
	return out
}

// pickOffered says whether value is a pick the choice e offers.
// An ability is written "ability:dex" and a damage type is its key.
func (c *content) pickOffered(e *Effect, value string) bool {
	switch e.Choice {
	case pickAbility:
		local, ok := strings.CutPrefix(value, abilityPrefix)
		_, known := abilityIndex[Ability(local)]
		return ok && known && (len(e.From) == 0 || slices.Contains(e.From, local))
	case pickDamageType:
		return c.isLiveDamageType(value) && (len(e.From) == 0 || slices.Contains(e.From, value))
	}
	return false
}

// sheetPick is the pick the stored picks make of the named choice e of owner.
// It is the first stored value the choice offers, or false when there is none.
func (c *content) sheetPick(stored []string, owner string, e *Effect) (string, bool) {
	key := PickChoiceKey(owner, e.Name)
	for _, k := range stored {
		if choiceKey, value, ok := SplitScopedChoice(k); ok && choiceKey == key && c.pickOffered(e, value) {
			return value, true
		}
	}
	return "", false
}

// pickValues are the picks the stored picks make of the named choices of owner, by name.
func (c *content) pickValues(stored []string, owner string) map[string]string {
	var out map[string]string
	for _, e := range c.namedPickChoices(owner) {
		if v, ok := c.sheetPick(stored, owner, e); ok {
			if out == nil {
				out = map[string]string{}
			}
			out[e.Name] = v
		}
	}
	return out
}

// resolvePicks is a copy of e with each token replaced by the local part of its pick in values.
// The formulas that name a pick are compiled again with the pick written out.
// It is false when e reads a pick that values lacks: the effect does not apply.
// An effect that reads no pick is returned as it is.
func (c *content) resolvePicks(e *Effect, values map[string]string) (*Effect, bool) {
	if len(e.picks) == 0 {
		return e, true
	}
	out := *e
	out.Targets, out.DamageTypes = slices.Clone(e.Targets), slices.Clone(e.DamageTypes)
	out.picks = nil
	for _, f := range pickFields {
		for _, p := range f.strings(&out) {
			orig := *p
			for _, name := range pickNames(orig) {
				v, ok := values[name]
				if !ok {
					return nil, false
				}
				*p = substitutePick(*p, name, pickLocal(v))
			}
		}
		if f.formula && !c.recompilePick(&out, f) {
			return nil, false
		}
	}
	return &out, true
}

// recompilePick compiles the formula of f in e again when its text changed.
// It says whether that worked.
func (c *content) recompilePick(e *Effect, f pickField) bool {
	prog := f.program(e)
	if *prog == nil || *f.strings(e)[0] == (*prog).Source() {
		return true
	}
	compiler := c.compiler
	if f.attr == "when" {
		compiler = compiler.WithStates()
	}
	p, err := compiler.Compile(*f.strings(e)[0], (*prog).Kind())
	if err != nil {
		return false
	}
	*prog = p
	return true
}

// optionSetKeys are the options of an option set the content has: the SRD's and the table's that joined it.
func (c *content) optionSetKeys(set string) []string {
	def, ok := optionSetOf(set)
	if !ok {
		return nil
	}
	var out []string
	for _, k := range def.Offerers {
		if f := c.features[k]; f != nil {
			for _, o := range f.Options {
				if !slices.Contains(out, o) {
					out = append(out, o)
				}
			}
		}
	}
	return out
}

// featOptionKeys are the options a feat offers: the list of its choice of features, or the options of the set it offers.
func (c *content) featOptionKeys(f *srd51.Feat) []string {
	if f.OptionSet != "" {
		return c.optionSetKeys(f.OptionSet)
	}
	return f.Options
}

// offerFeatSets makes a feat that offers an option set the offerer of every option of the set.
// It runs once every entry of the overlay is in, so the table's options that joined the set count.
func (b *overlayBuilder) offerFeatSets() {
	n := b.n
	for _, k := range sortedKeys(n.feats) {
		if f := n.feats[k]; f.OptionSet != "" {
			for _, o := range n.optionSetKeys(f.OptionSet) {
				n.offeredBy[o] = append(n.offeredBy[o], k)
			}
		}
	}
}

// picksOf are the picks Derived.FeatPicks holds for the named choices of owner, by name.
func (d Derived) picksOf(owner string) map[string]string {
	var out map[string]string
	for k, v := range d.FeatPicks {
		if name, ok := strings.CutPrefix(k, PickChoiceKey(owner, "")); ok {
			if out == nil {
				out = map[string]string{}
			}
			out[name] = v
		}
	}
	return out
}
