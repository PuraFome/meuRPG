import { timestampDate } from '@bufbuild/protobuf/wkt';

import { PendingMember } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';

/** One row of "Membros" for someone who has not created a character yet. */
export interface PendingMemberVm {
  readonly userId: string;
  /** The display name, or "Jogador sem nome": never an e-mail. */
  readonly name: string;
  /** "Entrou pelo convite em 28/09, ainda sem personagem. Sai da campanha em 28/10, se nada mudar." */
  readonly line: string;
  /** The day the membership goes away on its own, "28/10": the confirmation repeats it. */
  readonly expiresOn: string;
}

/**
 * "28/09", or "28/09/2025" when the year is not the current one: the short
 * day/month of the design, with the year only when it avoids a doubt.
 */
export function shortDate(date: Date, now: Date = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  const day = `${pad(date.getDate())}/${pad(date.getMonth() + 1)}`;
  return date.getFullYear() === now.getFullYear() ? day : `${day}/${date.getFullYear()}`;
}

export function pendingMemberRows(
  members: readonly PendingMember[],
  now: Date = new Date(),
): PendingMemberVm[] {
  return members.map((m) => {
    const joined = m.joinedAt ? shortDate(timestampDate(m.joinedAt), now) : '';
    const expiresOn = m.expiresAt ? shortDate(timestampDate(m.expiresAt), now) : '';
    const entered = joined
      ? `Entrou pelo convite em ${joined}, ainda sem personagem.`
      : 'Entrou pelo convite, ainda sem personagem.';
    const leaves = expiresOn ? ` Sai da campanha em ${expiresOn}, se nada mudar.` : '';
    return {
      userId: m.userId,
      name: m.displayName.trim() || 'Jogador sem nome',
      line: entered + leaves,
      expiresOn,
    };
  });
}
