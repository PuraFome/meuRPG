import {
  activeParts,
  defaultSlot,
  initialPicks,
  isOffPart,
  partFormula,
  partTotal,
  rerollText,
  rollFormula,
  typedFields,
} from './damage-parts';

const weapon = {
  key: 'weapon',
  labelPt: 'Espada curta',
  diceCount: 1,
  diceSides: 6,
  flat: 3,
} as never;
const rage = {
  key: 'rage',
  labelPt: 'Fúria',
  diceCount: 0,
  diceSides: 0,
  flat: 2,
  auto: true,
} as never;
const sneak = {
  key: 'sneak-attack',
  labelPt: 'Ataque Furtivo',
  diceCount: 2,
  diceSides: 6,
  flat: 0,
  choosable: true,
  selected: true,
  available: true,
} as never;
const smite = {
  key: 'divine-smite',
  labelPt: 'Golpe Divino',
  diceCount: 2,
  diceSides: 8,
  flat: 0,
  choosable: true,
  selected: false,
  available: true,
  needsSlot: true,
  slotOptions: [
    { level: 2, pact: false, free: 0, max: 2, diceCount: 3 },
    { level: 3, pact: false, free: 1, max: 2, diceCount: 4 },
    { level: 1, pact: false, free: 2, max: 4, diceCount: 2 },
  ],
} as never;

describe('damage parts', () => {
  it('writes the dice and the fixed number of a part', () => {
    expect(partFormula(weapon)).toBe('1d6 + 3');
    expect(partFormula(rage)).toBe('2');
    expect(partFormula(sneak)).toBe('2d6');
  });

  it('starts with the extras that come marked, and the lowest free slot for Golpe Divino', () => {
    expect(initialPicks([weapon, sneak, smite])).toEqual([
      { key: 'sneak-attack', slotLevel: 0, pact: false },
    ]);
    expect(defaultSlot(smite)?.level).toBe(1);
  });

  it('rolls the slot dice of Golpe Divino once it is marked', () => {
    const pick = { key: 'divine-smite', slotLevel: 3, pact: false };
    expect(partFormula(smite, pick)).toBe('4d8');
  });

  it('counts the weapon, the automatic lines and the marked extras', () => {
    const picks = [{ key: 'sneak-attack', slotLevel: 0, pact: false }];
    expect(activeParts([weapon, rage, sneak, smite], picks)).toEqual([weapon, rage, sneak]);
  });

  it('gives one number field for each marked part that rolls dice, the weapon included', () => {
    const picks = [{ key: 'sneak-attack', slotLevel: 0, pact: false }];
    expect(typedFields([weapon, rage, sneak, smite], picks)).toEqual([
      { key: 'weapon', label: 'Espada curta: 1d6', min: 1, max: 6 },
      { key: 'sneak-attack', label: 'Ataque Furtivo: 2d6', min: 2, max: 12 },
    ]);
  });

  it('says what a part made and what Great Weapon Fighting rolled again', () => {
    const roll = {
      sum: 9,
      flat: 3,
      rerolled: [
        { index: 0, from: 1, to: 4 },
        { index: 1, from: 2, to: 5 },
      ],
    } as never;
    expect(partTotal(roll)).toBe(12);
    expect(rerollText(roll)).toBe('rolou de novo 1→4, rolou de novo 2→5');
  });
});

describe('the off part of a spell hit effect', () => {
  const off = {
    key: 'frostfist',
    labelPt: 'Geada no Punho',
    diceCount: 2,
    diceSides: 8,
    flat: 0,
    available: false,
    reasonPt: 'só num golpe corpo a corpo. Continua esperando.',
  } as never;

  it('is no part of the damage: not active, not typed, not in the formula', () => {
    expect(isOffPart(off)).toBe(true);
    expect(isOffPart(weapon)).toBe(false);
    expect(isOffPart(sneak)).toBe(false);
    expect(activeParts([weapon, off], [])).toEqual([weapon]);
    expect(typedFields([weapon, off], []).map((f) => f.key)).toEqual(['weapon']);
    expect(rollFormula([weapon, off], [])).toBe('1d6 + 3');
  });
});

describe('rollFormula', () => {
  const part = (diceCount: number, diceSides: number, flat: number, extra = {}) =>
    ({
      key: `p${diceCount}${diceSides}${flat}`,
      labelPt: 'x',
      diceCount,
      diceSides,
      flat,
      ...extra,
    }) as never;

  it.each([
    ['the weapon alone', [part(1, 6, 3)], '1d6 + 3'],
    [
      'the weapon and a hit effect',
      [part(1, 8, 3), part(2, 6, 0, { auto: true })],
      '1d8 + 2d6 + 3',
    ],
    [
      'a critical: the dice doubled, the fixed number not',
      [part(2, 8, 3), part(4, 6, 0)],
      '2d8 + 4d6 + 3',
    ],
    ['a negative modifier', [part(1, 8, -1)], '1d8 − 1'],
    [
      'the fixed numbers of every part added',
      [part(1, 8, 3), part(0, 0, 2), part(2, 6, -1)],
      '1d8 + 2d6 + 4',
    ],
    ['a fixed sum that cancels', [part(1, 8, 2), part(1, 4, -2)], '1d8 + 1d4'],
    ['no dice at all', [part(0, 0, 2)], '2'],
    ['nothing', [], '0'],
  ])('%s', (_name, parts, text) => {
    expect(rollFormula(parts as never[], [])).toBe(text);
  });

  it('counts a marked extra and leaves out one that is not marked', () => {
    const extra = part(1, 6, 0, { key: 'sneak', choosable: true, available: true });
    expect(rollFormula([part(1, 8, 3), extra], [])).toBe('1d8 + 3');
    expect(rollFormula([part(1, 8, 3), extra], [{ key: 'sneak', slotLevel: 0, pact: false }])).toBe(
      '1d8 + 1d6 + 3',
    );
  });
});
