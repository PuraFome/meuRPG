import { DamageStepKind } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { AttackOutcome, PendingDamageStatus } from '../../../gen/meurpg/play/v1/combat_pb';
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

describe('a power attack in the log, as each viewer reads it', () => {
  const hit = (powerAttack: object | undefined, damage: object) =>
    ({
      targetLabel: 'Zumbi 1',
      key: 'equipment:greatsword',
      keyNamePt: 'Montante',
      outcome: AttackOutcome.HIT,
      bonusDice: [],
      powerAttack,
      damage,
    }) as never;
  const applied = {
    status: PendingDamageStatus.APPLIED,
    amount: 21,
    parts: [],
    steps: [],
    rolled: [],
  };

  it('tells the attacker and the master the numbers, then the damage', () => {
    expect(
      attackText(
        hit(
          { key: 'feat:golpe@mesa', namePt: 'Golpe Pesado', penalty: 5, damageBonus: 10 },
          applied,
        ),
      ),
    ).toBe(
      ' ataca o Zumbi 1 com o Montante: acertou, com ataque poderoso (−5 no ataque, +10 no dano), 21 de dano',
    );
  });

  it('tells another player only that a trade was made, then the damage as rolled', () => {
    expect(
      attackText(hit({ key: '', namePt: 'Golpe Pesado', penalty: 0, damageBonus: 0 }, applied)),
    ).toBe(' ataca o Zumbi 1 com o Montante: acertou, usou um ataque poderoso, 21 de dano');
  });

  const part = (over: object) =>
    ({
      partKey: 'power_attack',
      labelPt: 'Golpe Pesado',
      diceCount: 0,
      diceSides: 0,
      faces: [],
      flat: 0,
      sum: 0,
      counted: true,
      damageTypeKey: 'damage-type:slashing',
      rerolled: [],
      countedDice: [],
      ...over,
    }) as never;

  it('names the part of another player with its damage type and no number, and "Outra fonte" when it is hidden', () => {
    const notes = (label: string) =>
      damageNotes({ parts: [part({ labelPt: label })], steps: [] } as never);
    expect(notes('Golpe Pesado')).toEqual(['Golpe Pesado cortante']);
    expect(notes('Outra fonte')).toEqual(['Outra fonte cortante']);
  });

  it('gives the attacker and the master the flat number of the part', () => {
    expect(damageNotes({ parts: [part({ flat: 10 })], steps: [] } as never)).toEqual([
      'Golpe Pesado + 10 = 10',
    ]);
  });
});
