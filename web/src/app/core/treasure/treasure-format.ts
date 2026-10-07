import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  type GetMagicItemResponse,
  MagicItemRarity,
  TreasureCoinKind,
  type TreasureCoinStack,
  TreasureMode,
  type TreasureItem,
  type TreasurePiece,
} from '../../../gen/meurpg/maps/v1/treasure_pb';
import { formatInt, tight } from '../format/text';

/**
 * How the treasure screens write what the server returned (MR-044): the coins, the gems and the art,
 * the items grouped, the values with their units and the sentences about gold and XP. Plain
 * functions: the browser draws what the server rolled and rounded, it rolls nothing. The only
 * arithmetic is the sum of the two totals the server gave, for "Valor total", and the doubling that
 * says what a halved value was before the halving (the server says only the halved number).
 */

/** The label the values carry, as the server writes it (`GetMagicItemResponse.value_label`). */
export const VALUE_LABEL = 'Valores do SRD 5.2.1 (regras de 2024)';

/** Said under every value of an item, with "Créditos" next to it. */
export const CONSUMABLE_RULE = 'Itens que se gastam valem a metade, menos os pergaminhos de magia.';

/** "1.200 PO", "460 PO": the number and its unit tied by a no-break space, pt-BR thousands separator. */
export function po(value: number): string {
  return `${formatInt(value)}\u00a0PO`;
}

const COIN_ABBR: Readonly<Record<number, string>> = {
  [TreasureCoinKind.COPPER]: 'PC',
  [TreasureCoinKind.SILVER]: 'PP',
  [TreasureCoinKind.ELECTRUM]: 'PE',
  [TreasureCoinKind.GOLD]: 'PO',
  [TreasureCoinKind.PLATINUM]: 'PL',
};

/** The line under a coin that is not gold, saying what it is worth ("10 PP valem 1 PO"); `''` for the gold coin. */
const COIN_RATE: Readonly<Record<number, string>> = {
  [TreasureCoinKind.COPPER]: '100 PC valem 1 PO',
  [TreasureCoinKind.SILVER]: '10 PP valem 1 PO',
  [TreasureCoinKind.ELECTRUM]: '2 PE valem 1 PO',
  [TreasureCoinKind.GOLD]: '',
  [TreasureCoinKind.PLATINUM]: '1 PL vale 10 PO',
};

/** One coin row: "340 PO", then (for silver, copper, electrum and platinum) what it is worth, then its value in PO. */
export interface CoinRow {
  readonly key: string;
  readonly count: string;
  readonly rate: string;
  readonly value: string;
}

export function coinRows(coins: readonly TreasureCoinStack[]): CoinRow[] {
  return coins.map((c) => ({
    key: String(c.coin),
    count: `${formatInt(c.count)}\u00a0${COIN_ABBR[c.coin] ?? ''}`.trim(),
    rate: COIN_RATE[c.coin] ?? '',
    value: po(c.valuePo),
  }));
}

/** One kind of gem or art object, with how many of it: "2 × Ágata", "10 PO cada", "20 PO". */
export interface PieceRow {
  readonly name: string;
  readonly count: number;
  /** "2 × Ágata", or just the name for one. */
  readonly title: string;
  /** "10 PO cada" for more than one, `''` for one. */
  readonly each: string;
  readonly value: string;
}

export function pieceRows(pieces: readonly TreasurePiece[]): PieceRow[] {
  return pieces.map((p) => ({
    name: p.namePt,
    count: p.count,
    title: p.count > 1 ? `${p.count} × ${p.namePt}` : p.namePt,
    each: p.count > 1 ? `${po(p.valuePo)} cada` : '',
    value: po(p.valuePo * p.count),
  }));
}

/** The magic items of a treasure with the identical ones together ("2 ×"), in the order each first came up. */
export interface ItemGroup {
  readonly item: TreasureItem;
  readonly count: number;
  /** "2 × Poção de Cura", or just the name for one. */
  readonly title: string;
  /** What the group is worth: the item's value times how many. */
  readonly value: string;
  /** "50 PO cada" for a group of more than one, `''` for one. */
  readonly each: string;
  /** "metade de 100 PO" under the value of a consumable, `''` otherwise. */
  readonly halvedNote: string;
}

export function groupItems(items: readonly TreasureItem[]): ItemGroup[] {
  const groups: { item: TreasureItem; count: number }[] = [];
  for (const item of items) {
    const found = groups.find((g) => g.item.key === item.key);
    if (found) {
      found.count++;
    } else {
      groups.push({ item, count: 1 });
    }
  }
  return groups.map(({ item, count }) => ({
    item,
    count,
    title: count > 1 ? `${count} × ${item.namePt}` : item.namePt,
    value: po(item.valuePo * count),
    each: count > 1 ? `${po(item.valuePo)} cada` : '',
    halvedNote: item.halved ? `metade de ${po(item.valuePo * 2)}` : '',
  }));
}

const RARITY: Readonly<Record<number, string>> = {
  [MagicItemRarity.COMMON]: 'Comum',
  [MagicItemRarity.UNCOMMON]: 'Incomum',
  [MagicItemRarity.RARE]: 'Raro',
  [MagicItemRarity.VERY_RARE]: 'Muito raro',
  [MagicItemRarity.LEGENDARY]: 'Lendário',
  [MagicItemRarity.ARTIFACT]: 'Artefato',
  [MagicItemRarity.VARIES]: 'Varia',
};

/** "Incomum": the rarity as a tag, always a word. */
export function rarityText(rarity: MagicItemRarity): string {
  return RARITY[rarity] ?? '';
}

/** The tags of an item, each with its word: the rarity, "Consumível" and "Exige sintonização". */
export function itemTags(item: {
  rarity: MagicItemRarity;
  consumable: boolean;
  attunement: boolean;
}): string[] {
  return [
    rarityText(item.rarity),
    item.consumable ? 'Consumível' : '',
    item.attunement ? 'Exige sintonização' : '',
  ].filter((t) => t !== '');
}

/** "Exige sintonização por um paladino" for the sheet; plain "Exige sintonização" for anyone. */
export function attunementText(item: { attunement: boolean; attunementByPt: string }): string {
  if (!item.attunement) {
    return '';
  }
  return item.attunementByPt ? `Exige sintonização ${item.attunementByPt}` : 'Exige sintonização';
}

/** The value of one item in its sheet: "4.000 PO", "sem preço" for an artifact, or what a family says about its variants. */
export function itemValueText(item: GetMagicItemResponse): string {
  if (item.priceless) {
    return 'sem preço';
  }
  if (item.valuePo === undefined) {
    return 'depende da variante';
  }
  return po(item.valuePo);
}

/** The rule behind an item's value, said in words (state 3): the rarity's value, the halving or the scroll's exception. */
export function itemValueRule(item: GetMagicItemResponse): string {
  if (item.priceless) {
    return 'Um artefato não tem preço.';
  }
  if (item.valuePo === undefined) {
    return 'Este item tem raridades diferentes; o valor é o de cada variante.';
  }
  if (item.halved) {
    return `${rarityText(item.rarity)} vale ${po(item.valuePo * 2)}; um item que se gasta vale a metade.`;
  }
  if (item.spellScroll) {
    return 'Um pergaminho de magia vale o valor inteiro da raridade: ele não é dividido ao meio. A raridade vem da tabela do SRD 5.1, pelo nível da magia.';
  }
  return item.consumable ? CONSUMABLE_RULE : `${CONSUMABLE_RULE} Este não se gasta.`;
}

/** "De covil" or "Individual". */
export function modeText(mode: TreasureMode): string {
  return mode === TreasureMode.HOARD ? 'De covil' : 'Individual';
}

/** "Tesouro de covil · nível 4", "Tesouro individual · nível 4". */
export function treasureTitle(mode: TreasureMode, level: number): string {
  return `${mode === TreasureMode.HOARD ? 'Tesouro de covil' : 'Tesouro individual'} · nível ${level}`;
}

/** The sentence under "Nível do grupo": the party, then why the lowest level is the one the table reads. */
export function partyHelp(
  party: { livingCount: number; lowestLevel: number; highestLevel: number } | null,
): string {
  if (!party) {
    return '';
  }
  if (party.livingCount === 0) {
    return 'A campanha ainda não tem personagem de jogador vivo. Escolha o nível, de 1 a 20.';
  }
  const where =
    party.lowestLevel === party.highestLevel
      ? `O grupo está no nível ${party.lowestLevel}.`
      : `O grupo está nos níveis ${party.lowestLevel} e ${party.highestLevel}.`;
  return `${where} A tabela usa o menor nível, para o tesouro não passar do que o grupo aguenta.`;
}

/** What happens to the treasure's gold in this campaign (MR-041, RN-09), on the result, before it is put on the map. */
export function goldLine(xpMode: XpMode, campaignName: string): string {
  switch (xpMode) {
    case XpMode.GOLD:
      return `${campaignName} dá XP por ouro: o grupo converte isto em XP em “Voltar à cidade”.`;
    case XpMode.MILESTONES:
      return `${campaignName} sobe de nível por marcos: o ouro do tesouro não vira XP.`;
    default:
      return `${campaignName} dá XP por inimigos: o ouro do tesouro não vira XP.`;
  }
}

/** The same sentence once the treasure is on the map, with the amount and what stays in the description (E10-10 state 5). */
export function placedXpLine(
  xpMode: XpMode,
  campaignName: string,
  goldPo: number,
  itemCount: number,
): string {
  const items =
    itemCount === 0
      ? ''
      : itemCount === 1
        ? ' O item fica na descrição.'
        : ` Os ${itemCount} itens ficam na descrição.`;
  const line =
    xpMode === XpMode.GOLD
      ? `${campaignName} dá XP por ouro: o grupo converte ${po(goldPo)} em XP em “Voltar à cidade”.`
      : goldLine(xpMode, campaignName);
  return tight(`${line}${items}`);
}

/** What the gold is made of: a hoard's coins, gems and art, an individual treasure's coins only. */
export function goldKinds(hoard: boolean): string {
  return hoard ? 'em moedas, gemas e arte' : 'em moedas';
}

/** "515 PO em moedas, gemas e arte e 4 itens": what was put on the map, the gold first and the items apart. */
export function placedSummary(goldPo: number, itemCount: number, hoard: boolean): string {
  const items = itemCount === 0 ? '' : itemCount === 1 ? ' e 1 item' : ` e ${itemCount} itens`;
  return tight(`${po(goldPo)} ${goldKinds(hoard)}${items}`);
}
