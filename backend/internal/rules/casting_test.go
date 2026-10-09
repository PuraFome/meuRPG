package rules

import "testing"

func TestCastingTimeMinutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		raw          string
		minutes      int
		ritualMinute int
		long, rLong  bool
	}{
		// SRD 5.1, "Casting Time" and "Rituals": the ritual version takes 10 minutes longer.
		{"an action is cast at once", "1 action", 0, 10, false, true},
		{"a bonus action", "1 bonus action", 0, 10, false, true},
		{"a reaction", "1 reaction", 0, 10, false, true},
		{"1 minute", "1 minute", 1, 11, true, true},
		{"10 minutes (Prayer of Healing)", "10 minutes", 10, 20, true, true},
		{"1 hour (Find Familiar)", "1 hour", 60, 70, true, true},
		{"8 hours", "8 hours", 480, 490, true, true},
		{"24 hours", "24 hours", 1440, 1450, true, true},
		{"a casting time the content cannot read", "special", 0, 10, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ct := parseCastingTime(tt.raw)
			if got := CastMinutes(ct, false); got != tt.minutes {
				t.Errorf("CastMinutes(%q) = %d, want %d", tt.raw, got, tt.minutes)
			}
			if got := CastMinutes(ct, true); got != tt.ritualMinute {
				t.Errorf("CastMinutes(%q, ritual) = %d, want %d", tt.raw, got, tt.ritualMinute)
			}
			if got := IsLongCast(ct, false); got != tt.long {
				t.Errorf("IsLongCast(%q) = %v, want %v", tt.raw, got, tt.long)
			}
			if got := IsLongCast(ct, true); got != tt.rLong {
				t.Errorf("IsLongCast(%q, ritual) = %v, want %v (a ritual is always long)", tt.raw, got, tt.rLong)
			}
		})
	}
}

func TestSpellDurationSecondsAndRests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		seconds int
		timed   bool
		lasts   bool
		rest    string
	}{
		{"instantaneous leaves nothing to end", "Instantaneous", 0, false, false, RestNone},
		{"until dispelled outlasts every rest", "Until dispelled", 0, false, true, RestNone},
		{"a special duration is left to the table", "Special", 0, false, false, RestNone},
		{"a round is 6 seconds", "1 round", 6, true, true, RestShort},
		{"1 minute", "Up to 1 minute", 60, true, true, RestShort},
		{"10 minutes (Detect Magic)", "Up to 10 minutes", 600, true, true, RestShort},
		{"exactly 1 hour fits a short rest (False Life)", "1 hour", 3600, true, true, RestShort},
		{"2 hours needs a long rest", "2 hours", 7200, true, true, RestLong},
		{"8 hours is Mage Armor and Aid", "8 hours", 28800, true, true, RestLong},
		{"24 hours outlasts a long rest", "24 hours", 86400, true, true, RestNone},
		{"1 day", "1 day", 86400, true, true, RestNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := parseDuration(tt.raw, false)
			secs, timed := d.Seconds()
			if secs != tt.seconds || timed != tt.timed {
				t.Errorf("Seconds(%q) = %d, %v; want %d, %v", tt.raw, secs, timed, tt.seconds, tt.timed)
			}
			if got := d.Lasts(); got != tt.lasts {
				t.Errorf("Lasts(%q) = %v, want %v", tt.raw, got, tt.lasts)
			}
			if got := EndedByRest(d); got != tt.rest {
				t.Errorf("EndedByRest(%q) = %q, want %q", tt.raw, got, tt.rest)
			}
		})
	}
	// A rest ends a spell when its whole duration fits in the rest's length.
	hour, eight := parseDuration("1 hour", false), parseDuration("8 hours", false)
	if !RestEnds(hour, ShortRestMinutes) || RestEnds(eight, ShortRestMinutes) || !RestEnds(eight, LongRestMinutes) {
		t.Errorf("RestEnds: a short rest should end the 1-hour spell only, a long rest both")
	}
	if RestEnds(parseDuration("Until dispelled", false), LongRestMinutes) {
		t.Errorf("a long rest does not end a spell that lasts until dispelled")
	}
}

func TestSpellStandingRituals(t *testing.T) {
	t.Parallel()
	spell := func(key string, prepared bool) CharacterSpell {
		return CharacterSpell{Spell: SpellEntry{Key: key, Ritual: true}, Prepared: prepared}
	}
	wizard := Spellcasting{Class: "class:wizard", Ritual: true, PreparesSpells: true}
	cleric := Spellcasting{Class: "class:cleric", Ritual: true, PreparesSpells: true}
	bard := Spellcasting{Class: "class:bard", Ritual: true}
	sorcerer := Spellcasting{Class: "class:sorcerer"}
	tests := []struct {
		name string
		d    Derived
		key  string
		tag  bool
		want SpellStanding
	}{
		// SRD 5.1, "Rituals": the caster must have the spell prepared or on the list of
		// spells known, unless the ritual feature says otherwise, as the wizard's does.
		{"a wizard casts a ritual from the spellbook without preparing it", Derived{Spellcasting: []Spellcasting{wizard}, Spells: []CharacterSpell{spell("spell:detect-magic", false)}}, "spell:detect-magic", true, SpellStanding{Known: true, CanRitual: true}},
		{"a cleric needs the ritual prepared", Derived{Spellcasting: []Spellcasting{cleric}, Spells: []CharacterSpell{spell("spell:detect-magic", false)}}, "spell:detect-magic", true, SpellStanding{Known: true}},
		{"a cleric casts a prepared ritual", Derived{Spellcasting: []Spellcasting{cleric}, Spells: []CharacterSpell{spell("spell:detect-magic", true)}}, "spell:detect-magic", true, SpellStanding{Known: true, Prepared: true, CanRitual: true}},
		{"a bard casts a known ritual", Derived{Spellcasting: []Spellcasting{bard}, Spells: []CharacterSpell{spell("spell:detect-magic", true)}}, "spell:detect-magic", true, SpellStanding{Known: true, Prepared: true, CanRitual: true}},
		{"a sorcerer has no ritual feature", Derived{Spellcasting: []Spellcasting{sorcerer}, Spells: []CharacterSpell{spell("spell:detect-magic", true)}}, "spell:detect-magic", true, SpellStanding{Known: true, Prepared: true}},
		{"a spell without the tag is never a ritual", Derived{Spellcasting: []Spellcasting{wizard}, Spells: []CharacterSpell{spell("spell:magic-missile", true)}}, "spell:magic-missile", false, SpellStanding{Known: true, Prepared: true}},
		{"a spell that is not on the sheet", Derived{Spellcasting: []Spellcasting{wizard}}, "spell:detect-magic", true, SpellStanding{}},
		{"the Pact of the Chain casts Find Familiar as a ritual without having it", Derived{Features: []Feature{{Key: "feature:pact-of-the-chain"}}}, "spell:find-familiar", true, SpellStanding{Known: true, CanRitual: true}},
		{"the pact gives no other ritual", Derived{Features: []Feature{{Key: "feature:pact-of-the-chain"}}}, "spell:detect-magic", true, SpellStanding{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.d.SpellStanding(tt.key, tt.tag); got != tt.want {
				t.Errorf("SpellStanding(%s) = %+v, want %+v", tt.key, got, tt.want)
			}
		})
	}
}

// TestEverySRDRitualClassCasts checks the class table: the SRD gives the Ritual
// Casting feature to the bard, cleric, druid and wizard, and to no other class.
func TestEverySRDRitualClassCasts(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	want := map[string]bool{"class:bard": true, "class:cleric": true, "class:druid": true, "class:wizard": true}
	for _, cl := range []string{"barbarian", "bard", "cleric", "druid", "fighter", "monk", "paladin", "ranger", "rogue", "sorcerer", "warlock", "wizard"} {
		key := "class:" + cl
		d := Derive(standard(key, 5), c)
		got := false
		for _, sc := range d.Spellcasting {
			got = got || sc.Ritual
		}
		if got != want[key] {
			t.Errorf("%s casts rituals = %v, want %v", key, got, want[key])
		}
	}
}

// TestRitualSpellsAreTheSRDRituals counts what the content calls a ritual and a long
// spell: 29 rituals, in the casting times the cast flow reads.
func TestRitualSpellsAreTheSRDRituals(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	rituals, long, unread := 0, 0, 0
	for _, e := range c.ListSpells(SpellFilter{}) {
		if e.Ritual {
			rituals++
		}
		if e.CastingTime.Unit == CastMinute || e.CastingTime.Unit == CastHour {
			long++
		}
		if e.CastingTime.Unit == "" {
			unread++
		}
	}
	if rituals != 29 {
		t.Errorf("%d ritual spells, want the SRD's 29", rituals)
	}
	if long != 59 { // 57 that cannot be cast today, and Find Familiar and Animate Dead, which CastSummon casts
		t.Errorf("%d spells with a casting time of minutes or hours, want 59", long)
	}
	if unread != 0 {
		t.Errorf("%d spells have a casting time that did not parse", unread)
	}
}

func TestMageArmorBaseArmorClass(t *testing.T) {
	t.Parallel()
	c := loadForTest(t)
	wizard := func(mage bool) Derived {
		b := pensantus() // DEX 16 (+3): 10 + 3 = 13 without it, 13 + 3 = 16 with it
		b.MageArmor = mage
		return Derive(b, c)
	}
	if got := wizard(false).ArmorClass; got != 13 {
		t.Fatalf("AC without Mage Armor = %d, want 13", got)
	}
	d := wizard(true)
	if d.ArmorClass != 16 || d.ArmorClassDescription != "Armadura Arcana" {
		t.Errorf("AC with Mage Armor = %d (%q), want 16 (Armadura Arcana)", d.ArmorClass, d.ArmorClassDescription)
	}
	// A shield adds on top of the new base.
	b := pensantus()
	b.MageArmor, b.Shield = true, true
	if got := Derive(b, c).ArmorClass; got != 18 {
		t.Errorf("AC with Mage Armor and a shield = %d, want 18", got)
	}
	// The spell asks for a creature that wears no armor.
	b = pensantus()
	b.MageArmor, b.Armor = true, "equipment:leather-armor"
	worn := Derive(b, c).ArmorClass
	b.MageArmor = false
	if plain := Derive(b, c).ArmorClass; worn != plain {
		t.Errorf("AC in leather armor with Mage Armor = %d, without = %d; the spell does nothing for armor", worn, plain)
	}
	// The best base wins: a barbarian's Unarmored Defense (10 + DEX + CON) is higher than Mage Armor's 13 + DEX with a CON modifier above +3.
	b = standard("class:barbarian", 1)
	b.BaseScores = map[Ability]int{STR: 15, DEX: 14, CON: 20, INT: 8, WIS: 10, CHA: 8} // human: +1 each
	base := Derive(b, c).ArmorClass
	b.MageArmor = true
	if got := Derive(b, c).ArmorClass; got != base {
		t.Errorf("a barbarian with Unarmored Defense 10 + DEX + CON has AC %d, and Mage Armor must not lower it (got %d)", base, got)
	}
}
