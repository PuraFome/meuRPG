import type {
  ItemRecharge,
  SheetAmmunition,
  SheetMagicItem,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { ItemLoss } from '../../../gen/meurpg/characters/v1/characters_pb';
import { emptyLine, lostLine, rechargeLine, startLine } from './recharge-text';

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
  /** What the catalog says of how the item recharges and what its last charge does; absent for an item with no such rules. */
  readonly recharge?: ItemRecharge;
}

/** The charges of an item, or `undefined` for one with no charge data (`charges_max` 0): its row stays as it is. */
export function chargesOf(m: SheetMagicItem): ItemChargesVm | undefined {
  if (m.chargesMax <= 0) {
    return undefined;
  }
  const left = Math.max(0, m.chargesLeft);
  const max = m.chargesMax;
  const recharge = m.recharge;
  switch (m.lost) {
    case ItemLoss.GONE:
      return { max, left: 0, state: 'destroyed', recharge };
    case ItemLoss.PLAIN_ITEM:
      return { max, left: 0, state: 'plain', recharge };
    case ItemLoss.PARTLY_MAGIC:
      return { max, left: 0, state: 'partly', recharge };
  }
  if (m.lastRollPending) {
    return { max, left, state: 'pending', recharge };
  }
  if (m.chargesLeft < 0) {
    return { max, left: 0, state: 'unset', recharge };
  }
  return { max, left, state: left === 0 ? 'empty' : 'ready', recharge };
}

/** The line under an item row that says why it cannot be used (or what it waits for), with its icon. */
export interface ChargesLine {
  readonly icon: string;
  readonly text: string;
  /** The line asks something of the owner: drawn as a warning. */
  readonly warn?: boolean;
}

/**
 * The line under an item row. With the item's recharge rules it says them (when the charges come back, the dice of a rolled
 * start, what a lost item is now); without them it says only why the item cannot be used. `name` is the catalog name, which
 * the noun of "Virou um bordão comum." comes from.
 */
export function chargesLine(c: ItemChargesVm, name = ''): ChargesLine | null {
  const r = c.recharge;
  switch (c.state) {
    case 'empty':
      return { icon: 'block', text: r ? emptyLine(r) : 'Sem cargas.' };
    case 'unset':
      return { icon: 'info', text: r ? startLine(r) : 'O mestre define as cargas iniciais.' };
    case 'pending':
      return { icon: 'warning', text: 'Falta rolar o d20 da última carga.', warn: true };
    case 'destroyed':
      return { icon: 'block', text: r ? lostLine(name, r, 'destroyed') : 'A última carga se foi.' };
    case 'plain':
      return {
        icon: 'block',
        text: r && name ? lostLine(name, r, 'plain') : 'Virou um item comum.',
      };
    case 'partly':
      return {
        icon: 'block',
        text: r ? lostLine(name, r, 'partly') : 'Ficou só o bônus no ataque e no dano.',
      };
    default:
      return rechargeLine(r);
  }
}

/** One of the four kinds of ammunition of the SRD, and the bundle it comes in. */
export interface AmmunitionKind {
  readonly key: string;
  /** The heading of the kind: "Flechas". */
  readonly namePt: string;
  /** What fires it: "Arcos". */
  readonly usedBy: string;
  readonly bundle: number;
  /** One piece and several, in lower case: "flecha", "flechas". */
  readonly one: string;
  readonly many: string;
}

export const AMMUNITION_KINDS: readonly AmmunitionKind[] = [
  {
    key: 'equipment:arrow',
    namePt: 'Flechas',
    usedBy: 'Arcos',
    bundle: 20,
    one: 'flecha',
    many: 'flechas',
  },
  {
    key: 'equipment:crossbow-bolt',
    namePt: 'Virotes',
    usedBy: 'Bestas',
    bundle: 20,
    one: 'virote',
    many: 'virotes',
  },
  {
    key: 'equipment:sling-bullet',
    namePt: 'Balas de funda',
    usedBy: 'Funda',
    bundle: 20,
    one: 'bala de funda',
    many: 'balas de funda',
  },
  {
    key: 'equipment:blowgun-needle',
    namePt: 'Agulhas de zarabatana',
    usedBy: 'Zarabatana',
    bundle: 50,
    one: 'agulha de zarabatana',
    many: 'agulhas de zarabatana',
  },
];

/** The kind of an ammunition catalog key (`Attack.ammunition_key`); `undefined` for a key the table does not know. */
export function ammunitionKindOfKey(ammunitionKey: string): AmmunitionKind | undefined {
  return AMMUNITION_KINDS.find((k) => k.key === ammunitionKey);
}

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

export { articleOf } from './article-of';
