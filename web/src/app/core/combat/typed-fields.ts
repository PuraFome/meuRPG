import type { DamagePart } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { type ExtraPick, activeParts, partDice } from './damage-parts';

/** The extra die of Divine Smite: whether it counts depends on the target, so the server keeps it out of every group
 * (`inGroup` in `combat_parts.go`) and it keeps its own `part_key`. */
const OWN_KEY_PARTS: ReadonlySet<string> = new Set(['divine-smite-extra']);

/** One number the player types for physical dice: the sum of one damage type, or of one part that needs its own key. */
export interface TypedField {
  /** The `damage-type:` key of a type field, or the part's key of a part field; unique among the fields. */
  readonly key: string;
  /** Which of `damage_type` and `part_key` of a `TypedPart` the answer goes in. */
  readonly kind: 'type' | 'part';
  readonly label: string;
  /** "De 2 a 12.": what the sum may be (the dice as the server sends them, doubled on a critical hit). */
  readonly hint: string;
  readonly min: number;
  readonly max: number;
}

/** What goes on the wire for one typed sum: exactly one of the two keys. */
export type TypedEntry =
  | { readonly partKey: string; readonly sum: number }
  | { readonly damageType: string; readonly sum: number };

interface Rolled {
  readonly part: DamagePart;
  readonly count: number;
  readonly sides: number;
}

/** Whether the server can cover a part with the sum of its damage type. */
function groupable(p: DamagePart): boolean {
  return p.damageTypeKey.startsWith('damage-type:') && !OWN_KEY_PARTS.has(p.key);
}

function rangeHint(min: number, max: number): string {
  return min === max ? `Deve ser ${min}.` : `De ${min} a ${max}.`;
}

/**
 * The fields of the damage typed from physical dice: one for each damage type (a resistance acts on a type), in the
 * order the types first appear. A type with one part reads "Rapieira, perfurante: 1d8"; with several, one shared "Soma
 * dos dados: 1d8 + 1d6" over their combined range. A part the server needs its own key for (the extra die of Golpe
 * Divino), or with no damage type, keeps a field of its own. The dice are the parts' own counts, which the server already
 * sends doubled on a critical hit (`doubled`), so the ranges are the critical's.
 */
export function typedFields(
  parts: readonly DamagePart[],
  picks: readonly ExtraPick[],
): readonly TypedField[] {
  const groups = new Map<string, { own: boolean; rolled: Rolled[] }>();
  for (const part of activeParts(parts, picks)) {
    const { count, sides } = partDice(
      part,
      picks.find((x) => x.key === part.key),
    );
    if (count <= 0) {
      continue;
    }
    const own = !groupable(part);
    const key = own ? part.key : part.damageTypeKey;
    const group = groups.get(key) ?? { own, rolled: [] };
    group.rolled.push({ part, count, sides });
    groups.set(key, group);
  }
  const named = groups.size > 1;
  return [...groups].map(([key, g]) => field(key, g.own, g.rolled, named));
}

function field(key: string, own: boolean, rolled: readonly Rolled[], named: boolean): TypedField {
  const min = rolled.reduce((n, r) => n + r.count, 0);
  const max = rolled.reduce((n, r) => n + r.count * r.sides, 0);
  const formula = rolled.map((r) => `${r.count}d${r.sides}`).join(' + ');
  const typePt = rolled[0].part.damageTypePt;
  let label: string;
  if (rolled.length > 1) {
    label = `Soma dos dados${named && typePt ? `, ${typePt}` : ''}: ${formula}`;
  } else {
    label = `${rolled[0].part.labelPt}${typePt ? `, ${typePt}` : ''}: ${formula}`;
  }
  return { key, kind: own ? 'part' : 'type', label, hint: rangeHint(min, max), min, max };
}

/** The `typed_parts` of the request: the sums in the order of the fields, each under its type or its part. */
export function typedEntries(
  fields: readonly TypedField[],
  sums: readonly number[],
): readonly TypedEntry[] {
  return fields.map((f, i) =>
    f.kind === 'type' ? { damageType: f.key, sum: sums[i] } : { partKey: f.key, sum: sums[i] },
  );
}

/** "Sem o + 3: o app soma o bônus.": the fixed number the player does not type; nothing when there is none. */
export function typedBonusLine(flat: number): string {
  return flat === 0 ? '' : `Sem o ${flat < 0 ? '−' : '+'} ${Math.abs(flat)}: o app soma o bônus.`;
}
