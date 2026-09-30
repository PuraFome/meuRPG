import { timestampDate } from '@bufbuild/protobuf/wkt';

import { Invite, InviteState } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { inviteStateLabel } from '../../../core/campaigns/campaign-labels';
import { formatDateAt } from '../campaign-detail.copy';

/** "1 de 3 usos", "0 de 1 uso": how many joined out of how many may. */
export function inviteUsesLabel(invite: Pick<Invite, 'useCount' | 'maxUses'>): string {
  return `${invite.useCount} de ${invite.maxUses} ${invite.maxUses === 1 ? 'uso' : 'usos'}`;
}

/**
 * The row's date line. An active invite says when it stops working ("Vence
 * em 06/10/2026 às 20:14"), an expired one when it did ("Venceu em ...");
 * a used-up or revoked one, whose expiry no longer matters, when it was
 * made ("Criado em ..."). It never repeats the state's own word (the tag
 * says "Revogado"), so each state word appears once per row. Empty when
 * the server sent no date.
 */
export function inviteDateLabel(invite: Pick<Invite, 'state' | 'expiresAt' | 'createdAt'>): string {
  switch (invite.state) {
    case InviteState.ACTIVE:
      return invite.expiresAt ? `Vence em ${formatDateAt(timestampDate(invite.expiresAt))}` : '';
    case InviteState.EXPIRED:
      return invite.expiresAt ? `Venceu em ${formatDateAt(timestampDate(invite.expiresAt))}` : '';
    default:
      return invite.createdAt ? `Criado em ${formatDateAt(timestampDate(invite.createdAt))}` : '';
  }
}

/** Whether whoever uses it waits for the master's approval (RN-15). */
export function inviteApprovalLabel(invite: Pick<Invite, 'requiresApproval'>): string {
  return invite.requiresApproval ? 'Exige aprovação do mestre' : 'Entra direto na campanha';
}

/** The state tag: MR-002's words (ativo, usado, expirado, revogado), as a
 * one-word tag in sentence case. */
export function inviteStateTag(state: InviteState): string {
  const label = inviteStateLabel(state);
  return label.charAt(0).toUpperCase() + label.slice(1);
}
