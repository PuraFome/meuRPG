import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  CharacterKind,
  CharacterService,
  CharacterState,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { npcKindLabel } from '../combat/combat-view';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** An NPC the master may put on the stage (MR-031). */
export interface StageCandidate {
  readonly characterId: string;
  readonly name: string;
  /** "Inimigo", "Minion"... as the combat lists say it. */
  readonly kindLabel: string;
  /** The portrait's thumbnail ("/images/<id>/thumb"), or empty for none. */
  readonly portraitUrl: string;
}

/**
 * The campaign's living NPCs for "Pôr em cena", from one `ListCharacters`
 * call: each NPC row carries its portrait URL (master only), so no sheet is
 * read. `providedIn: 'root'`, replaced in tests.
 */
@Injectable({ providedIn: 'root' })
export class StageRoster {
  private readonly client = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  async list(campaignId: string): Promise<StageCandidate[]> {
    const res = await this.client.listCharacters({ campaignId });
    const npcs = res.characters.filter(
      (c) =>
        c.kind !== CharacterKind.PLAYER &&
        (c.state === CharacterState.DRAFT || c.state === CharacterState.LOCKED),
    );
    return npcs.map((c) => ({
      characterId: c.id,
      name: c.name,
      kindLabel: npcKindLabel(c.kind),
      portraitUrl: c.portraitUrl ? `${c.portraitUrl}/thumb` : '',
    }));
  }
}
