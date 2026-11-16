package play

import (
	"context"

	playv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/play/v1"
	"github.com/PuraFome/meuRPG/backend/internal/play/playdb"
)

// The heal a spell takes from the damage it dealt (SRD 5.1, Vampiric Touch: the caster regains half the damage dealt).

// drainSpell is the spell whose damage heals its caster by half of it.
const drainSpell = "spell:vampiric-touch"

// drainHeal heals the caster of a drain spell by half the damage dealt.
// The half rounds down and the heal stops at the maximum.
// The heal is a line of the log, written just before the event of the damage.
// It answers the caster's vitals when the caster is a player's character.
// A damage of another spell, or one too small to heal, does nothing.
func (s *Service) drainHeal(ctx context.Context, c *combatTx, p playdb.PendingDamage, caster, target playdb.Combatant, dealt int32) (*playv1.CharacterVitals, error) {
	if p.AttackKey != drainSpell || p.Healing || dealt/2 <= 0 {
		return nil, nil
	}
	hit, vit, err := s.healCombatant(ctx, c, caster, dealt/2)
	if err != nil {
		return nil, err
	}
	ev := actionEvent{
		Round: c.enc.Round, Secret: secretOf(caster, target), Actor: caster.ID, Key: p.AttackKey, Heal: true, Amount: hit.Amount,
		Before: hit.Before, After: hit.After, DeathBefore: hit.DeathBefore, DrainOf: p.ID,
	}
	return vit, insertEvent(ctx, c, eventActionTaken, &c.actorUserID, nil, ev)
}
