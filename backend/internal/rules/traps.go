package rules

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// Trap presets (MR-035, Etapa 9, D5): the eight sample traps of the SRD 5.1
// ("Sample Traps" in the Dungeon Master's rules, 5e-database's 5e-SRD-Rules
// section "sample-traps"), in our own words, with the SRD's numbers, plus the
// SRD's tables of how severe a trap is. The file is effects/traps.json, written
// by hand: the loader checks it against a closed schema and refuses anything
// else (ADR-0008), and changing it raises the effects revision.
//
// A preset only fills the master's form; the master edits anything. The play
// module rolls and applies a trap's parts, with what package grid says about
// the area; nothing here rolls a die.

// The ways a trap is triggered.
const (
	// TrapTriggerEnter: a creature of a player enters the area (a pressure
	// plate, a trip wire, a false floor). NPCs never fire it by themselves.
	TrapTriggerEnter = "enter"
	// TrapTriggerManual: the master fires it (a chest opened, a door).
	TrapTriggerManual = "manual"
)

// Who a trap's effect reaches.
const (
	// TrapTargetsArea: every creature in the trap's area.
	TrapTargetsArea = "area"
	// TrapTargetsManual: the master picks who is caught, because the effect
	// comes from somewhere else (a statue's cone, a rolling sphere) or hits at
	// random (darts).
	TrapTargetsManual = "manual"
)

// Whom a trap's saving throw is asked of.
const (
	// TrapSaveCaught: each creature the trap caught.
	TrapSaveCaught = "caught"
	// TrapSaveHit: each creature one of the trap's attacks hit.
	TrapSaveHit = "hit"
)

// What a creature that passes the saving throw takes.
const (
	// TrapPassHalf: half the damage the failure would have done.
	TrapPassHalf = "half"
	// TrapPassNone: nothing.
	TrapPassNone = "none"
)

// The kinds of trap, as the SRD says.
const (
	TrapMechanical = "mechanical"
	TrapMagic      = "magic"
)

// TrapDamage is damage of one type. Dice may be a flat number ("1" for the
// poison needle's piercing damage).
type TrapDamage struct {
	Dice DiceFormula
	// Type is a damage type key, "damage-type:poison".
	Type string
}

// TrapCondition is a condition a trap gives to every creature it caught.
type TrapCondition struct {
	// Condition is a condition key, "condition:restrained".
	Condition string
	// DurationPT says how long it lasts, as text for the master ("1 hora"),
	// or is empty.
	DurationPT string
}

// TrapAttack is a trap that attacks: Count attacks with a bonus, each against
// one target. A hit does Damage; a saving throw that follows is asked of the
// creatures hit.
type TrapAttack struct {
	Bonus  int
	Count  int
	Damage TrapDamage
}

// TrapOnFail is what a creature that fails the saving throw takes: damage
// and/or a condition.
type TrapOnFail struct {
	Damage []TrapDamage
	// Condition is a condition key, or empty, and DurationPT how long it lasts.
	Condition  string
	DurationPT string
}

// TrapSave is the saving throw of a trap.
type TrapSave struct {
	Ability Ability
	DC      int
	// AppliesTo is TrapSaveCaught or TrapSaveHit.
	AppliesTo string
	OnFail    TrapOnFail
	// OnPass is TrapPassHalf or TrapPassNone.
	OnPass string
}

// TrapPreset is a trap the master can start from.
type TrapPreset struct {
	// Key is "trap:<name>"; NamePT is its Portuguese name.
	Key    string
	NamePT string
	// Kind is TrapMechanical or TrapMagic.
	Kind string
	// DescriptionPT is the preset's text, in our own words.
	DescriptionPT string
	// NoticeDC is the DC to notice it (Wisdom (Perception), passive or active),
	// and 0 when the SRD gives none (the poison needle). FindDC is the DC to find
	// it by searching (Intelligence (Investigation)); where the SRD gives only
	// one DC for spotting, it is used for both.
	NoticeDC int
	FindDC   int
	// FindDCIsOurs says the SRD gives no DC to find this trap (only to spot it),
	// so FindDC is ours: the DC to spot, reused. The DCs of the other presets
	// are the SRD's.
	FindDCIsOurs bool
	// Trigger is TrapTriggerEnter or TrapTriggerManual.
	Trigger string
	// AreaSize is the side, in squares, of the block around the trap's square
	// that fires it: 1 for the square alone, up to 4.
	AreaSize int
	// Targets is TrapTargetsArea or TrapTargetsManual.
	Targets string
	// FallFt is the depth of a pit in feet, 0 for a trap that is not a pit. The
	// fall damage in Damage is FallDice(FallFt).
	FallFt int
	// Damage lands on every creature caught, with no roll to avoid it.
	Damage []TrapDamage
	// Conditions are given to every creature caught.
	Conditions []TrapCondition
	// Attack and Save are nil when the trap has none.
	Attack *TrapAttack
	Save   *TrapSave
}

// IntRange is a closed range of whole numbers.
type IntRange struct {
	Min, Max int
}

// TrapSeverity is one row of the SRD's "Trap Save DCs and Attack Bonuses".
type TrapSeverity struct {
	// Key is "setback", "dangerous" or "deadly", NamePT "Revés", "Perigosa" or
	// "Mortal".
	Key, NamePT string
	SaveDC      IntRange
	AttackBonus IntRange
}

// TrapDamageRow is one row of the SRD's "Damage Severity by Level": the damage
// that suits characters of levels FromLevel to ToLevel.
type TrapDamageRow struct {
	FromLevel, ToLevel int
	Setback            DiceFormula
	Dangerous          DiceFormula
	Deadly             DiceFormula
}

// FallDice is the damage of a fall (SRD): 1d6 bludgeoning for every 10 feet,
// up to 20d6. A fall of less than 10 feet does none (Count 0).
func FallDice(feet int) DiceFormula {
	n := min(max(feet, 0)/10, 20)
	if n == 0 {
		return DiceFormula{}
	}
	return DiceFormula{Count: n, Sides: 6}
}

// traps is what effects/traps.json holds, once checked.
type traps struct {
	presets    []TrapPreset
	severities []TrapSeverity
	damageRows []TrapDamageRow
}

type trapsFile struct {
	Severity struct {
		Levels []struct {
			Key         string                 `json:"key"`
			NamePT      string                 `json:"name_pt"`
			SaveDC      struct{ Min, Max int } `json:"save_dc"`
			AttackBonus struct{ Min, Max int } `json:"attack_bonus"`
		} `json:"levels"`
		DamageByLevel []struct {
			From      int    `json:"from"`
			To        int    `json:"to"`
			Setback   string `json:"setback"`
			Dangerous string `json:"dangerous"`
			Deadly    string `json:"deadly"`
		} `json:"damage_by_level"`
	} `json:"severity"`
	Traps []trapJSON `json:"traps"`
}

type trapDamageJSON struct {
	Dice string `json:"dice"`
	Type string `json:"type"`
}

type trapJSON struct {
	Key           string           `json:"key"`
	Kind          string           `json:"kind"`
	DescriptionPT string           `json:"description_pt"`
	NoticeDC      int              `json:"notice_dc,omitempty"`
	FindDC        int              `json:"find_dc"`
	FindDCOurs    bool             `json:"find_dc_ours,omitempty"`
	Trigger       string           `json:"trigger"`
	AreaSize      int              `json:"area_size"`
	Targets       string           `json:"targets"`
	FallFt        int              `json:"fall_ft,omitempty"`
	Damage        []trapDamageJSON `json:"damage,omitempty"`
	Conditions    []struct {
		Condition  string `json:"condition"`
		DurationPT string `json:"duration_pt,omitempty"`
	} `json:"conditions,omitempty"`
	Attack *struct {
		Bonus  int            `json:"bonus"`
		Count  int            `json:"count"`
		Damage trapDamageJSON `json:"damage"`
	} `json:"attack,omitempty"`
	Save *struct {
		Ability   string `json:"ability"`
		DC        int    `json:"dc"`
		AppliesTo string `json:"applies_to"`
		OnFail    struct {
			Damage     []trapDamageJSON `json:"damage,omitempty"`
			Condition  string           `json:"condition,omitempty"`
			DurationPT string           `json:"duration_pt,omitempty"`
		} `json:"on_fail"`
		OnPass string `json:"on_pass"`
	} `json:"save,omitempty"`
}

var severityOrder = []string{"setback", "dangerous", "deadly"}

// loadTraps reads and checks effects/traps.json.
func (c *content) loadTraps(fsys fs.FS) error {
	const name = "effects/traps.json"
	var f trapsFile
	if err := readJSON(fsys, name, &f); err != nil {
		return err
	}
	fail := func(format string, a ...any) error {
		return fmt.Errorf("%s: %s", name, fmt.Sprintf(format, a...))
	}

	if len(f.Severity.Levels) != len(severityOrder) {
		return fail("severity.levels needs %d entries, got %d", len(severityOrder), len(f.Severity.Levels))
	}
	for i, l := range f.Severity.Levels {
		saveDC, attack := IntRange(l.SaveDC), IntRange(l.AttackBonus)
		switch {
		case l.Key != severityOrder[i]:
			return fail("severity.levels[%d] must be %q, got %q", i, severityOrder[i], l.Key)
		case strings.TrimSpace(l.NamePT) == "":
			return fail("severity %s has no name_pt", l.Key)
		case saveDC.Min < 1 || saveDC.Min > saveDC.Max || attack.Min < 0 || attack.Min > attack.Max:
			return fail("severity %s has an empty or negative range", l.Key)
		case i > 0 && (saveDC.Min <= c.traps.severities[i-1].SaveDC.Max || attack.Min <= c.traps.severities[i-1].AttackBonus.Max):
			return fail("severity %s must be above %s", l.Key, severityOrder[i-1])
		}
		c.traps.severities = append(c.traps.severities, TrapSeverity{Key: l.Key, NamePT: l.NamePT, SaveDC: saveDC, AttackBonus: attack})
	}
	next := 1
	for _, r := range f.Severity.DamageByLevel {
		row := TrapDamageRow{FromLevel: r.From, ToLevel: r.To}
		if r.From != next || r.To < r.From || r.To > MaxLevel {
			return fail("severity.damage_by_level must cover levels 1 to %d in order, with no gap (at level %d)", MaxLevel, next)
		}
		next = r.To + 1
		for _, one := range []struct {
			text string
			into *DiceFormula
		}{{r.Setback, &row.Setback}, {r.Dangerous, &row.Dangerous}, {r.Deadly, &row.Deadly}} {
			d, ok := ParseDice(one.text)
			if !ok || d.Count < 1 || d.Bonus != 0 || d.AddsModifier {
				return fail("severity.damage_by_level %d-%d: %q is not dice such as \"2d10\"", r.From, r.To, one.text)
			}
			*one.into = d
		}
		c.traps.damageRows = append(c.traps.damageRows, row)
	}
	if next != MaxLevel+1 {
		return fail("severity.damage_by_level stops at level %d, it must reach %d", next-1, MaxLevel)
	}

	damage := func(where string, in trapDamageJSON) (TrapDamage, error) {
		d, ok := ParseDice(in.Dice)
		if !ok || d.AddsModifier || (d.Count == 0 && d.Bonus < 1) || (d.Count > 0 && d.Bonus != 0) {
			return TrapDamage{}, fail("%s: %q is not dice such as \"2d10\" or a flat number", where, in.Dice)
		}
		if !strings.HasPrefix(in.Type, "damage-type:") || !c.exists(in.Type) {
			return TrapDamage{}, fail("%s: %q is not a damage type", where, in.Type)
		}
		return TrapDamage{Dice: d, Type: in.Type}, nil
	}
	damages := func(where string, in []trapDamageJSON) ([]TrapDamage, error) {
		var out []TrapDamage
		for i, d := range in {
			td, err := damage(fmt.Sprintf("%s[%d]", where, i), d)
			if err != nil {
				return nil, err
			}
			out = append(out, td)
		}
		return out, nil
	}
	condition := func(where, key string) error {
		if !strings.HasPrefix(key, "condition:") || !c.exists(key) {
			return fail("%s: %q is not a condition", where, key)
		}
		return nil
	}

	seen := map[string]bool{}
	for _, in := range f.Traps {
		key := "trap:" + in.Key
		at := func(s string) string { return key + " " + s }
		switch {
		case in.Key == "" || strings.ContainsAny(in.Key, " :") || seen[key]:
			return fail("trap key %q is empty, has a space or colon, or is repeated", in.Key)
		case in.Kind != TrapMechanical && in.Kind != TrapMagic:
			return fail("%s: kind must be mechanical or magic", key)
		case strings.TrimSpace(in.DescriptionPT) == "":
			return fail("%s has no description_pt", key)
		case in.NoticeDC < 0 || in.NoticeDC > 30 || in.FindDC < 1 || in.FindDC > 30:
			return fail("%s: the DCs must be 1 to 30 (notice_dc may be left out)", key)
		case in.Trigger != TrapTriggerEnter && in.Trigger != TrapTriggerManual:
			return fail("%s: trigger must be enter or manual", key)
		case in.AreaSize < 1 || in.AreaSize > 4:
			return fail("%s: area_size must be 1 to 4", key)
		case in.Targets != TrapTargetsArea && in.Targets != TrapTargetsManual:
			return fail("%s: targets must be area or manual", key)
		case in.FallFt < 0 || in.FallFt%10 != 0 || in.FallFt > 200:
			return fail("%s: fall_ft must be a multiple of 10, up to 200", key)
		}
		seen[key] = true
		p := TrapPreset{
			Key: key, NamePT: c.namesPT[key], Kind: in.Kind, DescriptionPT: in.DescriptionPT, NoticeDC: in.NoticeDC, FindDC: in.FindDC, FindDCIsOurs: in.FindDCOurs,
			Trigger: in.Trigger, AreaSize: in.AreaSize, Targets: in.Targets, FallFt: in.FallFt,
		}
		if p.NamePT == "" {
			return fail("%s has no Portuguese name in effects/names_pt.json", key)
		}
		var err error
		if p.Damage, err = damages(at("damage"), in.Damage); err != nil {
			return err
		}
		if in.FallFt > 0 {
			want := FallDice(in.FallFt)
			if len(p.Damage) == 0 || p.Damage[0].Dice != want || p.Damage[0].Type != "damage-type:bludgeoning" {
				return fail("%s: a pit's first damage is the fall, %dd6 bludgeoning for %d ft", key, want.Count, in.FallFt)
			}
		}
		for i, cond := range in.Conditions {
			if err := condition(at(fmt.Sprintf("conditions[%d]", i)), cond.Condition); err != nil {
				return err
			}
			p.Conditions = append(p.Conditions, TrapCondition{Condition: cond.Condition, DurationPT: cond.DurationPT})
		}
		if a := in.Attack; a != nil {
			if a.Bonus < 0 || a.Bonus > 20 || a.Count < 1 || a.Count > 10 {
				return fail("%s: an attack needs a bonus of 0 to 20 and 1 to 10 attacks", key)
			}
			d, err := damage(at("attack.damage"), a.Damage)
			if err != nil {
				return err
			}
			p.Attack = &TrapAttack{Bonus: a.Bonus, Count: a.Count, Damage: d}
		}
		if s := in.Save; s != nil {
			ab, ok := ability(s.Ability)
			if !ok {
				return fail("%s: a saving throw needs an ability (str, dex, con, int, wis or cha), got %q", key, s.Ability)
			}
			if s.DC < 1 || s.DC > 30 {
				return fail("%s: the saving throw's DC must be 1 to 30", key)
			}
			if s.AppliesTo != TrapSaveCaught && s.AppliesTo != TrapSaveHit {
				return fail("%s: save.applies_to must be caught or hit", key)
			}
			if s.AppliesTo == TrapSaveHit && in.Attack == nil {
				return fail("%s: a save that applies to the creatures hit needs an attack", key)
			}
			if s.OnPass != TrapPassHalf && s.OnPass != TrapPassNone {
				return fail("%s: save.on_pass must be half or none", key)
			}
			fl, err := damages(at("save.on_fail.damage"), s.OnFail.Damage)
			if err != nil {
				return err
			}
			if len(fl) == 0 && s.OnFail.Condition == "" {
				return fail("%s: a failed save must do damage or give a condition", key)
			}
			if s.OnFail.Condition != "" {
				if err := condition(at("save.on_fail"), s.OnFail.Condition); err != nil {
					return err
				}
			}
			if s.OnFail.Condition == "" && s.OnFail.DurationPT != "" {
				return fail("%s: save.on_fail has a duration but no condition", key)
			}
			if s.OnPass == TrapPassHalf && len(fl) == 0 {
				return fail("%s: half damage on a pass needs damage on a fail", key)
			}
			p.Save = &TrapSave{
				Ability: ab, DC: s.DC, AppliesTo: s.AppliesTo, OnPass: s.OnPass,
				OnFail: TrapOnFail{Damage: fl, Condition: s.OnFail.Condition, DurationPT: s.OnFail.DurationPT},
			}
		}
		c.traps.presets = append(c.traps.presets, p)
	}
	// Every Portuguese name must be of a preset: a renamed or removed trap
	// leaves no stray name behind.
	for k := range c.namesPT {
		if strings.HasPrefix(k, "trap:") && !seen[k] {
			return fail("effects/names_pt.json names %q, which is not in traps.json", k)
		}
	}
	return nil
}

// clone is a copy of the preset that shares nothing with it.
func (p TrapPreset) clone() TrapPreset {
	p.Damage = slices.Clone(p.Damage)
	p.Conditions = slices.Clone(p.Conditions)
	if p.Attack != nil {
		a := *p.Attack
		p.Attack = &a
	}
	if p.Save != nil {
		s := *p.Save
		s.OnFail.Damage = slices.Clone(s.OnFail.Damage)
		p.Save = &s
	}
	return p
}

// TrapPresets returns the SRD's sample traps, in the order of
// effects/traps.json, for the master's form. The caller gets a deep copy.
func (c *Content) TrapPresets() []TrapPreset {
	out := make([]TrapPreset, len(c.c.traps.presets))
	for i, p := range c.c.traps.presets {
		out[i] = p.clone()
	}
	return out
}

// TrapPreset returns a deep copy of the preset with a key such as
// "trap:poison-needle".
func (c *Content) TrapPreset(key string) (TrapPreset, bool) {
	for _, p := range c.c.traps.presets {
		if p.Key == key {
			return p.clone(), true
		}
	}
	return TrapPreset{}, false
}

// TrapSeverities returns the SRD's three levels of trap severity (Revés,
// Perigosa, Mortal) with the save DCs and attack bonuses that suit them: the
// hint the master's form shows.
func (c *Content) TrapSeverities() []TrapSeverity {
	return slices.Clone(c.c.traps.severities)
}

// TrapDamageByLevel returns the SRD's damage that suits each severity at four
// ranges of character levels.
func (c *Content) TrapDamageByLevel() []TrapDamageRow {
	return slices.Clone(c.c.traps.damageRows)
}
