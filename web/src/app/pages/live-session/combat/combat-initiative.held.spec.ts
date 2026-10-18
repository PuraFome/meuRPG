// The initiative whose d20 the server kept for a question (the halfling's Lucky about a 1, a Bardic Inspiration die).
import { create } from '@bufbuild/protobuf';

import {
  type Encounter,
  HeldRollKind,
  SubmitInitiativeResponseSchema,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { heldAnswer, heldDie } from '../../../core/held-rolls/held-roll-testing';
import { CombatInitiative, type CombatInitiativeContext } from './combat-initiative';

function build(submit: ReturnType<typeof vi.fn>, ownId = 'toren') {
  const shown: Encounter[] = [];
  const e = encounter({
    id: 'enc',
    combatants: [combatant({ id: 'toren', label: 'Toren', initiativeBonus: 2 })],
  });
  const set = encounter({
    id: 'enc',
    combatants: [
      combatant({ id: 'toren', label: 'Toren', initiativeBonus: 2, initiativeFace: 7, initiative: 13 }),
    ],
  });
  const ctx = {
    api: { turn: { submitInitiative: submit } },
    answerHeld: vi.fn(async () =>
      heldAnswer({
        result: { case: 'initiative', value: create(SubmitInitiativeResponseSchema, { encounter: set }) },
      }),
    ),
    animator: { play: vi.fn() },
    calls: {
      run: vi.fn(async (fn: (enc: Encounter) => Promise<Encounter>) => {
        shown.push(await fn(e));
        return true;
      }),
    },
    campaignId: () => 'c',
    encounter: () => shown.at(-1) ?? e,
    ownId: () => ownId,
  } as unknown as CombatInitiativeContext;
  return { ctx, initiative: new CombatInitiative(ctx), shown, e, set };
}

describe('an initiative kept for a question', () => {
  it('asks the player the question and plays nothing until it is answered', async () => {
    const offer = heldDie(7, 2, 8, { kind: HeldRollKind.INITIATIVE, combatantId: 'toren' });
    const submit = vi.fn();
    const { ctx, initiative, e } = build(submit);
    submit.mockResolvedValue({ encounter: e, held: offer });
    expect(await initiative.rollInApp('toren')).toBe(true);
    expect(initiative.held.offer()?.holdId).toBe(offer.holdId);
    expect(ctx.animator.play).not.toHaveBeenCalled();
  });

  it('settles the initiative with the answer: the combat after it and the d20 on the table', async () => {
    const submit = vi.fn();
    const { ctx, initiative, shown, e } = build(submit);
    submit.mockResolvedValue({
      encounter: e,
      held: heldDie(7, 2, 8, { kind: HeldRollKind.INITIATIVE, combatantId: 'toren' }),
    });
    await initiative.submitFace('toren', 7);
    await initiative.held.answer({ kind: 'die', use: false, roll: null });
    expect(shown.at(-1)?.combatants[0].initiative).toBe(13);
    expect(initiative.held.offer()).toBeNull();
    expect(ctx.animator.play).toHaveBeenCalledTimes(1);
  });

  it("does not ask the master the player's question when he rolled for the player", async () => {
    const submit = vi.fn();
    const { initiative, e } = build(submit, 'master');
    submit.mockResolvedValue({
      encounter: e,
      held: heldDie(7, 2, 8, { kind: HeldRollKind.INITIATIVE, combatantId: 'toren' }),
    });
    await initiative.submitFace('toren', 7);
    expect(initiative.held.offer()).toBeNull();
  });

  it('asks again from the combat after a reload, and stops when the combat no longer says it waits', () => {
    const { initiative } = build(vi.fn());
    const offer = heldDie(7, 2, 8, { kind: HeldRollKind.INITIATIVE, combatantId: 'toren' });
    initiative.resume(offer);
    expect(initiative.held.offer()?.holdId).toBe(offer.holdId);
    initiative.resume(undefined);
    expect(initiative.held.offer()).toBeNull();
  });
});
