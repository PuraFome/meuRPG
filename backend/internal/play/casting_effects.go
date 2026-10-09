package play

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
	"uuid"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/authz"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
	"github.com/PuraFome/meuRPG/backend/internal/rules"
	"github.com/PuraFome/meuRPG/backend/internal/rules/combat"
)

// What a cast does when it takes effect (SRD 5.1, each spell's own text). The server
// applies what the engine knows: the slot, the concentration, the hit points of a
// healing spell (Cure Wounds, Healing Word, Prayer of Healing, Heal and the others) and
// of False Life, the armor class of Mage Armor and the creatures of a summoning
// spell. Anything else is a cast the log records and the master narrates.

// stub is the combatant a character that is not in a combat stands for in the
// helpers of the combat's hit points (healCombatant, readFxTarget).
func stub(ch link.Character) playdb.Combatant {
	return playdb.Combatant{Kind: kindPlayer, CharacterID: ch.ID}
}

// castRolled is the dice a cast rolled.
type castRolled struct {
	count, sides int32
	faces        []int32
	total        int32
	physical     bool
}

// takeEffect makes the cast take effect: it spends the slot, ends the caster's other
// concentration, applies the spell's effect and closes the row (it lasts, or it is
// over). The row exists already, in the casting state.
func (s *Service) takeEffect(ctx context.Context, c *combatTx, m authz.Membership, plan castPlan) (castOutcome, error) {
	g, osp := plan.g, plan.osp
	out := castOutcome{guard: g}

	// The slot goes now, not at the start of a long casting (SRD 5.1, "Longer Casting
	// Times": a casting that fails spends none). An NPC keeps no slots.
	if plan.slot != nil && g.caster.Player {
		v, err := s.spendSlot(ctx, c, g.caster.ID, *plan.slot, 1)
		if err != nil {
			return out, err
		}
		out.vitals = append(out.vitals, v)
	}

	// A concentration spell ends the caster's other (SRD 5.1, "Duration": a creature
	// can't concentrate on two spells at once).
	if osp.Spell.Concentration {
		ended, dismissed, vitals, err := s.endOtherConcentration(ctx, c, g.caster, plan.row.ID)
		if err != nil {
			return out, err
		}
		out.ended, out.dismissed = append(out.ended, ended...), append(out.dismissed, dismissed...)
		out.vitals = append(out.vitals, vitals...)
	}

	var rolled castRolled
	var err error
	switch osp.Effect {
	case link.EffectHeal:
		rolled, out.targets, err = s.healTargets(ctx, c, plan, &out)
	case link.EffectTempHP:
		rolled, out.targets, err = s.tempHPTargets(ctx, c, plan, &out)
	case link.EffectArmorClass:
		out.targets, err = s.armorTargets(ctx, c, m, plan)
	case link.EffectSummon:
		out.targets = narratedTargets(plan)
		if !osp.NPC {
			err = s.summonFor(ctx, c, m, plan, &out)
		}
	default:
		out.targets = narratedTargets(plan)
	}
	if err != nil {
		return out, err
	}

	// It lasts, or it is over (SRD 5.1, "Duration").
	lasts := osp.Lasts || osp.Spell.Concentration
	now := c.now
	status, reason, endedAt := castActive, (*string)(nil), (*time.Time)(nil)
	if !lasts {
		status, reason, endedAt = castEnded, ptr(endInstant), &now
	}
	targets, err := json.Marshal(nonNilTargets(out.targets))
	if err != nil {
		return out, fmt.Errorf("encode the cast's targets: %w", err)
	}
	row, err := c.q.FinishSpellCast(ctx, playdb.FinishSpellCastParams{
		ID: plan.row.ID, Status: status, EndReason: reason, EndedAt: endedAt,
		Concentrating: lasts && osp.Spell.Concentration, Targets: targets,
		DiceCount: rolled.count, DiceSides: rolled.sides, RollFaces: nonNilFaces(rolled.faces), RollTotal: rolled.total,
		Physical: rolled.physical, CreatureIds: nonNil(out.created), CastAt: &now,
		SlotLevel: castSlotLevel(plan.slot), SlotPact: plan.slot != nil && plan.slot.Pact,
	})
	if err != nil {
		return out, fmt.Errorf("finish the cast: %w", err)
	}
	out.row = row
	out.ev = out.event()
	return out, nil
}

func ptr[T any](v T) *T { return &v }

func nonNilTargets(t []castTarget) []castTarget {
	if t == nil {
		return []castTarget{}
	}
	return t
}

func nonNilFaces(f []int32) []int32 {
	if f == nil {
		return []int32{}
	}
	return f
}

// narratedTargets lists the targets of a cast the server applies nothing to: the log
// still says who the master was told it reached.
func narratedTargets(plan castPlan) []castTarget {
	var out []castTarget
	for _, t := range plan.g.targets {
		out = append(out, castTarget{ID: t.ID, Effect: castNarrated})
	}
	if len(out) == 0 && selfOnly(plan.osp.Spell) {
		out = append(out, castTarget{ID: plan.g.caster.ID, Effect: castNarrated})
	}
	return out
}

// levelOf is the level the spell has in this casting: the slot's, or its own (SRD
// 5.1, "Casting a Spell at a Higher Level").
func levelOf(plan castPlan) int {
	if plan.slot != nil {
		return int(plan.slot.Level)
	}
	return plan.osp.Spell.Level
}

// healTargets heals each player character among the targets. The spell's dice are one
// roll for the whole cast (as a combat's heal is); the Life Domain adds to it (SRD
// 5.1, Cleric: Life Domain): Disciple of Life to each target, Supreme Healing makes
// the dice their highest number, and Blessed Healer heals the caster once when someone
// else regained hit points. An NPC target changes nothing: it has no hit points
// outside a combat, and the master narrates.
func (s *Service) healTargets(ctx context.Context, c *combatTx, plan castPlan, out *castOutcome) (castRolled, []castTarget, error) {
	osp := plan.osp
	h := combat.HealFeatures{Disciple: osp.Healing.Disciple, Blessed: osp.Healing.Blessed, Supreme: osp.Healing.Supreme}
	var rolled castRolled
	amount := 0
	switch {
	case osp.Spell.HP != nil && osp.Spell.HP.Kind == rules.SpellKindFlatHeal:
		amount = osp.Spell.HP.Heal // Heal restores a fixed number
	case osp.Spell.Heal != nil:
		d := osp.Spell.Heal
		sum := 0
		if d.Count > 0 && !h.Supreme {
			r, err := s.poolRoll(link.Dice{Count: d.Count, Sides: d.Sides}, plan.in)
			if err != nil {
				return rolled, nil, err
			}
			sum, rolled.faces, rolled.physical = r.Total, faces32(r.Faces), r.Physical
		}
		if d.Count > 0 && h.Supreme {
			rolled.faces = make([]int32, d.Count)
			for i := range rolled.faces {
				rolled.faces[i] = clamp32(d.Sides, 0, math.MaxInt32)
			}
		}
		amount = h.HealTotal(d.Count, d.Sides, d.Bonus, sum)
		rolled.count, rolled.sides = clamp32(d.Count, 0, 100), clamp32(d.Sides, 0, 100)
	}
	amount = max(amount, 0)
	rolled.total = clamp32(amount, 0, math.MaxInt32)
	level := levelOf(plan)
	extra := h.TargetExtra(level)

	var targets []castTarget
	healedOther := false
	for _, t := range plan.g.targets {
		if !t.Player {
			targets = append(targets, castTarget{ID: t.ID, Effect: castNarrated})
			continue
		}
		hit, v, err := s.healCombatant(ctx, c, stub(t), clamp32(amount+extra, 0, math.MaxInt32))
		if err != nil {
			return rolled, nil, err
		}
		out.vitals = append(out.vitals, v)
		ct := castTarget{ID: t.ID, Effect: castHeal, Amount: hit.Amount, Extra: clamp32(extra, 0, math.MaxInt32)}
		if hit.Before != nil && hit.After != nil {
			ct.Before, ct.After = hit.Before.HP, hit.After.HP
		}
		targets = append(targets, ct)
		healedOther = healedOther || (t.ID != plan.g.caster.ID && hit.Amount > 0)
	}
	if len(plan.g.targets) == 0 && selfOnly(osp.Spell) && plan.g.caster.Player {
		hit, v, err := s.healCombatant(ctx, c, stub(plan.g.caster), clamp32(amount+extra, 0, math.MaxInt32))
		if err != nil {
			return rolled, nil, err
		}
		out.vitals = append(out.vitals, v)
		ct := castTarget{ID: plan.g.caster.ID, Effect: castHeal, Amount: hit.Amount, Extra: clamp32(extra, 0, math.MaxInt32)}
		if hit.Before != nil && hit.After != nil {
			ct.Before, ct.After = hit.Before.HP, hit.After.HP
		}
		targets = append(targets, ct)
	}
	// Blessed Healer: once for the casting, when a creature other than the caster regained
	// hit points.
	if self := h.SelfExtra(level, healedOther); self > 0 && plan.g.caster.Player {
		hit, v, err := s.healCombatant(ctx, c, stub(plan.g.caster), clamp32(self, 0, math.MaxInt32))
		if err != nil {
			return rolled, nil, err
		}
		out.vitals = append(out.vitals, v)
		if i := indexOfTarget(targets, plan.g.caster.ID); i >= 0 {
			targets[i].Amount += hit.Amount
			if hit.After != nil {
				targets[i].After = hit.After.HP
			}
		} else {
			ct := castTarget{ID: plan.g.caster.ID, Effect: castHeal, Amount: hit.Amount, Blessed: true}
			if hit.Before != nil && hit.After != nil {
				ct.Before, ct.After = hit.Before.HP, hit.After.HP
			}
			targets = append(targets, ct)
		}
	}
	return rolled, targets, nil
}

func indexOfTarget(targets []castTarget, id string) int {
	for i, t := range targets {
		if t.ID == id {
			return i
		}
	}
	return -1
}

// tempHPTargets gives the temporary hit points of False Life: the dice plus the fixed
// amount. They do not stack: a target keeps the larger of what it had and what the
// spell gives (SRD 5.1, "Temporary Hit Points").
func (s *Service) tempHPTargets(ctx context.Context, c *combatTx, plan castPlan, out *castOutcome) (castRolled, []castTarget, error) {
	fx := plan.osp.Spell.HP
	var rolled castRolled
	gained := fx.Amount
	if fx.Pool.Count > 0 {
		r, err := s.poolRoll(link.Dice{Count: fx.Pool.Count, Sides: fx.Pool.Sides}, plan.in)
		if err != nil {
			return rolled, nil, err
		}
		gained += r.Total
		rolled = castRolled{count: clamp32(fx.Pool.Count, 0, 100), sides: clamp32(fx.Pool.Sides, 0, 100), faces: faces32(r.Faces), physical: r.Physical}
	}
	rolled.total = clamp32(gained, 0, math.MaxInt32)
	var targets []castTarget
	for _, t := range s.effectTargetsAll(plan) {
		if !t.Player {
			targets = append(targets, castTarget{ID: t.ID, Effect: castNarrated})
			continue
		}
		ft, err := s.readFxTarget(ctx, c, stub(t))
		if err != nil {
			return rolled, nil, err
		}
		var hit castHit
		v, err := s.giveTempHP(ctx, c, ft, gained, &hit)
		if err != nil {
			return rolled, nil, err
		}
		out.vitals = append(out.vitals, v)
		ct := castTarget{ID: t.ID, Effect: castTempHP, Before: clamp32(ft.temp, 0, math.MaxInt32), After: clamp32(max(ft.temp, gained), 0, math.MaxInt32)}
		if hit.Healed != nil {
			ct.Amount = *hit.Healed
		}
		targets = append(targets, ct)
	}
	return rolled, targets, nil
}

// effectTargetsAll are the targets of an effect, the caster standing in for a spell that
// reaches only the caster: the NPCs too, which the effect passes over.
func (s *Service) effectTargetsAll(plan castPlan) []link.Character {
	if len(plan.g.targets) == 0 && selfOnly(plan.osp.Spell) {
		return []link.Character{plan.g.caster}
	}
	return plan.g.targets
}

// armorTargets puts Mage Armor on each player character among the targets (SRD 5.1:
// the base armor class becomes 13 + Dexterity for a creature that wears no armor).
// The sheet is not changed: a combat reads the armor class from the combatant, and
// the combatants of the character in a combat that is open take it now.
func (s *Service) armorTargets(ctx context.Context, c *combatTx, m authz.Membership, plan castPlan) ([]castTarget, error) {
	var targets []castTarget
	for _, t := range s.effectTargetsAll(plan) {
		if !t.Player {
			targets = append(targets, castTarget{ID: t.ID, Effect: castNarrated})
			continue
		}
		mage, err := s.roster.MageArmorAC(ctx, c.tx, m.CampaignID, t.ID)
		if err != nil {
			return nil, err
		}
		if !mage.Applies {
			targets = append(targets, castTarget{ID: t.ID, Effect: castNarrated})
			continue
		}
		ac := clamp32(mage.AC, 1, 60)
		if err := c.q.SetMageArmorACOfCharacter(ctx, playdb.SetMageArmorACOfCharacterParams{CharacterID: t.ID, MageArmorAc: &ac}); err != nil {
			return nil, fmt.Errorf("put Mage Armor on the combatants: %w", err)
		}
		targets = append(targets, castTarget{ID: t.ID, Effect: castArmor, AC: ac})
	}
	return targets, nil
}

// summonFor makes the creatures of a summoning spell, as PlayService.CastSummon does:
// the choice is checked against the slot and the sheet, the creatures are recorded
// for the caster, and a new familiar or a new concentration sends the old ones away.
func (s *Service) summonFor(ctx context.Context, c *combatTx, m authz.Membership, plan castPlan, out *castOutcome) error {
	circle := 0
	if plan.slot != nil {
		circle = int(plan.slot.Level)
	}
	chk, err := s.checkSummonChoice(ctx, c.tx, m.CampaignID, plan.g.caster.ID, plan.spell, circle, plan.pick)
	if err != nil {
		return err
	}
	if plan.ritual && (!chk.CanRitual || !chk.Ritual) {
		return badCast("the character cannot cast this spell as a ritual")
	}
	given, err := summonNames(plan.pick.GetNames(), len(chk.Creatures))
	if err != nil {
		return err
	}
	sum, err := s.roster.SummonCreatures(ctx, c.tx, link.Summon{
		CampaignID: m.CampaignID, CharacterID: plan.g.caster.ID, Source: summonSource(plan.spell), GroupID: uuid.New().String(),
		Concentration: chk.Concentration, Creatures: summonSpecs(chk.Creatures, given),
	})
	if err != nil {
		return err
	}
	for _, cr := range sum.Created {
		out.created = append(out.created, cr.ID)
	}
	was := c.characterID
	c.characterID = &plan.g.caster.ID
	defer func() { c.characterID = was }()
	for _, cr := range sum.Replaced {
		out.dismissed = append(out.dismissed, cr.ID)
		reason := "concentration"
		if cr.Source == "familiar" {
			reason = "replaced"
		}
		if err := insertEvent(ctx, c, eventCreatureDismissed, &m.UserID, nil, actionEvent{OwnerCharacter: plan.g.caster.ID, Created: []string{cr.ID}, Reason: reason}); err != nil {
			return err
		}
	}
	return nil
}

// takeBackMaxHP takes Aid's hit point bonus off the targets of a cast that ended.
func (s *Service) takeBackMaxHP(_ context.Context, _ *combatTx, _ playdb.SpellCast) ([]*playv1.CharacterVitals, error) {
	return nil, nil
}
