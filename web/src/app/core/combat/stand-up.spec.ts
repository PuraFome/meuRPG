import { create } from '@bufbuild/protobuf';

import { CombatantSchema } from '../../../gen/meurpg/play/v1/combat_pb';
import { standUpRow } from './stand-up';

const prone = (over: object = {}) =>
  create(CombatantSchema, {
    conditions: ['condition:prone'],
    standUpCostDft: 150,
    movementLeftDft: 300,
    ...over,
  });

describe('standUpRow, the Levantar-se of a prone combatant (SRD 5.1, Being Prone)', () => {
  it('is nothing for a combatant that stands', () => {
    expect(standUpRow(create(CombatantSchema, { standUpCostDft: 0 }))).toBeNull();
  });

  it('says what it costs and is on when the movement left pays for it', () => {
    expect(standUpRow(prone())).toEqual({
      detail: 'Derrubado. Levantar-se gasta 4,5 m de movimento.',
      off: false,
      reason: '',
    });
    expect(standUpRow(prone({ movementLeftDft: 150 }))?.off).toBe(false);
  });

  it('is off with the missing movement as the reason', () => {
    const row = standUpRow(prone({ movementLeftDft: 100 }));
    expect(row?.off).toBe(true);
    expect(row?.reason).toMatch(/^Faltam\s1,5\sm de movimento\.$/u);
  });

  it('is off with no speed', () => {
    expect(standUpRow(prone({ standUpCostDft: 0 }))).toEqual({
      detail: 'Derrubado.',
      off: true,
      reason: 'Sem velocidade, não dá para se levantar.',
    });
  });
});
