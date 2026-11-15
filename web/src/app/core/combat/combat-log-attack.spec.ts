import { DamageStepKind } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { AttackOutcome } from '../../../gen/meurpg/play/v1/combat_pb';
import { attackText, damageNotes } from './combat-log-attack';

describe('damageNotes for a damage reduction step', () => {
  const damage = (labelPt: string, ignored = false) =>
    ({
      parts: [],
      rolled: [],
      steps: [{ kind: DamageStepKind.REDUCTION, labelPt, before: 8, after: 5, ignored }],
    }) as never;

  it('writes the step with the label the server sent', () => {
    expect(damageNotes(damage('Redução de dano de arma (Couro grosso)'))).toContain(
      'Redução de dano de arma (Couro grosso): 8 → 5',
    );
  });

  it('names the step by its kind when the label is empty, and says the master left it out', () => {
    expect(damageNotes(damage('', true))).toContain(
      'redução de dano: 8 → 5 (o mestre deixou de fora)',
    );
  });
});

describe('damageNotes for a typed damage-type group', () => {
  const part = (over: object) => ({
    partKey: 'p',
    labelPt: 'Adaga',
    diceCount: 1,
    diceSides: 4,
    faces: [],
    flat: 0,
    sum: 0,
    counted: true,
    damageTypeKey: 'piercing',
    countedDice: [],
    ...over,
  });
  const damage = (parts: object[]) => ({ parts, rolled: [], steps: [] }) as never;

  it('does not say "= 0" for the part of the group that carries no sum of its own', () => {
    const notes = damageNotes(
      damage([
        part({ partKey: 'a', flat: 3, sum: 6 }),
        part({ partKey: 'b', labelPt: 'Gume Afiado', diceSides: 8 }),
      ]),
    );
    expect(notes).toEqual(['Adaga 1d4 + 3 = 9', 'Gume Afiado 1d8 (somado em Adaga)']);
  });

  it('keeps "= 0" for a part alone in its type, and for another type', () => {
    const notes = damageNotes(
      damage([
        part({ partKey: 'a', sum: 6 }),
        part({ partKey: 'b', labelPt: 'Brasa', damageTypeKey: 'fire' }),
      ]),
    );
    expect(notes).toEqual(['Adaga 1d4 = 6', 'Brasa 1d4 = 0']);
  });

  it('keeps the flat number of the part, which counts by itself', () => {
    const notes = damageNotes(
      damage([part({ partKey: 'a', sum: 6 }), part({ partKey: 'b', labelPt: 'Gume', flat: 2 })]),
    );
    expect(notes[1]).toBe('Gume 1d4 + 2 = 2');
  });
});

describe('attackText with a power attack', () => {
  const entry = (powerAttack: object | undefined) =>
    ({
      targetLabel: 'Goblin',
      key: 'equipment:greatsword',
      keyNamePt: 'Montante',
      outcome: AttackOutcome.MISS,
      bonusDice: [],
      powerAttack,
    }) as never;

  it('tells the attacker and the master the trade with the numbers and a true minus', () => {
    expect(
      attackText(entry({ key: 'feat:golpe@mesa', namePt: 'Golpe', penalty: 5, damageBonus: 10 })),
    ).toContain(': errou, com ataque poderoso (−5 no ataque, +10 no dano)');
  });

  it('tells another player that a trade was made, without the numbers', () => {
    const text = attackText(entry({ key: '', namePt: 'Golpe', penalty: 0, damageBonus: 0 }));
    expect(text).toContain(': errou, usou um ataque poderoso');
    expect(text).not.toMatch(/\d/);
  });

  it('tells another player the same when the master hid the feature', () => {
    const text = attackText(
      entry({ key: '', namePt: 'Outra fonte', penalty: 0, damageBonus: 0, hidden: true }),
    );
    expect(text).toContain(': errou, usou um ataque poderoso');
    expect(text).not.toContain('Outra fonte');
  });

  it('adds nothing to an attack that made no trade', () => {
    expect(attackText(entry(undefined))).toMatch(/errou$/);
  });
});
