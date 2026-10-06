import {
  effectOf,
  factorFromMeters,
  factorLabel,
  fits,
  maxDrawnColumns,
  metersField,
  rulesGrid,
} from './calibration';

describe('grid calibration', () => {
  it('says what a square of the drawing is worth in metres', () => {
    // The number and the unit never split: a no-break space between them.
    expect(factorLabel(1)).toBe('1,5\u00a0m');
    expect(factorLabel(2)).toBe('3\u00a0m');
    expect(factorLabel(3)).toBe('4,5\u00a0m');
    expect(factorLabel(20)).toBe('30\u00a0m');
  });

  it('reads a typed length as a factor only when it is a multiple of 1,5 m up to 30 m', () => {
    expect(factorFromMeters('4,5')).toBe(3);
    expect(factorFromMeters('4.5')).toBe(3);
    expect(factorFromMeters('3')).toBe(2);
    expect(factorFromMeters(' 30 ')).toBe(20);
    expect(factorFromMeters('1,5')).toBe(1);
    expect(factorFromMeters('2')).toBeNull();
    expect(factorFromMeters('0')).toBeNull();
    expect(factorFromMeters('31,5')).toBeNull();
    expect(factorFromMeters('')).toBeNull();
    expect(factorFromMeters('abc')).toBeNull();
    expect(metersField(3)).toBe('4,5');
    expect(metersField(2)).toBe('3');
  });

  it('keeps and scales what is painted for a larger multiple, and clears it for any other change', () => {
    expect(effectOf(1, 2)).toBe('scales');
    expect(effectOf(1, 3)).toBe('scales');
    expect(effectOf(2, 4)).toBe('scales');
    expect(effectOf(2, 6)).toBe('scales');
    expect(effectOf(2, 3)).toBe('clears');
    expect(effectOf(3, 2)).toBe('clears');
    expect(effectOf(2, 1)).toBe('clears');
    expect(effectOf(2, 2)).toBe('same');
  });

  it('works out the rules grid and the limits of 200 columns and 400 rows', () => {
    expect(rulesGrid(12, 8, 2)).toEqual({ columns: 24, rows: 16 });
    expect(rulesGrid(12, 8, 3)).toEqual({ columns: 36, rows: 24 });
    expect(fits(100, 100, 2)).toBe(true);
    expect(fits(101, 100, 2)).toBe(false);
    expect(fits(20, 201, 2)).toBe(false);
    expect(maxDrawnColumns(2)).toBe(100);
    expect(maxDrawnColumns(1)).toBe(200);
  });
});
