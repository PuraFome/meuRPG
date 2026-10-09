import {
  type Combatant,
  type GetMoveOptionsResponse,
  type ReachableSquare,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { Square } from './combat-grid';
import { whoIs, named } from './contest-view';
import { metersFixed } from '../units';

/**
 * Moving with a grappled creature (W7-X, board W7-Xb 6): the mover drags the creature it holds, the speed is halved unless it
 * is two or more sizes smaller (SRD 5.1, Moving a Grappled Creature), and the creature ends on the square the mover left just
 * before the last square of its path. The server works the halving and the square out (`GetMoveOptions.dragging_halved`,
 * `dragged_to`, `movement_left_dft` already halved); here only the words are written.
 */

/** Who the mover drags: the combatant, or `null` when it drags nobody. */
export function dragged(
  options: GetMoveOptionsResponse | null,
  combatants: readonly Combatant[],
): Combatant | null {
  const id = options?.draggingCombatantId ?? '';
  return id ? (combatants.find((c) => c.id === id) ?? null) : null;
}

/** "Deslocamento: 4,5 m (metade, arrastando o Hobgoblin)". */
export function dragMovementLine(
  leftDft: number,
  halved: boolean,
  who: Pick<Combatant, 'label' | 'kind'>,
): string {
  const name = named(whoIs(who));
  return `Deslocamento: ${metersFixed(leftDft / 10)} (${halved ? 'metade, ' : ''}arrastando ${name})`;
}

/** "Onde o Hobgoblin termina". */
export function dragLegend(who: Pick<Combatant, 'label' | 'kind'>): string {
  return `Onde ${named(whoIs(who))} termina`;
}

/** The compass word of a move from one square to another, the rows growing southwards: "oeste", "noroeste". */
export function compass(from: Square, to: Square): string {
  const dx = to.col - from.col;
  const dy = to.row - from.row;
  // A direction is the dominant axis, or both when neither is less than half the other.
  const east = Math.abs(dx) * 2 > Math.abs(dy);
  const south = Math.abs(dy) * 2 > Math.abs(dx);
  if (south && east) {
    if (dy < 0) {
      return dx < 0 ? 'noroeste' : 'nordeste';
    }
    return dx < 0 ? 'sudoeste' : 'sudeste';
  }
  if (south) {
    return dy < 0 ? 'norte' : 'sul';
  }
  return dx < 0 ? 'oeste' : 'leste';
}

/** The number of squares of a move: the longer side, as the grid counts (a diagonal square is one). */
export function squaresOf(from: Square, to: Square): number {
  return Math.max(Math.abs(to.col - from.col), Math.abs(to.row - from.row));
}

/**
 * "Você anda 2 casas para o oeste; o Hobgoblin, que você agarra, vem atrás e para na casa que você deixou. Cabem até 4,5 m neste
 * turno." The path and where the creature ends are the server's; the sentence only says them.
 */
export function dragSentence(
  from: Square,
  to: Square,
  who: Pick<Combatant, 'label' | 'kind'>,
  leftDft: number,
): string {
  const n = squaresOf(from, to);
  const name = named(whoIs(who));
  const walk = n === 1 ? 'Você anda 1 casa' : `Você anda ${n} casas`;
  return `${walk} para o ${compass(from, to)}; ${name}, que você agarra, vem atrás e para na casa que você deixou. Cabem até ${metersFixed(leftDft / 10)} neste turno.`;
}

/** The square the dragged creature ends on if the mover goes to `square`, or `null`. */
export function draggedTo(square: ReachableSquare | undefined): Square | null {
  return square?.draggedTo ? { col: square.draggedTo.col, row: square.draggedTo.row } : null;
}
