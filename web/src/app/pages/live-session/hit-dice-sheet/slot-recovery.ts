import type { SlotUsageVm } from '../../../core/play/resource-usage.types';
import type { VitalsVm } from '../../../core/play/live-session.types';

/** The highest slot level a recovery gives back (SRD 5.1: "none ... 6th level or higher"). */
export const SLOT_RECOVERY_MAX_LEVEL = 5;

/** The features that give expended slots back after a short rest: they differ in the class, the resource and the words. */
export interface SlotRecoveryDef {
  readonly kind: 'arcane' | 'natural';
  /** The once-a-day use (`rules.SlotRecovery.ResourceKey`). */
  readonly resourceKey: string;
  readonly title: string;
  /** The call of the feature in `ResourceClient`. */
  readonly call: 'useArcaneRecovery' | 'useNaturalRecovery';
  /** The combined spell level the slots may add up to. */
  readonly allowance: (v: VitalsVm) => number;
}

export const SLOT_RECOVERIES: readonly SlotRecoveryDef[] = [
  {
    kind: 'arcane',
    resourceKey: 'arcane_recovery',
    title: 'Recuperação Arcana',
    call: 'useArcaneRecovery',
    allowance: (v) => v.arcaneRecoveryAllowance ?? 0,
  },
  {
    kind: 'natural',
    resourceKey: 'natural_recovery',
    title: 'Recuperação Natural',
    call: 'useNaturalRecovery',
    allowance: (v) => v.naturalRecoveryAllowance ?? 0,
  },
];

/** Where a recovery stands for a character. */
export interface SlotRecoveryState {
  readonly def: SlotRecoveryDef;
  /** The combined spell level the slots may add up to. */
  readonly allowance: number;
  /** The use is spent: it comes back on a long rest. */
  readonly spent: boolean;
}

/** The recoveries the character has (a resource and an allowance), in the order of `SLOT_RECOVERIES`. */
export function slotRecoveryStates(v: VitalsVm): readonly SlotRecoveryState[] {
  return SLOT_RECOVERIES.flatMap((def) => {
    const use = v.resources?.find((r) => r.key === def.resourceKey);
    const allowance = def.allowance(v);
    return use && allowance >= 1 ? [{ def, allowance, spent: use.total - use.used < 1 }] : [];
  });
}

/** The slots of levels 1 to 5 with some expended, lowest first: the only ones a recovery can give back. */
export function expendedSlots(
  v: VitalsVm,
): readonly (SlotUsageVm & { readonly expended: number })[] {
  return v.spellSlots
    .filter((s) => s.level <= SLOT_RECOVERY_MAX_LEVEL && s.used > 0)
    .map((s) => ({ ...s, expended: Math.min(s.used, s.total) }))
    .filter((s) => s.expended > 0);
}

/** "2 níveis", "1 nível". */
export function levelsWords(n: number): string {
  return `${n} ${n === 1 ? 'nível' : 'níveis'}`;
}
