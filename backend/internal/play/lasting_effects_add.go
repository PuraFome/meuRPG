package play

import (
	"errors"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// What AddCharacterEffect (outside a combat) and AddLastingEffect (in one) have in common: the
// master picks an effect of the catalog by its key, may say who reads it, and may label it for
// the players.

// errInvalidEffect is the invalid_argument of the master's writes of effects.
func errInvalidEffect(text string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(text))
}

// errNotMasterEffect is the refusal of a catalog key the master may not add.
func errNotMasterEffect() error {
	return errInvalidEffect("catalog_key is not an effect the master may add")
}

// checkEffectLabel refuses a label of the players' card that is too long, in characters.
func checkEffectLabel(label string) error {
	if utf8.RuneCountInString(label) > maxEffectLabel {
		return errInvalidEffect("player_label must have at most 30 characters")
	}
	return nil
}

// The three kinds of key the catalog holds, by the prefix of the key: the conditions, the spells
// and the effects of the master (the rest).
const (
	catalogCondition = "condition"
	catalogSpell     = "spell"
	catalogMaster    = "master"
)

// catalogKindOf says which part of the catalog a key names.
func catalogKindOf(key string) string {
	switch {
	case strings.HasPrefix(key, "condition:"):
		return catalogCondition
	case strings.HasPrefix(key, "spell:"):
		return catalogSpell
	}
	return catalogMaster
}

// conditionOfCatalog is the condition the master may put on a creature: all but exhaustion, whose
// levels have a way of their own.
func conditionOfCatalog(content *rules.Content, key string) (rules.ConditionInfo, error) {
	info, ok := content.ConditionInfo(key)
	if !ok || key == conditionExhaustion {
		return info, errNotMasterEffect()
	}
	return info, nil
}

// spellEffectOfCatalog is the effect a spell puts on its targets.
func spellEffectOfCatalog(content *rules.Content, key string) (*rules.EffectDef, error) {
	def, ok := content.CombatSpellEffect(key)
	if !ok {
		return nil, errNotMasterEffect()
	}
	return def, nil
}

// masterEffectOfCatalog is an effect of the catalog that is the master's own.
func masterEffectOfCatalog(content *rules.Content, key string) (*rules.EffectDef, error) {
	def, ok := content.CombatEffect(key)
	if !ok || def.Kind != "master" {
		return nil, errNotMasterEffect()
	}
	return def, nil
}

// chosenAudience is who reads the effect once the master has chosen: the choice when there is
// one, else the effect's own. A value the server does not know reads as the empty audience.
func chosenAudience(choice playv1.EffectAudience, own string) string {
	if choice == playv1.EffectAudience_EFFECT_AUDIENCE_UNSPECIFIED {
		return own
	}
	return map[playv1.EffectAudience]string{
		playv1.EffectAudience_EFFECT_AUDIENCE_ALL:   rules.EffectVisibilityPublic,
		playv1.EffectAudience_EFFECT_AUDIENCE_OWNER: rules.EffectVisibilityOwner,
	}[choice]
}
