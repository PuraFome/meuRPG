import { create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  CombatLogEntrySchema,
  CombatLogKind,
  PendingDamageStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { undoLabel } from './combat-log';

const entry = (over: object) => create(CombatLogEntrySchema, { id: 'x', ...over } as never);

describe('the undo label, kind by kind', () => {
  it('leaves the damage out of an attack that waits for the roll, and says a miss missed', () => {
    const waiting = entry({
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Brisa',
      targetLabel: 'Goblin 1',
      outcome: AttackOutcome.HIT,
      damage: { status: PendingDamageStatus.AWAITING_ROLL, amount: 0 },
    });
    expect(undoLabel(waiting)).toBe('o ataque da Brisa ao Goblin 1');
    const missed = entry({
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Toren',
      targetLabel: '',
      outcome: AttackOutcome.MISS,
      damage: { status: PendingDamageStatus.APPLIED, amount: 4 },
    });
    expect(undoLabel(missed)).toBe('o ataque do Toren ao alguém (errou)');
  });

  it('names an action, a hit point adjustment, a reaction, an offer and a change of conditions', () => {
    const label = (over: object) => undoLabel(entry(over));
    expect(label({ kind: CombatLogKind.ACTION, actorLabel: 'Brisa', keyNamePt: 'Esquivar' })).toBe(
      'a ação de Brisa (Esquivar)',
    );
    expect(label({ kind: CombatLogKind.ACTION, actorLabel: 'Brisa' })).toBe('a ação de Brisa');
    expect(
      label({ kind: CombatLogKind.HIT_POINTS_ADJUSTED, actorLabel: '', targetLabel: 'Goblin 2' }),
    ).toBe('o ajuste de PV de Goblin 2');
    expect(label({ kind: CombatLogKind.REACTION, actorLabel: 'Brisa' })).toBe('a reação da Brisa');
    expect(label({ kind: CombatLogKind.OPPORTUNITY_OFFERED, targetLabel: 'Toren' })).toBe(
      'a oferta de ataque de oportunidade a Toren',
    );
    expect(label({ kind: CombatLogKind.OPPORTUNITY_OFFERED })).toBe(
      'a oferta de ataque de oportunidade a alguém',
    );
    expect(label({ kind: CombatLogKind.CONDITIONS_CHANGED, targetLabel: 'Toren' })).toBe(
      'a mudança de condições do Toren',
    );
    expect(label({ kind: CombatLogKind.SPELL_CAST, actorLabel: 'Brisa' })).toBe('a magia da Brisa');
  });

  it('falls back to "a última ação" for any other kind', () => {
    expect(undoLabel(entry({ kind: CombatLogKind.MOVED, actorLabel: 'Brisa' }))).toBe(
      'a última ação',
    );
  });
});
