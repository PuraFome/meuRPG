import { tight } from '../../../core/format/text';

/**
 * The arithmetic of printing a map to scale (MR-033, E8-12), apart from the
 * components so it is tested without a DOM. Everything is in centimetres.
 *
 * The map is `columns` squares wide, each `squareCm` across, so its width in
 * cm is columns x squareCm and its height follows the image's proportions
 * (rows use the same size, and the last row may be partial). It is cut in
 * sheets of a paper with 1 cm margins; each sheet repeats the 1 cm of its
 * neighbours, so the pieces can be glued with no gap.
 */

export const DEFAULT_SQUARE_CM = 2.54;
export const MIN_SQUARE_CM = 1;
export const MAX_SQUARE_CM = 10;
/** The browser leaves this much blank on every side of a sheet. */
export const MARGIN_CM = 1;
/** Each sheet repeats this much of the next one. */
export const OVERLAP_CM = 1;
/** Over this many sheets the app warns and shrinks the labels. */
export const MANY_SHEETS = 16;
/** Over this many sheets the preview hides the labels altogether. */
export const TOO_MANY_LABELS = 36;

export type PaperId = 'a4' | 'a3' | 'a2' | 'a1' | 'carta' | 'oficio';
export type Orientation = 'landscape' | 'portrait';

export interface Paper {
  readonly id: PaperId;
  readonly name: string;
  /** "No A1", "Na Carta": the article each name takes in a sentence. */
  readonly inName: string;
  readonly shortCm: number;
  readonly longCm: number;
}

/** Ofício is the Brazilian 21,6 x 33 cm (the US Legal is 21,6 x 35,6). */
export const PAPERS: readonly Paper[] = [
  { id: 'a4', name: 'A4', inName: 'No A4', shortCm: 21, longCm: 29.7 },
  { id: 'a3', name: 'A3', inName: 'No A3', shortCm: 29.7, longCm: 42 },
  { id: 'a2', name: 'A2', inName: 'No A2', shortCm: 42, longCm: 59.4 },
  { id: 'a1', name: 'A1', inName: 'No A1', shortCm: 59.4, longCm: 84.1 },
  { id: 'carta', name: 'Carta', inName: 'Na Carta', shortCm: 21.59, longCm: 27.94 },
  { id: 'oficio', name: 'Ofício', inName: 'No Ofício', shortCm: 21.6, longCm: 33 },
];

/** The square size in the field: "2,54" or "2.54", from 1 to 10; else null. */
export function parseSquareCm(text: string): number | null {
  const typed = text.trim().replace(',', '.');
  if (!/^\d+(\.\d{1,3})?$/.test(typed)) {
    return null;
  }
  const value = Number(typed);
  return value >= MIN_SQUARE_CM && value <= MAX_SQUARE_CM ? value : null;
}

export interface MapSizeCm {
  readonly width: number;
  readonly height: number;
}

/** The map on paper: `columns` squares wide, the height in the image's
 * proportions (the last row of squares may be partial). */
export function mapSizeCm(
  columns: number,
  imageWidth: number,
  imageHeight: number,
  squareCm: number,
): MapSizeCm {
  const width = columns * squareCm;
  return { width, height: (width * imageHeight) / Math.max(1, imageWidth) };
}

/** One way of cutting the map: a paper, an orientation, how many sheets. */
export interface Plan {
  readonly paper: Paper;
  readonly orientation: Orientation;
  /** The sheet's size in this orientation. */
  readonly paperW: number;
  readonly paperH: number;
  /** What a sheet prints: the paper minus the margins. */
  readonly usableW: number;
  readonly usableH: number;
  readonly columns: number;
  readonly rows: number;
  readonly sheets: number;
}

/** Sheets along one side: ceil((size - 1) / (usable - 1)), the 1 cm overlap.
 * The rounding keeps a map that fits exactly (60 cm on 3 x 20 cm) from
 * needing one more sheet because of floating point. */
export function sheetsAlong(sizeCm: number, usableCm: number): number {
  const exact = (sizeCm - OVERLAP_CM) / (usableCm - OVERLAP_CM);
  return Math.max(1, Math.ceil(Math.round(exact * 1e6) / 1e6));
}

export function planFor(paper: Paper, orientation: Orientation, map: MapSizeCm): Plan {
  const landscape = orientation === 'landscape';
  const paperW = landscape ? paper.longCm : paper.shortCm;
  const paperH = landscape ? paper.shortCm : paper.longCm;
  const usableW = paperW - 2 * MARGIN_CM;
  const usableH = paperH - 2 * MARGIN_CM;
  const columns = sheetsAlong(map.width, usableW);
  const rows = sheetsAlong(map.height, usableH);
  return { paper, orientation, paperW, paperH, usableW, usableH, columns, rows, sheets: columns * rows };
}

export interface PaperPlans {
  readonly paper: Paper;
  readonly landscape: Plan;
  readonly portrait: Plan;
  /** The orientation with fewer sheets; landscape on a tie. */
  readonly best: Plan;
}

export function plansForPaper(paper: Paper, map: MapSizeCm): PaperPlans {
  const landscape = planFor(paper, 'landscape', map);
  const portrait = planFor(paper, 'portrait', map);
  return { paper, landscape, portrait, best: portrait.sheets < landscape.sheets ? portrait : landscape };
}

/** The paper that spends the fewest sheets, the first of the list on a tie. */
export function leanestPaper(all: readonly PaperPlans[]): PaperPlans {
  return all.reduce((a, b) => (b.best.sheets < a.best.sheets ? b : a));
}

/** The label letters of the rows: A to Z, then AA, AB... */
export function rowLetters(row: number): string {
  let n = row;
  let letters = '';
  do {
    letters = String.fromCharCode(65 + (n % 26)) + letters;
    n = Math.floor(n / 26) - 1;
  } while (n >= 0);
  return letters;
}

/** A sheet's name: the row's letter and the column's number, "B2". Rows run
 * A, B, C from the top, columns 1, 2, 3 from the left. */
export function sheetName(row: number, column: number): string {
  return `${rowLetters(row)}${column + 1}`;
}

/** "Página B2 · cole à direita da B1 e abaixo da A2": where to glue it. */
export function sheetLabel(row: number, column: number): string {
  const glue: string[] = [];
  if (column > 0) {
    glue.push(`à direita da ${sheetName(row, column - 1)}`);
  }
  if (row > 0) {
    glue.push(`abaixo da ${sheetName(row - 1, column)}`);
  }
  const title = `Página ${sheetName(row, column)}`;
  return glue.length === 0 ? `${title} · canto de cima, à esquerda` : `${title} · cole ${glue.join(' e ')}`;
}

/** Where a sheet's content starts on the map, in cm. */
export function sheetOrigin(plan: Plan, row: number, column: number): { x: number; y: number } {
  return {
    x: column * (plan.usableW - OVERLAP_CM),
    y: row * (plan.usableH - OVERLAP_CM),
  };
}

/** The size of all the sheets glued together. */
export function gluedSize(plan: Plan): MapSizeCm {
  return {
    width: plan.columns * plan.usableW - (plan.columns - 1) * OVERLAP_CM,
    height: plan.rows * plan.usableH - (plan.rows - 1) * OVERLAP_CM,
  };
}

/** The blank paper past the map's right and bottom edges, in cm (0 when the
 * map covers the sheets). */
export function spareCm(plan: Plan, map: MapSizeCm): { right: number; bottom: number } {
  const glued = gluedSize(plan);
  const clean = (n: number): number => (n < 0.05 ? 0 : n);
  return { right: clean(glued.width - map.width), bottom: clean(glued.height - map.height) };
}

/** The grid lines a sheet draws, as positions on the sheet in cm: a line at
 * every multiple of the square, only inside the map. */
export function gridLines(
  plan: Plan,
  map: MapSizeCm,
  squareCm: number,
  row: number,
  column: number,
): { xs: number[]; ys: number[]; width: number; height: number } {
  const o = sheetOrigin(plan, row, column);
  const width = Math.min(plan.usableW, map.width - o.x);
  const height = Math.min(plan.usableH, map.height - o.y);
  const along = (origin: number, extent: number): number[] => {
    const out: number[] = [];
    const first = Math.ceil(Math.round((origin / squareCm) * 1e6) / 1e6);
    for (let k = first; ; k++) {
      const at = Math.round((k * squareCm - origin) * 1e4) / 1e4;
      if (at > extent + 1e-6) {
        break;
      }
      out.push(at);
    }
    return out;
  };
  return { xs: along(o.x, width), ys: along(o.y, height), width, height };
}

/** Keeps every number tied to its unit and to the "×" around it, so a line
 * of the page never ends on "76,2 ×" or begins with "cm": the shared `tight`
 * (de 1 a 10) plus the units this page writes. */
export function tightCm(text: string): string {
  return tight(text)
    .replace(/(\d) (cm|folhas?|colunas?|linhas?|quadrados?)\b/g, '$1\u00a0$2')
    .replace(/ × /g, '\u00a0×\u00a0');
}

/** "76,2" for the sums: one decimal, comma. */
export function cm1(value: number): string {
  return (Math.round(value * 10) / 10).toFixed(1).replace('.', ',');
}

/** "21", "29,7": one decimal at most, no ",0". */
export function cmShort(value: number): string {
  return String(Math.round(value * 10) / 10).replace('.', ',');
}

/** The count with its noun: "1 folha", "9 folhas". */
export function sheetCount(n: number): string {
  return `${n} ${n === 1 ? 'folha' : 'folhas'}`;
}
