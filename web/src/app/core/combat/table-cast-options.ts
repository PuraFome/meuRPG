import type { SpellTableOption } from '../../../gen/meurpg/rules/v1/rules_pb';
import { DAMAGE_TYPE_OPTIONS } from '../characters/character-labels';

/** A damage type a swap may land on: the content key and the Portuguese name. */
export interface TableTypeChoice {
  readonly key: string;
  readonly name: string;
}

/** A cast option of a table feature as the cast sheet lists it. */
export interface TableOptionRow {
  readonly key: string;
  readonly name: string;
  readonly summary: string;
  /** Applies by itself: shown, not ticked. */
  readonly alwaysOn: boolean;
  readonly checked: boolean;
  /** Why it cannot be ticked (the server's reason); empty when it can. */
  readonly blocked: string;
  /** "Gasta 1 uso · restam 2", or empty when it spends nothing. */
  readonly cost: string;
  /** The damage types a swap may pick; empty for any other change. */
  readonly types: readonly TableTypeChoice[];
  /** The type picked for a ticked swap. */
  readonly type: string;
}

/** What a cast sends for a ticked option (`SpellOptionChoice`). */
export interface TableOptionPick {
  readonly key: string;
  readonly damageTypeKey: string;
}

const SWAP = 'damage_type_swap';

/** The Portuguese name of a damage type key ("damage-type:fire" is "Fogo"); the key itself when it is unknown. */
export function damageTypeName(key: string): string {
  const slug = key.replace(/^damage-type:/, '');
  return DAMAGE_TYPE_OPTIONS.find((o) => o.key === slug)?.label ?? key;
}

/** "Gasta 1 uso · restam 2" for an option that names a resource. */
function costText(o: SpellTableOption): string {
  if (o.resource === '') {
    return '';
  }
  return `Gasta 1 uso · ${o.usesLeft === 1 ? 'resta 1' : `restam ${o.usesLeft}`}`;
}

/** The damage type of a ticked swap: the one picked while it is still one of the option's, else its first. */
export function swapType(o: SpellTableOption, picked: Readonly<Record<string, string>>): string {
  const keys = o.damageTypeKeys;
  return keys.find((k) => k === picked[o.key]) ?? keys[0] ?? '';
}

/** Whether an option is a damage type swap. */
export function isSwap(o: SpellTableOption): boolean {
  return o.change === SWAP;
}

/** The options of a spell with their state: what is always on is shown ticked and fixed; an option the caster may not tick
 * now stays grey with the server's reason. */
export function tableOptionRows(
  options: readonly SpellTableOption[],
  ticked: readonly string[],
  types: Readonly<Record<string, string>>,
): TableOptionRow[] {
  return options.map((o) => ({
    key: o.key,
    name: o.namePt,
    summary: o.summaryPt,
    alwaysOn: o.alwaysOn,
    checked: o.alwaysOn || ticked.includes(o.key),
    blocked: o.alwaysOn || o.allowed ? '' : o.disabledReasonPt,
    cost: costText(o),
    types: isSwap(o) ? o.damageTypeKeys.map((key) => ({ key, name: damageTypeName(key) })) : [],
    type: isSwap(o) ? swapType(o, types) : '',
  }));
}

/** The options a cast sends: the ticked ones that are not always on and may be ticked, each with the type of its swap. */
export function tableOptionPicks(
  options: readonly SpellTableOption[],
  ticked: readonly string[],
  types: Readonly<Record<string, string>>,
): TableOptionPick[] {
  return options
    .filter((o) => !o.alwaysOn && o.allowed && ticked.includes(o.key))
    .map((o) => ({ key: o.key, damageTypeKey: isSwap(o) ? swapType(o, types) : '' }));
}

/** The keys with `key` ticked or unticked. */
export function toggledTable(ticked: readonly string[], key: string): string[] {
  return ticked.includes(key) ? ticked.filter((k) => k !== key) : [...ticked, key];
}
