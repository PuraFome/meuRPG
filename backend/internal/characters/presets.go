package characters

import (
	"context"
	"strconv"

	"connectrpc.com/connect"

	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The trap and light presets (MR-035, MR-036) are public SRD rules, like the
// creatures: nothing is read from the database, these methods check access and
// turn package rules' answers into messages. The presets only fill the
// master's form on the map editor; a trap or light on a map keeps its own copy
// of the numbers (package maps).

// ListTrapPresets implements rulesv1connect.ContentServiceHandler.
func (s *Service) ListTrapPresets(
	ctx context.Context,
	req *connect.Request[rulesv1.ListTrapPresetsRequest],
) (*connect.Response[rulesv1.ListTrapPresetsResponse], error) {
	// Active members only: a pending member needs no trap (RN-15).
	if _, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	res := &rulesv1.ListTrapPresetsResponse{}
	for _, p := range s.rules.TrapPresets() {
		res.Presets = append(res.Presets, s.trapPresetToProto(p))
	}
	for _, v := range s.rules.TrapSeverities() {
		res.Severities = append(res.Severities, &rulesv1.TrapSeverity{
			Key: v.Key, NamePt: v.NamePT,
			SaveDcMin: i32(v.SaveDC.Min), SaveDcMax: i32(v.SaveDC.Max),
			AttackBonusMin: i32(v.AttackBonus.Min), AttackBonusMax: i32(v.AttackBonus.Max),
		})
	}
	for _, r := range s.rules.TrapDamageByLevel() {
		res.DamageByLevel = append(res.DamageByLevel, &rulesv1.TrapDamageRow{
			FromLevel: i32(r.FromLevel), ToLevel: i32(r.ToLevel),
			Setback: trapDiceText(r.Setback), Dangerous: trapDiceText(r.Dangerous), Deadly: trapDiceText(r.Deadly),
		})
	}
	return connect.NewResponse(res), nil
}

// ListLightPresets implements rulesv1connect.ContentServiceHandler.
func (s *Service) ListLightPresets(
	ctx context.Context,
	req *connect.Request[rulesv1.ListLightPresetsRequest],
) (*connect.Response[rulesv1.ListLightPresetsResponse], error) {
	if _, err := authz.RequireCampaignMember(ctx, req.Msg.GetCampaignId()); err != nil {
		return nil, err
	}
	res := &rulesv1.ListLightPresetsResponse{}
	for _, l := range s.rules.LightPresets() {
		res.Presets = append(res.Presets, &rulesv1.LightPreset{
			Key: l.Key, NamePt: l.NamePT, BrightFt: i32(l.BrightFt), DimFt: i32(l.DimFt), DurationPt: l.DurationPT,
		})
	}
	return connect.NewResponse(res), nil
}

var (
	trapTriggerToProto = map[string]rulesv1.TrapTrigger{
		rules.TrapTriggerEnter:  rulesv1.TrapTrigger_TRAP_TRIGGER_ENTER,
		rules.TrapTriggerManual: rulesv1.TrapTrigger_TRAP_TRIGGER_MANUAL,
	}
	trapTargetsToProto = map[string]rulesv1.TrapTargets{
		rules.TrapTargetsArea:   rulesv1.TrapTargets_TRAP_TARGETS_AREA,
		rules.TrapTargetsManual: rulesv1.TrapTargets_TRAP_TARGETS_MANUAL,
	}
	trapAppliesToProto = map[string]rulesv1.TrapSaveApplies{
		rules.TrapSaveCaught: rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_CAUGHT,
		rules.TrapSaveHit:    rulesv1.TrapSaveApplies_TRAP_SAVE_APPLIES_HIT,
	}
	trapPassToProto = map[string]rulesv1.TrapPassOutcome{
		rules.TrapPassHalf: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_HALF,
		rules.TrapPassNone: rulesv1.TrapPassOutcome_TRAP_PASS_OUTCOME_NONE,
	}
)

// trapDiceText writes a damage formula as the API says it: "2d6", or a flat number
// "1".
func trapDiceText(d rules.DiceFormula) string {
	if d.Count == 0 {
		return strconv.Itoa(d.Bonus)
	}
	return strconv.Itoa(d.Count) + "d" + strconv.Itoa(d.Sides)
}

func (s *Service) trapDamageToProto(d rules.TrapDamage) *rulesv1.TrapDamage {
	return &rulesv1.TrapDamage{Dice: trapDiceText(d.Dice), DamageTypeKey: d.Type, DamageTypePt: s.rules.NamePT(d.Type)}
}

func (s *Service) trapDamagesToProto(list []rules.TrapDamage) []*rulesv1.TrapDamage {
	var out []*rulesv1.TrapDamage
	for _, d := range list {
		out = append(out, s.trapDamageToProto(d))
	}
	return out
}

func (s *Service) trapConditionToProto(key, duration string) *rulesv1.TrapCondition {
	return &rulesv1.TrapCondition{ConditionKey: key, ConditionPt: s.rules.NamePT(key), DurationPt: duration}
}

// trapPresetToProto copies a preset, with the effect in parts as the map's trap
// keeps it.
func (s *Service) trapPresetToProto(p rules.TrapPreset) *rulesv1.TrapPreset {
	effect := &rulesv1.TrapEffect{
		Damage:  s.trapDamagesToProto(p.Damage),
		Targets: trapTargetsToProto[p.Targets],
	}
	for _, c := range p.Conditions {
		effect.Conditions = append(effect.Conditions, s.trapConditionToProto(c.Condition, c.DurationPT))
	}
	if a := p.Attack; a != nil {
		effect.Attack = &rulesv1.TrapAttack{Bonus: i32(a.Bonus), Count: i32(a.Count), Damage: s.trapDamageToProto(a.Damage)}
	}
	if sv := p.Save; sv != nil {
		onFail := &rulesv1.TrapOnFail{Damage: s.trapDamagesToProto(sv.OnFail.Damage)}
		if sv.OnFail.Condition != "" {
			onFail.Condition = s.trapConditionToProto(sv.OnFail.Condition, sv.OnFail.DurationPT)
		}
		effect.Save = &rulesv1.TrapSaveEffect{
			Ability: abilityToProto[sv.Ability], Dc: i32(sv.DC), AppliesTo: trapAppliesToProto[sv.AppliesTo],
			OnFail: onFail, OnPass: trapPassToProto[sv.OnPass],
		}
	}
	return &rulesv1.TrapPreset{
		Key: p.Key, NamePt: p.NamePT, Kind: p.Kind, DescriptionPt: p.DescriptionPT,
		NoticeDc: i32(p.NoticeDC), FindDc: i32(p.FindDC), FindDcIsOurs: p.FindDCIsOurs,
		Trigger: trapTriggerToProto[p.Trigger], AreaSize: i32(p.AreaSize), FallFt: i32(p.FallFt), Effect: effect,
	}
}
