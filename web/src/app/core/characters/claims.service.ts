import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { CharacterService } from '../../../gen/meurpg/characters/v1/characters_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * Thin wrapper around the claim calls of the generated `CharacterService` client (MR-049): the
 * master's reserved characters and their links, and the page a player opens with a link.
 *
 * `token` is the link's secret: it only ever goes in a request body, never in a URL, a route
 * parameter or a log line (docs/architecture.md, "The claim flow"). An empty token means the link the
 * person signed in with, which the server kept for ten minutes.
 */
@Injectable({ providedIn: 'root' })
export class ClaimsService {
  private readonly client = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  /** Makes the character's link; the answer holds its secret, once. `validityDays` is 1, 7 or 30. */
  createLink(campaignId: string, characterId: string, validityDays: number) {
    return this.client.createClaimLink({ campaignId, characterId, validityDays });
  }

  revokeLink(campaignId: string, characterId: string) {
    return this.client.revokeClaimLink({ campaignId, characterId });
  }

  returnToReserve(campaignId: string, characterId: string) {
    return this.client.returnCharacterToReserve({ campaignId, characterId });
  }

  deleteReserved(campaignId: string, characterId: string) {
    return this.client.deleteReservedCharacter({ campaignId, characterId });
  }

  /** The public card of the character a link is for (the person must be signed in). */
  preview(token: string) {
    return this.client.previewClaim({ token });
  }

  /** Takes the character. Only this call claims: signing in never does. */
  claim(token: string) {
    return this.client.claimCharacter({ token });
  }
}
