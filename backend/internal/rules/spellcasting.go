package rules

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// preparation turns a spellcasting effect into PreparationKnown,
// PreparationPrepared or PreparationSpellbook.
func preparation(e *Effect) string {
	switch {
	case e.Spellbook:
		return PreparationSpellbook
	case e.Prepares:
		return PreparationPrepared
	}
	return PreparationKnown
}

// caster is one casting class of the character.
type caster struct {
	oc  ownedClass
	e   *Effect
	row *srd51.Level
	sc  *Spellcasting
}

// spellcasting computes each casting class's numbers, the spell slots
// (with the multiclass spellcaster table when more than one class casts,
// pact magic apart) and the list of the character's spells, then checks
// the spell choices.
func (x *deriver) spellcasting() {
	c := x.c
	var casters []caster
	for _, oc := range x.classes {
		cast, ok := c.casting[oc.key]
		if !ok || oc.level < cast.level {
			continue
		}
		e := cast.effect
		a := Ability(e.Ability)
		row := c.classLevels[oc.key][oc.level-1]
		sc := Spellcasting{
			Class: oc.key, ClassNamePT: c.namePT(oc.key), Ability: a,
			SaveDC: 8 + x.prof + x.mods[a], AttackBonus: x.prof + x.mods[a],
			PreparesSpells: e.Prepares, Ritual: e.Ritual,
		}
		if s := row.Spellcasting; s != nil {
			sc.CantripsKnown = s.CantripsKnown
			if !e.Prepares {
				sc.SpellsKnownMax = s.SpellsKnown
			}
			sc.MaxSpellLevel = MaxSpellLevelFromSlots(s.Slots)
		}
		if e.Prepares {
			n, err := e.preparedMax.Int(x.env)
			if err != nil {
				x.issue(IssueFormula, "", "O número de magias preparadas de %s não pôde ser calculado.", sc.ClassNamePT)
			}
			sc.PreparedMax = max(n, 0)
		}
		x.d.Spellcasting = append(x.d.Spellcasting, sc)
		casters = append(casters, caster{oc: oc, e: e, row: row})
	}
	// casters[i] goes with Spellcasting[i]; point at it once the slice
	// stops growing.
	for i := range casters {
		casters[i].sc = &x.d.Spellcasting[i]
	}

	x.d.SpellSlots = make([]int, 9)
	var slotCasters []caster
	for _, cs := range casters {
		if cs.e.Progression == "pact" {
			x.pactMagic(cs)
			continue
		}
		slotCasters = append(slotCasters, cs)
	}
	switch len(slotCasters) {
	case 0:
	case 1:
		if s := slotCasters[0].row.Spellcasting; s != nil {
			copy(x.d.SpellSlots, s.Slots[:])
		}
	default:
		x.multiclassSlots(slotCasters)
	}

	x.characterSpells(casters)
}

// pactMagic reads the warlock row's slots: all of one level.
func (x *deriver) pactMagic(cs caster) {
	s := cs.row.Spellcasting
	if s == nil {
		return
	}
	for i, n := range s.Slots {
		if n > 0 {
			x.d.PactMagic = &PactMagic{SlotLevel: i + 1, Slots: n}
		}
	}
}

// multiclassSlots applies the multiclass spellcaster table: add the levels
// of full casters and half the levels (rounded down) of half casters, and
// read the slots of that caster level. The table is the same as a full
// caster's class table, so the slots come from the first full-caster class
// in the content.
func (x *deriver) multiclassSlots(casters []caster) {
	level := 0
	for _, cs := range casters {
		switch cs.e.Progression {
		case "full":
			level += cs.oc.level
		case "half":
			level += cs.oc.level / 2
		}
	}
	if level < 1 {
		return
	}
	for _, key := range sortedKeys(x.c.casting) {
		if x.c.casting[key].effect.Progression != "full" {
			continue
		}
		if s := x.c.classLevels[key][min(level, MaxLevel)-1].Spellcasting; s != nil {
			copy(x.d.SpellSlots, s.Slots[:])
		}
		return
	}
}

// characterSpells builds Derived.Spells and checks the spell choices.
func (x *deriver) characterSpells(casters []caster) {
	c := x.c
	onClassList := func(s *srd51.Spell) bool {
		for _, cs := range casters {
			if slices.Contains(s.Classes, cs.oc.key) {
				return true
			}
			if cs.oc.subclass != nil && slices.Contains(s.Subclasses, cs.oc.subclass.Key) {
				return true
			}
		}
		return false
	}

	// What effects add: granted spells (Infernal Legacy) and cantrip
	// choices from another list (the high elf's wizard cantrip).
	granted := map[string]bool{}
	extraCantrips := 0
	var extraCantripLists []string
	for _, a := range x.active {
		e := a.effect
		for _, s := range e.Spells {
			granted[s] = true
		}
		if e.Type == "choice" && e.Choice == "cantrip" {
			extraCantrips += e.Count
			extraCantripLists = append(extraCantripLists, e.From...)
		}
	}

	// The subclass's always-prepared spells at its class level.
	alwaysPrepared := map[string]bool{}
	for _, cs := range casters {
		if cs.oc.subclass == nil {
			continue
		}
		for _, ss := range cs.oc.subclass.Spells {
			if ss.ClassLevel > cs.oc.level {
				continue
			}
			ok := true
			for _, f := range ss.WithFeatures {
				ok = ok && slices.Contains(x.b.FeatureChoices, f)
			}
			if ok {
				alwaysPrepared[ss.Spell] = true
			}
		}
	}

	maxLevel := 0
	for i, n := range x.d.SpellSlots {
		if n > 0 {
			maxLevel = i + 1
		}
	}
	if x.d.PactMagic != nil {
		maxLevel = max(maxLevel, x.d.PactMagic.SlotLevel)
	}
	hasKnownCaster := false
	hasSpellbook := false
	onlySpellbook := true
	knownMax, preparedMax, cantripsMax := 0, 0, extraCantrips
	for _, cs := range casters {
		cantripsMax += cs.sc.CantripsKnown
		switch preparation(cs.e) {
		case PreparationKnown:
			hasKnownCaster = true
			knownMax += cs.sc.SpellsKnownMax
		case PreparationSpellbook:
			hasSpellbook = true
			preparedMax += cs.sc.PreparedMax
		case PreparationPrepared:
			onlySpellbook = false
			preparedMax += cs.sc.PreparedMax
		}
	}
	knownByCaster := func(s *srd51.Spell) bool {
		for _, cs := range casters {
			if preparation(cs.e) == PreparationKnown && slices.Contains(s.Classes, cs.oc.key) {
				return true
			}
		}
		return false
	}

	seen := map[string]bool{}
	addSpell := func(s *srd51.Spell, prepared bool) {
		if seen[s.Key] {
			if prepared {
				for i := range x.d.Spells {
					if x.d.Spells[i].Spell.Key == s.Key {
						x.d.Spells[i].Prepared = true
					}
				}
			}
			return
		}
		seen[s.Key] = true
		x.d.Spells = append(x.d.Spells, CharacterSpell{Spell: c.spellEntries[s.Key], Prepared: prepared})
	}

	// Cantrips.
	for i, key := range x.b.Cantrips {
		field := fmt.Sprintf("full.cantrip_keys[%d]", i)
		s, ok := c.spells[key]
		if !ok {
			x.issue(IssueUnknownKey, field, "A magia escolhida não existe no conteúdo %s.", c.version)
			continue
		}
		if s.Level != 0 {
			x.issue(IssueSpellLevel, field, "%s não é um truque.", c.namePT(key))
		}
		fromExtraList := slices.ContainsFunc(extraCantripLists, func(cl string) bool { return slices.Contains(s.Classes, cl) })
		if !onClassList(s) && !granted[key] && !fromExtraList {
			x.issue(IssueSpellNotOnList, field, "%s não está na lista de magias do personagem.", c.namePT(key))
		}
		addSpell(s, true)
	}
	if len(x.b.Cantrips) > cantripsMax {
		x.issue(IssueSpellCount, "full.cantrip_keys", "Há %d truques; o personagem conhece %d.", len(x.b.Cantrips), cantripsMax)
	}

	// Spells known (a wizard's spellbook, or a known caster's spells) and
	// prepared.
	checkSpell := func(field, key string) (*srd51.Spell, bool) {
		s, ok := c.spells[key]
		if !ok {
			x.issue(IssueUnknownKey, field, "A magia escolhida não existe no conteúdo %s.", c.version)
			return nil, false
		}
		switch {
		case s.Level == 0:
			x.issue(IssueSpellLevel, field, "%s é um truque: vai na lista de truques.", c.namePT(key))
		case s.Level > maxLevel:
			x.issue(IssueSpellLevel, field, "%s é de %dº nível; o personagem conjura até o %dº.", c.namePT(key), s.Level, maxLevel)
		}
		if !onClassList(s) && !granted[key] && !alwaysPrepared[key] {
			x.issue(IssueSpellNotOnList, field, "%s não está na lista de magias do personagem.", c.namePT(key))
		}
		return s, true
	}
	for i, key := range x.b.SpellsKnown {
		if s, ok := checkSpell(fmt.Sprintf("full.known_spell_keys[%d]", i), key); ok {
			addSpell(s, knownByCaster(s))
		}
	}
	counted := 0
	for i, key := range x.b.SpellsPrepared {
		field := fmt.Sprintf("full.prepared_spell_keys[%d]", i)
		s, ok := checkSpell(field, key)
		if !ok {
			continue
		}
		if !alwaysPrepared[key] {
			counted++
		}
		if hasSpellbook && onlySpellbook && !slices.Contains(x.b.SpellsKnown, key) {
			x.issue(IssueSpellNotOnList, field, "%s está preparada mas não está no grimório.", c.namePT(key))
		}
		addSpell(s, true)
	}
	for _, key := range sortedKeys(alwaysPrepared) {
		if s, ok := c.spells[key]; ok {
			addSpell(s, true)
		}
	}
	if hasKnownCaster && !hasSpellbook && len(x.b.SpellsKnown) > knownMax {
		x.issue(IssueSpellCount, "full.known_spell_keys", "Há %d magias conhecidas; o personagem conhece %d.", len(x.b.SpellsKnown), knownMax)
	}
	if preparedMax > 0 && counted > preparedMax {
		x.issue(IssueSpellCount, "full.prepared_spell_keys", "Há %d magias preparadas; o personagem prepara %d.", counted, preparedMax)
	}

	slices.SortFunc(x.d.Spells, func(a, b CharacterSpell) int {
		return cmp.Or(cmp.Compare(a.Spell.Level, b.Spell.Level), comparePT(a.Spell.NamePT, b.Spell.NamePT))
	})
}

// MaxSpellLevelFromSlots is the highest spell level a class table row can
// cast: the last circle with at least one slot, or 0 when the row has none
// (Paladin and Ranger at level 1). It also works for the Warlock, whose row
// holds all its pact slots at one circle, so that circle is the highest.
func MaxSpellLevelFromSlots(slots [9]int) int {
	highest := 0
	for i, n := range slots {
		if n > 0 {
			highest = i + 1
		}
	}
	return highest
}
