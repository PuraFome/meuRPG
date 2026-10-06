import { Code, ConnectError } from '@connectrpc/connect';

import { DungeonInfo } from './dungeon-info';
import { FakeDungeonsClient, roomsResponse } from './dungeons-testing';

describe('DungeonInfo', () => {
  it('holds the rooms of a generated dungeon', async () => {
    const api = new FakeDungeonsClient();
    const info = new DungeonInfo(api);
    await info.load('camp-1', 'dungeon-1');
    expect(info.rooms()?.rooms).toHaveLength(3);
    expect(api.calls).toEqual(['rooms dungeon-1']);
  });

  it('holds nothing for a map the generator did not make (not_found), and that is not a failure', async () => {
    const api = new FakeDungeonsClient();
    const info = new DungeonInfo(api);
    await info.load('camp-1', 'dungeon-1');
    api.roomsAnswer = null;
    await info.load('camp-1', 'plain-map');
    expect(info.rooms()).toBeNull();
    expect(info.failed()).toBe(false);
  });

  it('any other failure is a failure the page says (with "Tentar de novo"), and a good read clears it', async () => {
    const api = new FakeDungeonsClient();
    const info = new DungeonInfo(api);
    api.failWith.set('rooms', new ConnectError('down', Code.Unavailable));
    await info.load('camp-1', 'dungeon-1');
    expect(info.rooms()).toBeNull();
    expect(info.failed()).toBe(true);
    api.failWith.clear();
    await info.load('camp-1', 'dungeon-1');
    expect(info.failed()).toBe(false);
    expect(info.rooms()).not.toBeNull();
  });

  it('never lets a stale answer replace a newer read', async () => {
    let release!: () => void;
    const slow = new Promise<void>((r) => (release = r));
    const api = new FakeDungeonsClient();
    const first = roomsResponse({ seed: 1n });
    const second = roomsResponse({ seed: 2n });
    const answers = [first, second];
    const info = new DungeonInfo({
      rooms: async () => {
        const mine = answers.shift()!;
        if (mine === first) {
          await slow;
        }
        return mine;
      },
    });
    void api;
    const a = info.load('camp-1', 'dungeon-1');
    await info.load('camp-1', 'dungeon-1');
    release();
    await a;
    expect(info.rooms()?.seed).toBe(2n);
  });

  it('forgets the dungeon on clear', async () => {
    const info = new DungeonInfo(new FakeDungeonsClient());
    await info.load('camp-1', 'dungeon-1');
    info.clear();
    expect(info.rooms()).toBeNull();
  });
});
