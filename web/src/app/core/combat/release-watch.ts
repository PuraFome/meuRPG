import { DestroyRef, computed, effect, inject, signal, untracked } from '@angular/core';

import {
  CombatantState,
  type Encounter,
  type ReactionWindow,
  ReadiedStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { readiedSecondsLeft, waitBeganMs } from './readied-wait';
import { type GoneView, releaseGoneView } from './release-text';

/** Which readied spell or move the release is for. */
export interface ReleaseWatchIds {
  readonly windowId: string;
  readonly readiedId: string;
  readonly reactorId: string;
  /** The held spell's name; `''` for a move. */
  readonly spellName: string;
  readonly move: boolean;
}

/** How often the clock of the wait redraws, in ms. */
const TICK_MS = 1000;

/**
 * What a release (the cast sheet's map, the Mover page) reads of its READIED window while it is open (#857, #856): whether it is
 * still in the combat, the seconds left of the wait and, once it went away, what happened (the master undid the trigger, the reaction
 * was used, the time ran out). The map goes away with the window and the one button left is "Voltar à sessão". Build it in an
 * injection context: it keeps an effect and a one-second timer, both stopped with the screen.
 */
export class ReleaseWatch {
  private readonly now = signal(Date.now());
  private readonly seenAt = Date.now();
  private last: ReactionWindow | undefined;
  private readonly goneAt = signal<number | null>(null);
  private readonly frozen = signal<GoneView | null>(null);

  /** The window as the combat has it now. */
  readonly window = computed(() =>
    this.encounter()?.reactionWindows.find((w) => w.id === this.ids.windowId),
  );
  readonly gone = computed(() => !this.window());
  /** The seconds left before the master's screen passes the window: the 120 s once the player was on the map, else the 30 s. */
  readonly secondsLeft = computed(() =>
    readiedSecondsLeft(
      this.window() ?? this.last ?? ({} as ReactionWindow),
      this.seenAt,
      this.goneAt() ?? this.now(),
    ),
  );
  /** The player opened the map while the window was open: its wait is the 120 s, not the 30 s. */
  readonly waitedOnMap = signal(false);
  /** What happened to the window; `null` while it is open. It is said once, when the window is first seen gone. */
  readonly view = computed(() => this.frozen());

  constructor(
    private readonly encounter: () => Encounter | null,
    private readonly ids: ReleaseWatchIds,
  ) {
    const timer = setInterval(() => this.now.set(Date.now()), TICK_MS);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
    const first = this.window();
    if (first) {
      this.see(first);
    }
    effect(() => {
      const w = this.window();
      const e = this.encounter();
      untracked(() => (w ? this.see(w) : this.settle(e)));
    });
  }

  private see(w: ReactionWindow): void {
    this.last = w;
    this.waitedOnMap.set(waitBeganMs(w) !== null);
  }

  private settle(e: Encounter | null): void {
    if (this.goneAt() !== null || !e) {
      return;
    }
    const at = Date.now();
    this.goneAt.set(at);
    const who = e.combatants.find((c) => c.id === this.ids.reactorId);
    const held = who?.readied;
    this.frozen.set(
      releaseGoneView({
        spell: this.ids.spellName,
        move: this.ids.move,
        stillWaiting: held?.id === this.ids.readiedId && held.status === ReadiedStatus.WAITING,
        reactionUsed: who?.reactionUsed ?? false,
        down: who?.state === CombatantState.DOWN,
        timedOut: !!this.last && readiedSecondsLeft(this.last, this.seenAt, at) === 0,
        afterMap: this.waitedOnMap(),
      }),
    );
  }
}
