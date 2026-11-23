import { Injectable, inject } from '@angular/core';
import { Code, ConnectError, createClient } from '@connectrpc/connect';

import type { CuttingWordsAnswered } from '../../../gen/meurpg/play/v1/resources_pb';
import { ResourceService } from '../../../gen/meurpg/play/v1/resources_pb';
import { PlayService } from '../../../gen/meurpg/play/v1/play_pb';
import type { NpcCheckCandidate, NpcCheckRead } from '../../../gen/meurpg/play/v1/scene_pb';
import { sessionClosed } from '../combat/combat-errors';
import { CONNECT_TRANSPORT } from '../connect/transport';
import type { NpcCheckApi } from './npc-check-state';

/** How the master's d20 comes: the app rolls it, or the master typed the face of a real die (the master rolls either way, whatever
 * the table's dice mode). */
export type NpcDie = { readonly inApp: true } | { readonly face: number };

/** What the master asks to roll. */
export interface NpcCheckRoll {
  /** The NPC's character, else the SRD creature's key. */
  readonly characterId: string;
  readonly monsterKey: string;
  readonly checkKey: string;
  readonly die: NpcDie;
  /** The box "Os bardos veem a criatura e ela os ouve"; `false` when the map measures the creature and the box is not shown. */
  readonly bardSees: boolean;
}

/** How a bard's die comes (RN-18): the app rolls it, or the player typed the face of a real die. */
export type CutAnswer =
  | { readonly cut: false }
  | { readonly cut: true; readonly die: { readonly inApp: true } | { readonly face: number } };

/**
 * The calls of the NPC check outside a combat (`PlayService`: RollNpcCheck, SkipNpcCheckAsks, ListNpcCheckPicker, GetNpcCheck,
 * master only) and the bard's answer (`ResourceService.AnswerHeldRoll`, the cutting_words arm). Callers map errors to Portuguese
 * (`npc-check-errors`). `providedIn: 'root'`, imported only by lazy code.
 */
@Injectable({ providedIn: 'root' })
export class NpcCheckClient implements NpcCheckApi {
  private readonly play = createClient(PlayService, inject(CONNECT_TRANSPORT));
  private readonly resources = createClient(ResourceService, inject(CONNECT_TRANSPORT));

  /** The creatures the master may pick: the stage, the map's NPC tokens and, with `monsterKey`, that SRD creature. */
  async picker(campaignId: string, monsterKey = ''): Promise<readonly NpcCheckCandidate[]> {
    const res = await this.play.listNpcCheckPicker({ campaignId, monsterKey });
    return res.candidates;
  }

  async roll(
    campaignId: string,
    roll: NpcCheckRoll,
    idempotencyKey: string,
  ): Promise<NpcCheckRead> {
    const res = await this.play.rollNpcCheck({
      campaignId,
      npc: roll.characterId
        ? { case: 'npcCharacterId', value: roll.characterId }
        : { case: 'monsterKey', value: roll.monsterKey },
      checkKey: roll.checkKey,
      roll:
        'inApp' in roll.die
          ? { case: 'rollInApp', value: true }
          : { case: 'd20Face', value: roll.die.face },
      bardSees: roll.bardSees,
      idempotencyKey,
    });
    return need(res.check, 'RollNpcCheck');
  }

  async skip(campaignId: string, holdId: string, idempotencyKey: string): Promise<NpcCheckRead> {
    const res = await this.play.skipNpcCheckAsks({ campaignId, holdId, idempotencyKey });
    return need(res.check, 'SkipNpcCheckAsks');
  }

  /** The latest check of the open session; `null` when it has had none, and when no session is open (nothing to read yet). */
  async latest(campaignId: string): Promise<NpcCheckRead | null> {
    try {
      const res = await this.play.getNpcCheck({ campaignId });
      return res.check ?? null;
    } catch (err) {
      if (
        sessionClosed(err) ||
        ConnectError.from(err, Code.Unavailable).code === Code.PermissionDenied
      ) {
        return null;
      }
      throw err;
    }
  }

  /** The bard's answer to the question of an NPC check: its own die and the uses left, never the creature's roll. */
  async answer(
    campaignId: string,
    holdId: string,
    answer: CutAnswer,
    idempotencyKey: string,
  ): Promise<CuttingWordsAnswered> {
    const res = await this.resources.answerHeldRoll({
      campaignId,
      holdId,
      idempotencyKey,
      answer: {
        case: 'cuttingWords',
        value: {
          cut: answer.cut,
          roll: !answer.cut
            ? { case: undefined }
            : 'inApp' in answer.die
              ? { case: 'rollInApp', value: true }
              : { case: 'typedFace', value: answer.die.face },
        },
      },
    });
    if (res.result.case !== 'cuttingWords') {
      throw new Error("AnswerHeldRoll answered with another result than the bard's");
    }
    return res.result.value;
  }
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without the check`);
  }
  return value;
}
