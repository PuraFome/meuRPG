import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import {
  CastingService,
  type CastSpellOutsideCombatResponse,
  type ConfirmCastTimePassedResponse,
  type EndActiveSpellResponse,
  type GetCastOptionsResponse,
  type ListSpellCastsResponse,
  type OutsideCast,
  type AbandonCastResponse,
} from '../../../gen/meurpg/play/v1/casting_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How the dice of a cast come: the app rolls them, or the sum of the physical dice (without the modifier). */
export type CastDice = { readonly inApp: true } | { readonly typedSum: number };

/** What a summoning spell brings: the option and a content key per creature, each with a name or none. */
export interface CastSummon {
  readonly option: number;
  readonly creatureKeys: readonly string[];
  readonly names?: readonly string[];
}

/** One cast outside a combat: who casts what, with which slot (or as a ritual), on whom. */
export interface OutsideCastRequest {
  readonly campaignId: string;
  readonly casterId: string;
  readonly spellKey: string;
  readonly asRitual: boolean;
  /** The slot; `null` for a cantrip and a ritual. */
  readonly slot: { readonly level: number; readonly pact: boolean } | null;
  readonly targetIds: readonly string[];
  readonly dice: CastDice | null;
  readonly summon?: CastSummon;
  /** Aprimorar Habilidade only: the ability it is cast for ("str" to "cha"). */
  readonly abilityKey?: string;
}

/**
 * Thin wrapper around the generated `CastingService` client (casting outside a combat). `providedIn: 'root'`, and
 * only lazy code imports it, so the generated code stays out of the initial bundle (like `CombatClient`). Every
 * change takes the key the caller keeps with its `ActionKey`: a double tap or a retry after a lost answer is the
 * same cast, and the server answers with what the first one did. Callers map errors with `casting-errors.ts`.
 */
@Injectable({ providedIn: 'root' })
export class CastingClient {
  private readonly client = createClient(CastingService, inject(CONNECT_TRANSPORT));

  async options(campaignId: string, characterId: string): Promise<GetCastOptionsResponse> {
    return this.client.getCastOptions({ campaignId, characterId });
  }

  async list(campaignId: string, characterId = ''): Promise<ListSpellCastsResponse> {
    return this.client.listSpellCasts({ campaignId, characterId });
  }

  async cast(
    req: OutsideCastRequest,
    idempotencyKey: string,
  ): Promise<CastSpellOutsideCombatResponse> {
    return this.client.castSpellOutsideCombat({
      campaignId: req.campaignId,
      casterCharacterId: req.casterId,
      spellKey: req.spellKey,
      asRitual: req.asRitual,
      ...(req.slot ? { slot: { level: req.slot.level, pact: req.slot.pact } } : {}),
      targetIds: [...req.targetIds],
      idempotencyKey,
      ...(req.dice ? { roll: diceOf(req.dice) } : {}),
      ...(req.abilityKey ? { abilityKey: req.abilityKey } : {}),
      ...(req.summon
        ? {
            summon: {
              option: req.summon.option,
              creatureKeys: [...req.summon.creatureKeys],
              names: [...(req.summon.names ?? [])],
            },
          }
        : {}),
    });
  }

  /** "Concluir conjuração": the master says the casting time has passed. */
  async confirm(
    campaignId: string,
    castId: string,
    dice: CastDice | null,
    idempotencyKey: string,
    summon?: CastSummon,
  ): Promise<ConfirmCastTimePassedResponse> {
    return this.client.confirmCastTimePassed({
      campaignId,
      castId,
      idempotencyKey,
      ...(dice ? { roll: diceOf(dice) } : {}),
      ...(summon
        ? {
            summon: {
              option: summon.option,
              creatureKeys: [...summon.creatureKeys],
              names: [...(summon.names ?? [])],
            },
          }
        : {}),
    });
  }

  /** "Parar a conjuração": the casting fails and spends no slot. */
  async abandon(
    campaignId: string,
    castId: string,
    idempotencyKey: string,
  ): Promise<AbandonCastResponse> {
    return this.client.abandonCast({ campaignId, castId, idempotencyKey });
  }

  /** "Encerrar": the spell that lasts ends. */
  async end(
    campaignId: string,
    castId: string,
    idempotencyKey: string,
  ): Promise<EndActiveSpellResponse> {
    return this.client.endActiveSpell({ campaignId, castId, idempotencyKey });
  }
}

function diceOf(dice: CastDice) {
  return 'inApp' in dice
    ? ({ case: 'rollInApp', value: true } as const)
    : ({ case: 'typedSum', value: dice.typedSum } as const);
}

export type { OutsideCast };
