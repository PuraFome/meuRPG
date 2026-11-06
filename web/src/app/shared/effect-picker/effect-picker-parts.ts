import {
  type EffectDraft,
  type EffectMenuVm,
  type MenuOption,
} from '../../core/content/effect-draft';
import {
  CRITICAL_COPY,
  EFFECT_REFUSAL_TEXTS,
  RESOURCE_PICK_COPY,
} from '../../core/content/effect-field-copy';
import { GENERIC_EFFECT_TEXTS } from '../../core/content/content-violation-texts';
import { CRITICAL_RANGE_TARGET } from '../../core/content/effect-visible';
import type { SelectOption } from '../form-fields/select-field';

/**
 * The parts of the effect picker that are plain functions of the draft and the menu: the resources of the entry a reroll may
 * spend, the mode a target fixes, and what a refusal says of a field several types share.
 */

/** "sorte_teimosa" as a person says it: "Sorte Teimosa". */
function titled(key: string): string {
  return key
    .split('_')
    .filter((w) => w !== '')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

/** "Sorte Teimosa (3 por descanso longo)": a resource the entry defines, with its uses and when they come back. */
function resourceLabel(r: EffectDraft, menu: EffectMenuVm): string {
  const max = r.max.trim();
  const when = r.recharge === '' ? '' : menu.nameOf('recharges', r.recharge).toLowerCase();
  if (max === '') {
    return titled(r.resource);
  }
  const rest = r.recharge === 'short_rest' || r.recharge === 'long_rest';
  const uses = when === '' ? max : `${max}${rest ? ' por ' : ', '}${when}`;
  return `${titled(r.resource)} (${uses})`;
}

/** What "Gasta usos de" offers a reroll. */
export interface ResourcePick {
  readonly options: SelectOption[];
  /** The entry defines no resource and none is picked: the field shows "Nenhum" and cannot change. */
  readonly locked: boolean;
  /** The picked resource is not one the entry defines (any more). */
  readonly stale: boolean;
}

/** The entry's own resource effects (the siblings of the effect), with "Nenhum" first; a picked name the entry no longer defines is
 * listed too, as what it is, so the field shows the truth and the refusal has a value to point at. */
export function resourcePick(
  siblings: readonly EffectDraft[],
  menu: EffectMenuVm,
  current: string,
): ResourcePick {
  const own = siblings.filter((e) => e.type === 'resource' && e.resource.trim() !== '');
  const options: SelectOption[] = [{ value: '', label: RESOURCE_PICK_COPY.none }];
  const seen = new Set<string>();
  for (const r of own) {
    const key = r.resource.trim();
    if (!seen.has(key)) {
      seen.add(key);
      options.push({ value: key, label: resourceLabel(r, menu) });
    }
  }
  const stale = current !== '' && !seen.has(current);
  if (stale) {
    options.push({ value: current, label: RESOURCE_PICK_COPY.stale });
  }
  return { options, locked: seen.size === 0 && !stale, stale };
}

/** The mode a modifier target fixes (the critical range is "set"), or '' when it takes every mode. */
export function fixedMode(menu: EffectMenuVm, target: string): string {
  const only: readonly string[] = menu.optionOf('modifier_targets', target)?.onlyModes ?? [];
  return only.length === 1 ? only[0] : '';
}

/** The effect after its target changed: a target that fixes its mode sets it, and only the critical range keeps a scope. */
export function afterTarget(menu: EffectMenuVm, d: EffectDraft): EffectDraft {
  return {
    ...d,
    mode: fixedMode(menu, d.target) || d.mode,
    applies: d.target === CRITICAL_RANGE_TARGET ? d.applies : '',
  };
}

/** Whether the critical range is the modifier's target. */
export function isCriticalRange(e: Pick<EffectDraft, 'type' | 'target'>): boolean {
  return e.type === 'modifier' && e.target === CRITICAL_RANGE_TARGET;
}

/** The refusals of a field with the words of the effect's own type where the server's reason said it generally. */
export function refusalTexts(
  e: EffectDraft,
  field: string,
  texts: readonly string[],
): readonly string[] {
  const type = isCriticalRange(e) ? `modifier.${CRITICAL_RANGE_TARGET}` : e.type;
  const own = EFFECT_REFUSAL_TEXTS[`${type}.${field}`];
  return own ? texts.map((t) => (GENERIC_EFFECT_TEXTS.has(t) ? own : t)) : texts;
}

/** The target's value in the menu, with the critical range in the board's words. */
export function targetOption(o: MenuOption): SelectOption & { hint: string } {
  const critical = o.key === CRITICAL_RANGE_TARGET;
  return {
    value: o.key,
    label: critical ? CRITICAL_COPY.target : o.namePt,
    hint: critical ? CRITICAL_COPY.targetHint : o.hintPt,
  };
}
