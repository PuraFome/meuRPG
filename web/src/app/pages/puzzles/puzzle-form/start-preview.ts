import type { PuzzleState } from '../../../../gen/meurpg/play/v1/puzzles_pb';

/** The start the form shows for the lights and the pillars, as the server drew it (`PreviewPuzzleStart`). */
export interface StartPreview {
  readonly status: 'idle' | 'loading' | 'ready' | 'error';
  readonly start: PuzzleState | undefined;
  /** The fewest moves from the start; only meaningful when `solvable`. */
  readonly moves: number;
  readonly solvable: boolean;
  /** Why the start could not be drawn, in words. */
  readonly message: string;
}

export const NO_PREVIEW: StartPreview = {
  status: 'idle',
  start: undefined,
  moves: 0,
  solvable: true,
  message: '',
};
