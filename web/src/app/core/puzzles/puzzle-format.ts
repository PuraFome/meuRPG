import {
  type Puzzle,
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
export function pillarsLinked(links: readonly { readonly alsoTurns: readonly number[] }[]): boolean {
  return links.some((l) => l.alsoTurns.length > 0);
}

/** The second line of a list row: "Apagar as luzes · 5 × 5", "Fechadura de combinação · 4 rodas de runas". */
export function puzzleSummary(puzzle: Puzzle): string {
  const config = puzzle.config?.kind;
  switch (config?.case) {
    case 'lights':
      return joinDots([kindName(puzzle.kind), `${config.value.size} × ${config.value.size}`]);
    case 'lock':
      return joinDots([kindName(puzzle.kind), `${plural(config.value.wheels, 'roda', 'rodas')} de ${alphabetWord(config.value.alphabet)}`]);
    case 'pillars':
      return joinDots([
        kindName(puzzle.kind),
        `${plural(config.value.pillars, 'pilar', 'pilares')}, ${pillarsLinked(config.value.links) ? 'girando juntos' : 'sem ligações'}`,
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
export function lastMoveParts(last: PuzzleLastMove | undefined, own = ''): { readonly who: string; readonly what: string } | null {
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
    default:
      return null;
  }
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
