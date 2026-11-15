import type {
  SheetAmmunition,
  SheetMagicItem,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { ItemLoss } from '../../../gen/meurpg/characters/v1/characters_pb';

/** Where an item with charges stands, as the server says it: `ready` can be used; the others cannot. */
export type ItemChargesState =
  | 'ready'
  /** No charge left. */
  | 'empty'
  /** An item that rolls its start and the master has not set it. */
  | 'unset'
  /** The last charge went from an item that asks a d20 and the owner has not rolled it. */
  | 'pending'
  | 'destroyed'
  /** Lost as a plain item: nothing of its magic is left. */
  | 'plain'
  /** Lost but for the bonus to attack and damage. */
  | 'partly';

/** The charges of one magic item of the sheet. `left` is 0 for a rolled start that is not set. */
export interface ItemChargesVm {
  readonly max: number;
  readonly left: number;
  readonly state: ItemChargesState;
}

/** The charges of an item, or `undefined` for one with no charge data (`charges_max` 0): its row stays as it is. */
export function chargesOf(m: SheetMagicItem): ItemChargesVm | undefined {
  if (m.chargesMax <= 0) {
    return undefined;
  }
  const left = Math.max(0, m.chargesLeft);
  const max = m.chargesMax;
  switch (m.lost) {
    case ItemLoss.GONE:
      return { max, left: 0, state: 'destroyed' };
    case ItemLoss.PLAIN_ITEM:
      return { max, left: 0, state: 'plain' };
    case ItemLoss.PARTLY_MAGIC:
      return { max, left: 0, state: 'partly' };
  }
  if (m.lastRollPending) {
    return { max, left, state: 'pending' };
  }
  if (m.chargesLeft < 0) {
    return { max, left: 0, state: 'unset' };
  }
  return { max, left, state: left === 0 ? 'empty' : 'ready' };
}

/** "5 de 7 cargas". */
export function chargesWords(c: ItemChargesVm): string {
  return `${c.left} de ${c.max} ${c.max === 1 ? 'carga' : 'cargas'}`;
}

/** The line under an item row that says why it cannot be used (or what it waits for), with its icon. */
export interface ChargesLine {
  readonly icon: string;
  readonly text: string;
  /** The line asks something of the owner: drawn as a warning. */
  readonly warn?: boolean;
}

export function chargesLine(c: ItemChargesVm): ChargesLine | null {
  switch (c.state) {
    case 'empty':
      return { icon: 'block', text: 'Sem cargas.' };
    case 'unset':
      return { icon: 'info', text: 'O mestre define as cargas iniciais.' };
    case 'pending':
      return { icon: 'warning', text: 'Falta rolar o d20 da última carga.', warn: true };
    case 'destroyed':
      return { icon: 'block', text: 'A última carga se foi.' };
    case 'plain':
      return { icon: 'block', text: 'Virou um item comum.' };
    case 'partly':
      return { icon: 'block', text: 'Ficou só o bônus no ataque e no dano.' };
    default:
      return null;
  }
}

/** One of the four kinds of ammunition of the SRD and the bundle it comes in. */
export interface AmmunitionKind {
  readonly key: string;
  readonly namePt: string;
  /** What fires it: "Arcos". */
  readonly usedBy: string;
  readonly bundle: number;
}

export const AMMUNITION_KINDS: readonly AmmunitionKind[] = [
  { key: 'equipment:arrow', namePt: 'Flechas', usedBy: 'Arcos', bundle: 20 },
  { key: 'equipment:crossbow-bolt', namePt: 'Virotes', usedBy: 'Bestas', bundle: 20 },
  { key: 'equipment:sling-bullet', namePt: 'Balas de funda', usedBy: 'Funda', bundle: 20 },
  {
    key: 'equipment:blowgun-needle',
    namePt: 'Agulhas de zarabatana',
    usedBy: 'Zarabatana',
    bundle: 50,
  },
];

/** What the character holds of one kind of ammunition and what the last combat did to it. */
export interface AmmunitionVm {
  readonly key: string;
  readonly namePt: string;
  readonly quantity: number;
  /** Fired in the character's latest combat. */
  readonly spent: number;
  /** Half of what was fired, rounded down: what the recovery brings back. */
  readonly back: number;
  /** The recovery of that combat was done. */
  readonly recovered: boolean;
}

export function ammunitionOf(list: readonly SheetAmmunition[]): AmmunitionVm[] {
  return list.map((a) => ({
    key: a.key,
    namePt: a.namePt,
    quantity: a.quantity,
    spent: a.spentLastCombat,
    back: Math.floor(a.spentLastCombat / 2),
    recovered: a.recovered,
  }));
}

/** The state of "Recuperar a metade": what it would bring back, `done` after it ran, `nothing` when no kind has a piece to take back. */
export interface RecoverVm {
  readonly state: 'ready' | 'done' | 'nothing';
  readonly count: number;
  /** "3 flechas", or "3 peças" for several kinds or one piece: what the recovery brings back. */
  readonly pieces: string;
  readonly label: string;
}

export function recoverOf(list: readonly AmmunitionVm[]): RecoverVm {
  const fired = list.filter((a) => a.back > 0);
  const count = fired.reduce((sum, a) => sum + a.back, 0);
  const first = fired[0];
  const pieces =
    first && fired.length === 1 && count > 1
      ? `${count} ${first.namePt.toLowerCase()}`
      : `${count} ${count === 1 ? 'peça' : 'peças'}`;
  const label = `Recuperar a metade: ${pieces}`;
  if (count === 0) {
    return { state: 'nothing', count: 0, pieces: '', label: 'Recuperar a metade' };
  }
  if (fired.every((a) => a.recovered)) {
    return { state: 'done', count, pieces, label };
  }
  return { state: 'ready', count, pieces, label };
}

/** The free-text item that looks like ammunition: the app counts a kind only once it is added in "Munição". */
export interface AmmunitionHint {
  readonly name: string;
  readonly key: string;
  readonly quantity: number;
}

const LOOKS_LIKE: readonly { readonly key: string; readonly pattern: RegExp }[] = [
  { key: 'equipment:arrow', pattern: /\bflechas?\b/i },
  { key: 'equipment:crossbow-bolt', pattern: /\bvirotes?\b/i },
  { key: 'equipment:sling-bullet', pattern: /\bbalas?\s+de\s+funda\b/i },
  { key: 'equipment:blowgun-needle', pattern: /\bagulhas?\b/i },
];

export function ammunitionHint(
  items: readonly { readonly name: string; readonly quantity: number }[],
): AmmunitionHint | null {
  for (const item of items) {
    const kind = LOOKS_LIKE.find((k) => k.pattern.test(item.name));
    if (kind) {
      const written = /\((\d{1,4})\)/.exec(item.name)?.[1];
      return {
        name: item.name,
        key: kind.key,
        quantity: written ? Number(written) : Math.max(item.quantity, 1),
      };
    }
  }
  return null;
}

const MASCULINE_IN_A = new Set(['mapa']);

/** The article a catalog name takes at the start of a sentence: "A Varinha de teia", "O Bordão" (the catalog carries no gender, so the first word's ending decides). */
export function articleOf(name: string): 'A' | 'O' {
  const first = (name.trim().split(/\s+/)[0] ?? '').toLowerCase();
  return first.endsWith('a') && !MASCULINE_IN_A.has(first) ? 'A' : 'O';
}
