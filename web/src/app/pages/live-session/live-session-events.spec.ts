import { LiveSessionEvents } from './live-session-events';

/** The handlers of the page's stream, with the rest of the page faked: only what a re-read touches. */
function setup(readSnapshot: () => Promise<boolean>, openCombat: boolean) {
  const touchLog = vi.fn();
  const calls: string[] = [];
  const services = {
    xpChanges: { bump: () => calls.push('xp') },
    spellCatalog: { forget: () => calls.push('catalog') },
    favor: { follow: () => calls.push('favor'), refresh: () => calls.push('favor-refresh') },
  };
  const parts = {
    scene: { refresh: () => undefined },
    trapBoard: { refresh: () => calls.push('traps'), refreshDamages: () => undefined },
    conversation: { sync: () => calls.push('conversation') },
    combat: { touchLog, encounter: () => (openCombat ? { id: 'e1', revision: 3 } : null) },
    mapState: { refresh: () => undefined },
  };
  const ticks = { held: 0, recheck: 0 };
  const state = {
    heldRollsTick: { update: () => (ticks.held += 100) },
    heldRollsRecheck: { update: (f: (n: number) => number) => (ticks.recheck = f(ticks.recheck)) },
    currentMapId: () => 'm1',
    isMaster: () => false,
    vitals: Object.assign(() => [], { update: () => undefined }),
  };
  const loader = {
    readSnapshot: vi.fn(readSnapshot),
    scheduleVision: vi.fn(),
    loadCombat: vi.fn(),
  };
  const view = { inTheatre: () => false };
  const events = new LiveSessionEvents(
    services as never,
    state as never,
    parts as never,
    view as never,
    loader as never,
  );
  const priv = events as unknown as {
    sessionHandlers(
      c: string,
      g: number,
    ): {
      onBehind(): Promise<boolean>;
      onReady(): void;
      onVitals(v: unknown): void;
      onHeldRollsChanged(): void;
    };
    mapHandlers(g: number): {
      onVisionChanged(mapId: string): void;
      onMapChanged(mapId: string): void;
    };
    tableHandlers(): { onSceneChanged(): void; onStageChanged(): void };
    combatHandlers(g: number): {
      onEncounterChanged(c: { encounterId: string; revision: number }): void;
    };
  };
  return { priv, loader, touchLog, calls, ticks };
}

describe('LiveSessionEvents: a heartbeat ahead of the stream', () => {
  it('reads what a `ready` reads and answers with whether the snapshot worked', async () => {
    for (const worked of [true, false]) {
      const { priv, loader, calls } = setup(() => Promise.resolve(worked), true);
      expect(await priv.sessionHandlers('c1', 1).onBehind()).toBe(worked);
      expect(loader.readSnapshot).toHaveBeenCalledTimes(1);
      expect(calls).toEqual(['xp', 'catalog', 'traps', 'favor', 'conversation']);
    }
  });
});

describe('LiveSessionEvents: reading the whole combat log', () => {
  it('asks for it on a vision hint and a hint with no revision while a combat is open', () => {
    const { priv, touchLog } = setup(() => Promise.resolve(true), true);
    priv.mapHandlers(1).onVisionChanged('m1');
    priv.combatHandlers(1).onEncounterChanged({ encounterId: 'e1', revision: 0 });
    expect(touchLog).toHaveBeenCalledTimes(2);
    expect(touchLog).toHaveBeenCalledWith(true);
  });

  it('leaves the flag alone when no combat is open', () => {
    const { priv, touchLog } = setup(() => Promise.resolve(true), false);
    priv.mapHandlers(1).onVisionChanged('m1');
    priv.combatHandlers(1).onEncounterChanged({ encounterId: 'e1', revision: 0 });
    expect(touchLog).not.toHaveBeenCalled();
  });
});

describe('LiveSessionEvents: the questions of the held rolls', () => {
  it('moves the recheck, never the held-rolls tick, when a map, a vision, a scene or a stage changed', () => {
    const { priv, ticks } = setup(() => Promise.resolve(true), false);
    priv.mapHandlers(1).onMapChanged('m2');
    priv.mapHandlers(1).onVisionChanged('m2');
    priv.tableHandlers().onSceneChanged();
    priv.tableHandlers().onStageChanged();
    expect(ticks.recheck).toBe(4);
    expect(ticks.held).toBe(0);
  });

  it('moves the held-rolls tick for the hint that says the questions changed', () => {
    const { priv, ticks } = setup(() => Promise.resolve(true), false);
    priv.sessionHandlers('c1', 1).onHeldRollsChanged();
    expect(ticks.held).toBe(100);
    expect(ticks.recheck).toBe(0);
  });
});

describe("LiveSessionEvents: the master's Inspiração (RN-43)", () => {
  it('follows the session on a ready, and again after a lost hint', async () => {
    const { priv, calls } = setup(() => Promise.resolve(true), false);
    priv.sessionHandlers('c1', 1).onReady();
    await priv.sessionHandlers('c1', 1).onBehind();
    expect(calls.filter((c) => c === 'favor')).toHaveLength(2);
  });

  it('reads who has it again on the vitals hint, the only one a grant tells', () => {
    const { priv, calls } = setup(() => Promise.resolve(true), false);
    priv.sessionHandlers('c1', 1).onVitals({ characterId: 'a' });
    expect(calls).toContain('favor-refresh');
  });
});
