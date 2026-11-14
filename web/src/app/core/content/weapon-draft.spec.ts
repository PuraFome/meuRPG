import { create } from '@bufbuild/protobuf';

import { WeaponCategory } from '../../../gen/meurpg/rules/v1/rules_pb';
import { TableWeaponSchema } from '../../../gen/meurpg/rules/v1/table_content_pb';
import {
  draftToWeapon,
  emptyWeapon,
  lockedProperties,
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
    it('locks reach and loading on the wrong side of the melee/ranged line', () => {
      expect(lockedProperties(false)).toContain('weapon-property:ammunition');
      expect(lockedProperties(false)).toContain('weapon-property:loading');
      expect(lockedProperties(true)).toContain('weapon-property:reach');
    });

    it('never locks the ones both kinds may carry', () => {
      expect(lockedProperties(true)).not.toContain('weapon-property:light');
      expect(lockedProperties(false)).not.toContain('weapon-property:light');
    });
  });
});
