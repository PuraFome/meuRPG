import {
  type Combatant,
  CombatantKind,
  type Encounter,
  type OpportunityOffer,
} from '../../../gen/meurpg/play/v1/combat_pb';
import type { Attack } from '../../../gen/meurpg/rules/v1/rules_pb';
import { joinDots, tight } from '../format/text';
import { article } from './combat-log';
import { attackName, attackTitle, damageText } from './combat-options';
import { listNames } from './joint-turn';
import { ofThe } from './move-plan';

/**
 * Opportunity attacks on screen (MR-034, RN-21, E9-13): what the server's
 * `Encounter.opportunity_offers` say, in words. The server detects the exit
 * from a reach, makes the offer and decides who may answer; the browser reads
 * the list it was sent (a player gets only the offers on their own character or
 * that they answer, and never a reactor they do not see) and never works out
 * who provokes whom. Pure functions, tested without a DOM.
 */

/** The offers the caller answers: the reactor's controller, and the master for all. */
export function offersToAnswer(e: Encounter): OpportunityOffer[] {
  return e.opportunityOffers.filter((o) => o.forYou);
}

/** The offers that hold a combatant's turn: it moved, and each is not answered yet. */
export function offersOn(e: Encounter, moverId: string): OpportunityOffer[] {
  return e.opportunityOffers.filter((o) => o.moverId === moverId);
}

function combatant(e: Encounter, id: string): Combatant | undefined {
  return id ? e.combatants.find((c) => c.id === id) : undefined;
}

/** Whether the reactor is the master's (an NPC, not a player's character or its creature). */
export function reactorIsMasters(e: Encounter, offer: OpportunityOffer): boolean {
  const reactor = combatant(e, offer.reactorId);
  return !reactor || reactor.kind === CombatantKind.NPC;
}

/** What the waiting mover reads: the title (a live region) and the line under it.
 * A reactor they do not see is not named: "Esperando o mestre". */
export function waitingText(e: Encounter, offers: readonly OpportunityOffer[]): { title: string; detail: string } | null {
  if (offers.length === 0) {
    return null;
  }
  const seen = offers.filter((o) => o.reactorId !== '');
  const unseen = offers.length - seen.length;
  const trail = 'Seu movimento já valeu; a sua vez continua quando responderem.';
  if (seen.length === 0) {
    return { title: 'Esperando o mestre', detail: `Seu movimento já valeu; a sua vez continua quando ele responder.` };
  }
  const masters = seen.filter((o) => reactorIsMasters(e, o));
  const players = seen.filter((o) => !reactorIsMasters(e, o));
  const who = [
    ...(masters.length > 0 || unseen > 0 ? ['do mestre'] : []),
    ...players.map((o) => `do jogador de ${o.reactorLabel}`),
  ];
  const names = listNames(masters.map((o) => `${article(o.reactorLabel)} ${o.reactorLabel}`));
  const detail =
    masters.length > 0
      ? `${capitalize(names)} ${masters.length === 1 ? 'pode fazer um ataque' : 'podem fazer ataques'} de oportunidade. ${trail}`
      : trail;
  return { title: `Esperando a reação ${listNames(who)}`, detail };
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** "Esperando a sua reação: Goblin 2" on the master's bar, for the offers he answers for NPCs; or
 * who he waits on when the reactor is a player's ("Esperando a reação do Caio (Toren)"). */
export function barText(e: Encounter, playerName: (characterId: string) => string): string {
  const own = offersToAnswer(e).filter((o) => reactorIsMasters(e, o));
  if (own.length > 0) {
    return `Esperando a sua reação: ${listNames(own.map((o) => o.reactorLabel))}`;
  }
  const other = offersToAnswer(e)[0];
  if (!other) {
    return '';
  }
  const who = combatant(e, other.reactorId);
  const name = who ? playerName(who.characterId) : '';
  return `Esperando a reação ${name ? `${ofThe([name])} (${other.reactorLabel})` : `do jogador de ${other.reactorLabel}`}`;
}

/** The player's question: "O Goblin 2 está saindo do seu alcance. Ataque de oportunidade?" */
export function playerQuestion(offer: OpportunityOffer): string {
  return `${capitalize(`${article(offer.moverLabel)} ${offer.moverLabel}`)} está saindo do seu alcance. Ataque de oportunidade?`;
}

/** The master's: "O Toren saiu do alcance do Goblin 2." */
export function masterNews(offer: OpportunityOffer): string {
  return `${capitalize(`${article(offer.moverLabel)} ${offer.moverLabel}`)} saiu do alcance ${ofThe([offer.reactorLabel])}.`;
}

/** "Goblin 2 ataca o Toren?" */
export function masterAsk(offer: OpportunityOffer): string {
  return `${offer.reactorLabel} ataca ${article(offer.moverLabel)} ${offer.moverLabel}?`;
}

/** An attack of the offer, with what the sheet knows of it. */
export interface ReactorAttack {
  readonly key: string;
  readonly name: string;
  /** "Cimitarra +4 · 1d6 + 2 cortante". */
  readonly detail: string;
  readonly attack: Attack | null;
}

/** The offer's attacks with their numbers, from the reactor's options (its own
 * `GetTurnOptions`: the player's, or the master's read of an NPC). An attack the
 * options do not list is still offered by name. */
export function reactorAttacks(offer: OpportunityOffer, attacks: readonly Attack[]): ReactorAttack[] {
  return offer.attacks.map((a) => {
    const attack = attacks.find((x) => x.key === a.key) ?? null;
    return {
      key: a.key,
      name: a.namePt || a.key,
      detail: attack ? tight(joinDots([attackTitle(attack), damageText(attack)])) : a.namePt,
      attack,
    };
  });
}

/** "Atacar com Espada longa". */
export function attackLabel(a: ReactorAttack): string {
  return `Atacar com ${a.attack ? attackName(a.attack) : a.name}`;
}
