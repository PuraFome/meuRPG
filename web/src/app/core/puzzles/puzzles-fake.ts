import { type MessageInitShape, create } from '@bufbuild/protobuf';

import {
  type MasterPuzzleRun,
  type Puzzle,
  PuzzleSchema,
  type PuzzleRun,
  type PuzzleSummary,
  type PreviewPuzzleStartResponse,
  type TryPuzzleHintResponse,
  type CipherSolutionSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import type { HintDie, MoveAnswer, PuzzleInit, PuzzlesClient } from './puzzles-client';

/** The stand-in for `PuzzlesClient` the puzzle specs build on (split from `puzzles-testing.ts`, which re-exports it). */

type Call = readonly [string, ...unknown[]];

/** A `PuzzlesClient` that remembers its calls and answers what a spec set. */
export class FakePuzzlesClient {
  calls: Call[] = [];
  listResult: Puzzle[] = [];
  getResult: Puzzle | undefined;
  createResult: Puzzle | undefined;
  previewResult: PreviewPuzzleStartResponse | undefined;
  sessionResult: MasterPuzzleRun[] = [];
  runResults = new Map<string, MasterPuzzleRun>();
  shownResult: PuzzleSummary[] = [];
  playerRunResult: PuzzleRun | undefined;
  moveResult: ((n: number) => MoveAnswer | Promise<MoveAnswer>) | undefined;
  hintResult: ((n: number) => TryPuzzleHintResponse | Promise<TryPuzzleHintResponse>) | undefined;
  cipherResult = 'R WHVRXUR HVWD VRE R DOWDU';
  hintCount = 0;
  hintKeys: string[] = [];
  failWith: unknown;
  moveCount = 0;
  moveKeys: string[] = [];

  private async answer<T>(name: string, args: unknown[], value: T): Promise<T> {
    this.calls.push([name, ...args]);
    if (this.failWith !== undefined) {
      const err = this.failWith;
      throw err;
    }
    return value;
  }

  list(campaignId: string, includeArchived = false): Promise<Puzzle[]> {
    return this.answer('list', [campaignId, includeArchived], this.listResult);
  }
  get(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return this.answer('get', [campaignId, puzzleId], this.getResult as Puzzle);
  }
  create(campaignId: string, init: PuzzleInit): Promise<Puzzle> {
    return this.answer('create', [campaignId, init], this.createResult as Puzzle);
  }
  update(campaignId: string, puzzleId: string, init: PuzzleInit): Promise<Puzzle> {
    return this.answer('update', [campaignId, puzzleId, init], this.getResult as Puzzle);
  }
  previewStart(
    campaignId: string,
    config: unknown,
    solution: unknown,
    seed: bigint,
  ): Promise<PreviewPuzzleStartResponse> {
    return this.answer(
      'previewStart',
      [campaignId, config, solution, seed],
      this.previewResult as PreviewPuzzleStartResponse,
    );
  }
  archive(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return this.answer(
      'archive',
      [campaignId, puzzleId],
      create(PuzzleSchema, { id: puzzleId, archived: true }),
    );
  }
  unarchive(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return this.answer('unarchive', [campaignId, puzzleId], create(PuzzleSchema, { id: puzzleId }));
  }
  listSession(campaignId: string): Promise<MasterPuzzleRun[]> {
    return this.answer('listSession', [campaignId], this.sessionResult);
  }
  masterRun(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer(
      'masterRun',
      [campaignId, puzzleId],
      this.runResults.get(puzzleId) as MasterPuzzleRun,
    );
  }
  show(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer(
      'show',
      [campaignId, puzzleId],
      this.runResults.get(puzzleId) as MasterPuzzleRun,
    );
  }
  reset(campaignId: string, puzzleId: string, expectedRevision = 0): Promise<MasterPuzzleRun> {
    return this.answer(
      'reset',
      [campaignId, puzzleId, expectedRevision],
      this.runResults.get(puzzleId) as MasterPuzzleRun,
    );
  }
  reseed(campaignId: string, puzzleId: string, expectedRevision = 0): Promise<MasterPuzzleRun> {
    return this.answer(
      'reseed',
      [campaignId, puzzleId, expectedRevision],
      this.runResults.get(puzzleId) as MasterPuzzleRun,
    );
  }
  close(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer(
      'close',
      [campaignId, puzzleId],
      this.runResults.get(puzzleId) as MasterPuzzleRun,
    );
  }
  releaseHint(
    campaignId: string,
    puzzleId: string,
    expectedRevision = 0,
  ): Promise<MasterPuzzleRun> {
    return this.answer(
      'releaseHint',
      [campaignId, puzzleId, expectedRevision],
      this.runResults.get(puzzleId) as MasterPuzzleRun,
    );
  }
  listShown(campaignId: string): Promise<PuzzleSummary[]> {
    return this.answer('listShown', [campaignId], this.shownResult);
  }
  run(campaignId: string, puzzleId: string): Promise<PuzzleRun> {
    return this.answer('run', [campaignId, puzzleId], this.playerRunResult as PuzzleRun);
  }
  async previewCipher(
    campaignId: string,
    solution: MessageInitShape<typeof CipherSolutionSchema>,
  ): Promise<string> {
    return this.answer('previewCipher', [campaignId, solution], this.cipherResult);
  }
  playSequence(
    campaignId: string,
    puzzleId: string,
    expectedRevision = 0,
  ): Promise<MasterPuzzleRun> {
    return this.answer(
      'playSequence',
      [campaignId, puzzleId, expectedRevision],
      this.runResults.get(puzzleId) as MasterPuzzleRun,
    );
  }
  async tryHint(
    campaignId: string,
    puzzleId: string,
    die: HintDie,
    key: string,
  ): Promise<TryPuzzleHintResponse> {
    this.calls.push(['tryHint', campaignId, puzzleId, die, key]);
    this.hintKeys.push(key);
    const n = ++this.hintCount;
    if (!this.hintResult) {
      throw new Error('no hintResult set');
    }
    return this.hintResult(n);
  }
  async move(
    campaignId: string,
    puzzleId: string,
    move: unknown,
    key: string,
  ): Promise<MoveAnswer> {
    this.calls.push(['move', campaignId, puzzleId, move, key]);
    this.moveKeys.push(key);
    const n = ++this.moveCount;
    if (!this.moveResult) {
      throw new Error('no moveResult set');
    }
    return this.moveResult(n);
  }
}

/** The fake, typed as the client it stands in for. */
export function asClient(fake: FakePuzzlesClient): PuzzlesClient {
  return fake as unknown as PuzzlesClient;
}
