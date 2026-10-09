import { Code, ConnectError } from '@connectrpc/connect';

import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { describeConnectError, isRateLimited } from '../connect/connect-errors';
import { describeCharacterError } from './character-errors';

/** What a failed `PreviewClaim` or `ClaimCharacter` means for the claim page (MR-049). */
export type ClaimFailure =
  /** A link that cannot be used, whatever the reason: the server answers every one the same way. */
  | { readonly kind: 'unusable' }
  /** RN-03: the person has a living character in this campaign already. */
  | {
      readonly kind: 'living';
      readonly campaignId: string;
      readonly characterId: string;
      readonly characterName: string;
    }
  /** The master opened their own link. */
  | { readonly kind: 'own-link' }
  /** The session is gone: signing in again is what helps. */
  | { readonly kind: 'signed-out' }
  /** The server or the network failed, or the person asked too often: another try may work. */
  | { readonly kind: 'transient'; readonly message: string };

/** Reads a failed claim call by its typed code and detail, never by the server's message. */
export function claimFailure(err: unknown): ClaimFailure {
  const e = ConnectError.from(err, Code.Unavailable);
  switch (e.code) {
    case Code.NotFound:
      return { kind: 'unusable' };
    case Code.Unauthenticated:
      return { kind: 'signed-out' };
    case Code.FailedPrecondition: {
      const [detail] = e.findDetails(CharacterBlockedSchema);
      if (detail?.reason === CharacterBlockedReason.LIVING_CHARACTER_EXISTS) {
        return {
          kind: 'living',
          campaignId: detail.campaignId,
          characterId: detail.characterId,
          characterName: detail.characterName,
        };
      }
      if (detail?.reason === CharacterBlockedReason.CLAIM_OWN_LINK) {
        return { kind: 'own-link' };
      }
      return { kind: 'unusable' };
    }
    default:
      return {
        kind: 'transient',
        message: isRateLimited(e)
          ? describeConnectError(e, {})
          : 'Não foi possível falar com o servidor agora. Tente de novo em instantes.',
      };
  }
}

/** The reason a master's action on a reserved character failed, with who used the link when that is the reason. */
export interface ReserveActionFailure {
  readonly message: string;
  /** Set when the revoke lost to a player who took the character a moment before: the list must be read again. */
  readonly usedBy: { readonly name: string } | null;
}

/** Turns a failed "Gerar link", "Revogar o link", "Devolver à reserva" or "Excluir" into what the row says. */
export function reserveActionFailure(err: unknown): ReserveActionFailure {
  const e = ConnectError.from(err, Code.Unavailable);
  if (e.code === Code.FailedPrecondition) {
    const [detail] = e.findDetails(CharacterBlockedSchema);
    if (detail?.reason === CharacterBlockedReason.CLAIM_LINK_USED) {
      return { message: '', usedBy: { name: detail.playerDisplayName } };
    }
  }
  return { message: describeCharacterError(err), usedBy: null };
}
