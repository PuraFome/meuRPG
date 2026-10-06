import type { MessageInitShape } from '@bufbuild/protobuf';

import {
  type Puzzle,
  type PuzzleHintCheck,
  type PuzzleOnWrong,
  type PuzzlePart,
  PuzzleAlphabet,
  PuzzleKind,
  PuzzleSolveAction,
  type PuzzleOnSolveSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { alphabetFaces, pillarFaces } from './puzzle-symbols';
import type { PuzzleInit } from './puzzles-client';

/** The three kinds this slice makes. */
export type FormKind = 'lights' | 'lock' | 'pillars';

export const FORM_KINDS: readonly FormKind[] = ['lights', 'lock', 'pillars'];

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
  /**
   * What slice 10.7b's screens will edit: the hint's skill check, the split information and "Ao errar". This form has no field for
   * them, but `UpdatePuzzle` replaces the whole puzzle, so an edit carries them through untouched or it would erase them.
   */
  readonly hintCheck?: PuzzleHintCheck;
  readonly parts: readonly PuzzlePart[];
  readonly onWrong?: PuzzleOnWrong;
  /** The seed of the start the form shows (lights and pillars); `0n` lets the server draw one. */
  readonly seed: bigint;
}

export const NO_SOLVE: SolveDraft = { choice: 'notify', mapId: '', col: -1, row: -1, pointId: '', clueId: '', message: '' };

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
    parts: [],
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

/** The draft of an existing puzzle, for the edit page. */
export function draftOf(puzzle: Puzzle): Draft | null {
  const base = {
    ...newDraft('lights'),
    name: puzzle.name,
    clue: puzzle.clue,
    hints: [...puzzle.hints],
    solve: solveDraftOf(puzzle),
    hintCheck: puzzle.hintCheck,
    parts: puzzle.parts,
    onWrong: puzzle.onWrong,
  };
  const config = puzzle.config?.kind;
  switch (config?.case) {
    case 'lights':
      return { ...base, kind: 'lights', size: config.value.size };
    case 'lock': {
      const solution = puzzle.solution?.kind.case === 'lock' ? puzzle.solution.kind.value.wheels : [];
      const start = puzzle.start?.kind.case === 'lock' ? puzzle.start.kind.value.wheels : [];
      return {
        ...base,
        kind: 'lock',
        wheels: config.value.wheels,
        alphabet: config.value.alphabet,
        lockSolution: resized(solution, config.value.wheels),
        lockStart: resized(start, config.value.wheels),
      };
    }
    case 'pillars': {
      const mural = puzzle.solution?.kind.case === 'pillars' ? puzzle.solution.kind.value.pillars : [];
      return {
        ...base,
        kind: 'pillars',
        pillars: config.value.pillars,
        symbols: config.value.symbols,
        linked: config.value.links.some((l) => l.alsoTurns.length > 0),
        mural: resized(mural, config.value.pillars),
      };
    }
    default:
      return null;
  }
}

function solveDraftOf(puzzle: Puzzle): SolveDraft {
  const on = puzzle.onSolve;
  const message = on?.message ?? '';
  switch (on?.target.case) {
    case 'door':
      return { ...NO_SOLVE, choice: 'door', mapId: on.target.value.mapId, col: on.target.value.col, row: on.target.value.row, message };
    case 'point':
      return { ...NO_SOLVE, choice: 'point', mapId: on.target.value.mapId, pointId: on.target.value.pointId, message };
    case 'clue':
      return { ...NO_SOLVE, choice: 'clue', clueId: on.target.value.clueId, message };
    default:
      return { ...NO_SOLVE, message };
  }
}

/** The pillars' links when "Girar junto com os vizinhos" is on: each pillar turns the one on its left and the one on its right. */
export function neighbourLinks(count: number, linked: boolean): { alsoTurns: number[] }[] {
  return Array.from({ length: count }, (_, i) => ({
    alsoTurns: linked ? [i - 1, i + 1].filter((n) => n >= 0 && n < count) : [],
  }));
}

/** The `config` and `solution` the server needs to draw a start for the lights or the pillars. */
export function startRequestOf(draft: Draft): { config: PuzzleInit['config']; solution: PuzzleInit['solution'] } | null {
  if (draft.kind === 'lights') {
    return { config: { kind: { case: 'lights', value: { size: draft.size } } }, solution: undefined };
  }
  if (draft.kind === 'pillars') {
    return {
      config: { kind: { case: 'pillars', value: { pillars: draft.pillars, symbols: draft.symbols, links: neighbourLinks(draft.pillars, draft.linked) } } },
      solution: { kind: { case: 'pillars', value: { pillars: [...draft.mural] } } },
    };
  }
  return null;
}

/** "Ao resolver" as the request writes it. */
export function onSolveOf(solve: SolveDraft): MessageInitShape<typeof PuzzleOnSolveSchema> {
  const message = solve.message.trim();
  switch (solve.choice) {
    case 'door':
      return { action: PuzzleSolveAction.OPEN_DOOR, message, target: { case: 'door', value: { mapId: solve.mapId, col: solve.col, row: solve.row } } };
    case 'point':
      return { action: PuzzleSolveAction.REVEAL_POINT, message, target: { case: 'point', value: { mapId: solve.mapId, pointId: solve.pointId } } };
    case 'clue':
      return { action: PuzzleSolveAction.REVEAL_CLUE, message, target: { case: 'clue', value: { clueId: solve.clueId } } };
    default:
      return { action: PuzzleSolveAction.NOTIFY, message };
  }
}

/** The request body for `CreatePuzzle` and `UpdatePuzzle`. */
export function toInit(draft: Draft): PuzzleInit {
  const common = {
    name: draft.name.trim(),
    clue: draft.clue.trim(),
    hints: draft.hints.map((h) => h.trim()).filter((h) => h !== ''),
    onSolve: onSolveOf(draft.solve),
    seed: draft.seed,
    // Carried through as they were (see `Draft.hintCheck`).
    ...(draft.hintCheck ? { hintCheck: draft.hintCheck } : {}),
    parts: [...draft.parts],
    ...(draft.onWrong ? { onWrong: draft.onWrong } : {}),
  };
  switch (draft.kind) {
    case 'lights':
      return { ...common, config: { kind: { case: 'lights', value: { size: draft.size } } } };
    case 'lock':
      return {
        ...common,
        seed: 0n,
        config: { kind: { case: 'lock', value: { wheels: draft.wheels, alphabet: draft.alphabet } } },
        solution: { kind: { case: 'lock', value: { wheels: [...draft.lockSolution] } } },
        start: { kind: { case: 'lock', value: { wheels: [...draft.lockStart] } } },
      };
    case 'pillars': {
      const request = startRequestOf(draft);
      return { ...common, config: request?.config, solution: request?.solution };
    }
  }
}

/** Characters as the server counts them: code points, not UTF-16 units. */
export function textLength(text: string): number {
  return [...text].length;
}

/** What is wrong in a draft, by field, in words. Empty when it can be sent. The server stays the authority. */
export interface DraftErrors {
  name?: string;
  clue?: string;
  /** By hint's index. */
  hints: Record<number, string>;
  message?: string;
  target?: string;
  lock?: string;
}

export function draftErrors(draft: Draft): DraftErrors {
  const errors: DraftErrors = { hints: {} };
  const name = textLength(draft.name.trim());
  if (name === 0) {
    errors.name = 'Dê um nome ao quebra-cabeça.';
  } else if (name > NAME_MAX) {
    errors.name = `O nome passa de ${NAME_MAX} letras: tem ${name}.`;
  }
  if (textLength(draft.clue.trim()) > CLUE_MAX) {
    errors.clue = `A pista passa de ${CLUE_MAX} letras: tem ${textLength(draft.clue.trim())}.`;
  }
  draft.hints.forEach((hint, i) => {
    const n = textLength(hint.trim());
    if (n === 0) {
      errors.hints[i] = 'Escreva a dica, ou tire esta.';
    } else if (n > HINT_MAX) {
      errors.hints[i] = `A dica passa de ${HINT_MAX} letras: tem ${n}.`;
    }
  });
  if (textLength(draft.solve.message.trim()) > MESSAGE_MAX) {
    errors.message = `A mensagem passa de ${MESSAGE_MAX} letras: tem ${textLength(draft.solve.message.trim())}.`;
  }
  const solve = draft.solve;
  if (solve.choice === 'door' && (solve.mapId === '' || solve.col < 0 || solve.row < 0)) {
    errors.target = 'Escolha a porta que se abre.';
  } else if (solve.choice === 'point' && (solve.mapId === '' || solve.pointId === '')) {
    errors.target = 'Escolha o ponto que aparece no mapa.';
  } else if (solve.choice === 'clue' && solve.clueId === '') {
    errors.target = 'Escolha a pista que vai a quem resolver.';
  }
  if (draft.kind === 'lock' && draft.lockStart.every((v, i) => v === draft.lockSolution[i])) {
    errors.lock = 'O começo é igual à solução. Gire uma roda para o começo ser outro.';
  }
  return errors;
}

/** Whether a draft has nothing to fix. */
export function isValid(errors: DraftErrors): boolean {
  return !errors.name && !errors.clue && !errors.message && !errors.target && !errors.lock && Object.keys(errors.hints).length === 0;
}

/** The pillars' faces for a draft, to size the mural's choices. */
export function faceCount(draft: Draft): number {
  return draft.kind === 'pillars' ? pillarFaces(draft.symbols).length : alphabetSize(draft.alphabet);
}

/** The kind a form builds, as the proto says it. */
export function protoKindOf(kind: FormKind): PuzzleKind {
  return kind === 'lights' ? PuzzleKind.LIGHTS : kind === 'lock' ? PuzzleKind.LOCK : PuzzleKind.PILLARS;
}

/** The form kind of a proto kind, or `null` for the three kinds of slice 10.15b. */
export function formKindOf(kind: PuzzleKind): FormKind | null {
  return kind === PuzzleKind.LIGHTS ? 'lights' : kind === PuzzleKind.LOCK ? 'lock' : kind === PuzzleKind.PILLARS ? 'pillars' : null;
}
