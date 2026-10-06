package rules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/PuraFome/meuRPG/backend/internal/rules/srd51"
)

// overlayEffectTypes is the closed menu of what the table's features may do
// (ADR-0018, section 4): the effect types of the SRD minus the ones that need
// code or the engine's own tables. There is no "handler" (the table never runs
// code, not even a handler that exists), no "spellcasting" (the engine writes it
// from the class's casting) and no "wild_shape".
var overlayEffectTypes = []string{
	"modifier", "proficiency", "resource", "sense", "roll_mode", "grant_action",
	"extra_attack", "choice", "note",
}

// overlayChoiceKinds are the choices a table feature may offer. The subclass and
// the Ability Score Improvement are the class table's, not a feature's.
var overlayChoiceKinds = []string{
	"skill", "expertise", "cantrip", "spell", "language", "tool", "feature",
}

// featureKind is where a feature belongs: a class, a subclass, a race (trait)
// or a background.
type featureKind struct {
	// prefix is the key prefix: "feature:", "trait:" or "background-feature:".
	prefix string
	// owner is the entry's key (a class, subclass, race, subrace or background),
	// and class and subclass set the Feature's own fields.
	owner, class, subclass string
	level                  int
}

// addFeature registers one table feature (or trait) in the clone: its name, its
// text and the effects, which are compiled later, once every entry exists. A
// feature that offers a choice of SRD options (a "choice" effect of kind
// "feature") also becomes what the level-up and Derive read as the options'
// offerer.
func (b *overlayBuilder) addFeature(f *TableFeature, k featureKind, path string) error {
	n := b.n
	if err := checkTableKey(k.prefix, f.Key); err != nil {
		return locate(err, path)
	}
	if err := b.claim(f.Key); err != nil {
		return locate(err, path)
	}
	if err := checkEntryName(f.Key, f.NamePT); err != nil {
		return locate(err, path)
	}
	if err := checkText(f.Key, f.DescPT); err != nil {
		return locate(err, path+".desc_pt")
	}
	n.namesEN[f.Key] = f.NamePT
	n.namesPT[f.Key] = f.NamePT
	switch k.prefix {
	case "feature:":
		feat := &srd51.Feature{Key: f.Key, Name: f.NamePT, Class: k.class, Subclass: k.subclass, Level: k.level, Desc: slices.Clone(f.DescPT)}
		for _, e := range f.Effects {
			if e.Type != "choice" || e.Choice != "feature" {
				continue
			}
			if len(feat.Options) > 0 {
				return ovErr(f.Key, "a feature offers one choice of options").at(path+".effects", ReasonEffect)
			}
			feat.Options, feat.OptionsChoose = slices.Clone(e.From), e.Count
			for _, o := range e.From {
				n.offeredBy[o] = append(n.offeredBy[o], f.Key)
			}
		}
		n.features[f.Key] = feat
	case "trait:":
		t := &srd51.Trait{Key: f.Key, Name: f.NamePT, Desc: slices.Clone(f.DescPT)}
		if strings.HasPrefix(k.owner, "subrace:") {
			t.Subraces = []string{k.owner}
		} else {
			t.Races = []string{k.owner}
		}
		n.traits[f.Key] = t
	}
	if len(f.Effects) > 0 {
		b.pending = append(b.pending, pendingEffects{owner: f.Key, effects: f.Effects, path: path})
	}
	return nil
}

// checkEffect is the closed menu: what is not on it is refused, naming the key.
// The error carries the effect's path.
func (b *overlayBuilder) checkEffect(owner, path string, e *Effect) error {
	fail := func(attr, reason, format string, args ...any) error {
		return ovErr(owner, format, args...).at(path+attr, reason)
	}
	if e.Type == "handler" {
		return fail(".type", ReasonEffect, "a handler effect is not allowed: the table's content never runs code")
	}
	if !slices.Contains(overlayEffectTypes, e.Type) {
		return fail(".type", ReasonEffect, "effect type %q is not on the table's menu", e.Type)
	}
	if len(e.Spells) > 0 {
		// A spell the effect grants (a race that knows a cantrip, a once-a-day
		// spell): a note with the spells, as the SRD's Infernal Legacy. The uses
		// per rest are a resource effect of the same feature.
		if e.Type != "note" {
			return fail(".spells", ReasonEffect, "only a note grants spells in the table's content")
		}
		for _, sp := range e.Spells {
			if !b.isSpell(sp) {
				return fail(".spells", ReasonReference, "the granted spell %q does not exist", sp)
			}
		}
	}
	switch e.Type {
	case "choice":
		if !slices.Contains(overlayChoiceKinds, e.Choice) {
			return fail(".choice", ReasonEffect, "choice %q is not on the table's menu", e.Choice)
		}
		if e.Count < 1 {
			return fail(".count", ReasonValue, "a choice needs a count of at least 1")
		}
		if e.Choice == "feature" && len(e.From) == 0 {
			return fail(".from", ReasonValue, "a choice of features needs the options to choose from")
		}
		for _, k := range e.From {
			// The options come from an SRD set: a key the SRD has, never one of the
			// table's (and never one that does not exist).
			if isTableKey(k) || !b.base.exists(k) {
				return fail(".from", ReasonReference, "choice option %q is not in the SRD", k)
			}
			if e.Choice == "feature" && !isOption(b.base, k) {
				return fail(".from", ReasonReference, "%q is not an option of an SRD feature", k)
			}
		}
	case "resource":
		if !validResourceName(e.Resource) {
			return fail(".resource", ReasonValue, "a resource name has 1 to 40 characters of a-z, 0-9 and _")
		}
		if _, taken := b.base.namesPT["resource:"+e.Resource]; taken {
			return fail(".resource", ReasonValue, "resource %q is an SRD resource; use another name", e.Resource)
		}
	}
	return nil
}

func validResourceName(s string) bool {
	if len(s) < 1 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

// compileEffects checks and compiles every pending effect with the SRD's own
// compiler (compileEffect), once the entries they refer to exist. The overlay's
// Effect values are copied, so the caller's are never written.
func (b *overlayBuilder) compileEffects() error {
	for _, p := range b.pending {
		out := make([]*Effect, 0, len(p.effects))
		for i := range p.effects {
			e := p.effects[i]
			at := fmt.Sprintf("%s.effects[%d]", p.path, i)
			e.Tags, e.Targets, e.From, e.Spells = slices.Clone(e.Tags), slices.Clone(e.Targets), slices.Clone(e.From), slices.Clone(e.Spells)
			if err := b.checkEffect(p.owner, at, &e); err != nil {
				return err
			}
			if err := b.n.compileEffect(p.owner, &e); err != nil {
				attr, reason := "", ReasonValue
				if strings.Contains(err.Error(), "formula") {
					attr, reason = ".formula", ReasonFormula
				}
				return ovErr(p.owner, "%v", err).at(at+attr, reason)
			}
			out = append(out, &e)
		}
		b.n.effects[p.owner] = append(b.n.effects[p.owner], out...)
	}
	return nil
}

// compileOwn compiles an effect the engine wrote itself (the spellcasting
// effect), which is not on the table's menu.
func (b *overlayBuilder) compileOwn(owner string, e *Effect) error {
	if err := b.n.compileEffect(owner, e); err != nil {
		return ovErr(owner, "%v", err)
	}
	b.n.effects[owner] = append(b.n.effects[owner], e)
	return nil
}

// bonusMap turns a map of ability bonuses into the SRD's shape, checking the
// abilities and the size of each bonus.
func bonusMap(key string, in map[Ability]int) (map[string]int, error) {
	out := make(map[string]int, len(in))
	for a, v := range in {
		if _, ok := abilityIndex[a]; !ok {
			return nil, ovErr(key, "unknown ability in the ability bonuses")
		}
		if v < -maxRaceBonus || v > maxRaceBonus {
			return nil, ovErr(key, "an ability bonus is %d to %d", -maxRaceBonus, maxRaceBonus)
		}
		out[string(a)] = v
	}
	return out, nil
}

// maxRaceBonus bounds a race's ability bonus, in either direction.
const maxRaceBonus = 4

// addRace registers a race and its traits.
func (b *overlayBuilder) addRace(tr *TableRace, path string) error {
	n := b.n
	key := tr.Key
	if !slices.Contains([]string{"Tiny", "Small", "Medium", "Large"}, tr.Size) {
		return ovErr(key, "size is Tiny, Small, Medium or Large")
	}
	if tr.SpeedFt < 5 || tr.SpeedFt > 120 || tr.SpeedFt%5 != 0 {
		return ovErr(key, "the speed is 5 to 120 feet, in steps of 5")
	}
	if tr.DarkvisionFt < 0 || tr.DarkvisionFt > 120 || tr.DarkvisionFt%5 != 0 {
		return ovErr(key, "the darkvision is 0 or 5 to 120 feet, in steps of 5")
	}
	bonuses, err := bonusMap(key, tr.AbilityBonuses)
	if err != nil {
		return err
	}
	choice := slices.Clone(tr.ChoiceBonuses)
	if len(choice) > len(abilityIndex) {
		return ovErr(key, "at most %d bonuses to place", len(abilityIndex))
	}
	for _, v := range choice {
		if v < 1 || v > maxRaceBonus {
			return ovErr(key, "a bonus to place is 1 to %d", maxRaceBonus)
		}
	}
	slices.SortFunc(choice, func(a, c int) int { return c - a })
	if tr.LanguageChoices < 0 || tr.LanguageChoices > 4 {
		return ovErr(key, "languages to choose is 0 to 4")
	}
	for _, l := range tr.Languages {
		if _, ok := b.base.languages[l]; !ok {
			return ovErr(key, "language %q is not in the SRD", l)
		}
	}
	race := &srd51.Race{
		Key: key, Name: tr.NamePT, SpeedFt: tr.SpeedFt, Size: tr.Size, AbilityBonuses: bonuses,
		Languages: slices.Clone(tr.Languages), LanguageChoices: tr.LanguageChoices,
	}
	for i := range tr.Traits {
		t := &tr.Traits[i]
		if err := b.addFeature(t, featureKind{prefix: "trait:", owner: key}, fmt.Sprintf("%s.traits[%d]", path, i)); err != nil {
			return err
		}
		race.Traits = append(race.Traits, t.Key)
	}
	b.register(tr.TableEntry)
	n.races[key] = race
	if len(choice) > 0 {
		n.raceChoice[key] = choice
	}
	if tr.DarkvisionFt > 0 {
		// The darkvision is an effect of the race itself: Derive reads those too.
		b.pending = append(b.pending, pendingEffects{owner: key, effects: []Effect{{Type: "sense", Sense: "darkvision", RangeFt: tr.DarkvisionFt}}, path: path + ".darkvision_ft"})
	}
	return nil
}

// addSubrace registers a subrace and puts it in its race's list, on a copy of
// the race when the race is the SRD's.
func (b *overlayBuilder) addSubrace(ts *TableSubrace, path string) error {
	key := ts.Key
	if !b.isRace(ts.Race) {
		return ovErr(key, "the race %q does not exist", ts.Race)
	}
	bonuses, err := bonusMap(key, ts.AbilityBonuses)
	if err != nil {
		return err
	}
	sub := &srd51.Subrace{Key: key, Name: ts.NamePT, Race: ts.Race, AbilityBonuses: bonuses}
	for i := range ts.Traits {
		t := &ts.Traits[i]
		if err := b.addFeature(t, featureKind{prefix: "trait:", owner: key}, fmt.Sprintf("%s.traits[%d]", path, i)); err != nil {
			return err
		}
		sub.Traits = append(sub.Traits, t.Key)
	}
	b.register(ts.TableEntry)
	b.n.subraces[key] = sub
	// The race lists its subraces: on a copy, because the SRD's race is shared.
	race := *b.n.races[ts.Race]
	race.Subraces = append(slices.Clone(race.Subraces), key)
	b.n.races[ts.Race] = &race
	return nil
}

// addBackground registers a background and its feature.
func (b *overlayBuilder) addBackground(tb *TableBackground, path string) error {
	n := b.n
	key := tb.Key
	if len(tb.Skills) != CustomBackgroundSkillCount || tb.Skills[0] == tb.Skills[1] {
		return ovErr(key, "a background gives %d different skills", CustomBackgroundSkillCount)
	}
	for _, s := range tb.Skills {
		if _, ok := b.base.skills[s]; !ok {
			return ovErr(key, "skill %q is not in the SRD", s)
		}
	}
	if len(tb.Tools) > 4 || tb.LanguageChoices < 0 || tb.LanguageChoices > 4 {
		return ovErr(key, "at most 4 tools and 4 languages to choose")
	}
	for _, t := range tb.Tools {
		if p, ok := b.base.proficiencies[t]; !ok || (p.Kind != "tool" && p.Kind != "other") {
			return ovErr(key, "%q is not a tool proficiency of the SRD", t)
		}
	}
	if err := checkText(key, []string{tb.EquipmentPT}); err != nil {
		return err
	}
	f := &tb.Feature
	if err := b.addFeature(f, featureKind{prefix: "background-feature:", owner: key}, path+".feature"); err != nil {
		return err
	}
	b.register(tb.TableEntry)
	n.backgrounds[key] = &srd51.Background{
		Key: key, Name: tb.NamePT, Skills: slices.Clone(tb.Skills), Proficiencies: slices.Clone(tb.Tools),
		LanguageChoices: tb.LanguageChoices,
		Feature:         srd51.BackgroundFeature{Key: f.Key, Name: f.NamePT, Desc: slices.Clone(f.DescPT)},
	}
	if tb.EquipmentPT != "" {
		n.bgEquipment[key] = tb.EquipmentPT
	}
	return nil
}
