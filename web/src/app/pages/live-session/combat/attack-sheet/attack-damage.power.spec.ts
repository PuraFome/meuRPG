import { signal } from '@angular/core';

import type { DamagePart } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { AttackDamage } from './attack-damage';
import type { AttackState } from './attack-state';

const part = (over: Partial<DamagePart>): DamagePart =>
  ({
    key: 'weapon',
    labelPt: 'Montante',
    diceCount: 2,
    diceSides: 6,
    flat: 3,
    damageTypeKey: 'damage-type:slashing',
    damageTypePt: 'cortante',
    ...over,
  }) as DamagePart;

/** The +10 of Golpe Pesado: an automatic part with no dice, which the server never doubles on a critical hit. */
const heavy = part({
  key: 'power_attack',
  labelPt: 'Golpe Pesado',
  diceCount: 0,
  diceSides: 0,
  flat: 10,
  auto: true,
});

function damageOf(parts: readonly DamagePart[]): AttackDamage {
  const s = { pending: signal({ parts }), picks: signal([]) } as unknown as AttackState;
  return new AttackDamage(s, 'Montante', true, signal(false), signal(''));
}

describe('the typed damage of a power attack', () => {
  const dmg = damageOf([part({}), heavy]);

  it('keeps the +10 out of the field: the range is the weapon dice alone', () => {
    expect(dmg.fields()).toEqual([
      {
        key: 'damage-type:slashing',
        label: 'Montante, cortante: 2d6',
        hint: 'De 2 a 12.',
        min: 2,
        max: 12,
      },
    ]);
  });

  it('says the app adds the weapon bonus and the trade: "Sem o + 13"', () => {
    expect(dmg.flat()).toBe(13);
    expect(dmg.bonusLine()).toBe('Sem o + 13: o app soma o bônus e o Golpe Pesado.');
  });

  it('sums the fields plus both on the button: 7 typed is "Aplicar 20 de dano"', () => {
    expect(dmg.applyLabel(7 + dmg.flat())).toBe('Aplicar 20 de dano');
    expect(dmg.applyLabel(null)).toBe('Aplicar dano');
  });

  it('rolls "2d6 + 13" in the app, the trade named in the formula', () => {
    expect(dmg.partsLabel()).toBe('Rolar 2d6 + 13 no app');
  });

  it('doubles the dice on a critical hit and not the +10', () => {
    const crit = damageOf([part({ diceCount: 4, doubled: true }), heavy]);
    expect(crit.fields().map((f) => [f.label, f.min, f.max])).toEqual([
      ['Montante, cortante: 4d6', 4, 24],
    ]);
    expect(crit.flat()).toBe(13);
    expect(crit.bonusLine()).toBe('Sem o + 13: o app soma o bônus e o Golpe Pesado.');
    expect(crit.partsLabel()).toBe('Rolar 4d6 + 13 no app');
  });
});
