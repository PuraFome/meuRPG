package rules

// The table's 20-level defaults (MR-025): the numbers the class
// editor starts from, so the browser never computes a class table. They come
// from the SRD's own class tables, read from the content, and the master edits
// them from there; every default passes the same checks as an edit (the tests
// build a class from each).

// DefaultRow is one row of a default class table.
type DefaultRow struct {
	// Level is 1 to 20.
	Level int
	// ProfBonus is the SRD's proficiency bonus at the level.
	ProfBonus int
	// CantripsKnown and SpellsKnown are the casting columns; both are 0 for a
	// class that prepares from its list (SpellsKnown) or that gets no cantrips.
	CantripsKnown, SpellsKnown int
	// Slots are the spell slots by circle, 1st first; for pact magic exactly one
	// entry is not zero (the pact slots, all of one level). All zero before the
	// casting starts.
	Slots [9]int
}

// DefaultTable is the default table of one kind of casting.
type DefaultTable struct {
	// Kind is CastingNone, CastingFull, CastingHalf, CastingPact or CastingThird
	// (a subclass's).
	Kind string
	// Preparation is PreparationKnown or PreparationPrepared; empty for a class
	// that does not cast.
	Preparation string
	// StartLevel is the class level casting starts at by default: 1 for a full
	// caster and for pact magic, 2 for a half caster, 3 for a third caster.
	StartLevel int
	// Rows are the 20 rows, level 1 first.
	Rows []DefaultRow
	// Reference is the SRD class whose table the numbers are, empty for a third
	// caster (whose slots are the full table at a third of the level, rounded up).
	Reference string
}

// TableDefaults are the defaults the class editor starts from.
type TableDefaults struct {
	// ProfBonus is the proficiency bonus by level, the same in every table: +2 at
	// levels 1 to 4 up to +6 at 17 to 20.
	ProfBonus []int
	// ASILevels are the levels with an Ability Score Improvement (4, 8, 12, 16 and
	// 19).
	ASILevels []int
	// SubclassLevel is the level a class chooses its subclass at unless it says
	// (3).
	SubclassLevel int
	// Tables are the casting tables: none, then full, half, pact and third, each
	// with the ways of preparing that it offers.
	Tables []DefaultTable
}

// defaultSources says which SRD class gives each table the numbers: the slots
// of every kind, the cantrips and, for a class that knows its spells, the spells
// known. A class that prepares has no spells known, and a half caster has no
// cantrips (the Paladin's and the Ranger's tables).
var defaultSources = []struct {
	kind, preparation, reference string
	start                        int
}{
	{CastingFull, PreparationPrepared, "class:cleric", 1},
	{CastingFull, PreparationKnown, "class:sorcerer", 1},
	{CastingHalf, PreparationPrepared, "class:paladin", 2},
	{CastingHalf, PreparationKnown, "class:ranger", 2},
	{CastingPact, PreparationKnown, "class:warlock", 1},
	{CastingPact, PreparationPrepared, "class:warlock", 1},
}

// The third caster's defaults (a subclass that casts): its slots are the
// full table at a third of the level, rounded up; two cantrips, a third from
// level 10; and, for the one that knows its spells, 3 at levels 3 and 4, one
// more every two levels after that, 11 at level 20.
const (
	thirdStartLevel       = 3
	thirdLevelDivisor     = 3
	thirdCantrips         = 2
	thirdMoreCantripLevel = 10
	thirdSpellsKnownStart = 3
	thirdSpellsKnownEvery = 2
)

// profBonusEvery is how many levels each step of the proficiency bonus lasts.
const profBonusEvery = 4

// TableDefaults gives the defaults of the 20-level table, from the SRD's class
// tables. Table classes never feed it: the numbers are always the SRD's.
func (c *Content) TableDefaults() TableDefaults {
	d := TableDefaults{
		ASILevels:     append([]int(nil), defaultASILevels...),
		SubclassLevel: defaultSubclassLevel,
	}
	for lvl := 1; lvl <= MaxLevel; lvl++ {
		d.ProfBonus = append(d.ProfBonus, 2+(lvl-1)/profBonusEvery)
	}
	none := DefaultTable{Kind: CastingNone, StartLevel: 0}
	for lvl := 1; lvl <= MaxLevel; lvl++ {
		none.Rows = append(none.Rows, d.defaultRow(lvl))
	}
	d.Tables = append(d.Tables, none)
	for _, src := range defaultSources {
		d.Tables = append(d.Tables, c.sourceDefaultTable(&d, src.kind, src.preparation, src.reference, src.start))
	}
	// These numbers are our default, written in our own words: the master
	// edits what they want.
	for _, prep := range []string{PreparationKnown, PreparationPrepared} {
		d.Tables = append(d.Tables, c.thirdDefaultTable(&d, prep))
	}
	return d
}

// defaultRow is a level's row with its proficiency bonus and nothing else.
func (d *TableDefaults) defaultRow(lvl int) DefaultRow {
	return DefaultRow{Level: lvl, ProfBonus: d.ProfBonus[lvl-1]}
}

// sourceDefaultTable copies a reference class's spellcasting columns from its
// start level; a class that prepares has no spells known.
func (c *Content) sourceDefaultTable(d *TableDefaults, kind, preparation, reference string, start int) DefaultTable {
	t := DefaultTable{Kind: kind, Preparation: preparation, StartLevel: start, Reference: reference}
	for lvl := 1; lvl <= MaxLevel; lvl++ {
		r := d.defaultRow(lvl)
		if sc := c.c.classLevels[reference][lvl-1].Spellcasting; sc != nil && lvl >= start {
			r.CantripsKnown, r.Slots = sc.CantripsKnown, sc.Slots
			if preparation == PreparationKnown {
				r.SpellsKnown = sc.SpellsKnown
			}
		}
		t.Rows = append(t.Rows, r)
	}
	return t
}

// thirdDefaultTable is the third caster's table (the constants above).
func (c *Content) thirdDefaultTable(d *TableDefaults, prep string) DefaultTable {
	t := DefaultTable{Kind: CastingThird, Preparation: prep, StartLevel: thirdStartLevel}
	for lvl := 1; lvl <= MaxLevel; lvl++ {
		r := d.defaultRow(lvl)
		if lvl >= t.StartLevel {
			r.Slots = c.c.classLevels["class:wizard"][(lvl+thirdLevelDivisor-1)/thirdLevelDivisor-1].Spellcasting.Slots
			r.CantripsKnown = thirdCantrips
			if lvl >= thirdMoreCantripLevel {
				r.CantripsKnown = thirdCantrips + 1
			}
			if prep == PreparationKnown {
				r.SpellsKnown = thirdSpellsKnownStart + (lvl-thirdStartLevel)/thirdSpellsKnownEvery
			}
		}
		t.Rows = append(t.Rows, r)
	}
	return t
}
