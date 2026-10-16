import type { MessageInitShape } from '@bufbuild/protobuf';

import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import type { AdvantageSource, RollMode } from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import type {
  PuzzleMoveSchema,
  PuzzleRun,
  TryPuzzleHintResponse,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import type { InspirationRoll, LuckyRoll } from '../resources/resources-client';
import type { HintDie, MoveAnswer } from './puzzles-client';

/** What the player's page needs from the client; `PuzzlesClient` is one. */
export interface PlayApi {
  run(campaignId: string, puzzleId: string): Promise<PuzzleRun>;
  move(
    campaignId: string,
    puzzleId: string,
    move: MessageInitShape<typeof PuzzleMoveSchema>,
    idempotencyKey: string,
  ): Promise<MoveAnswer>;
  tryHint(
    campaignId: string,
    puzzleId: string,
    die: HintDie,
    idempotencyKey: string,
    extraDieFaces?: readonly number[],
  ): Promise<TryPuzzleHintResponse>;
  /** The answer about the Bardic Inspiration die a try waits on; absent in a client that has no such question. */
  answerHintDie?(
    campaignId: string,
    holdId: string,
    use: boolean,
    die: InspirationRoll | null,
    idempotencyKey: string,
  ): Promise<TryPuzzleHintResponse>;
  /** The answer to the Lucky question a try waits on (a halfling's d20 of 1); absent in a client that has no such question. */
  answerHintLucky?(
    campaignId: string,
    holdId: string,
    reroll: boolean,
    die: LuckyRoll | null,
    idempotencyKey: string,
  ): Promise<TryPuzzleHintResponse>;
}

/** What a move came to, for the page that sent it: a typed answer or a deciphered message is judged, not applied. */
export interface MoveVerdict {
  /** The move reached the server and was judged. */
  readonly sent: boolean;
  /** The judged move was wrong (the player's own: a wrong answer, a wrong bell). */
  readonly wrong: boolean;
  readonly solved: boolean;
}

/** The player's own try for a hint by a skill check, as the page says it ("Você conseguiu." / "Não deu desta vez."). */
export interface HintTry {
  readonly passed: boolean;
  /** The player's own roll: the d20 and the total with the bonus. Never the DC. */
  readonly roll: DiceRoll | undefined;
  /** The mode the d20 rolled with and the circumstances behind it. */
  readonly mode: RollMode;
  readonly sources: readonly AdvantageSource[];
}

/** Calls `fn` after `ms` and returns what cancels it: the sequence's reveal reads the run again at each step. */
export type Schedule = (ms: number, fn: () => void) => () => void;

/** A move that never reached the server, or was not judged: what a dropped, queued or stale move comes to. */
export function notSent(): MoveVerdict {
  return { sent: false, wrong: false, solved: false };
}
