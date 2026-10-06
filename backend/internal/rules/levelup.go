package rules

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// The guided level-up (MR-040, RN-01, RN-12). Until the full level-up
// exists, a character that can level up may change its locked sheet, but
// only to add what the next level of one of its classes gives. Three pure
// functions say what that is:
//
//   - LevelUpOptions: what the next level of a class gives, for the screens;
//   - ApplyLevelUp: the Build that the player's choices make;
//   - CheckLevelUp: refuses every difference between the two Builds that the
//     level does not allow.
//
// Like the rest of the package they use no database, clock or randomness:
// the hit die is rolled elsewhere and passed in.

// Reasons of a LevelUpError. They are stable codes: the app maps each one to
// its own Portuguese copy, so nobody parses Message.
const (
	// LevelUpReasonClass: not exactly one class of the character, one level up
	// (a new class, which is multiclassing, stays with the master's editor).
	LevelUpReasonClass = "class"
	// LevelUpReasonMaxLevel: the character is already level 20.
	LevelUpReasonMaxLevel = "max_level"
	// LevelUpReasonLocked: a field the level does not change (name, race, base
	// scores, equipment...) is different. Field says which.
	LevelUpReasonLocked = "locked_field"
	// LevelUpReasonAbilityNotDue: an ability increase at a level that has none.
	LevelUpReasonAbilityNotDue = "ability_not_due"
	// LevelUpReasonAbilityShape: the increase is not +2 in one ability or +1 in two.
	LevelUpReasonAbilityShape = "ability_shape"
	// LevelUpReasonAbilityAbove20: the increase takes an ability above 20.
	LevelUpReasonAbilityAbove20 = "ability_above_20"
	// LevelUpReasonHitPoints: the new hit points entry is missing, outside 1 to
	// the hit die, or the earlier levels changed.
	LevelUpReasonHitPoints = "hit_points"
	// LevelUpReasonSubclass: a subclass that is not due, or is due and is missing.
	LevelUpReasonSubclass = "subclass"
	// LevelUpReasonCantrips: not exactly the new cantrips the level gives, or one
	// was removed.
	LevelUpReasonCantrips = "cantrips"
	// LevelUpReasonSpells: not exactly the new known (or spellbook) spells the
	// level gives, or one was removed.
	LevelUpReasonSpells = "spells"
	// LevelUpReasonPrepared: a prepared spell was removed, or one was added by a
	// class that does not prepare.
	LevelUpReasonPrepared = "prepared"
	// LevelUpReasonFeatureChoice: not exactly the options the new features offer.
	LevelUpReasonFeatureChoice = "feature_choice"
	// LevelUpReasonSkills: not exactly the skills the new level lets the player
	// choose.
	LevelUpReasonSkills = "skills"
	// LevelUpReasonExpertise: not exactly the expertise the new level gives.
	LevelUpReasonExpertise = "expertise"
	// LevelUpReasonSheetIssue: the new sheet has a problem the old one did not
	// (a spell off the class list or above the circle, too many prepared
	// spells...). Code is the Issue code and Field the Issue's field.
	LevelUpReasonSheetIssue = "sheet_issue"
)

// LevelUpError is what ApplyLevelUp, LevelUpOptions and CheckLevelUp return
// for something the level does not allow. Field names the sheet field with
// the CharacterSheet proto's field names ("full.base_scores.strength"), and
// Reason is one of the LevelUpReason* codes.
type LevelUpError struct {
	Field  string
	Reason string
	// Code is the Issue code, for LevelUpSheetIssue only.
	Code string
	// Message is English text for logs and tests, never for the app.
	Message string
}

func (e *LevelUpError) Error() string {
	return e.Field + ": " + e.Reason + ": " + e.Message
}

func refuse(field, reason, format string, args ...any) *LevelUpError {
	return &LevelUpError{Field: field, Reason: reason, Message: fmt.Sprintf(format, args...)}
}

// LevelUpOffer is what one class's next level gives, in data the screens
// can draw. Everything the player chooses is in the "to choose" part; the
// rest is automatic and only shown.
type LevelUpOffer struct {
	// Class is the class key and ClassNamePT its name. FromLevel and ToLevel
	// are its levels, and TotalFrom and TotalTo the character's.
	Class       string
	ClassNamePT string
	FromLevel   int
	ToLevel     int
	TotalFrom   int
	TotalTo     int

	// HitDie is the class's die (8 for a d8) and HitPointAverage the fixed
	// gain: half the die plus one, before the Constitution modifier.
	HitDie          int
	HitPointAverage int

	// To choose.

	// AbilityScoreImprovement says this level has one: +2 in one ability or
	// +1 in two, none above 20. The SRD 5.1 has no feats.
	AbilityScoreImprovement bool
	// SubclassDue says the class picks its subclass now, from Subclasses.
	SubclassDue bool
	Subclasses  []LevelUpSubclass
	// Cantrips is how many new cantrips to add, from the class's list.
	Cantrips int
	// Spells is how many new spells to add: to the spellbook (wizard) or to
	// the spells known (bard, ranger, sorcerer, warlock). SpellsKind says
	// which (PreparationSpellbook or PreparationKnown), "" for none.
	Spells     int
	SpellsKind string
	// SpellList is the class key whose list the new spells come from, and
	// MaxSpellLevel the highest circle they may have. Both are empty/zero
	// when the class casts nothing at the new level.
	SpellList     string
	MaxSpellLevel int
	// AnyClassSpells is how many of Spells may come from any class's list, not
	// only SpellList's: the Bard's Magical Secrets (levels 10, 14 and 18).
	// They must still be of a circle the class can cast.
	AnyClassSpells int
	// Prepares says the class prepares spells. PreparedMax is the maximum
	// now, and PreparedMaxAfter the one with the new level and the
	// abilities as they are; an ability increase can raise it, so the
	// screens ask PreviewLevelUp for the final number.
	Prepares         bool
	PreparedMax      int
	PreparedMaxAfter int
	// FeatureChoices are the options the features of the new level offer
	// (a fighting style, a metamagic), for the class's own features and,
	// when the subclass is already chosen, the subclass's.
	FeatureChoices []LevelUpFeatureChoice
	// SkillChoices and ExpertiseChoices are how many new skills and
	// expertise the new level lets the player pick (the Bard's Lore
	// college, the Rogue's Expertise).
	SkillChoices     int
	ExpertiseChoices int

	// Automatic.

	// ProficiencyBonusBefore and After; SlotsBefore and After are the
	// spell slots per circle (index 0 is the 1st circle); PactBefore and
	// After are the warlock's pact slots, nil for none.
	ProficiencyBonusBefore int
	ProficiencyBonusAfter  int
	SlotsBefore            [9]int
	SlotsAfter             [9]int
	PactBefore             *PactMagic
	PactAfter              *PactMagic
	// NewFeatures are the class features the level gives by name.
	NewFeatures []NamedKey
	// MasterAdds are the features of the level whose choice the guided level-up
	// does not cover (Mystic Arcanum, Spell Mastery, Signature Spells,
	// Additional Magical Secrets, favored enemies and terrains): the master
	// adds them in the sheet editor.
	MasterAdds []NamedKey
}

// LevelUpSubclass is a subclass the player may pick, with the options its
// features at the new level offer.
type LevelUpSubclass struct {
	Key, NamePT    string
	FeatureChoices []LevelUpFeatureChoice
	// What this subclass adds to the level's counts: cantrips (the Land
	// Druid's bonus cantrip), skills (the College of Lore's three) and
	// expertise. The top-level counts of the offer leave them out until the
	// subclass is chosen.
	Cantrips, SkillChoices, ExpertiseChoices int
	// Spells is how many spells a third caster's subclass adds (known ones, with
	// SpellsKind PreparationKnown); the table's subclass that casts is the only one.
	Spells     int
	SpellsKind string
	// SpellList is the class whose list those spells come from, and
	// MaxSpellLevel the highest circle they may have (zero values for a subclass
	// that does not cast).
	SpellList     string
	MaxSpellLevel int
	// Prepares says a third caster's subclass prepares its spells, and
	// PreparedMaxAfter is how many it may prepare at the new level once chosen.
	Prepares         bool
	PreparedMaxAfter int
	// Archived says the table retired this subclass (see RaceEntry.Archived):
	// the screen does not offer it as a new choice.
	Archived bool
	// Off says the master switched this subclass off for the players (RN-23): its own
	// switch. The offer is for a character that has the class, so an off class does
	// not hide it.
	Off bool
}

// LevelUpFeatureChoice is "choose Choose of Options" for a feature gained
// at the new level. Subclass is the key of the subclass the feature belongs
// to, or empty for a class feature.
type LevelUpFeatureChoice struct {
	Feature  NamedKey
	Subclass string
	Choose   int
	Options  []NamedKey
}

// LevelUpChoices is what the player chose for one level up, in content
// keys. The lists hold only what is new, never the whole sheet.
type LevelUpChoices struct {
	// Class is the class key that gains the level.
	Class string
	// AbilityIncrease is +2 in one ability or +1 in two; empty for none.
	AbilityIncrease map[Ability]int
	// Subclass is the subclass key, when one is due.
	Subclass string
	// The new cantrips, known (or spellbook) spells and prepared spells.
	Cantrips []string
	Spells   []string
	Prepared []string
	// FeatureChoices are the new options, SkillProficiencies the new skills
	// and Expertise the new expertise.
	FeatureChoices     []string
	SkillProficiencies []string
	Expertise          []string
	// HitPoints is the gain of this level.
	HitPoints LevelUpHitPoints
}

// LevelUpHitPoints is the hit points of the new level: the fixed average,
// or a die result (rolled in the app or typed from a physical die, which
// the rules cannot tell apart).
type LevelUpHitPoints struct {
	Average bool
	// Roll is the die result, 1 to the hit die; ignored with Average.
	Roll int
}

// clone copies a Build deeply, so the level-up never changes the caller's.
func (b Build) clone() Build {
	b.BaseScores = maps.Clone(b.BaseScores)
	b.ExtraAbilityBonuses = maps.Clone(b.ExtraAbilityBonuses)
	b.Classes = slices.Clone(b.Classes)
	b.CustomBackgroundSkills = slices.Clone(b.CustomBackgroundSkills)
	b.CustomBackgroundProficiencies = slices.Clone(b.CustomBackgroundProficiencies)
	b.SkillProficiencies = slices.Clone(b.SkillProficiencies)
	b.Expertise = slices.Clone(b.Expertise)
	b.Weapons = slices.Clone(b.Weapons)
	b.Cantrips = slices.Clone(b.Cantrips)
	b.SpellsKnown = slices.Clone(b.SpellsKnown)
	b.SpellsPrepared = slices.Clone(b.SpellsPrepared)
	b.FeatureChoices = slices.Clone(b.FeatureChoices)
	b.HitPoints.Rolls = slices.Clone(b.HitPoints.Rolls)
	return b
}

func (b Build) totalLevel() int {
	n := 0
	for _, cl := range b.Classes {
		n += cl.Level
	}
	return n
}

// levelUpClass finds the class that gains the level, and refuses one the
// character does not have, or a character at level 20.
func levelUpClass(b Build, classKey string, c *content) (int, *LevelUpError) {
	idx := slices.IndexFunc(b.Classes, func(cl ClassLevel) bool { return cl.Class == classKey })
	if _, known := c.classes[classKey]; idx < 0 || !known {
		return 0, refuse("full.classes", LevelUpReasonClass, "the character has no such class: a new class is multiclassing")
	}
	if b.totalLevel() >= MaxLevel || b.Classes[idx].Level >= MaxLevel {
		return 0, refuse(fmt.Sprintf("full.classes[%d].level", idx), LevelUpReasonMaxLevel, "the character is already level %d", MaxLevel)
	}
	return idx, nil
}

// LevelUpOptions says what the next level of class classKey gives a
// character built as before. It refuses a class the character does not have
// (LevelUpClass) and a character at level 20 (LevelUpMaxLevel).
func LevelUpOptions(before Build, classKey string, c *Content) (LevelUpOffer, error) {
	o, err := levelUpOptions(before, classKey, c.c)
	if err != nil {
		return LevelUpOffer{}, err
	}
	return o, nil
}

func levelUpOptions(b Build, classKey string, c *content) (LevelUpOffer, *LevelUpError) {
	idx, lerr := levelUpClass(b, classKey, c)
	if lerr != nil {
		return LevelUpOffer{}, lerr
	}
	o, _ := levelUpOptionsWith(b, idx, c, c.subclasses[b.Classes[idx].Subclass])
	return o, nil
}

// levelUpOptionsWith is levelUpOptions as if the class had subclass sub at the
// new level: the counts of the top level then include what that subclass
// gives. CheckLevelUp calls it with the subclass the new sheet chose, so the
// options and the check read the same numbers; the options themselves call it
// with the subclass the character already has, and give each candidate
// subclass its own part (LevelUpSubclass).
func levelUpOptionsWith(b Build, idx int, c *content, sub *srd51.Subclass) (LevelUpOffer, *LevelUpError) {
	cl := b.Classes[idx]
	classKey := cl.Class
	class := c.classes[classKey]
	newLevel := cl.Level + 1

	// The level with nothing chosen yet: what follows from the level alone.
	bare := b.clone()
	bare.Classes[idx].Level = newLevel
	if sub != nil && bare.Classes[idx].Subclass == "" {
		// A third caster casts through its subclass: the level with the subclass
		// chosen has its casting numbers.
		bare.Classes[idx].Subclass = sub.Key
	}
	dBefore, dBare := derive(b, c), derive(bare, c)

	o := LevelUpOffer{
		Class: classKey, ClassNamePT: c.namePT(classKey),
		FromLevel: cl.Level, ToLevel: newLevel, TotalFrom: b.totalLevel(), TotalTo: b.totalLevel() + 1,
		HitDie: class.HitDie, HitPointAverage: class.HitDie/2 + 1,
		ProficiencyBonusBefore: dBefore.ProficiencyBonus, ProficiencyBonusAfter: dBare.ProficiencyBonus,
		PactBefore: dBefore.PactMagic, PactAfter: dBare.PactMagic,
	}
	copy(o.SlotsBefore[:], dBefore.SpellSlots)
	copy(o.SlotsAfter[:], dBare.SpellSlots)

	row := c.classLevels[classKey][newLevel-1]
	o.AbilityScoreImprovement = isASILevel(row)

	// Features the level adds, by name; and the ones the guided flow does not
	// cover, which the master adds in the editor.
	had := map[string]bool{}
	for _, f := range dBefore.Features {
		had[f.Key] = true
	}
	for _, f := range dBare.Features {
		if !had[f.Key] && f.Source == classKey {
			o.NewFeatures = append(o.NewFeatures, NamedKey{Key: f.Key, NamePT: f.NamePT})
		}
	}
	classFeatures, subFeatures := c.levelFeatures(classKey, sub, newLevel)
	for _, fk := range slices.Concat(classFeatures, subFeatures) {
		if masterAdds[fk] {
			o.MasterAdds = append(o.MasterAdds, NamedKey{Key: fk, NamePT: c.namePT(fk)})
		}
		if strings.HasPrefix(fk, "feature:magical-secrets-") && !isTableKey(fk) {
			o.AnyClassSpells = 2
		}
	}

	// What the class gives, and the subclass when it is already chosen.
	gains := c.classGains(classKey, newLevel)
	if sub != nil {
		gains = gains.plus(c.subclassGains(sub, newLevel))
	}
	o.FeatureChoices = c.namedChoices(b, gains.choices)
	o.SkillChoices, o.ExpertiseChoices = gains.skills, gains.expertise

	// A subclass that is due: each candidate's own part.
	if cl.Subclass == "" && cl.CustomSubclassName == "" && class.SubclassLevel == newLevel {
		o.SubclassDue = true
		for _, key := range class.Subclasses {
			if s, ok := c.subclasses[key]; ok {
				g := c.subclassGains(s, newLevel)
				ls := LevelUpSubclass{
					Key: key, NamePT: c.namePT(key), Archived: c.archived[key], Off: c.off[key], FeatureChoices: c.namedChoices(b, g.choices),
					Cantrips: g.cantrips, SkillChoices: g.skills, ExpertiseChoices: g.expertise,
				}
				if _, third := c.subCasting[key]; third && sub == nil {
					// A third caster's table starts with the subclass: what it adds to
					// the level's counts is the difference with the class alone.
					with, _ := levelUpOptionsWith(b, idx, c, s)
					ls.Cantrips += max(with.Cantrips-o.Cantrips, 0)
					ls.Spells, ls.SpellsKind = max(with.Spells-o.Spells, 0), with.SpellsKind
					ls.SpellList, ls.MaxSpellLevel = with.SpellList, with.MaxSpellLevel
					ls.Prepares, ls.PreparedMaxAfter = with.Prepares, with.PreparedMaxAfter
				}
				o.Subclasses = append(o.Subclasses, ls)
			}
		}
		slices.SortFunc(o.Subclasses, func(a, b LevelUpSubclass) int { return comparePT(a.NamePT, b.NamePT) })
	}

	// Spells: the difference between the class's rows (or the spellbook's
	// two per level).
	before, after := spellcastingOf(dBefore, classKey), spellcastingOf(dBare, classKey)
	if after != nil {
		cast, _ := c.castingFor(classKey, sub)
		o.SpellList, o.MaxSpellLevel = cast.list, after.MaxSpellLevel
		oldCantrips, oldKnown := 0, 0
		if before != nil {
			oldCantrips, oldKnown = before.CantripsKnown, before.SpellsKnownMax
		}
		o.Cantrips = max(after.CantripsKnown-oldCantrips, 0) + gains.cantrips
		switch preparation(cast.effect) {
		case PreparationSpellbook:
			if before != nil {
				o.Spells, o.SpellsKind = 2, PreparationSpellbook
			}
		case PreparationKnown:
			o.Spells, o.SpellsKind = max(after.SpellsKnownMax-oldKnown, 0), PreparationKnown
		}
		o.Prepares = after.PreparesSpells
		if before != nil {
			o.PreparedMax = before.PreparedMax
		}
		o.PreparedMaxAfter = after.PreparedMax
	} else {
		o.Cantrips = gains.cantrips
	}
	o.AnyClassSpells = min(o.AnyClassSpells, o.Spells)
	return o, nil
}

// spellcastingOf finds a class's casting numbers, or nil.
func spellcastingOf(d Derived, classKey string) *Spellcasting {
	for i := range d.Spellcasting {
		if d.Spellcasting[i].Class == classKey {
			return &d.Spellcasting[i]
		}
	}
	return nil
}

// isASILevel says whether a class table row has an Ability Score
// Improvement.
func isASILevel(row *srd51.Level) bool {
	return row != nil && slices.ContainsFunc(row.Features, func(k string) bool {
		return strings.Contains(k, "-ability-score-improvement-")
	})
}

// masterAdds are the features that ask the player for a choice the sheet has
// no field for, or that the engine has no data for. The guided level-up lists
// them (LevelUpOffer.MasterAdds) and the master adds them in the editor
// (RN-12): the Warlock's Mystic Arcanum, the Wizard's Spell Mastery and
// Signature Spells, the College of Lore's Additional Magical Secrets, and the
// Ranger's favored enemies and terrains.
var masterAdds = map[string]bool{
	"feature:mystic-arcanum-6th-level":         true,
	"feature:mystic-arcanum-7th-level":         true,
	"feature:mystic-arcanum-8th-level":         true,
	"feature:mystic-arcanum-9th-level":         true,
	"feature:spell-mastery":                    true,
	"feature:signature-spell":                  true,
	"feature:additional-magical-secrets":       true,
	"feature:favored-enemy-2-types":            true,
	"feature:favored-enemy-3-enemies":          true,
	"feature:natural-explorer-2-terrain-types": true,
	"feature:natural-explorer-3-terrain-types": true,
}

// invocationsFeature is the Warlock's Eldritch Invocations. How many the
// character knows comes from the class table (invocations_known), level by
// level, not from the feature, which only exists at level 2.
const invocationsFeature = "feature:eldritch-invocations"

// levelFeatures lists the keys of the features that a class, and a subclass
// when it has one, gain at a class level. Class features come first.
func (c *content) levelFeatures(classKey string, sub *srd51.Subclass, level int) (classFeatures, subclassFeatures []string) {
	if rows := c.classLevels[classKey]; level >= 1 && level <= len(rows) {
		classFeatures = rows[level-1].Features
	}
	if sub != nil {
		if row := c.subclassLevels[sub.Key][level]; row != nil {
			subclassFeatures = row.Features
		}
	}
	return classFeatures, subclassFeatures
}

// dueChoice is "choose choose of options" for a feature gained at a level.
type dueChoice struct {
	feature  string
	subclass string
	choose   int
	options  []string
}

// levelGains is what a set of features asks the player to choose.
type levelGains struct {
	choices   []dueChoice
	skills    int
	expertise int
	cantrips  int
}

func (g levelGains) plus(o levelGains) levelGains {
	return levelGains{
		choices: slices.Concat(g.choices, o.choices), skills: g.skills + o.skills,
		expertise: g.expertise + o.expertise, cantrips: g.cantrips + o.cantrips,
	}
}

// featureGains reads the choices of the features with these keys: their
// options (the feature's options_choose or, for one that does not say how
// many, the hand-written choice effect's count; a feature with neither is not
// enforced), the skills and expertise they give, and the extra cantrips.
func (c *content) featureGains(keys []string, subclass string) levelGains {
	var g levelGains
	for _, fk := range keys {
		f := c.features[fk]
		if f == nil {
			continue
		}
		g.expertise += f.ExpertiseChoices
		choose := f.OptionsChoose
		for _, e := range c.effects[fk] {
			if e.Type != "choice" {
				continue
			}
			switch e.Choice {
			case "feature":
				if choose == 0 {
					choose = e.Count
				}
			case "expertise":
				// An SRD feature says its expertise in the data (ExpertiseChoices); the
				// table's say it in a choice effect.
				if isTableKey(fk) {
					g.expertise += e.Count
				}
			case "skill":
				g.skills += e.Count
			case "cantrip":
				g.cantrips += e.Count
			}
		}
		if fk != invocationsFeature && len(f.Options) > 0 && choose > 0 {
			g.choices = append(g.choices, dueChoice{feature: fk, subclass: subclass, choose: choose, options: f.Options})
		}
	}
	return g
}

// classGains is featureGains for a class level, plus the invocations the
// class table adds at that level.
func (c *content) classGains(classKey string, level int) levelGains {
	classFeatures, _ := c.levelFeatures(classKey, nil, level)
	g := c.featureGains(classFeatures, "")
	if rows := c.classLevels[classKey]; level >= 1 && level <= len(rows) {
		now, before := invocationsKnown(rows[level-1]), 0
		if level > 1 {
			before = invocationsKnown(rows[level-2])
		}
		if f := c.features[invocationsFeature]; f != nil && now > before && len(f.Options) > 0 {
			g.choices = append(g.choices, dueChoice{feature: invocationsFeature, choose: now - before, options: f.Options})
		}
	}
	return g
}

// subclassGains is featureGains for a subclass's features at a class level.
func (c *content) subclassGains(sub *srd51.Subclass, level int) levelGains {
	_, subFeatures := c.levelFeatures("", sub, level)
	return c.featureGains(subFeatures, sub.Key)
}

// invocationsKnown reads the class table's invocations_known column.
func invocationsKnown(row *srd51.Level) int {
	var cs struct {
		Known int `json:"invocations_known"`
	}
	if len(row.ClassSpecific) == 0 || json.Unmarshal(row.ClassSpecific, &cs) != nil {
		return 0
	}
	return cs.Known
}

// namedChoices is the due choices with names, leaving out the options the
// character already has.
func (c *content) namedChoices(b Build, due []dueChoice) []LevelUpFeatureChoice {
	var out []LevelUpFeatureChoice
	for _, d := range due {
		fc := LevelUpFeatureChoice{Feature: NamedKey{Key: d.feature, NamePT: c.namePT(d.feature)}, Subclass: d.subclass, Choose: d.choose}
		for _, o := range d.options {
			if !slices.Contains(b.FeatureChoices, o) {
				fc.Options = append(fc.Options, NamedKey{Key: o, NamePT: c.namePT(o)})
			}
		}
		out = append(out, fc)
	}
	return out
}

// rollDice lists the hit die of each entry of HitPoints.Rolls (character
// levels 2, 3...), in Build.Classes order.
func rollDice(b Build, c *content) []int {
	var dice []int
	first := true
	for _, cl := range b.Classes {
		class := c.classes[cl.Class]
		for l := 0; l < cl.Level; l++ {
			if first {
				first = false
				continue
			}
			die := 0
			if class != nil {
				die = class.HitDie
			}
			dice = append(dice, die)
		}
	}
	return dice
}

// oldRolls are the rolls of the earlier levels as the Build stands: its own
// when it rolls (a missing one is the average), the averages otherwise.
func oldRolls(b Build, c *content) []int {
	dice := rollDice(b, c)
	out := make([]int, len(dice))
	for i, die := range dice {
		out[i] = die/2 + 1
		if b.HitPoints.Method == HitPointsRolled && i < len(b.HitPoints.Rolls) {
			out[i] = b.HitPoints.Rolls[i]
		}
	}
	return out
}

// rollIndex is where the new level's roll goes in HitPoints.Rolls: after
// every earlier level of the class that gains it and of the classes before it.
func rollIndex(b Build, classIdx int) int {
	n := 0
	for _, cl := range b.Classes[:classIdx+1] {
		n += cl.Level
	}
	return n - 1
}

// ApplyLevelUp returns the Build that the choices make from before: the
// class one level higher, and the choices added to it. It never judges the
// choices (CheckLevelUp does); it only refuses a class the character does
// not have (LevelUpClass) and level 20 (LevelUpMaxLevel).
//
// The hit points follow the sheet's method. With the fixed average the Build
// stays as it is; a rolled value switches a fixed sheet to rolled, filling
// the earlier levels with their averages so nothing else changes, and an
// average on a rolled sheet adds the average as a roll.
func ApplyLevelUp(before Build, ch LevelUpChoices, c *Content) (Build, error) {
	idx, lerr := levelUpClass(before, ch.Class, c.c)
	if lerr != nil {
		return Build{}, lerr
	}
	after := before.clone()
	cl := &after.Classes[idx]
	cl.Level++
	if ch.Subclass != "" {
		cl.Subclass = ch.Subclass
	}
	for a, v := range ch.AbilityIncrease {
		if after.ExtraAbilityBonuses == nil {
			after.ExtraAbilityBonuses = map[Ability]int{}
		}
		after.ExtraAbilityBonuses[a] += v
	}
	after.Cantrips = append(after.Cantrips, ch.Cantrips...)
	after.SpellsKnown = append(after.SpellsKnown, ch.Spells...)
	after.SpellsPrepared = append(after.SpellsPrepared, ch.Prepared...)
	after.FeatureChoices = append(after.FeatureChoices, ch.FeatureChoices...)
	after.SkillProficiencies = append(after.SkillProficiencies, ch.SkillProficiencies...)
	after.Expertise = append(after.Expertise, ch.Expertise...)

	die := c.c.classes[ch.Class].HitDie
	value := ch.HitPoints.Roll
	if ch.HitPoints.Average {
		value = die/2 + 1
	}
	if before.HitPoints.Method == HitPointsFixed && ch.HitPoints.Average {
		return after, nil // nothing to store: the average is what Fixed means
	}
	rolls := oldRolls(before, c.c)
	i := rollIndex(before, idx)
	after.HitPoints = HitPoints{Method: HitPointsRolled, Rolls: slices.Insert(rolls, min(i, len(rolls)), value)}
	return after, nil
}

// CheckLevelUp refuses any difference between before and after that one
// level of one class does not allow, with the first refusal's field and
// reason. The only allowed differences are:
//
//   - exactly one existing class gains exactly one level;
//   - the ability increase, only at an Ability Score Improvement level: +2 in
//     one ability or +1 in two, none above 20, as extra_ability_bonuses;
//   - the new level's hit points, in 1 to the hit die, with the earlier
//     levels unchanged (a fixed sheet that rolls takes their averages);
//   - exactly the new cantrips, known or spellbook spells the level gives, and
//     any new prepared spells, with none removed;
//   - the subclass when it is due, the options the new features offer, and the
//     skills and expertise the new level lets the player choose.
//
// Anything else differs is refused: name, race, base scores, equipment,
// weapons, the other classes... (RN-01: the rest stays locked). A sheet that
// ends with a problem the old one did not have (a spell off the class list,
// too many prepared spells) is refused as LevelUpSheetIssue.
func CheckLevelUp(before, after Build, c *Content) error {
	if err := checkLevelUp(before, after, c.c); err != nil {
		return err
	}
	return nil
}

func checkLevelUp(before, after Build, c *content) *LevelUpError {
	// One class, one level.
	idx, lerr := levelUpClassOf(before, after, c)
	if lerr != nil {
		return lerr
	}
	classKey := before.Classes[idx].Class
	class := c.classes[classKey]
	newLevel := before.Classes[idx].Level + 1

	if lerr := checkLocked(before, after); lerr != nil {
		return lerr
	}
	dBefore, dAfter := derive(before, c), derive(after, c)
	if lerr := checkAbilities(before, after, dAfter, isASILevel(c.classLevels[classKey][newLevel-1])); lerr != nil {
		return lerr
	}
	if lerr := checkHitPoints(before, after, idx, class.HitDie, c); lerr != nil {
		return lerr
	}

	// The subclass, then what depends on it.
	bcl, acl := before.Classes[idx], after.Classes[idx]
	due := bcl.Subclass == "" && bcl.CustomSubclassName == "" && class.SubclassLevel == newLevel
	field := fmt.Sprintf("full.classes[%d]", idx)
	switch {
	case acl.CustomSubclassName != bcl.CustomSubclassName:
		return refuse(field+".custom_subclass_name", LevelUpReasonSubclass, "a custom subclass is the master's to set")
	case due && acl.Subclass == "":
		return refuse(field+".subclass_key", LevelUpReasonSubclass, "the subclass is chosen at this level")
	case due && c.subclasses[acl.Subclass] == nil, due && c.subclasses[acl.Subclass].Class != classKey:
		return refuse(field+".subclass_key", LevelUpReasonSubclass, "not a subclass of the class")
	case !due && acl.Subclass != bcl.Subclass:
		return refuse(field+".subclass_key", LevelUpReasonSubclass, "the subclass is not chosen at this level")
	}
	sub := c.subclasses[acl.Subclass]

	if lerr := checkDuplicates(after); lerr != nil {
		return lerr
	}
	// The same numbers the options give: the class's, and the chosen
	// subclass's, which are not in the options' top level while it is due.
	offer, _ := levelUpOptionsWith(before, idx, c, sub)
	offOwnList, lerr := checkSpells(before, after, offer, offer.SpellList, c)
	if lerr != nil {
		return lerr
	}
	gains := c.classGains(classKey, newLevel)
	if sub != nil {
		gains = gains.plus(c.subclassGains(sub, newLevel))
	}

	// Options, skills and expertise.
	newChoices, removed := added(before.FeatureChoices, after.FeatureChoices)
	if removed {
		return refuse("full.feature_choice_keys", LevelUpReasonFeatureChoice, "an option was removed")
	}
	left := slices.Clone(newChoices)
	for _, d := range gains.choices {
		taken := 0
		left = slices.DeleteFunc(left, func(k string) bool {
			if taken < d.choose && slices.Contains(d.options, k) {
				taken++
				return true
			}
			return false
		})
		if taken != d.choose {
			return refuse("full.feature_choice_keys", LevelUpReasonFeatureChoice, "%s needs %d options, got %d", d.feature, d.choose, taken)
		}
	}
	if len(left) > 0 {
		return refuse("full.feature_choice_keys", LevelUpReasonFeatureChoice, "an option the new features do not offer")
	}
	wantSkills, wantExpertise := gains.skills, gains.expertise
	if got, removed := added(before.SkillProficiencies, after.SkillProficiencies); removed || len(got) != wantSkills {
		return refuse("full.skill_proficiency_keys", LevelUpReasonSkills, "the new level lets the player choose %d skills", wantSkills)
	}
	if got, removed := added(before.Expertise, after.Expertise); removed || len(got) != wantExpertise {
		return refuse("full.expertise_skill_keys", LevelUpReasonExpertise, "the new level gives %d expertise", wantExpertise)
	}

	// The sheet must not end up with a problem it did not have.
	had := map[string]bool{}
	for _, is := range dBefore.Issues {
		had[is.Code+"|"+is.Field+"|"+is.Message] = true
	}
	for _, is := range dAfter.Issues {
		// Magical Secrets spells are off the class list on purpose.
		if is.Code == IssueSpellNotOnList && offOwnList[is.Field] {
			continue
		}
		if !had[is.Code+"|"+is.Field+"|"+is.Message] {
			return &LevelUpError{Field: is.Field, Reason: LevelUpReasonSheetIssue, Code: is.Code, Message: is.Message}
		}
	}
	return nil
}

// levelUpClassOf finds the class that gained one level, and refuses any
// other change to the classes: another class, a class added or removed.
func levelUpClassOf(before, after Build, c *content) (int, *LevelUpError) {
	const field = "full.classes"
	if len(before.Classes) != len(after.Classes) {
		return 0, refuse(field, LevelUpReasonClass, "a class was added or removed")
	}
	idx := -1
	for i := range before.Classes {
		b, a := before.Classes[i], after.Classes[i]
		switch {
		case a.Class != b.Class:
			return 0, refuse(fmt.Sprintf("%s[%d].class_key", field, i), LevelUpReasonClass, "a class changed")
		case a.Level == b.Level:
		case a.Level == b.Level+1 && idx < 0:
			idx = i
		default:
			return 0, refuse(fmt.Sprintf("%s[%d].level", field, i), LevelUpReasonClass, "exactly one class gains exactly one level")
		}
	}
	if idx < 0 {
		return 0, refuse(field, LevelUpReasonClass, "no class gained a level")
	}
	if _, lerr := levelUpClass(before, before.Classes[idx].Class, c); lerr != nil {
		return 0, lerr
	}
	return idx, nil
}

// checkLocked refuses a difference in anything the level does not touch.
func checkLocked(before, after Build) *LevelUpError {
	locked := func(field string) *LevelUpError {
		return refuse(field, LevelUpReasonLocked, "the sheet is locked: only the master changes this")
	}
	for _, a := range AllAbilities() {
		if before.BaseScores[a] != after.BaseScores[a] {
			return locked("full.base_scores." + protoAbility[a])
		}
	}
	switch {
	case before.Race != after.Race:
		return locked("full.race_key")
	case before.Subrace != after.Subrace:
		return locked("full.subrace_key")
	case before.Background != after.Background:
		return locked("full.background_key")
	case before.CustomBackgroundName != after.CustomBackgroundName,
		!slices.Equal(before.CustomBackgroundSkills, after.CustomBackgroundSkills),
		!slices.Equal(before.CustomBackgroundProficiencies, after.CustomBackgroundProficiencies),
		before.CustomBackgroundFeatureName != after.CustomBackgroundFeatureName,
		before.CustomBackgroundFeature != after.CustomBackgroundFeature,
		before.CustomBackgroundEquipment != after.CustomBackgroundEquipment:
		return locked("full.custom_background")
	case before.Armor != after.Armor:
		return locked("full.armor_key")
	case before.Shield != after.Shield:
		return locked("full.shield")
	case !slices.Equal(before.Weapons, after.Weapons):
		return locked("full.weapon_keys")
	}
	return nil
}

// added returns the entries of after that before does not have, and whether
// after dropped any entry of before.
func added(before, after []string) (fresh []string, removed bool) {
	for _, k := range before {
		if !slices.Contains(after, k) {
			removed = true
		}
	}
	for _, k := range after {
		if !slices.Contains(before, k) {
			fresh = append(fresh, k)
		}
	}
	return fresh, removed
}

// checkAbilities checks the ability increase: only at an ASI level, +2 in one
// ability or +1 in two, and none above 20.
func checkAbilities(before, after Build, dAfter Derived, asi bool) *LevelUpError {
	const field = "full.extra_ability_bonuses"
	var raised []Ability
	total := 0
	for _, a := range AllAbilities() {
		d := after.ExtraAbilityBonuses[a] - before.ExtraAbilityBonuses[a]
		switch {
		case d < 0 || d > 2:
			return refuse(field+"."+protoAbility[a], LevelUpReasonAbilityShape, "an ability goes up by 1 or 2 at most")
		case d > 0:
			raised = append(raised, a)
			total += d
		}
	}
	for a := range after.ExtraAbilityBonuses {
		if _, ok := abilityIndex[a]; !ok {
			return refuse(field, LevelUpReasonAbilityShape, "an unknown ability")
		}
	}
	if total == 0 {
		return nil
	}
	if !asi {
		return refuse(field, LevelUpReasonAbilityNotDue, "this level has no Ability Score Improvement")
	}
	if total != 2 {
		return refuse(field, LevelUpReasonAbilityShape, "+2 in one ability or +1 in two")
	}
	for _, a := range raised {
		for _, s := range dAfter.Abilities {
			if s.Ability == a && s.Score > 20 {
				return refuse(field+"."+protoAbility[a], LevelUpReasonAbilityAbove20, "an ability score cannot pass 20")
			}
		}
	}
	return nil
}

// checkHitPoints checks the hit points of the new level: the new entry is in
// 1 to the die, and the earlier levels are as they were.
func checkHitPoints(before, after Build, classIdx, die int, c *content) *LevelUpError {
	const field = "full.hit_points"
	if before.HitPoints.Method == HitPointsFixed && after.HitPoints.Method == HitPointsFixed {
		if !slices.Equal(before.HitPoints.Rolls, after.HitPoints.Rolls) {
			return refuse(field+".rolls", LevelUpReasonHitPoints, "the rolls of a fixed sheet do not change")
		}
		return nil
	}
	if after.HitPoints.Method != HitPointsRolled {
		return refuse(field+".method", LevelUpReasonHitPoints, "a sheet that rolls its hit points keeps rolling")
	}
	old := oldRolls(before, c)
	i := rollIndex(before, classIdx)
	rolls := after.HitPoints.Rolls
	if len(rolls) != len(old)+1 || i > len(old) {
		return refuse(field+".rolls", LevelUpReasonHitPoints, "one new roll for the new level")
	}
	if got := slices.Delete(slices.Clone(rolls), i, i+1); !slices.Equal(got, old) {
		return refuse(field+".rolls", LevelUpReasonHitPoints, "the earlier levels do not change")
	}
	if rolls[i] < 1 || rolls[i] > die {
		return refuse(fmt.Sprintf("%s.rolls[%d]", field, i), LevelUpReasonHitPoints, "the roll is 1 to %d", die)
	}
	return nil
}

// checkSpells checks the new cantrips, known or spellbook spells, and
// prepared spells against what the level gives. It also returns the sheet
// fields of the new known spells that are off the class's list, which the
// Bard's Magical Secrets allow (up to offer.AnyClassSpells of them).
func checkSpells(before, after Build, offer LevelUpOffer, list string, c *content) (map[string]bool, *LevelUpError) {
	newCantrips, removed := added(before.Cantrips, after.Cantrips)
	if removed || len(newCantrips) != offer.Cantrips {
		return nil, refuse("full.cantrip_keys", LevelUpReasonCantrips, "the level gives %d new cantrips, none removed", offer.Cantrips)
	}
	newKnown, removed := added(before.SpellsKnown, after.SpellsKnown)
	if removed || len(newKnown) != offer.Spells {
		return nil, refuse("full.known_spell_keys", LevelUpReasonSpells, "the level gives %d new spells, none removed", offer.Spells)
	}
	newPrepared, removed := added(before.SpellsPrepared, after.SpellsPrepared)
	if removed || (len(newPrepared) > 0 && !offer.Prepares) {
		return nil, refuse("full.prepared_spell_keys", LevelUpReasonPrepared, "prepared spells are only added by a class that prepares, none removed")
	}
	offList := map[string]bool{}
	for i, key := range after.SpellsKnown {
		if !slices.Contains(newKnown, key) {
			continue
		}
		if s := c.spells[key]; s != nil && !c.onList(s, list) {
			offList[fmt.Sprintf("full.known_spell_keys[%d]", i)] = true
		}
	}
	if len(offList) > offer.AnyClassSpells {
		return nil, refuse("full.known_spell_keys", LevelUpReasonSpells, "only %d of the new spells may come from another class's list", offer.AnyClassSpells)
	}
	return offList, nil
}

// checkDuplicates refuses a key twice in any list of the new sheet.
func checkDuplicates(b Build) *LevelUpError {
	for _, l := range []struct {
		field, reason string
		keys          []string
	}{
		{"full.cantrip_keys", LevelUpReasonCantrips, b.Cantrips},
		{"full.known_spell_keys", LevelUpReasonSpells, b.SpellsKnown},
		{"full.prepared_spell_keys", LevelUpReasonPrepared, b.SpellsPrepared},
		{"full.feature_choice_keys", LevelUpReasonFeatureChoice, b.FeatureChoices},
		{"full.skill_proficiency_keys", LevelUpReasonSkills, b.SkillProficiencies},
		{"full.expertise_skill_keys", LevelUpReasonExpertise, b.Expertise},
	} {
		seen := map[string]bool{}
		for i, k := range l.keys {
			if seen[k] {
				return refuse(fmt.Sprintf("%s[%d]", l.field, i), l.reason, "the same entry twice")
			}
			seen[k] = true
		}
	}
	return nil
}
