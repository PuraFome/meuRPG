import type { InspirationDie } from '../../../gen/meurpg/play/v1/combat_pb';
import { type Pool } from './pools';

/** The bard's voice reaches 60 feet (SRD 5.1, Bard): the server lists who is within it; this is for the words. */
export const BARDIC_INSPIRATION_REACH_FT = 60;

/** A round is 6 seconds, so a minute is 10 rounds and the die's 10 minutes are 100 rounds. */
export const ROUNDS_PER_MINUTE = 10;

/** "d8". */
export function inspirationDieName(sides: number): string {
  return `d${sides}`;
}

/** "até 8 min": the time the die has left, in whole minutes counted from the round the combat is in (rounded up: a
 * die with 3 rounds left is "até 1 min"); a die about to run out says so. */
export function expiresText(expiresAtRound: number, round: number): string {
  const rounds = expiresAtRound - round;
  if (rounds <= 0) {
    return 'acaba agora';
  }
  return `até ${Math.ceil(rounds / ROUNDS_PER_MINUTE)} min`;
}

/** The die card of the sheet and of the turn: "Inspiração de Bardo: d8" over "de Orla · até 8 min". */
export function dieCard(die: InspirationDie, round: number): { title: string; sub: string } {
  const from = die.fromLabel ? `de ${die.fromLabel} · ` : '';
  return {
    title: `Inspiração de Bardo: ${inspirationDieName(die.sides)}`,
    sub: `${from}${expiresText(die.expiresAtRound, round)}`,
  };
}

/** "Usar a Inspiração de Bardo (d8)?" */
export function promptTitle(die: InspirationDie | undefined): string {
  return `Usar a Inspiração de Bardo (${inspirationDieName(die?.sides ?? 0)})?`;
}

/** The label of the "use" button: "Somar o d8 (Inspiração de Orla)". */
export function useLabel(die: InspirationDie | undefined): string {
  const name = inspirationDieName(die?.sides ?? 0);
  return die?.fromLabel ? `Somar o ${name} (Inspiração de ${die.fromLabel})` : `Somar o ${name}`;
}

/** "1 uso: restam 2 de 3", for the cost tile of the give dialog. */
export function useCost(uses: Pool): string {
  return `1 uso: restam ${Math.max(0, uses.left - 1)} de ${uses.total}`;
}
