package play

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The mode of the ability checks and saving throws a player rolls outside the attacks
// of a turn: the checks of an RP scene, a puzzle's hint, the search for traps (SRD 5.1,
// "Advantage and Disadvantage", Conditions). The conditions and the states live on the
// combatants, so a character in a running combat has them, and one outside a combat
// rolls a normal d20.

// skillAbilities are the abilities of the SRD's skills.
var skillAbilities = map[string]string{
	"skill:acrobatics": "dex", "skill:animal-handling": "wis", "skill:arcana": "int", "skill:athletics": "str",
	"skill:deception": "cha", "skill:history": "int", "skill:insight": "wis", "skill:intimidation": "cha",
	"skill:investigation": "int", "skill:medicine": "wis", "skill:nature": "int", "skill:perception": "wis",
	"skill:performance": "cha", "skill:persuasion": "cha", "skill:religion": "int", "skill:sleight-of-hand": "dex",
	"skill:stealth": "dex", "skill:survival": "wis",
}

// checkKey reads the key of a scene action ("skill:investigation", "ability:str",
// "save:wis") as an ability and whether it is an ability check (a save is not).
func checkKey(key string) (ability string, check bool) {
	switch {
	case strings.HasPrefix(key, "skill:"):
		return skillAbilities[key], true
	case strings.HasPrefix(key, "ability:"):
		return strings.TrimPrefix(key, "ability:"), true
	case strings.HasPrefix(key, "save:"):
		return strings.TrimPrefix(key, "save:"), false
	}
	return "", true
}

// checkMode is the mode of a check and the circumstances behind it.
type checkMode struct {
	Mode    combat.RollMode
	Sources []combat.Source
	// Dice are the dice effects add to the roll (Bênção on a saving throw), Rolled the same
	// with their faces once rolled; hide says which sources come from an effect the master hides.
	Dice, Rolled []effectDie
	hide         func(combat.Source) bool
}

// checkModeOf works out the mode of a check or a saving throw of a player's character.
// The character's combatant is read in the session's running combat, if there is one.
func (s *Service) checkModeOf(ctx context.Context, tx pgx.Tx, campaignID, sessionID, characterID, ability string, check bool) (checkMode, error) {
	q := s.queriesIn(tx)
	enc, err := q.GetLatestEncounter(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && enc.Status == statusEnded) {
		return s.outsideModeOf(ctx, tx, campaignID, characterID, ability, check)
	}
	if err != nil {
		return checkMode{}, err
	}
	cs, err := q.ListCombatants(ctx, enc.ID)
	if err != nil {
		return checkMode{}, err
	}
	var who playdb.Combatant
	found := false
	for _, c := range cs {
		if c.CharacterID == characterID && c.Kind == kindPlayer && !c.Defeated {
			who, found = c, true
		}
	}
	if !found {
		return s.outsideModeOf(ctx, tx, campaignID, characterID, ability, check)
	}
	states, err := s.readStates(ctx, tx, enc.ID)
	if err != nil {
		return checkMode{}, err
	}
	sheet, err := s.roster.CombatSheet(ctx, tx, campaignID, characterID)
	if err != nil {
		return checkMode{}, err
	}
	sources := combat.SaveMode(combat.SaveScene{Creature: creatureFacts(who, states, sheet.Traits), Ability: ability, Check: check, EffectVisible: true})
	applies := rules.RollAppliesSave
	if check {
		applies = rules.RollAppliesCheck
	}
	return checkMode{
		Mode: combat.Resolve(sources), Sources: sources, Dice: effectDiceFor(states, who.ID, applies),
		hide: hideFor(hiddenConditionsOf(states, who), hasHiddenEffect(states, who.ID)),
	}, nil
}
