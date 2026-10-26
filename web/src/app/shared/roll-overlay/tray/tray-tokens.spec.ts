import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

type Rgb = [number, number, number];

// `ng test` runs from the web workspace (a path from `import.meta.url` changes under `--coverage`).
const STYLES = resolve(process.cwd(), 'src/styles.scss');
const css = readFileSync(STYLES, 'utf8');

/** A `light-dark(#a, #b)` token of `styles.scss`: its two sides. */
function pair(name: string): { light: Rgb; dark: Rgb } {
  const m = new RegExp(
    `${name}:\\s*light-dark\\(\\s*(#[0-9a-f]{6}),\\s*(#[0-9a-f]{6})\\s*\\)`,
    'i',
  ).exec(css);
  if (!m) {
    throw new Error(`${name} is not a light-dark() pair of hex colours in styles.scss`);
  }
  const rgb = (h: string): Rgb => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16)) as Rgb;
  return { light: rgb(m[1]), dark: rgb(m[2]) };
}

const lin = (c: number): number => {
  const v = c / 255;
  return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
};
const lum = (c: Rgb): number => 0.2126 * lin(c[0]) + 0.7152 * lin(c[1]) + 0.0722 * lin(c[2]);
const ratio = (a: Rgb, b: Rgb): number => {
  const [x, y] = [lum(a), lum(b)];
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
};
const mix = (p: Rgb, q: Rgb, t: number): Rgb => p.map((v, i) => v + (q[i] - v) * t) as Rgb;
const WHITE: Rgb = [255, 255, 255];

/** The tone the tray gives a face whose normal is tilted off the light by `q` bands (dice-draw.ts `tone`). */
function toned(base: Rgb, shade: Rgb, q: number): Rgb {
  const v = q * 0.09;
  return v >= 0
    ? mix(base, WHITE, Math.min(0.5, v * 1.3))
    : mix(base, shade, Math.min(0.9, -v * 0.75));
}

const NUMBER_MIN = 4.5;
const EDGE_MIN = 3;
// A die at rest is within ±10° of flat: its top face is at most one band off the light.
const BANDS = [-1, 0, 1];
const THEMES = ['light', 'dark'] as const;

describe('the tray tokens', () => {
  const felt = pair('--mr-tray-felt');
  const paper = pair('--mr-die-paper');
  const paperShade = pair('--mr-die-paper-shade');
  const ink = pair('--mr-die-ink');
  const garnet = pair('--mr-die-garnet');
  const garnetShade = pair('--mr-die-garnet-shade');
  const onGarnet = pair('--mr-die-on-garnet');
  const edge = pair('--mr-die-edge');
  const gold = pair('--mr-die-gold');
  const onGold = pair('--mr-die-on-gold');

  for (const theme of THEMES) {
    it(`keeps the numbers at 4.5:1 or more against their face (${theme})`, () => {
      for (const q of BANDS) {
        const onPaper = ratio(ink[theme], toned(paper[theme], paperShade[theme], q));
        const onRed = ratio(onGarnet[theme], toned(garnet[theme], garnetShade[theme], q));
        expect(onPaper).toBeGreaterThanOrEqual(NUMBER_MIN);
        expect(onRed).toBeGreaterThanOrEqual(NUMBER_MIN);
      }
    });

    it(`keeps a critical's gold number readable on the garnet (${theme})`, () => {
      expect(ratio(gold[theme], garnet[theme])).toBeGreaterThanOrEqual(NUMBER_MIN);
      expect(ratio(onGold[theme], gold[theme])).toBeGreaterThanOrEqual(NUMBER_MIN);
    });

    it(`keeps the die edges at 3:1 or more against the felt (${theme})`, () => {
      expect(ratio(edge[theme], felt[theme])).toBeGreaterThanOrEqual(EDGE_MIN);
    });
  }
});
