import type { MessageInitShape } from '@bufbuild/protobuf';

import {
  type TableCreature,
  type TableCreatureAttack,
  type TableCreatureAttackSchema,
  type TableCreatureDamageMod,
  type TableCreatureSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { newRowId, rangeFeet, rangeMeters, unreadableRange } from './effect-draft';
import { readWhole } from './whole-number';

/**
 * The table creature form (MR-025, #578): an SRD creature chosen once as the base (`baseKey`), a name, four numbers each with a
 * fixed value and a formula of the owner's sheet, traits (name and text) and attacks. Every number is kept as typed so a
 * half-typed one is never rewritten: an empty fixed number is the base's (the damage bonus: nothing), an empty formula keeps the
 * fixed number. Distances are metres in the form and feet in the entry. The resistances, immunities and vulnerabilities
 * (`damage_mods`) have no rows yet: they go through `kept` as the entry has them, like the spell's summon, so a save never
 * loses them.
 */

export type CreatureInit = MessageInitShape<typeof TableCreatureSchema>;

/** The most attacks and traits one creature has (the server's: MaxCreatureAttacks, MaxCreatureTraits). */
export const MAX_ATTACKS = 8;
export const MAX_TRAITS = 10;

/** The sides of the dice of an attack. */
export const DICE_SIDES: readonly string[] = ['4', '6', '8', '10', '12', '20'];

export interface CreatureTraitDraft {
  /** A stable id the editor tracks the row by (never sent). */
  id: string;
  name: string;
  text: string;
}

export interface CreatureAttackDraft {
  id: string;
  name: string;
  melee: boolean;
  /** Metres, as typed; empty is 1,5 m (the server's 0). */
  reachM: string;
  rangeM: string;
  longRangeM: string;
  diceCount: string;
  /** One of `DICE_SIDES`, as text; empty until chosen. */
  diceSides: string;
  /** A "damage-type:" key. */
  damageType: string;
  /** A formula; empty is the creature's damage bonus. */
  damageBonus: string;
  /** A formula; empty is the creature's attack bonus. */
  toHit: string;
}

export interface CreatureDraft {
  name: string;
  /** The SRD creature it is made from ("monster:wolf"); empty until the master picks one. It never changes after the first save. */
  baseKey: string;
  /** The fixed numbers, as typed: empty is the base's (the damage bonus: 0). */
  hitPoints: string;
  armorClass: string;
  damageBonus: string;
  /** The formulas of the owner's sheet; empty keeps the fixed number. */
  hitPointsFormula: string;
  armorClassFormula: string;
  damageBonusFormula: string;
  attackBonusFormula: string;
  traits: CreatureTraitDraft[];
  attacks: CreatureAttackDraft[];
  /** What the form has no rows for yet, sent back as the entry has it (the resistances, immunities and vulnerabilities). */
  kept?: { damageMods: readonly TableCreatureDamageMod[] };
}

export function emptyCreature(): CreatureDraft {
  return {
    name: '',
    baseKey: '',
    hitPoints: '',
    armorClass: '',
    damageBonus: '',
    hitPointsFormula: '',
    armorClassFormula: '',
    damageBonusFormula: '',
    attackBonusFormula: '',
    traits: [],
    attacks: [],
  };
}

export function emptyTrait(): CreatureTraitDraft {
  return { id: newRowId(), name: '', text: '' };
}

export function emptyAttack(): CreatureAttackDraft {
  return {
    id: newRowId(),
    name: '',
    melee: true,
    reachM: '',
    rangeM: '',
    longRangeM: '',
    diceCount: '1',
    diceSides: '6',
    damageType: '',
    damageBonus: '',
    toHit: '',
  };
}

const text = (n: number): string => (n !== 0 ? String(n) : '');

function attackToDraft(a: TableCreatureAttack): CreatureAttackDraft {
  return {
    id: newRowId(),
    name: a.namePt,
    melee: a.melee,
    reachM: rangeMeters(a.reachFt),
    rangeM: rangeMeters(a.rangeFt),
    longRangeM: rangeMeters(a.longRangeFt),
    diceCount: text(a.diceCount),
    diceSides: text(a.diceSides),
    damageType: a.damageType,
    damageBonus: a.damageBonus,
    toHit: a.toHit,
  };
}

export function creatureToDraft(c: TableCreature): CreatureDraft {
  return {
    name: c.namePt,
    baseKey: c.baseKey,
    hitPoints: text(c.hitPoints),
    armorClass: text(c.armorClass),
    damageBonus: text(c.damageBonus),
    hitPointsFormula: c.hitPointsFormula,
    armorClassFormula: c.armorClassFormula,
    damageBonusFormula: c.damageBonusFormula,
    attackBonusFormula: c.attackBonusFormula,
    traits: c.traits.map((t) => ({ id: newRowId(), name: t.namePt, text: t.textPt })),
    attacks: c.attacks.map(attackToDraft),
    kept: { damageMods: c.damageMods },
  };
}

function draftToAttack(a: CreatureAttackDraft): MessageInitShape<typeof TableCreatureAttackSchema> {
  return {
    namePt: a.name.trim(),
    melee: a.melee,
    reachFt: a.melee ? rangeFeet(a.reachM) : 0,
    rangeFt: a.melee ? 0 : rangeFeet(a.rangeM),
    longRangeFt: a.melee ? 0 : rangeFeet(a.longRangeM),
    diceCount: readWhole(a.diceCount, false),
    diceSides: a.diceSides === '' ? 0 : Number(a.diceSides),
    damageType: a.damageType,
    damageBonus: a.damageBonus.trim(),
    toHit: a.toHit.trim(),
  };
}

/** The entry the form sends. The numbers go as typed (an unreadable one is NaN: `unreadableCreaturePaths` stops the save first). */
export function draftToCreature(d: CreatureDraft): CreatureInit {
  return {
    namePt: d.name.trim(),
    baseKey: d.baseKey,
    hitPoints: readWhole(d.hitPoints, false),
    armorClass: readWhole(d.armorClass, false),
    damageBonus: readWhole(d.damageBonus, true),
    traits: d.traits.map((t) => ({ namePt: t.name.trim(), textPt: t.text.trim() })),
    hitPointsFormula: d.hitPointsFormula.trim(),
    armorClassFormula: d.armorClassFormula.trim(),
    damageBonusFormula: d.damageBonusFormula.trim(),
    attackBonusFormula: d.attackBonusFormula.trim(),
    attacks: d.attacks.map(draftToAttack),
    damageMods: [...(d.kept?.damageMods ?? [])],
  };
}

const P = 'table_creature';

/** The paths of the inputs the creature editor draws (a violation at another path lands on the nearest one above). */
export function creaturePaths(d: CreatureDraft): string[] {
  const out = [
    `${P}.name_pt`,
    `${P}.base_key`,
    `${P}.hit_points`,
    `${P}.armor_class`,
    `${P}.damage_bonus`,
    `${P}.hit_points_formula`,
    `${P}.armor_class_formula`,
    `${P}.damage_bonus_formula`,
    `${P}.attack_bonus_formula`,
    `${P}.traits`,
    `${P}.attacks`,
  ];
  d.traits.forEach((_, k) => {
    out.push(`${P}.traits[${k}]`, `${P}.traits[${k}].name_pt`, `${P}.traits[${k}].text_pt`);
  });
  d.attacks.forEach((_, k) => {
    const at = `${P}.attacks[${k}]`;
    out.push(
      at,
      `${at}.name_pt`,
      `${at}.melee`,
      `${at}.reach_ft`,
      `${at}.range_ft`,
      `${at}.long_range_ft`,
      `${at}.dice_count`,
      `${at}.dice_sides`,
      `${at}.damage_type`,
      `${at}.damage_bonus`,
      `${at}.to_hit`,
    );
  });
  return out;
}

/** The paths of the number fields whose text the save cannot send (no whole number, or a distance that is no number of metres). */
export function unreadableCreaturePaths(d: CreatureDraft): string[] {
  const out: string[] = [];
  const whole = (path: string, value: string, signed: boolean) => {
    if (Number.isNaN(readWhole(value, signed))) {
      out.push(path);
    }
  };
  whole(`${P}.hit_points`, d.hitPoints, false);
  whole(`${P}.armor_class`, d.armorClass, false);
  whole(`${P}.damage_bonus`, d.damageBonus, true);
  d.attacks.forEach((a, k) => {
    const at = `${P}.attacks[${k}]`;
    whole(`${at}.dice_count`, a.diceCount, false);
    const dist = (name: string, value: string) => {
      if (unreadableRange(value)) {
        out.push(`${at}.${name}`);
      }
    };
    if (a.melee) {
      dist('reach_ft', a.reachM);
    } else {
      dist('range_ft', a.rangeM);
      dist('long_range_ft', a.longRangeM);
    }
  });
  return out;
}

/** What a distance field says when it cannot be read. */
export { UNREADABLE_RANGE } from './effect-draft';
