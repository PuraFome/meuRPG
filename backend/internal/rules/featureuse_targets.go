package rules

// What a use of a feature may be aimed at (UseFeature, GetTurnOptions): the side, the creature's state and the range. The
// play module reads the creatures from the combat and asks this file whether each one can be picked, so that the list the
// player reads and the check the server makes are one rule.

// The reasons a creature cannot be picked, as the list writes them next to it (RN-10: none names a creature's type).
const (
	FeatureTargetWrongSide  = "Lado errado"
	FeatureTargetYou        = "É você"
	FeatureTargetOnlyYou    = "Só você"
	FeatureTargetDown       = "Caído"
	FeatureTargetNotOnMap   = "Fora do mapa"
	FeatureTargetOutOfReach = "Fora de alcance"
)

// FeatureCandidate is a creature of the combat as a use of a feature sees it.
type FeatureCandidate struct {
	// Self says it is the creature that uses the feature.
	Self bool
	// SameSide says it is on the user's side (the user is on its own side).
	SameSide bool
	// OnlyAlly says the user has no other ally in the combat: an "ally" use may then be aimed at the user.
	OnlyAlly bool
	// Down says it is a player's character at 0 hit points: only a use that heals reaches it.
	Down bool
	// OnMap says both the user and the creature stand on the map; Distance is the feet between them. Both only matter
	// when the feature has a range and the combat has a map (IgnoreRange false).
	OnMap    bool
	Distance int
	// IgnoreRange says the range is not checked: the master acts, or the combat has no grid (the master judges the reach).
	IgnoreRange bool
}

// Refusal is why the creature cannot be picked, "" when it can. The first reason that applies wins: the side, the state,
// then the range.
func (t FeatureTargets) Refusal(c FeatureCandidate, heals bool) string {
	switch t.Who {
	case FeatureWhoSelf:
		if !c.Self {
			return FeatureTargetOnlyYou
		}
	case FeatureWhoAlly:
		switch {
		case !c.SameSide:
			return FeatureTargetWrongSide
		case c.Self && !c.OnlyAlly:
			return FeatureTargetYou
		}
	case FeatureWhoEnemy:
		if c.SameSide {
			return FeatureTargetWrongSide
		}
	}
	if c.Down && !heals {
		return FeatureTargetDown
	}
	if t.RangeFt > 0 && !c.Self && !c.IgnoreRange {
		switch {
		case !c.OnMap:
			return FeatureTargetNotOnMap
		case c.Distance > t.RangeFt:
			return FeatureTargetOutOfReach
		}
	}
	return ""
}

// Aimed says the use picks creatures: it takes at least one target. A use with no targets declared is used by its
// user alone.
func (t FeatureTargets) Aimed() bool { return t.Max > 0 }

// Allows says how many targets a use may be given: from the least (a use that picks creatures takes at least one, whatever
// its min says; a use aimed at self takes none or the user) to Max.
func (t FeatureTargets) Allows(n int) bool {
	if !t.Aimed() {
		return n == 0
	}
	if t.Who == FeatureWhoSelf {
		return n <= 1
	}
	return n >= max(t.Min, 1) && n <= t.Max
}
