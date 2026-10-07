import {
  type Combatant,
  CombatantKind,
  type Encounter,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { article } from './combat-log';

/**
 * How the table names a player's creatures (MR-037, E9-12): "Lobo atroz 1", "Lobos atrozes (2)",
 * "da Sálvia". Words only: what a creature is and whose it is come from the server
 * (`monster_name_pt`, `owner_character_id`); nothing here decides a rule.
 */

/** The words that stay as they are in a plural name ("Cobra de pedra" is "Cobras de pedra"). */
const FIXED = new Set(['de', 'da', 'do', 'das', 'dos', 'e', 'em', 'com', 'sem']);

/** The plural of one word: "Lobo" is "Lobos", "atroz" "atrozes", "Rato" "Ratos", "Porco-espinho" "Porcos-espinhos". */
function pluralWord(word: string): string {
  const lower = word.toLowerCase();
  if (FIXED.has(lower) || /\d/.test(word) || /s$/i.test(word) || /x$/i.test(word)) {
    return word;
  }
  if (/[rz]$/i.test(word)) {
    return `${word}es`;
  }
  if (/m$/i.test(word)) {
    return `${word.slice(0, -1)}ns`;
  }
  if (/ão$/i.test(word)) {
    return `${word.slice(0, -2)}ões`;
  }
  if (/l$/i.test(word)) {
    return `${word.slice(0, -1)}is`;
  }
  return `${word}s`;
}

/** The plural of a creature's name, word by word up to a preposition: "Lobo atroz" is "Lobos atrozes", "Cobra de pedra" is "Cobras de pedra". */
export function pluralName(name: string): string {
  let after = false;
  return name
    .trim()
    .split(/\s+/)
    .map((w) => {
      if (after || FIXED.has(w.toLowerCase())) {
        after = true;
        return w;
      }
      return w.split('-').map(pluralWord).join('-');
    })
    .join(' ');
}

export function isCreature(c: Combatant): boolean {
  return c.kind === CombatantKind.CREATURE;
}

/** The combatant a creature belongs to: its owner's, in the same combat. */
export function ownerOf(e: Encounter, c: Combatant): Combatant | null {
  return c.ownerCharacterId
    ? (e.combatants.find(
        (x) => x.characterId === c.ownerCharacterId && x.kind === CombatantKind.PLAYER,
      ) ?? null)
    : null;
}

/** "da Sálvia", "do Pensantus": the owner for a line like "Fera · CA 14 · da Sálvia". */
export function ofOwner(e: Encounter, c: Combatant): string {
  const owner = ownerOf(e, c);
  return owner ? `${article(owner.label) === 'a' ? 'da' : 'do'} ${owner.label}` : '';
}

/** The kind of a creature as its line says it: the book's name when it is not already the combatant's own name. */
export function kindWord(c: Combatant): string {
  const base = c.label.replace(/\s+\d+$/, '');
  return c.monsterNamePt && c.monsterNamePt.toLowerCase() !== base.toLowerCase()
    ? c.monsterNamePt
    : 'Criatura';
}

/** What a group of creatures is called: "Lobos atrozes" when they are all of one kind, else "Criaturas"; a creature alone keeps its name. */
export function groupName(members: readonly Combatant[]): string {
  if (members.length === 1) {
    return members[0].label;
  }
  const keys = new Set(members.map((m) => m.monsterKey));
  const first = members[0];
  return keys.size === 1 && first.monsterNamePt ? pluralName(first.monsterNamePt) : 'Criaturas';
}

/** Whether a group's name is feminine ("Cobras", "Criaturas") for "das suas" / "dos seus". */
export function groupFeminine(members: readonly Combatant[]): boolean {
  if (members.length === 1) {
    return article(members[0].label) === 'a';
  }
  const keys = new Set(members.map((m) => m.monsterKey));
  const first = (members[0].monsterNamePt || members[0].label).split(/\s+/)[0];
  return keys.size > 1 || article(first) === 'a';
}

/**
 * The title of a creature group's turn: "Vez dos seus Lobos atrozes" for their player, "Vez dos Lobos
 * atrozes da Sálvia" for the others, "Vez do seu Nanquim" and "Vez do Nanquim" for one creature.
 */
export function creatureTurnTitle(
  e: Encounter,
  members: readonly Combatant[],
  mine: boolean,
): string {
  const name = groupName(members);
  const several = members.length > 1;
  const f = groupFeminine(members);
  const owner = ofOwner(e, members[0]);
  if (mine) {
    return several
      ? `Vez ${f ? 'das suas' : 'dos seus'} ${name}`
      : `Vez ${f ? 'da sua' : 'do seu'} ${name}`;
  }
  if (several) {
    return `Vez ${f ? 'das' : 'dos'} ${name}${owner ? ` ${owner}` : ''}`;
  }
  return `Vez ${f ? 'da' : 'do'} ${name}`;
}

/** "Encerrar a parte dos Lobos" for a group (a joint turn: the part of each member), "Encerrar a vez do Nanquim" for one creature. */
export function endLabel(members: readonly Combatant[]): string {
  if (members.length > 1) {
    return `Encerrar a parte ${groupFeminine(members) ? 'das' : 'dos'} ${groupName(members).split(' ')[0]}`;
  }
  return `Encerrar a vez ${groupFeminine(members) ? 'da' : 'do'} ${members[0].label}`;
}

/**
 * What the part about to end still has, for its question: "Os 2 Lobos ainda têm ação e movimento.", "O Nanquim ainda tem
 * movimento.". It is read from the creatures' own economy (the server's numbers); empty when nothing is left.
 */
export function partLeftSentence(acting: readonly Combatant[]): string {
  if (acting.length === 0) {
    return '';
  }
  const items = [
    ...(acting.some((m) => !m.actionUsed) ? ['ação'] : []),
    ...(acting.some((m) => m.movementLeftDft > 0) ? ['movimento'] : []),
  ];
  if (items.length === 0) {
    return '';
  }
  const who =
    acting.length === 1
      ? `${article(acting[0].label) === 'a' ? 'A' : 'O'} ${acting[0].label}`
      : `${groupFeminine(acting) ? 'As' : 'Os'} ${acting.length} ${groupName(acting).split(' ')[0]}`;
  return `${who} ainda ${acting.length === 1 ? 'tem' : 'têm'} ${items.join(' e ')}.`;
}
