package characters

import (
	"fmt"
	"slices"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The multiclass prerequisites of a sheet that is saved (SRD 5.1, "Multiclassing",
// "Prerequisites"): a character with several classes meets the main ability of
// every one of them. The guided level-up refuses a class the character does not
// qualify for (package rules); a player's save refuses the same, and the master's
// editor goes past it, with the sheet's multiclass_prerequisite issue.

// unmetMulticlass lists the classes of a sheet whose multiclass prerequisite the
// final scores do not meet, with the prerequisite, in the sheet's order. A sheet
// with one class has no multiclass prerequisite to meet.
func unmetMulticlass(content *rules.Content, f *charactersv1.FullSheet) []rules.MulticlassPrerequisite {
	if len(f.GetClasses()) < 2 {
		return nil
	}
	b := buildOf(f)
	keys := make([]string, 0, len(b.Classes))
	for _, cl := range b.Classes {
		keys = append(keys, cl.Class)
	}
	var out []rules.MulticlassPrerequisite
	for _, p := range rules.MulticlassPrerequisites(b, keys, content) {
		if !p.Met {
			out = append(out, p)
		}
	}
	return out
}

// refuseMulticlassGap refuses a player's sheet that has a class whose multiclass
// prerequisite the stored sheet did not already miss (nil when creating): a
// character made before the rule, or by the master, keeps working, and nothing
// stops the master. It answers `failed_precondition` with the LevelUpRefusal
// detail (MULTICLASS_PREREQUISITE, the class and the missing score) at the
// class's field in the sheet.
func refuseMulticlassGap(content *rules.Content, m authz.Membership, stored, sheet *charactersv1.FullSheet) error {
	if isMaster(m) {
		return nil
	}
	already := map[string]bool{}
	for _, p := range unmetMulticlass(content, stored) {
		already[p.Class] = true
	}
	for _, p := range unmetMulticlass(content, sheet) {
		if already[p.Class] {
			continue
		}
		miss := rules.MulticlassRequirement{}
		for _, r := range p.Requirements {
			switch {
			case r.Met:
			case miss.Minimum == 0, p.AnyOf && r.Have > miss.Have:
				miss = r
			}
		}
		idx := slices.IndexFunc(sheet.GetClasses(), func(c *charactersv1.ClassLevel) bool { return c.GetClassKey() == p.Class })
		return errRefused(&charactersv1.LevelUpRefusal{
			Field:    fmt.Sprintf("sheet.full.classes[%d].class_key", idx),
			Reason:   charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_MULTICLASS_PREREQUISITE,
			ClassKey: p.Class, Ability: abilityToProto[miss.Ability], Minimum: i32(miss.Minimum), Have: i32(miss.Have),
		})
	}
	return nil
}
