import { Injectable, inject } from '@angular/core';
import { createClient } from '@connectrpc/connect';

import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import type { OpenSceneInfo, SceneRoll, StageNpc } from '../../../gen/meurpg/play/v1/scene_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How a scene check's d20 comes (RN-18): the app rolls it, or the player's
 * typed face of a real die. */
export type SceneDie =
  { readonly inApp: true } | { readonly face: number } | { readonly faces: readonly number[] };

/**
 * Thin wrapper around the RP scene calls of `PlayService` (MR-015, play.proto):
 * the master opens and closes the scene, everyone reads it (each as they may
 * see it) and a player rolls an action. Callers map errors to Portuguese
 * (`sceneErrorMessage`). `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class SceneClient {
  private readonly client = createClient(PlayService, inject(CONNECT_TRANSPORT));

  async open(campaignId: string, pointId: string): Promise<OpenSceneInfo> {
    const res = await this.client.openScene({ campaignId, pointId });
    return need(res.scene, 'OpenScene');
  }

  async close(campaignId: string): Promise<void> {
    await this.client.closeScene({ campaignId });
  }

  /** The open scene as the caller sees it; `null` when none is open. */
  async get(campaignId: string): Promise<OpenSceneInfo | null> {
    const res = await this.client.getOpenScene({ campaignId });
    return res.scene ?? null;
  }

  /** `PutOnStage` (MR-031): the NPC comes in after those already there. The
   * answer is the stage as it is now. */
  async putOnStage(campaignId: string, characterId: string): Promise<readonly StageNpc[]> {
    const res = await this.client.putOnStage({ campaignId, characterId });
    return res.stage;
  }

  /** `TakeOffStage`: at once, nothing to confirm. */
  async takeOffStage(campaignId: string, characterId: string): Promise<readonly StageNpc[]> {
    const res = await this.client.takeOffStage({ campaignId, characterId });
    return res.stage;
  }

  /** `SetSpeaker`: who speaks now, or nobody (an empty `characterId`). */
  async setSpeaker(campaignId: string, characterId: string): Promise<readonly StageNpc[]> {
    const res = await this.client.setSpeaker({ campaignId, characterId });
    return res.stage;
  }

  /** `RollSceneCheck`. The key is made once per roll and sent again on a retry:
   * the server then answers with the first roll and changes nothing. */
  async roll(
    campaignId: string,
    actionId: string,
    die: SceneDie,
    idempotencyKey: string,
    extraDieFaces: readonly number[] = [],
  ): Promise<SceneRoll> {
    const res = await this.client.rollSceneCheck({
      campaignId,
      actionId,
      idempotencyKey,
      roll:
        'inApp' in die
          ? { case: 'rollInApp', value: true }
          : 'face' in die
            ? { case: 'd20Face', value: die.face }
            : { case: undefined },
      d20Faces: 'faces' in die ? [...die.faces] : [],
      extraDieFaces: [...extraDieFaces],
    });
    return need(res.roll, 'RollSceneCheck');
  }

  /** `GrantSceneAttempt` (MR-015): one more attempt for one character at one
   * action. The key is made once per grant and sent again on a retry. The
   * answer is the open scene as the master sees it. */
  async grantAttempt(
    campaignId: string,
    actionId: string,
    characterId: string,
    idempotencyKey: string,
  ): Promise<OpenSceneInfo> {
    const res = await this.client.grantSceneAttempt({
      campaignId,
      actionId,
      characterId,
      idempotencyKey,
    });
    return need(res.scene, 'GrantSceneAttempt');
  }
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without its result`);
  }
  return value;
}
