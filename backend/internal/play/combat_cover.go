package play

import (
	"slices"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules/grid"
)

// Cover (MR-034, RN-20, Etapa 9 D4, Q70). For a weapon attack, a spell attack
// and a spell that asks for a Dexterity save, the target's cover against the
// attacker is the larger of two things: what the map gives along the straight
// line between the two squares (a wall is total, a painted cover square its
// degree, another creature in between half: grid.Terrain.CoverBetween) and the
// master's manual mark on the target. Degrees never add. Half cover is +2 to AC
// and to Dexterity saves, three-quarters +5; a target with total cover cannot be
// targeted by an attack or a single-target spell.
//
// The cover depends on who attacks, so it is worked out for each attacker and
// never stored; only the master's mark is a column. The armor class it changes
// stays on the server: a player gets the degree and its source ("Meia
// cobertura, do mapa"), never a number of armor class (RN-20).

// coverView is the cover one target has against one attacker, and where it
// comes from.
type coverView struct {
	degree grid.Cover
	source playv1.CoverSource
}

// coverAgainst works out the cover target has against an attack from attacker's
// square. cs are the combat's combatants: the ones standing between them give
// half cover. A combatant with no square, or an attacker that is its own target
// (a spell on itself), has none from the map.
func coverAgainst(t grid.Terrain, attacker, target playdb.Combatant, cs []playdb.Combatant) coverView {
	if attacker.ID == target.ID {
		return coverView{}
	}
	mark := coverToGrid[target.CoverMark]
	var mapCover grid.Cover
	if placed(attacker) && placed(target) {
		var between grid.Squares
		for _, o := range cs {
			// A player's attack counts only the creatures the players see (RN-10): a
			// hidden one on the line would otherwise leak as "cover do mapa". When the
			// master attacks, every creature counts (the players only see hit or miss).
			if o.ID != attacker.ID && o.ID != target.ID && !o.Defeated && placed(o) && (!o.Hidden || attacker.Kind != kindPlayer) {
				between = append(between, squareOfCombatant(o))
			}
		}
		mapCover = t.CoverBetween(squareOfCombatant(attacker), squareOfCombatant(target), between)
	}
	switch {
	case mark > mapCover:
		return coverView{degree: mark, source: playv1.CoverSource_COVER_SOURCE_MARK}
	case mapCover > grid.CoverNone:
		return coverView{degree: mapCover, source: playv1.CoverSource_COVER_SOURCE_MAP}
	}
	return coverView{}
}

// coverPool is the combatants whose bodies count as cover for what the viewer's
// player does: all of them for the master, and for a player only the ones they see
// (RN-10, MR-036): a creature in the dark, or hidden, between the two would
// otherwise leak as "cover do mapa".
func coverPool(cs []playdb.Combatant, v combatViewer) []playdb.Combatant {
	if v.master {
		return cs
	}
	return slices.DeleteFunc(slices.Clone(cs), func(o playdb.Combatant) bool { return !v.sees(o) })
}

// bonus is what the cover adds to the target's armor class and Dexterity saves:
// 2 for half, 5 for three-quarters. Total cover adds nothing: it makes the
// target untargetable, and where the master lets an attack through anyway the
// armor class is the sheet's.
func (c coverView) bonus() int {
	switch c.degree {
	case grid.CoverHalf:
		return 2
	case grid.CoverThreeQuarters:
		return 5
	}
	return 0
}

// total says the target cannot be targeted.
func (c coverView) total() bool { return c.degree == grid.CoverTotal }

// The cover and its source as an event keeps them (empty for none).
var (
	coverKeys  = map[grid.Cover]string{grid.CoverHalf: "half", grid.CoverThreeQuarters: "three_quarters", grid.CoverTotal: "total"}
	sourceKeys = map[playv1.CoverSource]string{playv1.CoverSource_COVER_SOURCE_MAP: "map", playv1.CoverSource_COVER_SOURCE_MARK: "mark"}
)

func (c coverView) key() string       { return coverKeys[c.degree] }
func (c coverView) sourceKey() string { return sourceKeys[c.source] }

// coverDegreeProto is a cover as the API says it: NONE when there is none.
func coverDegreeProto(key string) playv1.CoverDegree {
	switch key {
	case "half":
		return playv1.CoverDegree_COVER_DEGREE_HALF
	case "three_quarters":
		return playv1.CoverDegree_COVER_DEGREE_THREE_QUARTERS
	case "total":
		return playv1.CoverDegree_COVER_DEGREE_TOTAL
	}
	return playv1.CoverDegree_COVER_DEGREE_NONE
}

// coverSourceProto is a cover's source as the API says it: UNSPECIFIED for none.
func coverSourceProto(key string) playv1.CoverSource {
	switch key {
	case "map":
		return playv1.CoverSource_COVER_SOURCE_MAP
	case "mark":
		return playv1.CoverSource_COVER_SOURCE_MARK
	}
	return playv1.CoverSource_COVER_SOURCE_UNSPECIFIED
}

// errCoverTotal is the refusal of a target with total cover.
func errCoverTotal() error {
	return errEncounter(playv1.EncounterBlockedReason_ENCOUNTER_BLOCKED_REASON_TARGET_COVER_TOTAL, "the target has total cover")
}
