import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  CombatLogEffectChange,
  CombatLogEffectSchema,
  type CombatLogEntry,
  CombatLogEntrySchema,
  AttackOutcome,
  PendingDamageStatus,
  CombatLogKind,
  HitRiderChoice,
  HitRiderLogResult,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { logLine } from './combat-log';

const ctx = { master: false, players: new Set(['Brisa']) };
const clean = (s?: string) => s?.replace(/\u00a0/g, ' ');

describe('the log lines of a spell hit effect', () => {
  const rider = (over: object, entry: object = {}): CombatLogEntry =>
    create(CombatLogEntrySchema, {
      id: 'r',
      kind: CombatLogKind.HIT_RIDER,
      actorLabel: 'Brisa',
      targetLabel: 'Zumbi 1',
      key: 'spell:branding-smite',
      hitRider: { choice: HitRiderChoice.APPLY, ...over } as never,
      ...entry,
    });

  it('says a failed save of the effect, with the target as the subject (not the monk map)', () => {
    const line = logLine(
      rider({ result: HitRiderLogResult.FAILED }, { keyNamePt: 'Clarão na Lâmina' }),
      '',
      ctx,
    )!;
    expect(line.actor).toBe('');
    expect(clean(line.text)).toBe(
      'O Zumbi 1 falhou no teste de resistência contra o Clarão na Lâmina',
    );
  });

  it('says a passed save, and keeps the numbers for whoever the server gave them to', () => {
    const line = logLine(
      rider(
        { result: HitRiderLogResult.SAVED, numbers: true, d20: 12, saveTotal: 15, dc: 14 },
        { keyNamePt: 'Clarão na Lâmina', targetLabel: 'Zumbi 2' },
      ),
      '',
      ctx,
    )!;
    expect(clean(line.text)).toMatch(
      /^O Zumbi 2 passou no teste de resistência contra o Clarão na Lâmina \(.*= 15 contra CD 14\)$/,
    );
  });

  it('does not invent a name when the entry has none', () => {
    const line = logLine(rider({ result: HitRiderLogResult.SAVED }), '', ctx)!;
    expect(clean(line.text)).toBe('O Zumbi 1 passou no teste de resistência contra o efeito');
  });

  it('names a player character without an article', () => {
    const line = logLine(
      rider({ result: HitRiderLogResult.FAILED }, { targetLabel: 'Brisa' }),
      '',
      ctx,
    )!;
    expect(clean(line.text)).toBe('Brisa falhou no teste de resistência contra o efeito');
  });

  const damageEntry = (parts: object[]): CombatLogEntry =>
    create(CombatLogEntrySchema, {
      id: 'a',
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Brisa',
      targetLabel: 'Zumbi 1',
      keyNamePt: 'Rapieira',
      outcome: AttackOutcome.HIT,
      damage: { amount: 13, status: PendingDamageStatus.APPLIED, parts } as never,
    });
  const part = (partKey: string, labelPt: string, counted = true) => ({
    partKey,
    labelPt,
    counted,
    diceCount: 1,
    diceSides: 8,
    faces: [4],
    sum: 4,
  });

  it('adds "(<efeito> incluído)" after the total when the hit carried the spell part', () => {
    const text = clean(
      logLine(
        damageEntry([part('weapon', 'Rapieira'), part('spell:branding-smite', 'Clarão na Lâmina')]),
        '',
        ctx,
      )?.text,
    );
    expect(text).toContain('13 de dano (Clarão na Lâmina incluído)');
  });

  it('says nothing for a hit with no spell part, or one that did not count', () => {
    for (const parts of [
      [part('weapon', 'Rapieira'), part('sneak-attack', 'Ataque Furtivo')],
      [part('weapon', 'Rapieira'), part('spell:x', 'Clarão', false)],
      [],
    ]) {
      expect(logLine(damageEntry(parts), '', ctx)?.text).not.toContain('incluído');
    }
  });
});

describe('the ended line of a hit effect', () => {
  const ended = (fx: MessageInitShape<typeof CombatLogEffectSchema>): CombatLogEntry =>
    create(CombatLogEntrySchema, {
      id: 'e',
      kind: CombatLogKind.EFFECT,
      effect: create(CombatLogEffectSchema, {
        sourceKey: 'spell:branding-smite',
        sourceNamePt: 'Clarão na Lâmina',
        targetLabels: ['Brisa'],
        change: CombatLogEffectChange.ENDED,
        reason: 'concentration',
        ...fx,
      }),
    });
  const text = (e: CombatLogEntry) => clean(logLine(e, '', ctx)?.text);

  it('says it ended with no hit when the server says it ended unused', () => {
    expect(text(ended({ unused: true }))).toBe(
      'O Clarão na Lâmina de Brisa acabou sem golpe: a concentração acabou',
    );
  });

  it('keeps the plain wording when a hit spent it or it is no on-hit effect', () => {
    expect(text(ended({ unused: false }))).toBe(
      'Clarão na Lâmina acabou em Brisa: a concentração acabou',
    );
  });

  it('says why an unused effect ended for another reason', () => {
    expect(text(ended({ unused: true, reason: 'duration' }))).toBe(
      'O Clarão na Lâmina de Brisa acabou sem golpe: o tempo da magia passou',
    );
  });
});
