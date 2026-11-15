import { type GoneView, type ReleaseBand, moveBand, moveNote, masterFoot } from './release-text';

/** The readied move a page releases (#856): the window, who holds it and what the master said would trigger it. */
export interface MoveRelease {
  readonly windowId: string;
  readonly readiedId: string;
  readonly reactorId: string;
  /** "Tor". */
  readonly reactor: string;
  readonly trigger: string;
  /** The master moves for the player: the band says so and the foot says the player may still answer. */
  readonly master: boolean;
}

/** What the Mover page shows of the release: the band, the note under the cost card, the master's footer and what became of the window. */
export interface MoveReleaseView {
  readonly band: ReleaseBand;
  readonly note: string;
  /** The master's footer; `''` for the player. */
  readonly foot: string;
  /** The window went away under the page: the move cannot be made and "Voltar à sessão" is the one button. */
  readonly gone: GoneView | null;
}

/** The words of the Mover page of a readied move (boards 857-06 and 857-07); `turnOf` is "vez do Ogro" ('' when nobody is on turn). */
export function moveReleaseView(
  r: MoveRelease,
  turnOf: string,
  gone: GoneView | null,
): MoveReleaseView {
  return {
    band: moveBand({ trigger: r.trigger, reactor: r.reactor, turnOf, master: r.master }),
    note: moveNote(r.reactor, r.master),
    foot: r.master ? masterFoot(r.reactor) : '',
    gone,
  };
}
