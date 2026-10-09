import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { CharacterService } from '../../../gen/meurpg/characters/v1/characters_pb';
import {
  type PreviewRevivifyResponse,
  type RevivifyRequest,
  RevivifyService,
} from '../../../gen/meurpg/play/v1/revivify_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/**
 * Thin wrapper around the master's side of Revivify (`RevivifyService`, revivify.proto) and "Reviver"
 * (`CharacterService.ReviveCharacter`): the casts outside a combat that wait for the master, his answer, the switch
 * "Revivificar não funciona nesta morte" and the revival itself. Callers map the errors to Portuguese
 * (`describeCharacterError`, `livingRefusal`). `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class RevivifyClient {
  private readonly revivify = createClient(RevivifyService, inject(CONNECT_TRANSPORT));
  private readonly characters = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  /** The casts of the open game session, newest first: the master reads all of them. */
  async list(campaignId: string): Promise<readonly RevivifyRequest[]> {
    const res = await this.revivify.listRevivifyRequests({ campaignId });
    return res.requests;
  }

  /** Who the caster can reach now: in a combat by the combatant, outside one by the character. Read-only. */
  async preview(
    campaignId: string,
    caster:
      | { readonly encounterId: string; readonly casterId: string }
      | { readonly casterCharacterId: string },
  ): Promise<PreviewRevivifyResponse> {
    return this.revivify.previewRevivify({ campaignId, ...caster });
  }

  /** The cast outside a combat: a request that waits for the master, with nothing spent. The key is the caller's, kept across its retries. */
  async request(
    campaignId: string,
    casterCharacterId: string,
    targetCharacterId: string,
    slot: { readonly level: number; readonly pact: boolean },
    idempotencyKey: string,
  ): Promise<RevivifyRequest | undefined> {
    const res = await this.revivify.requestRevivify({
      campaignId,
      casterCharacterId,
      targetCharacterId,
      slot,
      materialConfirmed: true,
      idempotencyKey,
    });
    return res.request;
  }

  /** The master's answer to a cast outside a combat; the key is made once per answer and sent again on a retry. */
  async confirmTime(
    campaignId: string,
    pendingId: string,
    withinMinute: boolean,
    idempotencyKey: string,
  ): Promise<RevivifyRequest | undefined> {
    const res = await this.revivify.confirmRevivifyTime({
      campaignId,
      pendingId,
      withinMinute,
      idempotencyKey,
    });
    return res.request;
  }

  /** The master's switch, for a combatant of a combat that is not over. */
  async setBlocked(campaignId: string, combatantId: string, blocked: boolean): Promise<void> {
    await this.revivify.setRevivifyBlocked({
      campaignId,
      target: { case: 'combatantId', value: combatantId },
      blocked,
    });
  }

  /** "Reviver": a dead player's character lives again with 1 hit point. */
  async revive(campaignId: string, characterId: string, idempotencyKey: string): Promise<void> {
    await this.characters.reviveCharacter({ campaignId, characterId, idempotencyKey });
  }
}
