import { InviteState, Role, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';

/** "mestre" or "jogador" (proto's Role comment: "The app shows 'mestre' and
 * 'jogador'"). Unspecified only happens for a malformed response and reads
 * as "jogador" — the least-privileged label — rather than crashing a list. */
export function roleLabel(role: Role): string {
  return role === Role.MASTER ? 'mestre' : 'jogador';
}

export function xpModeLabel(xpMode: XpMode): string {
  switch (xpMode) {
    case XpMode.ENEMIES:
      return 'Por inimigos derrotados';
    case XpMode.GOLD:
      return 'Por ouro';
    case XpMode.MILESTONES:
      return 'Por marcos';
    default:
      return 'Não definido';
  }
}

/** ativo / usado / expirado / revogado, as MR-002's invite list asks for. */
export function inviteStateLabel(state: InviteState): string {
  switch (state) {
    case InviteState.ACTIVE:
      return 'ativo';
    case InviteState.USED_UP:
      return 'usado';
    case InviteState.EXPIRED:
      return 'expirado';
    case InviteState.REVOKED:
      return 'revogado';
    default:
      return 'desconhecido';
  }
}

/** The member/campaign name to show, never an e-mail (docs/privacidade.md:
 * the display name is the only name anyone else in a campaign ever sees). */
export function displayNameOrFallback(displayName: string): string {
  const trimmed = displayName.trim();
  return trimmed.length > 0 ? trimmed : 'Sem nome';
}
