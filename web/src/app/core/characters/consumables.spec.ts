import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  ConsumableTargetSchema,
  ConsumableTargetState,
  UseConsumableResponseSchema,
} from '../../../gen/meurpg/play/v1/charges_pb';
import { ConsumableUseOutcome } from '../../../gen/meurpg/play/v1/combat_pb';
import type { AttunementEntryVm } from './attunement';
import {
  consumableKind,
  costsAction,
  effectIcon,
  effectLine,
  healingDice,
  leftWords,
  magicRows,
  subtitleOf,
  targetRows,
  useNotice,
} from './consumables';

const entry = (over: Partial<AttunementEntryVm> = {}): AttunementEntryVm => ({
  id: 'm1',
  key: 'item:potion-of-healing',
  namePt: 'Poção de cura',
  kind: 'magic',
  attuned: false,
  asks: false,
  restrictionPt: '',
  restriction: 'none',
  sameItemAttuned: false,
  effectPt: 'Cura 2d4 + 2',
  ...over,
});

const CATEGORY = new Map([
  ['item:potion-of-healing', 'potion'],
  ['item:potion-of-fire-resistance', 'potion'],
  ['item:spell-scroll', 'scroll'],
  ['item:ring-of-protection', 'ring'],
]);
const categoryOf = (key: string) => CATEGORY.get(key) ?? '';

describe('consumableKind', () => {
  it('knows the potions and the scrolls by the catalog category, and nothing else', () => {
    expect(consumableKind('potion')).toBe('potion');
    expect(consumableKind('scroll')).toBe('scroll');
    expect(consumableKind('ring')).toBeNull();
    expect(consumableKind('')).toBeNull();
  });
});

describe('magicRows', () => {
  it('makes one row of the entries that share a key, at the first one, and sends the id of one of them', () => {
    const rows = magicRows(
      [
        entry({ id: 'p1' }),
        entry({ id: 'r1', key: 'item:ring-of-protection', namePt: 'Anel', effectPt: '' }),
        entry({ id: 'p2' }),
        entry({
          id: 'f1',
          key: 'item:potion-of-fire-resistance',
          namePt: 'Poção de resistência',
          effectPt: 'Resistência a fogo por 1 hora',
        }),
      ],
      categoryOf,
    );
    expect(rows.map((r) => r.id)).toEqual(['consumable:item:potion-of-healing', 'r1', 'consumable:item:potion-of-fire-resistance']);
    const heal = rows[0];
    expect(heal.type === 'consumable' && heal.row.count).toBe(2);
    expect(heal.type === 'consumable' && heal.row.first.id).toBe('p1');
    expect(heal.type === 'consumable' && heal.row.icon).toBe('science');
    expect(rows[1].type).toBe('item');
  });

  it('draws a page for a scroll and a flask for a potion', () => {
    const rows = magicRows(
      [entry({ key: 'item:spell-scroll', namePt: 'Pergaminho de magia', effectPt: '' })],
      categoryOf,
    );
    expect(rows[0].type === 'consumable' && rows[0].row.icon).toBe('description');
    expect(rows[0].type === 'consumable' && rows[0].row.kind).toBe('scroll');
  });

  it('keeps an item that is no consumable as the row it was, even with the same key twice', () => {
    const rows = magicRows(
      [
        entry({ id: 'a', key: 'item:ring-of-protection', effectPt: '' }),
        entry({ id: 'b', key: 'item:ring-of-protection', effectPt: '' }),
      ],
      categoryOf,
    );
    expect(rows.map((r) => r.type)).toEqual(['item', 'item']);
  });
});

describe('the effect line', () => {
  it('says what the server says, or that the item only shows its text', () => {
    expect(effectLine('Cura 2d4 + 2')).toBe('Cura 2d4 + 2');
    expect(effectLine('')).toBe('Só mostra o texto');
  });

  it('draws a heart for a healing, a shield for a resistance and a book for the text alone', () => {
    expect(effectIcon('Cura 2d4 + 2')).toBe('favorite');
    expect(effectIcon('Resistência a fogo por 1 hora')).toBe('shield');
    expect(effectIcon('')).toBe('menu_book');
  });
});

describe('healingDice', () => {
  it('reads the dice and the number added of a healing line', () => {
    expect(healingDice('Cura 2d4 + 2')).toEqual({ count: 2, sides: 4, modifier: 2, text: '2d4 + 2' });
    expect(healingDice('Cura 10d4 + 20')).toEqual({
      count: 10,
      sides: 4,
      modifier: 20,
      text: '10d4 + 20',
    });
  });

  it('reads a healing with no number added', () => {
    expect(healingDice('Cura 3d6')).toEqual({ count: 3, sides: 6, modifier: 0, text: '3d6' });
  });

  it('has no dice for the lines that are no healing', () => {
    expect(healingDice('Resistência a fogo por 1 hora')).toBeNull();
    expect(healingDice('')).toBeNull();
  });
});

describe('costsAction', () => {
  it('is only for the potions the app applies, the healing and the resistance', () => {
    expect(costsAction('potion', 'Cura 2d4 + 2')).toBe(true);
    expect(costsAction('potion', 'Resistência a fogo por 1 hora')).toBe(true);
    expect(costsAction('potion', '')).toBe(false);
    expect(costsAction('scroll', '')).toBe(false);
  });
});

describe('the words under the title', () => {
  it('says how many are left', () => {
    expect(leftWords(1)).toBe('resta 1');
    expect(leftWords(2)).toBe('restam 2');
  });

  it('opens the owner subtitle with the count and adds what a combat costs', () => {
    expect(subtitleOf({ itemName: 'Poção de cura', left: 2, actingFor: null, inCombat: true, costsAction: true })).toBe(
      'Restam 2 · em combate, gasta a ação',
    );
  });

  it('names the item first for the master, whose title names the character', () => {
    expect(
      subtitleOf({ itemName: 'Poção de resistência', left: 1, actingFor: 'Tor', inCombat: false, costsAction: true }),
    ).toBe('Poção de resistência · resta 1');
  });

  it('keeps the cost out of the line when the page knows there is no combat', () => {
    expect(subtitleOf({ itemName: 'Poção de cura', left: 1, actingFor: null, inCombat: false, costsAction: true })).toBe(
      'Resta 1',
    );
  });

  it('says no cost for an item that costs no action, in a combat or not', () => {
    expect(
      subtitleOf({ itemName: 'Poção de escalada', left: 1, actingFor: null, inCombat: true, costsAction: false }),
    ).toBe('Resta 1');
  });

  it('keeps the cost when the sheet does not know whether there is a combat', () => {
    expect(subtitleOf({ itemName: 'Poção de cura', left: 3, actingFor: null, inCombat: null, costsAction: true })).toBe(
      'Restam 3 · em combate, gasta a ação',
    );
  });
});

describe('targetRows', () => {
  const target = (name: string, distanceFt: number, state: ConsumableTargetState) =>
    create(ConsumableTargetSchema, {
      characterId: `id-${name}`,
      name,
      distanceFt,
      state,
      mayReceive: true,
    });

  it('says the distance and how the character stands, as the board does', () => {
    const rows = targetRows([
      target('Brisa', 5, ConsumableTargetState.DOWN),
      target('Jabuti', 5, ConsumableTargetState.OK),
    ]);
    expect(rows.map((r) => [r.id, r.name, r.line, r.down])).toEqual([
      ['id-Brisa', 'Brisa', 'A 1,5\u00a0m · Caída', true],
      ['id-Jabuti', 'Jabuti', 'A 1,5\u00a0m · Ileso', false],
    ]);
  });

  it('says nothing of a distance or of how they stand out of a combat, where the server tells neither', () => {
    const rows = targetRows([
      target('Pensantus', 0, ConsumableTargetState.UNSPECIFIED),
      target('Lia', 0, ConsumableTargetState.UNSPECIFIED),
    ]);
    expect(rows.map((r) => [r.line, r.down])).toEqual([
      ['', false],
      ['', false],
    ]);
  });

  it('leaves out who may not receive it', () => {
    const rows = targetRows([
      create(ConsumableTargetSchema, { characterId: 'x', name: 'Zé', mayReceive: false }),
    ]);
    expect(rows).toEqual([]);
  });
});

describe('useNotice', () => {
  const healed = (over: MessageInitShape<typeof UseConsumableResponseSchema> = {}) =>
    create(UseConsumableResponseSchema, {
      outcome: ConsumableUseOutcome.HEALED,
      diceText: '2d4+2',
      faces: [3, 2],
      rollTotal: 7,
      healed: 7,
      ...over,
    });
  const base = {
    itemName: 'Poção de cura',
    effectPt: 'Cura 2d4 + 2',
    userName: 'Tor',
    receiverName: 'Brisa',
    gift: true,
    woke: true,
    left: 1,
  };

  it('says who got the hit points and wakes, with the dice written out and what is left', () => {
    expect(useNotice(healed(), base)).toEqual({
      tone: 'good',
      icon: 'favorite',
      headline: 'Brisa recebeu 7 PV e acorda.',
      detail: '2d4 + 2: 3 + 2 + 2 = 7. Resta 1 Poção de cura.',
      text: [],
    });
  });

  it('does not wake who was on their feet', () => {
    expect(useNotice(healed(), { ...base, woke: false, receiverName: 'Tor', gift: false }).headline).toBe(
      'Tor recebeu 7 PV.',
    );
  });

  it('says what is left when none is, and when there are several', () => {
    expect(useNotice(healed(), { ...base, left: 0 }).detail).toContain('Era a última unidade.');
    expect(useNotice(healed(), { ...base, left: 3 }).detail).toContain(
      'Restam 3 unidades de Poção de cura.',
    );
  });

  it('says the potion did nothing for who had all their hit points', () => {
    const n = useNotice(healed({ healed: 0, faces: [1, 1], rollTotal: 4 }), {
      ...base,
      woke: false,
    });
    expect(n.headline).toBe('Brisa já estava com os PV cheios: a poção não curou nada.');
    expect(n.detail).toBe('2d4 + 2: 1 + 1 + 2 = 4. Resta 1 Poção de cura.');
  });

  it('writes a number taken off the roll with its sign', () => {
    const n = useNotice(healed({ faces: [2], rollTotal: 1, healed: 1, diceText: '1d4-1' }), {
      ...base,
      effectPt: 'Cura 1d4 − 1',
      woke: false,
    });
    expect(n.detail).toContain('1d4 − 1: 2 − 1 = 1.');
  });

  it('lands a resistance in "Seus efeitos" and says so in a line', () => {
    const n = useNotice(
      create(UseConsumableResponseSchema, {
        outcome: ConsumableUseOutcome.RESISTANCE,
        damageTypeKey: 'damage:fire',
      }),
      {
        ...base,
        itemName: 'Poção de resistência',
        effectPt: 'Resistência a fogo por 1 hora',
        receiverName: 'Tor',
        gift: false,
        woke: false,
      },
    );
    expect(n).toEqual({
      tone: 'good',
      icon: 'check_circle',
      headline: 'Tor bebeu a Poção de resistência: resistência a fogo por 1 hora.',
      detail: '',
      text: [],
    });
  });

  it('says a resistance given to someone was received', () => {
    const n = useNotice(
      create(UseConsumableResponseSchema, { outcome: ConsumableUseOutcome.RESISTANCE }),
      { ...base, itemName: 'Poção de resistência', effectPt: 'Resistência a fogo por 1 hora' },
    );
    expect(n.headline).toBe('Brisa recebeu a Poção de resistência: resistência a fogo por 1 hora.');
  });

  it('is neutral for a potion with no effect of its own and brings the text of the item', () => {
    const n = useNotice(
      create(UseConsumableResponseSchema, {
        outcome: ConsumableUseOutcome.TEXT_ONLY,
        text: ['Por 1 hora, você escala com o mesmo deslocamento.'],
      }),
      { ...base, itemName: 'Poção de escalada', effectPt: '', gift: false, receiverName: 'Tor' },
    );
    expect(n).toEqual({
      tone: 'neutral',
      icon: 'menu_book',
      headline: 'Tor bebeu a Poção de escalada.',
      detail: 'O app não aplica este efeito: o mestre e você seguem o texto.',
      text: ['Por 1 hora, você escala com o mesmo deslocamento.'],
    });
  });

  it('says who the potion was given to when it has no effect of its own', () => {
    const n = useNotice(create(UseConsumableResponseSchema, { outcome: ConsumableUseOutcome.TEXT_ONLY }), {
      ...base,
      itemName: 'Poção de escalada',
      effectPt: '',
    });
    expect(n.headline).toBe('Tor deu a Poção de escalada a Brisa.');
  });
});
