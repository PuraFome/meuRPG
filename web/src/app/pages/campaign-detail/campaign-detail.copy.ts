import { DicePreference, Member } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { roleLabel } from '../../core/campaigns/campaign-labels';

/** One row of the "Membros" panel. */
export interface MemberRowVm {
  readonly userId: string;
  /** The display name, or "Sem nome no perfil" (the role is in its own column):
   * never an e-mail (docs/privacy.md) and never a bare "Sem nome". */
  readonly name: string;
  /** "mestre" or "jogador", lowercase as in the design's member list. */
  readonly role: string;
  /** The signed-in person themself. */
  readonly isViewer: boolean;
  /** Whether the member chose a display name ("Meu perfil"). */
  readonly hasName: boolean;
  /** The member's dice choice (RN-18); only the master receives it. */
  readonly dicePreference: DicePreference;
}

export function memberRows(members: readonly Member[], viewerId: string | null): MemberRowVm[] {
  return members.map((m) => {
    const name = m.displayName.trim();
    return {
      userId: m.userId,
      name: name || 'Sem nome no perfil',
      role: roleLabel(m.role),
      isViewer: viewerId !== null && m.userId === viewerId,
      hasName: name.length > 0,
      dicePreference: m.dicePreference,
    };
  });
}

/**
 * "29/09/2026 às 20:14", in local time: the date format of the campaign
 * page (the session's start, an invite's date). A plain formatter, like
 * `formatDateTime` in core, so no `DatePipe` locale data joins the bundle.
 * The spaces around "às" do not break, so a line never ends on "às".
 */
export function formatDateAt(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return (
    `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()}` +
    `\u00a0às\u00a0${pad(date.getHours())}:${pad(date.getMinutes())}`
  );
}
