import { ItemRechargeKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import type {
  SheetAmmunition,
  SheetMagicItem,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import type { DawnRestored } from '../../../gen/meurpg/play/v1/charges_pb';
import { chargesOf, type ItemChargesState } from '../characters/charges';
import { diceText, rechargeShort } from '../characters/recharge-text';

/** What the master's "Cargas e munição" reads of one character: the sheet's magic items and counted ammunition. */
export interface PartyMemberSheet {
  readonly characterId: string;
  readonly name: string;
  readonly magicItems: readonly SheetMagicItem[];
  readonly ammunition: readonly SheetAmmunition[];
}

/** One item with charges, as a row of the panel. */
export interface ChargesItemRow {
  readonly entryId: string;
  readonly name: string;
  readonly attuned: boolean;
  readonly max: number;
  readonly left: number;
  /** "5 de 7". */
  readonly value: string;
  /** "volta 1d6 + 1", empty for a lost item. */
  readonly volta: string;
  readonly state: ItemChargesState;
  readonly empty: boolean;
  readonly destroyed: boolean;
  /** A rolled start the master has not set. */
  readonly unset: boolean;
  /** How the dawn treats it, for the lists that say what waits for it. */
  readonly dawn: DawnRule | null;
}

/** The part of an item's recharge rules the dawn lists need. */
export interface DawnRule {
  readonly kind: ItemRechargeKind;
  readonly dice: string;
  readonly waitDawns: number;
}

export interface AmmunitionRow {
  readonly key: string;
  readonly name: string;
  readonly quantity: number;
}

export interface CharacterChargesVm {
  readonly characterId: string;
  readonly name: string;
  readonly items: readonly ChargesItemRow[];
  readonly ammunition: readonly AmmunitionRow[];
}

export interface PartyChargesVm {
  /** The characters with something to correct, in the party's order. */
  readonly characters: readonly CharacterChargesVm[];
  /** The names of the characters with no item with charges and no ammunition. */
  readonly quiet: readonly string[];
}

export function partyChargesOf(members: readonly PartyMemberSheet[]): PartyChargesVm {
  const characters: CharacterChargesVm[] = [];
  const quiet: string[] = [];
  for (const m of members) {
    const items: ChargesItemRow[] = [];
    for (const item of m.magicItems) {
      const c = chargesOf(item);
      if (!c) {
        continue;
      }
      const lost = c.state === 'destroyed' || c.state === 'plain' || c.state === 'partly';
      const recharge = item.recharge;
      items.push({
        entryId: item.id,
        name: item.namePt,
        attuned: item.attuned,
        max: c.max,
        left: c.left,
        value: `${c.left} de ${c.max}`,
        volta: lost ? '' : rechargeShort(recharge),
        state: c.state,
        empty: c.state === 'empty',
        destroyed: c.state === 'destroyed',
        unset: c.state === 'unset',
        dawn:
          recharge && recharge.kind !== ItemRechargeKind.UNSPECIFIED
            ? { kind: recharge.kind, dice: recharge.regainDice, waitDawns: recharge.waitDawns }
            : null,
      });
    }
    const ammunition = m.ammunition.map((a) => ({
      key: a.key,
      name: a.namePt,
      quantity: a.quantity,
    }));
    if (items.length === 0 && ammunition.length === 0) {
      quiet.push(m.name);
    } else {
      characters.push({ characterId: m.characterId, name: m.name, items, ammunition });
    }
  }
  return { characters, quiet };
}

/** "Brisa e Jabuti não têm itens com cargas nem munição." */
export function quietText(names: readonly string[]): string {
  if (names.length === 0) {
    return '';
  }
  const last = names[names.length - 1]!;
  const who = names.length === 1 ? last : `${names.slice(0, -1).join(', ')} e ${last}`;
  return `${who} ${names.length === 1 ? 'não tem' : 'não têm'} itens com cargas nem munição.`;
}

/** One line of "what the dawn would give back": whose item and how much. */
export interface DawnWaitingLine {
  readonly characterName: string;
  /** "Varinha de mísseis mágicos: 1d6 + 1, até 7 (tem 5)". */
  readonly text: string;
}

/**
 * The items a dawn touches, as the keeper's rules say them: a lost item, one that waits for its d20, one with an
 * unset start, one that never recharges and one that is full are left out, and an item that waits for dawns after its
 * last charge counts one only with none left.
 */
export function dawnWaiting(view: PartyChargesVm): DawnWaitingLine[] {
  const lines: DawnWaitingLine[] = [];
  for (const c of view.characters) {
    for (const item of c.items) {
      const rule = item.dawn;
      if (!rule || (item.state !== 'ready' && item.state !== 'empty') || item.left >= item.max) {
        continue;
      }
      if (rule.kind === ItemRechargeKind.DAWNS_AFTER_EMPTY && item.left > 0) {
        continue;
      }
      const back =
        rule.kind === ItemRechargeKind.DAWNS_AFTER_EMPTY
          ? `todas depois de ${rule.waitDawns} amanheceres`
          : rule.dice
            ? diceText(rule.dice)
            : 'todas';
      lines.push({
        characterName: c.name,
        text: `${item.name}: ${back}, até ${item.max} (tem ${item.left})`,
      });
    }
  }
  return lines;
}

export interface DawnResult {
  /** "Amanhecer. 3 itens receberam cargas:", or the sentence that no item waits. */
  readonly head: string;
  readonly lines: readonly string[];
}

/** What the master's "Amanhecer" answered: each item's dice, what it regained and where it stands now. */
export function dawnResultOf(
  restored: readonly DawnRestored[],
  nameOf: (characterId: string) => string,
): DawnResult {
  if (restored.length === 0) {
    return { head: 'Nenhum item espera o amanhecer.', lines: [] };
  }
  const lines = restored.map((r) => {
    const item = r.item;
    const dice = item?.recharge?.regainDice;
    const back = dice ? `${diceText(dice)} = ${r.regained}` : 'todas as cargas';
    return `${item?.namePt ?? ''} de ${nameOf(r.characterId)}: ${back}, fica com ${item?.chargesLeft ?? 0} de ${item?.chargesMax ?? 0}.`;
  });
  const n = restored.length;
  return {
    head: `Amanhecer. ${n} ${n === 1 ? 'item recebeu' : 'itens receberam'} cargas:`,
    lines,
  };
}
