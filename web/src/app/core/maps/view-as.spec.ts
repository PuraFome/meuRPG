import { describe, expect, it, vi } from 'vitest';

import { ViewAsCounts } from './view-as';
import { visionResponse } from './vision-testing';

describe('ViewAsCounts', () => {
  it('counts the squares each character sees from the packed states, and the squares of the map', async () => {
    const load = vi.fn(async (_map: string, character: string) => visionResponse(character === 'toren' ? ['B..', '...'] : ['BdB', 'g.r']));
    const counts = new ViewAsCounts(load);
    await counts.read('m1', ['toren', 'pensantus']);
    expect(counts.counts().get('toren')).toBe(1);
    expect(counts.counts().get('pensantus')).toBe(4);
    expect(counts.total()).toBe(6);
  });

  it('marks a character whose read failed, and keeps the others', async () => {
    const counts = new ViewAsCounts(async (_m, c) => (c === 'brisa' ? Promise.reject(new Error('x')) : visionResponse(['BB'])));
    await counts.read('m1', ['brisa', 'toren']);
    expect(counts.counts().get('brisa')).toBeNull();
    expect(counts.counts().get('toren')).toBe(2);
  });

  it('never lets a stale count overwrite a newer one', async () => {
    let release: (r: ReturnType<typeof visionResponse>) => void = () => undefined;
    const slow = new Promise<ReturnType<typeof visionResponse>>((resolve) => (release = resolve));
    const load = vi.fn().mockReturnValueOnce(slow).mockResolvedValueOnce(visionResponse(['BBBB']));
    const counts = new ViewAsCounts(load);
    const first = counts.read('m1', ['toren']);
    await counts.read('m1', ['toren']);
    release(visionResponse(['B']));
    await first;
    expect(counts.counts().get('toren')).toBe(4);
  });
});
