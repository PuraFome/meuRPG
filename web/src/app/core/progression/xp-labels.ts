import { timestampDate } from '@bufbuild/protobuf/wkt';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { type XPAward, XPAwardMode } from '../../../gen/meurpg/progression/v1/progression_pb';
import { formatInt, tight } from '../format/text';
import { formatDayAt } from '../../shared/session-time/session-time';

/** The tag on each history line (E7-09). */
export function modeTag(mode: XPAwardMode): string {
  switch (mode) {
    case XPAwardMode.XP_AWARD_MODE_ENEMIES:
      return 'Por inimigos';
    case XPAwardMode.XP_AWARD_MODE_GOLD:
      return 'Por ouro';
    case XPAwardMode.XP_AWARD_MODE_MANUAL:
      return 'Avulso';
    case XPAwardMode.XP_AWARD_MODE_MILESTONE:
      return 'Marco';
    default:
      return '';
  }
}

/** "Pensantus, Toren e Brisa". */
export function nameList(names: readonly string[]): string {
  if (names.length <= 1) {
    return names.join('');
  }
  return `${names.slice(0, -1).join(', ')} e ${names[names.length - 1]}`;
}

/** "Samuel deu a Pensantus, Toren e Brisa" (an award) or "Samuel marcou
 * Pensantus, Toren e Brisa" (a milestone). An account that was deleted has no
 * name: "O mestre". */
export function givenLine(award: XPAward): string {
  const who = award.givenByDisplayName.trim() || 'O mestre';
  const names = nameList(award.shares.map((s) => s.characterName));
  return award.mode === XPAwardMode.XP_AWARD_MODE_MILESTONE
    ? `${who} marcou ${names}`
    : `${who} deu a ${names}`;
}

/** "02/10 às 20:41", or empty for a line with no date. */
export function awardWhen(award: XPAward): string {
  return award.createdAt ? formatDayAt(timestampDate(award.createdAt)) : '';
}

/** "Desfeito por Samuel em 02/10 às 21:10." */
export function undoneLine(award: XPAward): string {
  const who = award.undoneByDisplayName.trim() || 'o mestre';
  const when = award.undoneAt ? ` em ${formatDayAt(timestampDate(award.undoneAt))}` : '';
  return `Desfeito por ${who}${when}.`;
}

/** What each one got: shares are equal, so the first one says it. A milestone
 * has no number. */
export function eachGot(award: XPAward): number {
  return award.shares[0]?.xp ?? 0;
}

/** "116 XP para cada". */
export function awardEach(award: XPAward): string {
  return tight(`${formatInt(eachGot(award))} XP para cada`);
}

/** "Total de 350 XP". */
export function awardTotal(award: XPAward): string {
  return tight(`Total de ${formatInt(award.totalXp)} XP`);
}

/** How a character stands towards the next level, as the rows of "Experiência"
 * and the sheet's block read it. */
export interface Progress {
  /** "2.716 de 2.700 XP", or "2.700 XP" at the top level. */
  readonly of: string;
  /** 0 to 100, for the bar. */
  readonly percent: number;
  /** "Faltam 334 XP", or empty when it can level up or is at level 20. */
  readonly missing: string;
}

export function progress(xp: number, nextLevelXp: number, canLevelUp: boolean): Progress {
  if (nextLevelXp <= 0) {
    return { of: tight(`${formatInt(xp)} XP`), percent: 100, missing: '' };
  }
  const percent = Math.min(100, Math.max(0, Math.round((xp / nextLevelXp) * 100)));
  const missing = !canLevelUp && xp < nextLevelXp ? tight(`Faltam ${formatInt(nextLevelXp - xp)} XP`) : '';
  return { of: tight(`${formatInt(xp)} de ${formatInt(nextLevelXp)} XP`), percent, missing };
}

/** The line under "Experiência": what the campaign counts and, when the whole
 * group is at the same level, what the next one asks. */
export function experienceLead(
  mode: XpMode,
  next: { readonly level: number; readonly xp: number } | null,
): string {
  if (mode === XpMode.MILESTONES) {
    return 'Campanha por marcos: o nível sobe quando o mestre marca um marco, sem contar XP.';
  }
  const how = mode === XpMode.GOLD ? 'XP por ouro encontrado.' : 'XP por inimigos derrotados.';
  return next ? tight(`${how} O nível ${next.level} pede ${formatInt(next.xp)} XP.`) : how;
}

/** "350 XP dados: 116 para cada. 2 XP se perderam na divisão." (what the
 * master reads right after giving). */
export function givenText(totalXp: number, xpEach: number, lostXp: number): string {
  const lost = lostXp === 0 ? '' : lostXp === 1 ? ' 1 XP se perdeu na divisão.' : ` ${formatInt(lostXp)} XP se perderam na divisão.`;
  return tight(`${formatInt(totalXp)} XP dados: ${formatInt(xpEach)} para cada.${lost}`);
}

/** "Marco registrado: todos podem subir de nível. Pensantus, Toren e Brisa
 * ganharam a marca." `everyone` is whether the mark went to the whole group. */
export function milestoneText(award: XPAward, everyone: boolean): string {
  const names = award.shares.map((s) => s.characterName);
  const lead = everyone
    ? 'Marco registrado: todos podem subir de nível.'
    : `Marco registrado: ${nameList(names)} ${names.length > 1 ? 'podem' : 'pode'} subir de nível.`;
  return everyone ? `${lead} ${nameList(names)} ${names.length > 1 ? 'ganharam' : 'ganhou'} a marca.` : lead;
}
