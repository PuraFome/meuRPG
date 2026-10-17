package play

import (
	"context"
	"fmt"

	"github.com/PuraFome/meuRPG/backend/internal/platform/conv"
	"github.com/PuraFome/meuRPG/backend/internal/play/link"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// affectZone is what the cast of a spell that keeps an area does: the area goes on the map as a
// zone. Web also keeps what it always did to the creatures inside (the effect on each), so the
// targets are hit as for any placed area; the other three hit nobody at the cast.
func (f *castFlow) affectZone(ctx context.Context, def zoneDef) error {
	if def.keepsCastHits {
		for i, t := range f.targs {
			if err := f.hitTarget(ctx, i, t); err != nil {
				return err
			}
		}
		if err := f.s.applyLastingSpell(ctx, f.c, f.cs, f.sp, f.slotLevel, f.caster, f.targs, f.abilityKey, &f.out.made); err != nil {
			return err
		}
	}
	return f.s.createZone(ctx, f.c, f.caster, f.sp, f.slotLevel, f.out.made.CastID, f.plan)
}

// createZone puts the area of the cast on the map. It runs after the concentration is set, so the zone
// the caster held before has already gone with it.
func (s *Service) createZone(ctx context.Context, c *combatTx, owner playdb.Combatant, sp link.Spell, slotLevel int, castID string, plan areaPlan) error {
	def, ok := zoneDefOf(sp.Key)
	if !ok {
		return nil
	}
	content, err := s.contentOf(ctx, c)
	if err != nil {
		return err
	}
	var ends *int32
	if rounds := content.SpellEffectRounds(sp.Key); rounds > 0 {
		ends = new(c.enc.Round + conv.Int32(rounds))
	}
	dice, dtype := zoneDamageOf(sp)
	dc := sp.CasterDC
	if dc == 0 {
		dc = sp.SaveDC
	}
	if _, err := c.q.InsertSpellZone(ctx, playdb.InsertSpellZoneParams{
		EncounterID: c.enc.ID, OwnerID: owner.ID, SpellKey: sp.Key, CastID: castID, SlotLevel: clamp32(slotLevel, 0, 9),
		OriginCol: clamp32(plan.origin.Col, 0, 1<<30), OriginRow: clamp32(plan.origin.Row, 0, 1<<30), Squares: squaresToJSON(plan.squares),
		SaveAbility: def.save, SaveDc: clamp32(dc, 0, 40), DamageDice: dice, DamageType: dtype,
		Concentration: sp.Concentration, StartedRound: c.enc.Round, EndsRound: ends, CreatedAt: c.now,
	}); err != nil {
		return fmt.Errorf("put the spell zone on the map: %w", err)
	}
	return nil
}

// zoneDamageOf are the dice and the type of the damage a zone deals, as the cast's slot and the
// caster's pick made them; empty for a zone with no damage.
func zoneDamageOf(sp link.Spell) (dice, dtype string) {
	if len(sp.Damages) == 0 {
		return "", ""
	}
	d := sp.Damages[0]
	return fmt.Sprintf("%dd%d", d.Count, d.Sides), d.DamageType
}

// endZonesOf ends the zones a combatant's concentration holds.
func (s *Service) endZonesOf(ctx context.Context, c *combatTx, owner playdb.Combatant) error {
	if _, err := c.q.DeleteConcentrationZonesOf(ctx, owner.ID); err != nil {
		return fmt.Errorf("end the spell zones: %w", err)
	}
	return nil
}
