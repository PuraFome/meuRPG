package rules

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// The picks of a feat (RN-23, MR-025): a `choice` effect with a Name is a pick the
// player makes (an ability, a damage type...), and a later effect of the same feat
// writes "@name" where the pick goes: proficiency "save.@ability", target
// "score.@ability", damage_type "damage-type:@type", or mod("@ability") in a formula.
// The token stands for the local part of the pick (the text after the first ":": the
// pick "ability:dex" is "dex"; an ability pick is the bare key "dex").
//
// This file is the one place that says which fields carry a token (pickFields,
// pickFormulaFields), checks them (checkPicks) and replaces them (substitutePick).
// The engine that reads the character's picks (Derive) calls substitutePick on the
// fields of pickFields, and resolves the formulas' "@name" in Env.Mod and Env.Score.

// The kinds of choice a token may refer to.
const (
	pickAbility    = "ability"
	pickDamageType = "damage_type"
)

// pickToken is "@name" inside a field.
var pickToken = regexp.MustCompile(`@([a-z0-9_]+)`)

// reservedPickName is the word after the "@" of a table key ("feature:x@mesa"): a pick
// cannot be called that, or a table key would read as a token.
var reservedPickName = strings.TrimPrefix(tableSuffix, "@")

// validPickName says whether s can name a pick: the rules of a state's name, and not
// the table suffix.
func validPickName(s string) bool { return formula.ValidStateName(s) && s != reservedPickName }

// pickField is a field of an Effect that holds a key, where a token may stand: its
// name in the effect, the kind of pick that fits it, and its strings.
type pickField struct {
	attr    string
	kind    string
	strings func(e *Effect) []*string
}

// pickFields are the key-valued fields of an effect that may carry "@name". The
// editor, the validator and the engine read this list, so the rule lives once.
var pickFields = []pickField{
	{"proficiency", pickAbility, func(e *Effect) []*string { return []*string{&e.Proficiency} }},
	{"target", pickAbility, func(e *Effect) []*string { return []*string{&e.Target} }},
	{"targets", pickAbility, func(e *Effect) []*string { return slicePtrs(e.Targets) }},
	{"ability", pickAbility, func(e *Effect) []*string { return []*string{&e.Ability} }},
	{"attack_ability", pickAbility, func(e *Effect) []*string { return []*string{&e.AttackAbility} }},
	{"damage_ability", pickAbility, func(e *Effect) []*string { return []*string{&e.DamageAbility} }},
	{"damage_type", pickDamageType, func(e *Effect) []*string { return []*string{&e.DamageType} }},
	{"damage_types", pickDamageType, func(e *Effect) []*string { return slicePtrs(e.DamageTypes) }},
	{"except", pickDamageType, func(e *Effect) []*string { return []*string{&e.Except} }},
}

// pickFormulaFields are the formulas of an effect, where mod("@name") and
// score("@name") may stand (an ability pick).
var pickFormulaFields = []pickField{
	{"value", pickAbility, func(e *Effect) []*string { return []*string{&e.Value} }},
	{"when", pickAbility, func(e *Effect) []*string { return []*string{&e.When} }},
	{"max", pickAbility, func(e *Effect) []*string { return []*string{&e.Max} }},
	{"prepared_max", pickAbility, func(e *Effect) []*string { return []*string{&e.PreparedMax} }},
	{"recharge_if", pickAbility, func(e *Effect) []*string { return []*string{&e.RechargeIf} }},
	{"dice", pickAbility, func(e *Effect) []*string { return []*string{&e.Dice} }},
	{"duration_rounds", pickAbility, func(e *Effect) []*string { return []*string{&e.DurationRounds} }},
	{"from_level", pickAbility, func(e *Effect) []*string { return []*string{&e.FromLevel} }},
	{"cost", pickAbility, func(e *Effect) []*string { return []*string{&e.Cost} }},
	{"cost_max", pickAbility, func(e *Effect) []*string { return []*string{&e.CostMax} }},
	{"temp_hp", pickAbility, func(e *Effect) []*string { return []*string{&e.TempHP} }},
	{"value_per_point", pickAbility, func(e *Effect) []*string { return []*string{&e.ValuePerPoint} }},
	{"max_cr_from", pickAbility, func(e *Effect) []*string { return []*string{&e.MaxCRFrom} }},
	{"form_cost", pickAbility, func(e *Effect) []*string { return []*string{&e.FormCost} }},
	{"gives_slot", pickAbility, func(e *Effect) []*string { return []*string{&e.GivesSlot} }},
	{"recover_max_level", pickAbility, func(e *Effect) []*string { return []*string{&e.RecoverMaxLevel} }},
	{"recover_levels", pickAbility, func(e *Effect) []*string { return []*string{&e.RecoverLevels} }},
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

// pickLocal is the local part of a pick: the text after the first ":", or the whole
// pick when it has none ("ability:dex" and "dex" are "dex"; "damage-type:fire" is
// "fire").
func pickLocal(pick string) string {
	if _, after, ok := strings.Cut(pick, ":"); ok {
		return after
	}
	return pick
}

// substitutePick replaces every token "@name" in field by local, the local part of the
// pick (pickLocal): "save.@ability" with "dex" is "save.dex". Another name's token and
// the table suffix are left alone. This is how the engine turns a field of an effect
// of a feat into the key the character's pick makes of it.
func substitutePick(field, name, local string) string {
	if name == reservedPickName || !strings.Contains(field, PickMark+name) {
		return field
	}
	return pickToken.ReplaceAllStringFunc(field, func(m string) string {
		if m[len(PickMark):] == name {
			return local
		}
		return m
	})
}

// PickMark starts a token: "@" in "@ability".
const PickMark = formula.PickPrefix

// checkPicks checks the names a choice declares and the tokens an effect writes, against
// the picks the owner's earlier effects declared (declared maps a name to the kind of
// its choice), and declares the effect's own name. A name is declared by a choice of a
// feat; a token refers to an earlier one of the same entry, and its field takes the
// kind of the pick. Every problem is at the field's own path. It also sets the picks the
// effect reads (Effect.picks) for the compile.
func (b *overlayBuilder) checkPicks(site effectSite, e *Effect, declared map[string]string) error {
	if err := checkPickName(site, e, declared); err != nil {
		return err
	}
	used := false
	for _, f := range slices.Concat(pickFields, pickFormulaFields) {
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

// checkPickName checks the name a choice gives: only a choice of a feat has one, in the
// letters of a state's name, once in the entry.
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

// samplePicks puts a valid sample (the first ability, a damage type of the content) in
// place of every token of the key fields of e, so the checks of the effect judge the rest
// of the key ("score.@ability" is "score.str"), and returns the function that puts the
// tokens back. The formulas are not touched: they compile with their tokens.
func (c *content) samplePicks(e *Effect) (restore func()) {
	if len(e.picks) == 0 {
		return func() {}
	}
	type saved struct {
		p *string
		s string
	}
	var undo []saved
	for _, f := range pickFields {
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

// pickKeyExists says whether k is a key a choice of the kind may list in `from`: an
// ability for the picks of abilities, a damage type for the picks of damage types,
// anything the content has for the rest.
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
