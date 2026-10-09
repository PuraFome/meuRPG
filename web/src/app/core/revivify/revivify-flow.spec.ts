import { create } from '@bufbuild/protobuf';

import {
  CombatLogEntrySchema,
  CombatLogRoundSchema,
  CombatLogSpellSchema,
  EncounterStatus,
  CombatLogSpellTargetSchema,
  SpellEffectResultSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import {
  RevivifyRequestSchema,
  RevivifyRequestStatus,
  RevivifyTargetSchema,
} from '../../../gen/meurpg/play/v1/revivify_pb';
import { combatant, encounter } from '../combat/combat-testing';
import {
  canCast,
  casterOfRevival,
  costLine,
  isRevivify,
  requestOutcome,
  resultLines,
  returnPlace,
  roundsAgo,
  sheetSubtitle,
  stepMarks,
  targetRows,
} from './revivify-flow';

const dots = (...parts: string[]) => parts.join('\u00a0· ');

describe('revivify flow', () => {
  it('knows the spell by its key', () => {
    expect(isRevivify('spell:revivify')).toBe(true);
    expect(isRevivify('spell:shield')).toBe(false);
  });

  it('marks the steps done, current and to come', () => {
    expect(stepMarks('target').map((m) => m.state)).toEqual(['current', 'todo', 'todo']);
    expect(stepMarks('confirm').map((m) => m.state)).toEqual(['done', 'current', 'todo']);
    expect(stepMarks('result').map((m) => `${m.n} ${m.label} ${m.state}`)).toEqual([
      '1 Alvo done',
      '2 Confirmar done',
      '3 Resultado current',
    ]);
  });

  it('says how long ago a creature died', () => {
    expect(roundsAgo(5)).toBe('há 5 rodadas');
    expect(roundsAgo(1)).toBe('há 1 rodada');
    expect(roundsAgo(0)).toBe('nesta rodada');
  });

  it('lists a combat target with its round and the tag, and an outside one with the master', () => {
    const [inCombat] = targetRows([
      create(RevivifyTargetSchema, {
        targetId: 't',
        name: 'Toren',
        deathRound: 3,
        roundsSinceDeath: 5,
      }),
    ]);
    expect(inCombat.detail).toBe(dots('Ao lado', 'morreu na rodada 3, há 5 rodadas'));
    expect(inCombat.tag).toBe('Pode ser revivido');
    expect(inCombat.waitsMaster).toBe(false);
    const [outside] = targetRows([
      create(RevivifyTargetSchema, { targetId: 't', name: 'Toren', needsMasterConfirmation: true }),
    ]);
    expect(outside.detail).toBe(dots('Ao lado', 'morreu fora de combate'));
    expect(outside.tag).toBe('');
    expect(outside.waitsMaster).toBe(true);
  });

  it('writes the header with the caster and the class', () => {
    expect(sheetSubtitle('Ilaria', 'Clérigo 5')).toBe(
      dots('3º\u00a0nível', '1 ação', 'toque', 'Ilaria, Clérigo 5'),
    );
    expect(sheetSubtitle('Ilaria', '')).toBe(dots('3º\u00a0nível', '1 ação', 'toque', 'Ilaria'));
  });

  it('says what the cast costs: the action only in a combat', () => {
    expect(costLine(3, 2, true)).toBe(
      'Gasta um espaço de 3º\u00a0nível (você tem 2) e a sua ação.',
    );
    expect(costLine(3, 2, false)).toBe('Gasta um espaço de 3º\u00a0nível (você tem 2).');
  });

  it('casts only with a target, a slot and the diamonds ticked', () => {
    expect(canCast('t', true, true)).toBe(true);
    expect(canCast('t', true, false)).toBe(false);
    expect(canCast('', true, true)).toBe(false);
    expect(canCast('t', false, true)).toBe(false);
  });

  it('writes the result with what was spent', () => {
    const l = resultLines('Toren', 3, 1, 2, true);
    expect(l.lead).toBe('Toren voltou à vida.');
    expect(l.rest).toBe('Está com 1 PV, acordado e sem testes contra a morte.');
    expect(l.spent).toBe('Você gastou um espaço de 3º\u00a0nível (restam 1 de 2) e a sua ação.');
    expect(l.diamonds).toBe('Diamantes de 300 PO gastos, como você confirmou.');
    expect(resultLines('Toren', 3, 0, 1, false).spent).toBe(
      'Você gastou um espaço de 3º\u00a0nível (restam 0 de 1).',
    );
  });

  it('reads where the request stands from the list', () => {
    const asked = (status: RevivifyRequestStatus) =>
      create(RevivifyRequestSchema, { id: 'r', status });
    expect(requestOutcome([asked(RevivifyRequestStatus.PENDING)], 'r').outcome).toBe('pending');
    expect(requestOutcome([asked(RevivifyRequestStatus.CONFIRMED)], 'r').outcome).toBe('confirmed');
    expect(requestOutcome([asked(RevivifyRequestStatus.DENIED)], 'r').outcome).toBe('denied');
    expect(requestOutcome([asked(RevivifyRequestStatus.DENIED)], 'other').outcome).toBe('gone');
  });
});

describe('who cast it and where the revived one returns', () => {
  const entry = (actor: string, characterId: string) =>
    create(CombatLogEntrySchema, {
      actorLabel: actor,
      spell: create(CombatLogSpellSchema, {
        targets: [
          create(CombatLogSpellTargetSchema, {
            effect: create(SpellEffectResultSchema, {
              revivedCharacterId: characterId,
            }),
          }),
        ],
      }),
    });

  it('reads the caster from the newest cast on that character', () => {
    const rounds = [
      create(CombatLogRoundSchema, { round: 4, entries: [entry('Brisa', 'toren')] }),
      create(CombatLogRoundSchema, {
        round: 8,
        entries: [entry('Ilaria', 'toren'), entry('Pensantus', 'other')],
      }),
    ];
    expect(casterOfRevival(rounds, 'toren')).toBe('Ilaria');
    expect(casterOfRevival(rounds, 'nobody')).toBe('');
  });

  const order = (currentCombatantId: string, round = 8) =>
    encounter({
      status: EncounterStatus.ACTIVE,
      round,
      currentCombatantId,
      combatants: [
        combatant({ id: 'b', label: 'Brisa' }),
        combatant({ id: 't', label: 'Toren', mine: true, characterId: 'toren' }),
        combatant({ id: 'p', label: 'Pensantus' }),
        combatant({ id: 'i', label: 'Ilaria' }),
      ],
    });

  it('acts on the next round when the turn on screen is after its place', () => {
    expect(returnPlace(order('i'), 'toren')).toEqual({
      between: 'entre Brisa e Pensantus',
      round: 9,
    });
  });

  it('acts this round when the turn on screen is before its place', () => {
    expect(returnPlace(order('b'), 'toren')?.round).toBe(8);
  });

  it('says nothing without a running combat, a place or a living combatant', () => {
    expect(returnPlace(null, 'toren')).toBeNull();
    expect(returnPlace(order('i'), 'other')).toBeNull();
    expect(returnPlace({ ...order('i'), status: EncounterStatus.ENDED }, 'toren')).toBeNull();
    const down = order('i');
    down.combatants[1] = { ...down.combatants[1], defeated: true };
    expect(returnPlace(down, 'toren')).toBeNull();
  });
});
