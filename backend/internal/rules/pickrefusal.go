package rules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The refusal of a new pick that no source takes. A pick the stored sheet already holds is never refused here, whether it
// fits or not: the fit issue of a stored sheet is the master's alone and the sheet saves and plays.

// PickRefusal is why a new pick is refused: Field is its place in the sheet and Key the pick.
type PickRefusal struct {
	Field, Key string
	// Unknown says the pick is a key the view does not have: a tool pick that is no tool, or of an owner a player may not read
	// that the sheet does not hold. Otherwise no list of the sheet takes the pick.
	Unknown bool
}

// NewPickRefusal is the first new skill, expertise or tool pick of the new sheet that the view refuses (stored is the stored
// sheet, nil for a new character): a tool pick that is an unknown key first, whatever other picks no list places, then the
// first pick no source takes (UnplacedNewPick). c is the caller's view of the content.
func (c *Content) NewPickRefusal(stored *Build, now Build) (PickRefusal, bool) {
	unknown, unplaced := c.scanNewPicks(stored, now)
	switch {
	case unknown != nil:
		return *unknown, true
	case unplaced != nil:
		return *unplaced, true
	}
	return PickRefusal{}, false
}

// UnplacedNewPick returns the sheet field and the key of the first skill, expertise or tool pick of the new sheet that
// the stored sheet (nil for a new character) does not hold and that no source of the content takes: the largest placement
// of the picks in the sources is no larger with the pick than without it.
// A held pick that fits takes part in the placement, so a new pick that fits only when a held pick moves to another source
// is not refused.
// A held pick that the stored sheet could not place leaves the sources alone.
// The sources are those of the sheet itself (deriver.skillSources and its twins): what the reader may read changes no list.
func (c *Content) UnplacedNewPick(stored *Build, now Build) (field, key string, found bool) {
	if _, unplaced := c.scanNewPicks(stored, now); unplaced != nil {
		return unplaced.Field, unplaced.Key, true
	}
	return "", "", false
}

// scanNewPicks reads the new picks once: unknown is the first new tool pick that is an unknown key, unplaced the first new
// pick no source takes (skills, then expertise, then tools).
func (c *Content) scanNewPicks(stored *Build, now Build) (unknown, unplaced *PickRefusal) {
	x := c.deriverFor(now)
	var held Build
	var xs *deriver
	if stored != nil {
		held, xs = *stored, c.deriverFor(*stored)
	}
	unknown, toolUnplaced := x.scanNewTools(held, xs)
	if len(x.classes) == 0 {
		return unknown, nil
	}
	if f, k, ok := c.c.unplacedNewSkillPick(x, xs, held, now); ok {
		return unknown, &PickRefusal{Field: f, Key: k}
	}
	return unknown, toolUnplaced
}

// unplacedNewSkillPick is UnplacedNewPick for the skills and the expertise: x is the deriver of the new sheet now, xs the
// one of the stored sheet (nil when there is none) and held the stored sheet.
func (c *content) unplacedNewSkillPick(x, xs *deriver, held, now Build) (field, key string, found bool) {
	skill := func(d *deriver) func(string) bool {
		return func(k string) bool { _, ok := c.skills[k]; return ok && !d.automaticSkills[k] }
	}
	expertise := func(k string) bool { _, ok := c.skills[k]; return ok }
	var stuckSkills, stuckExpertise []string
	heldSkills := held.SkillProficiencies
	if xs != nil {
		heldSkills = pickList(held.SkillProficiencies, skill(xs)) // a skill the stored sheet had by another means is no held pick
		stuckSkills = unplacedPicks(xs.skillSources(), pickList(held.SkillProficiencies, skill(xs)))
		stuckExpertise = unplacedPicks(xs.expertiseSources(), pickList(held.Expertise, expertise))
	}
	if f, k, ok := unplacedNewList(skillsField, x.skillSources(), now.SkillProficiencies, heldSkills, stuckSkills, skill(x)); ok {
		return f, k, true
	}
	return unplacedNewList(expertiseField, x.expertiseSources(), now.Expertise, held.Expertise, stuckExpertise, expertise)
}

// deriverFor is the deriver of the sheet b, made with the entries it loses left out.
func (c *Content) deriverFor(b Build) *deriver { return c.c.deriverFor(b) }

func (c *content) deriverFor(b Build) *deriver {
	_, lost, _ := c.settleLost(b)
	return deriverOf(b, c, lost)
}

// pickList are the entries of the list that count, each once.
func pickList(list []string, counted func(string) bool) []string {
	var picks []string
	for _, k := range list {
		if counted(k) && !slices.Contains(picks, k) {
			picks = append(picks, k)
		}
	}
	return picks
}

// unplacedNewList is UnplacedNewPick for one list of the sheet: counted says an entry is a pick of the sources, held are the
// picks the stored sheet holds and stuck are the held picks it could not place.
func unplacedNewList(field string, sources []pickSource, list, held, stuck []string, counted func(string) bool) (string, string, bool) {
	picks := pickList(list, counted)
	left := unplacedNewPicks(sources, competing(picks, held, stuck), held)
	for i, k := range list {
		if slices.Contains(left, k) {
			return fmt.Sprintf("%s[%d]", field, i), k, true
		}
	}
	return "", "", false
}

// unplacedNewPicks are the new picks that the sources leave out when the held picks are placed first and the new ones follow
// in order: each is a pick the sources have no room for, whatever its place in the sheet.
func unplacedNewPicks(sources []pickSource, picks, held []string) []string {
	isHeld := func(k string) bool { return slices.Contains(held, k) }
	ordered := slices.Concat(slices.DeleteFunc(slices.Clone(picks), func(k string) bool { return !isHeld(k) }), slices.DeleteFunc(slices.Clone(picks), isHeld))
	return slices.DeleteFunc(unplacedPicks(sources, ordered), isHeld)
}

// competing are the picks that take part in the placement of a new pick: the new ones and the held ones the stored sheet could place.
func competing(picks, held, stuck []string) []string {
	return slices.DeleteFunc(slices.Clone(picks), func(k string) bool { return slices.Contains(held, k) && slices.Contains(stuck, k) })
}

// placedCount is how many of the picks a placement in the sources takes.
func placedCount(sources []pickSource, picks []string) int {
	return len(picks) - len(unplacedPicks(sources, picks))
}

// toolPicksByOwner are the tool picks of the sheet whose owner it has, owner by owner, each value once.
func (x *deriver) toolPicksByOwner() map[string][]string {
	onSheet := ownedBy(x.b, *x.d)
	out := map[string][]string{}
	for _, stored := range x.b.FeatureChoices {
		choiceKey, value, ok := SplitScopedChoice(stored)
		if !ok {
			continue
		}
		if owner, isTool := strings.CutSuffix(choiceKey, toolPickSuffix); isTool && (x.owned[owner] || onSheet(owner)) && !slices.Contains(out[owner], value) {
			out[owner] = append(out[owner], value)
		}
	}
	return out
}

// scanNewTools reads the new tool picks of the sheet once, in the sheet's order; xs is the stored sheet's deriver or nil.
// unknown is the first that is no tool, or whose owner the view may not read and the sheet lacks (a key the content does not
// have: a pick of an owner the sheet does not have is none of these unless the owner is closed to the reader). unplaced is the first that no tool choice of its owner
// takes, when the sheet has a class.
func (x *deriver) scanNewTools(held Build, xs *deriver) (unknown, unplaced *PickRefusal) {
	byOwner := x.toolPicksByOwner()
	stuck := xs.stuckTools()
	for i, stored := range x.b.FeatureChoices {
		choiceKey, owner, value, ok := newToolPick(stored, held)
		if !ok {
			continue
		}
		known, closed := x.toolOwnerKnown(byOwner, owner)
		if !known {
			continue
		}
		if !x.c.isToolPickKey(value) || closed {
			return &PickRefusal{Field: fmt.Sprintf("%s[%d]", choiceKeysField, i), Key: value, Unknown: true}, unplaced
		}
		if unplaced != nil || len(x.classes) == 0 {
			continue
		}
		if x.toolLeftOut(held, choiceKey, owner, value, byOwner[owner], stuck[owner]) {
			unplaced = &PickRefusal{Field: fmt.Sprintf("%s[%d]", choiceKeysField, i), Key: value}
		}
	}
	return nil, unplaced
}

// newToolPick splits a feature choice of the sheet that is a tool pick the stored sheet does not hold: its choice key, its owner
// and its value.
func newToolPick(stored string, held Build) (choiceKey, owner, value string, ok bool) {
	choiceKey, value, ok = SplitScopedChoice(stored)
	if !ok || slices.Contains(held.FeatureChoices, stored) {
		return "", "", "", false
	}
	owner, ok = strings.CutSuffix(choiceKey, toolPickSuffix)
	return choiceKey, owner, value, ok
}

// toolOwnerKnown says whether the owner of a tool pick is known to the reader, and whether it is closed to them. An owner the
// sheet holds or gets is known, archived or not: its tool choice works like its skills. An owner it neither holds nor gets is
// known only while the table keeps it open to the reader (a player's: not archived or switched off; the master's: not archived);
// a closed one is known too, so its pick is answered as a key that does not exist.
func (x *deriver) toolOwnerKnown(byOwner map[string][]string, owner string) (known, closed bool) {
	if byOwner[owner] != nil {
		return true, false
	}
	closed = x.closedOwner(owner)
	return closed, closed
}

// toolLeftOut says the tool choices of the owner have no room for the new pick value beside the owner's other picks: the held
// ones are placed first, but those the stored sheet could not place (stuck) do not compete.
func (x *deriver) toolLeftOut(held Build, choiceKey, owner, value string, picks, stuck []string) bool {
	heldValues := slices.DeleteFunc(slices.Clone(picks), func(k string) bool { return !slices.Contains(held.FeatureChoices, ScopedChoice(choiceKey, k)) })
	return slices.Contains(unplacedNewPicks(x.c.toolSources(owner), competing(picks, heldValues, stuck), heldValues), value)
}

// stuckTools are the tool picks of the stored sheet its tool choices could not place, owner by owner; none for a nil deriver.
func (x *deriver) stuckTools() map[string][]string {
	stuck := map[string][]string{}
	if x == nil {
		return stuck
	}
	for owner, values := range x.toolPicksByOwner() {
		stuck[owner] = unplacedPicks(x.c.toolSources(owner), values)
	}
	return stuck
}

// closedOwner says the owner of a tool pick is an entry the reader does not have: archived, or for a player also switched off
// (hidden). It is asked of an owner the sheet neither holds nor gets.
func (x *deriver) closedOwner(owner string) bool {
	if x.c.player {
		return x.c.hidden(owner)
	}
	return x.c.archived[owner]
}

// IsToolProficiency says the key is a proficiency a tool choice may offer: a tool, a vehicle, a kit or a weapon.
func (c *Content) IsToolProficiency(key string) bool { return c.c.isToolPickKey(key) }

// PickAllowed is the keys the sources of one kind of pick allow. Any is set when a source takes any key.
type PickAllowed struct {
	Any  bool
	Keys []string
}

// PickLists are the skills the sheet's skill sources allow and, when every expertise source names a list, the skills
// expertise may go on (nil otherwise). They are the lists the save's refusal reads (unplacedNewList).
func (c *Content) PickLists(b Build) (skills PickAllowed, expertise *PickAllowed) {
	skills, e, listed := c.deriverFor(b).pickLists()
	if listed && !e.Any {
		expertise = &e
	}
	return skills, expertise
}

// LevelPickKeys are the skills and the expertise a new pick may go on in the sheet b (the sheet after a level), as keys: the
// lists of PickLists, then one narrowing step (narrowPickKeys).
func (c *Content) LevelPickKeys(b Build) (skills, expertise []string) {
	return c.c.levelPickKeys(b, nil, 0)
}

// levelPickKeys is LevelPickKeys for a level that may add a class: row is the class's multiclass skill pick (nil when the
// level adds no class or its row gives no skill) and extra the skills the level asks besides it. A skill the sheet picks
// at a class added by multiclassing comes from the row's list, without what the sheet has (openSkills), the one list the
// write reads too (checkMulticlassSkills); the lists of the sheet's earlier sources add to it only for the extra skills.
func (c *content) levelPickKeys(b Build, row *MulticlassPick, extra int) (skills, expertise []string) {
	x := c.deriverFor(b)
	s, e, listed := x.pickLists()
	skills, expertise = x.narrowPickKeys(b, s, e, listed, row, extra)
	return skills, expertise
}

// narrowPickKeys turns the lists of the sources of the sheet b (pickLists) into the keys a new pick may take:
//   - a source that names no list gives every skill the sheet can pick (the ones it does not hold automatically) and, for
//     expertise, every skill; expertise is empty when listed is false;
//   - a skill the sheet has by another means (race, background) is no pick: another one stands in its place;
//   - a key the sheet already holds, or whose pick the sources have no room for beside the held picks, takes no new pick;
//   - at a class added by multiclassing the skills are the row's (MulticlassPick.pickable);
//   - an expertise asks for the proficiency: it stays when the sheet is proficient in the skill or the skills list gives it.
func (x *deriver) narrowPickKeys(b Build, s, e PickAllowed, listed bool, row *MulticlassPick, extra int) (skills, expertise []string) {
	skills = s.Keys
	if s.Any {
		skills = x.pickableSkills()
	}
	switch {
	case !listed:
	case e.Any:
		expertise = slices.Clone(x.c.skillOrder)
	default:
		expertise = e.Keys
	}
	skills = slices.DeleteFunc(slices.Clone(skills), func(k string) bool { return x.automaticSkills[k] })
	skills = x.stillOpen(x.skillSources(), skills, b.SkillProficiencies, func(k string) bool { _, ok := x.c.skills[k]; return ok && !x.automaticSkills[k] })
	if row != nil && row.Choose > 0 {
		skills = row.pickable(skills, extra > 0)
	}
	expertise = x.stillOpen(x.expertiseSources(), expertise, b.Expertise, func(k string) bool { _, ok := x.c.skills[k]; return ok })
	if len(expertise) > 0 {
		proficient := map[string]bool{}
		for _, sk := range derive(b, x.c).Skills {
			proficient[sk.Key] = sk.Proficiency >= ProficiencyFull
		}
		expertise = slices.DeleteFunc(expertise, func(k string) bool { return !proficient[k] && !slices.Contains(skills, k) })
	}
	return skills, expertise
}

// stillOpen are the keys of the list a new pick may take: the ones the sheet does not hold whose pick the sources
// place beside the held picks (counted says an entry of the sheet is a pick of the sources).
func (x *deriver) stillOpen(sources []pickSource, keys, held []string, counted func(string) bool) []string {
	held = pickList(held, counted)
	var out []string
	for _, k := range keys {
		if !slices.Contains(held, k) && !slices.Contains(unplacedNewPicks(sources, append(slices.Clone(held), k), held), k) {
			out = append(out, k)
		}
	}
	return out
}

// pickLists are the lists of the sources of the sheet; listed says the sheet has a source of expertise.
func (x *deriver) pickLists() (skills, expertise PickAllowed, listed bool) {
	skills = allowedBy(x.skillSources())
	if sources := x.expertiseSources(); len(sources) > 0 {
		return skills, allowedBy(sources), true
	}
	return skills, PickAllowed{}, false
}

// allowedBy is the union of the lists of sources, each key once in the order met.
func allowedBy(sources []pickSource) PickAllowed {
	var out PickAllowed
	for _, s := range sources {
		if s.Any {
			out.Any = true
			continue
		}
		for _, k := range s.From {
			if !slices.Contains(out.Keys, k) {
				out.Keys = append(out.Keys, k)
			}
		}
	}
	if out.Any {
		out.Keys = nil
	}
	return out
}

// OnAList says whether some source of the sheet b lists key for the pick field (the skill, the expertise or a tool pick
// of the sheet's choice keys). A source that names no list lists every key. A field of another kind is never on a list.
func (c *Content) OnAList(b Build, field, key string) bool {
	x := c.deriverFor(b)
	var sources []pickSource
	switch {
	case strings.HasPrefix(field, skillsField):
		sources = x.skillSources()
	case strings.HasPrefix(field, expertiseField):
		sources = x.expertiseSources()
	case strings.HasPrefix(field, choiceKeysField):
		sources = x.toolSourcesAt(field)
	}
	return slices.ContainsFunc(sources, func(s pickSource) bool { return s.allows(key) })
}

// toolSourcesAt are the sources of the tool pick the sheet's choice keys hold at the field's place: the lists of the tool
// choices of its owner. A place that holds no tool pick has none.
func (x *deriver) toolSourcesAt(field string) []pickSource {
	place, ok := strings.CutPrefix(field, choiceKeysField+"[")
	if !ok {
		return nil
	}
	i, err := strconv.Atoi(strings.TrimSuffix(place, "]"))
	if err != nil || i < 0 || i >= len(x.b.FeatureChoices) {
		return nil
	}
	choiceKey, _, ok := SplitScopedChoice(x.b.FeatureChoices[i])
	owner, isTool := strings.CutSuffix(choiceKey, toolPickSuffix)
	if !ok || !isTool {
		return nil
	}
	return x.c.toolSources(owner)
}
