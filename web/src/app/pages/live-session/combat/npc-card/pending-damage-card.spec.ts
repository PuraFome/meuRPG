import { PendingDamageStatus } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { effectLine, pendingCard, trapLabel } from './pending-damage-card';

describe('pending damage card', () => {
  const pending = (over: object = {}) =>
    ({
      id: 'p',
      attackerId: 'a',
      targetId: 't',
      status: PendingDamageStatus.ROLLED,
      diceCount: 2,
      diceSides: 6,
      bonus: 1,
      criticalMax: 0,
      amount: 5,
      damageTypeKey: 'fire',
      damageTypePt: 'fogo',
      ...over,
    }) as never;

  it('numbers the cards of a trap by type, and leaves a lone card unnumbered', () => {
    const a = pending({ id: '1', trapPointId: 'pt' });
    const b = pending({ id: '2', trapPointId: 'pt' });
    expect(trapLabel(a, [a, b])).toBe('Armadilha: fogo (1 de 2)');
    expect(trapLabel(b, [a, b])).toBe('Armadilha: fogo (2 de 2)');
    expect(trapLabel(a, [a])).toBe('Armadilha: fogo');
    expect(trapLabel(pending(), [])).toBe('');
  });

  it('previews the hit points, the beast first, and says nothing without a maximum', () => {
    const plain = combatant({ label: 'Ana', hitPointsCurrent: 10, hitPointsMax: 12 } as never);
    expect(effectLine(plain, 4)).toContain('depois 6');
    expect(effectLine(combatant({} as never), 4)).toBe('');
    const druid = combatant({
      label: 'Nina',
      hitPointsCurrent: 20,
      hitPointsMax: 20,
      wildShapeBeastKey: 'wolf',
      wildShapeBeastNamePt: 'Lobo',
      wildShapeHitPointsCurrent: 3,
      wildShapeHitPointsMax: 11,
    } as never);
    expect(effectLine(druid, 5)).toContain('o resto (2) passa para Nina');
  });

  it('builds the card of a rolled damage and of one that waits for the player', () => {
    const e = encounter({
      combatants: [
        combatant({ id: 'a', label: 'Goblin' }),
        combatant({ id: 't', label: 'Ana', hitPointsCurrent: 9, hitPointsMax: 9 } as never),
      ],
    });
    const card = pendingCard(pending(), e, []);
    expect(card).toMatchObject({ total: 5, rolled: true, attacker: 'Goblin', target: 'Ana' });
    expect(card.effect).toContain('depois 4');
    const waiting = pendingCard(pending({ status: PendingDamageStatus.AWAITING_ROLL }), e, []);
    expect(waiting).toMatchObject({
      rolled: false,
      effect: '',
      label: 'Role 2d6 para o dano: some os dois',
    });
  });
});
