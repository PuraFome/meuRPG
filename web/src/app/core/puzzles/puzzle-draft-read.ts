import type {
  CipherConfig,
  LockConfig,
  PillarsConfig,
  Puzzle,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  DEFAULT_ATTEMPTS,
  DEFAULT_SHIFT,
  type Draft,
  NO_HINT_CHECK,
  NO_SOLVE,
  NO_WRONG,
  type SolveDraft,
  type WrongDraft,
  type WrongOption,
  SECONDS_PER_MINUTE,
  newDraft,
  resized,
} from './puzzle-draft-model';

function riddleAnswers(puzzle: Puzzle): readonly string[] {
  return puzzle.solution?.kind.case === 'riddle' ? puzzle.solution.kind.value.answers : [];
}

function sequenceSteps(puzzle: Puzzle): readonly number[] {
  return puzzle.solution?.kind.case === 'sequence' ? puzzle.solution.kind.value.steps : [];
}

function lockDraft(puzzle: Puzzle, config: LockConfig): Partial<Draft> {
  const solution = puzzle.solution?.kind.case === 'lock' ? puzzle.solution.kind.value.wheels : [];
  const start = puzzle.start?.kind.case === 'lock' ? puzzle.start.kind.value.wheels : [];
  return {
    kind: 'lock',
    wheels: config.wheels,
    alphabet: config.alphabet,
    lockSolution: resized(solution, config.wheels),
    lockStart: resized(start, config.wheels),
  };
}

function pillarsDraft(puzzle: Puzzle, config: PillarsConfig): Partial<Draft> {
  const mural = puzzle.solution?.kind.case === 'pillars' ? puzzle.solution.kind.value.pillars : [];
  return {
    kind: 'pillars',
    pillars: config.pillars,
    symbols: config.symbols,
    linked: config.links.some((l) => l.alsoTurns.length > 0),
    mural: resized(mural, config.pillars),
  };
}

function cipherDraft(puzzle: Puzzle, config: CipherConfig): Partial<Draft> {
  const solution = puzzle.solution?.kind.case === 'cipher' ? puzzle.solution.kind.value : undefined;
  const keyword = solution?.method.case === 'keyword' ? solution.method.value : '';
  return {
    kind: 'cipher',
    cipherMessage: solution?.message ?? '',
    cipherMethod: keyword === '' ? 'shift' : 'keyword',
    shift: solution?.method.case === 'shift' ? solution.method.value : DEFAULT_SHIFT,
    keyword,
    keyClueId: config.keyClueId,
  };
}

/** What is particular to the kind the puzzle is, over the fields every kind has; `null` for a puzzle with no config. */
function kindDraftOf(puzzle: Puzzle): Partial<Draft> | null {
  const config = puzzle.config?.kind;
  switch (config?.case) {
    case 'lights':
      return { kind: 'lights', size: config.value.size };
    case 'lock':
      return lockDraft(puzzle, config.value);
    case 'pillars':
      return pillarsDraft(puzzle, config.value);
    case 'riddle':
      return { kind: 'riddle', riddleText: config.value.text, answers: [...riddleAnswers(puzzle)] };
    case 'sequence':
      return { kind: 'sequence', bells: config.value.bells, steps: [...sequenceSteps(puzzle)] };
    case 'cipher':
      return cipherDraft(puzzle, config.value);
    default:
      return null;
  }
}

/** The draft of an existing puzzle, for the edit page. */
export function draftOf(puzzle: Puzzle): Draft | null {
  const own = kindDraftOf(puzzle);
  if (!own) {
    return null;
  }
  return {
    ...newDraft('lights'),
    name: puzzle.name,
    clue: puzzle.clue,
    hints: [...puzzle.hints],
    solve: solveDraftOf(puzzle),
    hintCheck: puzzle.hintCheck
      ? { skillKey: puzzle.hintCheck.skillKey, dcText: String(puzzle.hintCheck.dc) }
      : NO_HINT_CHECK,
    parts: puzzle.parts.map((p) => ({
      characterId: p.characterId,
      text: p.text,
      ownerUnavailable: p.ownerUnavailable,
    })),
    wrong: wrongDraftOf(puzzle),
    ...own,
  };
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
    attempts: attempts ? on.attemptsPerPlayer : DEFAULT_ATTEMPTS,
    movesText: on.maxMoves > 0 ? String(on.maxMoves) : '',
    minutesText:
      on.timeLimitSeconds > 0
        ? String(Math.max(1, Math.round(on.timeLimitSeconds / SECONDS_PER_MINUTE)))
        : '',
    keptSeconds: on.timeLimitSeconds,
    combined: kinds > 1,
  };
}

function solveDraftOf(puzzle: Puzzle): SolveDraft {
  const on = puzzle.onSolve;
  const message = on?.message ?? '';
  switch (on?.target.case) {
    case 'door':
      return {
        ...NO_SOLVE,
        choice: 'door',
        mapId: on.target.value.mapId,
        col: on.target.value.col,
        row: on.target.value.row,
        message,
      };
    case 'point':
      return {
        ...NO_SOLVE,
        choice: 'point',
        mapId: on.target.value.mapId,
        pointId: on.target.value.pointId,
        message,
      };
    case 'clue':
      return { ...NO_SOLVE, choice: 'clue', clueId: on.target.value.clueId, message };
    default:
      return { ...NO_SOLVE, message };
  }
}
