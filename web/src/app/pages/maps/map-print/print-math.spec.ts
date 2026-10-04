import {
  MANY_SHEETS,
  PAPERS,
  TOO_MANY_LABELS,
  gluedSize,
  gridLines,
  leanestPaper,
  mapSizeCm,
  parseSquareCm,
  plansForPaper,
  rowLetters,
  sheetLabel,
  sheetName,
  sheetsAlong,
  spareCm,
} from './print-math';

/** The artboard's map: 30 x 20 squares on a 3.000 x 2.000 px image. */
const map = (squareCm: number) => mapSizeCm(30, 3000, 2000, squareCm);
const paper = (id: string) => PAPERS.find((p) => p.id === id)!;
const counts = (id: string, squareCm: number) => {
  const p = plansForPaper(paper(id), map(squareCm));
  return { landscape: p.landscape.sheets, portrait: p.portrait.sheets, best: p.best.sheets };
};

describe('print-math: the map in cm', () => {
  it('is squares x size: 30 x 20 x 2,54 is 76,2 x 50,8', () => {
    const m = map(2.54);
    expect(m.width).toBeCloseTo(76.2, 6);
    expect(m.height).toBeCloseTo(50.8, 6);
  });

  it('keeps the image proportions when the rows do not divide evenly', () => {
    const m = mapSizeCm(30, 3000, 1990, 2);
    expect(m.width).toBe(60);
    expect(m.height).toBeCloseTo(39.8, 6);
  });
});

describe('print-math: sheets (REVIEW-2 verified counts)', () => {
  // [paper, landscape, portrait] at each size.
  const table: Record<number, [string, number, number][]> = {
    2.54: [
      ['a4', 9, 10],
      ['a3', 4, 6],
      ['a2', 4, 2],
      ['a1', 1, 2],
      ['carta', 12, 10],
      ['oficio', 9, 10],
    ],
    5: [
      ['a4', 36, 36],
      ['a3', 16, 18],
      ['a2', 9, 8],
      ['a1', 4, 6],
      ['carta', 36, 36],
      ['oficio', 30, 36],
    ],
  };

  for (const [size, rows] of Object.entries(table)) {
    for (const [id, landscape, portrait] of rows) {
      it(`${size} cm on ${id}: ${landscape} in landscape, ${portrait} in portrait`, () => {
        const c = counts(id, Number(size));
        expect(c.landscape).toBe(landscape);
        expect(c.portrait).toBe(portrait);
        expect(c.best).toBe(Math.min(landscape, portrait));
      });
    }
  }

  it('2 cm on A3 is 3 sheets in portrait (4 in landscape)', () => {
    expect(counts('a3', 2)).toEqual({ landscape: 4, portrait: 3, best: 3 });
  });

  it('A4 at 2,54 cm is 3 x 3 in landscape, with 4,9 and 4,2 cm of blank paper', () => {
    const plan = plansForPaper(paper('a4'), map(2.54)).best;
    expect(plan).toMatchObject({ orientation: 'landscape', columns: 3, rows: 3, sheets: 9 });
    expect(plan.usableW).toBeCloseTo(27.7, 6);
    expect(plan.usableH).toBeCloseTo(19, 6);
    const spare = spareCm(plan, map(2.54));
    expect(spare.right).toBeCloseTo(4.9, 6);
    expect(spare.bottom).toBeCloseTo(4.2, 6);
  });

  it('Carta in landscape misses the map by 0,38 cm, so portrait wins', () => {
    const p = plansForPaper(paper('carta'), map(2.54));
    expect(p.landscape.sheets).toBe(12);
    expect(p.best.orientation).toBe('portrait');
    expect(p.best.sheets).toBe(10);
  });

  it('Ofício is 21,6 x 33 cm', () => {
    expect(paper('oficio')).toMatchObject({ shortCm: 21.6, longCm: 33 });
  });

  it('breaks a tie in favour of landscape', () => {
    const p = plansForPaper(paper('a4'), map(5));
    expect(p.landscape.sheets).toBe(p.portrait.sheets);
    expect(p.best.orientation).toBe('landscape');
  });

  it('picks portrait when it has fewer sheets', () => {
    expect(plansForPaper(paper('a3'), map(2)).best.orientation).toBe('portrait');
  });

  it('does not add a sheet to a map that fits exactly (floating point)', () => {
    // 3 sheets of 20 cm: 3 x 20 - 2 x 1 = 58 cm.
    expect(sheetsAlong(58, 20)).toBe(3);
    expect(sheetsAlong(58.01, 20)).toBe(4);
    expect(sheetsAlong(1, 20)).toBe(1);
  });

  it('names the paper that spends fewer sheets (A1 for 5 cm)', () => {
    const all = PAPERS.map((p) => plansForPaper(p, map(5)));
    const lean = leanestPaper(all);
    expect(lean.paper.id).toBe('a1');
    expect(lean.best).toMatchObject({ orientation: 'landscape', columns: 2, rows: 2, sheets: 4 });
  });

  it('puts the thresholds at 16 and 36 sheets', () => {
    expect(MANY_SHEETS).toBe(16);
    expect(TOO_MANY_LABELS).toBe(36);
    // A3 at 5 cm is exactly 16 (no notice); A4 is 36 (the notice, labels still shown).
    expect(counts('a3', 5).best).toBe(16);
    expect(counts('a4', 5).best).toBe(36);
    // 10 cm gives 136 on A4 (144 in landscape, the figure on the artboard): the labels go.
    expect(counts('a4', 10).best).toBe(136);
  });

  it('glues the sheets back to the map size or more', () => {
    for (const p of PAPERS) {
      for (const size of [1, 2, 2.54, 5, 10]) {
        const plan = plansForPaper(p, map(size)).best;
        const glued = gluedSize(plan);
        expect(glued.width).toBeGreaterThanOrEqual(map(size).width - 1e-6);
        expect(glued.height).toBeGreaterThanOrEqual(map(size).height - 1e-6);
      }
    }
  });
});

describe('print-math: the field', () => {
  it('takes a comma or a dot', () => {
    expect(parseSquareCm('2,54')).toBe(2.54);
    expect(parseSquareCm('2.54')).toBe(2.54);
    expect(parseSquareCm(' 5 ')).toBe(5);
  });

  it('takes 1 to 10 and nothing else', () => {
    expect(parseSquareCm('1')).toBe(1);
    expect(parseSquareCm('10')).toBe(10);
    for (const bad of ['0,5', '0', '10,1', '11', '', 'abc', '2,5,4', '-2', '2 cm', '1e1']) {
      expect(parseSquareCm(bad)).toBeNull();
    }
  });
});

describe('print-math: labels', () => {
  it('names a sheet by row letter and column number', () => {
    expect(sheetName(0, 0)).toBe('A1');
    expect(sheetName(1, 1)).toBe('B2');
    expect(sheetName(2, 2)).toBe('C3');
  });

  it('goes on with AA after Z', () => {
    expect(rowLetters(25)).toBe('Z');
    expect(rowLetters(26)).toBe('AA');
    expect(rowLetters(27)).toBe('AB');
    expect(rowLetters(52)).toBe('BA');
  });

  it('says where to glue the sheet', () => {
    expect(sheetLabel(1, 1)).toBe('Página B2 · cole à direita da B1 e abaixo da A2');
    expect(sheetLabel(0, 1)).toBe('Página A2 · cole à direita da A1');
    expect(sheetLabel(2, 0)).toBe('Página C1 · cole abaixo da B1');
    expect(sheetLabel(0, 0)).toContain('Página A1');
  });
});

describe('print-math: the grid on a sheet', () => {
  const plan = plansForPaper(paper('a4'), map(2.54)).best;

  it('draws a line at every square, inside the map', () => {
    const g = gridLines(plan, map(2.54), 2.54, 0, 0);
    expect(g.xs[0]).toBe(0);
    expect(g.xs[1]).toBeCloseTo(2.54, 6);
    // 27,7 cm holds 10 whole squares of 2,54 cm: lines at 0 .. 25,4.
    expect(g.xs.length).toBe(11);
    expect(g.ys.length).toBe(8); // 19 cm: 0 .. 17,78
  });

  it('continues the grid across the overlap, with no jump', () => {
    // The second sheet starts at 26,7 cm: the first line is at 2,54 x 11 = 27,94.
    const g = gridLines(plan, map(2.54), 2.54, 0, 1);
    expect(g.xs[0]).toBeCloseTo(27.94 - 26.7, 4);
  });

  it('stops at the map edge on the last sheet', () => {
    const g = gridLines(plan, map(2.54), 2.54, 2, 2);
    // Origin x = 2 x 26,7 = 53,4: 22,8 cm of map left on this sheet.
    expect(g.width).toBeCloseTo(76.2 - 53.4, 6);
    expect(Math.max(...g.xs)).toBeLessThanOrEqual(g.width + 1e-6);
    expect(g.height).toBeCloseTo(50.8 - 36, 6);
  });
});
