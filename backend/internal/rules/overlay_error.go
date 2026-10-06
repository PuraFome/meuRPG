package rules

import (
	"errors"
	"strings"
)

// locate gives an OverlayError that has no Field yet the path of the entry it
// came from (the caller's own index, "classes[2]"), plus the attribute the
// message is about when it is known, and a Reason. Errors raised with `at`
// (the features and effects, which know their own path) are left as they are.
func locate(err error, path string) error {
	var oe *OverlayError
	if !errors.As(err, &oe) {
		return err
	}
	for _, o := range oe.Violations() {
		if o.Field == "" {
			o.Field = path
			if attr := attrOf(o.Message); attr != "" {
				o.Field += "." + attr
			}
		}
		if o.Reason == "" {
			o.Reason = reasonOf(o.Message)
		}
	}
	return oe
}

var reasonWords = []struct{ words, reason string }{
	{"casting time", ReasonValue}, // a spell's, not a class's casting
	{"reserved", ReasonReservedKey},
	{"already exists", ReasonDuplicateKey},
	{"the key must|slug", ReasonKey},
	{"limit|more than|at most", ReasonLimit},
	{"the name", ReasonName},
	{"paragraph|the text has", ReasonText},
	{"handler|menu|cannot grant", ReasonEffect},
	{"formula", ReasonFormula},
	{"does not exist|not in the SRD|no spell list|is not an option|reuses another list|not a tool|not an armor", ReasonReference},
	{"rows|row for|level %d|level 1|casting columns|pact magic|needs spell slots|ascending|subclass levels|before casting", ReasonTable},
	{"casting|third caster|spell list|prepared_max|spells are|spellcasting ability|own list", ReasonCasting},
}

func reasonOf(msg string) string {
	for _, r := range reasonWords {
		for _, w := range strings.Split(r.words, "|") {
			if strings.Contains(msg, w) {
				return r.reason
			}
		}
	}
	return ReasonValue
}

// attrWords maps what a message is about to the Overlay field's name.
var attrWords = []struct{ word, attr string }{
	{"the key", "key"},
	{"slug", "key"},
	{"the name", "name_pt"},
	{"hit die", "hit_die"},
	{"saving throw", "saving_throws"},
	{"skill", "skill_from"},
	{"proficienc", "proficiencies"},
	{"subclass is chosen", "subclass_level"},
	{"class table needs", "levels"},
	{"Ability Score Improvement", "asi_levels"},
	{"casting time", "casting_time"},
	{"casting", "casting"},
	{"spell list", "casting.list_from"},
	{"prepared_max", "casting.prepared_max"},
	{"multiclass", "minimums"},
	{"size is", "size"},
	{"speed", "speed_ft"},
	{"darkvision", "darkvision_ft"},
	{"language", "languages"},
	{"bonus", "ability_bonuses"},
	{"tool", "tools"},
	{"spell level", "level"},
	{"school", "school_key"},
	{"target", "target"},
	{"area", "target"},
	{"saving throw is", "save"},
	{"damage", "damage"},
	{"healing", "heal"},
	{"cantrip does not heal", "heal"},
	{"range", "range"},
	{"duration", "duration"},
	{"concentration", "duration"},
	{"material", "components"},
	{"ritual", "ritual"},
	{"attack", "attack"},
	{"always-prepared", "always_prepared"},
	{"the race", "race_key"},
	{"the class", "class_key"},
	{"subclass levels", "levels"},
	{"table row", "levels"},
}

func attrOf(msg string) string {
	for _, a := range attrWords {
		if strings.Contains(msg, a.word) {
			return a.attr
		}
	}
	return ""
}
