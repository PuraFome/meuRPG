import { WeaponCategory } from '../../../gen/meurpg/rules/v1/rules_pb';
import type { TableWeapon } from '../../../gen/meurpg/rules/v1/table_content_pb';
import { joinDots } from '../format/text';
import { metersText, poundsToKg } from '../units';

/**
 * What a table weapon says (RN-23, #580 S2), as the list, the read view and the editor's preview write it: pure
 * functions over the entry and a way to name a key. The server decides what a weapon does; this only writes it.
 */

type NameOf = (key: string) => string;

const THROWN = 'weapon-property:thrown';
const VERSATILE = 'weapon-property:versatile';

/** "Simples, corpo a corpo". */
export function weaponUse(w: TableWeapon): string {
  const category = w.category === WeaponCategory.MARTIAL ? 'Marcial' : 'Simples';
  return `${category}, ${w.ranged ? 'à distância' : 'corpo a corpo'}`;
}

/** "6 m / 18 m" for a ranged weapon or a thrown one; empty for any other. */
export function weaponRange(w: TableWeapon): string {
  if (!w.ranged && !w.properties.includes(THROWN)) {
    return '';
  }
  if (w.normalRangeFt <= 0 || w.longRangeFt <= 0) {
    return '';
  }
  return `${metersText(w.normalRangeFt)} / ${metersText(w.longRangeFt)}`;
}

/** "1d6 cortante". */
export function weaponDamage(w: TableWeapon, nameOf: NameOf): string {
  const type = w.damageType ? nameOf(w.damageType).toLowerCase() : '';
  return [w.damage, type].filter(Boolean).join(' ');
}

/** The property names, Versátil with its two-handed damage and Arremesso with its range. */
export function weaponProperties(w: TableWeapon, nameOf: NameOf): string[] {
  const range = weaponRange(w);
  return w.properties.map((key) => {
    const name = nameOf(key);
    if (key === VERSATILE && w.twoHandedDamage) {
      return `${name} (${w.twoHandedDamage})`;
    }
    if (key === THROWN && range) {
      return `${name} (${range})`;
    }
    return name;
  });
}

/** "2 kg (4 lb)" from ounces; empty for none. */
export function weightText(oz: number): string {
  if (oz <= 0) {
    return '';
  }
  const pounds = Math.round((oz / 16) * 10) / 10;
  const lb = String(pounds).replace('.', ',');
  return `${poundsToKg(pounds)} (${lb} lb)`.replace(/(\d) (kg|lb)/g, '$1 $2');
}

/** The coins a price is written in, the biggest first (PO 100 cp, PP 10 cp, PC 1 cp). */
export const COINS: readonly {
  readonly code: string;
  readonly cp: number;
  readonly label: string;
}[] = [
  { code: 'PO', cp: 100, label: 'PO · ouro' },
  { code: 'PP', cp: 10, label: 'PP · prata' },
  { code: 'PC', cp: 1, label: 'PC · cobre' },
];

/** "4 PO": the biggest coin that holds the price whole; empty for none. */
export function priceText(cp: number): string {
  if (cp <= 0) {
    return '';
  }
  const coin = COINS.find((c) => cp % c.cp === 0) ?? COINS[COINS.length - 1];
  return `${cp / coin.cp} ${coin.code}`;
}

/** The list's line: use, damage, properties. */
export function weaponSupport(w: TableWeapon, nameOf: NameOf): string {
  return joinDots([weaponUse(w), weaponDamage(w, nameOf), weaponProperties(w, nameOf).join(', ')]);
}
