import {
  ConsumableKind as ServerKind,
  type SheetConsumable,
} from '../../../gen/meurpg/characters/v1/characters_pb';

/**
 * What a consumable entry of the sheet is and costs, as the server says it (`SheetMagicItem.consumable`). The web reads it
 * and decides nothing: which items are consumables, which are oils and which cost the action are the server's rules.
 */

/** What a consumable is: a potion (drunk or given), an oil (applied), a scroll, or any other single-use item. */
export type ConsumableKind = 'potion' | 'oil' | 'scroll' | 'other';

/** The dice of a healing use: how many, their faces and the number added; `text` is the formula as a line writes it. */
export interface HealingDice {
  readonly count: number;
  readonly sides: number;
  readonly modifier: number;
  readonly text: string;
}

/** An entry of the sheet that "Usar" accepts. */
export interface ConsumableVm {
  readonly kind: ConsumableKind;
  /** Using it spends the user's action in a running combat. */
  readonly costsAction: boolean;
  /** The healing roll of the use; `null` when the use rolls nothing. */
  readonly heal: HealingDice | null;
}

const KINDS: Record<number, ConsumableKind> = {
  [ServerKind.POTION]: 'potion',
  [ServerKind.OIL]: 'oil',
  [ServerKind.SCROLL]: 'scroll',
};

/** The text a number added to the dice is said with: "+ 2", "− 1". */
export function signedText(modifier: number): string {
  return `${modifier < 0 ? '−' : '+'} ${Math.abs(modifier)}`;
}

/** The consumable of an entry, or `undefined` for an item that is none (the server leaves the field empty). */
export function consumableOf(c: SheetConsumable | undefined): ConsumableVm | undefined {
  if (!c) {
    return undefined;
  }
  const heal: HealingDice | null =
    c.healDiceCount > 0 && c.healDiceSides > 0
      ? {
          count: c.healDiceCount,
          sides: c.healDiceSides,
          modifier: c.healBonus,
          text: `${c.healDiceCount}d${c.healDiceSides}${c.healBonus === 0 ? '' : ` ${signedText(c.healBonus)}`}`,
        }
      : null;
  return { kind: KINDS[c.kind] ?? 'other', costsAction: c.costsAction, heal };
}
