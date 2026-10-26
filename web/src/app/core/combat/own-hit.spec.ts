import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import {
  CombatLogDamageSchema,
  CombatLogEntrySchema,
  CombatLogKind,
  CombatLogRoundSchema,
  PendingDamageStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { ownHitOf } from './own-hit';

function hit(
  id: string,
  at: number,
  over: Record<string, unknown> = {},
  damage: Record<string, unknown> = {},
) {
  return create(CombatLogEntrySchema, {
    id,
    kind: CombatLogKind.ATTACK,
    at: timestampFromDate(new Date(at)),
    actorLabel: 'Espada voadora 2',
    targetId: 'me',
    damage: create(CombatLogDamageSchema, {
      status: PendingDamageStatus.APPLIED,
      amount: 9,
      damageTypePt: 'Cortante',
      ...damage,
    }),
    ...over,
  });
}

const rounds = (...entries: ReturnType<typeof hit>[]) => [
  create(CombatLogRoundSchema, { round: 1, entries }),
];

describe('ownHitOf', () => {
  it("says the attack's damage, with who did it", () => {
    expect(ownHitOf(rounds(hit('a', 1000)), 'me')).toEqual({
      id: 'a',
      text: 'A Espada voadora 2 acertou você: 9 de dano cortante.',
    });
  });

  it('writes the other types with "de"', () => {
    const r = rounds(hit('a', 1000, { actorLabel: 'Armadura animada' }, { damageTypePt: 'Fogo' }));
    expect(ownHitOf(r, 'me')?.text).toBe('A Armadura animada acertou você: 9 de dano de fogo.');
  });

  it('takes the newest attack on the character, whatever the order of the entries', () => {
    const r = rounds(
      hit('old', 1000),
      hit('new', 5000, { actorLabel: 'Goblin 1' }),
      hit('other', 9000, { targetId: 'someone' }),
    );
    expect(ownHitOf(r, 'me')?.id).toBe('new');
  });

  it('leaves out what is not damage the attack did: a miss, a pending roll, a heal, the master adjusting', () => {
    const r = rounds(
      hit('pending', 1000, {}, { status: PendingDamageStatus.AWAITING_ROLL, amount: 0 }),
      hit('heal', 2000, {}, { healing: true }),
      hit('adjust', 3000, { kind: CombatLogKind.HIT_POINTS_ADJUSTED }),
      hit('miss', 4000, { damage: undefined }),
    );
    expect(ownHitOf(r, 'me')).toBeNull();
    expect(ownHitOf(rounds(hit('a', 1000)), '')).toBeNull();
  });
});
