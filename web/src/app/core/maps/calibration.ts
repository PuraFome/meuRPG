import { SQUARE_M, formatMeters } from '../units';

/**
 * The grid calibration (MR-025, RN-25): "Cada quadrado deste desenho vale" N squares of 1,5 m (`square_factor`, 1 to 20).
 * Pure helpers, so the question's words are tested without a screen. The rules' grid is the drawing's times the factor;
 * the server holds every limit and refuses what is over them, so these only keep the screen from offering what it will
 * refuse, and say what a change does to the painted layers (docs of `SetMapGrid`).
 */

/** `SetMapGrid.square_factor` goes from 1 (1,5 m) to 20 (30 m). */
export const MAX_FACTOR = 20;
/** The rules' grid stays within 200 columns and 400 rows. */
export const MAX_RULE_COLUMNS = 200;
export const MAX_RULE_ROWS = 400;

/** The choices the question lists before "Outro": 1,5 m, 3 m, 4,5 m and 6 m. */
export const PRESET_FACTORS: readonly number[] = [1, 2, 3, 4];

/** What a square of the drawing is worth, in metres: factor 2 is "3 m", factor 3 is "4,5 m". */
export function factorLabel(factor: number): string {
  return formatMeters(factor * SQUARE_M);
}

/** The factor a typed length stands for ("4,5" is 3), or `null` when it is not a multiple of 1,5 m from 1,5 to 30 m. */
export function factorFromMeters(text: string): number | null {
  const cleaned = text.trim().replace(',', '.');
  if (!/^\d{1,2}(\.\d)?$/.test(cleaned)) {
    return null;
  }
  const factor = Math.round(Number(cleaned) / SQUARE_M);
  const exact = Math.abs(factor * SQUARE_M - Number(cleaned)) < 1e-9;
  return exact && factor >= 1 && factor <= MAX_FACTOR ? factor : null;
}

/** The text a field shows for a factor: "4,5" for 3. */
export function metersField(factor: number): string {
  return String(Math.round(factor * SQUARE_M * 10) / 10).replace('.', ',');
}

/** What the change does to what was painted: nothing to do, scaled with it kept, or cleared (the app asks first). */
export type CalibrationEffect = 'same' | 'scales' | 'clears';

/**
 * A larger factor that is a multiple of the old one, with the same columns, keeps everything, scaled (each painted
 * square becomes `new/old` by `new/old`); any other change clears the layers and what the players remember.
 */
export function effectOf(oldFactor: number, newFactor: number): CalibrationEffect {
  if (newFactor === oldFactor) {
    return 'same';
  }
  return newFactor > oldFactor && newFactor % oldFactor === 0 ? 'scales' : 'clears';
}

/** The rules' grid a factor makes: "24 × 16" squares for a drawing of 12 × 8 at 3 m. */
export function rulesGrid(
  drawnColumns: number,
  drawnRows: number,
  factor: number,
): { columns: number; rows: number } {
  return { columns: drawnColumns * factor, rows: drawnRows * factor };
}

/** Whether the rules' grid of that factor fits the server's limits. */
export function fits(drawnColumns: number, drawnRows: number, factor: number): boolean {
  const g = rulesGrid(drawnColumns, drawnRows, factor);
  return g.columns <= MAX_RULE_COLUMNS && g.rows <= MAX_RULE_ROWS;
}

/** The most columns a drawing can have at that factor. */
export function maxDrawnColumns(factor: number): number {
  return Math.floor(MAX_RULE_COLUMNS / Math.max(1, factor));
}
