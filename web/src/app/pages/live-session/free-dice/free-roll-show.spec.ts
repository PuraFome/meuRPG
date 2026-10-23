import { freeRoll } from '../../../core/free-dice/free-dice-testing';
import { freeRollShow } from './free-roll-show';

describe('freeRollShow', () => {
  it('labels the roller, shows the dice and the total, and says it is a free roll', () => {
    expect(freeRollShow(freeRoll('r1'))).toEqual({
      label: 'Brisa rolou',
      dice: [
        { sides: 6, face: 4, counts: true },
        { sides: 6, face: 4, counts: true },
      ],
      line: '11',
      note: 'Rolagem livre · 2d6 + 3',
      outcome: undefined,
    });
  });

  it('dims the discarded d20 and says the natural 20 in words', () => {
    const show = freeRollShow(
      freeRoll('r2', {
        sides: 20,
        count: 1,
        mode: 'advantage',
        formula: '1d20 + 5 com vantagem',
        dice: [
          { face: 20, discarded: false },
          { face: 9, discarded: true },
        ],
        total: 25,
      }),
    );
    expect(show?.dice.map((d) => d.counts)).toEqual([true, false]);
    expect(show?.outcome).toEqual({ word: '20 natural', good: true });
    expect(show?.note).toBe('Rolagem livre · 1d20 + 5 com vantagem');
  });

  it('says the natural 1 in words, in the bad colour', () => {
    const show = freeRollShow(
      freeRoll('r3', { sides: 20, count: 1, dice: [{ face: 1, discarded: false }], total: 6 }),
    );
    expect(show?.outcome).toEqual({ word: '1 natural', good: false });
  });

  it('draws nothing for a roll with no dice', () => {
    expect(freeRollShow(freeRoll('r4', { dice: [] }))).toBeNull();
  });
});
