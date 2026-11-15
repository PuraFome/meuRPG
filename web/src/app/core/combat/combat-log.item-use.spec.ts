import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
  ItemUseOutcome,
  ItemUseLogLineSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { ItemLoss } from '../../../gen/meurpg/characters/v1/characters_pb';
import { logGroups, logLine } from './combat-log';

const entry = (
  kind: CombatLogKind,
  actorLabel: string,
  use: MessageInitShape<typeof ItemUseLogLineSchema> | null,
  over: MessageInitShape<typeof CombatLogEntrySchema> = {},
): CombatLogEntry =>
  create(CombatLogEntrySchema, {
    id: 'x',
    kind,
    actorLabel,
    itemUse: use ? create(ItemUseLogLineSchema, use) : undefined,
    ...over,
  });

/** The line as the log draws it: the actor in bold, then the text. */
const said = (e: CombatLogEntry) => {
  const line = logLine(e);
  return line ? `${line.actor}${line.text}`.replace(/ /g, ' ') : null;
};

describe('the lines of charges and ammunition in the log (the master and the owner read them)', () => {
  describe('ITEM_CHARGE_SPENT', () => {
    const spent = (use: MessageInitShape<typeof ItemUseLogLineSchema>) =>
      entry(CombatLogKind.ITEM_CHARGE_SPENT, 'Pensantus', use);

    it('says the last charge of the item, as the board has it', () => {
      expect(
        said(
          spent({
            itemNamePt: 'Varinha de teia',
            amount: 1,
            left: 0,
            max: 7,
            outcome: ItemUseOutcome.LAST_CHARGE,
          }),
        ),
      ).toBe('Pensantus gastou a última carga da Varinha de teia');
    });

    it('says how many charges went and how many are left', () => {
      expect(
        said(
          spent({
            itemNamePt: 'Varinha de mísseis mágicos',
            amount: 2,
            left: 3,
            max: 7,
            outcome: ItemUseOutcome.SPENT,
          }),
        ),
      ).toBe('Pensantus gastou 2 cargas da Varinha de mísseis mágicos: restam 3 de 7');
      expect(
        said(
          spent({
            itemNamePt: 'Bordão do poder',
            amount: 1,
            left: 1,
            max: 7,
            outcome: ItemUseOutcome.SPENT,
          }),
        ),
      ).toBe('Pensantus gastou 1 carga do Bordão do poder: resta 1 de 7');
    });

    it.each([
      [ItemLoss.GONE, 'a Varinha de teia virou cinzas'],
      [ItemLoss.PLAIN_ITEM, 'a Varinha de teia virou um item comum'],
      [ItemLoss.PARTLY_MAGIC, 'a Varinha de teia perdeu a magia e ficou só com o bônus'],
    ])('says what became of the item lost with its last charge (%s)', (lost, what) => {
      expect(
        said(
          spent({
            itemNamePt: 'Varinha de teia',
            amount: 1,
            left: 0,
            max: 7,
            outcome: ItemUseOutcome.LOST,
            lost,
          }),
        ),
      ).toBe(`Pensantus gastou a última carga da Varinha de teia: ${what}`);
    });

    it('has an icon and writes nothing for an entry with no item_use', () => {
      expect(
        logLine(spent({ itemNamePt: 'Varinha de teia', outcome: ItemUseOutcome.SPENT }))?.icon,
      ).toBe('bolt');
      expect(logLine(entry(CombatLogKind.ITEM_CHARGE_SPENT, 'Pensantus', null))).toBeNull();
    });
  });

  describe('ITEM_LAST_CHARGE_ROLLED', () => {
    const rolled = (use: MessageInitShape<typeof ItemUseLogLineSchema>) =>
      entry(CombatLogKind.ITEM_LAST_CHARGE_ROLLED, 'Pensantus', {
        itemNamePt: 'Varinha de teia',
        ...use,
      });

    it('says the d20 and that the item turned to ashes, as the board has it', () => {
      expect(said(rolled({ d20: 1, outcome: ItemUseOutcome.LOST, lost: ItemLoss.GONE }))).toBe(
        'Pensantus rolou 1 no d20 da última carga: a Varinha de teia virou cinzas',
      );
    });

    it('says what is left of an item that was not destroyed', () => {
      expect(
        said(rolled({ d20: 1, outcome: ItemUseOutcome.LOST, lost: ItemLoss.PLAIN_ITEM })),
      ).toBe('Pensantus rolou 1 no d20 da última carga: a Varinha de teia virou um item comum');
      expect(
        said(rolled({ d20: 1, outcome: ItemUseOutcome.LOST, lost: ItemLoss.PARTLY_MAGIC })),
      ).toBe(
        'Pensantus rolou 1 no d20 da última carga: a Varinha de teia perdeu a magia e ficou só com o bônus',
      );
    });

    it('says the item stays with no charges', () => {
      expect(said(rolled({ d20: 14, outcome: ItemUseOutcome.STAYS }))).toBe(
        'Pensantus rolou 14 no d20 da última carga: a Varinha de teia fica, sem cargas',
      );
    });

    it('says the charges a 20 gave back', () => {
      expect(said(rolled({ d20: 20, amount: 4, outcome: ItemUseOutcome.REGAINED }))).toBe(
        'Pensantus rolou 20 no d20 da última carga: a Varinha de teia recuperou 4 cargas',
      );
      expect(said(rolled({ d20: 20, amount: 1, outcome: ItemUseOutcome.REGAINED }))).toBe(
        'Pensantus rolou 20 no d20 da última carga: a Varinha de teia recuperou 1 carga',
      );
    });

    it('has the dice icon', () => {
      expect(logLine(rolled({ d20: 1, outcome: ItemUseOutcome.LOST }))?.icon).toBe('casino');
    });
  });

  describe('AMMUNITION_SPENT', () => {
    const shot = (use: MessageInitShape<typeof ItemUseLogLineSchema>) =>
      entry(CombatLogKind.AMMUNITION_SPENT, 'Tor', {
        itemNamePt: 'Arco longo',
        ammunitionNamePt: 'Flecha',
        outcome: ItemUseOutcome.SPENT,
        ...use,
      });

    it('says the weapon, the piece and what is left, as the board has it', () => {
      expect(said(shot({ amount: 1, left: 3 }))).toBe(
        'Tor atirou com o Arco longo: 1 flecha, restam 3',
      );
    });

    it('says "resta 1" for the last piece but one and pluralizes the kinds the server names', () => {
      expect(said(shot({ amount: 1, left: 1 }))).toBe(
        'Tor atirou com o Arco longo: 1 flecha, resta 1',
      );
      expect(said(shot({ amount: 2, left: 5 }))).toBe(
        'Tor atirou com o Arco longo: 2 flechas, restam 5',
      );
      expect(
        said(
          shot({
            itemNamePt: 'Besta leve',
            ammunitionNamePt: 'Virote de besta',
            amount: 2,
            left: 0,
          }),
        ),
      ).toBe('Tor atirou com a Besta leve: 2 virotes de besta, restam 0');
      expect(
        said(
          shot({ itemNamePt: 'Funda', ammunitionNamePt: 'Projétil de funda', amount: 2, left: 4 }),
        ),
      ).toBe('Tor atirou com a Funda: 2 projéteis de funda, restam 4');
    });

    it('has the arrow icon', () => {
      expect(logLine(shot({ amount: 1, left: 3 }))?.icon).toBe('north_east');
    });
  });

  it('is grouped by round like every other line and ends with "(pelo mestre)" for a master action', () => {
    const lines = logGroups(
      [
        {
          round: 5,
          entries: [
            entry(
              CombatLogKind.ITEM_CHARGE_SPENT,
              'Pensantus',
              { itemNamePt: 'Varinha de teia', amount: 1, outcome: ItemUseOutcome.LAST_CHARGE },
              { byMaster: true },
            ),
          ],
        } as never,
      ],
      5,
    )[0].lines;
    expect(lines.map((l) => l.text)).toContain(
      ' gastou a última carga da Varinha de teia (pelo mestre)',
    );
  });
});
