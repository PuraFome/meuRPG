import { simulate } from './dice-hindsight';
import type { DiceSides, DieSpec } from './dice-types';

const SEEDS = [1, 2, 3, 7, 42, 99, 123, 2024, 31337, 4000000000];
const TRAYS = [
  { width: 320, height: 176 },
  { width: 358, height: 197 },
  { width: 480, height: 260 },
  { width: 520, height: 280 },
];
const TIDY_DEGREES = 10;
const CAP_MS = 3000;

/** A result for a die from a seed, so the sweep covers every face over the seeds. */
function resultFor(sides: DiceSides, seed: number, i: number): number {
  return ((seed * 31 + i * 17) % sides) + 1;
}

const dieOf = (sides: DiceSides, seed: number, i = 0): DieSpec => ({
  sides,
  result: resultFor(sides, seed, i),
});

/** What the tray must read back: a d100 is two d10, "00" to "90" and "0" to "9". */
function expectedFaces(spec: DieSpec): string[] {
  if (spec.sides === 100) {
    const tens = Math.floor((spec.result % 100) / 10) * 10;
    return [String(tens).padStart(2, '0'), String(spec.result % 10)];
  }
  return [String(spec.result)];
}

describe('DiceTray.simulate', () => {
  for (const sides of [4, 6, 8, 10, 12, 20, 100] as const) {
    it(`lands a d${sides} on the requested face, within 3 s and ±${TIDY_DEGREES}°, on every seed and tray`, () => {
      for (const tray of TRAYS) {
        for (const seed of SEEDS) {
          const dice = [dieOf(sides, seed)];
          const out = simulate(dice, { ...tray, seed });
          expect(out.finalFaces, `d${sides} seed ${seed} ${tray.width}`).toEqual(
            expectedFaces(dice[0]),
          );
          expect(out.durationMs).toBeLessThanOrEqual(CAP_MS);
          for (const tilt of out.restTilts) {
            expect(Math.abs(tilt)).toBeLessThanOrEqual((TIDY_DEGREES * Math.PI) / 180 + 1e-9);
          }
        }
      }
    });
  }

  it('lands several dice of every kind on their results together', () => {
    const mix: DiceSides[] = [20, 20, 6, 6, 6, 8, 4, 100];
    for (const seed of SEEDS) {
      const dice = mix.map((sides, i) => dieOf(sides, seed, i));
      const out = simulate(dice, { width: 358, height: 197, seed });
      expect(out.finalFaces).toEqual(dice.flatMap(expectedFaces));
      expect(out.durationMs).toBeLessThanOrEqual(CAP_MS);
    }
  });

  it('gives the same roll for the same seed, and another for another seed', () => {
    const dice: DieSpec[] = [dieOf(20, 5), dieOf(6, 5, 1), dieOf(12, 5, 2)];
    const a = simulate(dice, { seed: 77 });
    const b = simulate(dice, { seed: 77 });
    const c = simulate(dice, { seed: 78 });
    expect(b).toEqual(a);
    expect(c.durationMs).not.toBe(a.durationMs);
  });

  it('takes about one to two seconds for a normal roll', () => {
    const times = SEEDS.map(
      (seed) => simulate([dieOf(20, seed), dieOf(20, seed, 1)], { seed }).durationMs,
    );
    expect(Math.min(...times)).toBeGreaterThanOrEqual(600);
    expect(Math.max(...times)).toBeLessThanOrEqual(CAP_MS);
  });

  it('refuses a die that does not exist and a face it does not have', () => {
    expect(() => simulate([])).toThrow(TypeError);
    expect(() => simulate([{ sides: 7 as DiceSides, result: 3 }])).toThrow(RangeError);
    expect(() => simulate([{ sides: 6, result: 7 }])).toThrow(RangeError);
    expect(() => simulate([{ sides: 6, result: 0 }])).toThrow(RangeError);
    expect(() =>
      simulate(Array.from({ length: 13 }, () => ({ sides: 6 as const, result: 1 }))),
    ).toThrow(RangeError);
  });
});
