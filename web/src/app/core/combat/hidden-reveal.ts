import type { Encounter, HiddenRevealQuestion } from '../../../gen/meurpg/play/v1/combat_pb';
import { tieNumbers } from '../format/text';
import { listNames } from './joint-turn';

/**
 * The turn held by a question about hidden creatures (PM-02c, RN-10, RN-20): a player's area spell hit creatures the
 * players do not see, the table asks the master whether they appear, and the turn waits for his answer.
 *
 * - A player reads only `Encounter.turn_held`, the same "Esperando o mestre" as any other wait for the master: nothing
 *   says why, how many questions there are, or that a spell hit anyone (`heldWait`).
 * - The master reads `Encounter.pending_hidden_reveals`, oldest first: the bar says what waits, "Próximo turno" says why
 *   it cannot pass, and each question is a card he answers in order.
 *
 * Pure functions, tested without a DOM.
 */

/** What a player reads while the turn waits for the master: the turn of their own, or someone else's. */
export function heldWait(
  e: Encounter,
  ownTurn: boolean,
): { readonly title: string; readonly detail: string } | null {
  if (!e.turnHeld) {
    return null;
  }
  return {
    title: 'Esperando o mestre',
    detail: ownTurn
      ? 'A sua vez continua quando ele responder.'
      : 'O turno continua quando ele responder.',
  };
}

/** The questions the master answers, oldest first (what the server sends him; a player's list is always empty). */
export function questions(e: Encounter): readonly HiddenRevealQuestion[] {
  return e.pendingHiddenReveals ?? [];
}

/**
 * "Área da última magia" (PM-02c 9): the squares and the origin of the oldest question that waits, drawn on the master's map.
 * Nothing for a player (his list is empty) or with no question.
 */
export function lastArea(e: Encounter): {
  readonly origin: { col: number; row: number } | null;
  readonly squares: readonly { col: number; row: number }[];
} | null {
  const area = questions(e)[0]?.area;
  if (!area?.squares.length) {
    return null;
  }
  return {
    origin: area.origin ? { col: area.origin.col, row: area.origin.row } : null,
    squares: area.squares.map((s) => ({ col: s.col, row: s.row })),
  };
}

/** The master's bar: "Esperando a sua resposta: escondidas atingidas", or how many questions wait. */
export function revealBarText(e: Encounter): string {
  const n = questions(e).length;
  if (n === 0) {
    return '';
  }
  if (n === 1) {
    return questions(e)[0].combatantIds.length === 0
      ? 'Esperando a sua resposta: magia de área'
      : 'Esperando a sua resposta: escondidas atingidas';
  }
  return `Esperando a sua resposta: ${n} perguntas de escondidas`;
}

/** Why "Próximo turno" cannot be pressed while questions wait. */
export function revealWhy(e: Encounter): string {
  const n = questions(e).length;
  if (n === 0) {
    return '';
  }
  return n === 1
    ? 'Responda ao pedido abaixo para seguir.'
    : 'Responda aos pedidos abaixo para seguir.';
}

/** "Bola de Fogo atingiu 2 criaturas escondidas". */
export function questionTitle(spell: string, hits: number): string {
  if (hits === 0) {
    // "Perguntar a cada vez" holds every area spell a player casts, so that the wait never says a hidden creature was there.
    return tieNumbers(`${spell}: nenhuma criatura escondida na área`);
  }
  return tieNumbers(
    `${spell} atingiu ${hits} ${hits === 1 ? 'criatura escondida' : 'criaturas escondidas'}`,
  );
}

/** The names of the hidden creatures a question is about, as the combat names them (the master's copy has them). */
export function hitNames(e: Encounter, q: HiddenRevealQuestion): string[] {
  return q.combatantIds.map((id) => e.combatants.find((c) => c.id === id)?.label ?? 'Criatura');
}

/** "Goblin 3 e Goblin 4". */
export function namesText(names: readonly string[]): string {
  return tieNumbers(listNames(names));
}

/** "Pergunta 1 de 2 · responda esta primeiro", "Pergunta 2 de 2 · depois da primeira"; nothing with only one. */
export function questionKicker(index: number, total: number): string {
  if (total < 2) {
    return '';
  }
  const where = index === 0 ? 'responda esta primeiro' : 'depois da primeira';
  return `Pergunta ${index + 1} de ${total} · ${where}`;
}
