package play

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
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

// countedFace is the d20 a check counts: the face, or the floor of the check for a face below it.
func countedFace(face int, f rollFloor) int {
	return max(face, f.Value)
}

// countedAs is the number to tell when Reliable Talent changed the d20 that counts, 0 when it counted as it came up.
func countedAs(face, counted int) int32 {
	if counted == face {
		return 0
	}
	return clamp32(counted, 0, 20)
}

// checkD20 rolls (or takes, from a physical die) the d20 of an ability check, as d20 does, and counts it by the floor.
// The returned roll's total uses the counted number, the face is what came up.
func (s *Service) checkD20(in rollInput, modifier int, f rollFloor) (face int, roll dice.Result, err error) {
	face, roll, err = s.d20(in, modifier)
	if err != nil {
		return 0, dice.Result{}, err
	}
	if counted := countedFace(face, f); counted != face {
		roll.Total = counted + modifier
	}
	return face, roll, nil
}

// checkRoll is a d20 check's roll as the API tells it: the face that came up, the modifier and the total.
// It says the number the d20 counted as and the floor's feature only when the floor changed it: a 14 is never marked.
func checkRoll(face, modifier int32, physical bool, f rollFloor) *playv1.DiceRoll {
	counted := countedFace(int(face), f)
	out := diceRoll(1, 20, []int32{face}, modifier, clamp32(counted+int(modifier), -1<<31, 1<<31-1), physical)
	if counted != int(face) {
		out.TreatedAs, out.TreatedAsSource = new(clamp32(counted, 0, 20)), f.Source
	}
	return out
}

// treatedRoll is checkRoll for a roll stored with its total only (a scene's roll, a puzzle's hint try).
// The floor changed the d20 exactly when the total differs from face plus modifier, and source is the feature that did it.
func treatedRoll(face, modifier, total int32, physical bool, source string) *playv1.DiceRoll {
	out := diceRoll(1, 20, []int32{face}, modifier, total, physical)
	if counted := total - modifier; counted != face {
		if src := legacySource(source); src != "" {
			out.TreatedAs, out.TreatedAsSource = new(clamp32(int(counted), 0, 20)), src
		}
	}
	return out
}

// legacySource is the feature a stored roll names as the one that changed its d20. A roll stored before the table
// minimum (counted_by empty) whose total differs from face plus modifier could only be Reliable Talent's.
func legacySource(source string) string {
	if source == "" {
		return reliableTalentSource
	}
	return source
}

// rollFloor is the lowest number the d20 of a roll counts as, and the content key of the feature that gives it.
// The zero value is no floor.
type rollFloor struct {
	Value  int
	Source string
}

// floorFor is the floor of a d20 of the kind (rules.RollApplies*) for a character with the traits.
// It is the highest of the table's minimums of that kind that are not limited to a damage type.
// Reliable Talent's floor competes when reliable says the check gets it, and a tie keeps Reliable Talent.
// Only a skill check with proficiency passes reliable true.
func floorFor(traits link.Traits, kind string, reliable bool) rollFloor {
	var best rollFloor
	if reliable {
		best = rollFloor{Value: rules.ReliableTalentFloor, Source: reliableTalentSource}
	}
	for _, m := range traits.RollMinimums {
		if m.DamageType == "" && m.Value > best.Value && slices.Contains(m.Applies, kind) {
			best = rollFloor{Value: m.Value, Source: m.Source}
		}
	}
	return best
}

// floorOfCharacter is the floor of a d20 of the kind for a character, from the traits of its sheet.
func (s *Service) floorOfCharacter(ctx context.Context, tx pgx.Tx, campaignID, characterID, kind string, reliable bool) (rollFloor, error) {
	sheet, err := s.roster.CombatSheet(ctx, tx, campaignID, characterID)
	if err != nil {
		return rollFloor{}, fmt.Errorf("read the sheet of the roll: %w", err)
	}
	return floorFor(sheet.Traits, kind, reliable), nil
}

// floorOfCombatant is floorOfCharacter for a combatant: a character's sheet, or a creature's stat block.
func (s *Service) floorOfCombatant(ctx context.Context, tx pgx.Tx, campaignID string, c playdb.Combatant, kind string, reliable bool) (rollFloor, error) {
	sheet, err := s.sheetOf(ctx, tx, campaignID, c)
	if err != nil {
		return rollFloor{}, err
	}
	return floorFor(sheet.Traits, kind, reliable), nil
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

// countedAsPtr is the number the d20 counted as, nil when it counted as it came up.
func (c countedRoll) countedAsPtr() *int32 {
	if c.CountedAs == 0 {
		return nil
	}
	return &c.CountedAs
}

// value is the number the d20 that showed face counted as.
func (c countedRoll) value(face int32) int32 {
	if c.CountedAs != 0 {
		return c.CountedAs
	}
	return face
}

// countedNote is what a line adds after a total when a minimum raised the d20 that showed face.
// It is empty when the d20 counted as it came up.
func (c countedRoll) countedNote(face int32) string {
	if c.CountedAs == 0 {
		return ""
	}
	return fmt.Sprintf(" (d20 %d, conta como %d)", face, c.CountedAs)
}

// faceText is the d20 a line shows inside its parentheses: the face, and the number it counted as when a minimum raised it.
func (c countedRoll) faceText(face int32) string {
	if c.CountedAs == 0 {
		return fmt.Sprintf("%d", face)
	}
	return fmt.Sprintf("%d, conta como %d", face, c.CountedAs)
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
		out.TreatedAs, out.TreatedAsSource = new(clamp32(int(counted), 0, 20)), legacySource(source)
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
