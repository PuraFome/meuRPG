import { describe, expect, it } from 'vitest';

import { PAPERS, planFor } from '../print-math';
import { buildSheet } from './print-sheet';

describe('buildSheet', () => {
  const size = { width: 60, height: 80 };
  const plan = planFor(PAPERS[0], 'portrait', size);

  it('names the neighbours that exist and no others', () => {
    const first = buildSheet(plan, size, 2.54, 0, 0);
    expect(first).toMatchObject({ key: 'A1', above: null, left: null, below: 'B1', right: 'A2' });
    const middle = buildSheet(plan, size, 2.54, 1, 1);
    expect(middle).toMatchObject({ above: 'A2', below: 'C2', left: 'B1', right: 'B3' });
  });

  it('puts a band, a dashed edge and a cross on every side that has a neighbour', () => {
    const middle = buildSheet(plan, size, 2.54, 1, 1);
    expect(middle.bands).toHaveLength(4);
    expect(middle.marks).toBe(
      `M1 0V${plan.usableH}M${plan.usableW - 1} 0V${plan.usableH}M0 1H${plan.usableW}M0 ${plan.usableH - 1}H${plan.usableW}`,
    );
    expect(middle.crosses.startsWith('M0.65 1H1.35M1 0.65V1.35')).toBe(true);
    expect(buildSheet(plan, size, 2.54, 0, 0).bands).toHaveLength(2);
  });

  it('has neither bands nor crosses when the map fits one sheet', () => {
    const small = { width: 10, height: 10 };
    const one = buildSheet(planFor(PAPERS[0], 'portrait', small), small, 2.54, 0, 0);
    expect(one).toMatchObject({ bands: [], marks: '', crosses: '' });
  });
});
