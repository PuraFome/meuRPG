import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';

import { LevelUpSchema } from '../../../gen/meurpg/characters/v1/characters_pb';
import { LevelUpClient } from './levelup-client';
import { FRESH_MS, LevelUpFeed } from './levelup-feed';

const at = (ms: number) => create(LevelUpSchema, { id: String(ms), characterId: 'p', createdAt: timestampFromMs(ms) });

describe('LevelUpFeed', () => {
  const list = vi.fn();

  function setup(items: ReturnType<typeof at>[]) {
    list.mockReset().mockResolvedValue({ levelUps: items, nextPageToken: '' });
    TestBed.configureTestingModule({ providers: [LevelUpFeed, { provide: LevelUpClient, useValue: { list } }] });
    return TestBed.inject(LevelUpFeed);
  }

  it('is fresh for a day after the player confirmed, and not after', async () => {
    const feed = setup([at(Date.now() - 1000), at(Date.now() - FRESH_MS - 60_000)]);
    await feed.load('camp-1');
    expect(feed.items()).toHaveLength(2);
    expect(feed.fresh()).toHaveLength(1);
    expect(feed.isFresh(feed.items()[0])).toBe(true);
    expect(feed.isFresh(feed.items()[1])).toBe(false);
  });

  it('finds the newest level-up of a character', async () => {
    const feed = setup([at(Date.now() - 1000)]);
    await feed.load('camp-1');
    expect(feed.latestOf('p')?.id).toBeDefined();
    expect(feed.latestOf('nobody')).toBeUndefined();
  });

  it('reads a campaign once, and again on refresh', async () => {
    const feed = setup([]);
    await feed.load('camp-1');
    await feed.load('camp-1');
    expect(list).toHaveBeenCalledTimes(1);
    await feed.refresh();
    expect(list).toHaveBeenCalledTimes(2);
  });

  it('brings the status line back when a new level-up arrives after it was dismissed', async () => {
    const feed = setup([at(Date.now() - 1000)]);
    await feed.load('camp-1');
    feed.dismiss();
    expect(feed.dismissed()).toBe(true);
    list.mockResolvedValue({ levelUps: [at(Date.now()), at(Date.now() - 1000)], nextPageToken: '' });
    await feed.refresh();
    expect(feed.dismissed()).toBe(false);
  });

  it('keeps working when the read fails', async () => {
    list.mockReset().mockRejectedValue(new Error('x'));
    TestBed.configureTestingModule({ providers: [LevelUpFeed, { provide: LevelUpClient, useValue: { list } }] });
    const feed = TestBed.inject(LevelUpFeed);
    await feed.load('camp-1');
    expect(feed.items()).toEqual([]);
    expect(feed.loaded()).toBe(true);
  });

  it('reads every page, so the newest is never lost to the page size', async () => {
    list.mockReset();
    list
      .mockResolvedValueOnce({ levelUps: [at(Date.now() - 1000), at(Date.now() - 2000)], nextPageToken: 'next' })
      .mockResolvedValueOnce({ levelUps: [at(Date.now() - 3000)], nextPageToken: '' });
    TestBed.configureTestingModule({ providers: [LevelUpFeed, { provide: LevelUpClient, useValue: { list } }] });
    const feed = TestBed.inject(LevelUpFeed);
    await feed.load('camp-1');
    expect(list).toHaveBeenCalledTimes(2);
    expect(list.mock.calls[1][2]).toBe('next');
    expect(feed.items()).toHaveLength(3);
  });

  it('brings the line back for a newer level-up even when the list is as long as before (the page is full)', async () => {
    const full = (newest: number) => Array.from({ length: 50 }, (_, i) => at(newest - i * 1000));
    list.mockReset().mockResolvedValue({ levelUps: full(Date.now() - 1000), nextPageToken: '' });
    TestBed.configureTestingModule({ providers: [LevelUpFeed, { provide: LevelUpClient, useValue: { list } }] });
    const feed = TestBed.inject(LevelUpFeed);
    await feed.load('camp-1');
    feed.dismiss();
    list.mockResolvedValue({ levelUps: full(Date.now()), nextPageToken: '' });
    await feed.refresh();
    expect(feed.items()).toHaveLength(50);
    expect(feed.dismissed()).toBe(false);
  });

  it('keeps the line dismissed when the newest level-up is the same', async () => {
    const same = [at(Date.now() - 1000)];
    list.mockReset().mockResolvedValue({ levelUps: same, nextPageToken: '' });
    TestBed.configureTestingModule({ providers: [LevelUpFeed, { provide: LevelUpClient, useValue: { list } }] });
    const feed = TestBed.inject(LevelUpFeed);
    await feed.load('camp-1');
    feed.dismiss();
    await feed.refresh();
    expect(feed.dismissed()).toBe(true);
  });
});
