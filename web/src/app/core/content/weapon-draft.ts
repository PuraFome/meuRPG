import type { MessageInitShape } from '@bufbuild/protobuf';

import { WeaponCategory } from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  type TableWeapon,
  type TableWeaponSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { metersToFeet, rangeMeters } from '../units';
import { COINS } from './weapon-text';

/**
 * The weapon form (RN-23, #580 S2): what the master types is meters, kilograms and a coin, and the server stores feet, ounces
 * and copper pieces, so this is where they turn into each other. A number is kept as typed so a half-typed one is never
 * rewritten; one that reads as no number is NaN, and the editor refuses to save it. Whether the properties go together is the
 * server's; the editor only locks the ones that cannot fit the weapon's use (board 580-11).
 */

export type WeaponInit = MessageInitShape<typeof TableWeaponSchema>;

const PREFIX = 'weapon-property:';
export const AMMUNITION = `${PREFIX}ammunition`;
export const LOADING = `${PREFIX}loading`;
export const REACH = `${PREFIX}reach`;
export const THROWN = `${PREFIX}thrown`;
export const VERSATILE = `${PREFIX}versatile`;

/** The most a weapon's name holds, as the server counts it. */
export const WEAPON_NAME_MAX = 60;

/** One ounce is 1/16 of a pound, and a pound is half a kilogram at the table's rate. */
const OZ_PER_KG = 32;

export interface WeaponDraft {
  name: string;
  description: string;
  category: WeaponCategory;
  ranged: boolean;
  damage: string;
  /** A damage-type key; empty is none (a weapon with no damage). */
  damageType: string;
  twoHandedDamage: string;
  properties: string[];
  /** Meters, as typed. */
  normalRange: string;
  longRange: string;
  /** Kilograms, as typed. */
  weightKg: string;
  /** The price in `coin`, as typed. */
  price: string;
  /** "PO", "PP" or "PC". */
  coin: string;
}

export function emptyWeapon(): WeaponDraft {
  return {
    name: '',
    description: '',
    category: WeaponCategory.SIMPLE,
    ranged: false,
    damage: '',
    damageType: '',
    twoHandedDamage: '',
    properties: [],
    normalRange: '',
    longRange: '',
    weightKg: '',
    price: '',
    coin: 'PO',
  };
}

export interface PropertyLock {
  key: string;
  name: string;
  /** Why it is shut, said after the name. */
  reason: string;
}

const SPECIAL = `${PREFIX}special`;

/** The properties this weapon cannot take now, with the reason (board 580-11): they stay on screen, shut. */
export function propertyLocks(ranged: boolean, properties: readonly string[]): PropertyLock[] {
  const locks: PropertyLock[] = [];
  if (ranged) {
    locks.push({ key: REACH, name: 'Alcance', reason: 'só corpo a corpo' });
    if (!properties.includes(AMMUNITION)) {
      locks.push({ key: LOADING, name: 'Recarga', reason: 'pede Munição' });
    }
  }
  locks.push({
    key: SPECIAL,
    name: 'Especial',
    reason: 'pede um texto de regra, que a arma da mesa não tem',
  });
  return locks;
}

/** The one line under the chips that says why each shut one is shut. */
export function lockNote(locks: readonly PropertyLock[]): string {
  if (locks.length === 1) {
    return `${locks[0].name} ${locks[0].reason}.`;
  }
  return `${locks.map((l) => `${l.name}: ${l.reason}`).join(' · ')}.`;
}

/** The number a field's text holds, with a comma or a point: empty is 0, text that is no non-negative number is NaN. */
export function readDecimal(text: string): number {
  const t = text.trim();
  if (t === '') {
    return 0;
  }
  if (!/^\d+([.,]\d+)?$/.test(t)) {
    return Number.NaN;
  }
  return Number(t.replace(',', '.'));
}

const coinCp = (code: string): number => COINS.find((c) => c.code === code)?.cp ?? 1;

export function draftToWeapon(d: WeaponDraft): WeaponInit {
  return {
    namePt: d.name.trim(),
    descPt: d.description.trim() ? [d.description.trim()] : [],
    category: d.category,
    ranged: d.ranged,
    damage: d.damage.trim(),
    damageType: d.damageType,
    twoHandedDamage: d.properties.includes(VERSATILE) ? d.twoHandedDamage.trim() : '',
    properties: [...d.properties],
    normalRangeFt: metersToFeet(readDecimal(d.normalRange)),
    longRangeFt: metersToFeet(readDecimal(d.longRange)),
    weightOz: Math.round(readDecimal(d.weightKg) * OZ_PER_KG),
    priceCp: Math.round(readDecimal(d.price) * coinCp(d.coin)),
  };
}

export function weaponToDraft(w: TableWeapon): WeaponDraft {
  const coin = w.priceCp > 0 ? (COINS.find((c) => w.priceCp % c.cp === 0) ?? COINS[2]) : COINS[0];
  const kg = w.weightOz / OZ_PER_KG;
  return {
    name: w.namePt,
    description: w.descPt.join('\n\n'),
    category: w.category,
    ranged: w.ranged,
    damage: w.damage,
    damageType: w.damageType,
    twoHandedDamage: w.twoHandedDamage,
    properties: [...w.properties],
    normalRange: rangeMeters(w.normalRangeFt),
    longRange: rangeMeters(w.longRangeFt),
    weightKg: kg > 0 ? String(Math.round(kg * 10) / 10).replace('.', ',') : '',
    price: w.priceCp > 0 ? String(w.priceCp / coin.cp) : '',
    coin: coin.code,
  };
}

/** The paths of the number fields whose text is no number, for the editor to refuse. */
export function unreadableWeaponPaths(d: WeaponDraft): string[] {
  const p = 'table_weapon';
  const bad = (t: string): boolean => Number.isNaN(readDecimal(t));
  return [
    bad(d.normalRange) ? `${p}.normal_range_ft` : '',
    bad(d.longRange) ? `${p}.long_range_ft` : '',
    bad(d.weightKg) ? `${p}.weight_oz` : '',
    bad(d.price) ? `${p}.price_cp` : '',
  ].filter(Boolean);
}

/**
 * The combination of properties the SRD's weapons never have, in words (the server refuses them all at `.properties` and does
 * not say which, so the editor says it as soon as the combination is on the form); empty when there is none.
 */
export function propertyClash(ranged: boolean, properties: readonly string[]): string {
  const has = (p: string): boolean => properties.includes(`${PREFIX}${p}`);
  if (has('loading') && !has('ammunition')) {
    return 'Recarga só vale para uma arma com Munição.';
  }
  if (has('ammunition') && !ranged) {
    return 'Munição só numa arma à distância.';
  }
  if (has('reach') && ranged) {
    return 'Alcance só numa arma corpo a corpo.';
  }
  if (has('two-handed') && has('versatile')) {
    return 'Uma arma de Duas mãos não é Versátil.';
  }
  if (has('light') && has('heavy')) {
    return 'Leve e Pesada não vão juntas. Tire uma.';
  }
  if (has('two-handed') && has('light')) {
    return 'Leve é de arma usada em cada mão: ela não pode exigir Duas mãos.';
  }
  return '';
}
