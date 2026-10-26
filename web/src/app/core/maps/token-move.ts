import { MapFormation, MapMoveStop } from '../../../gen/meurpg/maps/v1/maps_pb';
import { formatMeters, squaresText, squaresToMeters } from '../units';

/** The basis points of a map run from 0 to 10000 on each axis. */
const FULL = 10000;
const HALF = 0.5;
/** A block is rows of two. */
const BLOCK_WIDE = 2;
/** "Quadrados por clique" goes from 1 to 20 (B2-01). */
export const MIN_STEP = 1;
export const MAX_STEP = 20;

export type Direction = 'nw' | 'n' | 'ne' | 'w' | 'e' | 'sw' | 's' | 'se';

export interface DirectionInfo {
  readonly id: Direction;
  readonly dx: -1 | 0 | 1;
  readonly dy: -1 | 0 | 1;
  /** The arrow of the pad button. */
  readonly icon: string;
  /** What the button is called and how the summary says it: "para a direita". */
  readonly words: string;
}

/** The pad's eight directions in reading order (B2-01): the middle button is the move icon, not a direction. */
export const DIRECTIONS: readonly DirectionInfo[] = [
  {
    id: 'nw',
    dx: -1,
    dy: -1,
    icon: 'north_west',
    words: 'na diagonal, para cima e para a esquerda',
  },
  { id: 'n', dx: 0, dy: -1, icon: 'north', words: 'para cima' },
  { id: 'ne', dx: 1, dy: -1, icon: 'north_east', words: 'na diagonal, para cima e para a direita' },
  { id: 'w', dx: -1, dy: 0, icon: 'west', words: 'para a esquerda' },
  { id: 'e', dx: 1, dy: 0, icon: 'east', words: 'para a direita' },
  {
    id: 'sw',
    dx: -1,
    dy: 1,
    icon: 'south_west',
    words: 'na diagonal, para baixo e para a esquerda',
  },
  { id: 's', dx: 0, dy: 1, icon: 'south', words: 'para baixo' },
  { id: 'se', dx: 1, dy: 1, icon: 'south_east', words: 'na diagonal, para baixo e para a direita' },
];

export interface SquareRect {
  readonly col: number;
  readonly row: number;
  readonly cols: number;
  readonly rows: number;
}

/** The squares "Em fila" and "Em bloco" cover from the square tapped (its top-left corner), kept inside the grid: a line runs to the
 * right, a block is rows of two. An arc has no rectangle (null): it spreads around the first one. Only a preview: the server places. */
export function formationRect(
  formation: MapFormation,
  count: number,
  at: { col: number; row: number },
  grid: Grid,
): SquareRect | null {
  if (count <= 0 || formation === MapFormation.ARC) {
    return null;
  }
  const cols = formation === MapFormation.LINE ? count : Math.min(count, BLOCK_WIDE);
  const rows = formation === MapFormation.LINE ? 1 : Math.ceil(count / BLOCK_WIDE);
  const col = Math.max(0, Math.min(grid.columns - cols, at.col));
  const row = Math.max(0, Math.min(grid.rows - rows, at.row));
  return { col, row, cols, rows };
}

export interface Point {
  readonly xBp: number;
  readonly yBp: number;
}

export interface Grid {
  readonly columns: number;
  readonly rows: number;
}

/** The square that holds a point, kept inside the grid. */
export function squareAt(point: Point, grid: Grid): { col: number; row: number } {
  return {
    col: Math.max(0, Math.min(grid.columns - 1, Math.floor((point.xBp / FULL) * grid.columns))),
    row: Math.max(0, Math.min(grid.rows - 1, Math.floor((point.yBp / FULL) * grid.rows))),
  };
}

/** The middle of a square, in basis points. */
export function centerOf(col: number, row: number, grid: Grid): Point {
  return {
    xBp: Math.round(((col + HALF) / grid.columns) * FULL),
    yBp: Math.round(((row + HALF) / grid.rows) * FULL),
  };
}

/** Where `steps` squares in a direction from a token lead, kept on the map: the square the move asks for. */
export function targetAfter(from: Point, dir: DirectionInfo, steps: number, grid: Grid): Point {
  const at = squareAt(from, grid);
  const col = Math.max(0, Math.min(grid.columns - 1, at.col + dir.dx * steps));
  const row = Math.max(0, Math.min(grid.rows - 1, at.row + dir.dy * steps));
  return centerOf(col, row, grid);
}

/** The squares between two points, as the grid counts them: the larger of the column and the row distance. */
export function squaresBetween(from: Point, to: Point, grid: Grid): number {
  const a = squareAt(from, grid);
  const b = squareAt(to, grid);
  return Math.max(Math.abs(a.col - b.col), Math.abs(a.row - b.row));
}

/** "3 quadrados para a direita", "4 quadrados para a direita e 2 para baixo", "nenhum quadrado". */
export function stepsWords(from: Point, to: Point, grid: Grid): string {
  const a = squareAt(from, grid);
  const b = squareAt(to, grid);
  const dx = b.col - a.col;
  const dy = b.row - a.row;
  if (dx === 0 && dy === 0) {
    return 'nenhum quadrado';
  }
  const horizontal = dx === 0 ? '' : dx > 0 ? 'para a direita' : 'para a esquerda';
  const vertical = dy === 0 ? '' : dy > 0 ? 'para baixo' : 'para cima';
  if (dx === 0 || dy === 0) {
    return `${squaresText(Math.abs(dx || dy))} ${horizontal || vertical}`;
  }
  if (Math.abs(dx) === Math.abs(dy)) {
    return `${squaresText(Math.abs(dx))} na diagonal, ${vertical} e ${horizontal}`;
  }
  return `${squaresText(Math.abs(dx))} ${horizontal} e ${Math.abs(dy)} ${vertical}`;
}

/** "4,5 m" for a number of squares, at the table's rate (1,5 m a square). */
export function metersOf(squares: number): string {
  return formatMeters(squaresToMeters(squares));
}

/** What one token did in a move (`MapTokenMove`), as the screens use it. */
export interface TokenMoveResult {
  readonly characterId: string;
  readonly name: string;
  readonly xBp: number;
  readonly yBp: number;
  readonly asked: number;
  readonly moved: number;
  readonly stopped: MapMoveStop;
}

/** The answer of `MoveMapTokens`. */
export interface MoveOutcome {
  /** False when nobody moved. */
  readonly moved: boolean;
  readonly results: readonly TokenMoveResult[];
  /** For a group that did not move: the first whose arrival square did not fit. */
  readonly blocked: string;
}

/** The sentence that follows "O movimento parou antes" for each reason; empty when the token got there. */
function stopReason(stopped: MapMoveStop): string {
  switch (stopped) {
    case MapMoveStop.WALL:
      return 'Ninguém atravessa parede.';
    case MapMoveStop.NO_PASS:
      return 'O próximo quadrado está marcado como “Não dá para passar”.';
    case MapMoveStop.LOCKED:
      return 'A próxima porta está trancada.';
    case MapMoveStop.BLOCKED:
      return 'Algo no caminho barrou o movimento.';
    default:
      return '';
  }
}

/** A notice's weight: `done` is the green line, `stopped` the amber one. */
export interface MoveNotice {
  readonly tone: 'done' | 'stopped';
  /** The bold start of the notice ("O movimento parou antes."), or empty. */
  readonly lead: string;
  readonly text: string;
}

/** The "Pedido: 10 quadrados (15 m) · andou 5 (7,5 m)" line of one token's move. */
export function askedLine(r: TokenMoveResult): string {
  return `Pedido: ${squaresText(r.asked)} (${metersOf(r.asked)}) · andou ${r.moved} (${metersOf(r.moved)})`;
}

/** "Brisa andou 2 (3 m).": what a player reads of their own move (C9-02). */
function walkedLine(r: TokenMoveResult): string {
  return `${r.name} andou ${r.moved} (${metersOf(r.moved)}).`;
}

/** What the player reads after their own move (C9-02): "Brisa andou 2 (3 m).", or "O movimento parou antes." with the reason and how far
 * it got; when the server could not say why (`BLOCKED`: something the player has not seen, RN-10), only "O movimento parou antes.". */
function playerNotice(outcome: MoveOutcome): MoveNotice {
  const mine = outcome.results[0];
  if (!mine || mine.moved >= mine.asked) {
    return { tone: 'done', lead: '', text: mine ? walkedLine(mine) : '' };
  }
  const hidden = mine.stopped === MapMoveStop.BLOCKED;
  return {
    tone: 'stopped',
    lead: 'O movimento parou antes.',
    text: hidden ? '' : [stopReason(mine.stopped), walkedLine(mine)].filter(Boolean).join(' '),
  };
}

/** What the master reads after a move: where it stopped and why, or that it got there. The player has their own words (`player`). */
export function moveNotice(
  outcome: MoveOutcome,
  names: ReadonlyMap<string, string>,
  player = false,
): MoveNotice {
  if (player) {
    return playerNotice(outcome);
  }
  if (!outcome.moved && outcome.results.length > 1) {
    const who = names.get(outcome.blocked) ?? 'Alguém';
    return {
      tone: 'stopped',
      lead: 'Ninguém se moveu.',
      text: `${who} não cabe ali: o quadrado está bloqueado. Escolha outro destino ou tire ${who} da seleção.`,
    };
  }
  const short = outcome.results.filter((r) => r.moved < r.asked);
  if (short.length === 0) {
    const n = outcome.results.length;
    return {
      tone: 'done',
      lead: '',
      text:
        n === 1
          ? `${outcome.results[0].name} chegou ao destino.`
          : `Os ${n} chegaram ao destino, juntos.`,
    };
  }
  const first = short[0];
  const walked = `${first.name} andou ${metersOf(first.moved)}.`;
  const reason = stopReason(first.stopped);
  return {
    tone: 'stopped',
    lead:
      first.moved === 0 && short.length === 1
        ? 'O movimento não saiu do lugar.'
        : 'O movimento parou antes.',
    text: [reason, walked].filter(Boolean).join(' '),
  };
}

/** The label on the map at the point where a move stopped: "7,5 m · parou antes". */
export function stopLabel(r: TokenMoveResult): string {
  return `${metersOf(r.moved)} · parou antes`;
}
