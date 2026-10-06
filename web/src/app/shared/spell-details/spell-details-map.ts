import {
  CastingTimeUnit as GenCastingTimeUnit,
  SpellAttackType as GenSpellAttackType,
  SpellDetails as GenSpellDetails,
  SpellDurationKind as GenSpellDurationKind,
  SpellDurationUnit as GenSpellDurationUnit,
  SpellRangeKind as GenSpellRangeKind,
} from '../../../gen/meurpg/rules/v1/rules_pb';
import {
  CastingTimeUnitKey,
  DurationKindKey,
  DurationUnitKey,
  RangeKindKey,
  SpellDetailsVm,
} from './spell-details.types';

/**
 * The generated `SpellDetails` as the gen-free view-model the "?" sheet reads.
 * It lives next to the sheet, so the editor and the session both use it.
 */

const CASTING_UNIT_FROM_GEN: Record<GenCastingTimeUnit, CastingTimeUnitKey> = {
  [GenCastingTimeUnit.UNSPECIFIED]: '',
  [GenCastingTimeUnit.ACTION]: 'action',
  [GenCastingTimeUnit.BONUS_ACTION]: 'bonus_action',
  [GenCastingTimeUnit.REACTION]: 'reaction',
  [GenCastingTimeUnit.MINUTE]: 'minute',
  [GenCastingTimeUnit.HOUR]: 'hour',
};

const RANGE_KIND_FROM_GEN: Record<GenSpellRangeKind, RangeKindKey> = {
  [GenSpellRangeKind.UNSPECIFIED]: '',
  [GenSpellRangeKind.SELF]: 'self',
  [GenSpellRangeKind.TOUCH]: 'touch',
  [GenSpellRangeKind.RANGED]: 'ranged',
  [GenSpellRangeKind.SIGHT]: 'sight',
  [GenSpellRangeKind.UNLIMITED]: 'unlimited',
  [GenSpellRangeKind.SPECIAL]: 'special',
};

const DURATION_KIND_FROM_GEN: Record<GenSpellDurationKind, DurationKindKey> = {
  [GenSpellDurationKind.UNSPECIFIED]: '',
  [GenSpellDurationKind.INSTANTANEOUS]: 'instantaneous',
  [GenSpellDurationKind.TIMED]: 'timed',
  [GenSpellDurationKind.UNTIL_DISPELLED]: 'until_dispelled',
  [GenSpellDurationKind.SPECIAL]: 'special',
};

const DURATION_UNIT_FROM_GEN: Record<GenSpellDurationUnit, DurationUnitKey> = {
  [GenSpellDurationUnit.UNSPECIFIED]: '',
  [GenSpellDurationUnit.ROUND]: 'round',
  [GenSpellDurationUnit.MINUTE]: 'minute',
  [GenSpellDurationUnit.HOUR]: 'hour',
  [GenSpellDurationUnit.DAY]: 'day',
};

/** The dice at the lowest level a map names: the spell's own circle (or the character's first level). */
function firstDice(byLevel: Record<number, string>): string {
  const levels = Object.keys(byLevel).map(Number);
  return levels.length > 0 ? byLevel[Math.min(...levels)] : '';
}

/** A spell of the table's own: its key ends in "@mesa" (the one place that says so). */
export function isTableSpellKey(key: string): boolean {
  return key.endsWith('@mesa');
}

export function spellDetailsFromGen(d: GenSpellDetails): SpellDetailsVm {
  const spell = d.spell;
  return {
    key: spell?.key ?? '',
    namePt: spell?.namePt ?? '',
    nameEn: spell?.name ?? '',
    level: spell?.level ?? 0,
    schoolNamePt: spell?.schoolNamePt ?? '',
    ritual: spell?.ritual ?? false,
    concentration: spell?.concentration ?? false,
    castingTime: {
      amount: d.castingTime?.amount ?? 0,
      unit: CASTING_UNIT_FROM_GEN[d.castingTime?.unit ?? GenCastingTimeUnit.UNSPECIFIED],
      trigger: d.castingTime?.trigger ?? '',
      raw: d.castingTime?.raw ?? '',
    },
    range: {
      kind: RANGE_KIND_FROM_GEN[d.range?.kind ?? GenSpellRangeKind.UNSPECIFIED],
      distanceFt: d.range?.distanceFt ?? 0,
      raw: d.range?.raw ?? '',
    },
    components: {
      verbal: d.components?.verbal ?? false,
      somatic: d.components?.somatic ?? false,
      material: d.components?.material ?? false,
      materialText: d.components?.materialText ?? '',
    },
    duration: {
      kind: DURATION_KIND_FROM_GEN[d.duration?.kind ?? GenSpellDurationKind.UNSPECIFIED],
      amount: d.duration?.amount ?? 0,
      unit: DURATION_UNIT_FROM_GEN[d.duration?.unit ?? GenSpellDurationUnit.UNSPECIFIED],
      upTo: d.duration?.upTo ?? false,
      concentration: d.duration?.concentration ?? false,
      raw: d.duration?.raw ?? '',
    },
    description: d.description,
    higherLevel: d.higherLevel,
    classKeys: spell?.classKeys ?? [],
    archived: spell?.archived ?? false,
    table: isTableSpellKey(spell?.key ?? ''),
    targetLabel: d.target?.labelPt ?? '',
    attack:
      d.attackType === GenSpellAttackType.MELEE
        ? 'melee'
        : d.attackType === GenSpellAttackType.RANGED
          ? 'ranged'
          : 'none',
    damage: d.damage
      .map((x) => ({ dice: firstDice(x.bySlotLevel) || firstDice(x.byCharacterLevel), typePt: x.damageTypePt }))
      .filter((x) => x.dice !== ''),
  };
}
