import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import {
  FreeRollDieSchema,
  FreeRollMode,
  FreeRollSchema,
} from '../../../gen/meurpg/play/v1/free_dice_pb';
import {
  DEFAULT_FORM,
  type FreeForm,
  type FreeRollVm,
  audienceText,
  canAdvantage,
  formFromRoll,
  formulaOf,
  logParts,
  naturalWord,
  normalizeForm,
  repeatLabel,
  resultLine,
  rollButtonLabel,
  rollButtonName,
  summaryOf,
  toFreeRollVm,
  whenText,
} from './free-roll';

function roll(over: Partial<FreeRollVm> = {}): FreeRollVm {
  return {
    id: 'r1',
    rollerName: 'Brisa',
    byMaster: false,
    secret: false,
    sides: 6,
    count: 2,
    modifier: 3,
    mode: 'normal',
    dice: [
      { face: 4, discarded: false },
      { face: 4, discarded: false },
    ],
    total: 11,
    formula: '2d6 + 3',
    logText: 'Rolagem livre · Brisa: 2d6 + 3 = 11 (4, 4)',
    createdAt: new Date(2026, 9, 18, 20, 10),
    ...over,
  };
}

const form = (over: Partial<FreeForm> = {}): FreeForm => ({ ...DEFAULT_FORM, ...over });

describe('toFreeRollVm', () => {
  it('turns the wire roll into the view model, with the mode as a word', () => {
    const at = new Date(2026, 9, 18, 20, 10);
    const vm = toFreeRollVm(
      create(FreeRollSchema, {
        id: 'r9',
        rollerName: 'Brisa',
        sides: 20,
        count: 1,
        modifier: 5,
        mode: FreeRollMode.ADVANTAGE,
        dice: [
          create(FreeRollDieSchema, { face: 14 }),
          create(FreeRollDieSchema, { face: 9, discarded: true }),
        ],
        total: 19,
        formula: '1d20 + 5 com vantagem',
        logText: 'Rolagem livre · Brisa: 1d20 + 5 com vantagem = 19 (14, 9 descartado)',
        createdAt: timestampFromDate(at),
      }),
    );
    expect(vm.mode).toBe('advantage');
    expect(vm.dice).toEqual([
      { face: 14, discarded: false },
      { face: 9, discarded: true },
    ]);
    expect(vm.createdAt).toEqual(at);
    expect(vm.total).toBe(19);
  });

  it('reads the disadvantage and the plain roll', () => {
    expect(toFreeRollVm(create(FreeRollSchema, { mode: FreeRollMode.DISADVANTAGE })).mode).toBe(
      'disadvantage',
    );
    expect(toFreeRollVm(create(FreeRollSchema, {})).mode).toBe('normal');
  });
});

describe('the form', () => {
  it('starts at 1d20, no modifier, for the table', () => {
    expect(DEFAULT_FORM).toEqual({
      sides: 20,
      count: 1,
      modifier: 0,
      mode: 'normal',
      secret: false,
    });
  });

  it('keeps the count in 1 to 20 and the modifier in -20 to +20', () => {
    expect(normalizeForm(form({ count: 0 })).count).toBe(1);
    expect(normalizeForm(form({ count: 99 })).count).toBe(20);
    expect(normalizeForm(form({ modifier: -50 })).modifier).toBe(-20);
    expect(normalizeForm(form({ modifier: 50 })).modifier).toBe(20);
    expect(normalizeForm(form({ count: 2.7, modifier: 1.2 }))).toMatchObject({
      count: 2,
      modifier: 1,
    });
  });

  it('keeps a die the table has, and falls back to d20 for any other', () => {
    expect(normalizeForm(form({ sides: 12 })).sides).toBe(12);
    expect(normalizeForm(form({ sides: 7 })).sides).toBe(20);
  });

  it('lets advantage stand only for one d20', () => {
    expect(canAdvantage(form({ sides: 20, count: 1 }))).toBe(true);
    expect(canAdvantage(form({ sides: 20, count: 2 }))).toBe(false);
    expect(canAdvantage(form({ sides: 6, count: 1 }))).toBe(false);
    expect(normalizeForm(form({ sides: 20, count: 2, mode: 'advantage' })).mode).toBe('normal');
    expect(normalizeForm(form({ sides: 8, mode: 'disadvantage' })).mode).toBe('normal');
    expect(normalizeForm(form({ mode: 'advantage' })).mode).toBe('advantage');
  });

  it('writes the formula the way the server does', () => {
    expect(formulaOf(form({ sides: 6, count: 2, modifier: 3 }))).toBe('2d6 + 3');
    expect(formulaOf(form({ sides: 20, modifier: -2 }))).toBe('1d20 - 2');
    expect(formulaOf(form({ sides: 100 }))).toBe('1d100');
    expect(formulaOf(form({ modifier: 5, mode: 'advantage' }))).toBe('1d20 + 5 com vantagem');
    expect(formulaOf(form({ mode: 'disadvantage' }))).toBe('1d20 com desvantagem');
  });

  it('is rebuilt from a roll to repeat it, secret included', () => {
    const again = formFromRoll(
      roll({ sides: 20, count: 1, modifier: 5, mode: 'advantage', secret: true }),
    );
    expect(again).toEqual({ sides: 20, count: 1, modifier: 5, mode: 'advantage', secret: true });
  });
});

describe('the big button', () => {
  it('says the formula, and for whom when the master rolls', () => {
    expect(
      rollButtonLabel(form({ sides: 6, count: 2, modifier: 3 }), {
        master: false,
        physical: false,
      }),
    ).toBe('Rolar 2d6 + 3');
    expect(
      rollButtonLabel(form({ sides: 6, count: 2, modifier: 3 }), { master: true, physical: false }),
    ).toBe('Rolar 2d6 + 3 para a mesa');
    expect(
      rollButtonLabel(form({ mode: 'advantage', secret: true }), { master: true, physical: false }),
    ).toBe('Rolar 1d20 com vantagem, só eu');
    expect(rollButtonLabel(form({ secret: true }), { master: true, physical: false })).toBe(
      'Rolar 1d20, só eu',
    );
  });

  it('says "no app" at a table of physical dice', () => {
    expect(rollButtonLabel(form(), { master: false, physical: true })).toBe('Rolar no app');
    expect(rollButtonName(form(), { master: false, physical: true })).toBe('Rolar no app: 1d20');
  });

  it('says "de novo" on the result card\'s button, before who sees it', () => {
    const player = { master: false, physical: false };
    const master = { master: true, physical: false };
    expect(rollButtonLabel(form({ sides: 6, count: 2, modifier: 3 }), player, true)).toBe(
      'Rolar 2d6 + 3 de novo',
    );
    expect(rollButtonLabel(form({ secret: true }), master, true)).toBe('Rolar 1d20 de novo, só eu');
    expect(rollButtonLabel(form(), master, true)).toBe('Rolar 1d20 de novo, para a mesa');
  });

  it('names the repeat and the roll again', () => {
    expect(repeatLabel(roll())).toBe('Repetir 2d6 + 3');
    expect(repeatLabel(roll({ formula: '1d20 + 5 com vantagem' }))).toBe(
      'Repetir 1d20 + 5 com vantagem',
    );
  });
});

describe('the result', () => {
  it('says the sum and the dice', () => {
    expect(resultLine(roll())).toBe('2d6 + 3 = 11');
    expect(summaryOf(roll())).toBe('2d6 + 3 = 11 (4, 4)');
  });

  it('marks the discarded d20 in words', () => {
    const adv = roll({
      sides: 20,
      count: 1,
      modifier: 5,
      mode: 'advantage',
      formula: '1d20 + 5 com vantagem',
      dice: [
        { face: 14, discarded: false },
        { face: 9, discarded: true },
      ],
      total: 19,
    });
    expect(summaryOf(adv)).toBe('1d20 + 5 com vantagem = 19 (14, 9 descartado)');
  });

  it('says natural 20 and natural 1 in words, only for one d20', () => {
    const one = (face: number, over: Partial<FreeRollVm> = {}) =>
      roll({
        sides: 20,
        count: 1,
        modifier: 5,
        formula: '1d20 + 5',
        dice: [{ face, discarded: false }],
        total: face + 5,
        ...over,
      });
    expect(naturalWord(one(20))).toBe('20 natural');
    expect(naturalWord(one(1))).toBe('1 natural');
    expect(naturalWord(one(12))).toBeNull();
    expect(resultLine(one(20))).toBe('1d20 + 5 = 25 · 20 natural');
    expect(resultLine(one(1))).toBe('1d20 + 5 = 6 · 1 natural');
    // Two d20 are a sum, not a natural.
    expect(
      naturalWord(
        roll({
          sides: 20,
          count: 2,
          dice: [
            { face: 20, discarded: false },
            { face: 3, discarded: false },
          ],
        }),
      ),
    ).toBeNull();
    // A d6 that shows 1 is no natural 1.
    expect(
      naturalWord(roll({ sides: 6, count: 1, dice: [{ face: 1, discarded: false }] })),
    ).toBeNull();
  });

  it('counts the natural of the die that stayed under advantage', () => {
    const adv = roll({
      sides: 20,
      count: 1,
      mode: 'advantage',
      dice: [
        { face: 3, discarded: true },
        { face: 20, discarded: false },
      ],
    });
    expect(naturalWord(adv)).toBe('20 natural');
  });

  it('says who sees it', () => {
    expect(audienceText(roll())).toBe('para a mesa');
    expect(audienceText(roll({ secret: true }))).toBe('só você');
  });
});

describe('the lists', () => {
  it('splits the log line at the name so the app can set the who in bold', () => {
    expect(logParts(roll())).toEqual({
      who: 'Rolagem livre · Brisa:',
      rest: '2d6 + 3 = 11 (4, 4)',
    });
    expect(logParts(roll({ logText: 'sem dois pontos' }))).toEqual({
      who: '',
      rest: 'sem dois pontos',
    });
  });

  it('says how long ago', () => {
    const at = new Date(2026, 9, 18, 20, 10, 0);
    const later = (s: number) => new Date(at.getTime() + s * 1000);
    expect(whenText(at, later(0))).toBe('agora');
    expect(whenText(at, later(59))).toBe('agora');
    expect(whenText(at, later(60))).toBe('há 1 min');
    expect(whenText(at, later(179))).toBe('há 2 min');
    expect(whenText(at, later(3600))).toBe('há 1 h');
    // A clock a little behind never reads in the future.
    expect(whenText(at, later(-30))).toBe('agora');
  });
});
