import { describe, expect, it } from 'vitest';
import { create } from '@bufbuild/protobuf';

import { JumpLimitsSchema } from '../../../gen/meurpg/rules/v1/rules_pb';
import { HEIGHT_STEP_DFT, lineLengthDft, limitFor, limitsLine, maxHeight, runSeal, stepHeight } from './jump-plan';

// Toren, Força 16: 16 ft running / 8 ft standing, high 6 / 3 ft (README-B, "Numbers").
const toren = create(JumpLimitsSchema, {
  longRunningDft: 160,
  longStandingDft: 80,
  highRunningDft: 60,
  highStandingDft: 30,
  runningStart: true,
});
// Brisa, Força 10: 10 / 5 ft, high 3 / 1,5 ft: standing is 0,45 m.
const brisa = create(JumpLimitsSchema, {
  longRunningDft: 100,
  longStandingDft: 50,
  highRunningDft: 30,
  highStandingDft: 15,
  runningStart: false,
});

// The text ties numbers to their units with no-break spaces: read it with plain ones.
const plain = (t: string) => t.replace(/\u00a0/g, ' ');

describe('jump plan', () => {
  it('picks the limit the server says applies', () => {
    expect(limitFor(toren, 'long')).toBe(160);
    expect(limitFor(toren, 'high')).toBe(60);
    expect(limitFor(brisa, 'long')).toBe(50);
  });

  it('words both limits and the seal', () => {
    expect(plain(limitsLine(160, 80))).toBe('4,8 m com corrida · 2,4 m parado');
    expect(runSeal(toren).word).toBe('Com corrida');
    expect(runSeal(brisa).word).toBe('Parado');
    expect(plain(runSeal(brisa).reason)).toBe('Você ainda não andou pelo menos 3,0 m a pé neste turno.');
  });

  it('steps the high jump by 0,3 m, rounded down, never above the limit', () => {
    expect(maxHeight(60)).toBe(60);
    expect(maxHeight(15)).toBe(10);
    expect(maxHeight(9)).toBe(0);
    expect(stepHeight(60, 1, 60)).toBe(60);
    expect(stepHeight(60, -1, 60)).toBe(50);
    expect(stepHeight(HEIGHT_STEP_DFT, -1, 60)).toBe(HEIGHT_STEP_DFT);
    expect(stepHeight(10, 1, 15)).toBe(10);
  });

  it('measures the line the jump draws', () => {
    expect(lineLengthDft({ col: 10, row: 7 }, { col: 13, row: 7 })).toBe(150);
    expect(lineLengthDft({ col: 0, row: 0 }, { col: 1, row: 1 })).toBe(71);
  });
});
