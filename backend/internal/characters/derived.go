package characters

import (
	"math"
	"strings"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// This file turns package rules' Go types into the rules.v1 messages: the
// derived sheet (GetCharacter) and the catalog (ListContent). It only copies
// and renames; every number comes from package rules.

var abilityToProto = map[rules.Ability]rulesv1.Ability{
	rules.STR: rulesv1.Ability_ABILITY_STRENGTH,
	rules.DEX: rulesv1.Ability_ABILITY_DEXTERITY,
	rules.CON: rulesv1.Ability_ABILITY_CONSTITUTION,
	rules.INT: rulesv1.Ability_ABILITY_INTELLIGENCE,
	rules.WIS: rulesv1.Ability_ABILITY_WISDOM,
	rules.CHA: rulesv1.Ability_ABILITY_CHARISMA,
}

var proficiencyToProto = map[rules.ProficiencyLevel]rulesv1.ProficiencyLevel{
	rules.ProficiencyNone:      rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_NONE,
	rules.ProficiencyHalf:      rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_HALF,
	rules.ProficiencyFull:      rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_PROFICIENT,
	rules.ProficiencyExpertise: rulesv1.ProficiencyLevel_PROFICIENCY_LEVEL_EXPERTISE,
}

var attackKindToProto = map[string]rulesv1.AttackKind{
	"weapon": rulesv1.AttackKind_ATTACK_KIND_WEAPON,
	"spell":  rulesv1.AttackKind_ATTACK_KIND_SPELL,
}

// derivedToProto turns rules.Derived into the DerivedSheet the app shows.
func derivedToProto(d rules.Derived) *rulesv1.DerivedSheet {
	out := &rulesv1.DerivedSheet{
		ContentVersion:        d.ContentVersion,
		RaceNamePt:            d.RaceNamePT,
		SubraceNamePt:         d.SubraceNamePT,
		BackgroundNamePt:      d.BackgroundNamePT,
		TotalLevel:            i32(d.TotalLevel),
		ProficiencyBonus:      i32(d.ProficiencyBonus),
		PassivePerception:     i32(d.PassivePerception),
		PassiveInvestigation:  i32(d.PassiveInvestigation),
		PassiveInsight:        i32(d.PassiveInsight),
		Initiative:            i32(d.Initiative),
		ArmorClass:            i32(d.ArmorClass),
		ArmorClassDescription: d.ArmorClassDescription,
		HitPointsMax:          i32(d.HitPointsMax),
		SpeedWalkFt:           i32(d.SpeedWalkFt),
		Proficiencies:         &rulesv1.Proficiencies{},
	}
	for _, c := range d.Classes {
		out.Classes = append(out.Classes, &rulesv1.DerivedClass{
			ClassKey:       c.ClassKey,
			NamePt:         c.NamePT,
			Level:          i32(c.Level),
			SubclassNamePt: c.SubclassNamePT,
		})
	}
	for _, a := range d.Abilities {
		out.Abilities = append(out.Abilities, &rulesv1.DerivedAbility{
			Ability:  abilityToProto[a.Ability],
			NamePt:   a.NamePT,
			Base:     i32(a.Base),
			Bonus:    i32(a.Bonus),
			Score:    i32(a.Score),
			Modifier: i32(a.Modifier),
		})
	}
	for _, st := range d.SavingThrows {
		out.SavingThrows = append(out.SavingThrows, &rulesv1.SavingThrow{
			Ability:    abilityToProto[st.Ability],
			NamePt:     st.NamePT,
			Proficient: st.Proficient,
			Bonus:      i32(st.Bonus),
		})
	}
	for _, sk := range d.Skills {
		out.Skills = append(out.Skills, &rulesv1.DerivedSkill{
			Key:         sk.Key,
			NamePt:      sk.NamePT,
			Ability:     abilityToProto[sk.Ability],
			Proficiency: proficiencyToProto[sk.Proficiency],
			Bonus:       i32(sk.Bonus),
		})
	}
	for _, hd := range d.HitDice {
		out.HitDice = append(out.HitDice, &rulesv1.HitDice{Faces: i32(hd.Die), Count: i32(hd.Count)})
	}
	for _, se := range d.Senses {
		// The proto's key is what grants the sense, as the Hint's source_key.
		out.Senses = append(out.Senses, &rulesv1.Sense{Key: se.Source, NamePt: se.NamePT, RangeFt: i32(se.RangeFt)})
	}
	for _, sc := range d.Spellcasting {
		p := &rulesv1.Spellcasting{
			ClassKey:      sc.Class,
			ClassNamePt:   sc.ClassNamePT,
			Ability:       abilityToProto[sc.Ability],
			SaveDc:        i32(sc.SaveDC),
			AttackBonus:   i32(sc.AttackBonus),
			CantripsKnown: i32(sc.CantripsKnown),
		}
		// Only one of the two limits applies, and the other stays 0.
		if sc.PreparesSpells {
			p.PreparedMax = i32(sc.PreparedMax)
		} else {
			p.SpellsKnown = i32(sc.SpellsKnownMax)
		}
		out.Spellcasting = append(out.Spellcasting, p)
	}
	for i, n := range d.SpellSlots {
		if n > 0 {
			out.SpellSlots = append(out.SpellSlots, &rulesv1.SpellSlots{Level: i32(i + 1), Count: i32(n)})
		}
	}
	if d.PactMagic != nil {
		out.PactMagic = &rulesv1.PactMagic{SlotLevel: i32(d.PactMagic.SlotLevel), Count: i32(d.PactMagic.Slots)}
	}
	for _, sp := range d.Spells {
		out.Spells = append(out.Spells, &rulesv1.CharacterSpell{Spell: spellToProto(sp.Spell), Prepared: sp.Prepared})
	}
	for _, a := range d.Attacks {
		out.Attacks = append(out.Attacks, &rulesv1.Attack{
			Key:             a.Key,
			Name:            a.Name,
			NamePt:          a.NamePT,
			AttackBonus:     i32(a.AttackBonus),
			Damage:          a.Damage,
			DamageTypePt:    a.DamageTypeNamePT,
			Kind:            attackKindToProto[a.Kind],
			SaveDc:          i32(a.SaveDC),
			SaveAbility:     abilityToProto[a.SaveAbility],
			VersatileDamage: a.VersatileDamage,
			RangeFt:         i32(a.RangeFt),
			LongRangeFt:     i32(a.LongRangeFt),
		})
	}
	for _, f := range d.Features {
		out.Features = append(out.Features, &rulesv1.Feature{
			Key:         f.Key,
			Name:        f.Name,
			NamePt:      f.NamePT,
			SourcePt:    f.SourcePT,
			Description: strings.Join(f.Description, "\n\n"),
		})
	}
	for _, l := range d.Languages {
		out.Languages = append(out.Languages, l.NamePT)
	}
	for _, p := range d.Proficiencies {
		switch p.Kind {
		case "armor":
			out.Proficiencies.Armor = append(out.Proficiencies.Armor, p.NamePT)
		case "weapon":
			out.Proficiencies.Weapons = append(out.Proficiencies.Weapons, p.NamePT)
		case "tool":
			out.Proficiencies.Tools = append(out.Proficiencies.Tools, p.NamePT)
		} // "other" (such as saving throws) is shown elsewhere on the sheet
	}
	for _, h := range d.Hints {
		out.Hints = append(out.Hints, &rulesv1.Hint{SourceKey: h.Source, Text: h.TextPT})
	}
	for _, is := range d.Issues {
		out.Issues = append(out.Issues, &rulesv1.Issue{Code: is.Code, Field: is.Field, Message: is.Message})
	}
	return out
}

func spellToProto(s rules.SpellEntry) *rulesv1.Spell {
	return &rulesv1.Spell{
		Key:           s.Key,
		Name:          s.Name,
		NamePt:        s.NamePT,
		Level:         i32(s.Level),
		SchoolKey:     s.School,
		SchoolNamePt:  s.SchoolNamePT,
		ClassKeys:     s.Classes,
		Ritual:        s.Ritual,
		Concentration: s.Concentration,
	}
}

func abilityScores(m map[rules.Ability]int) *rulesv1.AbilityScores {
	return &rulesv1.AbilityScores{
		Strength:     i32(m[rules.STR]),
		Dexterity:    i32(m[rules.DEX]),
		Constitution: i32(m[rules.CON]),
		Intelligence: i32(m[rules.INT]),
		Wisdom:       i32(m[rules.WIS]),
		Charisma:     i32(m[rules.CHA]),
	}
}

var (
	preparationToProto = map[string]rulesv1.SpellPreparation{
		rules.PreparationKnown:     rulesv1.SpellPreparation_SPELL_PREPARATION_KNOWN,
		rules.PreparationPrepared:  rulesv1.SpellPreparation_SPELL_PREPARATION_PREPARED,
		rules.PreparationSpellbook: rulesv1.SpellPreparation_SPELL_PREPARATION_SPELLBOOK,
	}
	armorCategoryToProto = map[string]rulesv1.ArmorCategory{
		"light":  rulesv1.ArmorCategory_ARMOR_CATEGORY_LIGHT,
		"medium": rulesv1.ArmorCategory_ARMOR_CATEGORY_MEDIUM,
		"heavy":  rulesv1.ArmorCategory_ARMOR_CATEGORY_HEAVY,
	}
	weaponCategoryToProto = map[string]rulesv1.WeaponCategory{
		"simple":  rulesv1.WeaponCategory_WEAPON_CATEGORY_SIMPLE,
		"martial": rulesv1.WeaponCategory_WEAPON_CATEGORY_MARTIAL,
	}
)

// catalogToProto turns the rules catalog into ListContent's answer.
func catalogToProto(c rules.Catalog) *rulesv1.Content {
	out := &rulesv1.Content{ContentVersion: c.ContentVersion, Attribution: c.Attribution}
	for _, a := range c.Abilities {
		out.Abilities = append(out.Abilities, &rulesv1.AbilityInfo{
			Ability: abilityToProto[a.Ability], Name: a.Name, NamePt: a.NamePT, AbbreviationPt: a.AbbreviationPT,
		})
	}
	for _, r := range c.Races {
		out.Races = append(out.Races, &rulesv1.Race{
			Key: r.Key, Name: r.Name, NamePt: r.NamePT, SpeedFt: i32(r.SpeedFt), AbilityBonuses: abilityScores(r.AbilityBonuses),
		})
	}
	for _, s := range c.Subraces {
		out.Subraces = append(out.Subraces, &rulesv1.Subrace{
			Key: s.Key, Name: s.Name, NamePt: s.NamePT, RaceKey: s.Race, AbilityBonuses: abilityScores(s.AbilityBonuses),
		})
	}
	for _, cl := range c.Classes {
		p := &rulesv1.CharacterClass{
			Key:           cl.Key,
			Name:          cl.Name,
			NamePt:        cl.NamePT,
			HitDie:        i32(cl.HitDie),
			SkillChoice:   &rulesv1.SkillChoice{Count: i32(cl.SkillChoices), SkillKeys: cl.SkillOptions},
			SubclassLevel: i32(cl.SubclassLevel),
		}
		for _, a := range cl.SavingThrows {
			p.SavingThrows = append(p.SavingThrows, abilityToProto[a])
		}
		if cl.SpellcastingAbility != "" {
			p.Spellcasting = &rulesv1.ClassSpellcasting{
				Ability:     abilityToProto[cl.SpellcastingAbility],
				FirstLevel:  i32(cl.SpellcastingLevel),
				Preparation: preparationToProto[cl.SpellPreparation],
			}
			for _, n := range cl.MaxSpellLevelByLevel {
				p.Spellcasting.MaxSpellLevelByLevel = append(p.Spellcasting.MaxSpellLevelByLevel, i32(n))
			}
		}
		out.Classes = append(out.Classes, p)
	}
	for _, s := range c.Subclasses {
		out.Subclasses = append(out.Subclasses, &rulesv1.Subclass{Key: s.Key, Name: s.Name, NamePt: s.NamePT, ClassKey: s.Class})
	}
	for _, b := range c.Backgrounds {
		out.Backgrounds = append(out.Backgrounds, &rulesv1.Background{Key: b.Key, Name: b.Name, NamePt: b.NamePT, SkillKeys: b.SkillProficiencies})
	}
	for _, s := range c.Skills {
		out.Skills = append(out.Skills, &rulesv1.Skill{Key: s.Key, Name: s.Name, NamePt: s.NamePT, Ability: abilityToProto[s.Ability]})
	}
	for _, a := range c.Armor {
		out.Armor = append(out.Armor, &rulesv1.Armor{Key: a.Key, Name: a.Name, NamePt: a.NamePT, Category: armorCategoryToProto[a.Category]})
	}
	for _, w := range c.Weapons {
		out.Weapons = append(out.Weapons, &rulesv1.Weapon{
			Key: w.Key, Name: w.Name, NamePt: w.NamePT, Category: weaponCategoryToProto[w.Category], Ranged: w.Range == "ranged",
		})
	}
	for _, s := range c.Spells {
		out.Spells = append(out.Spells, spellToProto(s))
	}
	return out
}

// i32 converts a number from package rules for the API. Those numbers are
// small (the SRD's tables, and sheets that passed rules.Validate), so the
// clamp never changes a real value; it keeps the conversion provably safe.
func i32(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
}
