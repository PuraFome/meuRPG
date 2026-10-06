import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { MagicItemRarity, TreasureMode } from '../../../gen/meurpg/maps/v1/treasure_pb';
import {
  attunementText,
  coinRows,
  goldLine,
  groupItems,
  itemTags,
  itemValueRule,
  itemValueText,
  partyHelp,
  pieceRows,
  placedXpLine,
  goldKinds,
  placedSummary,
  po,
  rarityText,
  treasureTitle,
} from './treasure-format';
import { hoardItems, magicItemResponse, sampleHoard, treasureItem } from './treasure-testing';

const plain = (s: string) => s.replace(/ /g, ' ');

describe('treasure-format: what the server rolled, in words (MR-044)', () => {
  it('writes PO with a thousands dot and a no-break space between the number and the unit', () => {
    expect(po(515)).toBe('515 PO');
    expect(plain(po(5365))).toBe('5.365 PO');
    expect(plain(po(200000))).toBe('200.000 PO');
  });

  it('the coin rows say what each stack is worth, and what a silver piece is: 10 PP = 1 PO', () => {
    const rows = coinRows(sampleHoard().coins);
    expect(rows.map((r) => [plain(r.count), r.rate, plain(r.value)])).toEqual([
      ['1.200 PP', '10 PP valem 1 PO', '120 PO'],
      ['340 PO', '', '340 PO'],
    ]);
  });

  it('gems and art: "2 × Ágata" with "6 PO cada" and the stack\'s value; one piece has no "cada"', () => {
    const t = sampleHoard();
    expect(pieceRows(t.gems).map((r) => [r.title, plain(r.each), plain(r.value)])).toEqual([
      ['2 × Ágata', '6 PO cada', '12 PO'],
      ['Quartzo azul', '', '18 PO'],
    ]);
    expect(pieceRows(t.art).map((r) => [r.title, plain(r.value)])).toEqual([['Cálice de prata gravado', '27 PO']]);
  });

  it('identical magic items are one row with "2 ×", in the order each first came up', () => {
    const potion = hoardItems()[0]!;
    const items = [potion, hoardItems()[3]!, potion, potion];
    const groups = groupItems(items);
    expect(groups.map((g) => g.title)).toEqual(['3 × Poção de Cura', 'Anel de Proteção']);
    expect(plain(groups[0]!.value)).toBe('150 PO');
    expect(plain(groups[0]!.each)).toBe('50 PO cada');
    expect(plain(groups[0]!.halvedNote)).toBe('metade de 100 PO');
    expect(groups[1]!.each).toBe('');
    expect(groups[1]!.halvedNote).toBe('');
  });

  it('the tags are words: the rarity, "Consumível" and "Exige sintonização"', () => {
    expect(itemTags(hoardItems()[0]!)).toEqual(['Comum', 'Consumível']);
    expect(itemTags(hoardItems()[3]!)).toEqual(['Raro', 'Exige sintonização']);
    expect(rarityText(MagicItemRarity.VERY_RARE)).toBe('Muito raro');
  });

  it('an attunement by a class says who, in Portuguese', () => {
    expect(attunementText({ attunement: true, attunementByPt: 'por um paladino' })).toBe('Exige sintonização por um paladino');
    expect(attunementText({ attunement: false, attunementByPt: '' })).toBe('');
  });

  it('the value of an item: the number, "sem preço" for an artifact, and the variants for a family', () => {
    expect(plain(itemValueText(magicItemResponse()))).toBe('4.000 PO');
    expect(itemValueText(magicItemResponse({ valuePo: undefined, priceless: true, rarity: MagicItemRarity.ARTIFACT }))).toBe('sem preço');
    expect(itemValueText(magicItemResponse({ valuePo: undefined, rarity: MagicItemRarity.VARIES }))).toBe('depende da variante');
  });

  it('the rule behind a value is said in words: the halving, the scroll that is never halved, the item that is not used up', () => {
    expect(plain(itemValueRule(magicItemResponse({ valuePo: 50, halved: true, consumable: true, rarity: MagicItemRarity.COMMON })))).toBe(
      'Comum vale 100 PO; um item que se gasta vale a metade.',
    );
    expect(itemValueRule(magicItemResponse({ valuePo: 400, consumable: true, spellScroll: true, rarity: MagicItemRarity.UNCOMMON }))).toContain('não é dividido ao meio');
    expect(itemValueRule(magicItemResponse())).toBe('Itens que se gastam valem a metade, menos os pergaminhos de magia. Este não se gasta.');
    expect(itemValueRule(magicItemResponse({ valuePo: undefined, priceless: true }))).toBe('Um artefato não tem preço.');
  });

  it('the party sentence follows the server: two levels, one level, nobody', () => {
    expect(partyHelp({ livingCount: 2, lowestLevel: 4, highestLevel: 5 })).toBe(
      'O grupo está nos níveis 4 e 5. A tabela usa o menor nível, para o tesouro não passar do que o grupo aguenta.',
    );
    expect(partyHelp({ livingCount: 3, lowestLevel: 4, highestLevel: 4 })).toContain('O grupo está no nível 4.');
    expect(partyHelp({ livingCount: 0, lowestLevel: 0, highestLevel: 0 })).toContain('ainda não tem personagem de jogador vivo');
    // A party that could not be read is the page's own notice, never this sentence.
    expect(partyHelp(null)).toBe('');
  });

  it('the gold line says what the gold does in each campaign (RN-09, MR-041)', () => {
    expect(goldLine(XpMode.ENEMIES, 'Mirathel')).toBe('Mirathel dá XP por inimigos: o ouro do tesouro não vira XP.');
    expect(goldLine(XpMode.GOLD, 'Estrada de Ouro')).toBe('Estrada de Ouro dá XP por ouro: o grupo converte isto em XP em “Voltar à cidade”.');
    expect(goldLine(XpMode.MILESTONES, 'Mirathel')).toBe('Mirathel sobe de nível por marcos: o ouro do tesouro não vira XP.');
  });

  it('the confirmation counts only the gold; the items stay in the description', () => {
    expect(plain(placedXpLine(XpMode.GOLD, 'Estrada de Ouro', 515, 4))).toBe('Estrada de Ouro dá XP por ouro: o grupo converte 515 PO em XP em “Voltar à cidade”. Os 4 itens ficam na descrição.');
    expect(plain(placedXpLine(XpMode.ENEMIES, 'Mirathel', 33, 0))).toBe('Mirathel dá XP por inimigos: o ouro do tesouro não vira XP.');
    expect(plain(placedXpLine(XpMode.ENEMIES, 'Mirathel', 515, 1))).toContain('O item fica na descrição.');
    expect(plain(placedSummary(515, 4, true))).toBe('515 PO em moedas, gemas e arte e 4 itens');
    expect(plain(placedSummary(515, 1, true))).toBe('515 PO em moedas, gemas e arte e 1 item');
    expect(plain(placedSummary(33, 0, false))).toBe('33 PO em moedas');
    expect(goldKinds(false)).toBe('em moedas');
  });

  it('titles the treasure by its kind and level', () => {
    expect(treasureTitle(TreasureMode.HOARD, 4)).toBe('Tesouro de covil · nível 4');
    expect(treasureTitle(TreasureMode.INDIVIDUAL, 4)).toBe('Tesouro individual · nível 4');
    expect(treasureItem().namePt).toBe('Anel de Proteção');
  });
});
