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
  type TryPuzzleHintResponse,
  TryPuzzleHintResponseSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { alphabetFaces, bellFaces, pillarFaces } from './puzzle-symbols';

/**
 * Builders and a stand-in for the puzzle specs (never imported by the app itself, so never bundled): the messages as the
 * server sends them, always built with `create` so a new proto field never leaves a spec with a hand-built object that is
 * missing it.
 */
export function symbolsOf(
  kind: 'lock' | 'pillars',
  count: number,
  alphabet = PuzzleAlphabet.RUNES,
): MessageInitShape<typeof PuzzleSymbolSchema>[] {
  const faces =
    kind === 'lock' ? alphabetFaces(alphabet).slice(0, LOCK_SYMBOLS) : pillarFaces(count);
  return faces.map((f) => ({ key: f.key, namePt: f.namePt }));
}

const LOCK_SYMBOLS = 8;
const LIT_EVERY = 3;
const LOCK_WHEELS = 4;
const PILLAR_COUNT = 4;
const BELL_COUNT = 4;
const HINT_BONUS = 3;

/** The numbers of a fixture written as digits ("1035"), so a list of positions reads at a glance. */
function digits(text: string): number[] {
  return [...text].map(Number);
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
    start: {
      kind: {
        case: 'lights',
        value: { lit: Array.from({ length: 25 }, (_, i) => i % LIT_EVERY === 0) },
      },
    },
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
    solution: { kind: { case: 'lock', value: { wheels: digits('1035') } } },
    start: { kind: { case: 'lock', value: { wheels: digits('0025') } } },
    symbols: symbolsOf('lock', LOCK_WHEELS),
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
        value: {
          pillars: 4,
          symbols: 4,
          links: [
            { alsoTurns: digits('1') },
            { alsoTurns: digits('02') },
            { alsoTurns: digits('13') },
            { alsoTurns: digits('2') },
          ],
        },
      },
    },
    solution: { kind: { case: 'pillars', value: { pillars: digits('1032') } } },
    start: { kind: { case: 'pillars', value: { pillars: digits('2321') } } },
    minimum: { solvable: true, moves: 8, path: [] },
    symbols: symbolsOf('pillars', PILLAR_COUNT),
    ...(partial as object),
  });
}

/** The riddle of the artboard (E10-12): the master's text and the answers he accepts. */
export function riddlePuzzle(id: string, name: string, partial: Init = {}): Puzzle {
  return create(PuzzleSchema, {
    id,
    campaignId: 'camp-1',
    kind: PuzzleKind.RIDDLE,
    name,
    config: {
      kind: {
        case: 'riddle',
        value: { text: 'Moro embaixo de cada passo seu, mas nunca peso nada. O que sou?' },
      },
    },
    solution: { kind: { case: 'riddle', value: { answers: ['sombra', 'a sombra'] } } },
    start: { kind: { case: 'riddle', value: {} } },
    minimum: { solvable: true, moves: 1, path: [] },
    ...(partial as object),
  });
}

/** Four bells and six steps (E10-12). */
export function sequencePuzzle(id: string, name: string, partial: Init = {}): Puzzle {
  return create(PuzzleSchema, {
    id,
    campaignId: 'camp-1',
    kind: PuzzleKind.SEQUENCE,
    name,
    config: { kind: { case: 'sequence', value: { bells: 4, steps: 6 } } },
    solution: { kind: { case: 'sequence', value: { steps: digits('013021') } } },
    start: { kind: { case: 'sequence', value: { progress: 0 } } },
    symbols: bellFaces(BELL_COUNT).map((f) => ({ key: f.key, namePt: f.namePt })),
    minimum: { solvable: true, moves: 6, path: [] },
    ...(partial as object),
  });
}

/** "O tesouro está sob o altar", three letters on. */
export function cipherPuzzle(id: string, name: string, partial: Init = {}): Puzzle {
  return create(PuzzleSchema, {
    id,
    campaignId: 'camp-1',
    kind: PuzzleKind.CIPHER,
    name,
    config: {
      kind: { case: 'cipher', value: { ciphertext: 'R WHVRXUR HVWD VRE R DOWDU', keyClueId: '' } },
    },
    solution: {
      kind: {
        case: 'cipher',
        value: { message: 'O tesouro está sob o altar', method: { case: 'shift', value: 3 } },
      },
    },
    start: { kind: { case: 'cipher', value: {} } },
    minimum: { solvable: true, moves: 1, path: [] },
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

export function masterRun(
  puzzle: Puzzle,
  status: PuzzleRunStatus,
  partial: Init = {},
): MasterPuzzleRun {
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
  return create(PuzzleSummarySchema, {
    puzzleId: puzzle.id,
    name: puzzle.name,
    kind: puzzle.kind,
    ...(partial as object),
  });
}

/** A try for a hint: what the server answers. */
export function hintAnswer(
  run: PuzzleRun,
  passed: boolean,
  total = 17,
  partial: Init = {},
): TryPuzzleHintResponse {
  return create(TryPuzzleHintResponseSchema, {
    run,
    passed,
    roll: { diceCount: 1, diceSides: 20, faces: [total - HINT_BONUS], modifier: HINT_BONUS, total },
    ...(partial as object),
  });
}

export function preview(
  lit: readonly boolean[],
  moves: number,
  seed = 7n,
): PreviewPuzzleStartResponse {
  return create(PreviewPuzzleStartResponseSchema, {
    seed,
    start: { kind: { case: 'lights', value: { lit: [...lit] } } },
    minimum: { solvable: true, moves, path: [] },
  });
}

/** The time a spec's clock stands at. */
export const NOW = new Date('2026-10-06T21:12:40');
export const at = (secondsAgo: number) =>
  timestampFromDate(new Date(NOW.getTime() - secondsAgo * 1000));
/** A moment `seconds` from the real clock, for a page that counts down by it. */
export const fromNow = (seconds: number) =>
  timestampFromDate(new Date(Date.now() + seconds * 1000));

export { FakePuzzlesClient, asClient } from './puzzles-fake';

/** The 18 skills' names as the rules' content sends them (two of them are enough for a spec). */
export const SKILLS = [
  { key: 'skill:arcana', label: 'Arcanismo' },
  { key: 'skill:investigation', label: 'Investigação' },
];

/** A `SceneChecks` that answers at once, for the specs that render a field or a page that reads the skills' names. */
export const fakeChecks = { skills: async () => SKILLS };

/** A `RosterClient` with Toren, Brisa and Sálvia, the artboard's party (an NPC too: a part is only for a player's character). */
export const fakeRoster = {
  list: async () => [
    {
      id: 'c-toren',
      name: 'Toren',
      kind: 1,
      playerUserId: 'u1',
      classSummary: '',
      raceName: '',
      playerName: 'Ana',
    },
    {
      id: 'c-brisa',
      name: 'Brisa',
      kind: 1,
      playerUserId: 'u2',
      classSummary: '',
      raceName: '',
      playerName: 'Caio',
    },
    {
      id: 'c-salvia',
      name: 'Sálvia',
      kind: 1,
      playerUserId: 'u3',
      classSummary: '',
      raceName: '',
      playerName: 'Lia',
    },
    {
      id: 'c-goblin',
      name: 'Goblin',
      kind: 4,
      playerUserId: '',
      classSummary: '',
      raceName: '',
      playerName: null,
    },
  ],
};
