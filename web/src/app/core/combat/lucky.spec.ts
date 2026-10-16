import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  CombatLogEntrySchema,
  CombatLogKind,
  DiceRollSchema,
  LuckyOfferSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { logLine } from './combat-log';
import { LUCKY_TRAIT, luckyText, luckyWhat } from './lucky';

const d20 = (faces: number[], rerolled: MessageInitShape<typeof DiceRollSchema>['rerolled'] = []) =>
  create(DiceRollSchema, { diceCount: faces.length, diceSides: 20, faces, modifier: 5, rerolled });

describe('the words of the halfling’s Lucky (Sortudo)', () => {
  it('says the d20 is rolled again, and with two dice names the one that showed the 1', () => {
    expect(luckyWhat(create(LuckyOfferSchema, { d20: d20([1]) }))).toBe(
      'Sortudo: você pode rolar esse d20 de novo e fica com o novo resultado.',
    );
    expect(luckyWhat(create(LuckyOfferSchema, { d20: d20([1, 13]) }))).toBe(
      'Sortudo: você pode rolar de novo o dado que deu 1 e fica com o novo resultado.',
    );
  });

  it('writes "Sortudo: 1 → 14" for the d20 Lucky rolled again, and nothing for Great Weapon Fighting or a kept 1', () => {
    expect(luckyText(d20([14], [{ index: 0, from: 1, to: 14, sourceKey: LUCKY_TRAIT }]))).toBe(
      'Sortudo: 1 → 14',
    );
    expect(luckyText(d20([4], [{ index: 0, from: 1, to: 4, sourceKey: '' }]))).toBe('');
    expect(luckyText(d20([1]))).toBe('');
    expect(luckyText(undefined)).toBe('');
  });

  it('puts the reroll among the notes of the attack line, where a reload still finds it', () => {
    const line = logLine(
      create(CombatLogEntrySchema, {
        id: 'x',
        kind: CombatLogKind.ATTACK,
        actorLabel: 'Pipo',
        targetLabel: 'Goblin',
        keyNamePt: 'Machado de batalha',
        key: 'equipment:battleaxe',
        outcome: AttackOutcome.HIT,
        attackRoll: d20([15], [{ index: 0, from: 1, to: 15, sourceKey: LUCKY_TRAIT }]),
      }),
    );
    expect(line?.notes).toContain('Sortudo: 1 → 15');
  });
});
