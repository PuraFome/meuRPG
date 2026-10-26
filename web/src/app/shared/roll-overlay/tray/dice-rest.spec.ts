import { restRadius } from './dice-polyhedra';
import { FRAME, MAX_ON_TRAY, RIM, expand, simulateBodies } from './dice-sim';
import { staticScene } from './dice-scene';
import type { DiceSides, DieSpec } from './dice-types';

/** The sizes the overlay uses: a phone's width, a narrow phone, and the 520 × 280 cap. */
const TRAYS = [
  { width: 320, height: 176 },
  { width: 358, height: 197 },
  { width: 520, height: 280 },
];
const SEEDS = Array.from({ length: 60 }, (_, i) => 1 + i * 7919);
/** The room a "descartado" pill takes beside a die, at most (its height plus the gap). */
const LABEL = 24;
/** Float noise of the wall push-out, far below a pixel. */
const EPS = 1e-3;

const KINDS: DiceSides[] = [20, 6, 8, 4, 12, 10, 100, 20, 6, 8, 4, 12];

describe('where the dice come to rest', () => {
  for (const count of Array.from({ length: 8 }, (_, i) => i + 1)) {
    it(`keeps ${count} dice inside the felt, silhouette and label included, on every tray size`, () => {
      for (const tray of TRAYS) {
        for (const seed of SEEDS) {
          const dice: DieSpec[] = KINDS.slice(0, count).map((sides, i) => ({
            sides,
            result: 1 + ((seed + i) % sides),
            discarded: i === 0 && count > 1,
          }));
          const bodies = expand(dice);
          expect(bodies.length).toBeLessThanOrEqual(MAX_ON_TRAY);
          const sim = simulateBodies(bodies, { ...tray, seed });
          bodies.forEach((b, j) => {
            const o = (sim.steps * sim.n + j) * FRAME;
            const x = sim.frames[o];
            const y = sim.frames[o + 1];
            const reach = sim.R * restRadius(b.kind);
            const where = `${count} dice, d${b.kind}, seed ${seed}, ${tray.width}x${tray.height}`;
            expect(x - reach, where).toBeGreaterThanOrEqual(RIM - EPS);
            expect(x + reach, where).toBeLessThanOrEqual(tray.width - RIM + EPS);
            expect(y - reach, where).toBeGreaterThanOrEqual(RIM - EPS);
            expect(y + reach, where).toBeLessThanOrEqual(tray.height - RIM + EPS);
          });
        }
      }
    });
  }

  it('lays the dice out inside the felt too, at rest, for every count and size', () => {
    for (const tray of TRAYS) {
      for (const count of [1, 2, 3, 4, 5, 6, 7, 8]) {
        const dice: DieSpec[] = KINDS.slice(0, count).map((sides) => ({ sides, result: 1 }));
        const scene = staticScene(expand(dice), 1, tray.width, tray.height);
        scene.bodies.forEach((b, i) => {
          const v = scene.views[i];
          const reach = b.R * restRadius(b.kind);
          expect(v.x - reach).toBeGreaterThanOrEqual(RIM - EPS);
          expect(v.x + reach).toBeLessThanOrEqual(tray.width - RIM + EPS);
          expect(v.y - reach).toBeGreaterThanOrEqual(RIM - EPS);
          expect(v.y + reach).toBeLessThanOrEqual(tray.height - RIM + EPS);
        });
      }
    }
  });

  it('leaves room for the label of a discarded die when the tray is not crowded', () => {
    for (const tray of TRAYS) {
      for (const seed of SEEDS) {
        const bodies = expand([
          { sides: 20, result: 3, discarded: true },
          { sides: 20, result: 17 },
        ]);
        const sim = simulateBodies(bodies, { ...tray, seed });
        const o = sim.steps * sim.n * FRAME;
        const reach = sim.R * restRadius(20);
        const x = sim.frames[o];
        const y = sim.frames[o + 1];
        // the label sits under, over or beside the die: one of the four sides has room
        const room =
          y + reach + LABEL <= tray.height - RIM ||
          y - reach - LABEL >= RIM ||
          x + reach + LABEL <= tray.width - RIM ||
          x - reach - LABEL >= RIM;
        expect(room, `seed ${seed} ${tray.width}`).toBe(true);
      }
    }
  });
});
