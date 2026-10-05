import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { CharacterService } from '../../../gen/meurpg/characters/v1/characters_pb';
import { EncounterBlockedReason, FamiliarSightBlockedReason } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  PlayService,
  type StartFamiliarSightResponse,
  type StopFamiliarSightResponse,
} from '../../../gen/meurpg/play/v1/play_pb';
import { combatErrorMessage, encounterBlocked } from '../combat/combat-errors';
import { newKey } from '../connect/idempotency';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** Whether a player looks through their familiar's eyes, from `CharacterVitals.familiar_sight` (MR-036). */
export interface FamiliarSightVm {
  /** The familiar (a creature ID, as `ListCharacterCreatures` lists it). */
  readonly creatureId: string;
  /** Started in a combat: it ends by itself at the start of the character's next turn. */
  readonly inCombat: boolean;
}

/**
 * "Ver pelos olhos do familiar" (MR-036, E9-04): `StartFamiliarSight` and
 * `StopFamiliarSight` for a character, and the familiar's name for the band. The
 * server decides whether the familiar is near enough (30 m), whether the action is
 * there and what the sight does; the browser never measures. The results reach the
 * page's own state through the stream (`vitals_changed`, `encounter_changed`,
 * `vision_changed`), so a caller only needs to say it worked.
 */
@Injectable({ providedIn: 'root' })
export class FamiliarEyesClient {
  private readonly play = createClient(PlayService, inject(CONNECT_TRANSPORT));
  private readonly characters = createClient(CharacterService, inject(CONNECT_TRANSPORT));
  private readonly names = new Map<string, Promise<string | null>>();

  /** A key for each press, kept until the answer comes, so a retry after a lost answer never starts it twice. */
  start(campaignId: string, characterId: string, key = newKey()): Promise<StartFamiliarSightResponse> {
    return this.play.startFamiliarSight({ campaignId, characterId, idempotencyKey: key });
  }

  stop(campaignId: string, characterId: string, key = newKey()): Promise<StopFamiliarSightResponse> {
    return this.play.stopFamiliarSight({ campaignId, characterId, idempotencyKey: key });
  }

  /** The familiar's name ("Nanquim"), read once; `null` when it could not be read. */
  name(campaignId: string, characterId: string, creatureId: string): Promise<string | null> {
    const id = `${characterId}/${creatureId}`;
    let known = this.names.get(id);
    if (!known) {
      known = this.characters
        .listCharacterCreatures({ campaignId, characterId })
        .then((res) => res.creatures.find((c) => c.id === creatureId)?.name ?? null)
        .catch(() => {
          this.names.delete(id);
          return null;
        });
      this.names.set(id, known);
    }
    return known;
  }
}

/** Why "Ver pelos olhos" or "Voltar aos seus olhos" was refused, by the typed reason, never by the message. */
export function familiarSightMessage(err: unknown, name = 'O familiar'): string {
  const blocked = encounterBlocked(err);
  if (blocked?.reason === EncounterBlockedReason.FAMILIAR_SIGHT_BLOCKED) {
    switch (blocked.familiarSightReason) {
      case FamiliarSightBlockedReason.NO_FAMILIAR:
        return 'Você não tem um familiar agora.';
      case FamiliarSightBlockedReason.NOT_ON_MAP:
        return `Você e ${name} precisam estar no mapa, que precisa ter grade.`;
      case FamiliarSightBlockedReason.TOO_FAR:
        return `${name} está a mais de 30 m de você. Chegue mais perto para ver pelos olhos dele.`;
      case FamiliarSightBlockedReason.ALREADY_SEEING:
        return 'Você já está vendo pelos olhos do familiar.';
      case FamiliarSightBlockedReason.NOT_SEEING:
        return 'Você já voltou aos seus olhos.';
      case FamiliarSightBlockedReason.COMBAT_NOT_BEGUN:
        return 'O combate ainda não começou. Espere a sua vez para ver pelos olhos do familiar.';
      default:
        break;
    }
  }
  return combatErrorMessage(err, 'ver pelos olhos do familiar');
}

/** What the messages call the familiar: its name ("Nanquim"), or "O familiar" while the name is unknown. */
export function familiarName(name: string | null): string {
  return name && name.trim() !== '' ? name : 'O familiar';
}
