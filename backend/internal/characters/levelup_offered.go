package characters

import (
	"slices"
	"strings"

	charactersv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/characters/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// offeredChoices are the keys a level-up offers the player the character: the options of the features of the level (the
// class's, the subclasses the player may pick, the choices an earlier level left open and the new level's), and the options a
// swap may take. It is what GetLevelUpOptions lists after hideFromPlayer.
type offeredChoices struct {
	options map[string]bool
	swaps   map[string]bool
	// invocations are the Eldritch Invocations a warlock may take when it gives one up.
	invocations map[string]bool
	// owned are the options the character's sheet already has: their own picks (the Book of Ancient Secrets rituals)
	// are offered too.
	owned map[string]bool
}

// offeredChoicesOf reads what the level offers this player.
func offeredChoicesOf(t levelUpTarget, m authz.Membership) (offeredChoices, error) {
	offer, lerr := rules.LevelUpOptions(t.build, t.classKey, viewContent(t.content, m))
	if lerr != nil {
		return offeredChoices{}, levelUpRulesError(lerr)
	}
	o := offeredChoices{options: map[string]bool{}, swaps: map[string]bool{}, invocations: map[string]bool{}, owned: map[string]bool{}}
	for _, k := range t.build.FeatureChoices {
		o.owned[k] = true
	}
	for _, n := range offer.InvocationSwaps {
		if !hiddenOption(t, n.Key) {
			o.invocations[n.Key] = true
		}
	}
	o.addFeatureChoices(t, offer.FeatureChoices)
	o.addSubclasses(t, offer.Subclasses)
	o.addChoiceGroups(t, offer.LateChoices)
	o.addChoiceGroups(t, offer.NewChoices)
	o.addSwapPools(t, offer.PickSwaps)
	o.addSwapPools(t, offer.FeatureSwaps)
	return o, nil
}

// addSubclasses adds the options of the subclasses the player may pick: not one the table retired or switched off, unless the
// character already has it.
func (o offeredChoices) addSubclasses(t levelUpTarget, subs []rules.LevelUpSubclass) {
	for _, sub := range subs {
		if (sub.Archived || sub.Off) && !slices.ContainsFunc(t.build.Classes, func(c rules.ClassLevel) bool { return c.Subclass == sub.Key }) {
			continue
		}
		o.addFeatureChoices(t, sub.FeatureChoices)
	}
}

func (o offeredChoices) addSwapPools(t levelUpTarget, groups []rules.SwapGroup) {
	for _, g := range groups {
		for _, n := range g.Pool {
			if !hiddenOption(t, n.Key) {
				o.swaps[n.Key] = true
			}
		}
	}
}

func (o offeredChoices) addFeatureChoices(t levelUpTarget, choices []rules.LevelUpFeatureChoice) {
	for _, ch := range choices {
		for _, n := range ch.Options {
			o.addOption(t, n.Key)
		}
		for _, b := range ch.Blocked {
			o.addOption(t, b.Key)
		}
	}
}

func (o offeredChoices) addChoiceGroups(t levelUpTarget, groups []rules.ChoiceGroup) {
	for _, g := range groups {
		for _, ch := range g.Choices {
			for _, opt := range ch.Options {
				o.addOption(t, opt.Key)
				o.addOption(t, opt.Stored)
			}
		}
	}
}

func (o offeredChoices) addOption(t levelUpTarget, key string) {
	if key != "" && !hiddenOption(t, key) {
		o.options[key] = true
	}
}

// unofferedRefusal is the refusal of a request that sends a key the level does not offer this player, or nil: the answer to
// it is that refusal alone, with no sheet built from the key. The master reads the sheet as before.
func unofferedRefusal(t levelUpTarget, m authz.Membership, choices *charactersv1.LevelUpChoices) (*charactersv1.LevelUpRefusal, error) {
	if isMaster(m) {
		return nil, nil
	}
	if len(choices.GetFeatureChoiceKeys()) == 0 && len(choices.GetSwapFeatureOptions()) == 0 && choices.GetSwapPick() == nil {
		return nil, nil
	}
	offered, err := offeredChoicesOf(t, m)
	if err != nil {
		return nil, err
	}
	for _, k := range choices.GetFeatureChoiceKeys() {
		if !offered.offersKey(k, choices.GetSwapInvocationKey() != "") {
			return &charactersv1.LevelUpRefusal{
				Field: "choices.feature_choice_keys", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_FEATURE_CHOICE,
			}, nil
		}
	}
	for _, sw := range choices.GetSwapFeatureOptions() {
		if !offered.swaps[sw.GetToKey()] {
			return &charactersv1.LevelUpRefusal{
				Field: "choices.swap_feature_options", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_SWAP_NOT_ALLOWED,
			}, nil
		}
	}
	if sw := choices.GetSwapPick(); sw != nil && sw.GetToKey() != "" && !offered.swaps[sw.GetToKey()] {
		return &charactersv1.LevelUpRefusal{
			Field: "choices.swap_pick", Reason: charactersv1.LevelUpRefusalReason_LEVEL_UP_REFUSAL_REASON_SWAP_NOT_ALLOWED,
		}, nil
	}
	return nil, nil
}

// offersKey says the level offers the feature choice key k: an option, an invocation taken in the place of the one given
// up, or a pick of the choice of an option (a switched-off or retired pick is for the plan to refuse with its own reason).
func (o offeredChoices) offersKey(k string, swapsInvocation bool) bool {
	if o.options[k] || (swapsInvocation && o.invocations[k]) {
		return true
	}
	choiceKey, _, ok := rules.SplitScopedChoice(k)
	if !ok {
		return false
	}
	owner, _, _ := strings.Cut(choiceKey, "#")
	return o.options[owner] || o.owned[owner] || (swapsInvocation && o.invocations[owner])
}
