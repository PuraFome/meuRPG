package characters

import (
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// maxContentKeyLength bounds a content key sent by a client (a table key is at most 80
// characters long; the SRD's are shorter).
const maxContentKeyLength = 128

var featUnmetKinds = map[string]rulesv1.FeatUnmetKind{
	rules.FeatUnmetAbilityMinimum: rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_ABILITY_MINIMUM,
	rules.FeatUnmetAbilityAnyOf:   rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_ABILITY_ANY_OF,
	rules.FeatUnmetProficiency:    rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_PROFICIENCY,
	rules.FeatUnmetSpellcasting:   rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_SPELLCASTING,
	rules.FeatUnmetRace:           rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_RACE,
	rules.FeatUnmetLevel:          rulesv1.FeatUnmetKind_FEAT_UNMET_KIND_LEVEL,
}

// featOptionsToProto is the feats a character may take, as the app's feat picker reads
// them. Where the table does not use feats there are none. A feat the master retired is
// never offered, and one the master switched off is offered to the master only (RN-23).
func featOptionsToProto(in []rules.FeatOption, allowed, master bool) []*rulesv1.FeatOption {
	if !allowed {
		return nil
	}
	var out []*rulesv1.FeatOption
	for _, o := range in {
		if o.Archived || (o.Off && !master) {
			continue
		}
		out = append(out, featOptionOf(o))
	}
	return out
}

func featOptionOf(o rules.FeatOption) *rulesv1.FeatOption {
	p := o.Prerequisite
	out := &rulesv1.FeatOption{
		Key: o.Key, NamePt: o.NamePT, Name: o.Name, Desc: o.Desc, DescPt: o.DescPT, DescPtMissing: o.DescPTMissing, DescPtOnly: o.DescPTOnly, Table: o.Table, Qualifies: o.Qualifies,
		Prerequisite: &rulesv1.FeatPrerequisite{
			ProficiencyKey: p.Proficiency, Spellcasting: p.Spellcasting, RaceKey: p.Race, Level: i32(p.Level),
		},
	}
	if len(p.Minimums) > 0 {
		out.Prerequisite.Minimums = abilityScores(p.Minimums)
	}
	if len(p.AnyOf) > 0 {
		out.Prerequisite.AnyOf = abilityScores(p.AnyOf)
	}
	if inc := o.Increase; inc != nil {
		out.Increase = &rulesv1.FeatIncrease{Count: i32(inc.Count), Value: i32(inc.Value)}
		for _, a := range inc.From {
			out.Increase.From = append(out.Increase.From, abilityToProto[a])
		}
	}
	for _, a := range o.Capped {
		out.CappedAbilities = append(out.CappedAbilities, abilityToProto[a])
	}
	for _, u := range o.Unmet {
		pu := &rulesv1.FeatUnmet{Kind: featUnmetKinds[u.Kind], Key: u.Key, Value: i32(u.Value)}
		for _, a := range u.Abilities {
			pu.Abilities = append(pu.Abilities, &rulesv1.AbilityMinimum{Ability: abilityToProto[a.Ability], Minimum: i32(a.Minimum)})
		}
		out.Unmet = append(out.Unmet, pu)
	}
	return out
}
