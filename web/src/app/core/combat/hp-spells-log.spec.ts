import { create } from '@bufbuild/protobuf';

import {
  CombatEffect,
  CombatLogEntrySchema,
  CombatLogKind,
  SpellEffectGain,
  SpellEffectKind,
  SpellEffectOutcome,
  SpellEffectReason,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { logLine } from './combat-log';

/** The pool Pensantus rolled: 5d8 (2, 4, 1, 5, 3) = 15. */
const POOL = {
  diceCount: 5,
  diceSides: 8,
  faces: [2, 4, 1, 5, 3],
  modifier: 0,
  total: 15,
  physical: false,
};

const AFFECTED = SpellEffectOutcome.AFFECTED;
const NOT_AFFECTED = SpellEffectOutcome.NOT_AFFECTED;

/** What the server sends about a Sono: the master gets every number, a player none of the enemy's. */
function sleep(master: boolean) {
  return create(CombatLogEntrySchema, {
    id: 'sono',
    kind: CombatLogKind.SPELL_CAST,
    actorLabel: 'Pensantus',
    keyNamePt: 'Sono',
    spell: {
      slot: { level: 1, pact: false },
      effectKind: SpellEffectKind.POOL,
      effectConditionKey: 'condition:unconscious',
      // The caster's player gets the roll in the cast's answer; the log entry carries it too, and it never prints.
      poolRoll: POOL,
      targets: [
        {
          targetId: 'g1',
          targetLabel: 'Goblin 1',
          effect: master
            ? { outcome: AFFECTED, hitPointsBefore: 7, poolLeft: 8, poolOrder: 1 }
            : { outcome: AFFECTED },
        },
        {
          targetId: 'cap',
          targetLabel: 'Capitão Goblin',
          effect: master
            ? {
                outcome: NOT_AFFECTED,
                reason: SpellEffectReason.ABOVE_POOL,
                hitPointsBefore: 27,
                poolLeft: 8,
                poolOrder: 2,
              }
            : { outcome: NOT_AFFECTED },
        },
      ],
    },
  });
}

const players = new Set(['Pensantus', 'Brisa', 'Toren']);
const master = { master: true, players };
const player = { master: false, players };
const plain = (text: string) => text.replace(/\u00a0/g, ' ');

describe('Sono in the log (E8-03)', () => {
  it('gives the master the whole account: the dice, each creature from the lowest hit points up, what is left', () => {
    const line = logLine(sleep(true), '', master)!;
    expect(plain(line.text)).toBe(' conjura Sono (1º nível)');
    const card = line.card!;
    expect(card.roll).toBe('5d8 (2, 4, 1, 5, 3) = 15');
    expect(card.rows.map((r) => [r.label, r.hitPoints, plain(r.math), r.word, r.icon])).toEqual([
      ['Goblin 1', '7 PV', '15 − 7 = 8 restam', 'Adormeceu · Inconsciente', 'bedtime'],
      ['Capitão Goblin', '27 PV', '27 é mais que 8 restantes', 'Não afetado', 'block'],
    ]);
    expect(card.changeFor).toEqual([{ id: 'g1', label: 'Goblin 1' }]);
    expect(plain(card.slot)).toBe('Pensantus gastou um espaço de 1º nível.');
    expect(card.summary).toContain('Goblin 1 (7 PV) adormeceu');
  });

  it("puts the creatures in the order the pool went through them, not the caster's", () => {
    const entry = sleep(true);
    entry.spell!.targets.reverse();
    expect(logLine(entry, '', master)!.card!.rows.map((r) => r.label)).toEqual([
      'Goblin 1',
      'Capitão Goblin',
    ]);
  });

  it('gives a player only who was affected: no card, no hit points, no total, no reason', () => {
    const line = logLine(sleep(false), '', player)!;
    expect(plain(line.text)).toBe(
      ' conjura Sono: o Goblin 1 adormece. O Capitão Goblin não foi afetado.',
    );
    expect(line.card).toBeUndefined();
    // Not even the caster's pool: it is in the caster's own sheet, never in the table's log.
    expect(line.text).not.toMatch(/PV|restam|\d{2,}/);
  });
});

describe('the other spells that read hit points in the log (E8-03)', () => {
  const cast = (key: string, name: string, spell: object, actor = 'Oda') =>
    create(CombatLogEntrySchema, {
      id: key,
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: actor,
      keyNamePt: name,
      spell,
    } as never);

  const spare = (master: boolean) =>
    cast('poupar', 'Estabilizar', {
      effectKind: SpellEffectKind.ZERO_HP,
      targets: [
        {
          targetId: 'b',
          targetLabel: 'Brisa',
          effect: master ? { outcome: AFFECTED, hitPointsBefore: 0 } : { outcome: AFFECTED },
        },
      ],
    });

  const stun = (master: boolean) =>
    cast('atordoar', 'Palavra de Poder Atordoar', {
      slot: { level: 8, pact: false },
      effectKind: SpellEffectKind.THRESHOLD,
      effectConditionKey: 'condition:stunned',
      effectThreshold: master ? 150 : undefined,
      targets: [
        {
          targetId: 'c',
          targetLabel: 'Capitão Goblin',
          effect: master ? { outcome: AFFECTED, hitPointsBefore: 27 } : { outcome: AFFECTED },
        },
      ],
    });

  it('Estabilizar: the master reads the hit points, a player that Brisa is stable', () => {
    expect(logLine(spare(true), '', master)!.text).toBe(
      ' conjura Estabilizar em Brisa (0 PV): está estável',
    );
    expect(logLine(spare(false), '', player)!.text).toBe(
      ' conjura Estabilizar: Brisa está estável.',
    );
  });

  it('Palavra de Poder Atordoar: the master reads the hit points and the limit, a player the word', () => {
    expect(plain(logLine(stun(true), '', master)!.text)).toBe(
      ' conjura Palavra de Poder Atordoar no Capitão Goblin (27 PV, limite de 150): fica atordoado',
    );
    expect(plain(logLine(stun(false), '', player)!.text)).toBe(
      ' conjura Palavra de Poder Atordoar: o Capitão Goblin fica atordoado.',
    );
  });

  it('a creature above the limit is "não foi afetado" to a player, and the master is told why', () => {
    const above = (isMaster: boolean) =>
      cast('limite', 'Palavra de Poder Matar', {
        effectKind: SpellEffectKind.THRESHOLD,
        effectThreshold: isMaster ? 100 : undefined,
        targets: [
          {
            targetId: 'c',
            targetLabel: 'Capitão Goblin',
            effect: isMaster
              ? {
                  outcome: NOT_AFFECTED,
                  reason: SpellEffectReason.ABOVE_LIMIT,
                  hitPointsBefore: 120,
                }
              : { outcome: NOT_AFFECTED },
          },
        ],
      });
    expect(plain(logLine(above(true), '', master)!.text)).toContain(
      'não é afetado: 120 PV, acima do limite de 100',
    );
    expect(logLine(above(false), '', player)!.text).toBe(
      ' conjura Palavra de Poder Matar: o Capitão Goblin não foi afetado.',
    );
  });

  it('Cura Completa: the amount only where the server sends it', () => {
    const heal = (healed: number | undefined) =>
      cast('cura', 'Cura Completa', {
        effectKind: SpellEffectKind.FLAT_HEAL,
        targets: [{ targetId: 't', targetLabel: 'Toren', effect: { outcome: AFFECTED, healed } }],
      });
    expect(logLine(heal(70), '', master)!.text).toBe(
      ' conjura Cura Completa em Toren: recupera 70 PV',
    );
    // A player who is not the target (or who healed an NPC) gets no amount: "foi curado" is all there is.
    expect(logLine(heal(undefined), '', player)!.text).toBe(
      ' conjura Cura Completa: Toren é curado.',
    );
  });
});

describe('Ajuda and the Escudo Arcano in the log (PM-03a)', () => {
  const aid = (healed: number | undefined, gain: SpellEffectGain) =>
    create(CombatLogEntrySchema, {
      id: 'ajuda',
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: 'Pensantus',
      keyNamePt: 'Ajuda',
      spell: {
        effectKind: SpellEffectKind.MAX_HP,
        targets: [
          { targetId: 't', targetLabel: 'Toren', effect: { outcome: AFFECTED, healed, gain } },
        ],
      },
    });

  it('says a target at 0 woke up with the hit points, and never writes temporary hit points', () => {
    expect(logLine(aid(5, SpellEffectGain.CURRENT), '', master)!.text).toBe(
      ' conjura Ajuda em Toren: acorda com 5 PV',
    );
    expect(logLine(aid(5, SpellEffectGain.CURRENT), '', player)!.text).toBe(
      ' conjura Ajuda: Toren acordou com 5 PV (Ajuda).',
    );
    // A player who is not the target gets no amount.
    expect(logLine(aid(undefined, SpellEffectGain.CURRENT), '', player)!.text).toBe(
      ' conjura Ajuda: Toren acordou (Ajuda).',
    );
    expect(logLine(aid(5, SpellEffectGain.MAXIMUM), '', master)!.text).toBe(
      ' conjura Ajuda em Toren: ganha 5 PV máximos',
    );
  });

  const ended = (effect: CombatEffect, label: string, numbers = {}) =>
    create(CombatLogEntrySchema, {
      id: 'fim',
      kind: CombatLogKind.EFFECT_ENDED,
      actorLabel: label,
      effectEnd: { effect, ...numbers },
    });

  it('writes the end of the Escudo Arcano for everyone, with the name of its caster', () => {
    expect(logLine(ended(CombatEffect.SHIELD, 'Pensantus'), '', player)!.text).toBe(
      'O Escudo Arcano de Pensantus acabou',
    );
    expect(logLine(ended(CombatEffect.SHIELD, 'Mago 1'), '', master)!.text).toBe(
      'O Escudo Arcano do Mago 1 acabou',
    );
  });

  it('writes the end of Ajuda, with the hit points only for the master, who has the numbers', () => {
    const players = new Set(['Sálvia']);
    const master = { master: true, players };
    const player = { master: false, players };
    const numbers = {
      hitPointsBefore: 43,
      hitPointsAfter: 38,
      hitPointsMaxBefore: 43,
      hitPointsMaxAfter: 38,
    };
    expect(logLine(ended(CombatEffect.AID, 'Sálvia', numbers), '', master)!.text).toBe(
      'A Ajuda de Sálvia acabou: PV 43 → 38',
    );
    // The server sends zeros to everyone else: no numbers on the line.
    expect(logLine(ended(CombatEffect.AID, 'Sálvia'), '', player)!.text).toBe(
      'A Ajuda de Sálvia acabou',
    );
    expect(
      logLine(
        ended(CombatEffect.AID, 'Sálvia', { ...numbers, hitPointsBefore: 31, hitPointsAfter: 31 }),
        '',
        master,
      )!.text,
    ).toBe('A Ajuda de Sálvia acabou: PV 31, máximo 43 → 38');
  });
});
