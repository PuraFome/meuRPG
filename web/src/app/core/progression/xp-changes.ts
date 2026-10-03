import { Injectable, signal } from '@angular/core';

/**
 * "The campaign's XP changed" (`xp_changed` on the session stream): the
 * session page bumps `version`, and every XP block on it (the end-of-combat
 * block, the "Dar XP" launcher) reads again when it moves. A counter in a
 * signal, not an event, so a block that renders later still sees there was
 * news and a burst of messages costs one read per block.
 */
@Injectable({ providedIn: 'root' })
export class XpChanges {
  readonly version = signal(0);

  bump(): void {
    this.version.update((v) => v + 1);
  }
}
