import {
  type Encounter,
  ReactionKind,
  type ReactionWindow,
  ReactionWindowStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * The reaction window on screen (PM-04): what the server's `Encounter.reaction_windows` and `reaction_wait` say, in
 * words. The server decides who may react, to what and in which order, and writes what each person may read; the
 * browser only phrases the prompt and the result it was sent. Pure functions, tested without a DOM.
 */

// The prompts and the results live in the `reaction-*` files; the screens read them through this one.
export { REACTION_NAMES } from './reaction-view';
export type { PromptView, ResultView, SlotAfter } from './reaction-view';
export {
  concentrationMode,
  concentrationModeLine,
  dieSidesOf,
  promptView,
  rebukeRows,
  slotChoices,
  throwText,
} from './reaction-prompts';
export type { ConcentrationMode, RebukeRow } from './reaction-prompts';
export { concentrationResultView, resultView } from './reaction-results';

/** A window that still waits for an answer. */
export function isOpen(w: ReactionWindow): boolean {
  return (
    w.status !== ReactionWindowStatus.ANSWERED && w.status !== ReactionWindowStatus.CLOSED_BY_ITSELF
  );
}

/** The windows that are open, in the order the server answers them. */
export function openWindows(e: Encounter): readonly ReactionWindow[] {
  // The master's question about a hidden creature an area hit is a window of the server's, drawn by its own card; a contest
  // (W7-X) waits in a window of kind CONTEST that RespondContest and ResolveShove answer, never AnswerReaction: its sheets
  // read it with `contestWindows`, and the reaction cards and queues never see it.
  return e.reactionWindows.filter(
    (w) => isOpen(w) && w.kind !== ReactionKind.HIDDEN_REVEAL && w.kind !== ReactionKind.CONTEST,
  );
}

/** The contests that wait (windows of kind CONTEST that are open), as the caller may read them: the ones the caller answers
 * have `forYou`, and `contest.contestId` names the contest (read with `GetContestState`). */
export function contestWindows(e: Encounter): readonly ReactionWindow[] {
  return e.reactionWindows.filter((w) => isOpen(w) && w.kind === ReactionKind.CONTEST);
}

/** The contest id of a window of kind CONTEST, or `''` for any other. */
export function contestIdOf(w: ReactionWindow): string {
  return w.kind === ReactionKind.CONTEST && w.prompt.case === 'contest'
    ? w.prompt.value.contestId
    : '';
}

/** The window a player answers now: the first one that is theirs to answer. */
export function windowForPlayer(e: Encounter): ReactionWindow | undefined {
  return openWindows(e).find((w) => w.forYou);
}

/** The window whose sheet opens by itself for a player: the first they answer that has a prompt of its own (the
 * master's check and the aggressor's saving throw are the master's cards). */
export function sheetWindow(e: Encounter): ReactionWindow | undefined {
  return openWindows(e).find(
    (w) =>
      w.forYou &&
      w.prompt.case !== undefined &&
      w.prompt.case !== 'masterCheck' &&
      w.prompt.case !== 'hellishRebukeSave',
  );
}

/** What the combat waits for, as the caller may read it: the title (a live region) and the line under it. */
export function reactionWait(e: Encounter): { title: string; detail: string } | null {
  const wait = e.reactionWait;
  return wait && wait.titlePt ? { title: wait.titlePt, detail: wait.detailPt } : null;
}
