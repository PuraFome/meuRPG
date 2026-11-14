import { create } from '@bufbuild/protobuf';
import { describe, expect, it } from 'vitest';

import { WeaponCategory } from '../../../gen/meurpg/rules/v1/rules_pb';
import { TableWeaponSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { joinDots } from '../format/text';
import {
  priceText,
  weaponProperties,
  weaponRange,
  weaponSupport,
  weaponUse,
  weightText,
} from './weapon-text';

const NAMES: Record<string, string> = {
  'damage-type:slashing': 'Cortante',
  'damage-type:bludgeoning': 'Concussão',
  'weapon-property:versatile': 'Versátil',
  'weapon-property:thrown': 'Arremesso',
  'weapon-property:light': 'Leve',
  'weapon-property:reach': 'Alcance',
  'weapon-property:two-handed': 'Duas mãos',
};
const nameOf = (k: string): string => NAMES[k] ?? k;

const axe = create(TableWeaponSchema, {
  namePt: 'Machado de Lenhador',
  category: WeaponCategory.SIMPLE,
  damage: '1d6',
  damageType: 'damage-type:slashing',
  twoHandedDamage: '1d8',
  properties: ['weapon-property:versatile'],
  weightOz: 64,
  priceCp: 400,
});
const bolas = create(TableWeaponSchema, {
  namePt: 'Boleadeira',
  category: WeaponCategory.SIMPLE,
  ranged: true,
  damage: '1d4',
  damageType: 'damage-type:bludgeoning',
  properties: ['weapon-property:thrown'],
  normalRangeFt: 20,
  longRangeFt: 60,
  weightOz: 32,
  priceCp: 50,
});

describe('what a table weapon says', () => {
  it('names the category and the use', () => {
    expect(weaponUse(axe)).toBe('Simples, corpo a corpo');
    expect(weaponUse(bolas)).toBe('Simples, à distância');
    expect(weaponUse(create(TableWeaponSchema, { category: WeaponCategory.MARTIAL }))).toBe(
      'Marcial, corpo a corpo',
    );
  });

  it('writes the properties, the two-handed damage on Versátil and the range on Arremesso', () => {
    expect(weaponProperties(axe, nameOf)).toEqual(['Versátil (1d8)']);
    expect(weaponProperties(bolas, nameOf)).toEqual(['Arremesso (6\u00a0m / 18\u00a0m)']);
  });

  it('gives the range of a ranged weapon or a thrown one, and none otherwise', () => {
    expect(weaponRange(bolas)).toBe('6\u00a0m / 18\u00a0m');
    expect(weaponRange(axe)).toBe('');
    expect(
      weaponRange(
        create(TableWeaponSchema, {
          properties: ['weapon-property:thrown'],
          normalRangeFt: 20,
          longRangeFt: 60,
        }),
      ),
    ).toBe('6\u00a0m / 18\u00a0m');
  });

  it('writes the weight in kilograms and pounds and the price in the biggest coin that holds it', () => {
    expect(weightText(64)).toBe('2 kg (4 lb)');
    expect(weightText(32)).toBe('1 kg (2 lb)');
    expect(weightText(0)).toBe('');
    expect(priceText(400)).toBe('4 PO');
    expect(priceText(50)).toBe('5 PP');
    expect(priceText(7)).toBe('7 PC');
    expect(priceText(1500)).toBe('15 PO');
    expect(priceText(0)).toBe('');
  });

  it('is one line in the list: use, damage, properties', () => {
    expect(weaponSupport(axe, nameOf)).toBe(
      joinDots(['Simples, corpo a corpo', '1d6 cortante', 'Versátil (1d8)']),
    );
    expect(weaponSupport(bolas, nameOf)).toBe(
      joinDots(['Simples, à distância', '1d4 concussão', 'Arremesso (6\u00a0m / 18\u00a0m)']),
    );
  });
});
