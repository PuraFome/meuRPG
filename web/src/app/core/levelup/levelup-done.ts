import { NavigationEnd, type Router } from '@angular/router';
import { filter, take } from 'rxjs';

import type { LevelUpDone } from './levelup-flow';

/**
 * What the level-up page left for the sheet in the navigation state ("Pensantus subiu para o nível 4."), read
 * once. The router keeps that state in `history.state`, which a reload gives back, so once the navigation is
 * over the key is taken out of the history entry: the status shows after the level-up and not again on a
 * reload. (The router writes its own state at the end of the navigation, so the clean-up waits for it.)
 */
export function takeLevelUpDone(
  router: Router,
  win: Pick<Window, 'history'> = window,
): LevelUpDone | null {
  const done =
    (router.currentNavigation()?.extras.state?.['levelUp'] as LevelUpDone | undefined) ?? null;
  if (done) {
    router.events
      .pipe(
        filter((e) => e instanceof NavigationEnd),
        take(1),
      )
      .subscribe(() => {
        setTimeout(() => {
          const { levelUp: _gone, ...rest } = (win.history.state ?? {}) as Record<string, unknown>;
          win.history.replaceState(rest, '');
        });
      });
  }
  return done;
}
