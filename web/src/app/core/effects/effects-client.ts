import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import type { Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import type {
  CharacterEffect,
  EffectAudience,
  EffectDurationKind,
  EffectEndScope,
  EffectSaveResult,
} from '../../../gen/meurpg/play/v1/lasting_effects_pb';
import {
  type AddLastingEffectResponse,
  type EndLastingEffectResponse,
  type ExhaustionLowerReason,
  LastingEffectService,
  type ListLastingEffectsResponse,
  type LowerExhaustionResponse,
  type SetExhaustionResponse,
} from '../../../gen/meurpg/play/v1/lasting_effects_service_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How a player answers the saving throw an effect asks: the d20 in the app, the faces of physical dice, or the master. */
export type EffectSaveAnswer =
  | { readonly kind: 'app' }
  | {
      readonly kind: 'typed';
      readonly face: number;
      readonly second?: number;
      readonly extra: readonly number[];
    }
  | { readonly kind: 'hand' };

/** What `RollEffectSave` answers: the combat and the result (unset when the roll was left to the master). */
export interface EffectSaveOutcome {
  readonly encounter: Encounter;
  readonly result: EffectSaveResult | undefined;
}

/** The duration the master picks, as the form holds it (the server's `EffectDurationChoice`). */
export interface DurationSpec {
  readonly kind: EffectDurationKind;
  readonly rounds?: number;
  readonly anchorCombatantId?: string;
}

/** What "Adicionar efeito" sends. */
export interface AddEffectSpec {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly targetIds: readonly string[];
  readonly catalogKey: string;
  readonly casterId?: string;
  readonly duration: DurationSpec;
  readonly saveDc?: number;
  readonly playerVisible?: boolean;
  readonly audience?: EffectAudience;
  readonly playerLabel?: string;
}

/** Who an exhaustion call is about: a character, or a combatant of the combat that runs. */
export type ExhaustionSubject =
  { readonly characterId: string } | { readonly combatantId: string; readonly encounterId: string };

/**
 * The calls of `LastingEffectService`: the player's (the saving throw an effect asks, the effects on their characters)
 * and the master's (adding, changing and ending effects, exhaustion, game time). Every write takes the idempotency key
 * the screen keeps for that action; the screen reads again after the answer and after a live event.
 */
@Injectable({ providedIn: 'root' })
export class EffectsClient {
  private readonly client = createClient(LastingEffectService, inject(CONNECT_TRANSPORT));

  /** `RollEffectSave`: the window's saving throw, in the app, with physical dice or left to the master. */
  async rollEffectSave(
    campaignId: string,
    encounterId: string,
    windowId: string,
    how: EffectSaveAnswer,
    key: string,
  ): Promise<EffectSaveOutcome> {
    const typed = how.kind === 'typed' ? how : null;
    const res = await this.client.rollEffectSave({
      campaignId,
      encounterId,
      windowId,
      idempotencyKey: key,
      roll:
        how.kind === 'app'
          ? { case: 'rollInApp', value: true }
          : how.kind === 'typed'
            ? { case: 'd20Face', value: how.face }
            : { case: 'delegateToMaster', value: true },
      extraDieFaces: typed ? [...typed.extra] : [],
      ...(typed?.second !== undefined ? { secondD20Face: typed.second } : {}),
    });
    if (!res.encounter) {
      throw new Error('RollEffectSave answered without the combat');
    }
    return { encounter: res.encounter, result: res.result };
  }

  list(campaignId: string, encounterId: string): Promise<ListLastingEffectsResponse> {
    return this.client.listLastingEffects({ campaignId, encounterId });
  }

  /** The effects on the characters outside a combat; the master reads all of them. */
  async listCharacterEffects(campaignId: string): Promise<CharacterEffect[]> {
    return (await this.client.listCharacterEffects({ campaignId })).effects;
  }

  add(spec: AddEffectSpec, idempotencyKey: string): Promise<AddLastingEffectResponse> {
    return this.client.addLastingEffect({
      campaignId: spec.campaignId,
      encounterId: spec.encounterId,
      idempotencyKey,
      targetIds: [...spec.targetIds],
      catalogKey: spec.catalogKey,
      casterId: spec.casterId ?? '',
      duration: durationOf(spec.duration),
      saveDc: spec.saveDc,
      playerVisible: spec.playerVisible,
      audience: spec.audience,
      playerLabel: spec.playerLabel ?? '',
    });
  }

  async changeDuration(
    campaignId: string,
    encounterId: string,
    effectId: string,
    duration: DurationSpec,
    idempotencyKey: string,
  ): Promise<Encounter | undefined> {
    return (
      await this.client.changeLastingEffectDuration({
        campaignId,
        encounterId,
        effectId,
        idempotencyKey,
        duration: durationOf(duration),
      })
    ).encounter;
  }

  async setVisibility(
    campaignId: string,
    encounterId: string,
    effectId: string,
    visibility: {
      readonly playerVisible: boolean;
      readonly audience: EffectAudience;
      readonly playerLabel: string;
    },
    idempotencyKey: string,
  ): Promise<Encounter | undefined> {
    return (
      await this.client.setLastingEffectVisibility({
        campaignId,
        encounterId,
        effectId,
        idempotencyKey,
        ...visibility,
      })
    ).encounter;
  }

  /** An empty `encounterId` ends an effect on a character outside a combat. */
  end(
    campaignId: string,
    encounterId: string,
    effectId: string,
    scope: EffectEndScope,
    idempotencyKey: string,
  ): Promise<EndLastingEffectResponse> {
    return this.client.endLastingEffect({
      campaignId,
      encounterId,
      effectId,
      scope,
      idempotencyKey,
    });
  }

  async removeTarget(
    campaignId: string,
    encounterId: string,
    effectId: string,
    combatantId: string,
    idempotencyKey: string,
  ): Promise<Encounter | undefined> {
    return (
      await this.client.removeEffectTarget({
        campaignId,
        encounterId,
        effectId,
        combatantId,
        idempotencyKey,
      })
    ).encounter;
  }

  setExhaustion(
    campaignId: string,
    subject: ExhaustionSubject,
    change: {
      readonly level: number;
      readonly expectedLevel: number;
      readonly confirmDeath: boolean;
    },
    idempotencyKey: string,
  ): Promise<SetExhaustionResponse> {
    return this.client.setExhaustion({
      campaignId,
      idempotencyKey,
      subject: subjectOf(subject),
      encounterId: 'combatantId' in subject ? subject.encounterId : '',
      ...change,
    });
  }

  lowerExhaustion(
    campaignId: string,
    subject: ExhaustionSubject,
    change: {
      readonly by: number;
      readonly expectedLevel: number;
      readonly reason: ExhaustionLowerReason;
    },
    idempotencyKey: string,
  ): Promise<LowerExhaustionResponse> {
    return this.client.lowerExhaustion({
      campaignId,
      idempotencyKey,
      subject: subjectOf(subject),
      encounterId: 'combatantId' in subject ? subject.encounterId : '',
      ...change,
    });
  }

  /** Moves game time on outside a combat; answers how many effects ran out. */
  async advanceTime(campaignId: string, seconds: number, idempotencyKey: string): Promise<number> {
    return (await this.client.advanceGameTime({ campaignId, seconds, idempotencyKey }))
      .effectsEnded;
  }
}

function durationOf(duration: DurationSpec) {
  return {
    kind: duration.kind,
    rounds: duration.rounds ?? 0,
    anchorCombatantId: duration.anchorCombatantId ?? '',
  };
}

function subjectOf(subject: ExhaustionSubject) {
  return 'combatantId' in subject
    ? ({ case: 'combatantId', value: subject.combatantId } as const)
    : ({ case: 'characterId', value: subject.characterId } as const);
}
