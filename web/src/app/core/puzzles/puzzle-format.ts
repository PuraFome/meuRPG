import { timestampDate } from '@bufbuild/protobuf/wkt';

import {
  type Puzzle,
  type PuzzleRun,
  PuzzleAlphabet,
  PuzzleKind,
  type PuzzleLastMove,
  type PuzzleState,
  PuzzleSolveOutcome,
} from '../../../gen/meurpg/play/v1/puzzles_pb';
import { joinDots } from '../format/text';

/** The words for each kind of puzzle (E10-06). */
export function kindName(kind: PuzzleKind): string {
  switch (kind) {
    case PuzzleKind.LIGHTS:
      return 'Apagar as luzes';
    case PuzzleKind.LOCK:
      return 'Fechadura de combinação';
    case PuzzleKind.PILLARS:
      return 'Símbolos giratórios';
    case PuzzleKind.RIDDLE:
      return 'Enigma';
    case PuzzleKind.SEQUENCE:
      return 'Sequência';
    case PuzzleKind.CIPHER:
      return 'Cifra';
    default:
      return 'Quebra-cabeça';
  }
}

/** The Material Symbols icon beside a kind's name. */
export function kindIcon(kind: PuzzleKind): string {
  switch (kind) {
    case PuzzleKind.LIGHTS:
      return 'lightbulb';
    case PuzzleKind.LOCK:
      return 'key';
    case PuzzleKind.PILLARS:
      return 'diamond';
    case PuzzleKind.RIDDLE:
      return 'help';
    case PuzzleKind.SEQUENCE:
      return 'notifications';
    case PuzzleKind.CIPHER:
      return 'enhanced_encryption';
    default:
      return 'extension';
  }
}

/** "Faz todos jogarem juntos": what a player is told about who plays. */
export const PLAYED_TOGETHER = 'todos jogam juntos';

/** "Dígitos", "letras", "runas": what a lock's wheels show, for the list's second line. */
export function alphabetWord(alphabet: PuzzleAlphabet): string {
  switch (alphabet) {
    case PuzzleAlphabet.DIGITS:
      return 'dígitos';
    case PuzzleAlphabet.LETTERS:
      return 'letras';
    default:
      return 'runas';
  }
}

/** "1 roda", "4 rodas". */
export function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

/** Whether the pillars turn their neighbours too (any link). */
export function pillarsLinked(
  links: readonly { readonly alsoTurns: readonly number[] }[],
): boolean {
  return links.some((l) => l.alsoTurns.length > 0);
}

/** The second line of a list row: "Apagar as luzes · 5 × 5", "Fechadura de combinação · 4 rodas de runas". */
export function puzzleSummary(puzzle: Puzzle): string {
  const config = puzzle.config?.kind;
  switch (config?.case) {
    case 'lights':
      return joinDots([kindName(puzzle.kind), `${config.value.size} × ${config.value.size}`]);
    case 'lock':
      return joinDots([
        kindName(puzzle.kind),
        `${plural(config.value.wheels, 'roda', 'rodas')} de ${alphabetWord(config.value.alphabet)}`,
      ]);
    case 'pillars':
      return joinDots([
        kindName(puzzle.kind),
        `${plural(config.value.pillars, 'pilar', 'pilares')}, ${pillarsLinked(config.value.links) ? 'girando juntos' : 'sem ligações'}`,
      ]);
    case 'riddle':
      return joinDots([
        kindName(puzzle.kind),
        puzzle.solution?.kind.case === 'riddle'
          ? plural(
              puzzle.solution.kind.value.answers.length,
              'resposta aceita',
              'respostas aceitas',
            )
          : '',
      ]);
    case 'sequence':
      return joinDots([
        kindName(puzzle.kind),
        `${plural(config.value.steps, 'passo', 'passos')}, ${plural(config.value.bells, 'sino', 'sinos')}`,
      ]);
    case 'cipher':
      return joinDots([
        kindName(puzzle.kind),
        puzzle.solution?.kind.case === 'cipher' &&
        puzzle.solution.kind.value.method.case === 'keyword'
          ? 'palavra-chave'
          : 'deslocamento',
      ]);
    default:
      return kindName(puzzle.kind);
  }
}

/** How many lights are lit in a board (counting what the server sent; no rule is applied). */
export function litCount(state: PuzzleState | undefined): number {
  const kind = state?.kind;
  return kind?.case === 'lights' ? kind.value.lit.filter(Boolean).length : 0;
}

/** The accessible name of one light: "Luz na linha 2, coluna 3, acesa". Rows and columns count from 1 on screen. */
export function lightLabel(row: number, col: number, lit: boolean): string {
  return `Luz na linha ${row + 1}, coluna ${col + 1}, ${lit ? 'acesa' : 'apagada'}`;
}

/** "12 acesas", "1 acesa". */
export function litWords(n: number): string {
  return n === 1 ? '1 acesa' : `${n} acesas`;
}

/** The unit of a puzzle's moves, for "Faltam, no mínimo, 3 toques". */
export function moveWord(kind: PuzzleKind, n: number): string {
  switch (kind) {
    case PuzzleKind.LIGHTS:
      return n === 1 ? 'toque' : 'toques';
    case PuzzleKind.PILLARS:
      return n === 1 ? 'giro' : 'giros';
    default:
      return n === 1 ? 'jogada' : 'jogadas';
  }
}

/** "agora há pouco", "há 12 s", "há 3 min", "há 2 h": how long ago, from the two clocks. Never negative. */
export function agoText(at: Date, now: Date): string {
  const seconds = Math.max(0, Math.round((now.getTime() - at.getTime()) / 1000));
  if (seconds < 5) {
    return 'agora há pouco';
  }
  if (seconds < 60) {
    return `há ${seconds} s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `há ${minutes} min`;
  }
  return `há ${Math.floor(minutes / 60)} h`;
}

/** "3ª roda", "1º pilar" are written by the callers; this is the ordinal mark for a wheel. */
function wheelOrdinal(n: number): string {
  return `${n}ª`;
}

/** Who made the last move and what they did: "Lia" and "tocou numa luz". `own` is the reader's own character: "Você". */
export function lastMoveParts(
  last: PuzzleLastMove | undefined,
  own = '',
): { readonly who: string; readonly what: string } | null {
  if (!last) {
    return null;
  }
  const who = own !== '' && last.characterName === own ? 'Você' : last.characterName;
  switch (last.move?.kind.case) {
    case 'lights':
      return { who, what: 'tocou numa luz' };
    case 'lock':
      return { who, what: `girou a ${wheelOrdinal(last.move.kind.value.wheel + 1)} roda` };
    case 'pillars':
      return { who, what: `girou o pilar ${last.move.kind.value.pillar + 1}` };
    // The master's own read keeps what was typed or struck; a player's never does (puzzles.proto, `PuzzleLastMove.move`).
    case 'riddle':
      return {
        who,
        what: `${last.wrong ? 'tentou' : 'respondeu'} “${last.move.kind.value.answer}”: ${last.wrong ? 'errou' : 'acertou'}`,
      };
    case 'cipher':
      return {
        who,
        what: `digitou “${last.move.kind.value.text}”: ${last.wrong ? 'errou' : 'acertou'}`,
      };
    case 'sequence':
      return {
        who,
        what: last.wrong
          ? `errou no passo ${last.step}. A tentativa recomeçou`
          : `acertou o passo ${last.step}`,
      };
    default:
      return null;
  }
}

/** The name the master gave the trap point that the last wrong move fired ("Dardos envenenados"), or `''`. */
export function trapFired(last: PuzzleLastMove | undefined): string {
  return last?.trapName ?? '';
}

/** "4:48": seconds as minutes and seconds, for a clock that runs down. */
export function clockSeconds(total: number): string {
  const seconds = Math.max(0, Math.round(total));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
}

/** The seconds left until `deadline`, never negative. */
export function secondsUntil(deadline: Date, now: Date): number {
  return Math.max(0, Math.ceil((deadline.getTime() - now.getTime()) / 1000));
}

/** The last move in words, without the time: "Lia tocou numa luz", "Toren girou a 3ª roda", "Lia girou o pilar 1". Empty for a move this screen does not know. */
export function lastMoveText(last: PuzzleLastMove | undefined, own = ''): string {
  const parts = lastMoveParts(last, own);
  return parts ? `${parts.who} ${parts.what}` : '';
}

/** "O pilar 1 mudou.", "Os pilares 1 e 2 mudaram.", "Os pilares 1, 2 e 4 mudaram.": which pillars the last move turned (0-based in, 1-based out). */
export function pillarsChangedText(changed: readonly number[]): string {
  const names = changed.map((i) => i + 1);
  if (names.length === 0) {
    return '';
  }
  if (names.length === 1) {
    return `O pilar ${names[0]} mudou.`;
  }
  return `Os pilares ${names.slice(0, -1).join(', ')} e ${names[names.length - 1]} mudaram.`;
}

/** What the server did when the puzzle was solved, as the master reads it. */
export function outcomeText(outcome: PuzzleSolveOutcome): string {
  switch (outcome) {
    case PuzzleSolveOutcome.NOTIFIED:
      return 'Só você foi avisado.';
    case PuzzleSolveOutcome.DOOR_OPENED:
      return 'Uma porta se abriu.';
    case PuzzleSolveOutcome.DOOR_NOT_CLOSED:
      return 'A porta já estava aberta, ou foi pintada por cima.';
    case PuzzleSolveOutcome.POINT_REVEALED:
      return 'Um ponto apareceu no mapa.';
    case PuzzleSolveOutcome.POINT_ALREADY_REVEALED:
      return 'O ponto já estava à vista.';
    case PuzzleSolveOutcome.CLUE_REVEALED:
      return 'Uma pista foi para quem resolveu.';
    case PuzzleSolveOutcome.CLUE_ALREADY_HAD:
      return 'Quem resolveu já tinha a pista.';
    case PuzzleSolveOutcome.TARGET_GONE:
      return 'O alvo do “Ao resolver” foi apagado: nada aconteceu além do aviso.';
    default:
      return '';
  }
}

/** The clock of a solved puzzle, "21:12". */
export function clockOf(at: Date): string {
  return `${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`;
}

/** One counter that stays on a player's screen: "Suas tentativas  2 de 3". */
export interface CounterRow {
  readonly key: 'attempts' | 'moves' | 'time';
  readonly label: string;
  readonly value: string;
  /** The counter is at its end (no attempts left, no moves, no time). */
  readonly spent: boolean;
}

/**
 * The counters of "Ao errar" the player always sees on a puzzle that has limits (E10-12 state 10): the player's own attempts left, the
 * moves made out of the limit, and the time left out of the limit. The time ticks from the server's `deadline` (the seconds in the
 * message are worked out when it is read), so it never needs another read to run down.
 */
export function limitRows(run: PuzzleRun, now: Date): CounterRow[] {
  const limits = run.limits;
  if (!limits) {
    return [];
  }
  const rows: CounterRow[] = [];
  if (limits.attemptsPerPlayer > 0) {
    rows.push({
      key: 'attempts',
      label: 'Suas tentativas',
      value: `${limits.attemptsLeft} de ${limits.attemptsPerPlayer}`,
      spent: limits.attemptsLeft <= 0 && !run.solved,
    });
  }
  if (limits.maxMoves > 0) {
    rows.push({
      key: 'moves',
      label: 'Jogadas',
      value: `${limits.movesMade} de ${limits.maxMoves}`,
      spent: limits.movesMade >= limits.maxMoves && !run.solved,
    });
  }
  if (limits.timeLimitSeconds > 0) {
    // Once the puzzle is over (solved, or a limit stopped it) the clock stands where the server read it: a solved puzzle is not "acabou".
    const over = run.solved || run.stopped;
    const left =
      over || !limits.deadline
        ? limits.secondsLeft
        : secondsUntil(timestampDate(limits.deadline), now);
    rows.push({
      key: 'time',
      label: 'Tempo',
      value: `${clockSeconds(left)} de ${clockSeconds(limits.timeLimitSeconds)}`,
      spent: left <= 0 && !run.solved,
    });
  }
  return rows;
}
