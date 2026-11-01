package play

import (
	"fmt"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/rules"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
)

// Reliable Talent (SRD 5.1, Rogue, level 11): whenever the character makes an
// ability check that lets it add the proficiency bonus, a d20 of 9 or lower counts
// as 10. The rule is the server's: it is applied wherever the server rolls or takes
// the d20 of a skill check (a scene's action, a puzzle's hint, the search for traps),
// never to an attack, a saving throw, the initiative or a check with no proficiency
// in it (the roster says so, per check: link.SceneOption.ReliableTalent).

// reliableTalentSource is the content key of the feature, as DiceRoll.treated_as_source
// names it.
const reliableTalentSource = "feature:reliable-talent"

// countedFace is the d20 a check counts: the face, or 10 for a low face when
// the character has Reliable Talent for the check.
func countedFace(face int, reliable bool) int {
	if !reliable {
		return face
	}
	counted, _ := rules.ReliableTalentFace(face)
	return counted
}

// countedAs is the number to tell when Reliable Talent changed the d20 that counts, 0 when it counted as it came up.
func countedAs(face, counted int) int32 {
	if counted == face {
		return 0
	}
	return clamp32(counted, 0, 20)
}

// checkD20 rolls (or takes, from a physical die) the d20 of an ability check, as d20
// does, and counts it by Reliable Talent when reliable says the check gets it: the
// returned roll's total uses the counted number, the face is what came up.
func (s *Service) checkD20(in rollInput, modifier int, reliable bool) (face int, roll dice.Result, err error) {
	face, roll, err = s.d20(in, modifier)
	if err != nil {
		return 0, dice.Result{}, err
	}
	if counted := countedFace(face, reliable); counted != face {
		roll.Total = counted + modifier
	}
	return face, roll, nil
}

// checkRoll is a d20 check's roll as the API tells it: the face that came up, the
// modifier, and the total, with the number the d20 counted as when Reliable Talent
// changed it (and only then: a 14 is never marked). reliable is whether the check
// had the feature.
func checkRoll(face, modifier int32, physical, reliable bool) *playv1.DiceRoll {
	counted := countedFace(int(face), reliable)
	out := diceRoll(1, 20, []int32{face}, modifier, clamp32(counted+int(modifier), -1<<31, 1<<31-1), physical)
	if counted != int(face) {
		out.TreatedAs, out.TreatedAsSource = new(clamp32(counted, 0, 20)), reliableTalentSource
	}
	return out
}

// treatedRoll is checkRoll for a roll stored with its total only (a scene's roll, a
// puzzle's hint try): the feature changed the d20 exactly when the total differs from
// face plus modifier.
func treatedRoll(face, modifier, total int32, physical bool) *playv1.DiceRoll {
	out := diceRoll(1, 20, []int32{face}, modifier, total, physical)
	if counted := total - modifier; counted != face {
		out.TreatedAs, out.TreatedAsSource = new(clamp32(int(counted), 0, 20)), reliableTalentSource
	}
	return out
}

// rollFloor is the lowest number the d20 of a roll counts as, and the content key of the feature that gives it.
// The zero value is no floor.
type rollFloor struct {
	Value  int
	Source string
}

// reliableFloor is Reliable Talent's floor for a check: 10 when the check gets it.
func reliableFloor(reliable bool) rollFloor {
	if !reliable {
		return rollFloor{}
	}
	return rollFloor{Value: rules.ReliableTalentFloor, Source: reliableTalentSource}
}

// floorFor is the floor of a d20 of the kind (rules.RollApplies*) for a character with the traits.
// It is the highest of the table's minimums of that kind that are not limited to a damage type.
// Reliable Talent's floor competes when reliable says the check gets it, and a tie keeps Reliable Talent.
// Only the check paths (T3) pass reliable true: the others pass false.
func floorFor(traits link.Traits, kind string, reliable bool) rollFloor {
	best := reliableFloor(reliable)
	for _, m := range traits.RollMinimums {
		if m.DamageType == "" && m.Value > best.Value && slices.Contains(m.Applies, kind) {
			best = rollFloor{Value: m.Value, Source: m.Source}
		}
	}
	return best
}

// floorD20 counts the die that counts by the floor: it counts as the floor when it came up lower.
// The total uses that number and the faces stay as they came up.
// Treating each die before the pick and treating the one picked give the same number, so the pick stays the mode's.
func floorD20(r d20Roll, modifier int, f rollFloor) d20Roll {
	r.CountedValue, r.CountedBy = 0, ""
	r.Total = r.Face() + modifier
	if f.Value > r.Face() {
		r.CountedValue, r.CountedBy = f.Value, f.Source
		r.Total = f.Value + modifier
	}
	return r
}

// countedRoll is what a stored roll keeps of a floor: the number the die counted as and the feature that did it.
// Both are zero when the die counted as it came up.
type countedRoll struct {
	CountedAs int32  `json:"counted_as,omitempty"`
	CountedBy string `json:"counted_by,omitempty"`
}

// countedOf is what a roll keeps of its floor.
func countedOf(r d20Roll) countedRoll {
	return countedRoll{CountedAs: clamp32(r.CountedValue, 0, 20), CountedBy: r.CountedBy}
}

// value is the number the d20 that showed face counted as.
func (c countedRoll) value(face int32) int32 {
	if c.CountedAs != 0 {
		return c.CountedAs
	}
	return face
}

// countedNote is what a line adds after a total when the d20 that showed face counted as another number, "" otherwise.
func (c countedRoll) countedNote(face int32) string {
	if c.CountedAs == 0 {
		return ""
	}
	return fmt.Sprintf(" (d20 %d, conta como %d)", face, c.CountedAs)
}

// mark tells the API that the die counted as another number than it came up; the faces stay as they came up.
func (c countedRoll) mark(out *playv1.DiceRoll) *playv1.DiceRoll {
	if c.CountedAs != 0 {
		out.TreatedAs, out.TreatedAsSource = new(c.CountedAs), c.CountedBy
	}
	return out
}

// markTreated marks the die that counted of a roll when its total uses another number than the face that came up.
// The floor named by source changed it.
func markTreated(out *playv1.DiceRoll, modifier, total int32, source string) *playv1.DiceRoll {
	faces := out.GetFaces()
	i := int(out.GetCountedIndex())
	if i < 0 || i >= len(faces) {
		return out
	}
	if counted := total - modifier; counted != faces[i] {
		out.TreatedAs, out.TreatedAsSource = new(clamp32(int(counted), 0, 20)), source
	}
	return out
}

// checkKeyOf is the scene check key of a trap search's skill: Perception and
// Investigation have theirs; another skill is its own key.
func checkKeyOf(skill string) string {
	if strings.HasPrefix(skill, "skill:") {
		return skill
	}
	return searchCheckKey[skill]
}
