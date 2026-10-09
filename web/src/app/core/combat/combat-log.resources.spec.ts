import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  AttackOutcome,
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogResourceSchema,
  CombatLogKind,
  LayOnHandsCureKind,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { logLine } from './combat-log';

const entry = (over: MessageInitShape<typeof CombatLogEntrySchema>): CombatLogEntry =>
  create(CombatLogEntrySchema, { id: 'x', ...over });

const resource = (
  key: string,
  over: MessageInitShape<typeof CombatLogEntrySchema> = {},
  fields: MessageInitShape<typeof CombatLogResourceSchema> = {},
) =>
  entry({
    kind: CombatLogKind.RESOURCE,
    actorLabel: 'Tavo',
    targetLabel: 'Brisa',
    resource: create(CombatLogResourceSchema, { key, ...fields }),
    ...over,
  });

const text = (e: CombatLogEntry) => {
  const line = logLine(e);
  return `${line?.actor}${line?.text}`.replace(/\u00a0/g, ' ');
};

describe('the class resource lines of the log', () => {
  it('writes a heal with the hit points the entry carries', () => {
    expect(text(resource('feature:lay-on-hands', {}, { spent: 8, healed: 6 }))).toBe(
      'Tavo curou 6 PV de Brisa com a Cura pelas Mãos (8 pontos)',
    );
  });

  it('does not invent the hit points when the entry has none (not the target player or the master)', () => {
    expect(text(resource('feature:lay-on-hands', {}, { spent: 8 }))).toBe(
      'Tavo usou a Cura pelas Mãos em Brisa (8 pontos)',
    );
  });

  it('writes the cures', () => {
    expect(
      text(resource('feature:lay-on-hands', {}, { spent: 5, cure: LayOnHandsCureKind.POISON })),
    ).toBe('Tavo neutralizou o veneno de Brisa com a Cura pelas Mãos (5 pontos)');
    expect(
      text(resource('feature:lay-on-hands', {}, { spent: 5, cure: LayOnHandsCureKind.DISEASE })),
    ).toBe('Tavo curou a doença de Brisa com a Cura pelas Mãos (5 pontos)');
  });

  it('writes "sem efeito" for everyone, and the reason only when the entry carries it (the master)', () => {
    const playersLine = text(
      resource(
        'feature:lay-on-hands',
        { targetLabel: 'Esqueleto' },
        { spent: 8, nothingHappened: true },
      ),
    );
    expect(playersLine).toBe('Tavo tocou o Esqueleto com a Cura pelas Mãos (8 pontos): sem efeito');
    const masterLine = text(
      resource(
        'feature:lay-on-hands',
        { targetLabel: 'Esqueleto' },
        { spent: 8, nothingHappened: true, nothingReason: 'morto-vivo' },
      ),
    );
    expect(masterLine).toBe(
      'Tavo tocou o Esqueleto com a Cura pelas Mãos (8 pontos): sem efeito; o toque não agiu: morto-vivo',
    );
  });

  it('writes Flexible Casting in both directions', () => {
    expect(
      text(
        resource('feature:flexible-casting-creating-spell-slots', {}, { spent: 3, slotLevel: 2 }),
      ),
    ).toBe('Tavo criou um espaço de 2º nível com a Conjuração Flexível (3 pontos de feitiçaria)');
    expect(
      text(
        resource('feature:flexible-casting-converting-spell-slot', {}, { slotLevel: 2, gained: 2 }),
      ),
    ).toBe('Tavo converteu um espaço de 2º nível em 2 pontos de feitiçaria');
  });

  it('writes the gift of Bardic Inspiration, with the die when the entry has it', () => {
    expect(
      text(
        resource(
          'feature:bardic-inspiration',
          { actorLabel: 'Orla', targetLabel: 'Toren' },
          { dieSides: 8 },
        ),
      ),
    ).toBe('Orla deu um d8 da Inspiração de Bardo a Toren');
    expect(
      text(resource('feature:bardic-inspiration', { actorLabel: 'Orla', targetLabel: 'Toren' })),
    ).toBe('Orla deu a Inspiração de Bardo a Toren');
  });

  it('skips nothing for a resource key it does not know', () => {
    expect(text(resource('feature:new-thing'))).toBe('Tavo usou um recurso da classe');
  });
});

describe('the die added to an attack, and the Metamagic of a cast', () => {
  const attack = (over: MessageInitShape<typeof CombatLogEntrySchema>) =>
    entry({
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Toren',
      targetLabel: 'Capitão Goblin',
      keyNamePt: 'Espada longa',
      key: 'attack:longsword',
      outcome: AttackOutcome.HIT,
      ...over,
    });

  it('says the bonus die an attacker used, and not one that was kept', () => {
    expect(
      text(
        attack({
          bonusDice: [
            { sourceKey: 'feature:bardic-inspiration-d6', sides: 8, face: 6, used: true },
          ],
        }),
      ),
    ).toBe(
      'Toren ataca o Capitão Goblin com a Espada longa: acertou, com o d8 da Inspiração de Bardo (+6)',
    );
    expect(
      text(
        attack({
          bonusDice: [
            { sourceKey: 'feature:bardic-inspiration-d6', sides: 8, face: 0, used: false },
          ],
        }),
      ),
    ).toBe('Toren ataca o Capitão Goblin com a Espada longa: acertou');
  });

  it('says the Metamagic of a cast and the sorcery points it spent', () => {
    const cast = entry({
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: 'Nael',
      keyNamePt: 'Raio de Gelo',
      spell: {
        metamagicKeys: ['feature:metamagic-twinned-spell'],
        sorceryPointsSpent: 1,
        targets: [],
      },
    });
    expect(text(cast)).toContain(
      'conjura Raio de Gelo, com Magia Duplicada (1 ponto de feitiçaria)',
    );
  });
});
