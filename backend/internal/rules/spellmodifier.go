package rules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/formula"
)

// The spell_modifier effect (table content only): a cast option a feature offers for the spells it
// fits. The engine is generic; the table names it.

// What a spell_modifier changes in the spell's damage (its `spell_change`).
const (
	// SpellChangeDamageTypeSwap turns the damage of one of the effect's types into another of them,
	// which the caster picks at the cast.
	SpellChangeDamageTypeSwap = "damage_type_swap"
	// SpellChangeBonusDamage adds a fixed damage, of the spell's own type or of one the effect names.
	SpellChangeBonusDamage = "bonus_damage"
	// SpellChangeExtraDice adds dice of the spell's damage die for each slot level above the spell's own.
	SpellChangeExtraDice = "extra_dice_per_slot_over"
)

// Which spells a spell_modifier fits (its `applies_to`), besides an SRD school key ("school:evocation") or an
// SRD damage type key ("damage-type:fire").
const (
	SpellAppliesDamageSpell = "damage_spell"
	SpellAppliesCantrip     = "cantrip"
)

var (
	spellChanges      = []string{SpellChangeDamageTypeSwap, SpellChangeBonusDamage, SpellChangeExtraDice}
	spellAppliesWords = []string{SpellAppliesDamageSpell, SpellAppliesCantrip}
)

// Bounds of a spell_modifier: the types a swap lists, the fixed damage and the dice per slot level.
const (
	minSwapTypes        = 2
	maxSpellBonusDamage = 99
	maxSpellExtraDice   = 6
)

// SpellModifier is a spell_modifier effect of a feature the character has, worked out for the character.
type SpellModifier struct {
	// Key is the feature that gives it, NamePT its name.
	Key, NamePT string
	// AppliesTo is which spells it fits and Change what it does (the SpellChange* constants).
	AppliesTo, Change string
	// From are the damage types a swap is among.
	From []string
	// Bonus is the fixed damage of bonus_damage, of BonusType (empty is the spell's own).
	Bonus     int
	BonusType string
	// ExtraDice are the dice per slot level above the spell's own.
	ExtraDice int
	// Resource is the resource each use spends, one use; empty for none.
	Resource string
	// TextPT is the line the table reads.
	TextPT string
}

// spellModifierTarget says the value is one `applies_to` accepts.
func (c *content) spellModifierTarget(v string) bool {
	switch {
	case slices.Contains(spellAppliesWords, v):
		return true
	case strings.HasPrefix(v, "school:"):
		n, ok := c.named[v]
		return ok && n != nil
	}
	return c.isLiveDamageType(v)
}

// spellModifierShape names the field of a spell_modifier that is wrong and why (both "" when right). The
// checks that need the content (the keys) are spellModifierKeys'.
func spellModifierShape(e *Effect) (field, msg string) {
	if e.AppliesTo == "" {
		return "applies_to", "applies_to is required: which spells it fits"
	}
	if !slices.Contains(spellChanges, e.SpellChange) {
		return "spell_change", fmt.Sprintf("unknown spell_change %q", e.SpellChange)
	}
	switch e.SpellChange {
	case SpellChangeDamageTypeSwap:
		return swapShape(e)
	case SpellChangeBonusDamage:
		return bonusShape(e)
	}
	return diceShape(e)
}

func swapShape(e *Effect) (field, msg string) {
	switch {
	case len(e.From) < minSwapTypes || len(slices.Compact(slices.Sorted(slices.Values(e.From)))) != len(e.From):
		return "from", "a damage type swap lists at least two different damage types in from"
	case e.Value != "" || e.Count != 0 || e.DamageType != "":
		return "value", "a damage type swap takes from and nothing else of the change (no value, count or damage_type)"
	}
	return "", ""
}

func bonusShape(e *Effect) (field, msg string) {
	switch {
	case e.Value == "":
		return "value", "bonus_damage needs a value: the fixed damage added"
	case len(e.From) > 0 || e.Count != 0:
		return "from", "bonus_damage takes value and damage_type only"
	}
	return "", ""
}

func diceShape(e *Effect) (field, msg string) {
	switch {
	case e.Count < 1 || e.Count > maxSpellExtraDice:
		return "count", fmt.Sprintf("extra_dice_per_slot_over needs count 1 to %d: the dice added for each slot level above the spell's", maxSpellExtraDice)
	case len(e.From) > 0 || e.Value != "" || e.DamageType != "":
		return "value", "extra_dice_per_slot_over takes count only"
	}
	return "", ""
}

// spellModifierKeys checks the keys of a spell_modifier against the content: the spells it fits and the damage
// types it names.
func (c *content) spellModifierKeys(e *Effect) (field, msg string) {
	if !c.spellModifierTarget(e.AppliesTo) {
		return "applies_to", fmt.Sprintf("applies_to %q is not on the table's menu", e.AppliesTo)
	}
	for _, t := range e.From {
		if !c.isLiveDamageType(t) {
			return "from", fmt.Sprintf("from %q is not a damage type", t)
		}
	}
	if e.DamageType != "" && !c.isLiveDamageType(e.DamageType) {
		return "damage_type", fmt.Sprintf("damage_type %q is not a damage type", e.DamageType)
	}
	return "", ""
}

// spellModifier checks the fields of a spell_modifier effect and compiles its fixed damage.
func (x effectCompiler) spellModifier() error {
	e := x.e
	if field, msg := spellModifierShape(e); field != "" {
		return x.fail("%s", msg)
	}
	if field, msg := x.c.spellModifierKeys(e); field != "" {
		return x.fail("%s", msg)
	}
	var err error
	e.value, err = x.compile(e.Value, formula.Int)
	return err
}

// checkSpellModifier checks a spell_modifier of the table's content, with the field each problem is at.
func (b *overlayBuilder) checkSpellModifier(site effectSite, e *Effect) error {
	if field, msg := spellModifierShape(e); field != "" {
		return site.fail("."+field, ReasonValue, "%s", msg)
	}
	if field, msg := b.base.spellModifierKeys(e); field != "" {
		return site.fail("."+field, ReasonReference, "%s", msg)
	}
	if e.Resource != "" && !b.resourceDefined(site.owner, e.Resource) {
		return site.fail(".resource", ReasonReference, "the option spends %q, which no resource effect of this entry or its class defines", e.Resource)
	}
	return nil
}

// spellModifiers fills Derived.SpellModifiers from the spell_modifier effects the character has. A resource
// the character does not have at this level leaves the effect out.
func (x *deriver) spellModifiers() {
	for _, a := range x.active {
		e := a.effect
		if e.Type != "spell_modifier" || !x.applies(a) {
			continue
		}
		if e.Resource != "" && !slices.ContainsFunc(x.d.Resources, func(r Resource) bool { return r.Key == e.Resource }) {
			continue
		}
		m := SpellModifier{
			Key: a.owner, NamePT: x.c.namePT(a.owner), AppliesTo: e.AppliesTo, Change: e.SpellChange, From: slices.Clone(e.From),
			BonusType: e.DamageType, ExtraDice: e.Count, Resource: e.Resource, TextPT: e.TextPT,
		}
		if e.value != nil {
			n, err := e.value.Int(x.env)
			if err != nil || n < 1 || n > maxSpellBonusDamage {
				x.issue(IssueFormula, "", "Um efeito de %s foi ignorado: o dano da opção de magia não é válido.", m.NamePT)
				continue
			}
			m.Bonus = n
		}
		x.d.SpellModifiers = append(x.d.SpellModifiers, m)
	}
}

var spellChangeNms = map[string][2]string{
	SpellChangeDamageTypeSwap: {"Trocar o tipo de dano", "O dano de um dos tipos da lista vira outro tipo da lista, à escolha de quem conjura."},
	SpellChangeBonusDamage:    {"Dano extra fixo", "Soma um dano fixo à magia, do tipo dela ou de um tipo que a entrada diz."},
	SpellChangeExtraDice:      {"Dados extras por nível do espaço", "Soma dados do dado de dano da magia para cada nível do espaço de magia acima do nível da própria magia."},
}

var spellAppliesNms = map[string]string{
	SpellAppliesDamageSpell: "Magia que causa dano",
	SpellAppliesCantrip:     "Truque",
}

func spellChangeValues() []MenuValue {
	out := make([]MenuValue, 0, len(spellChanges))
	for _, k := range spellChanges {
		out = append(out, MenuValue{Key: k, NamePT: spellChangeNms[k][0], HintPT: spellChangeNms[k][1]})
	}
	return out
}

// menuSpellTargets are the spells a spell_modifier may fit: the two words, each SRD school and each damage type.
func (c *content) menuSpellTargets() []MenuValue {
	out := namedMenu(spellAppliesWords, func(k string) string { return spellAppliesNms[k] })
	for _, k := range sortedKeys(c.named) {
		if strings.HasPrefix(k, "school:") {
			out = append(out, MenuValue{Key: k, NamePT: "Escola: " + c.namePT(k)})
		}
	}
	for _, t := range c.menuDamageTypes() {
		out = append(out, MenuValue{Key: t.Key, NamePT: "Dano de " + t.NamePT})
	}
	return out
}
