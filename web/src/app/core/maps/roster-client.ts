import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { CharacterService, CharacterState } from '../../../gen/meurpg/characters/v1/characters_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** A character that can have a token on a map. */
export interface RosterEntry {
  readonly id: string;
  readonly name: string;
  readonly kind: number;
  /** The player's account ID, empty for an NPC. */
  readonly playerUserId: string;
  /** "Mago 3", or empty. */
  readonly classSummary: string;
  /** "Gnomo das Rochas", or empty. */
  readonly raceName: string;
  /** The player's display name, or `null`. */
  readonly playerName: string | null;
  /** An NPC's portrait: the gallery image's ID, or empty. */
  readonly portraitImageId: string;
  /** A character nobody owns yet (imported, waiting for a claim): it cannot fight. */
  readonly reserved?: boolean;
}

/**
 * The campaign's characters that may have a token (a living player
 * character or an NPC: not dead, not waiting for approval), for the
 * editor's "Adicionar token" menu. `providedIn: 'root'`, replaced in tests.
 */
@Injectable({ providedIn: 'root' })
export class RosterClient {
  private readonly client = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  async list(campaignId: string): Promise<RosterEntry[]> {
    const res = await this.client.listCharacters({ campaignId });
    return res.characters
      .filter((c) => c.state === CharacterState.DRAFT || c.state === CharacterState.LOCKED)
      .map((c) => ({
        id: c.id,
        name: c.name,
        kind: c.kind,
        playerUserId: c.playerUserId,
        classSummary: c.classSummary,
        raceName: c.raceNamePt,
        playerName: c.playerDisplayName.trim() || null,
        portraitImageId: c.portraitUrl.replace(/^\/images\//, '').replace(/\/thumb$/, ''),
        reserved: c.reserved,
      }));
  }
}
