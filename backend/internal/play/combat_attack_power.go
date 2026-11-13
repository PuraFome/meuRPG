package play

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	rulesv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/rules/v1"
	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
)

// The power attack of a table feature (the power_attack effect, rules/powerattack.go) on a weapon attack of a player's
// character: the request names it (power_attack_key), the penalty comes off the attack roll and, when the effect has a
// resource, one use is spent with the attack. The server checks the key, the attacker, the weapon line and the uses
// itself; the options only list what it will accept.

// powerAttackEvent is the trade an attack made, kept on its event: the feature, and what it took off the roll and
// added to the damage.
type powerAttackEvent struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Penalty     int32  `json:"penalty"`
	DamageBonus int32  `json:"damage_bonus"`
}

// powerAttackRule is the engine's view of a trade of the sheet, for its weapon filter.
func powerAttackRule(p link.PowerAttack) rules.PowerAttack {
	return rules.PowerAttack{
		Key: p.Key, NamePT: p.NamePT, AttackPenalty: p.AttackPenalty, DamageBonus: p.DamageBonus, Weapon: p.Weapon,
		Properties: p.Properties, PropertyNamesPT: p.PropertyNamesPT, OnlyProficient: p.OnlyProficient, Resource: p.Resource,
	}
}

// powerAttackLine is the attack line of the sheet as the weapon filter reads it.
func powerAttackLine(a link.Attack) rules.Attack {
	line := rules.Attack{Melee: a.Melee, Properties: a.Properties, Proficient: a.Proficient}
	if a.Weapon && !a.Spell {
		line.Kind = "weapon"
	}
	return line
}

// checkPowerAttack checks the trade the request names goes on this attack: a weapon attack of a player's character
// that has the feature, whose weapon filter accepts the line, and a use of its resource left. Nothing is spent here.
func (f *attackFlow) checkPowerAttack(ctx context.Context) error {
	key := f.req.GetPowerAttackKey()
	if key == "" {
		return nil
	}
	if f.attacker.Kind != kindPlayer || !backedByCharacter(f.attacker) || f.attack.Spell || !f.attack.Weapon {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("a power attack goes on a weapon attack of a player's character"))
	}
	i := slices.IndexFunc(f.sheet.Traits.PowerAttacks, func(p link.PowerAttack) bool { return p.Key == key })
	if i < 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("power_attack_key is not a power attack of the character"))
	}
	p := f.sheet.Traits.PowerAttacks[i]
	if ok, why := powerAttackRule(p).Allows(powerAttackLine(f.attack)); !ok {
		return connect.NewError(connect.CodeInvalidArgument, errors.New(why))
	}
	if p.Resource != "" {
		vit, err := f.s.vitals.GetVitalsTx(ctx, f.c.tx, f.m.CampaignID, f.attacker.CharacterID)
		if err != nil {
			return err
		}
		if left, _ := poolLeft(vit, p.Resource); left <= 0 {
			return resourceBlocked(playv1.ResourceBlockedReason_RESOURCE_BLOCKED_REASON_NO_USES_LEFT, "no uses left")
		}
	}
	f.power = &p
	return nil
}

// powerPenalty is what the trade takes off the attack roll, 0 for an attack that makes none.
func (f *attackFlow) powerPenalty() int {
	if f.power == nil {
		return 0
	}
	return f.power.AttackPenalty
}

// spendPowerAttack spends the use of the trade's resource with the attack, and notes it on the attack's resources so
// the undo gives it back (the resource of a maneuver die is kept the same way). A held roll writes nothing until its
// answer runs the attack again, so it spends once, there.
func (f *attackFlow) spendPowerAttack(ctx context.Context) error {
	if f.power == nil || f.power.Resource == "" {
		return nil
	}
	if _, err := f.s.spendResource(ctx, f.c, f.attacker.CharacterID, f.power.Resource, 1); err != nil {
		return err
	}
	f.maneuverSpent = append(f.maneuverSpent, f.power.Resource)
	return nil
}

// powerEvent is the trade as the attack's event keeps it, nil when the attack made none.
func (f *attackFlow) powerEvent() *powerAttackEvent {
	p := f.power
	if p == nil {
		return nil
	}
	return &powerAttackEvent{Key: p.Key, Name: p.NamePT, Penalty: conv.Int32(p.AttackPenalty), DamageBonus: conv.Int32(p.DamageBonus)}
}

// listPowerAttacks lists, on each weapon attack option of a player's character, the trades its table features offer for
// that attack line (the ones whose weapon filter accepts it). An NPC, and a line the filter refuses, list none.
func (s *Service) listPowerAttacks(ctx context.Context, who playdb.Combatant, reads *turnReads, opts *rulesv1.TurnOptions) error {
	if who.Kind != kindPlayer || !backedByCharacter(who) || len(opts.GetAttacks()) == 0 {
		return nil
	}
	sheet, err := reads.sheetOf(ctx)
	if err != nil {
		return s.dbError(ctx, "read the character's power attacks", err)
	}
	if len(sheet.Traits.PowerAttacks) == 0 {
		return nil
	}
	for _, o := range opts.GetAttacks() {
		i := slices.IndexFunc(sheet.Attacks, func(a link.Attack) bool { return a.Key == o.GetAttack().GetKey() })
		if i < 0 {
			continue
		}
		line := powerAttackLine(sheet.Attacks[i])
		for _, p := range sheet.Traits.PowerAttacks {
			if ok, _ := powerAttackRule(p).Allows(line); ok {
				o.PowerAttacks = append(o.PowerAttacks, &rulesv1.PowerAttackOption{
					Key: p.Key, Name: p.NamePT, Penalty: conv.Int32(p.AttackPenalty), DamageBonus: conv.Int32(p.DamageBonus),
				})
			}
		}
	}
	return nil
}
