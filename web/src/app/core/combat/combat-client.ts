import { Injectable, inject } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import {
  CombatService,
  type Encounter,
  type ParticipantSchema,
} from '../../../gen/meurpg/play/v1/combat_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** Who joins a combat: a character, how many copies (NPCs) and whether the
 * copies start hidden (unset: the server's default, hidden for an NPC). */
export interface JoinSpec {
  readonly characterId: string;
  readonly count?: number;
  readonly hidden?: boolean;
}

/** How a combatant's d20 comes (`SubmitInitiative`): the app rolls it, or a
 * face typed from a physical die. */
export type InitiativeRoll = { readonly inApp: true } | { readonly face: number };

/**
 * Thin wrapper around the generated `CombatService` client (MR-013), in the
 * same shape as `MapsClient`. `providedIn: 'root'`, and only lazy code
 * imports it, so the generated combat code stays out of the initial bundle.
 * Every write sends an idempotency key made for that call: a double tap
 * never runs twice, and a call the app repeats on purpose is a new action.
 * Callers map errors to Portuguese with `combat-errors.ts`.
 */
@Injectable({ providedIn: 'root' })
export class CombatClient {
  private readonly client = createClient(CombatService, inject(CONNECT_TRANSPORT));

  /** The latest combat of the open session, or `null` while it had none. */
  async get(campaignId: string): Promise<Encounter | null> {
    return (await this.client.getEncounter({ campaignId })).encounter ?? null;
  }

  async start(
    campaignId: string,
    name: string,
    participants: readonly JoinSpec[],
    idempotencyKey: string,
  ): Promise<Encounter> {
    const res = await this.client.startEncounter({
      campaignId,
      idempotencyKey,
      name,
      participants: participants.map(toParticipant),
    });
    return need(res.encounter, 'StartEncounter');
  }

  async submitInitiative(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    roll: InitiativeRoll,
  ): Promise<Encounter> {
    const res = await this.client.submitInitiative({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: newKey(),
      roll: 'inApp' in roll ? { case: 'rollInApp', value: true } : { case: 'd20Face', value: roll.face },
    });
    return need(res.encounter, 'SubmitInitiative');
  }

  async setOrder(
    campaignId: string,
    encounterId: string,
    combatantIds: readonly string[],
  ): Promise<Encounter> {
    const res = await this.client.setInitiativeOrder({
      campaignId,
      encounterId,
      idempotencyKey: newKey(),
      combatantIds: [...combatantIds],
    });
    return need(res.encounter, 'SetInitiativeOrder');
  }

  async begin(campaignId: string, encounterId: string): Promise<Encounter> {
    const res = await this.client.beginCombat({
      campaignId,
      encounterId,
      idempotencyKey: newKey(),
    });
    return need(res.encounter, 'BeginCombat');
  }

  /** `expectedCombatantId` is whose turn the screen thinks it is, so a double
   * tap never skips two turns; empty only when nobody is on turn. */
  async endTurn(
    campaignId: string,
    encounterId: string,
    expectedCombatantId: string,
  ): Promise<Encounter> {
    const res = await this.client.endTurn({
      campaignId,
      encounterId,
      idempotencyKey: newKey(),
      expectedCombatantId,
    });
    return need(res.encounter, 'EndTurn');
  }

  async move(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    col: number,
    row: number,
  ): Promise<Encounter> {
    const res = await this.client.moveCombatant({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: newKey(),
      col,
      row,
    });
    return need(res.encounter, 'MoveCombatant');
  }

  async setHidden(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    hidden: boolean,
  ): Promise<Encounter> {
    const res = await this.client.setCombatantHidden({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: newKey(),
      hidden,
    });
    return need(res.encounter, 'SetCombatantHidden');
  }

  async add(
    campaignId: string,
    encounterId: string,
    participants: readonly JoinSpec[],
  ): Promise<Encounter> {
    const res = await this.client.addCombatants({
      campaignId,
      encounterId,
      idempotencyKey: newKey(),
      participants: participants.map(toParticipant),
    });
    return need(res.encounter, 'AddCombatants');
  }

  async remove(campaignId: string, encounterId: string, combatantId: string): Promise<Encounter> {
    const res = await this.client.removeCombatant({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: newKey(),
    });
    return need(res.encounter, 'RemoveCombatant');
  }

  async end(campaignId: string, encounterId: string): Promise<Encounter> {
    const res = await this.client.endEncounter({
      campaignId,
      encounterId,
      idempotencyKey: newKey(),
    });
    return need(res.encounter, 'EndEncounter');
  }
}

/** A fresh idempotency key. The start dialog makes its own, once per open
 * dialog, so a second tap on "Iniciar combate" can't start two combats. */
export function newKey(): string {
  return crypto.randomUUID();
}

function toParticipant(spec: JoinSpec): MessageInitShape<typeof ParticipantSchema> {
  return { characterId: spec.characterId, count: spec.count ?? 1, hidden: spec.hidden };
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without its result`);
  }
  return value;
}
