package play

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// The effects on a character count in every roll it makes (RN-22), in a combat and out of it:
// the checks of an RP scene, a puzzle's hint, the search for traps, the saving throws a scene
// asks. Out of a combat the character's record is a row of character_effects.

// outsideModeOf works out the mode, the dice and the hidden origins of a roll of a character
// that is not a combatant of a running combat.
func (s *Service) outsideModeOf(ctx context.Context, tx pgx.Tx, campaignID, characterID, skillKey, ability string, check bool) (checkMode, error) {
	rows, err := s.queriesIn(tx).ListCharacterEffects(ctx, []string{characterID})
	if err != nil {
		return checkMode{}, fmt.Errorf("list the character's effects: %w", err)
	}
	level := 0
	if v, err := s.vitals.GetVitalsTx(ctx, tx, campaignID, characterID); err == nil {
		level = int(v.GetExhaustionLevel())
	} else if connect.CodeOf(err) != connect.CodeNotFound {
		return checkMode{}, err
	}
	if len(rows) == 0 && level == 0 {
		return checkMode{}, nil
	}
	creature := combat.Creature{Exhaustion: level}
	var hiddenConds []string
	hiddenAny := false
	applies := rules.RollAppliesSave
	if check {
		applies = rules.RollAppliesCheck
	}
	var diceOf []effectDie
	var bonuses []effectBonus
	for _, r := range rows {
		mods := modifiersOf(r.Modifiers)
		if n := combat.CheckBonus(mods, skillKey); n != 0 && check {
			bonuses = append(bonuses, effectBonus{Key: r.SourceKey, Value: n, Hidden: !r.PlayerVisible})
		}
		creature.Conditions = append(creature.Conditions, r.ConditionKeys...)
		for _, m := range mods {
			if m.Kind == rules.ModifierSaveAdvantage {
				creature.SaveAdvantage = append(creature.SaveAdvantage, m.Abilities...)
			}
		}
		for _, d := range combat.EffectDice(r.SourceKey, mods, applies) {
			diceOf = append(diceOf, effectDie{Key: d.Source, Faces: d.Faces, Sign: d.Sign, Hidden: !r.PlayerVisible})
		}
		if !r.PlayerVisible {
			hiddenAny = true
			hiddenConds = append(hiddenConds, r.ConditionKeys...)
		}
	}
	sources := combat.SaveMode(combat.SaveScene{Creature: creature, Ability: ability, Check: check, EffectVisible: true})
	return checkMode{Mode: combat.Resolve(sources), Sources: sources, Dice: diceOf, Bonuses: bonuses, hide: hideFor(hiddenConds, hiddenAny)}, nil
}

// hideFor says which sources come from an effect the master hides.
func hideFor(conds []string, anyHidden bool) func(combat.Source) bool {
	if len(conds) == 0 && !anyHidden {
		return nil
	}
	return func(src combat.Source) bool {
		switch src.Kind {
		case combat.SourceOutlinedTarget, combat.SourceEffectSave:
			return anyHidden
		}
		return src.Condition != "" && slices.Contains(conds, src.Condition)
	}
}

// modifiersOf reads the modifiers of an effect's record.
func modifiersOf(body []byte) []rules.EffectModifier {
	st := playdb.CombatantState{Modifiers: body}
	return effectModifiers(st)
}

// withEffectDice rolls the dice the effects add to the roll (the app rolls them) and returns the
// bonus with them: Bênção's d4 on a saving throw. The dice and where they came from are kept for
// the sources of the roll.
func (s *Service) withEffectDice(cm *checkMode, bonus int) (int, error) {
	for _, b := range cm.Bonuses {
		bonus += b.Value
	}
	if len(cm.Dice) == 0 {
		return bonus, nil
	}
	rolled, delta, err := s.rollEffectDice(rollInput{inApp: true}, cm.Dice, nil)
	if err != nil {
		return bonus, err
	}
	cm.Rolled = rolled
	return bonus + delta, nil
}

// shownCheck writes the sources of a check or a saving throw: the mode's, and each die an
// effect added ("Bênção +1d4: 3"). What an effect the master hides gives reads "Outra fonte".
func (cm checkMode) shownCheck(names func(string) string) []*playv1.AdvantageSource {
	out := shownSourcesHiding(cm.Sources, names, cm.hide)
	for _, b := range cm.Bonuses {
		name := names(b.Key)
		if b.Hidden {
			name = "Outra fonte"
		}
		out = append(out, &playv1.AdvantageSource{
			Kind: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_DIE, TextPt: fmt.Sprintf("%s %+d", name, b.Value),
		})
	}
	for _, d := range cm.Rolled {
		name, sign := names(d.Key), "+"
		if d.Sign < 0 {
			sign = "-"
		}
		if d.Hidden {
			name = "Outra fonte"
		}
		out = append(out, &playv1.AdvantageSource{
			Kind: playv1.AdvantageSourceKind_ADVANTAGE_SOURCE_KIND_EFFECT_DIE, TextPt: fmt.Sprintf("%s %s1d%d: %d", name, sign, d.Faces, d.Face),
		})
	}
	return out
}

var _ = errors.New

// effectBonus is a fixed bonus an effect adds to a check.
type effectBonus struct {
	Key    string
	Value  int
	Hidden bool
}
