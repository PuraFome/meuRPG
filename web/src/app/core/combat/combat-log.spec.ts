import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
  CombatLogRoundSchema,
  CoverDegree,
  CoverSource,
  DeathSaveOutcome,
  JumpKind,
  PendingDamageStatus,
  WildShapeEndReason,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  article,
  entryCount,
  latestLine,
  logGroups,
  logLine,
  truncateGroups,
  undoLabel,
  undoableEntry,
} from './combat-log';

type Over = Omit<MessageInitShape<typeof CombatLogEntrySchema>, 'damage' | '$typeName'> & {
  damage?: {
    status: PendingDamageStatus;
    amount: number;
    defeated?: boolean;
    rolledAmount?: number;
    concentrationDc?: number;
    deathFailuresAdded?: number;
    healing?: boolean;
    down?: boolean;
  };
};

function entry(over: Over): CombatLogEntry {
  const { damage, ...rest } = over;
  return create(CombatLogEntrySchema, {
    id: Math.random().toString(36).slice(2),
    ...rest,
    damage: damage && {
      ...damage,
      targetDefeated: damage.defeated ?? false,
      targetDown: damage.down ?? false,
    },
  });
}

const attack = (over: Over) =>
  entry({ kind: CombatLogKind.ATTACK, outcome: AttackOutcome.HIT, ...over });

describe('the combat log sentences (timeline.md, Rodadas 1 and 2)', () => {
  it('writes an attack with its damage, and a defeat', () => {
    expect(
      logLine(
        attack({
          actorLabel: 'Toren',
          targetLabel: 'Goblin 1',
          key: 'equipment:battleaxe',
          keyNamePt: 'Machado de batalha',
          damage: { status: PendingDamageStatus.APPLIED, amount: 9, defeated: true },
        }),
      )?.text,
    ).toBe(' ataca o Goblin 1 com o Machado de batalha: acertou, 9 de dano. Goblin 1 derrotado');
  });

  it('says "atira" for a shot, and the article of each name', () => {
    const shot = attack({
      actorLabel: 'Capitão Goblin',
      targetLabel: 'Toren',
      key: 'equipment:shortbow',
      keyNamePt: 'Arco curto',
      damage: { status: PendingDamageStatus.APPLIED, amount: 5 },
    });
    expect(logLine(shot)?.text).toBe(' atira no Toren com o Arco curto: acertou, 5 de dano');
    const onBrisa = attack({
      actorLabel: 'Goblin 2',
      targetLabel: 'Brisa',
      key: 'equipment:shortbow',
      keyNamePt: 'Arco curto',
      damage: { status: PendingDamageStatus.APPLIED, amount: 5 },
    });
    expect(logLine(onBrisa)?.text).toBe(' atira na Brisa com o Arco curto: acertou, 5 de dano');
    const rapier = attack({
      actorLabel: 'Brisa',
      targetLabel: 'Capitão Goblin',
      key: 'equipment:rapier',
      keyNamePt: 'Rapieira',
      damage: { status: PendingDamageStatus.APPLIED, amount: 8 },
    });
    expect(logLine(rapier)?.text).toBe(
      ' ataca o Capitão Goblin com a Rapieira: acertou, 8 de dano',
    );
  });

  it('draws a spell attack with the spell icon, a weapon attack with the swords', () => {
    expect(
      logLine(
        attack({
          actorLabel: 'Pensantus',
          targetLabel: 'Goblin 2',
          key: 'spell:fire-bolt',
          keyNamePt: 'Raio de Fogo',
        }),
      )?.icon,
    ).toBe('auto_awesome');
    expect(
      logLine(attack({ actorLabel: 'Toren', targetLabel: 'Goblin 1', key: 'equipment:battleaxe' }))
        ?.icon,
    ).toBe('swords');
  });

  it('says when Escudo stopped the hit', () => {
    expect(
      logLine(
        attack({
          actorLabel: 'Capitão Goblin',
          targetLabel: 'Pensantus',
          key: 'equipment:shortbow',
          keyNamePt: 'Arco curto',
          outcome: AttackOutcome.MISS,
          stoppedByReaction: true,
        }),
      )?.text,
    ).toBe(' atira no Pensantus com o Arco curto: errou, o Escudo Arcano segurou');
  });

  it('says a miss, a critical, a pending and a discarded damage', () => {
    expect(
      logLine(
        attack({
          actorLabel: 'Brisa',
          targetLabel: 'Capitão Goblin',
          keyNamePt: 'Rapieira',
          outcome: AttackOutcome.MISS,
        }),
      )?.text,
    ).toBe(' ataca o Capitão Goblin com a Rapieira: errou');
    expect(
      logLine(
        attack({
          actorLabel: 'Goblin 3',
          targetLabel: 'Brisa',
          outcome: AttackOutcome.CRITICAL_HIT,
          damage: { status: PendingDamageStatus.ROLLED, amount: 11 },
        }),
      )?.text,
    ).toBe(' ataca a Brisa: crítico, 11 de dano, esperando o mestre aplicar');
    expect(
      logLine(
        attack({
          actorLabel: 'Goblin 3',
          targetLabel: 'Brisa',
          damage: { status: PendingDamageStatus.AWAITING_ROLL, amount: 0 },
        }),
      )?.text,
    ).toContain('falta rolar o dano');
    expect(
      logLine(
        attack({
          actorLabel: 'Goblin 3',
          targetLabel: 'Brisa',
          damage: { status: PendingDamageStatus.DISCARDED, amount: 5 },
        }),
      )?.text,
    ).toContain('5 de dano descartado');
  });

  it('gives the master the sum of an attack on a covered target, and nobody else', () => {
    const base = {
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Pensantus',
      targetLabel: 'Goblin 2',
      key: 'spell:fire-bolt',
      keyNamePt: 'Raio de Fogo',
      outcome: AttackOutcome.MISS,
    };
    expect(
      logLine(
        entry({
          ...base,
          targetArmorClass: 17,
          coverBonus: 2,
          cover: CoverDegree.HALF,
          coverSource: CoverSource.MAP,
        }),
      )?.text,
    ).toBe(
      ' atira no Goblin 2 com o Raio de Fogo: errou (CA 17: 15 + 2 de meia cobertura, do mapa)',
    );
    // A player's line has no armor class, so it has no sum.
    expect(
      logLine(entry({ ...base, cover: CoverDegree.HALF, coverSource: CoverSource.MAP }))?.text,
    ).toBe(' atira no Goblin 2 com o Raio de Fogo: errou');
  });

  it('says "atira" for a net, and a heal as hit points regained', () => {
    expect(
      logLine(
        attack({
          actorLabel: 'Brisa',
          targetLabel: 'Toren',
          key: 'equipment:net',
          keyNamePt: 'Rede',
          outcome: AttackOutcome.MISS,
        }),
      )?.text,
    ).toBe(' atira no Toren com a Rede: errou');
    expect(
      logLine(
        attack({
          actorLabel: 'Brisa',
          targetLabel: 'Toren',
          damage: { status: PendingDamageStatus.APPLIED, amount: 7, healing: true },
        }),
      )?.text,
    ).toBe(' ataca o Toren: acertou recupera 7 PV');
  });

  it('says who fell, apart from who was defeated', () => {
    expect(
      logLine(
        attack({
          actorLabel: 'Toren',
          targetLabel: 'Brisa',
          damage: { status: PendingDamageStatus.APPLIED, amount: 5, down: true },
        }),
      )?.text,
    ).toBe(' ataca a Brisa: acertou, 5 de dano. Brisa caiu');
  });

  it("gives the cover sum only when the cover added to the armor class, and calls the mark the master's own", () => {
    const base = {
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Pensantus',
      targetLabel: 'Goblin 2',
      key: 'spell:fire-bolt',
      keyNamePt: 'Raio de Fogo',
      outcome: AttackOutcome.MISS,
      targetArmorClass: 17,
      cover: CoverDegree.HALF,
    };
    expect(logLine(entry({ ...base, coverBonus: 0, coverSource: CoverSource.MAP }))?.text).toBe(
      ' atira no Goblin 2 com o Raio de Fogo: errou',
    );
    expect(logLine(entry({ ...base, coverBonus: 2, coverSource: CoverSource.MARK }))?.text).toBe(
      ' atira no Goblin 2 com o Raio de Fogo: errou (CA 17: 15 + 2 de meia cobertura, marcada por você)',
    );
  });

  it("writes a jump with its length, a high one with its height, and the master's reminder for rubble (E9-06)", () => {
    const plain = (t: string | undefined) => (t ?? '').replace(/\u00a0/g, ' ');
    expect(
      plain(
        logLine(
          entry({
            kind: CombatLogKind.MOVED,
            actorLabel: 'Toren',
            distanceFt: 15,
            distanceDft: 150,
            jump: JumpKind.LONG,
          }),
        )?.text,
      ),
    ).toBe(' saltou 4,5 m');
    expect(
      plain(
        logLine(
          entry({
            kind: CombatLogKind.MOVED,
            actorLabel: 'Toren',
            distanceDft: 0,
            jump: JumpKind.HIGH,
            jumpHeightDft: 60,
          }),
        )?.text,
      ),
    ).toBe(' saltou 1,8 m para cima');
    expect(
      plain(
        logLine(
          entry({
            kind: CombatLogKind.MOVED,
            actorLabel: 'Toren',
            distanceDft: 140,
            jump: JumpKind.LONG,
            landingDifficult: true,
          }),
        )?.text,
      ),
    ).toBe(' saltou 4,2 m e caiu em terreno difícil. Acrobacia CD 10 ou cai Derrubado');
    expect(
      plain(
        logLine(
          entry({
            kind: CombatLogKind.MOVED,
            actorLabel: 'Toren',
            distanceFt: 10,
            distanceDft: 100,
          }),
        )?.text,
      ),
    ).toBe(' anda 3,0 m');
  });

  it('writes moves, standard actions, hit point changes, reveals, start and end', () => {
    expect(
      logLine(entry({ kind: CombatLogKind.MOVED, actorLabel: 'Pensantus', distanceFt: 10 }))?.text,
    ).toBe(' anda 3\u00a0m');
    expect(
      logLine(
        entry({
          kind: CombatLogKind.ACTION,
          actorLabel: 'Brisa',
          key: 'standard:hide',
          keyNamePt: 'Esconder',
        }),
      )?.text,
    ).toBe(' se esconde');
    expect(
      logLine(
        entry({
          kind: CombatLogKind.ACTION,
          actorLabel: 'Toren',
          key: 'standard:dash',
          keyNamePt: 'Disparada',
        }),
      )?.text,
    ).toBe(' usa Disparada');
    const adjusted = logLine(
      entry({
        kind: CombatLogKind.HIT_POINTS_ADJUSTED,
        targetLabel: 'Goblin 2',
        hitPointsDelta: -3,
        hitPointsAfter: 4,
      }),
    );
    expect([adjusted?.actor, adjusted?.text]).toEqual([
      'Goblin 2',
      ' perdeu 3 PV, por ajuste do mestre, agora com 4 PV',
    ]);
    const revealed = logLine(
      entry({ kind: CombatLogKind.REVEAL_CHANGED, targetLabel: 'Goblin 3', nowHidden: false }),
    );
    expect([revealed?.actor, revealed?.text]).toEqual(['Goblin 3', ' foi revelado aos jogadores']);
    expect(logLine(entry({ kind: CombatLogKind.COMBAT_BEGUN }), 'Emboscada na estrada')?.text).toBe(
      'Combate iniciado: Emboscada na estrada',
    );
    expect(logLine(entry({ kind: CombatLogKind.COMBAT_ENDED }))?.text).toBe('Combate encerrado');
  });

  it('marks what only the master sees, and skips a kind it does not know', () => {
    expect(
      logLine(
        entry({
          kind: CombatLogKind.ACTION,
          actorLabel: 'Goblin 3',
          key: 'standard:hide',
          hidden: true,
        }),
      )?.hidden,
    ).toBe(true);
    expect(logLine(entry({ kind: 99 as CombatLogKind, actorLabel: 'Alguém' }))).toBeNull();
    const rounds = [
      create(CombatLogRoundSchema, {
        round: 1,
        entries: [
          entry({ kind: 99 as CombatLogKind }),
          entry({ kind: CombatLogKind.MOVED, actorLabel: 'Toren', distanceFt: 5 }),
        ],
      }),
    ];
    expect(logGroups(rounds, 1)[0].lines.map((l) => l.actor)).toEqual(['Toren']);
  });
});

describe('the rounds of the log', () => {
  const rounds = [
    create(CombatLogRoundSchema, {
      round: 2,
      entries: [entry({ kind: CombatLogKind.MOVED, actorLabel: 'Pensantus', distanceFt: 10 })],
    }),
    create(CombatLogRoundSchema, {
      round: 1,
      entries: [entry({ kind: CombatLogKind.COMBAT_BEGUN })],
    }),
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

describe('the articles of the names', () => {
  it('knows the feminine words that do not end in "a"', () => {
    expect(article('Mace')).toBe('a');
    expect(article('Foice')).toBe('a');
    expect(article('Machado')).toBe('o');
  });
});

describe('the short view of the log', () => {
  it('keeps the rounds it reaches and stops when the lines run out', () => {
    const line = (text: string) => ({ text }) as never;
    const groups = [
      {
        round: 2,
        title: 'Rodada 2',
        status: 'em andamento',
        lines: [line('a'), line('b'), line('c')],
      },
      { round: 1, title: 'Rodada 1', status: 'encerrada', lines: [line('d'), line('e')] },
    ];
    expect(truncateGroups(groups, 2).map((g) => g.lines.length)).toEqual([2]);
    expect(truncateGroups(groups, 3).map((g) => g.lines.length)).toEqual([3]);
    expect(truncateGroups(groups, 4).map((g) => g.lines.length)).toEqual([3, 1]);
    expect(truncateGroups(groups, 0)).toEqual([]);
  });

  it('finds the undoable entry past the ones that cannot be undone', () => {
    const first = attack({ actorLabel: 'A', targetLabel: 'B', undoable: false });
    const second = attack({ actorLabel: 'A', targetLabel: 'B', undoable: true });
    const round = create(CombatLogRoundSchema, { round: 2, entries: [first, second] });
    expect(undoableEntry([round])).toBe(second);
    expect(
      undoableEntry([create(CombatLogRoundSchema, { round: 1, entries: [first] })]),
    ).toBeNull();
  });
});

describe('the undo label', () => {
  it('names the attack, with the damage', () => {
    const e = attack({
      actorLabel: 'Capitão Goblin',
      targetLabel: 'Toren',
      undoable: true,
      damage: { status: PendingDamageStatus.APPLIED, amount: 5 },
    });
    expect(undoLabel(e)).toBe('o ataque do Capitão Goblin ao Toren (5 de dano)');
    expect(
      undoLabel(
        attack({ actorLabel: 'Goblin 3', targetLabel: 'Brisa', outcome: AttackOutcome.MISS }),
      ),
    ).toBe('o ataque do Goblin 3 à Brisa (errou)');
    const round = create(CombatLogRoundSchema, { round: 2, entries: [e] });
    expect(undoableEntry([round])).toBe(e);
  });
});

describe('the log of spells, reactions, the fallen and conditions (slice 6.5c)', () => {
  const roll = (faces: number[], modifier: number) =>
    ({
      diceCount: faces.length,
      diceSides: 20,
      faces,
      modifier,
      total: faces.reduce((a, b) => a + b, 0) + modifier,
      physical: false,
    }) as never;

  it('writes a cast with its darts, a save and the concentration that ended', () => {
    const darts = entry({
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: 'Pensantus',
      keyNamePt: 'Mísseis Mágicos',
      spell: {
        slot: { level: 1, pact: false },
        concentrating: false,
        concentrationEndedKey: '',
        targets: [
          {
            targetId: 'c',
            targetLabel: 'Capitão Goblin',
            darts: 2,
            outcome: AttackOutcome.UNSPECIFIED,
            damage: { status: PendingDamageStatus.APPLIED, amount: 7 },
          },
          {
            targetId: 'g',
            targetLabel: 'Goblin 2',
            darts: 1,
            outcome: AttackOutcome.UNSPECIFIED,
            damage: { status: PendingDamageStatus.APPLIED, amount: 5 },
          },
        ],
      },
    } as never);
    expect(logLine(darts)?.text).toBe(
      ' conjura Mísseis Mágicos (1º\u00a0nível): 2 dardos no Capitão Goblin, 7 de dano; 1 dardo no Goblin 2, 5 de dano',
    );
    expect(logLine(darts)?.icon).toBe('auto_awesome');
    const save = entry({
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: 'Pensantus',
      keyNamePt: 'Mãos Flamejantes',
      spell: {
        slot: { level: 1, pact: false },
        concentrationEndedKey: 'spell:web',
        targets: [
          {
            targetId: 'g',
            targetLabel: 'Goblin 1',
            save: { outcome: 2, dc: 14 },
            damage: { status: PendingDamageStatus.APPLIED, amount: 10, half: false },
          },
          {
            targetId: 'h',
            targetLabel: 'Goblin 2',
            save: { outcome: 1, dc: 14 },
            damage: { status: PendingDamageStatus.APPLIED, amount: 5, half: true },
          },
        ],
      },
    } as never);
    expect(logLine(save)?.text).toBe(
      ' conjura Mãos Flamejantes (1º\u00a0nível): o Goblin 1 falhou (CD 14), 10 de dano; o Goblin 2 resistiu (CD 14), 5 de dano (metade). A concentração anterior acabou',
    );
  });

  it("tells the master that an NPC's saving throw has no known bonus, and nobody else", () => {
    const cast = entry({
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: 'Pensantus',
      keyNamePt: 'Mãos Flamejantes',
      spell: {
        slot: { level: 1, pact: false },
        targets: [
          {
            targetId: 'g',
            targetLabel: 'Goblin 1',
            save: {
              outcome: 2,
              dc: 14,
              bonusKnown: false,
              roll: { diceCount: 1, diceSides: 20, faces: [13], modifier: 0, total: 13 },
            },
          },
          {
            targetId: 'c',
            targetLabel: 'Capitão Goblin',
            save: {
              outcome: 1,
              dc: 14,
              bonusKnown: true,
              roll: { diceCount: 1, diceSides: 20, faces: [15], modifier: 2, total: 17 },
            },
          },
        ],
      },
    } as never);
    const master = { master: true, players: new Set<string>() };
    expect(logLine(cast, '', master)?.text).toContain(
      'o Goblin 1 falhou (CD 14; d20 13, bônus desconhecido)',
    );
    // Positive control: a known bonus says nothing more.
    expect(logLine(cast, '', master)?.text).toContain('o Capitão Goblin resistiu (CD 14)');
    // A player reads `bonus_known` false on their own roll too, and it means nothing.
    const player = { master: false, players: new Set(['Goblin 1']) };
    expect(logLine(cast, '', player)?.text).not.toContain('sem bônus');
  });

  it('names who a spell with no effect the app knows touches', () => {
    const sleep = entry({
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: 'Pensantus',
      keyNamePt: 'Sono',
      spell: {
        slot: { level: 1, pact: false },
        targets: [
          { targetId: 'g', targetLabel: 'Goblin 1' },
          { targetId: 'h', targetLabel: 'Goblin 2' },
        ],
      },
    } as never);
    expect(logLine(sleep)?.text).toBe(' conjura Sono (1º\u00a0nível) no Goblin 1 e no Goblin 2');
  });

  it('writes a reaction window as the one line the server wrote for whoever reads it', () => {
    const line = logLine(
      entry({
        kind: CombatLogKind.REACTION_WINDOW,
        actorLabel: 'Brisa',
        reactionTextPt: 'Brisa usou Esquiva Sobrenatural: o dano caiu pela metade.',
      }),
    );
    expect(line?.actor).toBe('');
    expect(line?.text).toBe('Brisa usou Esquiva Sobrenatural: o dano caiu pela metade.');
  });

  it('writes the reaction and an opportunity attack', () => {
    expect(
      logLine(
        entry({
          kind: CombatLogKind.REACTION,
          actorLabel: 'Pensantus',
          keyNamePt: 'Escudo Arcano',
          spell: { slot: { level: 1, pact: false } },
        } as never),
      )?.text,
    ).toBe(' conjura Escudo Arcano (1º\u00a0nível), com a reação');
    expect(
      logLine(
        attack({
          actorLabel: 'Pensantus',
          targetLabel: 'Goblin 1',
          key: 'equipment:dagger',
          keyNamePt: 'Adaga',
          asReaction: true,
          damage: { status: PendingDamageStatus.APPLIED, amount: 4 },
        }),
      )?.text,
    ).toBe(' ataca o Goblin 1 com a Adaga (ataque de oportunidade): acertou, 4 de dano');
  });

  it('says that an opportunity attack sent the mover back to the reach, or could not', () => {
    const opportunity = {
      actorLabel: 'Pensantus',
      targetLabel: 'Goblin 1',
      key: 'equipment:dagger',
      keyNamePt: 'Adaga',
      asReaction: true,
      damage: { status: PendingDamageStatus.APPLIED, amount: 9, defeated: true },
    };
    expect(logLine(attack({ ...opportunity, returnedToReach: true }))?.text).toBe(
      ' ataca o Goblin 1 com a Adaga (ataque de oportunidade): acertou, 9 de dano. Goblin 1 derrotado. Goblin 1 voltou ao último quadrado dentro do alcance',
    );
    expect(logLine(attack({ ...opportunity, returnBlocked: true }))?.text).toBe(
      ' ataca o Goblin 1 com a Adaga (ataque de oportunidade): acertou, 9 de dano. Goblin 1 derrotado. Goblin 1 não pôde voltar ao último quadrado dentro do alcance: ele estava ocupado',
    );
    expect(logLine(attack(opportunity))?.text).not.toContain('voltou');
  });

  it('tells every damage type of a spell on a target, and the master the armor class of its attack', () => {
    const storm = (target: object) =>
      entry({
        kind: CombatLogKind.SPELL_CAST,
        actorLabel: 'Pensantus',
        keyNamePt: 'Tempestade de Gelo',
        spell: { slot: { level: 4, pact: false }, targets: [target] },
      } as never);
    const damages = {
      damage: { status: PendingDamageStatus.APPLIED, amount: 12, damageTypePt: 'contundente' },
      moreDamages: [{ status: PendingDamageStatus.APPLIED, amount: 8, damageTypePt: 'gélido' }],
    };
    expect(
      logLine(
        storm({ targetId: 'g', targetLabel: 'Goblin 1', save: { outcome: 2, dc: 15 }, ...damages }),
      )?.text,
    ).toBe(
      ' conjura Tempestade de Gelo (4º\u00a0nível): o Goblin 1 falhou (CD 15), 12 de dano, 8 de dano',
    );
    const bolt = {
      targetId: 'g',
      targetLabel: 'Goblin 1',
      outcome: AttackOutcome.HIT,
      damage: { status: PendingDamageStatus.APPLIED, amount: 6 },
      cover: CoverDegree.HALF,
      coverSource: CoverSource.MAP,
    };
    // Only the master's entry carries the armor class and the cover bonus; the player's line is as before.
    expect(logLine(storm({ ...bolt, targetArmorClass: 17, coverBonus: 2 }))?.text).toBe(
      ' conjura Tempestade de Gelo (4º\u00a0nível): no Goblin 1: acertou (CA 17: 15 + 2 de meia cobertura, do mapa), 6 de dano',
    );
    expect(logLine(storm(bolt))?.text).toBe(
      ' conjura Tempestade de Gelo (4º\u00a0nível): no Goblin 1: acertou, 6 de dano',
    );
  });

  it('writes a damage the master changed, the death save failures and the concentration reminder', () => {
    const hit = entry({
      kind: CombatLogKind.ATTACK,
      outcome: AttackOutcome.HIT,
      actorLabel: 'Capitão Goblin',
      targetLabel: 'Toren',
      key: 'equipment:scimitar',
      keyNamePt: 'Cimitarra',
      damage: {
        status: PendingDamageStatus.APPLIED,
        amount: 2,
        rolledAmount: 5,
        concentrationDc: 10,
      },
    } as never);
    expect(logLine(hit)?.text).toBe(
      ' ataca o Toren com a Cimitarra: acertou, 2 de dano (o dado deu 5). Teste de Constituição, CD 10, para manter a concentração',
    );
    const down = entry({
      kind: CombatLogKind.ATTACK,
      outcome: AttackOutcome.HIT,
      actorLabel: 'Goblin 3',
      targetLabel: 'Brisa',
      key: 'equipment:shortbow',
      keyNamePt: 'Arco curto',
      damage: { status: PendingDamageStatus.APPLIED, amount: 0, deathFailuresAdded: 1 },
    } as never);
    expect(logLine(down)?.text).toContain(', uma falha no teste contra a morte');
  });

  it('writes a death save for the one who may see the dice, and for the others', () => {
    const mine = entry({
      kind: CombatLogKind.DEATH_SAVE,
      actorLabel: 'Brisa',
      deathSave: {
        roll: roll([14], 0),
        outcome: 1,
        successes: 1,
        failures: 1,
        stable: false,
        dying: false,
      },
    } as never);
    expect(logLine(mine)?.text).toBe(
      ' rola o teste contra a morte: 1d20 (14) = 14, sucesso (1 sucesso, 1 falha)',
    );
    const others = entry({
      kind: CombatLogKind.DEATH_SAVE,
      actorLabel: 'Brisa',
      deathSave: { outcome: 2, successes: 1, failures: 2 },
    } as never);
    expect(logLine(others)?.text).toBe(' faz um teste contra a morte: falha (1 sucesso, 2 falhas)');
    const dying = entry({
      kind: CombatLogKind.DEATH_SAVE,
      actorLabel: 'Brisa',
      deathSave: { roll: roll([5], 0), outcome: 2, successes: 0, failures: 3, dying: true },
    } as never);
    expect(logLine(dying)?.text).toContain('Morrendo: o mestre confirma a morte');
    const back = entry({
      kind: CombatLogKind.DEATH_SAVE,
      actorLabel: 'Brisa',
      deathSave: { roll: roll([20], 0), outcome: 4 },
    } as never);
    expect(logLine(back)?.text).toBe(
      ' rola o teste contra a morte: 1d20 (20) = 20, volta com 1 PV',
    );
  });

  it('keeps the roll when a stable save still carries it', () => {
    const stable = entry({
      kind: CombatLogKind.DEATH_SAVE,
      actorLabel: 'Brisa',
      deathSave: { roll: roll([12], 0), stable: true, successes: 3, failures: 0 },
    } as never);
    expect(logLine(stable)?.text).toContain('rola o teste contra a morte');
  });

  it('writes the confirmed death and the conditions', () => {
    expect(
      logLine(entry({ kind: CombatLogKind.DEATH_CONFIRMED, targetLabel: 'Brisa' } as never)),
    ).toMatchObject({ actor: 'Brisa', text: ' morreu' });
    const marked = entry({
      kind: CombatLogKind.CONDITIONS_CHANGED,
      targetLabel: 'Goblin 1',
      conditions: ['condition:poisoned', 'condition:prone'],
    } as never);
    expect(logLine(marked)).toMatchObject({
      actor: 'Goblin 1',
      text: ' ficou Envenenado e Derrubado',
    });
    const ended = entry({
      kind: CombatLogKind.CONDITIONS_CHANGED,
      targetLabel: 'Toren',
      concentrationEndedKey: 'spell:bless',
    } as never);
    expect(logLine(ended)?.text).toBe(' deixou de se concentrar');
    const cleared = entry({
      kind: CombatLogKind.CONDITIONS_CHANGED,
      targetLabel: 'Toren',
      conditions: [],
    } as never);
    expect(logLine(cleared)?.text).toBe(' ficou sem condições');
  });

  it('writes "Toren abriu a porta." for a door a move opened (RN-26): the actor, then the sentence', () => {
    const line = logLine(
      entry({
        kind: CombatLogKind.DOOR_OPENED,
        actorLabel: 'Toren',
        door: { col: 4, row: 2 },
      } as never),
    );
    expect(line).toMatchObject({ actor: 'Toren', icon: 'door_open', text: ' abriu a porta' });
  });

  it('writes the Wild Shape lines: the form, and why it ended, with the number only where the server sent it (RN-20)', () => {
    const ws = (over: {
      started?: boolean;
      endReason?: WildShapeEndReason;
      carriedDamage?: number;
    }) =>
      entry({
        kind: CombatLogKind.WILD_SHAPE,
        actorLabel: 'Sálvia',
        wildShape: {
          beastKey: 'monster:wolf',
          beastNamePt: 'Lobo',
          started: false,
          endReason: WildShapeEndReason.LEFT,
          carriedDamage: 0,
          ...over,
        },
      } as never);
    expect(logLine(ws({ started: true }))).toMatchObject({
      actor: 'Sálvia',
      icon: 'pets',
      text: ' virou o Lobo',
    });
    expect(logLine(ws({}))?.text).toBe(' voltou à forma normal');
    // The owner's player and the master get the number; another player's line is the same sentence with none.
    expect(logLine(ws({ endReason: WildShapeEndReason.DAMAGE, carriedDamage: 6 }))?.text).toBe(
      ' voltou à forma normal: o Lobo caiu a 0 PV e 6 de dano passaram para ela',
    );
    expect(logLine(ws({ endReason: WildShapeEndReason.DAMAGE }))?.text).toBe(
      ' voltou à forma normal: o Lobo caiu a 0 PV',
    );
    expect(logLine(ws({ endReason: WildShapeEndReason.MASTER }))?.text).toBe(
      ' voltou à forma normal, porque o mestre levou o Lobo a 0 PV',
    );
    expect(logLine(ws({ endReason: WildShapeEndReason.ZERO_HP }))?.text).toBe(
      ' voltou à forma normal, ao cair a 0 PV',
    );
    expect(logLine(ws({ endReason: WildShapeEndReason.UNCONSCIOUS }))?.text).toBe(
      ' voltou à forma normal, ao ficar inconsciente',
    );
  });

  it('names what an undo would take back', () => {
    expect(
      undoLabel(
        entry({
          kind: CombatLogKind.SPELL_CAST,
          actorLabel: 'Pensantus',
          keyNamePt: 'Sono',
        } as never),
      ),
    ).toBe('a magia Sono do Pensantus');
    expect(undoLabel(entry({ kind: CombatLogKind.DEATH_SAVE, actorLabel: 'Brisa' } as never))).toBe(
      'o teste contra a morte da Brisa',
    );
  });
});

describe('the monsters the master put in (MR-042, RN-29): a line for the master alone, with the hit points and the dice', () => {
  it("writes the rolled hit points with their dice, and flags the line as the master's alone", () => {
    const line = logLine(
      entry({
        kind: CombatLogKind.MONSTERS_ADDED,
        monsters: [
          {
            combatantId: 'm1',
            label: 'Bandido 1',
            hitPoints: 9,
            rolled: true,
            dice: '2d8+2',
            faces: [3, 4],
            modifier: 2,
          },
          {
            combatantId: 'm2',
            label: 'Bandido 2',
            hitPoints: 13,
            rolled: true,
            dice: '2d8+2',
            faces: [6, 5],
            modifier: 2,
          },
        ],
      } as never),
    );
    expect(line?.hidden).toBe(true);
    expect(line?.actor).toBe('');
    expect(line?.text).toBe(
      'Você pôs 2 monstros no combate. Bandido 1: 9 PV (2d8 + 2: 3, 4); Bandido 2: 13 PV (2d8 + 2: 6, 5)',
    );
  });

  it('writes the average as the average', () => {
    const line = logLine(
      entry({
        kind: CombatLogKind.MONSTERS_ADDED,
        monsters: [
          {
            combatantId: 'm1',
            label: 'Ogro',
            hitPoints: 59,
            rolled: false,
            dice: '',
            faces: [],
            modifier: 0,
          },
        ],
      } as never),
    );
    expect(line?.text).toBe('Você pôs um monstro no combate. Ogro: 59 PV (média)');
  });

  it('puts the round-0 group before the combat begins, titled in words', () => {
    const rounds = [
      create(CombatLogRoundSchema, {
        round: 1,
        entries: [create(CombatLogEntrySchema, { id: 'a', kind: CombatLogKind.COMBAT_BEGUN })],
      }),
      create(CombatLogRoundSchema, {
        round: 0,
        entries: [
          create(CombatLogEntrySchema, {
            id: 'b',
            kind: CombatLogKind.MONSTERS_ADDED,
            monsters: [
              {
                combatantId: 'm1',
                label: 'Bandido',
                hitPoints: 11,
                rolled: false,
                dice: '',
                faces: [],
                modifier: 0,
              },
            ],
          }),
        ],
      }),
    ];
    const groups = logGroups(rounds, 1, 'Emboscada');
    expect(groups.map((g) => [g.title, g.status])).toEqual([
      ['Rodada 1', 'em andamento'],
      ['Antes do combate', 'encerrada'],
    ]);
    expect(groups[1].lines[0].text).toContain('Você pôs um monstro no combate');
    // Still being set up: the round-0 group is the one under way.
    expect(logGroups([rounds[1]], 0)[0].status).toBe('em preparação');
  });
});

describe('the log of a combat without a map (RN-25) and of hidden death saves (RN-24)', () => {
  const theatre = { master: true, players: new Set<string>(), theatre: true };

  it('says movement spent by number, in meters, instead of a walk', () => {
    expect(
      logLine(
        entry({ kind: CombatLogKind.MOVED, actorLabel: 'Toren', distanceDft: 200, distanceFt: 20 }),
        '',
        theatre,
      ),
    ).toMatchObject({
      icon: 'directions_run',
      text: ' gastou 6,0\u00a0m de movimento',
    });
    // On a map the same entry is still a walk.
    expect(
      logLine(
        entry({ kind: CombatLogKind.MOVED, actorLabel: 'Toren', distanceDft: 200, distanceFt: 20 }),
      )?.text,
    ).toBe(' anda 6,0\u00a0m');
  });

  it('says the combat began without a map', () => {
    expect(
      logLine(entry({ kind: CombatLogKind.COMBAT_BEGUN }), 'Emboscada na estrada', theatre)?.text,
    ).toBe('Combate iniciado: Emboscada na estrada, sem mapa (teatro da mente)');
  });

  it("writes the master's offer with the mover as the actor and the reactor as the target, and names it for the undo", () => {
    const offered = entry({
      kind: CombatLogKind.OPPORTUNITY_OFFERED,
      actorLabel: 'Goblin 1',
      targetLabel: 'Toren',
      undoable: true,
    });
    expect(logLine(offered, '', theatre)).toMatchObject({
      actor: 'Goblin 1',
      icon: 'swords',
      text: ' saiu do alcance de Toren',
      undoable: true,
    });
    expect(undoLabel(offered)).toBe('a oferta de ataque de oportunidade a Toren');
  });

  it('reads a death-save line that carries only "stable" (no roll, no outcome) as "estabilizou"', () => {
    const stable = entry({
      kind: CombatLogKind.DEATH_SAVE,
      actorLabel: 'Brisa',
      deathSave: { stable: true, successes: 0, failures: 0 },
    } as never);
    expect(logLine(stable)?.text).toBe(' estabilizou');
  });

  it('keeps the whole sentence when the table shows the saves (a stable save with its outcome and counts)', () => {
    const stable = entry({
      kind: CombatLogKind.DEATH_SAVE,
      actorLabel: 'Brisa',
      deathSave: { outcome: DeathSaveOutcome.SUCCESS, stable: true, successes: 3, failures: 1 },
    } as never);
    expect(logLine(stable)?.text).toBe(
      ' faz um teste contra a morte: sucesso (3 sucessos, 1 falha). Estável: não rola mais',
    );
  });
});
