import {
  activeParts,
  defaultSlot,
  initialPicks,
  partFormula,
  partTotal,
  rerollText,
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
