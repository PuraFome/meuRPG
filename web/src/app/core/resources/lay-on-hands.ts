import type { ResourceTarget } from '../../../gen/meurpg/play/v1/combat_pb';
import { article } from '../combat/combat-log';
import { metersText } from '../units';
import { type Pool, pointsText } from './pools';
import { type ResourceRow, resourceRows } from './resource-targets';
import type { LayOnHandsEffect } from './resources-client';

/** What curing one disease or neutralizing one poison costs from the pool (SRD 5.1, Paladin). */
export const LAY_ON_HANDS_CURE_COST = 5;

/** The touch's reach in feet: the server lists who is within it; this is only for the words. */
export const LAY_ON_HANDS_REACH_FT = 5;

/** The rows of the target list: the creatures the server listed at the touch's reach, the paladin itself marked
 * "(você)". A creature beyond the reach is disabled with "Longe demais". Nothing here reads a creature type. */
export function touchTargetRows(targets: readonly ResourceTarget[], selfId: string): ResourceRow[] {
  return resourceRows(targets, LAY_ON_HANDS_REACH_FT, selfId);
}

/** The first amount the stepper shows: a cure's worth of points, or all that is left when there is less. */
export function firstAmount(pool: Pool): number {
  return Math.max(1, Math.min(pool.left, LAY_ON_HANDS_CURE_COST));
}

/** "Brisa recupera até 8 PV (não passa do máximo). Gasta 8: restam 9 de 25." */
export function healPreview(label: string, amount: number, pool: Pool): string {
  return `${label} recupera até ${amount} PV (não passa do máximo). Gasta ${amount}: restam ${pool.left - amount} de ${pool.total}.`;
}

/** "Gasta 5 da reserva (cada doença ou veneno custa 5, separados). Restam 12 de 25. Não cura PV." */
export function curePreview(pool: Pool): string {
  return `Gasta ${LAY_ON_HANDS_CURE_COST} da reserva (cada doença ou veneno custa ${LAY_ON_HANDS_CURE_COST}, separados). Restam ${pool.left - LAY_ON_HANDS_CURE_COST} de ${pool.total}. Não cura PV.`;
}

/** What the answer of a touch shows. */
export interface TouchResult {
  /** The pill: "Sem efeito", "Curou", "Veneno neutralizado". */
  readonly pill: string;
  /** Whether the touch did something: only for the icon. */
  readonly worked: boolean;
  readonly lines: readonly string[];
}

/** The answer of `UseLayOnHands`, in words. A touch that did nothing is always the same text, for any target: the
 * response does not say why and the app never guesses (RN-10). The hit points healed are only said when the server
 * sent them (it does not for an NPC: the number would say how many were missing). */
export function touchResult(
  answer: {
    readonly spent: number;
    readonly poolLeft: number;
    readonly healed?: number;
    readonly nothingHappened: boolean;
  },
  effect: LayOnHandsEffect,
  targetLabel: string,
  poolTotal: number,
): TouchResult {
  const who = `${article(targetLabel)} ${targetLabel}`;
  const left = `restam ${answer.poolLeft} de ${poolTotal}`;
  if (answer.nothingHappened) {
    return {
      pill: 'Sem efeito',
      worked: false,
      lines: [
        `Você tocou ${who} e gastou ${answer.spent} pontos da reserva (${left}). Nada acontece.`,
      ],
    };
  }
  if ('cure' in effect) {
    const poison = effect.cure === 'poison';
    return {
      pill: poison ? 'Veneno neutralizado' : 'Doença curada',
      worked: true,
      lines: [
        poison
          ? `Você neutralizou o veneno de ${targetLabel}.`
          : `Você curou a doença de ${targetLabel}.`,
        `Gastou ${pointsText(answer.spent)} da reserva: ${left}.`,
      ],
    };
  }
  return {
    pill: 'Curou',
    worked: true,
    lines: [
      answer.healed === undefined
        ? `Você tocou ${who}.`
        : `${targetLabel} recuperou ${answer.healed} PV.`,
      `Gastou ${pointsText(answer.spent)} da reserva: ${left}.`,
    ],
  };
}

/** The note under the list. */
export function reachNote(): string {
  return `Só quem está ao alcance do toque (${metersText(LAY_ON_HANDS_REACH_FT)}) pode ser escolhido.`;
}
