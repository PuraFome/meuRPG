import { NavigationEnd, type Router } from '@angular/router';
import { Subject } from 'rxjs';

import { takeLevelUpDone } from './levelup-done';

describe('takeLevelUpDone: the status after the level-up shows once', () => {
  function setup(state: Record<string, unknown> | undefined, history: Record<string, unknown>) {
    const events = new Subject<unknown>();
    const router = {
      currentNavigation: () => ({ extras: { state } }),
      events,
    } as unknown as Router;
    const replaceState = vi.fn();
    const win = { history: { state: history, replaceState } } as unknown as Pick<Window, 'history'>;
    return { router, events, win, replaceState };
  }

  it('reads what the level-up page left, and takes it out of the history entry once the navigation is over', () => {
    vi.useFakeTimers();
    const { router, events, win, replaceState } = setup(
      { levelUp: { name: 'Pensantus', level: 4 } },
      { levelUp: { name: 'Pensantus', level: 4 }, navigationId: 7 },
    );
    expect(takeLevelUpDone(router, win)).toEqual({ name: 'Pensantus', level: 4 });
    // Not yet: the router writes its own state at the end of the navigation.
    vi.runAllTimers();
    expect(replaceState).not.toHaveBeenCalled();
    events.next(new NavigationEnd(1, '/x', '/x'));
    vi.runAllTimers();
    // A reload now reads a history entry without the key: no status.
    expect(replaceState).toHaveBeenCalledWith({ navigationId: 7 }, '');
    vi.useRealTimers();
  });

  it('says nothing, and touches nothing, for a sheet opened any other way (or after a reload)', () => {
    const { router, events, win, replaceState } = setup(undefined, { navigationId: 3 });
    expect(takeLevelUpDone(router, win)).toBeNull();
    events.next(new NavigationEnd(1, '/x', '/x'));
    expect(replaceState).not.toHaveBeenCalled();
  });
});
