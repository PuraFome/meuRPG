import { create } from '@bufbuild/protobuf';

import { WeaponCategory } from '../../../gen/meurpg/rules/v1/rules_pb';
import { TableWeaponSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
  draftToWeapon,
  emptyWeapon,
  lockNote,
  propertyLocks,
  weaponToDraft,
  type WeaponDraft,
} from './weapon-draft';

const filled = (p: Partial<WeaponDraft>): WeaponDraft => ({
  ...emptyWeapon(),
  name: 'Lança',
  ...p,
});

describe('weapon draft', () => {
  it('starts as a simple melee weapon with nothing typed', () => {
    const d = emptyWeapon();
    expect(d.category).toBe(WeaponCategory.SIMPLE);
    expect(d.ranged).toBe(false);
    expect(d.properties).toEqual([]);
    expect(d.name).toBe('');
  });

  it('writes the range in feet from meters, the weight in ounces from kilograms and the price in copper from the coin', () => {
    const w = draftToWeapon(
      filled({
        ranged: true,
        damage: '1d8',
        damageType: 'damage-type:piercing',
        normalRange: '24',
        longRange: '96',
        weightKg: '2',
        price: '25',
        coin: 'PO',
      }),
    );
    expect(w.normalRangeFt).toBe(80);
    expect(w.longRangeFt).toBe(320);
    expect(w.weightOz).toBe(64);
    expect(w.priceCp).toBe(2500);
  });

  it('writes a price in silver and in copper', () => {
    expect(draftToWeapon(filled({ price: '3', coin: 'PP' })).priceCp).toBe(30);
    expect(draftToWeapon(filled({ price: '7', coin: 'PC' })).priceCp).toBe(7);
  });

  it('sends zero for what was left empty', () => {
    const w = draftToWeapon(filled({}));
    expect([w.normalRangeFt, w.longRangeFt, w.weightOz, w.priceCp]).toEqual([0, 0, 0, 0]);
  });

  it('keeps the two-handed damage only with the versatile property', () => {
    const base = { damage: '1d6', twoHandedDamage: '1d8' };
    expect(draftToWeapon(filled({ ...base })).twoHandedDamage).toBe('');
    expect(
      draftToWeapon(filled({ ...base, properties: ['weapon-property:versatile'] })).twoHandedDamage,
    ).toBe('1d8');
  });

  it('round-trips a stored weapon, the price in the biggest coin that holds it whole', () => {
    const w = draftToWeapon(
      filled({
        damage: '1d6',
        damageType: 'damage-type:piercing',
        properties: ['weapon-property:thrown'],
        ranged: false,
        normalRange: '6',
        longRange: '18',
        weightKg: '1,5',
        price: '5',
        coin: 'PO',
      }),
    );
    const back = weaponToDraft(create(TableWeaponSchema, w));
    expect(back.price).toBe('5');
    expect(back.coin).toBe('PO');
    expect(back.normalRange).toBe('6');
    expect(back.longRange).toBe('18');
    expect(back.name).toBe('Lança');
  });

  describe('locked properties (board 580-11)', () => {
    const keys = (ranged: boolean, props: string[] = []) =>
      propertyLocks(ranged, props).map((l) => l.key.replace('weapon-property:', ''));

    it('always locks Especial: a table weapon has no rule text to give it', () => {
      expect(keys(false)).toEqual(['special']);
    });

    it('locks Alcance on a ranged weapon, and Recarga while it has no Munição', () => {
      expect(keys(true)).toEqual(['reach', 'loading', 'special']);
      expect(keys(true, ['weapon-property:ammunition'])).toEqual(['reach', 'special']);
    });

    it('says why in one line: a sentence for a single lock, "Nome: motivo" for several', () => {
      expect(lockNote(propertyLocks(false, []))).toBe(
        'Especial pede um texto de regra, que a arma da mesa não tem.',
      );
      expect(lockNote(propertyLocks(true, []))).toBe(
        'Alcance: só corpo a corpo · Recarga: pede Munição · Especial: pede um texto de regra, que a arma da mesa não tem.',
      );
    });
  });
});
