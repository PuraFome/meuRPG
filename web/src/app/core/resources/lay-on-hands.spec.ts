import { create } from '@bufbuild/protobuf';

import {
  CombatantState,
  ResourceTargetSchema,
  TargetInReachSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  LAY_ON_HANDS_CURE_COST,
  curePreview,
  firstAmount,
  healPreview,
  touchResult,
  touchTargetRows,
} from './lay-on-hands';
import { plainText } from './plain-text';

function target(
  id: string,
  label: string,
  over: { state?: CombatantState; distanceFt?: number; tooFar?: boolean } = {},
) {
  return create(ResourceTargetSchema, {
    target: create(TargetInReachSchema, {
      combatantId: id,
      label,
      state: over.state ?? CombatantState.HURT,
      distanceFt: over.distanceFt,
      tooFar: over.tooFar ?? false,
    }),
  });
}

describe('touchTargetRows', () => {
  it('lists the server targets with how hurt each is and how far, and marks the paladin itself', () => {
    const rows = plainText(
      touchTargetRows(
        [
          target('b', 'Brisa', { distanceFt: 5 }),
          target('t', 'Tavo', { state: CombatantState.UNHURT, distanceFt: 0 }),
        ],
        't',
      ),
    );
    expect(rows).toEqual([
      { id: 'b', label: 'Brisa', sub: 'Ferido · a 1,5 m', blocked: '' },
      { id: 't', label: 'Tavo (você)', sub: 'Ileso', blocked: '' },
    ]);
  });

  it('disables a creature beyond the touch with "Longe demais"', () => {
    const [row] = plainText(
      touchTargetRows([target('s', 'Sálvia', { distanceFt: 20, tooFar: true })], 't'),
    );
    expect(row.blocked).toBe('Longe demais: alcance de 1,5 m');
  });
});

describe('the sentences of what the touch will do', () => {
  const pool = { left: 17, total: 25 };

  it('says the heal, the cost and what is left of the pool', () => {
    expect(healPreview('Brisa', 8, pool)).toBe(
      'Brisa recupera até 8 PV (não passa do máximo). Gasta 8: restam 9 de 25.',
    );
  });

  it('says a cure costs 5, is not a heal and what is left', () => {
    expect(LAY_ON_HANDS_CURE_COST).toBe(5);
    expect(curePreview(pool)).toBe(
      'Gasta 5 da reserva (cada doença ou veneno custa 5, separados). Restam 12 de 25. Não cura PV.',
    );
  });

  it('starts the stepper at a cure worth of points, or at what is left when that is less', () => {
    expect(firstAmount(pool)).toBe(5);
    expect(firstAmount({ left: 3, total: 25 })).toBe(3);
    expect(firstAmount({ left: 0, total: 25 })).toBe(1);
  });
});

describe('touchResult', () => {
  it('says the heal only with the hit points the server sent', () => {
    const r = touchResult(
      { spent: 8, poolLeft: 9, healed: 6, nothingHappened: false },
      { amount: 8 },
      'Brisa',
      25,
    );
    expect(r.worked).toBe(true);
    expect(r.lines).toEqual([
      'Brisa recuperou 6 PV.',
      'Gastou 8 pontos da reserva: restam 9 de 25.',
    ]);
  });

  it('does not invent the hit points of a target whose number the server withheld (an NPC)', () => {
    const r = touchResult(
      { spent: 8, poolLeft: 9, nothingHappened: false },
      { amount: 8 },
      'Capitão Goblin',
      25,
    );
    expect(r.lines.join(' ')).not.toMatch(/recuperou|PV\./);
    expect(r.lines[0]).toBe('Você tocou o Capitão Goblin.');
  });

  it('words a cure for the poison and for the disease', () => {
    const answer = { spent: 5, poolLeft: 12, nothingHappened: false };
    expect(touchResult(answer, { cure: 'poison' }, 'Brisa', 25).lines[0]).toBe(
      'Você neutralizou o veneno de Brisa.',
    );
    expect(touchResult(answer, { cure: 'disease' }, 'Brisa', 25).pill).toBe('Doença curada');
  });

  it('says "Nada acontece." in the same words for any target, and never a reason or a creature type', () => {
    const answer = { spent: 8, poolLeft: 9, nothingHappened: true };
    const skeleton = touchResult(answer, { amount: 8 }, 'Esqueleto', 25);
    const goblin = touchResult(answer, { amount: 8 }, 'Goblin', 25);
    expect(skeleton.pill).toBe('Sem efeito');
    expect(skeleton.worked).toBe(false);
    expect(skeleton.lines).toEqual([
      'Você tocou o Esqueleto e gastou 8 pontos da reserva (restam 9 de 25). Nada acontece.',
    ]);
    // The only thing that changes is the name the player already knew.
    expect(skeleton.lines[0].replace('Esqueleto', 'Goblin')).toBe(goblin.lines[0]);
    expect(skeleton.lines.join(' ')).not.toMatch(/morto-vivo|constructo|undead|construct|não age/i);
  });

  it('says "Nada acontece." for a cure that did nothing too', () => {
    const r = touchResult(
      { spent: 5, poolLeft: 12, nothingHappened: true },
      { cure: 'poison' },
      'Esqueleto',
      25,
    );
    expect(r.lines[0]).toContain('Nada acontece.');
  });
});
