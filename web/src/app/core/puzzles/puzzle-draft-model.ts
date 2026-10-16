import { PuzzleAlphabet } from '../../../gen/meurpg/play/v1/puzzles_pb';
import { alphabetFaces } from './puzzle-symbols';

/** The six kinds the forms make. */
export type FormKind = 'lights' | 'lock' | 'pillars' | 'riddle' | 'sequence' | 'cipher';

export const FORM_KINDS: readonly FormKind[] = [
  'lights',
  'lock',
  'pillars',
  'riddle',
  'sequence',
  'cipher',
];

/** The kinds that judge a move (a typed answer, a bell): only they have a wrong move to fire a trap or spend an attempt on. */
export const JUDGED_KINDS: readonly FormKind[] = ['riddle', 'sequence', 'cipher'];

// What the server checks, as numbers (puzzles.proto).
export const RIDDLE_MAX = 500;
export const ANSWERS_MAX = 10;
export const ANSWER_MAX = 80;
export const CIPHER_MESSAGE_MAX = 300;
export const KEYWORD_MIN = 3;
export const KEYWORD_MAX = 26;
export const SHIFT_MIN = 1;
export const SHIFT_MAX = 25;
export const BELLS_MIN = 3;
export const BELLS_MAX = 8;
export const STEPS_MIN = 3;
export const STEPS_MAX = 12;
export const PARTS_MAX = 8;
export const PART_MAX = 300;
export const DC_MIN = 1;
export const DC_MAX = 30;
export const ATTEMPTS_MAX = 10;
export const MOVES_LIMIT_MAX = 200;
export const MINUTES_MAX = 240;
export const SECONDS_PER_MINUTE = 60;
/** What a new "Ao errar" by attempts starts at, and the shift a new cipher starts at. */
export const DEFAULT_ATTEMPTS = 3;
export const DEFAULT_SHIFT = 3;

/** "Ao errar": the form shows one option at a time. */
export type WrongOption = 'none' | 'trap' | 'attempts' | 'limits';

export interface WrongDraft {
  readonly option: WrongOption;
  /** The trap point: a map and a point of it. */
  readonly mapId: string;
  readonly pointId: string;
  /** Wrong moves each player may make in a round, 1 to 10. */
  readonly attempts: number;
  /** What the two limit fields say: whole numbers as typed, `''` for none. */
  readonly movesText: string;
  readonly minutesText: string;
  /** The seconds the puzzle had when it was loaded: kept when the minutes were not touched (an API-made limit may not be whole minutes). */
  readonly keptSeconds: number;
  /** The puzzle came with more than one rule; saving keeps only the one chosen. */
  readonly combined: boolean;
}

export const NO_WRONG: WrongDraft = {
  option: 'none',
  mapId: '',
  pointId: '',
  attempts: DEFAULT_ATTEMPTS,
  movesText: '',
  minutesText: '',
  keptSeconds: 0,
  combined: false,
};

/** The skill check that wins a hint: both a skill and a DC, or neither. */
export interface HintCheckDraft {
  /** "skill:investigation", or `''` for none. */
  readonly skillKey: string;
  readonly dcText: string;
}

export const NO_HINT_CHECK: HintCheckDraft = { skillKey: '', dcText: '' };

/** One part of the split information: a character (or `''` for "Sem dono") and what they read. */
export interface PartDraft {
  readonly characterId: string;
  readonly text: string;
  /** The server's flag on a master's read: the owner is not a living player character any more. */
  readonly ownerUnavailable: boolean;
}

export type CipherMethod = 'shift' | 'keyword';

export const NAME_MAX = 80;
export const CLUE_MAX = 500;
export const HINT_MAX = 300;
export const HINTS_LIMIT = 10;
export const MESSAGE_MAX = 200;

/** "Ao resolver": what happens when the players solve it, as the form writes it. */
export type SolveChoice = 'notify' | 'door' | 'point' | 'clue';

export interface SolveDraft {
  readonly choice: SolveChoice;
  /** The map of a door or a point. */
  readonly mapId: string;
  /** The square of a door, from 0. `-1` before one is chosen. */
  readonly col: number;
  readonly row: number;
  readonly pointId: string;
  readonly clueId: string;
  /** What the players read once it is solved; optional. */
  readonly message: string;
}

/** Everything the three forms write, in one value. Plain data: the form edits copies of it. */
export interface Draft {
  readonly kind: FormKind;
  readonly name: string;
  readonly clue: string;
  readonly hints: readonly string[];
  readonly solve: SolveDraft;
  /** Lights. */
  readonly size: number;
  /** Lock. */
  readonly wheels: number;
  readonly alphabet: PuzzleAlphabet;
  readonly lockSolution: readonly number[];
  readonly lockStart: readonly number[];
  /** Pillars. */
  readonly pillars: number;
  readonly symbols: number;
  readonly linked: boolean;
  readonly mural: readonly number[];
  /** Riddle: what the players read, and the answers the master accepts. */
  readonly riddleText: string;
  readonly answers: readonly string[];
  /** Sequence: the bells in play and the bell of each step, in order. */
  readonly bells: number;
  readonly steps: readonly number[];
  /** Cipher: the plain message, how it is swapped, and the scene clue the key lives in. */
  readonly cipherMessage: string;
  readonly cipherMethod: CipherMethod;
  readonly shift: number;
  readonly keyword: string;
  readonly keyClueId: string;
  /** Every kind: the skill check that wins a hint, the split information and "Ao errar". */
  readonly hintCheck: HintCheckDraft;
  readonly parts: readonly PartDraft[];
  readonly wrong: WrongDraft;
  /** The seed of the start the form shows (lights and pillars); `0n` lets the server draw one. */
  readonly seed: bigint;
}

export const NO_SOLVE: SolveDraft = {
  choice: 'notify',
  mapId: '',
  col: -1,
  row: -1,
  pointId: '',
  clueId: '',
  message: '',
};

/** A new puzzle's form, with the values the artboard starts from. */
export function newDraft(kind: FormKind): Draft {
  return {
    kind,
    name: '',
    clue: '',
    hints: [],
    solve: NO_SOLVE,
    size: 5,
    wheels: 4,
    alphabet: PuzzleAlphabet.RUNES,
    lockSolution: [0, 0, 0, 0],
    lockStart: [0, 0, 0, 1],
    pillars: 4,
    symbols: 4,
    linked: true,
    mural: [0, 0, 0, 0],
    riddleText: '',
    answers: [],
    bells: 4,
    steps: [],
    cipherMessage: '',
    cipherMethod: 'shift',
    shift: DEFAULT_SHIFT,
    keyword: '',
    keyClueId: '',
    hintCheck: NO_HINT_CHECK,
    parts: [],
    wrong: NO_WRONG,
    seed: 0n,
  };
}

/** `values` made exactly `length` long: cut, or padded with `fill`. */
export function resized(values: readonly number[], length: number, fill = 0): number[] {
  return Array.from({ length }, (_, i) => values[i] ?? fill);
}

/** Every value kept inside `0 … count - 1` (a wheel that was on "Z" and goes to digits lands on a digit). */
export function clamped(values: readonly number[], count: number): number[] {
  return values.map((v) => ((v % count) + count) % count);
}

/** The lock's faces count for an alphabet, to keep the positions inside it. */
export function alphabetSize(alphabet: PuzzleAlphabet): number {
  return alphabetFaces(alphabet).length;
}
