import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
  CombatLogRoundSchema,
  PendingDamageStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { entryCount, latestLine, logGroups, logLine, undoLabel, undoableEntry } from './combat-log';

type Over = Omit<MessageInitShape<typeof CombatLogEntrySchema>, 'damage' | '$typeName'> & { damage?: { status: PendingDamageStatus; amount: number; defeated?: boolean } };

function entry(over: Over): CombatLogEntry {
  const { damage, ...rest } = over;
  return create(CombatLogEntrySchema, {
    id: Math.random().toString(36).slice(2),
    ...rest,
    damage: damage && { status: damage.status, amount: damage.amount, targetDefeated: damage.defeated ?? false },
  });
}

const attack = (over: Over) => entry({ kind: CombatLogKind.ATTACK, outcome: AttackOutcome.HIT, ...over });

describe('the combat log sentences (timeline.md, Rodadas 1 and 2)', () => {
  it('writes an attack with its damage, and a defeat', () => {
    expect(
      logLine(
        attack({
          actorLabel: 'Toren', targetLabel: 'Goblin 1', key: 'equipment:battleaxe', keyNamePt: 'Machado de batalha',
          damage: { status: PendingDamageStatus.APPLIED, amount: 9, defeated: true },
        }),
      )?.text,
    ).toBe(' ataca o Goblin 1 com o Machado de batalha: acertou, 9 de dano. Goblin 1 derrotado');
  });

  it('says "atira" for a shot, and the article of each name', () => {
    const shot = attack({
      actorLabel: 'Capitão Goblin', targetLabel: 'Toren', key: 'equipment:shortbow', keyNamePt: 'Arco curto',
      damage: { status: PendingDamageStatus.APPLIED, amount: 5 },
    });
    expect(logLine(shot)?.text).toBe(' atira no Toren com o Arco curto: acertou, 5 de dano');
    const onBrisa = attack({
      actorLabel: 'Goblin 2', targetLabel: 'Brisa', key: 'equipment:shortbow', keyNamePt: 'Arco curto',
      damage: { status: PendingDamageStatus.APPLIED, amount: 5 },
    });
    expect(logLine(onBrisa)?.text).toBe(' atira na Brisa com o Arco curto: acertou, 5 de dano');
    const rapier = attack({
      actorLabel: 'Brisa', targetLabel: 'Capitão Goblin', key: 'equipment:rapier', keyNamePt: 'Rapieira',
      damage: { status: PendingDamageStatus.APPLIED, amount: 8 },
    });
    expect(logLine(rapier)?.text).toBe(' ataca o Capitão Goblin com a Rapieira: acertou, 8 de dano');
  });

  it('draws a spell attack with the spell icon, a weapon attack with the swords', () => {
    expect(logLine(attack({ actorLabel: 'Pensantus', targetLabel: 'Goblin 2', key: 'spell:fire-bolt', keyNamePt: 'Raio de Fogo' }))?.icon).toBe('auto_awesome');
    expect(logLine(attack({ actorLabel: 'Toren', targetLabel: 'Goblin 1', key: 'equipment:battleaxe' }))?.icon).toBe('swords');
  });

  it('says a miss, a critical, a pending and a discarded damage', () => {
    expect(logLine(attack({ actorLabel: 'Brisa', targetLabel: 'Capitão Goblin', keyNamePt: 'Rapieira', outcome: AttackOutcome.MISS }))?.text).toBe(
      ' ataca o Capitão Goblin com a Rapieira: errou',
    );
    expect(logLine(attack({ actorLabel: 'Goblin 3', targetLabel: 'Brisa', outcome: AttackOutcome.CRITICAL_HIT, damage: { status: PendingDamageStatus.ROLLED, amount: 11 } }))?.text).toBe(
      ' ataca a Brisa: crítico, 11 de dano, esperando o mestre aplicar',
    );
    expect(logLine(attack({ actorLabel: 'Goblin 3', targetLabel: 'Brisa', damage: { status: PendingDamageStatus.AWAITING_ROLL, amount: 0 } }))?.text).toContain('falta rolar o dano');
    expect(logLine(attack({ actorLabel: 'Goblin 3', targetLabel: 'Brisa', damage: { status: PendingDamageStatus.DISCARDED, amount: 5 } }))?.text).toContain('5 de dano descartado');
  });

  it('writes moves, standard actions, hit point changes, reveals, start and end', () => {
    expect(logLine(entry({ kind: CombatLogKind.MOVED, actorLabel: 'Pensantus', distanceFt: 10 }))?.text).toBe(' anda 3 m');
    expect(logLine(entry({ kind: CombatLogKind.ACTION, actorLabel: 'Brisa', key: 'standard:hide', keyNamePt: 'Esconder' }))?.text).toBe(' se esconde');
    expect(logLine(entry({ kind: CombatLogKind.ACTION, actorLabel: 'Toren', key: 'standard:dash', keyNamePt: 'Disparada' }))?.text).toBe(' usa Disparada');
    const adjusted = logLine(entry({ kind: CombatLogKind.HIT_POINTS_ADJUSTED, targetLabel: 'Goblin 2', hitPointsDelta: -3, hitPointsAfter: 4 }));
    expect([adjusted?.actor, adjusted?.text]).toEqual(['Goblin 2', ' perdeu 3 PV, por ajuste do mestre, agora com 4 PV']);
    const revealed = logLine(entry({ kind: CombatLogKind.REVEAL_CHANGED, targetLabel: 'Goblin 3', nowHidden: false }));
    expect([revealed?.actor, revealed?.text]).toEqual(['Goblin 3', ' foi revelado aos jogadores']);
    expect(logLine(entry({ kind: CombatLogKind.COMBAT_BEGUN }), 'Emboscada na estrada')?.text).toBe('Combate iniciado: Emboscada na estrada');
    expect(logLine(entry({ kind: CombatLogKind.COMBAT_ENDED }))?.text).toBe('Combate encerrado');
  });

  it('marks what only the master sees, and skips a kind it does not know', () => {
    expect(logLine(entry({ kind: CombatLogKind.ACTION, actorLabel: 'Goblin 3', key: 'standard:hide', hidden: true }))?.hidden).toBe(true);
    expect(logLine(entry({ kind: 99 as CombatLogKind, actorLabel: 'Alguém' }))).toBeNull();
    const rounds = [create(CombatLogRoundSchema, { round: 1, entries: [entry({ kind: 99 as CombatLogKind }), entry({ kind: CombatLogKind.MOVED, actorLabel: 'Toren', distanceFt: 5 })] })];
    expect(logGroups(rounds, 1)[0].lines.map((l) => l.actor)).toEqual(['Toren']);
  });
});

describe('the rounds of the log', () => {
  const rounds = [
    create(CombatLogRoundSchema, { round: 2, entries: [entry({ kind: CombatLogKind.MOVED, actorLabel: 'Pensantus', distanceFt: 10 })] }),
    create(CombatLogRoundSchema, { round: 1, entries: [entry({ kind: CombatLogKind.COMBAT_BEGUN })] }),
  ];

  it('groups latest first, "em andamento" for the current round, "A rodada 2 começou" at its end', () => {
    const groups = logGroups(rounds, 2);
    expect(groups.map((g) => [g.title, g.status])).toEqual([
      ['Rodada 2', 'em andamento'],
      ['Rodada 1', 'encerrada'],
    ]);
    expect(groups[0].lines.at(-1)?.text).toBe('A rodada 2 começou');
    expect(latestLine(groups)?.actor).toBe('Pensantus');
    expect(entryCount(groups)).toBe(2);
  });
});

describe('the undo label', () => {
  it('names the attack, with the damage', () => {
    const e = attack({ actorLabel: 'Capitão Goblin', targetLabel: 'Toren', undoable: true, damage: { status: PendingDamageStatus.APPLIED, amount: 5 } });
    expect(undoLabel(e)).toBe('o ataque do Capitão Goblin ao Toren (5 de dano)');
    expect(undoLabel(attack({ actorLabel: 'Goblin 3', targetLabel: 'Brisa', outcome: AttackOutcome.MISS }))).toBe('o ataque do Goblin 3 à Brisa (errou)');
    const round = create(CombatLogRoundSchema, { round: 2, entries: [e] });
    expect(undoableEntry([round])).toBe(e);
  });
});
