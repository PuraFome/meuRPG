package characters

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// GetSpellDetails implements rulesv1connect.ContentServiceHandler. It is the
// long, per-spell counterpart of ListContent, which stays light.
func (s *Service) GetSpellDetails(
	ctx context.Context,
	req *connect.Request[rulesv1.GetSpellDetailsRequest],
) (*connect.Response[rulesv1.GetSpellDetailsResponse], error) {
	// The same access as ListContent: a pending member may look too (RN-15).
	if _, err := authz.RequireCampaignMemberOrPending(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	d, ok := s.rules.SpellDetails(req.Msg.GetSpellKey())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errUnknownSpell)
	}
	return connect.NewResponse(&rulesv1.GetSpellDetailsResponse{Spell: spellDetailsToProto(d)}), nil
}

var errUnknownSpell = errors.New("spell not found")

var (
	castingUnitToProto = map[string]rulesv1.CastingTimeUnit{
		rules.CastAction:      rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_ACTION,
		rules.CastBonusAction: rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_BONUS_ACTION,
		rules.CastReaction:    rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_REACTION,
		rules.CastMinute:      rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_MINUTE,
		rules.CastHour:        rulesv1.CastingTimeUnit_CASTING_TIME_UNIT_HOUR,
	}
	rangeKindToProto = map[string]rulesv1.SpellRangeKind{
		rules.RangeSelf:      rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SELF,
		rules.RangeTouch:     rulesv1.SpellRangeKind_SPELL_RANGE_KIND_TOUCH,
		rules.RangeRanged:    rulesv1.SpellRangeKind_SPELL_RANGE_KIND_RANGED,
		rules.RangeSight:     rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SIGHT,
		rules.RangeUnlimited: rulesv1.SpellRangeKind_SPELL_RANGE_KIND_UNLIMITED,
		rules.RangeSpecial:   rulesv1.SpellRangeKind_SPELL_RANGE_KIND_SPECIAL,
	}
	durationKindToProto = map[string]rulesv1.SpellDurationKind{
		rules.DurationInstantaneous:  rulesv1.SpellDurationKind_SPELL_DURATION_KIND_INSTANTANEOUS,
		rules.DurationTimed:          rulesv1.SpellDurationKind_SPELL_DURATION_KIND_TIMED,
		rules.DurationUntilDispelled: rulesv1.SpellDurationKind_SPELL_DURATION_KIND_UNTIL_DISPELLED,
		rules.DurationSpecial:        rulesv1.SpellDurationKind_SPELL_DURATION_KIND_SPECIAL,
	}
	durationUnitToProto = map[string]rulesv1.SpellDurationUnit{
		rules.DurationRound:  rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_ROUND,
		rules.DurationMinute: rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_MINUTE,
		rules.DurationHour:   rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_HOUR,
		rules.DurationDay:    rulesv1.SpellDurationUnit_SPELL_DURATION_UNIT_DAY,
	}
	spellAttackToProto = map[string]rulesv1.SpellAttackType{
		"":       rulesv1.SpellAttackType_SPELL_ATTACK_TYPE_NONE,
		"melee":  rulesv1.SpellAttackType_SPELL_ATTACK_TYPE_MELEE,
		"ranged": rulesv1.SpellAttackType_SPELL_ATTACK_TYPE_RANGED,
	}
	saveSuccessToProto = map[string]rulesv1.SpellSaveSuccess{
		"none":  rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_NONE,
		"half":  rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_HALF,
		"other": rulesv1.SpellSaveSuccess_SPELL_SAVE_SUCCESS_OTHER,
	}
)

func spellDetailsToProto(d *rules.SpellDetails) *rulesv1.SpellDetails {
	out := &rulesv1.SpellDetails{
		Spell: spellToProto(d.Spell),
		CastingTime: &rulesv1.SpellCastingTime{
			Amount: i32(d.CastingTime.Amount), Unit: castingUnitToProto[d.CastingTime.Unit],
			Trigger: d.CastingTime.Trigger, Raw: d.CastingTime.Raw,
		},
		Range: &rulesv1.SpellRange{Kind: rangeKindToProto[d.Range.Kind], DistanceFt: i32(d.Range.DistanceFt), Raw: d.Range.Raw},
		Components: &rulesv1.SpellComponents{
			Verbal: d.Components.Verbal, Somatic: d.Components.Somatic, Material: d.Components.Material,
			MaterialText: d.Components.MaterialText,
		},
		Duration: &rulesv1.SpellDuration{
			Kind: durationKindToProto[d.Duration.Kind], Amount: i32(d.Duration.Amount), Unit: durationUnitToProto[d.Duration.Unit],
			UpTo: d.Duration.UpTo, Concentration: d.Duration.Concentration, Raw: d.Duration.Raw,
		},
		AttackType:      spellAttackToProto[d.AttackType],
		HealBySlotLevel: levelsToProto(d.HealBySlotLevel),
		Description:     d.Description,
		HigherLevel:     d.HigherLevel,
	}
	if d.Save != nil {
		out.Save = &rulesv1.SpellSave{Ability: abilityToProto[d.Save.Ability], OnSuccess: saveSuccessToProto[d.Save.OnSuccess]}
	}
	for _, dm := range d.Damage {
		out.Damage = append(out.Damage, &rulesv1.SpellDamage{
			DamageTypeKey: dm.Type, DamageTypePt: dm.TypeNamePT,
			BySlotLevel: levelsToProto(dm.BySlotLevel), ByCharacterLevel: levelsToProto(dm.ByCharacterLevel),
		})
	}
	return out
}

func levelsToProto(m map[int]string) map[int32]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[int32]string, len(m))
	for l, v := range m {
		out[i32(l)] = v
	}
	return out
}
