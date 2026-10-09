package play

import (
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/platform/dice"
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

// checkKeyOf is the scene check key of a trap search's skill: Perception and
// Investigation have theirs; another skill is its own key.
func checkKeyOf(skill string) string {
	if strings.HasPrefix(skill, "skill:") {
		return skill
	}
	return searchCheckKey[skill]
}
