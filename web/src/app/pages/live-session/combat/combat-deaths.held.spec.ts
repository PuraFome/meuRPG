// The death save whose d20 the server kept for a question (the halfling's Lucky about a 1, a Bardic Inspiration die).
import { create } from '@bufbuild/protobuf';

import {
  DeathSaveOutcome,
  DeathSaveSchema,
  type Encounter,
  HeldRollKind,
  RollDeathSaveResponseSchema,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { encounter } from '../../../core/combat/combat-testing';
import { heldAnswer, heldDie, heldLucky } from '../../../core/held-rolls/held-roll-testing';
import { CombatDeaths, type CombatDeathsContext } from './combat-deaths';

const SAVE = create(DeathSaveSchema, { outcome: DeathSaveOutcome.SUCCESS, successes: 1 });

function build(over: Partial<CombatDeathsContext['api']['combatants']> = {}) {
  const shown: Encounter[] = [];
  const e = encounter({ id: 'enc' });
  const answers: { holdId: string; answer: unknown }[] = [];
  const ctx = {
    api: { combatants: { rollDeathSave: vi.fn(), ...over } },
    animator: { play: vi.fn() },
    calls: {
      run: vi.fn(async (fn: (enc: Encounter) => Promise<Encounter>) => {
        shown.push(await fn(e));
        return true;
      }),
    },
    answerHeld: vi.fn(async (holdId: string, answer: unknown) => {
      answers.push({ holdId, answer });
      return heldAnswer({
        result: {
          case: 'deathSave',
          value: create(RollDeathSaveResponseSchema, { encounter: e, deathSave: SAVE }),
        },
      });
    }),
    campaignId: () => 'c',
    encounter: () => e,
    ownId: () => 'toren',
    ownLabel: () => 'Toren',
    note: vi.fn(),
  } as unknown as CombatDeathsContext;
  return { ctx, deaths: new CombatDeaths(ctx), shown, answers, e };
}

describe('a death save kept for a question', () => {
  it('asks the question and writes no sentence until it is answered', async () => {
    const { ctx, deaths, e } = build();
    const offer = heldDie(7, 0, 8, { kind: HeldRollKind.DEATH_SAVE, combatantId: 'toren' });
    vi.mocked(ctx.api.combatants.rollDeathSave).mockResolvedValue({
      encounter: e,
      save: undefined,
      held: offer,
    });
    await deaths.roll({ inApp: true });
    expect(deaths.held.offer()?.holdId).toBe(offer.holdId);
    expect(deaths.result()).toBe('');
    expect(ctx.animator.play).not.toHaveBeenCalled();
  });

  it('settles the save with the answer: the sentence, the dice and the combat after it', async () => {
    const { ctx, deaths, e, shown, answers } = build();
    vi.mocked(ctx.api.combatants.rollDeathSave).mockResolvedValue({
      encounter: e,
      save: undefined,
      held: heldLucky([1], 0, { kind: HeldRollKind.DEATH_SAVE, combatantId: 'toren' }),
    });
    await deaths.roll({ face: 1 });
    await deaths.held.answer({ kind: 'lucky', reroll: false, roll: null });
    expect(answers).toHaveLength(1);
    expect(deaths.result()).toContain('Sucesso');
    expect(shown.at(-1)?.id).toBe(e.id);
    expect(deaths.held.offer()).toBeNull();
  });

  it('asks again from the combat after a reload, and stops when the combat no longer says it waits', () => {
    const { deaths } = build();
    const offer = heldDie(7, 0, 8, { kind: HeldRollKind.DEATH_SAVE, combatantId: 'toren' });
    deaths.resume(offer);
    expect(deaths.held.offer()?.holdId).toBe(offer.holdId);
    deaths.resume(undefined);
    expect(deaths.held.offer()).toBeNull();
  });

  it('keeps the master out of the player question: his roll for an absent player only says it waits', async () => {
    const { ctx, deaths, e } = build();
    vi.mocked(ctx.api.combatants.rollDeathSave).mockResolvedValue({
      encounter: e,
      save: undefined,
      held: heldDie(7, 0, 8, { kind: HeldRollKind.DEATH_SAVE, combatantId: 'other' }),
    });
    await deaths.rollMaster({ id: 'other', die: { face: 7 } });
    expect(deaths.held.offer()).toBeNull();
  });
});
