import type { MessageInitShape } from '@bufbuild/protobuf';

import {
  PuzzleKind,
  PuzzleSolveAction,
  type CipherSolutionSchema,
  type PuzzleOnSolveSchema,
  type PuzzleOnWrongSchema,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import {
  type Draft,
  type FormKind,
  SECONDS_PER_MINUTE,
  type SolveDraft,
  type WrongDraft,
} from './puzzle-draft-model';
import type { PuzzleInit } from './puzzles-client';

/** The pillars' links when "Girar junto com os vizinhos" is on: each pillar turns the one on its left and the one on its right. */
export function neighbourLinks(count: number, linked: boolean): { alsoTurns: number[] }[] {
  return Array.from({ length: count }, (_, i) => ({
    alsoTurns: linked ? [i - 1, i + 1].filter((n) => n >= 0 && n < count) : [],
  }));
}

/** The `config` and `solution` the server needs to draw a start for the lights or the pillars. */
export function startRequestOf(
  draft: Draft,
): { config: PuzzleInit['config']; solution: PuzzleInit['solution'] } | null {
  if (draft.kind === 'lights') {
    return {
      config: { kind: { case: 'lights', value: { size: draft.size } } },
      solution: undefined,
    };
  }
  if (draft.kind === 'pillars') {
    return {
      config: {
        kind: {
          case: 'pillars',
          value: {
            pillars: draft.pillars,
            symbols: draft.symbols,
            links: neighbourLinks(draft.pillars, draft.linked),
          },
        },
      },
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
      return {
        action: PuzzleSolveAction.OPEN_DOOR,
        message,
        target: { case: 'door', value: { mapId: solve.mapId, col: solve.col, row: solve.row } },
      };
    case 'point':
      return {
        action: PuzzleSolveAction.REVEAL_POINT,
        message,
        target: { case: 'point', value: { mapId: solve.mapId, pointId: solve.pointId } },
      };
    case 'clue':
      return {
        action: PuzzleSolveAction.REVEAL_CLUE,
        message,
        target: { case: 'clue', value: { clueId: solve.clueId } },
      };
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
    ...(draft.hintCheck.skillKey !== ''
      ? { hintCheck: { skillKey: draft.hintCheck.skillKey, dc: Number(draft.hintCheck.dcText) } }
      : {}),
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
        config: {
          kind: { case: 'lock', value: { wheels: draft.wheels, alphabet: draft.alphabet } },
        },
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
        solution: {
          kind: { case: 'riddle', value: { answers: draft.answers.map((a) => a.trim()) } },
        },
      };
    case 'sequence':
      return {
        ...common,
        seed: 0n,
        config: {
          kind: { case: 'sequence', value: { bells: draft.bells, steps: draft.steps.length } },
        },
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
    method:
      draft.cipherMethod === 'keyword'
        ? { case: 'keyword', value: draft.keyword.trim() }
        : { case: 'shift', value: draft.shift },
  };
}

/** The minutes as the server takes them, in seconds: untouched minutes keep the seconds the puzzle came with. */
function secondsOf(wrong: WrongDraft): number {
  const text = wrong.minutesText.trim();
  if (text === '') {
    return 0;
  }
  const minutes = Number(text);
  const kept =
    wrong.keptSeconds > 0 &&
    Math.max(1, Math.round(wrong.keptSeconds / SECONDS_PER_MINUTE)) === minutes;
  return kept ? wrong.keptSeconds : minutes * SECONDS_PER_MINUTE;
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
      return {
        onWrong: {
          maxMoves: Number(wrong.movesText.trim() || 0),
          timeLimitSeconds: secondsOf(wrong),
        },
      };
    default:
      return {};
  }
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
