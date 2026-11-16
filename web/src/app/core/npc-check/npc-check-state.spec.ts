import { Code, ConnectError } from '@connectrpc/connect';

import { NpcCheckAskState } from '../../../gen/meurpg/play/v1/combat_pb';
import type { NpcCheckRead } from '../../../gen/meurpg/play/v1/scene_pb';
import { NpcCheckState, type NpcCheckApi } from './npc-check-state';
import { ask, npcCheckRead } from './npc-check-testing';

function setup(latest: () => Promise<NpcCheckRead | null> = () => Promise.resolve(null)) {
  const calls: string[] = [];
  const skipKeys: string[] = [];
  let skipped: () => Promise<NpcCheckRead> = () =>
    Promise.resolve(
      npcCheckRead({ settled: true, total: 17, asks: [ask('Brisa', NpcCheckAskState.CLOSED)] }),
    );
  const api: NpcCheckApi = {
    latest: (campaignId) => {
      calls.push(`latest ${campaignId}`);
      return latest();
    },
    skip: (_campaignId, holdId, key) => {
      calls.push(`skip ${holdId}`);
      skipKeys.push(key);
      return skipped();
    },
  };
  let n = 0;
  const state = new NpcCheckState(api, () => `key-${++n}`);
  return { state, calls, skipKeys, failSkip: () => (skipped = () => Promise.reject(new ConnectError('x', Code.Unavailable))) };
}

describe('NpcCheckState', () => {
  it('reads the check the server holds, so the card survives a reload', async () => {
    const { state } = setup(() => Promise.resolve(npcCheckRead({ settled: true, total: 17, asks: [] })));
    await state.load('c1');
    expect(state.card()?.title).toBe('Capitão bandido · Enganação');
    expect(state.card()?.settled).toBe(true);
  });

  it('shows nothing when the session has had no check or is not open', async () => {
    const { state } = setup();
    await state.load('c1');
    expect(state.card()).toBeNull();
  });

  it('keeps what it shows when a read fails', async () => {
    let fail = false;
    const { state } = setup(() => (fail ? Promise.reject(new ConnectError('x', Code.Unavailable)) : Promise.resolve(npcCheckRead())));
    await state.load('c1');
    fail = true;
    await state.load('c1');
    expect(state.card()).not.toBeNull();
  });

  it('hides a settled card on this device with the X, and shows the next check again', async () => {
    let next = npcCheckRead({ settled: true, total: 17, asks: [] });
    const { state } = setup(() => Promise.resolve(next));
    await state.load('c1');
    state.dismiss();
    expect(state.card()).toBeNull();
    // A reload of the same check reads it again, but the X is in memory only for this page's life.
    await state.load('c1');
    expect(state.card()).toBeNull();
    next = npcCheckRead({ holdId: 'hold-2' });
    await state.load('c1');
    expect(state.card()?.key).toBe('hold-2');
  });

  it('shows a roll the master just made even when an old card was hidden', async () => {
    const { state } = setup(() => Promise.resolve(npcCheckRead({ holdId: '', settled: true, asks: [] })));
    await state.load('c1');
    state.dismiss();
    state.apply(npcCheckRead({ holdId: '', settled: true, asks: [] }));
    expect(state.card()).not.toBeNull();
  });

  it('only hides a card that is settled', async () => {
    const { state } = setup(() => Promise.resolve(npcCheckRead()));
    await state.load('c1');
    state.dismiss();
    expect(state.card()).not.toBeNull();
  });

  it('ignores a read that an newer one overtook', async () => {
    const slow: { resolve: (r: NpcCheckRead | null) => void }[] = [];
    const { state } = setup(() => new Promise((resolve) => slow.push({ resolve })));
    const first = state.load('c1');
    const second = state.load('c1');
    slow[1].resolve(npcCheckRead({ holdId: 'new' }));
    await second;
    slow[0].resolve(npcCheckRead({ holdId: 'old' }));
    await first;
    expect(state.card()?.key).toBe('new');
  });

  it('goes on without waiting: sends the hold, takes the settled check and keeps one key for a retry', async () => {
    const { state, calls, skipKeys, failSkip } = setup(() => Promise.resolve(npcCheckRead()));
    await state.load('c1');
    failSkip();
    await state.skip('c1');
    expect(state.error()).toBe('Não deu para continuar sem esperar agora. Tente de novo.');
    expect(state.busy()).toBe(false);
    expect(state.card()?.settled).toBe(false);
    await state.skip('c1');
    expect(calls.filter((c) => c === 'skip hold-1')).toHaveLength(2);
    expect(skipKeys[0]).toBe(skipKeys[1]);
  });

  it('settles the card with what the server answers', async () => {
    const { state } = setup(() => Promise.resolve(npcCheckRead()));
    await state.load('c1');
    await state.skip('c1');
    expect(state.card()?.settled).toBe(true);
    expect(state.card()?.rows[0].tag).toBe('Sem resposta');
    expect(state.error()).toBe('');
  });
});
