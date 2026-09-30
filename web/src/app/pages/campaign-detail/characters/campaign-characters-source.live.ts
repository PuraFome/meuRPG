import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  CharacterKind as GenCharacterKind,
  CharacterService,
  CharacterState as GenCharacterState,
  CharacterSummary,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import { CharacterKind, CharacterState } from '../../../core/characters/characters.types';
import { CONNECT_TRANSPORT } from '../../../core/connect/transport';
import {
  CampaignCharacterListItemVm,
  CampaignCharactersSource,
  CampaignCharactersVm,
} from './campaign-characters.types';

const KIND_FROM_GEN: Record<GenCharacterKind, CharacterKind> = {
  [GenCharacterKind.UNSPECIFIED]: 'player',
  [GenCharacterKind.PLAYER]: 'player',
  [GenCharacterKind.ENEMY]: 'enemy',
  [GenCharacterKind.BOSS]: 'boss',
  [GenCharacterKind.MINION]: 'minion',
  [GenCharacterKind.STORY]: 'story',
};

const STATE_FROM_GEN: Record<GenCharacterState, CharacterState> = {
  [GenCharacterState.UNSPECIFIED]: 'draft',
  [GenCharacterState.DRAFT]: 'draft',
  [GenCharacterState.LOCKED]: 'locked',
  [GenCharacterState.DEAD]: 'dead',
  [GenCharacterState.PENDING]: 'pending',
};

function toListItemVm(c: CharacterSummary): CampaignCharacterListItemVm {
  return {
    id: c.id,
    name: c.name,
    kind: KIND_FROM_GEN[c.kind],
    state: STATE_FROM_GEN[c.state],
    classSummary: c.classSummary,
    playerDisplayName: c.playerDisplayName || null,
  };
}

/**
 * `CampaignCharactersSource` over the generated `CharacterService` client
 * (`meurpg.characters.v1`, phase 2). Provided at the route level for
 * `/campanhas/:id` — see `../campaign-detail.routes.ts` — so this client
 * stays out of the eager bundle.
 *
 * `ListCharacters` already returns only what the caller may see (a player
 * never gets an NPC, per characters.proto) — this just splits the one list
 * by `kind`, which works unchanged for either role: a player's own NPC list
 * is always empty because the server never sends one.
 */
@Injectable()
export class CampaignCharactersSourceLive implements CampaignCharactersSource {
  private readonly client = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  async listCharacters(campaignId: string): Promise<CampaignCharactersVm> {
    const res = await this.client.listCharacters({ campaignId });
    const playerCharacters = res.characters
      .filter((c) => c.kind === GenCharacterKind.PLAYER)
      .map(toListItemVm);
    const npcs = res.characters
      .filter((c) => c.kind !== GenCharacterKind.PLAYER)
      .map(toListItemVm);
    return {
      playerCharacters,
      npcs,
      hasLivingCharacter: playerCharacters.some((c) => c.state !== 'dead'),
    };
  }
}
