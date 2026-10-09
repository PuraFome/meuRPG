import type { DiceRoll } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  type AdvantageSource,
  type DamageStep,
  type RollModeRequest,
  RollMode,
  RollModeRequestStatus,
} from '../../../gen/meurpg/play/v1/combat_rolls_pb';
import { rollFormula } from './combat-dice';

/** The three modes a d20 is rolled with, in the order the radio group shows them. */
export const ROLL_MODES: readonly RollMode[] = [
  RollMode.NORMAL,
  RollMode.ADVANTAGE,
  RollMode.DISADVANTAGE,
];

/** A mode the server left unset reads as a normal roll. */
export function orNormal(mode: RollMode): RollMode {
  return mode === RollMode.UNSPECIFIED ? RollMode.NORMAL : mode;
}

/** "Normal", "Vantagem" or "Desvantagem". */
export function modeWord(mode: RollMode): string {
  switch (orNormal(mode)) {
    case RollMode.ADVANTAGE:
      return 'Vantagem';
    case RollMode.DISADVANTAGE:
      return 'Desvantagem';
    default:
      return 'Normal';
  }
}

function rank(mode: RollMode): number {
  switch (orNormal(mode)) {
    case RollMode.ADVANTAGE:
      return 1;
    case RollMode.DISADVANTAGE:
      return -1;
    default:
      return 0;
  }
}

/** The chosen mode is better for the roller than the suggestion: only the master gives it. */
export function isBetter(chosen: RollMode, suggested: RollMode): boolean {
  return rank(chosen) > rank(suggested);
}

/** How many d20 the mode rolls: one, or two for advantage and disadvantage. */
export function d20Count(mode: RollMode): 1 | 2 {
  return orNormal(mode) === RollMode.NORMAL ? 1 : 2;
}

/** "Vantagem" or "Desvantagem" for the tag of a source. */
export function sourceWord(source: AdvantageSource): string {
  return source.effect === RollMode.ADVANTAGE ? 'Vantagem' : 'Desvantagem';
}

/** How the roll mode stands for whoever is about to roll. */
export type ModeStatus =
  /** Roll now with this mode. */
  | 'ready'
  /** A change from the suggestion needs its reason (1 to 120 characters). */
  | 'needs-reason'
  /** A better mode than the suggestion: ask the master. */
  | 'needs-request'
  /** The request waits for the master. */
  | 'pending'
  /** A better mode than the suggestion that this roll cannot ask for. */
  | 'blocked';

/** How a better mode than the suggested one is reached: the master sets it, a player asks him, or cannot. */
export type Approval = 'free' | 'request' | 'blocked';

/** The longest reason a request or a mode change takes. */
export const REASON_MAX = 120;

/** The reason is 1 to 120 characters on one line. */
export function reasonValid(reason: string): boolean {
  const text = reason.trim();
  return text.length >= 1 && text.length <= REASON_MAX && !/[\r\n]/.test(text);
}

/** Where the roll mode stands: what the screen asks before the dice may be rolled. */
export function modeStatus(
  approval: Approval,
  suggested: RollMode,
  chosen: RollMode,
  reason: string,
  request: RollModeRequest | null,
): ModeStatus {
  if (request?.status === RollModeRequestStatus.PENDING) {
    return 'pending';
  }
  if (request?.status === RollModeRequestStatus.ANSWERED) {
    return 'ready';
  }
  if (rank(chosen) === rank(suggested)) {
    return 'ready';
  }
  if (isBetter(chosen, suggested) && approval !== 'free') {
    if (approval === 'blocked') {
      return 'blocked';
    }
    return reasonValid(reason) ? 'needs-request' : 'needs-reason';
  }
  return reasonValid(reason) ? 'ready' : 'needs-reason';
}

/** The request of the attack is still open (waits or is answered), by id. */
export function liveRequest(
  requests: readonly RollModeRequest[],
  id: string,
): RollModeRequest | null {
  const r = requests.find((x) => x.id === id);
  return r && r.status !== RollModeRequestStatus.CLOSED ? r : null;
}

/** The master's card: "Pedido de Vantagem: Toren → Goblin (Espada curta) — “estou escondido”". */
export function requestLine(r: RollModeRequest, labelOf: (combatantId: string) => string): string {
  return `Pedido de ${modeWord(r.requestedMode)}: ${labelOf(r.combatantId)} → ${labelOf(r.targetId)} (${r.attackNamePt}) — “${r.reason}”`;
}

/** The two d20 of a roll, or the one: each face with whether it is the one that counts. */
export function d20Faces(roll: DiceRoll): readonly { face: number; counted: boolean }[] {
  const pair = roll.diceCount > 1 && roll.faces.length > 1;
  return roll.faces.map((face, i) => ({ face, counted: !pair || i === roll.countedIndex }));
}

/** "Resistência a fogo (tiefling): 10 → 5". */
export function stepLine(step: DamageStep): string {
  return `${step.labelPt}: ${step.before} → ${step.after}`;
}

/** The formula of a d20: `14 + 5 = 19` for the face that counts of a pair, the usual one for a single d20. */
export function d20Formula(roll: DiceRoll): string {
  if (roll.diceCount > 1 && roll.faces.length > 1) {
    const kept = roll.faces[roll.countedIndex] ?? roll.faces[0];
    const mod =
      roll.modifier === 0 ? '' : ` ${roll.modifier < 0 ? '−' : '+'} ${Math.abs(roll.modifier)}`;
    return `${kept}${mod} = ${roll.total}`;
  }
  return rollFormula(roll);
}
