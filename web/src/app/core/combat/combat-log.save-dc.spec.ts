// The DC of a save in the combat log: shown to whoever the server sent it to (the master always, the rolling player
// when the table shows it) and never made up when the field is absent or 0.
import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  CombatLogEffectChange,
  CombatLogDamageSchema,
  CombatLogEffectSchema,
  type CombatLogEntry,
  CombatLogEntrySchema,
  CombatLogKind,
  HitRiderChoice,
  HitRiderLogResult,
  PendingDamageStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { logLine } from './combat-log';

const PLAYER = { master: false, players: new Set(['Brisa', 'Toren']) };
const MASTER = { master: true, players: new Set(['Brisa', 'Toren']) };

const entry = (over: MessageInitShape<typeof CombatLogEntrySchema>): CombatLogEntry =>
  create(CombatLogEntrySchema, { id: 'e1', ...over } as never);
const line = (e: CombatLogEntry, ctx = PLAYER) => logLine(e, '', ctx)?.text.replace(/ /g, ' ');

describe('the DC of an effect save in the log', () => {
  const effect = (dc: number | undefined) =>
    entry({
      kind: CombatLogKind.EFFECT,
      effect: create(CombatLogEffectSchema, {
        change: CombatLogEffectChange.SAVE_FAILED,
        sourceNamePt: 'Teia',
        abilityNamePt: 'Destreza',
        targetLabels: ['Brisa'],
        dc,
      }),
    });

  it('shows "CD 14" to the rolling player when the server sent it', () => {
    expect(line(effect(14))).toBe('Brisa falhou no teste de Destreza contra Teia (CD 14)');
  });

  it('is the line it was when the DC is absent or 0', () => {
    expect(line(effect(undefined))).toBe('Brisa falhou no teste de Destreza contra Teia');
    expect(line(effect(0))).toBe('Brisa falhou no teste de Destreza contra Teia');
  });

  it('keeps the master with the dice and the DC together', () => {
    const withDice = entry({
      kind: CombatLogKind.EFFECT,
      effect: create(CombatLogEffectSchema, {
        change: CombatLogEffectChange.SAVE_FAILED,
        sourceNamePt: 'Teia',
        abilityNamePt: 'Destreza',
        targetLabels: ['Brisa'],
        d20: 12,
        total: 15,
        dc: 14,
      }),
    });
    expect(line(withDice, MASTER)).toContain('(d20 12, total 15 contra CD 14)');
  });
});

describe('the DC of a spell save in the log', () => {
  const cast = (dc: number) =>
    entry({
      kind: CombatLogKind.SPELL_CAST,
      actorLabel: 'Pensantus',
      keyNamePt: 'Mãos Flamejantes',
      spell: {
        slot: { level: 1, pact: false },
        targets: [{ targetId: 'b', targetLabel: 'Brisa', save: { outcome: 2, dc } }],
      },
    } as never);

  it('shows "CD 17" to the rolling player when the server sent it', () => {
    expect(line(cast(17))).toContain('falhou (CD 17)');
  });

  it('shows no DC when the server sent 0', () => {
    expect(line(cast(0))).not.toMatch(/CD/);
  });
});

describe('the DC of a monk rider save in the log', () => {
  const rider = (dc: number) =>
    entry({
      kind: CombatLogKind.HIT_RIDER,
      actorLabel: 'Toren',
      targetLabel: 'Brisa',
      hitRider: {
        choice: HitRiderChoice.PRONE,
        result: HitRiderLogResult.FAILED,
        numbers: true,
        d20: 4,
        saveTotal: 7,
        dc,
      } as never,
    });

  it('shows the DC beside the roll when the server sent it', () => {
    expect(line(rider(15))).toContain('falhou no teste (1d20 (4) + 3 = 7 contra CD 15)');
  });

  it('shows the roll alone, with no "CD 0", when the server sent none', () => {
    expect(line(rider(0))).toContain('falhou no teste (1d20 (4) + 3 = 7)');
    expect(line(rider(0))).not.toMatch(/CD/);
  });
});

describe('the DC of the concentration save a hit threatens, in the log', () => {
  const hit = (concentrationDc: number | undefined) =>
    entry({
      kind: CombatLogKind.ATTACK,
      actorLabel: 'Capitão Goblin',
      targetLabel: 'Pensantus',
      damage: create(CombatLogDamageSchema, {
        amount: 32,
        status: PendingDamageStatus.APPLIED,
        concentrationDc,
      }),
    });

  it('says "CD 16" when the server sent it', () => {
    expect(line(hit(16))).toContain('Teste de Constituição, CD 16, para manter a concentração');
  });

  it('says nothing of the save when the DC is absent', () => {
    expect(line(hit(undefined))).not.toMatch(/CD|concentração/);
  });
});
