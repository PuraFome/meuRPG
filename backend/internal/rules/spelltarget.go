package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// Whom a spell reaches (MR-025, MR-045, RN-23). A table spell says it in its
// own fields (SpellTarget, written by the master). An SRD spell says it in the
// 5e-database's structured area_of_effect (a shape and a size in feet) when it has
// one, and otherwise in prose, which this file reads with two patterns so a wrong
// guess has one place to fix (the master is never held to the number of targets,
// so a wrong guess never blocks a table).

var (
	// An area is a shape ("20-foot radius", "15-foot cone", "100-foot-long
	// line"), a point ("within 20 feet of a point") or "up to three creatures".
	areaRE = regexp.MustCompile(`(?i)\b\d+-foot[- ](radius|cone|cube|line|square|sphere|cylinder|long|wide)|within \d+ feet of a point|\bup to (two|three|four|five|six|seven|eight|nine|ten|twelve) (other )?(creatures|humanoids|willing creatures)|\bcreatures of your choice`)
	// A spell that gets "one additional creature" at a higher level takes one more
	// target for each level. Magic Missile's darts and a spell attack's targets are
	// not areas: the play module counts them.
	extraTargetRE = regexp.MustCompile(`(?i)additional (creature|target|humanoid)|one additional`)
)

// textArea says whether the SRD's prose makes a spell hit any number of targets:
// a spell attack takes one, a healing spell that is not mass or a prayer takes one,
// and a spell that comes out of the caster and damages (Mãos Flamejantes, Onda
// Trovejante) is an area; the rest is read from the description. It is the
// fallback for a spell the database gives no area_of_effect.
func textArea(s *srd51.Spell) bool {
	switch {
	case s.AttackType != "":
		return false
	case s.Key == "spell:magic-missile":
		return false
	case len(s.HealAtSlotLevel) > 0:
		return strings.HasPrefix(s.Key, "spell:mass-") || s.Key == "spell:prayer-of-healing"
	case s.Range == "Self" && (s.SaveAbility != "" || len(s.Damage) > 0):
		return true
	}
	return areaRE.MatchString(strings.Join(s.Desc, " "))
}

// textExtraTarget says the prose gives a spell one more target for each slot
// level above its own.
func textExtraTarget(s *srd51.Spell) bool {
	return extraTargetRE.MatchString(strings.Join(s.HigherLevel, " "))
}

// srdTarget says whom an SRD spell reaches: the structured area when the
// database has one, then the prose. The kinds are the table's (SpellTarget), with
// two differences: "creatures" with a Count of 0 is "as many as the caster picks"
// (the text says how many; the master is never held to it), and "creature" may
// take PerSlotLevel more for each circle above (Hold Person).
func srdTarget(s *srd51.Spell) SpellTarget {
	extra := 0
	if textExtraTarget(s) {
		extra = 1
	}
	switch {
	case s.AreaType != "":
		return SpellTarget{Kind: TargetArea, Shape: s.AreaType, SizeFt: s.AreaSizeFt}
	case s.Key == "spell:magic-missile", s.Key == "spell:scorching-ray":
		// A dart or a ray each: three at the spell's level and one more for each
		// circle above, as many targets as there are darts or rays.
		return SpellTarget{Kind: TargetCreatures, Count: 3, PerSlotLevel: 1}
	case s.AttackType != "":
		return SpellTarget{Kind: TargetCreature}
	case textArea(s):
		return SpellTarget{Kind: TargetCreatures, PerSlotLevel: extra}
	case s.Range == "Self":
		return SpellTarget{Kind: TargetSelf}
	}
	return SpellTarget{Kind: TargetCreature, PerSlotLevel: extra}
}

// AnyNumber says the spell takes any number of targets: an area, or "creatures"
// without a count (the text says how many and the caster picks).
func (t SpellTarget) AnyNumber() bool {
	return t.Kind == TargetArea || (t.Kind == TargetCreatures && t.Count == 0)
}

// LabelPT is the target as the "Magias" page and the spell editor's preview read
// it: "Uma criatura", "Várias criaturas", "Só quem conjura" or an area, in meters
// ("Cone de 4,5 m"). Empty for a spell without a target (none today: every spell
// of the content has one).
func (t SpellTarget) LabelPT() string {
	switch t.Kind {
	case TargetSelf:
		return "Só quem conjura"
	case TargetCreature:
		return "Uma criatura"
	case TargetCreatures:
		return "Várias criaturas"
	case TargetArea:
		shape := map[string]string{ShapeCone: "Cone", ShapeCube: "Cubo", ShapeCylinder: "Cilindro", ShapeLine: "Linha", ShapeSphere: "Esfera"}[t.Shape]
		if shape == "" {
			return "Área"
		}
		return shape + " de " + MetersPT(t.SizeFt)
	}
	return ""
}

// MetersPT writes a distance in feet in the table's units (5 ft = 1,5 m, so the
// meters are the feet times 0,3): "4,5 m", "18 m", and kilometers from 1 000 m
// ("1,6 km"). It is the text of the app's formatMeters and formatRangeFt.
func MetersPT(feet int) string {
	tenths := feet * 3 // tenths of a meter
	if tenths >= 10000 {
		km := (tenths + 500) / 1000 // tenths of a kilometer, rounded
		return decimalPT(km) + " km"
	}
	return decimalPT(tenths) + " m"
}

// decimalPT writes tenths as a number with a comma: 45 is "4,5", 180 is "18".
func decimalPT(tenths int) string {
	if tenths%10 == 0 {
		return strconv.Itoa(tenths / 10)
	}
	return fmt.Sprintf("%d,%d", tenths/10, tenths%10)
}
