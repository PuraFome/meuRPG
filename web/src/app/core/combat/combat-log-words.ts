import { article } from '../format/article';
import { distanceText } from '../units';
import { LogContext } from './combat-log-types';

/** "no Toren", "na Brisa". */
export function inThe(name: string): string {
  return `${article(name) === 'a' ? 'na' : 'no'} ${name}`;
}

/** "o Goblin 1", "a Brisa". */
export function the(name: string): string {
  return `${article(name)} ${name}`;
}

/** "o Goblin 1", "Brisa": a creature has an article, a player's character does not. */
export function subject(label: string, ctx: LogContext): string {
  return ctx.players.has(label) ? label : the(label);
}

/** "do Hobgoblin", "de Brisa". */
export function ofSubject(label: string, ctx: LogContext): string {
  return ctx.players.has(label)
    ? `de ${label}`
    : `${article(label) === 'a' ? 'da' : 'do'} ${label}`;
}

/**
 * What the log says of a push after a failed save: "empurrado 3 m · 2 quadrados", "empurrado 1,5 m · 1 quadrado e parou
 * antes: parede, porta, borda do mapa ou criatura", "não saiu do lugar: parede, porta, borda do mapa ou criatura", or, in a
 * combat with no map, that the table places the creature. Empty when nobody was pushed.
 */
export function pushText(push: {
  pushedFt: number;
  pushStopped: boolean;
  pushNoMap: boolean;
}): string {
  if (push.pushNoMap) {
    return 'o mestre posiciona a criatura, porque o combate não tem mapa';
  }
  const blockers = 'parede, porta, borda do mapa ou criatura';
  if (push.pushedFt > 0) {
    return `empurrado ${distanceText(push.pushedFt)}${push.pushStopped ? ` e parou antes: ${blockers}` : ''}`;
  }
  return push.pushStopped ? `não saiu do lugar: ${blockers}` : '';
}
