import type { XPAward } from '../../../../gen/meurpg/progression/v1/progression_pb';
import { XPAwardMode } from '../../../../gen/meurpg/progression/v1/progression_pb';
import { formatInt, tight } from '../../../core/format/text';
import type { ExperienceRow } from '../../../core/progression/experience-store';
import { treasureCount } from '../../../core/progression/treasure';
import { nameList } from '../../../core/progression/xp-labels';

/** "Desfazer o XP de “Combate: Emboscada na estrada”?" */
export function undoTitle(award: XPAward): string {
  return award.mode === XPAwardMode.XP_AWARD_MODE_MILESTONE
    ? `Desfazer o marco “${award.reason}”?`
    : `Desfazer o XP de “${award.reason}”?`;
}

/**
 * What undoing the last award does, in words, before the master says yes
 * (E7-09): who loses how much, and who stops being able to level up. It reads
 * the XP the screen shows now; the server does the subtraction (never below
 * 0). The history keeps the line and says it was undone.
 */
export function undoConsequence(award: XPAward, rows: readonly ExperienceRow[]): string {
  const names = award.shares.map((s) => s.characterName);
  if (award.mode === XPAwardMode.XP_AWARD_MODE_MILESTONE) {
    return `${nameList(names)} ${names.length > 1 ? 'perdem' : 'perde'} a marca “Pode subir de nível”. O histórico guarda o desfazer.`;
  }
  const each = award.shares[0]?.xp ?? 0;
  const lose = names.length > 1 ? 'perdem' : 'perde';
  const parts = [`${nameList(names)} ${lose} ${tight(`${formatInt(each)} XP`)} ${names.length > 1 ? 'cada' : ''}`.trim() + '.'];

  // "Voltar à cidade": the treasures it converted are free again.
  if (award.treasureCount > 0) {
    const many = award.treasureCount > 1;
    parts.push(
      tight(
        `${many ? `Os ${treasureCount(award.treasureCount)}` : 'O tesouro'} (${formatInt(award.gold)} PO) ${many ? 'voltam' : 'volta'} a “encontrado, não convertido”.`,
      ),
    );
  }

  // Who can level up now and would not after it (the next level's XP is out of reach again).
  const back = award.shares.flatMap((s) => {
    const row = rows.find((r) => r.id === s.characterId);
    if (!row?.canLevelUp || row.nextLevelXp <= 0) {
      return [];
    }
    const xp = Math.max(0, row.xp - s.xp);
    return xp < row.nextLevelXp ? [{ name: row.name, xp }] : [];
  });
  if (back.length === 1) {
    parts.push(`${back[0].name} volta para ${tight(`${formatInt(back[0].xp)} XP`)} e deixa de poder subir de nível.`);
  } else if (back.length > 1) {
    parts.push(`${nameList(back.map((b) => b.name))} deixam de poder subir de nível.`);
  }
  parts.push('O histórico guarda o desfazer.');
  return parts.join(' ');
}
