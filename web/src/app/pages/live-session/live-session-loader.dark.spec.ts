import { vi } from 'vitest';

import { LiveSessionLoader } from './live-session-loader';
import type { LiveSessionParts, LiveSessionServices } from './live-session-deps';
import type { LiveSessionState } from './live-session-state';

describe('LiveSessionLoader: the master’s light warning follows what moves', () => {
  function loader() {
    const fog = { refresh: vi.fn(async () => undefined) };
    const dark = { refresh: vi.fn(async () => undefined) };
    const state = { visionTick: { update: vi.fn() } } as unknown as LiveSessionState;
    const parts = { fog, dark } as unknown as LiveSessionParts;
    return {
      fog,
      dark,
      subject: new LiveSessionLoader({} as LiveSessionServices, state, parts),
    };
  }

  afterEach(() => vi.useRealTimers());

  it('reads the dark characters again with the vision, once for a burst of hints (token_moved, a combat move, map_changed)', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
    const { fog, dark, subject } = loader();
    subject.scheduleVision();
    subject.scheduleVision();
    subject.scheduleVision();
    await vi.advanceTimersByTimeAsync(1000);
    expect(fog.refresh).toHaveBeenCalledTimes(1);
    expect(dark.refresh).toHaveBeenCalledTimes(1);
  });
});
