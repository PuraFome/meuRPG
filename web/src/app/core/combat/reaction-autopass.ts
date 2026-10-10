import { ReactionKind, type ReactionWindow } from '../../../gen/meurpg/play/v1/combat_pb';
import { isOpen } from './reactions';

/**
 * How long a player's optional reaction waits before it passes by itself.
 * Vinicius, 10/10: an unanswered optional reaction passes after 30 s.
 */
export const AUTO_PASS_SECONDS = 30;

const MS_PER_SECOND = 1000;

/** The reactions a player may leave unanswered: using one is a choice, so not answering is the same as "Deixar passar". */
const OPTIONAL_KINDS: ReadonlySet<ReactionKind> = new Set([
  ReactionKind.SHIELD,
  ReactionKind.UNCANNY_DODGE,
  ReactionKind.HELLISH_REBUKE,
  ReactionKind.COUNTERSPELL,
  ReactionKind.CUTTING_WORDS,
  ReactionKind.DEFLECT_MISSILES,
  ReactionKind.FEATHER_FALL,
]);

/**
 * Whether the window is an optional reaction: never a saving throw (concentration, an effect's, the Repreensão Infernal
 * second step), a contest, the master's check or an opportunity attack, which are answered with other calls or have
 * to be rolled.
 */
export function isOptionalReaction(w: ReactionWindow): boolean {
  return OPTIONAL_KINDS.has(w.kind) && !w.secondStep && isOpen(w);
}

/** A window the master's screen passes after `AUTO_PASS_SECONDS`: an optional reaction of a player whose turn to answer it is. */
export function autoPasses(w: ReactionWindow): boolean {
  return isOptionalReaction(w) && w.reactorIsPlayer && w.answerNow;
}

/** The idempotency key of the automatic pass of a window: two tabs, or a retry, never answer twice. */
export function autoPassKey(windowId: string): string {
  return `auto-pass:${windowId}`;
}

/**
 * The clock of the automatic pass: when this screen first saw each window waiting for its player's answer. In memory
 * only; a reload starts the 30 s again.
 */
export class AutoPassClock {
  private readonly seen = new Map<string, number>();
  private readonly sent = new Set<string>();

  /** Looks at the open windows at `now` (ms): forgets the gone ones, starts the clock of the new ones. */
  sync(windows: readonly ReactionWindow[], now: number): void {
    const waiting = new Set(windows.filter(autoPasses).map((w) => w.id));
    for (const id of [...this.seen.keys()]) {
      if (!waiting.has(id)) {
        this.seen.delete(id);
      }
    }
    for (const id of waiting) {
      if (!this.seen.has(id)) {
        this.seen.set(id, now);
      }
    }
  }

  /** The seconds left of a window (rounded up, never below 0), or `null` for one with no clock. */
  secondsLeft(windowId: string, now: number): number | null {
    const since = this.seen.get(windowId);
    if (since === undefined) {
      return null;
    }
    return Math.max(0, Math.ceil(AUTO_PASS_SECONDS - (now - since) / MS_PER_SECOND));
  }

  /** The windows whose 30 s are up and were not passed yet: each is returned once. */
  due(now: number): string[] {
    const ids = [...this.seen.keys()].filter(
      (id) => !this.sent.has(id) && this.secondsLeft(id, now) === 0,
    );
    for (const id of ids) {
      this.sent.add(id);
    }
    return ids;
  }

  /** A pass that failed for a reason that is not a refusal (the network): its window may be sent again. */
  release(windowId: string): void {
    this.sent.delete(windowId);
  }

  /** The least seconds left among the windows with a clock, or `null` when none. */
  soonest(now: number): number | null {
    const left = [...this.seen.keys()].map((id) => this.secondsLeft(id, now) ?? Infinity);
    return left.length === 0 ? null : Math.min(...left);
  }
}

/** "Passa sozinho em 24 s". */
export function autoPassText(seconds: number): string {
  return `Passa sozinho em ${seconds} s`;
}

/** What the player reads on their own prompt: "Se você não responder, passa sozinho em 24 s.". */
export function playerAutoPassText(seconds: number): string {
  return `Se você não responder, passa sozinho em ${seconds} s.`;
}

/**
 * The seconds left on the player's own prompt, counted from when it opened (`openedAt`, ms). The player's client never
 * answers by itself: the master's screen sends the pass.
 */
export function playerSecondsLeft(openedAt: number, now: number): number {
  return Math.max(0, Math.ceil(AUTO_PASS_SECONDS - (now - openedAt) / MS_PER_SECOND));
}
