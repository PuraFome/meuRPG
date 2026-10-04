import { timestampDate } from '@bufbuild/protobuf/wkt';

import type { Milestone, XPAward } from '../../../gen/meurpg/progression/v1/progression_pb';
import { formatDayAt } from '../../shared/session-time/session-time';
import { nameList } from './xp-labels';

/** The longest milestone name the server takes (`progression.proto`). */
export const MILESTONE_MAX = 120;

/** The most milestones a campaign holds, the reached ones included. */
export const MILESTONES_LIMIT = 100;

/** The milestones still to come, in the master's order. A player never gets
 * one (`ListMilestones` leaves them out); the filter is for the master's. */
export function plannedOf(list: readonly Milestone[]): Milestone[] {
  return list.filter((m) => !m.reached && !m.offList);
}

/** The reached milestones, the last one reached first: what the master and the
 * players read as "Marcos alcançados". */
export function reachedOf(list: readonly Milestone[]): Milestone[] {
  return list
    .filter((m) => m.reached)
    .sort((a, b) => reachedTime(b) - reachedTime(a));
}

function reachedTime(m: Milestone): number {
  return m.reachedAt ? timestampDate(m.reachedAt).getTime() : 0;
}

/** "03/10 às 22:05": when the milestone was reached. */
export function reachedWhen(m: Milestone): string {
  return m.reachedAt ? formatDayAt(timestampDate(m.reachedAt)) : '';
}

/** The characters (by id) that have the milestone, through any of its marks. */
export function markedIds(m: Milestone): Set<string> {
  return new Set(m.marks.flatMap((a) => a.shares.map((s) => s.characterId)));
}

/** Who has it, in the order they got it: "Pensantus, Toren e Brisa". */
export function markedNames(m: Milestone): string {
  return nameList(m.marks.flatMap((a) => a.shares.map((s) => s.characterName)));
}

function givenBy(award: XPAward): string {
  return award.givenByDisplayName.trim() || 'O mestre';
}

/** The master's lines under a reached milestone: "Samuel marcou Pensantus e
 * Toren", then one line for each "Dar a mais alguém": "Samuel deu a Brisa ·
 * 04/10 às 19:30". */
export function markLines(m: Milestone): string[] {
  return m.marks.map((a, i) => {
    const names = nameList(a.shares.map((s) => s.characterName));
    if (i === 0) {
      return `${givenBy(a)} marcou ${names}`;
    }
    const when = a.createdAt ? ` · ${formatDayAt(timestampDate(a.createdAt))}` : '';
    return `${givenBy(a)} deu a ${names}${when}`;
  });
}

/** The player's line: "Subiram de nível: Pensantus e Toren" (everyone reads
 * who, as in the XP history, question 50). */
export function leveledLine(m: Milestone): string {
  const ids = markedIds(m);
  return `${ids.size > 1 ? 'Subiram' : 'Subiu'} de nível: ${markedNames(m)}`;
}

/** "3 marcos · só você vê", "1 marco · só você vê". */
export function plannedCount(n: number): string {
  return `${n} ${n === 1 ? 'marco' : 'marcos'} · só você vê`;
}

/** What the master reads right after marking: "Marco alcançado: Pensantus e
 * Toren podem subir de nível. Brisa continua como estava." `left` are the
 * characters that were not marked. */
export function reachedConfirmation(marked: readonly string[], left: readonly string[]): string {
  const lead = `Marco alcançado: ${nameList(marked)} ${marked.length > 1 ? 'podem' : 'pode'} subir de nível.`;
  return left.length === 0 ? lead : `${lead} ${nameList(left)} ${left.length > 1 ? 'continuam como estavam' : 'continua como estava'}.`;
}

/** What the master reads right after "Dar a mais alguém". */
export function givenConfirmation(marked: readonly string[]): string {
  return `${nameList(marked)} ${marked.length > 1 ? 'podem' : 'pode'} subir de nível com o marco.`;
}

/** The effect line of the "Marcar como alcançado" sheet: "2 personagens podem
 * subir de nível", empty with nobody checked. */
export function reachedEffect(n: number): string {
  return n === 0 ? '' : n === 1 ? '1 personagem pode subir de nível' : `${n} personagens podem subir de nível`;
}

/** The question before the master undoes the last mark of a milestone: who
 * loses the tag and whether the milestone goes back to the planned ones. */
export function undoMilestoneConsequence(m: Milestone, mark: XPAward): string {
  const names = nameList(mark.shares.map((s) => s.characterName));
  const lose = `${names} ${mark.shares.length > 1 ? 'perdem' : 'perde'} a marca “Pode subir de nível”, se ${mark.shares.length > 1 ? 'ainda não subiram' : 'ainda não subiu'} de nível na ficha.`;
  const back =
    m.offList || m.marks.length > 1
      ? m.offList
        ? 'O marco some desta lista.'
        : 'O marco continua alcançado para os outros.'
      : 'O marco volta para “Marcos planejados”.';
  return `${lose} ${back} Fica registrado.`;
}

/** What the player's live region says when a milestone appeared since the last
 * read: "O mestre marcou Chegar ao Vale Seco. Pensantus pode subir de nível."
 * It names only the player's own characters. */
export function playerAnnouncement(m: Milestone, mine: ReadonlySet<string>): string {
  const own = m.marks.flatMap((a) => a.shares.filter((s) => mine.has(s.characterId)).map((s) => s.characterName));
  const lead = `O mestre marcou ${m.text}.`;
  return own.length === 0 ? lead : `${lead} ${nameList(own)} ${own.length > 1 ? 'podem' : 'pode'} subir de nível.`;
}
