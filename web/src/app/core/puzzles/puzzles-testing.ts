import { type MessageInitShape, create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import {
  type MasterPuzzleRun,
  MasterPuzzleRunSchema,
  type Puzzle,
  PuzzleAlphabet,
  PuzzleKind,
  PuzzleRunSchema,
  PuzzleRunStatus,
  PuzzleSchema,
  PuzzleSymbolSchema,
  type PuzzleRun,
  type PuzzleSummary,
  PuzzleSummarySchema,
  type PreviewPuzzleStartResponse,
  PreviewPuzzleStartResponseSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { alphabetFaces, pillarFaces } from './puzzle-symbols';
import type { MoveAnswer, PuzzleInit, PuzzlesClient } from './puzzles-client';

/**
 * Builders and a stand-in for the puzzle specs (never imported by the app itself, so never bundled): the messages as the
 * server sends them, always built with `create` so a new proto field never leaves a spec with a hand-built object that is
 * missing it.
 */
export function symbolsOf(kind: 'lock' | 'pillars', count: number, alphabet = PuzzleAlphabet.RUNES): MessageInitShape<typeof PuzzleSymbolSchema>[] {
  const faces = kind === 'lock' ? alphabetFaces(alphabet).slice(0, 8) : pillarFaces(count);
  return faces.map((f) => ({ key: f.key, namePt: f.namePt }));
}

/** What a builder takes on top of its defaults: the message's own fields, as plain data (a nested message may be a plain object). */
type Init = Record<string, unknown>;

/** "Apagar as luzes", the artboard's 5 × 5 (E10-06). */
export function lightsPuzzle(id: string, name: string, partial: Init = {}): Puzzle {
  return create(PuzzleSchema, {
    id,
    campaignId: 'camp-1',
    kind: PuzzleKind.LIGHTS,
    name,
    config: { kind: { case: 'lights', value: { size: 5 } } },
    start: { kind: { case: 'lights', value: { lit: Array.from({ length: 25 }, (_, i) => i % 3 === 0) } } },
    minimum: { solvable: true, moves: 4, path: [] },
    ...(partial as object),
  });
}

export function lockPuzzle(id: string, name: string, partial: Init = {}): Puzzle {
  return create(PuzzleSchema, {
    id,
    campaignId: 'camp-1',
    kind: PuzzleKind.LOCK,
    name,
    config: { kind: { case: 'lock', value: { wheels: 4, alphabet: PuzzleAlphabet.RUNES } } },
    solution: { kind: { case: 'lock', value: { wheels: [1, 0, 3, 5] } } },
    start: { kind: { case: 'lock', value: { wheels: [0, 0, 2, 5] } } },
    symbols: symbolsOf('lock', 4),
    ...(partial as object),
  });
}

export function pillarsPuzzle(id: string, name: string, partial: Init = {}): Puzzle {
  return create(PuzzleSchema, {
    id,
    campaignId: 'camp-1',
    kind: PuzzleKind.PILLARS,
    name,
    config: {
      kind: {
        case: 'pillars',
        value: { pillars: 4, symbols: 4, links: [{ alsoTurns: [1] }, { alsoTurns: [0, 2] }, { alsoTurns: [1, 3] }, { alsoTurns: [2] }] },
      },
    },
    solution: { kind: { case: 'pillars', value: { pillars: [1, 0, 3, 2] } } },
    start: { kind: { case: 'pillars', value: { pillars: [2, 3, 2, 1] } } },
    minimum: { solvable: true, moves: 8, path: [] },
    symbols: symbolsOf('pillars', 4),
    ...(partial as object),
  });
}

/** What the players read of a lights puzzle. */
export function playerRun(puzzle: Puzzle, partial: Init = {}): PuzzleRun {
  return create(PuzzleRunSchema, {
    puzzleId: puzzle.id,
    name: puzzle.name,
    kind: puzzle.kind,
    config: puzzle.config,
    symbols: puzzle.symbols,
    state: puzzle.start,
    revision: 1,
    ...(partial as object),
  });
}

export function masterRun(puzzle: Puzzle, status: PuzzleRunStatus, partial: Init = {}): MasterPuzzleRun {
  const shown = status !== PuzzleRunStatus.NOT_SHOWN;
  return create(MasterPuzzleRunSchema, {
    puzzle,
    status,
    run: shown ? playerRun(puzzle) : undefined,
    start: puzzle.start,
    minimum: puzzle.minimum,
    minimumFromStart: puzzle.minimum,
    ...(partial as object),
  });
}

export function summary(puzzle: Puzzle, partial: Init = {}): PuzzleSummary {
  return create(PuzzleSummarySchema, { puzzleId: puzzle.id, name: puzzle.name, kind: puzzle.kind, ...(partial as object) });
}

export function preview(lit: readonly boolean[], moves: number, seed = 7n): PreviewPuzzleStartResponse {
  return create(PreviewPuzzleStartResponseSchema, {
    seed,
    start: { kind: { case: 'lights', value: { lit: [...lit] } } },
    minimum: { solvable: true, moves, path: [] },
  });
}

/** The time a spec's clock stands at. */
export const NOW = new Date(2026, 9, 6, 21, 12, 40);
export const at = (secondsAgo: number) => timestampFromDate(new Date(NOW.getTime() - secondsAgo * 1000));

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
  previewStart(campaignId: string, config: unknown, solution: unknown, seed: bigint): Promise<PreviewPuzzleStartResponse> {
    return this.answer('previewStart', [campaignId, config, solution, seed], this.previewResult as PreviewPuzzleStartResponse);
  }
  archive(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return this.answer('archive', [campaignId, puzzleId], create(PuzzleSchema, { id: puzzleId, archived: true }));
  }
  unarchive(campaignId: string, puzzleId: string): Promise<Puzzle> {
    return this.answer('unarchive', [campaignId, puzzleId], create(PuzzleSchema, { id: puzzleId }));
  }
  listSession(campaignId: string): Promise<MasterPuzzleRun[]> {
    return this.answer('listSession', [campaignId], this.sessionResult);
  }
  masterRun(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer('masterRun', [campaignId, puzzleId], this.runResults.get(puzzleId) as MasterPuzzleRun);
  }
  show(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer('show', [campaignId, puzzleId], this.runResults.get(puzzleId) as MasterPuzzleRun);
  }
  reset(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer('reset', [campaignId, puzzleId], this.runResults.get(puzzleId) as MasterPuzzleRun);
  }
  reseed(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer('reseed', [campaignId, puzzleId], this.runResults.get(puzzleId) as MasterPuzzleRun);
  }
  close(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer('close', [campaignId, puzzleId], this.runResults.get(puzzleId) as MasterPuzzleRun);
  }
  releaseHint(campaignId: string, puzzleId: string): Promise<MasterPuzzleRun> {
    return this.answer('releaseHint', [campaignId, puzzleId], this.runResults.get(puzzleId) as MasterPuzzleRun);
  }
  listShown(campaignId: string): Promise<PuzzleSummary[]> {
    return this.answer('listShown', [campaignId], this.shownResult);
  }
  run(campaignId: string, puzzleId: string): Promise<PuzzleRun> {
    return this.answer('run', [campaignId, puzzleId], this.playerRunResult as PuzzleRun);
  }
  async move(campaignId: string, puzzleId: string, move: unknown, key: string): Promise<MoveAnswer> {
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
