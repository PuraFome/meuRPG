import type { MessageInitShape } from '@bufbuild/protobuf';

import {
  type Puzzle,
  PuzzleAlphabet,
  PuzzleKind,
  PuzzleSolveAction,
  type CipherSolutionSchema,
  type PuzzleOnSolveSchema,
  type PuzzleOnWrongSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { alphabetFaces } from './puzzle-symbols';
import { fold, hasCipherLetter, keywordLetters, keywordSwapsNothing } from './puzzle-text';
import type { PuzzleInit } from './puzzles-client';

/** The six kinds the forms make. */
export type FormKind = 'lights' | 'lock' | 'pillars' | 'riddle' | 'sequence' | 'cipher';

export const FORM_KINDS: readonly FormKind[] = ['lights', 'lock', 'pillars', 'riddle', 'sequence', 'cipher'];

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

export const NO_WRONG: WrongDraft = { option: 'none', mapId: '', pointId: '', attempts: 3, movesText: '', minutesText: '', keptSeconds: 0, combined: false };

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
    riddleText: '',
    answers: [],
    bells: 4,
    steps: [],
    cipherMessage: '',
    cipherMethod: 'shift',
    shift: 3,
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

/** The draft of an existing puzzle, for the edit page. */
export function draftOf(puzzle: Puzzle): Draft | null {
  const base = {
    ...newDraft('lights'),
    name: puzzle.name,
    clue: puzzle.clue,
    hints: [...puzzle.hints],
    solve: solveDraftOf(puzzle),
    hintCheck: puzzle.hintCheck ? { skillKey: puzzle.hintCheck.skillKey, dcText: String(puzzle.hintCheck.dc) } : NO_HINT_CHECK,
    parts: puzzle.parts.map((p) => ({ characterId: p.characterId, text: p.text, ownerUnavailable: p.ownerUnavailable })),
    wrong: wrongDraftOf(puzzle),
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
    case 'riddle': {
      const answers = puzzle.solution?.kind.case === 'riddle' ? puzzle.solution.kind.value.answers : [];
      return { ...base, kind: 'riddle', riddleText: config.value.text, answers: [...answers] };
    }
    case 'sequence': {
      const steps = puzzle.solution?.kind.case === 'sequence' ? puzzle.solution.kind.value.steps : [];
      return { ...base, kind: 'sequence', bells: config.value.bells, steps: [...steps] };
    }
    case 'cipher': {
      const solution = puzzle.solution?.kind.case === 'cipher' ? puzzle.solution.kind.value : undefined;
      const keyword = solution?.method.case === 'keyword' ? solution.method.value : '';
      return {
        ...base,
        kind: 'cipher',
        cipherMessage: solution?.message ?? '',
        cipherMethod: keyword === '' ? 'shift' : 'keyword',
        shift: solution?.method.case === 'shift' ? solution.method.value : 3,
        keyword,
        keyClueId: config.value.keyClueId,
      };
    }
    default:
      return null;
  }
}

/** "Ao errar" as the form shows it: a trap first, then the attempts, then the limits; more than one is flagged. */
function wrongDraftOf(puzzle: Puzzle): WrongDraft {
  const on = puzzle.onWrong;
  if (!on) {
    return NO_WRONG;
  }
  const trap = on.trap;
  const attempts = on.attemptsPerPlayer > 0;
  const limits = on.maxMoves > 0 || on.timeLimitSeconds > 0;
  const kinds = [!!trap, attempts, limits].filter(Boolean).length;
  const option: WrongOption = trap ? 'trap' : attempts ? 'attempts' : limits ? 'limits' : 'none';
  return {
    option,
    mapId: trap?.mapId ?? '',
    pointId: trap?.pointId ?? '',
    attempts: attempts ? on.attemptsPerPlayer : 3,
    movesText: on.maxMoves > 0 ? String(on.maxMoves) : '',
    minutesText: on.timeLimitSeconds > 0 ? String(Math.max(1, Math.round(on.timeLimitSeconds / 60))) : '',
    keptSeconds: on.timeLimitSeconds,
    combined: kinds > 1,
  };
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
    ...(draft.hintCheck.skillKey !== '' ? { hintCheck: { skillKey: draft.hintCheck.skillKey, dc: Number(draft.hintCheck.dcText) } } : {}),
    parts: draft.parts.map((p) => ({ characterId: p.characterId, text: p.text.trim() })),
    ...onWrongOf(draft),
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
    case 'riddle':
      return {
        ...common,
        seed: 0n,
        config: { kind: { case: 'riddle', value: { text: draft.riddleText.trim() } } },
        solution: { kind: { case: 'riddle', value: { answers: draft.answers.map((a) => a.trim()) } } },
      };
    case 'sequence':
      return {
        ...common,
        seed: 0n,
        config: { kind: { case: 'sequence', value: { bells: draft.bells, steps: draft.steps.length } } },
        solution: { kind: { case: 'sequence', value: { steps: [...draft.steps] } } },
      };
    case 'cipher':
      return {
        ...common,
        seed: 0n,
        config: { kind: { case: 'cipher', value: { keyClueId: draft.keyClueId } } },
        solution: { kind: { case: 'cipher', value: cipherSolutionOf(draft) } },
      };
  }
}

/** The message and the key a cipher is made from (also what `PreviewPuzzleCipher` takes). */
export function cipherSolutionOf(draft: Draft): MessageInitShape<typeof CipherSolutionSchema> {
  return {
    message: draft.cipherMessage.trim(),
    method: draft.cipherMethod === 'keyword' ? { case: 'keyword', value: draft.keyword.trim() } : { case: 'shift', value: draft.shift },
  };
}

/** The minutes as the server takes them, in seconds: untouched minutes keep the seconds the puzzle came with. */
function secondsOf(wrong: WrongDraft): number {
  const text = wrong.minutesText.trim();
  if (text === '') {
    return 0;
  }
  const minutes = Number(text);
  const kept = wrong.keptSeconds > 0 && Math.max(1, Math.round(wrong.keptSeconds / 60)) === minutes;
  return kept ? wrong.keptSeconds : minutes * 60;
}

/** "Ao errar" as the request writes it: only what the chosen option opens; nothing for "Nada acontece". */
function onWrongOf(draft: Draft): { onWrong?: MessageInitShape<typeof PuzzleOnWrongSchema> } {
  const wrong = draft.wrong;
  switch (wrong.option) {
    case 'trap':
      return { onWrong: { trap: { mapId: wrong.mapId, pointId: wrong.pointId } } };
    case 'attempts':
      return { onWrong: { attemptsPerPlayer: wrong.attempts } };
    case 'limits':
      return { onWrong: { maxMoves: Number(wrong.movesText.trim() || 0), timeLimitSeconds: secondsOf(wrong) } };
    default:
      return {};
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
  riddle?: string;
  /** The answers' list as a whole; each answer's own problem is in `answerRows`. */
  answers?: string;
  answerRows: Record<number, string>;
  sequence?: string;
  cipherMessage?: string;
  cipherKey?: string;
  /** The skill check ("Escolha a perícia e a CD"). */
  check?: string;
  /** By part's index. */
  parts: Record<number, { owner?: string; text?: string }>;
  /** "Ao errar": what is missing for the chosen option. */
  wrong?: string;
}

export function draftErrors(draft: Draft): DraftErrors {
  const errors: DraftErrors = { hints: {}, answerRows: {}, parts: {} };
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
  if (draft.kind === 'riddle') {
    riddleErrors(draft, errors);
  } else if (draft.kind === 'sequence') {
    errors.sequence = sequenceError(draft);
  } else if (draft.kind === 'cipher') {
    cipherErrors(draft, errors);
  }
  errors.check = hintCheckError(draft);
  partsErrors(draft, errors);
  errors.wrong = wrongError(draft);
  return errors;
}

function riddleErrors(draft: Draft, errors: DraftErrors): void {
  const text = textLength(draft.riddleText.trim());
  if (text === 0) {
    errors.riddle = 'Escreva o enigma que os jogadores vão ler.';
  } else if (text > RIDDLE_MAX) {
    errors.riddle = `O enigma passa de ${RIDDLE_MAX} letras: tem ${text}.`;
  }
  if (draft.answers.length === 0) {
    errors.answers = 'Escreva pelo menos uma resposta aceita.';
  }
  const seen = new Set<string>();
  draft.answers.forEach((answer, i) => {
    const n = textLength(answer.trim());
    const folded = fold(answer);
    if (n > ANSWER_MAX) {
      errors.answerRows[i] = `Esta resposta passa de ${ANSWER_MAX} letras: tem ${n}.`;
    } else if (folded === '') {
      errors.answerRows[i] = 'Esta resposta não tem nenhuma letra ou número.';
    } else if (seen.has(folded)) {
      errors.answerRows[i] = 'Esta resposta é igual a outra: maiúsculas, acentos e pontuação não contam.';
    }
    seen.add(folded);
  });
}

/** What is wrong with a sequence's steps, in words, or `undefined`. */
export function sequenceError(draft: Draft): string | undefined {
  if (draft.steps.length < STEPS_MIN) {
    return `Faltam passos: a sequência vai de ${STEPS_MIN} a ${STEPS_MAX}, e tem ${draft.steps.length}.`;
  }
  if (new Set(draft.steps).size < 2) {
    return 'Use pelo menos dois sinos diferentes: um sino só, tocado várias vezes, não é uma sequência.';
  }
  return undefined;
}

function cipherErrors(draft: Draft, errors: DraftErrors): void {
  const n = textLength(draft.cipherMessage.trim());
  if (n === 0) {
    errors.cipherMessage = 'Escreva a mensagem que os jogadores vão decifrar.';
  } else if (n > CIPHER_MESSAGE_MAX) {
    errors.cipherMessage = `A mensagem passa de ${CIPHER_MESSAGE_MAX} letras: tem ${n}.`;
  } else if (!hasCipherLetter(draft.cipherMessage)) {
    errors.cipherMessage = 'A mensagem precisa de pelo menos uma letra, de A a Z, para trocar.';
  }
  if (draft.cipherMethod === 'keyword') {
    const letters = keywordLetters(draft.keyword);
    if (letters.length < KEYWORD_MIN || letters.length > KEYWORD_MAX) {
      errors.cipherKey = `A palavra-chave precisa de ${KEYWORD_MIN} a ${KEYWORD_MAX} letras diferentes: tem ${letters.length}.`;
    } else if (keywordSwapsNothing(letters)) {
      errors.cipherKey = 'Essa palavra-chave não troca nenhuma letra. Escolha outra.';
    }
  }
}

function hintCheckError(draft: Draft): string | undefined {
  const check = draft.hintCheck;
  const dcText = check.dcText.trim();
  if (check.skillKey === '' && dcText === '') {
    return undefined;
  }
  if (check.skillKey === '') {
    return 'Escolha a perícia do teste, ou limpe a CD.';
  }
  const dc = /^\d{1,3}$/.test(dcText) ? Number(dcText) : NaN;
  if (!(dc >= DC_MIN && dc <= DC_MAX)) {
    return `A CD vai de ${DC_MIN} a ${DC_MAX}.`;
  }
  if (draft.hints.every((h) => h.trim() === '')) {
    return 'O teste dá dicas: escreva pelo menos uma dica acima.';
  }
  return undefined;
}

function partsErrors(draft: Draft, errors: DraftErrors): void {
  const owners = new Set<string>();
  draft.parts.forEach((part, i) => {
    const row: { owner?: string; text?: string } = {};
    const n = textLength(part.text.trim());
    if (n === 0) {
      row.text = 'Escreva a parte, ou tire esta.';
    } else if (n > PART_MAX) {
      row.text = `A parte passa de ${PART_MAX} letras: tem ${n}.`;
    }
    if (part.characterId !== '') {
      if (owners.has(part.characterId)) {
        row.owner = 'Cada personagem lê uma parte só.';
      }
      owners.add(part.characterId);
    }
    if (row.owner || row.text) {
      errors.parts[i] = row;
    }
  });
}

/** The field of "Ao errar" a refusal belongs to, from the draft (the client's own check) or from the server's `on_wrong.…` field name. */
export type WrongTarget = 'trap' | 'attempts' | 'moves' | 'minutes' | 'both' | '';

export function wrongTargetOf(field: string): WrongTarget {
  switch (field) {
    case 'on_wrong.trap':
      return 'trap';
    case 'on_wrong.attempts_per_player':
      return 'attempts';
    case 'on_wrong.max_moves':
      return 'moves';
    case 'on_wrong.time_limit_seconds':
      return 'minutes';
    default:
      return '';
  }
}

/** Which field of the chosen option is wrong, for the form's own check. */
export function wrongTarget(draft: Draft): WrongTarget {
  const wrong = draft.wrong;
  if (wrong.option === 'trap') {
    return wrong.mapId === '' || wrong.pointId === '' ? 'trap' : '';
  }
  if (wrong.option !== 'limits') {
    return '';
  }
  const moves = wrong.movesText.trim();
  const minutes = wrong.minutesText.trim();
  if (moves === '' && minutes === '') {
    return 'both';
  }
  if (moves !== '' && !(/^\d{1,4}$/.test(moves) && Number(moves) >= 1 && Number(moves) <= MOVES_LIMIT_MAX)) {
    return 'moves';
  }
  if (minutes !== '' && !(/^\d{1,4}$/.test(minutes) && Number(minutes) >= 1 && Number(minutes) <= MINUTES_MAX)) {
    return 'minutes';
  }
  return '';
}

function wrongError(draft: Draft): string | undefined {
  const wrong = draft.wrong;
  if (wrong.option === 'trap' && (wrong.mapId === '' || wrong.pointId === '')) {
    return 'Escolha a armadilha do mapa que dispara.';
  }
  if (wrong.option !== 'limits') {
    return undefined;
  }
  const moves = wrong.movesText.trim();
  const minutes = wrong.minutesText.trim();
  if (moves === '' && minutes === '') {
    return 'Ponha um limite de jogadas, de minutos, ou os dois.';
  }
  if (moves !== '' && !(/^\d{1,4}$/.test(moves) && Number(moves) >= 1 && Number(moves) <= MOVES_LIMIT_MAX)) {
    return `O limite de jogadas vai de 1 a ${MOVES_LIMIT_MAX}.`;
  }
  if (minutes !== '' && !(/^\d{1,4}$/.test(minutes) && Number(minutes) >= 1 && Number(minutes) <= MINUTES_MAX)) {
    return `O limite de tempo vai de 1 a ${MINUTES_MAX} minutos.`;
  }
  return undefined;
}

/** Whether a draft has nothing to fix. */
export function isValid(errors: DraftErrors): boolean {
  return (
    !errors.name &&
    !errors.clue &&
    !errors.message &&
    !errors.target &&
    !errors.lock &&
    !errors.riddle &&
    !errors.answers &&
    !errors.sequence &&
    !errors.cipherMessage &&
    !errors.cipherKey &&
    !errors.check &&
    !errors.wrong &&
    Object.keys(errors.hints).length === 0 &&
    Object.keys(errors.answerRows).length === 0 &&
    Object.keys(errors.parts).length === 0
  );
}

/** The kind a form builds, as the proto says it. */
export function protoKindOf(kind: FormKind): PuzzleKind {
  switch (kind) {
    case 'lights':
      return PuzzleKind.LIGHTS;
    case 'lock':
      return PuzzleKind.LOCK;
    case 'pillars':
      return PuzzleKind.PILLARS;
    case 'riddle':
      return PuzzleKind.RIDDLE;
    case 'sequence':
      return PuzzleKind.SEQUENCE;
    case 'cipher':
      return PuzzleKind.CIPHER;
  }
}
