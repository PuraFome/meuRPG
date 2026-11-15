import { describe, expect, it, vi } from 'vitest';

import { encounter } from './combat-testing';
import { ReleaseMark } from './release-mark';

function setup(over: { mark?: ReturnType<typeof vi.fn> } = {}) {
  const mark =
    over.mark ??
    vi.fn(async (_c: string, _e: string, _w: string, _clear?: boolean) =>
      encounter({ revision: 5 }),
    );
  const apply = vi.fn();
  const m = new ReleaseMark(
    { markReactionRolling: mark } as never,
    { campaignId: 'camp', encounterId: 'enc', windowId: 'w1' },
    apply,
  );
  return { m, mark, apply };
}

describe("the wait mark of a readied window's release (#857, #856)", () => {
  it('marks the window on entering the map, and keeps what the server answers', async () => {
    const { m, mark, apply } = setup();
    await m.enter();
    expect(mark).toHaveBeenCalledWith('camp', 'enc', 'w1', false);
    expect(apply).toHaveBeenCalledWith(expect.objectContaining({ revision: 5 }));
    expect(m.on()).toBe(true);
  });

  it('marks once, however many times the map is entered', async () => {
    const { m, mark } = setup();
    await m.enter();
    await m.enter();
    expect(mark).toHaveBeenCalledTimes(1);
  });

  it('clears it with the same call and clear: true when the player goes back', async () => {
    const { m, mark, apply } = setup();
    await m.enter();
    await m.leave();
    expect(mark).toHaveBeenLastCalledWith('camp', 'enc', 'w1', true);
    expect(mark).toHaveBeenCalledTimes(2);
    expect(apply).toHaveBeenCalledTimes(2);
    expect(m.on()).toBe(false);
  });

  it('does not clear a mark it never set', async () => {
    const { m, mark } = setup();
    await m.leave();
    expect(mark).not.toHaveBeenCalled();
  });

  it('marks again on a second entry after going back (the wait does not restart: the server keeps its start)', async () => {
    const { m, mark } = setup();
    await m.enter();
    await m.leave();
    await m.enter();
    expect(mark.mock.calls.map((c) => c[3])).toEqual([false, true, false]);
  });

  it('goes on to the map when the mark could not be told, and does not clear what was not set', async () => {
    const mark = vi.fn().mockRejectedValue(new Error('network'));
    const { m } = setup({ mark });
    await expect(m.enter()).resolves.toBeUndefined();
    expect(m.on()).toBe(false);
    await m.leave();
    expect(mark).toHaveBeenCalledTimes(1);
  });

  it('goes back even when the clear could not be told', async () => {
    const mark = vi
      .fn()
      .mockResolvedValueOnce(encounter({ revision: 5 }))
      .mockRejectedValueOnce(new Error('network'));
    const { m } = setup({ mark });
    await m.enter();
    await expect(m.leave()).resolves.toBeUndefined();
    expect(m.on()).toBe(false);
  });

  it('clears a mark that was still on its way when the player tapped "Voltar"', async () => {
    const { m, mark } = setup();
    const entering = m.enter();
    const leaving = m.leave();
    await Promise.all([entering, leaving]);
    expect(mark.mock.calls.map((c) => c[3])).toEqual([false, true]);
    expect(m.on()).toBe(false);
  });

  it('does not send a second mark while the first is on its way', async () => {
    const { m, mark } = setup();
    await Promise.all([m.enter(), m.enter()]);
    expect(mark).toHaveBeenCalledTimes(1);
  });

  it('leaves nothing to clear after the release went through: the window closed with it', async () => {
    const { m, mark } = setup();
    await m.enter();
    m.finish();
    await m.leave();
    expect(mark).toHaveBeenCalledTimes(1);
  });
});
