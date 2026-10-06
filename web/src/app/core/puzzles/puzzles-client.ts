import { Injectable, inject } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import {
  type CreatePuzzleRequestSchema,
  type MasterPuzzleRun,
  type Puzzle,
  type PuzzleMoveSchema,
  type PuzzleRun,
  PuzzleService,
  type PuzzleSummary,
  type PreviewPuzzleStartResponse,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What the master writes about a puzzle, as `CreatePuzzle` takes it (`UpdatePuzzle` takes the same, with the ID). */
export type PuzzleInit = Omit<MessageInitShape<typeof CreatePuzzleRequestSchema>, 'campaignId' | '$typeName'>;

/** What a move answers: the run as the player reads it now, and whether this move solved the puzzle. */
export interface MoveAnswer {
  readonly run: PuzzleRun;
  readonly replayed: boolean;
  readonly solvedByThisMove: boolean;
}

/**
 * Thin wrapper around the generated `PuzzleService` client (MR-038, RN-27), in the same shape as
 * `MapsClient`. Callers map errors to Portuguese (`puzzleErrorMessage`). `providedIn: 'root'`, and only lazy code
 * imports it, so the generated puzzle code stays out of the initial bundle.
 *
 * `move` takes the idempotency key from the caller: a retry sends the same key, and the server then answers with
 * the run as it is now without playing the move again (`MovesSender` makes the key and does the retry).
 */
@Injectable({ providedIn: 'root' })
export class PuzzlesClient {
  private readonly client = createClient(PuzzleService, inject(CONNECT_TRANSPORT));

  async list(campaignId: string, includeArchived = false): Promise<Puzzle[]> {
    return (await this.client.listPuzzles({ campaignId, includeArchived })).puzzles;
  }

  async get(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return need((await this.client.getPuzzle({ campaignId, puzzleId })).puzzle, 'GetPuzzle');
  }

  async create(campaignId: string, init: PuzzleInit): Promise<Puzzle> {
    return need((await this.client.createPuzzle({ ...init, campaignId })).puzzle, 'CreatePuzzle');
  }

  async update(campaignId: string, puzzleId: string, init: PuzzleInit): Promise<Puzzle> {
    return need((await this.client.updatePuzzle({ ...init, campaignId, puzzleId })).puzzle, 'UpdatePuzzle');
  }

  /** The start the form shows, and the fewest moves from it; `seed` 0 draws a new one. */
  previewStart(
    campaignId: string,
    config: PuzzleInit['config'],
    solution: PuzzleInit['solution'],
    seed: bigint,
  ): Promise<PreviewPuzzleStartResponse> {
    return this.client.previewPuzzleStart({ campaignId, config, solution, seed });
  }

  async archive(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return need((await this.client.archivePuzzle({ campaignId, puzzleId })).puzzle, 'ArchivePuzzle');
  }

  async unarchive(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return need((await this.client.unarchivePuzzle({ campaignId, puzzleId })).puzzle, 'UnarchivePuzzle');
  }

  // The master's session.

  async listSession(campaignId: string): Promise<MasterPuzzleRun[]> {
    return (await this.client.listSessionPuzzles({ campaignId })).puzzles;
  }

  async masterRun(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return need((await this.client.getMasterPuzzleRun({ campaignId, puzzleId })).run, 'GetMasterPuzzleRun');
  }

  async show(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return need((await this.client.showPuzzle({ campaignId, puzzleId })).run, 'ShowPuzzle');
  }

  async reset(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return need((await this.client.resetPuzzle({ campaignId, puzzleId })).run, 'ResetPuzzle');
  }

  async reseed(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return need((await this.client.reseedPuzzle({ campaignId, puzzleId })).run, 'ReseedPuzzle');
  }

  async close(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return need((await this.client.closePuzzle({ campaignId, puzzleId })).run, 'ClosePuzzle');
  }

  async releaseHint(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return need((await this.client.releaseNextPuzzleHint({ campaignId, puzzleId })).run, 'ReleaseNextPuzzleHint');
  }

  // The player's session (the master may read what a player reads).

  async listShown(campaignId: string): Promise<PuzzleSummary[]> {
    return (await this.client.listShownPuzzles({ campaignId })).puzzles;
  }

  async run(campaignId: string, puzzleId: string): Promise<PuzzleRun> {
    return need((await this.client.getPuzzleRun({ campaignId, puzzleId })).run, 'GetPuzzleRun');
  }

  async move(campaignId: string, puzzleId: string, move: MessageInitShape<typeof PuzzleMoveSchema>, idempotencyKey: string): Promise<MoveAnswer> {
    const res = await this.client.makePuzzleMove({ campaignId, puzzleId, move, idempotencyKey });
    return { run: need(res.run, 'MakePuzzleMove'), replayed: res.replayed, solvedByThisMove: res.solvedByThisMove };
  }
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without its payload`);
  }
  return value;
}
