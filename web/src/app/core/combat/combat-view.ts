import { CharacterKind } from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  type Combatant,
  CombatantKind,
  CombatantState,
  type Encounter,
} from '../../../gen/meurpg/play/v1/combat_pb';

/**
 * What the combat screens say about an encounter, kept as small pure
 * functions: the words, the formula, the tie groups, whose turn comes next
 * and what the player's banner reads. Tested without a DOM.
 */

/** How hurt an NPC is, as a word a player may read (RN-20). */
export function stateWord(state: CombatantState): string {
  switch (state) {
    case CombatantState.UNHURT:
      return 'Ileso';
    case CombatantState.HURT:
      return 'Ferido';
    case CombatantState.BADLY_HURT:
      return 'Muito ferido';
    case CombatantState.DEFEATED:
      return 'Derrotado';
    default:
      return '';
  }
}

/** The letter or letters on a token: the first letter, plus the number of a
 * numbered copy ("Goblin 2" is "G2"). */
export function combatantInitial(label: string): string {
  const letter = label.trim().charAt(0).toUpperCase();
  const copy = /\s(\d+)$/.exec(label.trim());
  return copy ? `${letter}${copy[1]}` : letter;
}

export function isPlayer(c: Combatant): boolean {
  return c.kind === CombatantKind.PLAYER;
}

/** The initiative as the master reads it: `1d20 (15) + 4 = 19`. `null`
 * until it is rolled. */
export function initiativeFormula(c: Combatant): string | null {
  if (c.initiative === undefined || c.initiativeFace === undefined) {
    return null;
  }
  const bonus = c.initiativeBonus ?? 0;
  const sign = bonus < 0 ? '−' : '+';
  return `1d20 (${c.initiativeFace}) ${sign} ${Math.abs(bonus)} = ${c.initiative}`;
}

/** The roll still to come: `1d20 + 2 = —`. */
export function pendingFormula(c: Combatant): string {
  const bonus = c.initiativeBonus ?? 0;
  return `1d20 ${bonus < 0 ? '−' : '+'} ${Math.abs(bonus)} = —`;
}

/** A bonus with its sign: "+2", "−1", "+0". */
export function signed(n: number): string {
  return `${n < 0 ? '−' : '+'}${Math.abs(n)}`;
}

/** The runs of combatants tied on the same total and bonus that the master
 * has not ordered yet (`tie_unresolved`), each as the IDs in their order. */
export function tieGroups(combatants: readonly Combatant[]): string[][] {
  const groups: string[][] = [];
  let last: Combatant | null = null;
  for (const c of combatants) {
    if (!c.tieUnresolved) {
      last = null;
      continue;
    }
    if (last && last.initiative === c.initiative && last.initiativeBonus === c.initiativeBonus) {
      groups[groups.length - 1].push(c.id);
    } else {
      groups.push([c.id]);
    }
    last = c;
  }
  return groups;
}

/** A group's IDs with `id` moved one place up or down; the same list when it
 * is already at that end (the arrows stay inside the group). */
export function moveInGroup(group: readonly string[], id: string, direction: -1 | 1): string[] {
  const from = group.indexOf(id);
  const to = from + direction;
  const next = [...group];
  if (from < 0 || to < 0 || to >= group.length) {
    return next;
  }
  [next[from], next[to]] = [next[to], next[from]];
  return next;
}

/** "Empate em 12: Goblin 1 e Goblin 2. Escolha a ordem com as setas." */
export function tieSentence(labels: readonly string[], total: number | undefined): string {
  const names =
    labels.length <= 2
      ? labels.join(' e ')
      : `${labels.slice(0, -1).join(', ')} e ${labels[labels.length - 1]}`;
  return `Empate em ${total}: ${names}. Escolha a ordem com as setas.`;
}

/** The combatants who still have no initiative. */
export function missingInitiative(combatants: readonly Combatant[]): Combatant[] {
  return combatants.filter((c) => c.initiative === undefined);
}

/** The one on turn, or `null` (SETUP, ENDED, or a hidden one for a player). */
export function currentCombatant(e: Encounter): Combatant | null {
  return e.combatants.find((c) => c.id === e.currentCombatantId) ?? null;
}

/** Who plays after the current one: the next that is not defeated, going
 * round the order. `null` when the current is the only one, or nobody is on
 * turn (a player never knows about a hidden one in between). */
export function nextCombatant(e: Encounter): Combatant | null {
  const list = e.combatants;
  const at = list.findIndex((c) => c.id === e.currentCombatantId);
  if (at < 0) {
    return null;
  }
  for (let step = 1; step < list.length; step++) {
    const c = list[(at + step) % list.length];
    if (!c.defeated) {
      return c;
    }
  }
  return null;
}

/** The turn banner of the combat bar (master) and of the player's screen. */
export interface TurnBanner {
  /** "Vez do Capitão Goblin", "Vez do mestre" or "Sua vez, Pensantus". */
  readonly title: string;
  /** Who is on turn; `null` for the master's turn (no name, no token). */
  readonly who: Combatant | null;
  readonly mine: boolean;
  readonly masterTurn: boolean;
  /** "Em seguida: Pensantus" (master) or the player's "Você é o próximo". */
  readonly next: Combatant | null;
  /** Whether the next one is the caller's own combatant. */
  readonly nextIsMine: boolean;
}

export function turnBanner(e: Encounter): TurnBanner {
  if (e.masterTurn) {
    return {
      title: 'Vez do mestre',
      who: null,
      mine: false,
      masterTurn: true,
      next: null,
      nextIsMine: false,
    };
  }
  const who = currentCombatant(e);
  const next = nextCombatant(e);
  return {
    title: who ? (who.mine ? `Sua vez, ${who.label}` : `Vez do ${who.label}`) : 'Ninguém está na vez',
    who,
    mine: who?.mine ?? false,
    masterTurn: false,
    next,
    nextIsMine: next?.mine ?? false,
  };
}

/** The player's own combatant, if the master put it in the combat. */
export function ownCombatant(e: Encounter): Combatant | null {
  return e.combatants.find((c) => c.mine) ?? null;
}

/** "Rodada 2". */
export function roundLabel(round: number): string {
  return `Rodada ${round}`;
}

/** An NPC's kind as the lists say it ("Inimigo", "Minion"…); empty for a
 * player's character. */
export function npcKindLabel(kind: CharacterKind): string {
  switch (kind) {
    case CharacterKind.ENEMY:
      return 'Inimigo';
    case CharacterKind.BOSS:
      return 'Boss';
    case CharacterKind.MINION:
      return 'Minion';
    case CharacterKind.STORY:
      return 'NPC de história';
    default:
      return '';
  }
}
