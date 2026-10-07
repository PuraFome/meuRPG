package rules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// defaultASILevels are the levels with an Ability Score Improvement in the SRD
// classes (the Fighter and the Rogue have more, which a table class may list).
var defaultASILevels = []int{4, 8, 12, 16, 19}

// defaultSubclassLevel is the level a table class chooses its subclass at when
// it does not say.
const defaultSubclassLevel = 3

// castingFeatureName is the name of the feature that tells a table class or
// subclass casts spells (the same for both, and for the English name too).
const castingFeatureName = "Conjuração"

// bad is an OverlayError at an exact field of the entry at path: attr is the
// proto field's name with its leading dot (".hit_die", ".levels[4].slots[2]"),
// and reason a Reason* code. The editors point at the field it names.
func bad(key, path, attr, reason, format string, args ...any) *OverlayError {
	return ovErr(key, format, args...).at(path+attr, reason)
}

// checkCasting checks a class's or a subclass's casting and returns the class
// level it starts at (0 when the casting is wrong). path is the entry's, and the
// errors name a field of the casting ("classes[0].casting.ability"); every field
// that is wrong is reported, not only the first.
func (b *overlayBuilder) checkCasting(c *entryErrors, key, path string, cast *TableCasting, sub bool) int {
	kinds := []string{CastingFull, CastingHalf, CastingPact}
	if sub {
		kinds = []string{CastingThird}
	}
	at := path + ".casting"
	if !slices.Contains(kinds, cast.Kind) {
		c.at(key, at, ".kind", ReasonCasting, "casting kind %q is not allowed here", cast.Kind)
	}
	if _, found := abilityIndex[cast.Ability]; !found {
		c.at(key, at, ".ability", ReasonCasting, "the spellcasting ability is not one of the six")
	}
	if cast.Preparation != PreparationKnown && cast.Preparation != PreparationPrepared {
		c.at(key, at, ".preparation", ReasonCasting, "spells are %q or %q", PreparationKnown, PreparationPrepared)
	}
	if cast.PreparedMax != "" && cast.Preparation != PreparationPrepared {
		c.at(key, at, ".prepared_max", ReasonCasting, "prepared_max is for a class that prepares")
	}
	switch from := cast.ListFrom; {
	case from == "" && sub:
		c.at(key, at, ".list_from", ReasonCasting, "a third caster needs the class whose spell list it uses")
	case from == "":
	case from == key:
		c.at(key, at, ".list_from", ReasonCasting, "a class cannot reuse its own list")
	case !b.isClass(from):
		c.at(key, at, ".list_from", ReasonReference, "the spell list of class %q does not exist", from)
	default:
		// The list must be a caster's, and a class that reuses a list is not itself
		// reused, so the lists never chain.
		if _, casts := b.base.casting[from]; !casts && b.classCasts[from] == "" {
			c.at(key, at, ".list_from", ReasonCasting, "class %q has no spell list", from)
		} else if b.classListFrom[from] != "" {
			c.at(key, at, ".list_from", ReasonCasting, "class %q reuses another list; use that one", from)
		}
	}
	start := cast.StartLevel
	if start == 0 {
		switch cast.Kind {
		case CastingHalf:
			start = 2
		case CastingThird:
			start = 3
		default:
			start = 1
		}
	}
	if start < 1 || start > MaxLevel {
		c.at(key, at, ".start_level", ReasonCasting, "casting starts at level 1 to %d", MaxLevel)
		return 0
	}
	if !slices.Contains(kinds, cast.Kind) {
		return 0
	}
	return start
}

// castingEffect writes the spellcasting effect the engine reads, from the
// table's casting. owner is the class or subclass key, and classKey the class
// whose level the prepared formula reads.
func castingEffect(c *TableCasting, classKey string) *Effect {
	e := &Effect{
		Type: "spellcasting", Ability: string(c.Ability), Progression: c.Kind,
		Prepares: c.Preparation == PreparationPrepared, Ritual: c.Ritual,
	}
	if !e.Prepares {
		return e
	}
	if c.PreparedMax != "" {
		e.PreparedMax = c.PreparedMax
		return e
	}
	level := `classLevel("` + slugKey(classKey) + `")`
	switch c.Kind {
	case CastingHalf:
		level = "floor(" + level + " / 2)"
	case CastingThird:
		level = "floor(" + level + " / 3)"
	}
	e.PreparedMax = `max(1, mod("` + string(c.Ability) + `") + ` + level + ")"
	return e
}

// slugKey is a class key without its "class:" prefix: the index the formulas'
// classLevel takes.
func slugKey(classKey string) string { return strings.TrimPrefix(classKey, "class:") }

// checkRow checks the casting columns of one table row. path is the row's
// ("classes[0].levels[4]"), and the errors name the column. casts says the class
// casts at this level (the start level has come), pact that the slots are pact
// slots, and known that the casting itself was valid (with a wrong casting only
// the ranges are checked). Every cell that is wrong is reported.
func checkRow(c *entryErrors, key, path string, level int, r TableLevel, casts, pact, known bool) {
	before := len(c.list)
	if r.CantripsKnown < 0 || r.CantripsKnown > MaxCantrips {
		c.at(key, path, ".cantrips_known", ReasonTable, "level %d: cantrips known is 0 to %d", level, MaxCantrips)
	}
	if r.SpellsKnown < 0 || r.SpellsKnown > MaxKnownSpells {
		c.at(key, path, ".spells_known", ReasonTable, "level %d: spells known is 0 to %d", level, MaxKnownSpells)
	}
	slots, nonzero, second := 0, 0, -1
	for i, s := range r.Slots {
		if s < 0 || s > 9 {
			c.at(key, path, fmt.Sprintf(".slots[%d]", i), ReasonTable, "level %d: a spell slot count is 0 to 9", level)
			continue
		}
		slots += s
		if s > 0 {
			nonzero++
			if nonzero == 2 {
				second = i
			}
		}
	}
	if len(c.list) > before || !known {
		return
	}
	if !casts {
		if r.CantripsKnown != 0 {
			c.at(key, path, ".cantrips_known", ReasonTable, "level %d: the casting columns must be zero before casting starts", level)
		}
		if r.SpellsKnown != 0 {
			c.at(key, path, ".spells_known", ReasonTable, "level %d: the casting columns must be zero before casting starts", level)
		}
		if slots != 0 {
			first := slices.IndexFunc(r.Slots[:], func(n int) bool { return n > 0 })
			c.at(key, path, fmt.Sprintf(".slots[%d]", first), ReasonTable, "level %d: the casting columns must be zero before casting starts", level)
		}
		return
	}
	switch {
	case pact && nonzero != 1:
		at := ".slots"
		if second >= 0 {
			at = fmt.Sprintf(".slots[%d]", second)
		}
		c.at(key, path, at, ReasonTable, "level %d: pact magic has slots of one spell level", level)
	case nonzero == 0:
		c.at(key, path, ".slots", ReasonTable, "level %d: a caster needs spell slots", level)
	}
}

// addClass registers a class: its contract (ADR-0018, section 8), the 20-level
// table, the features, the Ability Score Improvements and the casting.
func (b *overlayBuilder) addClass(tc *TableClass, path string) error {
	n := b.n
	key := tc.Key
	slug := slugOfKey(key)
	// Every field that is wrong is reported, each at its own path, in one error.
	var c entryErrors
	if !slices.Contains([]int{6, 8, 10, 12}, tc.HitDie) {
		c.at(key, path, ".hit_die", ReasonValue, "the hit die is 6, 8, 10 or 12")
	}
	saves := make([]string, 0, 2)
	if len(tc.SavingThrows) != 2 {
		c.at(key, path, ".saving_throws", ReasonValue, "a class has two different saving throws")
	} else {
		for i, a := range tc.SavingThrows {
			if _, ok := abilityIndex[a]; !ok {
				c.at(key, path, fmt.Sprintf(".saving_throws[%d]", i), ReasonValue, "a saving throw is not one of the six abilities")
				continue
			}
			saves = append(saves, string(a))
		}
		if len(saves) == 2 && tc.SavingThrows[0] == tc.SavingThrows[1] {
			c.at(key, path, ".saving_throws[1]", ReasonValue, "a class has two different saving throws")
		}
	}
	if tc.SkillChoose < 0 || tc.SkillChoose > len(tc.SkillFrom) {
		c.at(key, path, ".skill_choose", ReasonValue, "the class chooses from the skills it lists")
	}
	for i, s := range tc.SkillFrom {
		if _, ok := b.base.skills[s]; !ok {
			c.at(key, path, fmt.Sprintf(".skill_from[%d]", i), ReasonReference, "skill %q is not in the SRD", s)
		}
	}
	for _, list := range []struct {
		field string
		keys  []string
	}{{"proficiencies", tc.Proficiencies}, {"multiclass_proficiencies", tc.MulticlassProficiencies}} {
		for i, p := range list.keys {
			if pr, ok := b.base.proficiencies[p]; !ok || pr.Kind == "skill" || pr.Kind == "saving-throw" {
				c.at(key, path, fmt.Sprintf(".%s[%d]", list.field, i), ReasonReference, "%q is not an armor, weapon or tool proficiency of the SRD", p)
			}
		}
	}
	if tc.MulticlassSkillChoose < 0 || tc.MulticlassSkillChoose > len(tc.SkillFrom) {
		c.at(key, path, ".multiclass_skill_choose", ReasonValue, "the multiclass skills come from the class's list")
	}
	mc := srd51.Multiclass{Proficiencies: slices.Clone(tc.MulticlassProficiencies)}
	mc.Minimums = minimums(&c, key, path+".minimums", tc.Minimums)
	mc.AnyOf = minimums(&c, key, path+".any_of", tc.AnyOf)
	if tc.MulticlassSkillChoose > 0 {
		mc.SkillChoices = &srd51.Choice{Choose: tc.MulticlassSkillChoose, From: slices.Clone(tc.SkillFrom)}
	}
	subLevel := tc.SubclassLevel
	if subLevel == 0 {
		subLevel = defaultSubclassLevel
	}
	if subLevel < 1 || subLevel > MaxLevel {
		c.at(key, path, ".subclass_level", ReasonValue, "the subclass is chosen at level 1 to %d", MaxLevel)
		subLevel = defaultSubclassLevel
	}
	b.classSubLevel[key] = subLevel
	if len(tc.Levels) != MaxLevel {
		c.at(key, path, ".levels", ReasonTable, "the class table needs %d rows, got %d", MaxLevel, len(tc.Levels))
	}
	asi := asiLevels(&c, key, path, tc.ASILevels)

	cl := &srd51.Class{
		Key: key, Name: tc.NamePT, HitDie: tc.HitDie, SavingThrows: saves,
		SkillChoices:  srd51.Choice{Choose: tc.SkillChoose, From: slices.Clone(tc.SkillFrom)},
		Proficiencies: slices.Clone(tc.Proficiencies), Multiclass: mc, SubclassLevel: subLevel,
	}

	// The casting, and the Spellcasting feature the engine writes for it.
	casts, start := tc.Casting.Kind != CastingNone, 0
	castKey := "feature:class-" + slug + "-spellcasting" + tableSuffix
	beforeCasting := len(c.list)
	if casts {
		start = b.checkCasting(&c, key, path, &tc.Casting, false)
		cl.Spellcasting = &srd51.ClassSpellcaster{Level: start, Ability: string(tc.Casting.Ability)}
	} else if tc.Casting.ListFrom != "" || tc.Casting.PreparedMax != "" {
		at := ".casting.list_from"
		if tc.Casting.ListFrom == "" {
			at = ".casting.prepared_max"
		}
		c.at(key, path, at, ReasonCasting, "a class that does not cast has no spell list to reuse")
	}
	// A casting with a wrong kind or start level has no rows to compare against: the cells are only range-checked.
	castingKnown := !casts || start > 0
	castingOK := len(c.list) == beforeCasting

	rows := make([]*srd51.Level, MaxLevel)
	userFeatures := 0
	limited := false
	for i := range tc.Levels {
		lvl := i + 1
		if lvl > MaxLevel {
			break
		}
		tl := &tc.Levels[i]
		if tl.ProfBonus < 0 || tl.ProfBonus > 12 {
			c.at(key, path, fmt.Sprintf(".levels[%d].prof_bonus", i), ReasonValue, "level %d: the proficiency bonus is 1 to 12 (0 for the SRD's)", lvl)
		}
		pb := tl.ProfBonus
		if pb == 0 {
			pb = 2 + (lvl-1)/4
		}
		checkRow(&c, key, fmt.Sprintf("%s.levels[%d]", path, i), lvl, tl.TableLevel, casts && lvl >= start, tc.Casting.Kind == CastingPact, castingKnown)
		row := &srd51.Level{Class: key, Level: lvl, ProfBonus: pb}
		if casts {
			row.Spellcasting = &srd51.LevelSpellcasting{CantripsKnown: tl.CantripsKnown, SpellsKnown: tl.SpellsKnown, Slots: tl.Slots}
		}
		if casts && lvl == start {
			if err := b.claim(castKey); err != nil {
				c.add(err)
			} else {
				n.features[castKey] = &srd51.Feature{Key: castKey, Name: castingFeatureName, Class: key, Level: lvl, Desc: []string{"Esta classe conjura magias: a ficha mostra a habilidade de conjuração, a CD, os espaços de magia e as magias."}}
				n.namesEN[castKey], n.namesPT[castKey] = castingFeatureName, castingFeatureName
				if castingOK {
					if err := b.compileOwn(castKey, path, castingEffect(&tc.Casting, key)); err != nil {
						c.add(err)
					}
				}
				row.Features = append(row.Features, castKey)
			}
		}
		for j := range tl.Features {
			f := &tl.Features[j]
			userFeatures++
			if userFeatures > MaxTableFeatures && !limited {
				limited = true
				c.at(key, path, fmt.Sprintf(".levels[%d].features[%d]", i, j), ReasonLimit, "more than %d features; the limit is %d per class", MaxTableFeatures, MaxTableFeatures)
			}
			if err := b.addFeature(f, featureKind{prefix: "feature:", owner: key, class: key, level: lvl}, fmt.Sprintf("%s.levels[%d].features[%d]", path, i, j)); err != nil {
				c.add(err)
				continue
			}
			row.Features = append(row.Features, f.Key)
		}
		if k := slices.Index(asi, lvl); k >= 0 {
			asiKey := "feature:class-" + slug + "-ability-score-improvement-" + strconv.Itoa(k+1) + tableSuffix
			if err := b.claim(asiKey); err != nil {
				c.add(err)
			} else {
				n.features[asiKey] = &srd51.Feature{Key: asiKey, Name: asiName, Class: key, Level: lvl, Desc: []string{asiText}}
				n.namesEN[asiKey], n.namesPT[asiKey] = asiName, asiName
				row.Features = append(row.Features, asiKey)
			}
		}
		rows[i] = row
	}
	if err := c.err(); err != nil {
		return err
	}

	b.register(tc.TableEntry)
	n.classes[key] = cl
	n.classLevels[key] = rows
	if tc.Casting.ListFrom != "" {
		n.listFrom[key] = tc.Casting.ListFrom
	}
	return nil
}

const (
	asiName = "Incremento no Valor de Habilidade"
	asiText = "Aumente uma habilidade em 2 ou duas habilidades em 1, até o máximo de 20."
)

// asiLevels checks the levels of the Ability Score Improvements, or gives the
// SRD's. Every level that is wrong is reported.
func asiLevels(c *entryErrors, key, path string, in []int) []int {
	if len(in) == 0 {
		return defaultASILevels
	}
	seen := map[int]bool{}
	for i, l := range in {
		if l < 1 || l > MaxLevel || seen[l] {
			c.at(key, path, fmt.Sprintf(".asi_levels[%d]", i), ReasonValue, "Ability Score Improvement levels are different, from 1 to %d", MaxLevel)
			continue
		}
		seen[l] = true
	}
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

// minimums turns the multiclass minimums into the SRD's shape. Every ability
// that is wrong is reported at its own field.
func minimums(c *entryErrors, key, field string, in map[Ability]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	named := 0
	// In the order of the six abilities, so the errors never depend on the map.
	for _, a := range AllAbilities() {
		v, ok := in[a]
		if !ok {
			continue
		}
		named++
		if v < 1 || v > MaxScore {
			c.list = append(c.list, ovErr(key, "a multiclass minimum is an ability at 1 to %d", MaxScore).at(field+"."+protoAbility[a], ReasonValue))
			continue
		}
		out[string(a)] = v
	}
	if named != len(in) {
		c.list = append(c.list, ovErr(key, "a multiclass minimum is one of the six abilities").at(field, ReasonValue))
	}
	return out
}

// addSubclass registers a subclass, and puts it in its class's list on a copy of
// the class.
func (b *overlayBuilder) addSubclass(ts *TableSubclass, path string) error {
	n := b.n
	key := ts.Key
	parent := ts.Class
	var c entryErrors
	parentOK := b.isClass(parent)
	if !parentOK {
		c.at(key, path, ".class_key", ReasonReference, "the class %q does not exist", parent)
	}
	subLevel := b.classSubLevel[parent]
	if subLevel == 0 {
		if cl, ok := b.base.classes[parent]; ok {
			subLevel = cl.SubclassLevel
		}
	}
	if parentOK && ts.Level != 0 && ts.Level != subLevel {
		c.at(key, path, ".level", ReasonValue, "the subclass is chosen at level %d, the class's; got %d", subLevel, ts.Level)
	}
	checkTextAt(&c, key, path, ".desc_pt", ts.DescPT)

	var casting *classCasting
	start := 0
	beforeCasting := len(c.list)
	if ts.Casting != nil {
		if _, casts := b.base.casting[parent]; parentOK && (casts || b.classCasts[parent] != CastingNone) {
			c.at(key, path, ".casting", ReasonCasting, "a third caster is a subclass of a class that does not cast")
		}
		start = b.checkCasting(&c, key, path, ts.Casting, true)
		if start > 0 && start < subLevel {
			c.at(key, path, ".casting.start_level", ReasonCasting, "casting cannot start before the subclass, at level %d", subLevel)
			start = 0
		}
		casting = &classCasting{level: start, list: ts.Casting.ListFrom, sub: key}
	}
	castingKnown := casting == nil || start > 0
	castingOK := len(c.list) == beforeCasting

	rows := map[int]*srd51.Level{}
	features, last := 0, 0
	limited := false
	for i := range ts.Levels {
		tl := &ts.Levels[i]
		if tl.Level < 1 || tl.Level > MaxLevel || tl.Level <= last {
			c.at(key, path, fmt.Sprintf(".levels[%d].level", i), ReasonTable, "subclass levels go from 1 to %d, in ascending order", MaxLevel)
			continue
		}
		last = tl.Level
		if len(tl.Features) > 0 && tl.Level < subLevel {
			c.at(key, path, fmt.Sprintf(".levels[%d].features", i), ReasonTable, "level %d: subclass features start at level %d", tl.Level, subLevel)
		}
		row := &srd51.Level{Class: parent, Subclass: key, Level: tl.Level}
		for j := range tl.Features {
			f := &tl.Features[j]
			features++
			if features > MaxTableFeatures && !limited {
				limited = true
				c.at(key, path, fmt.Sprintf(".levels[%d].features[%d]", i, j), ReasonLimit, "more than %d features; the limit is %d per subclass", MaxTableFeatures, MaxTableFeatures)
			}
			if err := b.addFeature(f, featureKind{prefix: "feature:", owner: key, class: parent, subclass: key, level: tl.Level}, fmt.Sprintf("%s.levels[%d].features[%d]", path, i, j)); err != nil {
				c.add(err)
				continue
			}
			row.Features = append(row.Features, f.Key)
		}
		rows[tl.Level] = row
		checkRow(&c, key, fmt.Sprintf("%s.levels[%d]", path, i), tl.Level, tl.TableLevel, casting != nil && tl.Level >= start, false, castingKnown)
		if casting != nil {
			row.Spellcasting = &srd51.LevelSpellcasting{CantripsKnown: tl.CantripsKnown, SpellsKnown: tl.SpellsKnown, Slots: tl.Slots}
		}
	}
	if casting != nil && start > 0 {
		for lvl := start; lvl <= MaxLevel; lvl++ {
			if rows[lvl] == nil {
				c.at(key, path, ".levels", ReasonTable, "a third caster needs a table row for every level from %d to %d (missing %d)", start, MaxLevel, lvl)
				break
			}
		}
		castKey := "feature:subclass-" + slugOfKey(key) + "-spellcasting" + tableSuffix
		if err := b.claim(castKey); err != nil {
			c.add(err)
		} else if castingOK && rows[start] != nil {
			n.features[castKey] = &srd51.Feature{Key: castKey, Name: castingFeatureName, Class: parent, Subclass: key, Level: start, Desc: []string{"Esta subclasse conjura magias: a ficha mostra a habilidade de conjuração, a CD, os espaços de magia e as magias."}}
			n.namesEN[castKey], n.namesPT[castKey] = castingFeatureName, castingFeatureName
			eff := castingEffect(ts.Casting, parent)
			if err := b.compileOwn(castKey, path, eff); err != nil {
				c.add(err)
			} else {
				rows[start].Features = append([]string{castKey}, rows[start].Features...)
				casting.effect = eff
				n.subCasting[key] = *casting
			}
		}
	}

	sub := &srd51.Subclass{Key: key, Name: ts.NamePT, Class: parent, Desc: slices.Clone(ts.DescPT)}
	if _, parentCasts := b.base.casting[parent]; parentOK && len(ts.AlwaysPrepared) > 0 && ts.Casting == nil && !parentCasts && b.classCasts[parent] == CastingNone {
		c.list = append(c.list, ovErr(key, "always-prepared spells need a class or subclass that casts").at(path+".always_prepared", ReasonCasting))
	}
	for i, ap := range ts.AlwaysPrepared {
		at := fmt.Sprintf(".always_prepared[%d]", i)
		if ap.ClassLevel < 1 || ap.ClassLevel > MaxLevel {
			c.at(key, path, at+".class_level", ReasonValue, "an always-prepared spell comes at class level 1 to %d", MaxLevel)
		}
		if !b.isSpell(ap.Spell) {
			c.at(key, path, at+".spell_key", ReasonReference, "the spell %q does not exist", ap.Spell)
			continue
		}
		if sp, ok := b.n.spells[ap.Spell]; ok && sp.Level == 0 {
			c.at(key, path, at+".spell_key", ReasonValue, "an always-prepared spell is a leveled spell, not a cantrip")
			continue
		}
		sub.Spells = append(sub.Spells, srd51.SubclassSpell{Spell: ap.Spell, ClassLevel: ap.ClassLevel})
	}
	if err := c.err(); err != nil {
		return err
	}
	b.register(ts.TableEntry)
	n.subclasses[key] = sub
	n.subclassLevels[key] = rows
	cp := *n.classes[parent]
	cp.Subclasses = append(slices.Clone(cp.Subclasses), key)
	n.classes[parent] = &cp
	return nil
}
