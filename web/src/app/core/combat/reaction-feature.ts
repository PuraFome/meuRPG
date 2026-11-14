import {
  type Encounter,
  ReactionKind,
  type ReactionWindow,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { FeatureTriggerKind } from '../../../gen/meurpg/rules/v1/rules_pb';
import { article } from '../format/article';
import { metersText } from '../units';

/**
 * The window of a reaction feature whose trigger happened (#743, `ReactionKind.FEATURE`): a creature started its turn near the
 * user, or came near, and the user's player uses the feature on it or does not. The window has no prompt of its own: the
 * creature, the feature, the distance and the moment are in `ReactionWindow.trigger`. Pure functions, tested without a DOM.
 */

/** Whether the window is a feature window. */
export function isFeatureWindow(w: ReactionWindow): boolean {
  return w.kind === ReactionKind.FEATURE;
}

/** The feature's own name, from the trigger; "Reação" when the server wrote none. */
export function featureWindowName(w: ReactionWindow): string {
  return w.trigger?.actionNamePt || 'Reação';
}

/** "começou a vez" or "chegou": what the creature did, with the distance after it when the window has one. */
function whatItDid(w: ReactionWindow): string {
  return w.trigger?.featureTrigger === FeatureTriggerKind.ENEMY_STARTS_TURN_WITHIN
    ? 'começou a vez'
    : 'chegou';
}

/** "a 1,5 m", or `''` when the window has no distance. */
function howFar(w: ReactionWindow): string {
  const feet = w.trigger?.distanceFt;
  return feet === undefined ? '' : `a ${metersText(feet)}`;
}

/** "O Zumbi 1 chegou a 1,5 m de você." (the player's own words): "chegou perto de você" when there is no distance. */
function leadText(w: ReactionWindow): string {
  const label = w.trigger?.actorLabel ?? '';
  const who = `${article(label) === 'a' ? 'A' : 'O'} ${label}`;
  const far = howFar(w);
  const did = whatItDid(w);
  return far
    ? `${who} ${did} ${far} de você.`
    : `${who} ${did === 'chegou' ? 'chegou perto' : did} de você.`;
}

/** What the master reads of the same moment, for his row: "o Hobgoblin começou a vez a 1,5 m de Pensantus". */
export function featureWindowFact(w: ReactionWindow): string {
  const label = w.trigger?.actorLabel ?? '';
  const far = howFar(w);
  return [`${article(label)} ${label}`, whatItDid(w), far, `de ${w.reactorLabel}`]
    .filter(Boolean)
    .join(' ');
}

/** "Hobgoblin começou a vez": the subtitle of the master's group. */
export function featureWindowGroup(w: ReactionWindow): string {
  return `${w.trigger?.actorLabel ?? 'Uma criatura'} ${whatItDid(w)}`;
}

/**
 * What the master reads on the question that came to him: the feature's cost, and why it is his. A hidden creature's window is
 * his from the start (the player does not see the creature); any other waited 10 s for the player.
 */
export function featureLateCost(e: Encounter, w: ReactionWindow, lateSeconds: number): string {
  const actor = e.combatants.find((c) => c.id === w.trigger?.actorId);
  const spent = 'Gasta a reação e um uso.';
  if (actor?.hidden) {
    return `${spent} ${article(actor.label) === 'a' ? 'A' : 'O'} ${actor.label} está escondid${article(actor.label)}: o jogador de ${w.reactorLabel} não ${article(actor.label)} vê, e você responde por ele.`;
  }
  return `${spent} O jogador não respondeu em ${lateSeconds} s; você responde por ele.`;
}

/** What the player's prompt says, ready to draw (board 743-02). */
export interface FeatureWindowView {
  readonly title: string;
  /** "Zumbi 1 · Rodada 2". */
  readonly subtitle: string;
  /** "O Zumbi 1 chegou a 1,5 m de você." (bold) */
  readonly lead: string;
  /** "Usar o Halo de esporos?" */
  readonly question: string;
  /** What it costs and who resolves it. */
  readonly note: string;
  readonly declineLabel: string;
  readonly ariaLabel: string;
  /** The button's words for the creature it is used on: "Usar no Zumbi 1". */
  readonly useLabel: (creature: string) => string;
}

export function featureWindowView(w: ReactionWindow, round: number): FeatureWindowView {
  const name = featureWindowName(w);
  const lead = leadText(w);
  const question = `Usar ${article(name)} ${name}?`;
  return {
    title: name,
    subtitle: `${w.trigger?.actorLabel ?? ''} · Rodada ${round}`,
    lead,
    question,
    note: 'Gasta a sua reação: ela volta no começo da sua próxima vez. O app registra o uso; o mestre resolve o resto.',
    declineLabel: 'Não usar',
    ariaLabel: `${lead.replace(/\.$/, '')}: ${question.charAt(0).toLowerCase()}${question.slice(1)}`,
    useLabel: (creature) => `Usar ${article(creature) === 'a' ? 'na' : 'no'} ${creature}`,
  };
}
