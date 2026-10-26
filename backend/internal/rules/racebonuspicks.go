package rules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The ability bonuses a table race lets the player place (choice_bonuses, a single set, or choice_bonus_sets) are
// picks of the choice engine, as the half-elf's +1s are: a choice of the set when the race has several, then one choice
// of abilities for each distinct bonus ("Bônus de +2", "Bônus de +1") that takes as many abilities as the set has of it.
// Each ability gets one bonus (the picks are different abilities), and Derive adds them to the race's own.
//
//	race:x#bonus-set=bonus-set:1   the second set
//	race:x#bonus-2=ability:str     +2 to Strength
//	race:x#bonus-1=ability:dex     +1 to Dexterity

// bonusSetPrefix starts the value of the pick of a set ("bonus-set:0").
const bonusSetPrefix = "bonus-set:"

// BonusSetChoiceKey is the key of a race's choice among its sets of bonuses.
func BonusSetChoiceKey(raceKey string) string { return raceKey + textSeparator + "bonus-set" }

// BonusChoiceKey is the key of a race's choice of the abilities that get the bonus.
func BonusChoiceKey(raceKey string, bonus int) string {
	return raceKey + textSeparator + "bonus-" + strconv.Itoa(bonus)
}

// bonusOfChoiceKey is the bonus a choice key of BonusChoiceKey stands for, 0 for any other key.
func bonusOfChoiceKey(key string) int {
	_, after, ok := strings.Cut(key, textSeparator+"bonus-")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(after)
	if err != nil {
		return 0
	}
	return n
}

// bonusSetValue writes the pick of the i-th set.
func bonusSetValue(i int) string { return bonusSetPrefix + strconv.Itoa(i) }

// bonusSetPick reads the pick of a set, -1 when the value is not one.
func bonusSetPick(value string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(value, bonusSetPrefix))
	if !strings.HasPrefix(value, bonusSetPrefix) || err != nil || n < 0 {
		return -1
	}
	return n
}

// bonusSetWords writes a set as the player reads it: "+2 e +1", "+1, +1 e +1".
func bonusSetWords(set []int) string {
	parts := make([]string, len(set))
	for i, v := range set {
		parts[i] = signed(v)
	}
	return joinList(parts)
}

// distinctBonuses are the values of a set, largest first, each once, with how many the set has of it.
func distinctBonuses(set []int) (values, counts []int) {
	for _, v := range set {
		if i := slices.Index(values, v); i >= 0 {
			counts[i]++
			continue
		}
		values, counts = append(values, v), append(counts, 1)
	}
	return values, counts
}

// racePicks are what a Build picked of a race's sets of bonuses.
type racePicks struct {
	// set is the index of the picked set, -1 while none is.
	set int
	// byBonus are the abilities picked for each bonus, in the order of the abilities, each ability once (the larger
	// bonus keeps it), and no more than the set has of the bonus.
	byBonus map[int][]Ability
}

// complete says the picks fill the set.
func (p racePicks) complete(sets [][]int) bool {
	if p.set < 0 || p.set >= len(sets) {
		return false
	}
	values, counts := distinctBonuses(sets[p.set])
	for i, v := range values {
		if len(p.byBonus[v]) != counts[i] {
			return false
		}
	}
	return true
}

// bonuses are the bonuses the picks give, by ability.
func (p racePicks) bonuses() map[Ability]int {
	out := map[Ability]int{}
	for v, abilities := range p.byBonus {
		for _, a := range abilities {
			out[a] += v
		}
	}
	return out
}

// racePicksOf reads the picks of a race's sets out of the scoped picks ("choice key" to values) of a Build.
func (c *content) racePicksOf(raceKey string, scoped map[string][]string) racePicks {
	sets := c.raceBonusSets(raceKey)
	out := racePicks{set: -1, byBonus: map[int][]Ability{}}
	if len(sets) == 0 {
		return out
	}
	if len(sets) == 1 {
		out.set = 0
	} else {
		for _, v := range scoped[BonusSetChoiceKey(raceKey)] {
			if i := bonusSetPick(v); i >= 0 && i < len(sets) {
				out.set = i
				break
			}
		}
	}
	if out.set < 0 {
		return out
	}
	values, counts := distinctBonuses(sets[out.set])
	taken := map[Ability]bool{}
	for i, v := range values {
		var picked []Ability
		for _, a := range AllAbilities() {
			if !taken[a] && len(picked) < counts[i] && slices.Contains(scoped[BonusChoiceKey(raceKey, v)], abilityValue(a)) {
				picked = append(picked, a)
				taken[a] = true
			}
		}
		out.byBonus[v] = picked
	}
	return out
}

// scopedOf are the scoped picks of the Build's feature choices, by choice key.
func scopedOf(featureChoices []string) map[string][]string {
	out := map[string][]string{}
	for _, k := range featureChoices {
		if choiceKey, value, ok := SplitScopedChoice(k); ok && !slices.Contains(out[choiceKey], value) {
			out[choiceKey] = append(out[choiceKey], value)
		}
	}
	return out
}

// manualCoversSet is the index of the first set the manual bonuses cover (a sheet that placed the bonuses by hand,
// as a table race asked before they were picks), -1 when they cover none.
func (c *content) manualCoversSet(raceKey string, manual map[Ability]int) int {
	var placed []int
	for _, v := range manual {
		if v > 0 {
			placed = append(placed, v)
		}
	}
	slices.SortFunc(placed, func(a, b int) int { return b - a })
	return slices.IndexFunc(c.raceBonusSets(raceKey), func(want []int) bool { return coversBonuses(placed, want) })
}

// bonusChoices makes the choices of a table race's sets of bonuses: the set, when the race has several, and the
// abilities of each bonus of the picked set.
func (cb *choiceBuilder) bonusChoices(raceKey string, sets [][]int) {
	c := cb.c
	g := cb.group(ChoiceOriginRace, raceKey, 0, 0)
	picks := c.racePicksOf(raceKey, cb.scoped)
	legacy := -1
	if picks.set < 0 || len(picks.bonuses()) == 0 {
		// A sheet that placed the bonuses in the manual ones, before they were picks, keeps them: they stand for the picks.
		if i := c.manualCoversSet(raceKey, cb.b.ExtraAbilityBonuses); i >= 0 && len(picks.bonuses()) == 0 && (len(sets) == 1 || len(cb.scoped[BonusSetChoiceKey(raceKey)]) == 0) {
			legacy = i
		}
	}
	chosen := picks.set
	if len(sets) > 1 {
		ch := cb.addChoice(g, &Choice{Key: BonusSetChoiceKey(raceKey), FeatureKey: raceKey, Kind: ChoiceKindOptions, TitlePT: "Grupo de bônus de habilidade", Picks: 1})
		for i, set := range sets {
			ch.Options = append(ch.Options, ChoiceOption{Key: bonusSetValue(i), Stored: ScopedChoice(ch.Key, bonusSetValue(i)), NamePT: bonusSetWords(set)})
		}
		cb.takeScoped(ch)
		if legacy >= 0 {
			ch.legacy = 1
			chosen = legacy
		}
	} else if legacy >= 0 {
		chosen = legacy
	}
	if chosen < 0 {
		return
	}
	cb.bonusAbilityChoices(g, raceKey, sets[chosen], legacy >= 0)
}

// bonusAbilityChoices makes the choice of abilities of each bonus of a set. An ability picked for a bonus is not offered
// for the others, and a repeated pick is one the rules refuse.
func (cb *choiceBuilder) bonusAbilityChoices(g *groupBuild, raceKey string, set []int, legacy bool) {
	c := cb.c
	values, counts := distinctBonuses(set)
	var made []*Choice
	for i, v := range values {
		key := BonusChoiceKey(raceKey, v)
		ch := cb.addChoice(g, &Choice{Key: key, FeatureKey: raceKey, Kind: ChoiceKindAbilities, TitlePT: fmt.Sprintf("Bônus de %s", signed(v)), Picks: counts[i]})
		if counts[i] > 1 {
			ch.TitlePT = fmt.Sprintf("%d bônus de %s", counts[i], signed(v))
		}
		for _, a := range AllAbilities() {
			ch.Options = append(ch.Options, ChoiceOption{Key: abilityValue(a), Stored: ScopedChoice(key, abilityValue(a)), NamePT: c.namePT(string(a)), SummaryPT: signed(v)})
		}
		cb.takeScoped(ch)
		if legacy {
			ch.legacy = counts[i]
		}
		made = append(made, ch)
	}
	cb.separateBonuses(made)
}

// separateBonuses makes each ability take one bonus: a pick of an ability another bonus already took is refused
// (unmet) and the ability is blocked in the other choices.
func (cb *choiceBuilder) separateBonuses(made []*Choice) {
	takenBy := map[string]*Choice{}
	for _, ch := range made {
		kept := ch.Picked[:0:0]
		for _, k := range ch.Picked {
			if _, dup := takenBy[k]; dup {
				ch.Unmet = append(ch.Unmet, k)
				continue
			}
			takenBy[k] = ch
			kept = append(kept, k)
		}
		ch.Picked = kept
	}
	for _, ch := range made {
		for i := range ch.Options {
			o := &ch.Options[i]
			if owner, ok := takenBy[o.Key]; ok && owner != ch {
				o.ReasonPT = "Esta habilidade já recebeu o outro bônus."
				o.Prerequisites = append(o.Prerequisites, OptionPrerequisite{Kind: PrerequisiteTaken, ReasonPT: o.ReasonPT})
			}
		}
	}
}
