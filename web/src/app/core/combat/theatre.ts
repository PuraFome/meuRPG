import {
  type Combatant,
  CombatantKind,
  CombatantSide,
  CoverDegree,
  type Encounter,
  EncounterMode,
  EncounterStatus,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { metersFixed } from '../units';
import { isDown } from './combat-view';

/**
 * Combat without a grid on screen, the "teatro da mente" (MR-025, RN-25, E10-04). The server decides everything
 * that depends on place; the browser only branches on `Encounter.mode`, draws a list instead of a map, and asks
 * for movement as a number. Nothing here works out a reach, a distance or a rule: the plan of the movement
 * sheet is arithmetic on what the server sent (`movement_left_*`), and the server refuses what is too far.
 * Pure functions, tested without a DOM.
 */

/** The combat is played without a map. A combat the server did not tell the mode of (an old copy) is a map one. */
export function isTheatre(e: Pick<Encounter, 'mode'> | null | undefined): boolean {
  return e?.mode === EncounterMode.THEATRE;
}

/** The one line that says why the mode exists, written once (E10-04 state 1): the dialog and the player's panel use it. */
export const THEATRE_WHY = 'Sem mapa, o app não sabe onde ninguém está.';

/** What follows the line in the start dialog. */
export const THEATRE_WHY_DIALOG =
  'Por isso não há mapa, alcance, névoa, armadilhas nem posição das criaturas convocadas. Dá para escolher só agora: o modo vale até o fim do combate.';

/** What follows the line on the player's panel. */
export const THEATRE_WHY_PLAYER = 'O mestre diz quem está ao alcance e a que distância; você diz quanto andou.';

/** The step of "Gastar movimento": one square, 5 ft, 1,5 m. */
export const SPEND_STEP_FT = 5;

/** The server's own limits of `SpendMovement.distance_ft`. */
export const SPEND_MAX_FT = 600;

/** What the movement sheet and the master's stepper work out from what is left. */
export interface SpendPlan {
  /** The amount now, in whole feet. */
  readonly ft: number;
  /** The most that can be spent (what is left, in whole feet; 0 when nothing is). */
  readonly limitFt: number;
  readonly canLess: boolean;
  readonly canMore: boolean;
  /** "1,5 m": the amount, as the field reads. */
  readonly amount: string;
  /** "3,0 m": what is left after spending this amount. */
  readonly after: string;
  /** What is left now. */
  readonly left: string;
  /** What the turn has in all. */
  readonly total: string;
  /** What was already spent this turn. */
  readonly spent: string;
  /** Percent of the turn's movement that is left after the amount, for the bar. */
  readonly afterPercent: number;
  readonly leftPercent: number;
}

/** The amount, kept between one step and what is left. Nothing left means 0. */
export function clampSpend(ft: number, leftFt: number): number {
  const limit = Math.min(Math.max(0, leftFt), SPEND_MAX_FT);
  if (limit <= 0) {
    return 0;
  }
  return Math.min(Math.max(SPEND_STEP_FT, Math.round(ft)), limit);
}

/** One step up or down; "+" stops at what is left (the last step may be shorter than 5 ft, the server speaks whole feet). */
export function stepSpend(ft: number, direction: -1 | 1, leftFt: number): number {
  const limit = Math.min(Math.max(0, leftFt), SPEND_MAX_FT);
  if (limit <= 0) {
    return 0;
  }
  const next = direction === 1 ? ft + SPEND_STEP_FT : ft - SPEND_STEP_FT;
  return clampSpend(next, limit);
}

/** The plan of one amount on a combatant: all of it read from the server's `*_dft` numbers. */
export function spendPlan(c: Pick<Combatant, 'speedDft' | 'movementLeftDft' | 'movementLeftFt' | 'movementUsedDft'>, ft: number): SpendPlan {
  const limitFt = Math.max(0, c.movementLeftFt);
  const amount = clampSpend(ft, limitFt);
  const total = Math.max(1, c.speedDft);
  const afterDft = Math.max(0, c.movementLeftDft - amount * 10);
  return {
    ft: amount,
    limitFt,
    canLess: amount > SPEND_STEP_FT,
    canMore: amount < Math.min(limitFt, SPEND_MAX_FT),
    amount: metersFixed(amount),
    after: metersFixed(afterDft / 10),
    left: metersFixed(c.movementLeftDft / 10),
    total: metersFixed(total / 10),
    spent: metersFixed(c.movementUsedDft / 10),
    afterPercent: Math.max(0, Math.min(100, (afterDft / total) * 100)),
    leftPercent: Math.max(0, Math.min(100, (c.movementLeftDft / total) * 100)),
  };
}

/** "Restam 3,0 m", "Sem movimento". */
export function restamText(leftDft: number): string {
  return leftDft > 0 ? `Restam ${metersFixed(leftDft / 10)}` : 'Sem movimento';
}

/** "Gastar 6,0 m". */
export function spendLabel(plan: SpendPlan): string {
  return `Gastar ${plan.amount}`;
}

/** The line a sheet says before the amount: "Você tem 9,0 m neste turno". */
export function turnHas(total: string): string {
  return `Você tem ${total} neste turno`;
}

/** What each side of the order is: the party (players, their creatures, an NPC marked "Aliado") or the enemies. */
function sideOf(c: Combatant): 'party' | 'enemy' {
  if (c.kind === CombatantKind.NPC) {
    return c.side === CombatantSide.PARTY ? 'party' : 'enemy';
  }
  return 'party';
}

/** Whether two combatants are on opposite sides. */
export function opposed(a: Combatant, b: Combatant): boolean {
  return sideOf(a) !== sideOf(b);
}

/** The combatant on turn, for a one-combatant turn: `null` for the master's turn or a joint one. */
function onTurn(e: Encounter): Combatant | null {
  return e.combatants.find((c) => c.id === e.currentCombatantId) ?? null;
}

/** A row of the master's "Oferecer ataque de oportunidade": the ones the mover could have left the reach of. */
export interface ReactorRow {
  readonly id: string;
  readonly label: string;
  /** "Guerreiro 4", from the roster (the master's), or `''`. */
  readonly sub: string;
  readonly player: boolean;
  /** The reaction was used: the row says so and is off (what the server already told, not a rule). */
  readonly spent: boolean;
  /** An offer to this one already waits (one per reactor): the row says so and is off. */
  readonly offered: boolean;
  readonly hidden: boolean;
}

/** The people the mover can have left the reach of: the opposite side, standing, in the order the combat has them.
 * The server still has the last word (`NO_OPPORTUNITY`, `REACTION_USED`). */
export function reactorRows(e: Encounter, sub: (c: Combatant) => string): ReactorRow[] {
  const mover = onTurn(e);
  if (!mover) {
    return [];
  }
  return e.combatants
    // A character at 0 hit points cannot react: it is not offered.
    .filter((c) => c.id !== mover.id && !c.defeated && !isDown(c) && opposed(mover, c))
    .map((c) => ({
      id: c.id,
      label: c.label,
      sub: sub(c),
      player: c.kind === CombatantKind.PLAYER,
      spent: c.reactionUsed,
      offered: e.opportunityOffers.some((o) => o.moverId === mover.id && o.reactorId === c.id),
      hidden: c.hidden,
    }));
}

/** The targets whose cover the master marks: the opposite side of whoever is on turn (what a hit would be aimed at). In the master's
 * turn or a joint one nobody is the single mover, so everyone standing is listed. A defeated one is out. */
export function coverTargets(e: Encounter): Combatant[] {
  if (e.status !== EncounterStatus.ACTIVE) {
    return [];
  }
  const mover = onTurn(e);
  return e.combatants.filter((c) => !c.defeated && (!mover || (c.id !== mover.id && opposed(mover, c))));
}

/** Everyone standing, for the cover editor opened from the order's menu (the fallback for any combatant). */
export function everyoneStanding(e: Encounter): Combatant[] {
  return e.combatants.filter((c) => !c.defeated);
}

/** One degree the master can mark, with the line under it. */
export interface CoverChoice {
  readonly value: CoverDegree;
  readonly name: string;
  readonly sub: string;
  readonly mark: 'half' | 'three' | null;
}

/** The four rows (E10-04 state 4). The words are the app's; the numbers are the SRD's, the server applies them. */
export const COVER_CHOICES: readonly CoverChoice[] = [
  { value: CoverDegree.NONE, name: 'Sem cobertura', sub: '', mark: null },
  { value: CoverDegree.HALF, name: 'Meia cobertura', sub: '+2 na CA e em Destreza', mark: 'half' },
  { value: CoverDegree.THREE_QUARTERS, name: 'Três quartos', sub: '+5 na CA e em Destreza', mark: 'three' },
  { value: CoverDegree.TOTAL, name: 'Total (não dá para mirar)', sub: '', mark: null },
];

/** "Sem cobertura", "Meia cobertura: +2 na CA", "Três quartos: +5 na CA", "Total: não dá para mirar". */
export function coverLine(cover: CoverDegree): string {
  switch (cover) {
    case CoverDegree.HALF:
      return 'Meia cobertura: +2 na CA';
    case CoverDegree.THREE_QUARTERS:
      return 'Três quartos: +5 na CA';
    case CoverDegree.TOTAL:
      return 'Total: não dá para mirar';
    default:
      return 'Sem cobertura';
  }
}

/** The combat's mode as the bar's pill says it. */
export const THEATRE_PILL = 'Teatro da mente';
