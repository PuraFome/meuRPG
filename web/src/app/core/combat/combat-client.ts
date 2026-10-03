import { Injectable, inject } from '@angular/core';
import type { MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import {
  type AttackRoll,
  CombatService,
  type Encounter,
  type GetTurnOptionsResponse,
  type ListCombatLogResponse,
  type ParticipantSchema,
  type PendingDamage,
  type ReactionOutcome,
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

/** How a d20 of an attack comes: the app rolls it, or a typed face. */
export type AttackDie = InitiativeRoll;

/** How the damage comes: the app rolls it, or the sum of the physical dice
 * (without the modifier; the server adds it). */
export type DamageDie = { readonly inApp: true } | { readonly sum: number };

/** What an attack answers: the combat, the roll and the damage it opened. */
export interface AttackResult {
  readonly encounter: Encounter;
  readonly roll: AttackRoll;
  readonly pending: PendingDamage | undefined;
}

/** What a damage call answers: the combat and the damage as it is now. */
export interface DamageResult {
  readonly encounter: Encounter;
  readonly pending: PendingDamage;
}

/** What Escudo did to a hit (`UseReaction`). */
export interface ReactionResult {
  readonly encounter: Encounter;
  readonly outcome: ReactionOutcome;
}

/** One adjustment of an NPC's hit points ("Dano/Cura"): at most one of the
 * three, and, apart or together, new temporary hit points. */
export interface HpAdjust {
  readonly change?: { readonly kind: 'damage' | 'heal' | 'exact'; readonly value: number };
  readonly temporary?: number;
}

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
    discardPendingDamage = false,
  ): Promise<Encounter> {
    const res = await this.client.endTurn({
      campaignId,
      encounterId,
      idempotencyKey: newKey(),
      expectedCombatantId,
      discardPendingDamage,
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

  /** What a combatant can do now ("Sua vez"), with the targets of each
   * attack and the damage still to roll or apply. */
  turnOptions(
    campaignId: string,
    encounterId: string,
    combatantId: string,
  ): Promise<GetTurnOptionsResponse> {
    return this.client.getTurnOptions({ campaignId, encounterId, combatantId });
  }

  /** The attack roll. `key` is made once per attack by the caller, so a retry
   * after a lost answer never rolls twice. */
  async rollAttack(
    campaignId: string,
    encounterId: string,
    attackerId: string,
    attackKey: string,
    targetId: string,
    die: AttackDie,
    key: string,
  ): Promise<AttackResult> {
    const res = await this.client.rollAttack({
      campaignId,
      encounterId,
      attackerId,
      attackKey,
      targetId,
      idempotencyKey: key,
      roll: 'inApp' in die ? { case: 'rollInApp', value: true } : { case: 'd20Face', value: die.face },
    });
    return {
      encounter: need(res.encounter, 'RollAttack'),
      roll: need(res.roll, 'RollAttack'),
      pending: res.pendingDamage,
    };
  }

  async rollDamage(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
    die: DamageDie,
    key: string,
  ): Promise<DamageResult> {
    const res = await this.client.rollDamage({
      campaignId,
      encounterId,
      pendingDamageId,
      idempotencyKey: key,
      roll: 'inApp' in die ? { case: 'rollInApp', value: true } : { case: 'typedSum', value: die.sum },
    });
    return { encounter: need(res.encounter, 'RollDamage'), pending: need(res.pendingDamage, 'RollDamage') };
  }

  async applyDamage(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
  ): Promise<DamageResult> {
    const res = await this.client.applyPendingDamage({
      campaignId,
      encounterId,
      pendingDamageId,
      idempotencyKey: newKey(),
    });
    return { encounter: need(res.encounter, 'ApplyPendingDamage'), pending: need(res.pendingDamage, 'ApplyPendingDamage') };
  }

  async discardDamage(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
  ): Promise<DamageResult> {
    const res = await this.client.discardPendingDamage({
      campaignId,
      encounterId,
      pendingDamageId,
      idempotencyKey: newKey(),
    });
    return { encounter: need(res.encounter, 'DiscardPendingDamage'), pending: need(res.pendingDamage, 'DiscardPendingDamage') };
  }

  /** The master answers for the target: cast Escudo with `slot`. */
  async useReaction(
    campaignId: string,
    encounterId: string,
    pendingDamageId: string,
    slot: { level: number; pact: boolean },
    key: string,
  ): Promise<ReactionResult> {
    const res = await this.client.useReaction({ campaignId, encounterId, pendingDamageId, slot, idempotencyKey: key });
    return { encounter: need(res.encounter, 'UseReaction'), outcome: res.outcome };
  }

  /** The master lets the hit go ("Seguir sem Escudo"). */
  async declineReaction(campaignId: string, encounterId: string, pendingDamageId: string, key: string): Promise<Encounter> {
    const res = await this.client.declineReaction({ campaignId, encounterId, pendingDamageId, idempotencyKey: key });
    return need(res.encounter, 'DeclineReaction');
  }

  /** A standard action ("standard:dash"). */
  async takeAction(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    actionKey: string,
  ): Promise<Encounter> {
    const res = await this.client.takeAction({
      campaignId,
      encounterId,
      combatantId,
      actionKey,
      idempotencyKey: newKey(),
    });
    return need(res.encounter, 'TakeAction');
  }

  /** The master's "Dano/Cura" on an NPC. */
  async adjustHitPoints(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    adjust: HpAdjust,
    key: string,
  ): Promise<Encounter> {
    const c = adjust.change;
    const res = await this.client.adjustCombatantHitPoints({
      campaignId,
      encounterId,
      combatantId,
      idempotencyKey: key,
      change: !c
        ? { case: undefined }
        : c.kind === 'damage'
          ? { case: 'damage', value: c.value }
          : c.kind === 'heal'
            ? { case: 'heal', value: c.value }
            : { case: 'hitPoints', value: c.value },
      hitPointsTemporary: adjust.temporary,
    });
    return need(res.encounter, 'AdjustCombatantHitPoints');
  }

  /** "Desfazer última ação": `expectedEventId` is the log's undoable event. */
  async undo(campaignId: string, encounterId: string, expectedEventId: string): Promise<Encounter> {
    const res = await this.client.undoLastAction({
      campaignId,
      encounterId,
      expectedEventId,
      idempotencyKey: newKey(),
    });
    return need(res.encounter, 'UndoLastAction');
  }

  /** The combat log, latest first, as the caller may see it. */
  log(campaignId: string, encounterId: string): Promise<ListCombatLogResponse> {
    return this.client.listCombatLog({ campaignId, encounterId });
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
