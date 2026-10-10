package play

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	// Bonuses are the fixed bonuses effects add to the check (Pass without Trace).
	Bonuses []effectBonus
	// Helps are the Helps that gave the check advantage: each is used up by the roll
	// (SRD 5.1, "Help"), see spendCheckHelps.
	Helps []string
	hide  func(combat.Source) bool
}

// checkModeOf works out the mode of a check or a saving throw of a player's character.
// The character's combatant is read in the session's running combat, if there is one.
func (s *Service) checkModeOf(ctx context.Context, tx pgx.Tx, campaignID, sessionID, characterID, skillKey string, ability string, check bool) (checkMode, error) {
	cm, err := s.baseCheckModeOf(ctx, tx, campaignID, sessionID, characterID, skillKey, ability, check)
	if err != nil || !check || skillKey != skillStealth {
		return cm, err
	}
	// Armor that gives disadvantage on Dexterity (Stealth) counts in every Stealth check,
	// in a combat or out of it (SRD 5.1, "Armor"). A combat read it already.
	if slices.ContainsFunc(cm.Sources, func(src combat.Source) bool { return src.Kind == combat.SourceStealthArmor }) {
		return cm, nil
	}
	sheet, err := s.roster.CombatSheet(ctx, tx, campaignID, characterID)
	if err != nil || !sheet.Traits.StealthDisadvantage {
		return cm, nil //nolint:nilerr // a sheet that cannot be read adds no circumstance
	}
	cm.Sources = append(slices.Clone(cm.Sources), combat.Source{Kind: combat.SourceStealthArmor, Effect: combat.ModeDisadvantage})
	cm.Mode = combat.Resolve(cm.Sources)
	return cm, nil
}

func (s *Service) baseCheckModeOf(ctx context.Context, tx pgx.Tx, campaignID, sessionID, characterID, skillKey string, ability string, check bool) (checkMode, error) {
	q := s.queriesIn(tx)
	enc, err := q.GetLatestEncounter(ctx, sessionID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && enc.Status == statusEnded) {
		return s.outsideModeOf(ctx, tx, campaignID, characterID, skillKey, ability, check)
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
		if c.CharacterID == characterID && c.Kind == kindPlayer {
			who, found = c, true
		}
	}
	if !found {
		return s.outsideModeOf(ctx, tx, campaignID, characterID, skillKey, ability, check)
	}
	states, err := s.readStates(ctx, tx, enc.ID)
	if err != nil {
		return checkMode{}, err
	}
	sheet, err := s.roster.CombatSheet(ctx, tx, campaignID, characterID)
	if err != nil {
		return checkMode{}, err
	}
	sources := combat.SaveMode(combat.SaveScene{
		Creature: creatureFacts(who, states, sheet.Traits), Ability: ability, Check: check, EffectVisible: true,
		StealthArmor: skillKey == skillStealth && sheet.Traits.StealthDisadvantage,
	})
	// A Help with this check gives advantage on it, whatever rolls it (SRD 5.1, "Help").
	var helpIDs []string
	if check {
		helps, err := s.liveHelps(ctx, q, enc, cs)
		if err != nil {
			return checkMode{}, err
		}
		for _, h := range helps {
			if h.Kind == helpCheck && helpsAlly(h, who) && deref(h.Task) == skillKey {
				sources = append(sources, combat.Source{Kind: combat.SourceHelp, Effect: combat.ModeAdvantage})
				helpIDs = append(helpIDs, h.ID)
			}
		}
	}
	applies := rules.RollAppliesSave
	if check {
		applies = rules.RollAppliesCheck
	}
	var bonuses []effectBonus
	for _, st := range effectsOn(states, who.ID) {
		if n := combat.CheckBonus(effectModifiers(st), skillKey); n != 0 && check {
			bonuses = append(bonuses, effectBonus{Key: deref(st.SourceKey), Value: n, Hidden: !st.PlayerVisible})
		}
	}
	diceOf := effectDiceFor(states, who.ID, applies)
	for i := range diceOf {
		diceOf[i].CharacterID = characterID
	}
	return checkMode{
		Mode: combat.Resolve(sources), Sources: sources, Dice: diceOf, Bonuses: bonuses, Helps: helpIDs,
		hide: hideFor(hiddenConditionsOf(states, who), hasHiddenEffect(states, who.ID)),
	}, nil
}

// spendCheckHelps uses up the Helps that gave a check advantage: the ally's next check for
// the task is the one the Help covers, whichever path rolls it (SRD 5.1, "Help").
func (s *Service) spendCheckHelps(ctx context.Context, c *combatTx, ids []string) error {
	for _, id := range ids {
		if err := c.q.SetHelpConsumed(ctx, playdb.SetHelpConsumedParams{ID: id, ConsumedAt: &c.now}); err != nil {
			return fmt.Errorf("use the help: %w", err)
		}
	}
	return nil
}

// notesOfSources writes the circumstances behind a check's mode as the notes a roll keeps.
func notesOfSources(sources []combat.Source, hide func(combat.Source) bool) []rollNote {
	var out []rollNote
	for _, src := range sources {
		label := textContext{self: true, names: func(k string) string { return k }}.sentence(src)
		if hide != nil && hide(src) {
			label = "Outra fonte" // an effect the master hides (RN-22)
		}
		out = append(out, rollNote{Kind: src.Kind, Label: label, Adv: src.Effect == combat.ModeAdvantage})
	}
	return out
}

// checkModeOfRoll is the mode the pure rules resolved, as a check's roll reads it.
func checkModeOfRoll(m combat.RollMode) combat.CheckMode {
	switch m {
	case combat.ModeAdvantage:
		return combat.CheckAdvantage
	case combat.ModeDisadvantage:
		return combat.CheckDisadvantage
	}
	return combat.CheckNormal
}
