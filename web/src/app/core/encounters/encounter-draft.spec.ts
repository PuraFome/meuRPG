import { Code, ConnectError } from '@connectrpc/connect';

import { BANDIT, FakeEncountersClient, GOBLIN, OGRE } from './encounters-testing';
import { EncounterDraft } from './encounter-draft';
import type { EncountersClient } from './encounters-client';

/** A promise to settle by hand, to hold an answer back. */
function gate(): { promise: Promise<void>; open: () => void } {
  let open!: () => void;
  const promise = new Promise<void>((resolve) => (open = resolve));
  return { promise, open };
}

describe('EncounterDraft: the browser does no maths, every change asks the server (MR-043, RN-29)', () => {
  let api: FakeEncountersClient;
  let draft: EncounterDraft;

  beforeEach(() => {
    vi.useFakeTimers();
    api = new FakeEncountersClient();
    draft = new EncounterDraft(api as unknown as EncountersClient, 'camp-1');
  });
  afterEach(() => {
    draft.stop();
    vi.useRealTimers();
  });

  it('measures after a pause, once for a run of taps', async () => {
    draft.add(OGRE);
    draft.setCount(OGRE.key, 2);
    draft.setCount(OGRE.key, 3);
    expect(api.evaluate).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(300);
    expect(api.evaluate).toHaveBeenCalledTimes(1);
    expect(api.evaluateCalls[0].entries).toEqual([{ creatureKey: OGRE.key, count: 3 }]);
    expect(draft.evaluation()?.totalXp).toBe(1350);
    expect(draft.state()).toBe('idle');
  });

  it('sends the party NPCs with the entries', async () => {
    draft.addNpc({ characterId: 'npc-1', name: '', level: 3, label: 'Orin, o guia' });
    await vi.advanceTimersByTimeAsync(300);
    expect(api.evaluateCalls[0].party).toEqual([{ characterId: 'npc-1', name: '', level: 3 }]);
    expect(draft.evaluation()?.budget?.moderate).toBe(2100);
  });

  it('asks one at a time: a change that comes while one is on its way asks again when it ends, with the newest draft', async () => {
    const held = gate();
    api.gates = [held.promise];
    draft.add(OGRE);
    await vi.advanceTimersByTimeAsync(300);
    expect(api.evaluate).toHaveBeenCalledTimes(1);
    draft.add(GOBLIN);
    await vi.advanceTimersByTimeAsync(300);
    // Still one: the second waits for the first.
    expect(api.evaluate).toHaveBeenCalledTimes(1);
    held.open();
    await vi.advanceTimersByTimeAsync(10);
    expect(api.evaluate).toHaveBeenCalledTimes(2);
    expect(api.evaluateCalls[1].entries.map((e) => e.creatureKey)).toEqual([OGRE.key, GOBLIN.key]);
    expect(draft.evaluation()?.totalXp).toBe(500);
  });

  it('drops an answer that is no longer for the draft on screen', async () => {
    const held = gate();
    api.gates = [held.promise];
    draft.add(OGRE);
    await vi.advanceTimersByTimeAsync(300);
    draft.setCount(OGRE.key, 0);
    held.open();
    await vi.advanceTimersByTimeAsync(400);
    // The answer for one Ogre never showed: the draft is empty and its own answer is 0 XP.
    expect(draft.evaluation()?.totalXp).toBe(0);
    expect(draft.entries()).toEqual([]);
  });

  it('takes a creature out at 0, adds one more of the same, and caps a count at 40 and the kinds at 20', async () => {
    draft.add(BANDIT);
    draft.add(BANDIT);
    expect(draft.entries()[0].count).toBe(2);
    draft.setCount(BANDIT.key, 99);
    expect(draft.entries()[0].count).toBe(40);
    draft.setCount(BANDIT.key, 0);
    expect(draft.entries()).toEqual([]);
    for (let i = 0; i < 25; i++) {
      draft.add({ ...OGRE, key: `monster:c${i}` });
    }
    expect(draft.entries()).toHaveLength(20);
  });

  it('says what failed in words and stays usable', async () => {
    api.evaluateFail = new ConnectError('boom', Code.Unavailable);
    draft.add(OGRE);
    await vi.advanceTimersByTimeAsync(300);
    expect(draft.state()).toBe('error');
    expect(draft.error()).toContain('o servidor não respondeu');
    api.evaluateFail = null;
    draft.measureNow();
    await vi.advanceTimersByTimeAsync(10);
    expect(draft.state()).toBe('idle');
    expect(draft.evaluation()?.totalXp).toBe(450);
  });

  it('keeps the seed of a generated encounter until the first edit', async () => {
    draft.replace([{ creature: OGRE, count: 1 }], 7731);
    expect(draft.seed()).toBe(7731);
    draft.setCount(OGRE.key, 2);
    expect(draft.seed()).toBeNull();
  });
});
