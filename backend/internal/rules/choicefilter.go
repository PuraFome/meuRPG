package rules

import "slices"

// What a `choice` effect may offer besides a list of keys: every proficiency of some kinds (a choice of tools with a
// filter), or the options of another feature (a choice of features with from_feature).

// The proficiency kinds a filter names, and the order the menu lists them in.
const (
	filterKindWeapon = "weapon"
	filterKindTool   = "tool"
	filterKindOther  = "other"
)

// filterKinds are the proficiency kinds a filter may name.
var filterKinds = []string{filterKindWeapon, filterKindTool, filterKindOther}

// ChoiceFilter says which entries a choice offers by what they are. It holds the kinds of proficiency of a choice of
// tools today; the fields for the spells of a choice of spells join it later.
type ChoiceFilter struct {
	ProficiencyKinds []string `json:"proficiency_kinds,omitempty"`
}

// proficiencyKindNames are the Portuguese names of the kinds a filter names.
var proficiencyKindNames = map[string]string{
	filterKindWeapon: "Armas",
	filterKindTool:   "Ferramentas",
	filterKindOther:  "Veículos e kits",
}

// menuProficienciesOfKind are the proficiencies of one kind, by Portuguese name: what a filter of that kind offers.
func (c *content) menuProficienciesOfKind(kind string) []MenuValue {
	var out []MenuValue
	for _, k := range c.proficiencyKeysOfKinds(kind) {
		out = append(out, MenuValue{Key: k, NamePT: c.proficiencyNamePT(k)})
	}
	sortMenuByName(out)
	return out
}

// proficiencyKeysOfKinds are the keys of the content's proficiencies of the kinds, sorted.
func (c *content) proficiencyKeysOfKinds(kinds ...string) []string {
	var out []string
	for k, p := range c.proficiencies {
		if slices.Contains(kinds, p.Kind) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}
