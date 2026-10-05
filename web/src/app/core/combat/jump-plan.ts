import type { JumpLimits } from '../../../gen/meurpg/rules/v1/rules_pb';
import { metersFixed } from '../units';
import { type Square, DFT_PER_SQUARE } from './combat-grid';
import { joinDots } from '../format/text';

/**
 * The jump on the "Mover" page (E9-06, MR-034). The limits are the server's
 * (`GetTurnOptions.options.jumps`, from the sheet's Força); this only picks the
 * ones that apply and words them. The server decides every jump: the browser
 * shows the line it will draw, and a jump the limit does not reach comes back
 * from `MoveCombatant` as `TOO_FAR` with the limit.
 */

export type JumpMode = 'long' | 'high';

/** The limit that applies now: with a running start or standing, as the server says. */
export function limitFor(limits: JumpLimits, kind: JumpMode): number {
  if (kind === 'long') {
    return limits.runningStart ? limits.longRunningDft : limits.longStandingDft;
  }
  return limits.runningStart ? limits.highRunningDft : limits.highStandingDft;
}

/** "4,8 m com corrida · 2,4 m parado". */
export function limitsLine(running: number, standing: number): string {
  return joinDots([`${metersFixed(running / 10)} com corrida`, `${metersFixed(standing / 10)} parado`]);
}

/** The move on foot that gives a running start: 10 ft (SRD), said in metres. */
const RUN_M = metersFixed(10);

/** The seal beside the limits: "Com corrida" or "Parado", with the reason. */
export function runSeal(limits: JumpLimits): { readonly word: string; readonly reason: string } {
  return limits.runningStart
    ? { word: 'Com corrida', reason: `Você andou ${RUN_M} a pé antes de saltar.` }
    : { word: 'Parado', reason: `Você ainda não andou ${RUN_M} a pé neste turno.` };
}

/** The high jump's stepper moves 0,3 m (1 ft) at a time, from 0,3 m up to the
 * limit rounded down to a step (never above it): Brisa standing, 0,45 m, tops
 * out at 0,3 m. `0` when not even a step fits: no high jump. */
export const HEIGHT_STEP_DFT = 10;

export function maxHeight(limitDft: number): number {
  return Math.floor(limitDft / HEIGHT_STEP_DFT) * HEIGHT_STEP_DFT;
}

/** One step up (`+1`) or down (`-1`) from `current`, kept between one step and the maximum. */
export function stepHeight(current: number, direction: 1 | -1, limitDft: number): number {
  const max = maxHeight(limitDft);
  return Math.min(max, Math.max(Math.min(HEIGHT_STEP_DFT, max), current + direction * HEIGHT_STEP_DFT));
}

/** The straight line from the jumper's square to the chosen one, in tenths of a
 * foot: what a long jump costs (difficult terrain on the way does not count).
 * Display only: it is the length of the line drawn, the same one the server
 * charges, and the server's answer is the one that counts. */
export function lineLengthDft(from: Square, to: Square): number {
  return Math.round(Math.hypot(from.col - to.col, from.row - to.row) * DFT_PER_SQUARE);
}
