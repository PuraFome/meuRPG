import { create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  CombatantState,
  PendingDamageSchema,
  PendingDamageStatus,
  TargetInReachSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { outcomeWord, stageAfterRoll, steps, targetAfter, targetRows } from './attack-flow';
import { CoverDegree, CoverSource } from '../../../gen/meurpg/play/v1/combat_pb';

const awaiting = create(PendingDamageSchema, { status: PendingDamageStatus.AWAITING_ROLL });

describe('the attack sheet steps', () => {
  it('marks the done steps, the current one and the rest', () => {
    expect(steps('target').map((s) => s.state)).toEqual(['current', 'todo', 'todo']);
    expect(steps('roll').map((s) => s.state)).toEqual(['done', 'current', 'todo']);
    expect(steps('damage').map((s) => s.state)).toEqual(['done', 'done', 'current']);
    expect(steps('done').map((s) => s.state)).toEqual(['done', 'done', 'done']);
  });

  it('goes on to the damage after a hit, and finishes after a miss', () => {
    expect(stageAfterRoll(AttackOutcome.HIT, awaiting)).toBe('damage');
    expect(stageAfterRoll(AttackOutcome.CRITICAL_HIT, awaiting)).toBe('damage');
    expect(stageAfterRoll(AttackOutcome.MISS, undefined)).toBe('done');
    expect(stageAfterRoll(AttackOutcome.HIT, undefined)).toBe('done');
  });

  it('names the outcome', () => {
    expect([AttackOutcome.HIT, AttackOutcome.CRITICAL_HIT, AttackOutcome.MISS].map(outcomeWord)).toEqual([
      'Acertou',
      'Crítico',
      'Errou',
    ]);
  });
});

describe('the targets', () => {
  it('writes the state word and the distance, and blocks one that is too far', () => {
    const rows = targetRows(
      [
        create(TargetInReachSchema, { combatantId: 'c', label: 'Capitão Goblin', state: CombatantState.HURT, distanceFt: 25 }),
        create(TargetInReachSchema, { combatantId: 'g', label: 'Goblin 2', state: CombatantState.UNHURT, distanceFt: 40, tooFar: true }),
      ],
      120,
    );
    const plain = (t: string) => t.replace(/\u00a0/g, ' ');
    expect(rows.map((r) => [plain(r.sub), plain(r.blocked)])).toEqual([
      ['Ferido · a 7,5 m', ''],
      ['Ileso · a 12 m', 'Longe demais: alcance de 36 m'],
    ]);
    expect(rows[1].blocked).toContain('alcance de\u00a036\u00a0m');
  });

  it('names the cover of each target with its source, disables a mark of total cover and leaves out a wall (E9-07)', () => {
    const rows = targetRows(
      [
        create(TargetInReachSchema, { combatantId: 'g2', label: 'Goblin 2', distanceFt: 25, cover: CoverDegree.HALF, coverSource: CoverSource.MAP }),
        create(TargetInReachSchema, { combatantId: 'c', label: 'Capitão Goblin', distanceFt: 35, cover: CoverDegree.THREE_QUARTERS, coverSource: CoverSource.MAP }),
        create(TargetInReachSchema, { combatantId: 'g1', label: 'Goblin 1', distanceFt: 30, cover: CoverDegree.TOTAL, coverSource: CoverSource.MAP, untargetable: true }),
        create(TargetInReachSchema, { combatantId: 'g3', label: 'Goblin 3', distanceFt: 20, cover: CoverDegree.TOTAL, coverSource: CoverSource.MARK, untargetable: true }),
        create(TargetInReachSchema, { combatantId: 'g4', label: 'Goblin 4', distanceFt: 20 }),
      ],
      120,
    );
    expect(rows.map((r) => r.id)).toEqual(['g2', 'c', 'g3', 'g4']);
    expect(rows.map((r) => [r.cover, r.coverMark, r.blocked])).toEqual([
      ['Meia cobertura (do mapa)', 'half', ''],
      ['Três quartos (do mapa)', 'three', ''],
      ['', null, 'Cobertura total (marcada pelo mestre): não pode ser alvo'],
      ['', null, ''],
    ]);
  });

  it('says what happened to the target', () => {
    const defeated = create(PendingDamageSchema, { status: PendingDamageStatus.APPLIED, targetDefeated: true });
    expect(targetAfter('Goblin 2', false, defeated, 'Derrotado')).toBe('Goblin 2 derrotado');
    const rolled = create(PendingDamageSchema, { status: PendingDamageStatus.ROLLED });
    expect(targetAfter('Toren', true, rolled, '')).toBe('Esperando o mestre aplicar o dano');
    const applied = create(PendingDamageSchema, { status: PendingDamageStatus.APPLIED });
    expect(targetAfter('Capitão Goblin', false, applied, 'Ferido')).toBe('Capitão Goblin: ferido');
  });
});

describe('the damage that waits for the master', () => {
  it('counts the temporary hit points first, and never goes below 0', async () => {
    const { hitPointsAfter, hitPointsLine } = await import('./attack-flow');
    expect(hitPointsAfter(26, 0, 5)).toBe(21);
    expect(hitPointsAfter(26, 3, 5)).toBe(24);
    expect(hitPointsAfter(4, 0, 9)).toBe(0);
    expect(hitPointsLine('Toren', 26, 31, 21)).toBe('Toren: 26 de 31 PV, depois 21');
  });

  it('says what is owed: to apply first, then to roll, else nothing', async () => {
    const { openDamages, pendingNote } = await import('./attack-flow');
    const rolled = create(PendingDamageSchema, { status: PendingDamageStatus.ROLLED, amount: 5 });
    const waiting = create(PendingDamageSchema, { status: PendingDamageStatus.AWAITING_ROLL });
    const applied = create(PendingDamageSchema, { status: PendingDamageStatus.APPLIED, amount: 2 });
    expect(pendingNote([waiting, rolled])).toBe('Falta aplicar 5 de dano');
    expect(pendingNote([waiting])).toBe('Falta rolar o dano');
    expect(pendingNote([applied])).toBeNull();
    expect(openDamages([waiting, rolled, applied])).toHaveLength(2);
  });
});
