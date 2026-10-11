import { Injectable, inject } from '@angular/core';
import { type MessageInitShape } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import type { Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  type CheckRollInputSchema,
  ContestService,
  type GetContestStateResponse,
} from '../../../gen/meurpg/play/v1/contests_pb';
import {
  type ContestKind,
  type ContestPurpose,
  type ContestSkill,
  type ContestView,
  type GroupCheckView,
  type HelpKind,
  type HelpView,
  type HideAttemptView,
  type ShoveOutcome,
  type SurpriseSuggestion,
  type SurpriseView,
} from '../../../gen/meurpg/play/v1/contest_types_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** How a check's d20 comes: the app rolls it, or the faces typed from the physical dice (one face, or the pair for a roll with
 * advantage or disadvantage). */
/** A group check roll that waits for the player's answer about a Bardic Inspiration die. */
export class InspirationAsked extends Error {
  constructor() {
    super('The roll waits for the answer about the Bardic Inspiration die');
  }
}

export type CheckDie = { readonly inApp: true } | { readonly faces: readonly number[] };

/** What a grapple, a shove or an escape asks (`StartContest`). */
export interface StartContestInput {
  readonly campaignId: string;
  readonly encounterId: string;
  /** The one that grapples, shoves or tries to escape. */
  readonly initiatorId: string;
  /** The target; empty for an escape (the one that holds is the defender). */
  readonly targetId: string;
  readonly purpose: ContestPurpose;
  readonly kind: ContestKind;
  readonly skill: ContestSkill;
  /** The initiator's d20; none for the master's grapple with a fixed escape DC (nobody rolls). */
  readonly die?: CheckDie;
  /** The creature's fixed escape DC (1 to 40) of an `ESCAPE_DC` grapple: the master's alone (RN-20). */
  readonly escapeDc?: number;
  /** A table maneuver (ContestAttackOption.maneuvers) that starts the grapple, and the face of its die when rolled with a physical die. */
  readonly maneuverKey?: string;
  readonly maneuverFace?: number;
}

/** What the defender answers (`RespondContest`): the skill and the roll, or the roll left to the master. */
export interface RespondContestInput {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly contestId: string;
  readonly skill: ContestSkill;
  /** `null`: "Deixar o mestre rolar por mim". */
  readonly die: CheckDie | null;
}

/** The Help action, as the sheet sends it (`Help`). */
export interface HelpInput {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly helperId: string;
  readonly kind: HelpKind;
  readonly allyId: string;
  /** The check's key of a CHECK help ("skill:perception"); empty for an attack. */
  readonly taskKey: string;
  /** The creature an ATTACK help is aimed at; empty for a check. */
  readonly targetId: string;
}

/** The master's decision on a Hide (`ResolveHide`). */
export interface HideDecision {
  readonly attemptId: string;
  /** True refuses it: there is nowhere to hide. */
  readonly refuse: boolean;
  /** The master's one sentence of the refusal (1 to 120 characters); empty takes the usual one. */
  readonly refusal: string;
  /** The creatures that see the hider clearly, whatever the totals. */
  readonly seesClearlyIds: readonly string[];
}

/** The combat and the contest as the caller reads it after a call. */
export interface ContestResult {
  readonly encounter: Encounter;
  readonly contest: ContestView;
}

/** The roll of a check, as the request carries it. */
export function rollInput(die: CheckDie): MessageInitShape<typeof CheckRollInputSchema> {
  return 'inApp' in die
    ? { roll: { case: 'rollInApp', value: true } }
    : { roll: { case: 'd20Faces', value: { faces: [...die.faces] } } };
}

function need<T>(value: T | undefined, call: string): T {
  if (value === undefined) {
    throw new Error(`${call} answered without its result`);
  }
  return value;
}

/**
 * The contest calls of a combat as the player makes them (`ContestService`, W7-X): the state of the contests the player is
 * in, a grapple or a shove, the defender's answer, the winner's choice, Hide, Help and the group check's roll. Every write
 * takes the key the caller made with `ActionKey.keyFor`, so a tap sent again after a lost answer changes nothing twice.
 * Errors are worded by `combatErrorMessage` (the `ContestBlocked` detail is decoded there, next to `EncounterBlocked`).
 */
@Injectable({ providedIn: 'root' })
export class ContestClient {
  private readonly client = createClient(ContestService, inject(CONNECT_TRANSPORT));

  /** Every contest fact the caller may read: the contests they are in, the Hide actions, who is hidden, the Helps. */
  state(campaignId: string, encounterId: string): Promise<GetContestStateResponse> {
    return this.client.getContestState({ campaignId, encounterId });
  }

  /** A grapple, a shove or an escape: it spends the attack (or the action) and opens the contest. */
  async start(input: StartContestInput, key: string): Promise<ContestResult> {
    const res = await this.client.startContest({
      campaignId: input.campaignId,
      encounterId: input.encounterId,
      idempotencyKey: key,
      initiatorId: input.initiatorId,
      targetId: input.targetId,
      purpose: input.purpose,
      kind: input.kind,
      skill: input.skill,
      ...(input.die ? { roll: rollInput(input.die) } : {}),
      ...(input.escapeDc ? { escapeDc: input.escapeDc } : {}),
      ...(input.maneuverKey ? { maneuverKey: input.maneuverKey } : {}),
      ...(input.maneuverFace ? { maneuverFace: input.maneuverFace } : {}),
    });
    return {
      encounter: need(res.encounter, 'StartContest'),
      contest: need(res.contest, 'StartContest'),
    };
  }

  /** The defender's roll, or "Deixar o mestre rolar por mim". */
  async respond(input: RespondContestInput, key: string): Promise<ContestResult> {
    const res = await this.client.respondContest({
      campaignId: input.campaignId,
      encounterId: input.encounterId,
      idempotencyKey: key,
      contestId: input.contestId,
      skill: input.skill,
      ...(input.die ? { roll: rollInput(input.die) } : {}),
      deferToMaster: input.die === null,
    });
    return {
      encounter: need(res.encounter, 'RespondContest'),
      contest: need(res.contest, 'RespondContest'),
    };
  }

  /** The choice of a won shove: knock prone or push 1,5 m. */
  async resolveShove(
    campaignId: string,
    encounterId: string,
    contestId: string,
    outcome: ShoveOutcome,
    key: string,
  ): Promise<ContestResult> {
    const res = await this.client.resolveShove({
      campaignId,
      encounterId,
      idempotencyKey: key,
      contestId,
      outcome,
    });
    return {
      encounter: need(res.encounter, 'ResolveShove'),
      contest: need(res.contest, 'ResolveShove'),
    };
  }

  /** The Hide action: the Dexterity (Stealth) check; the attempt waits for the master. */
  async hide(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    actionKey: string,
    die: CheckDie,
    key: string,
  ): Promise<{ readonly encounter: Encounter; readonly attempt: HideAttemptView }> {
    const res = await this.client.hide({
      campaignId,
      encounterId,
      idempotencyKey: key,
      combatantId,
      actionKey,
      roll: rollInput(die),
    });
    return {
      encounter: need(res.encounter, 'Hide'),
      attempt: need(res.attempt, 'Hide'),
    };
  }

  /** The Help action, for a check or for an ally's attack. */
  async help(
    input: HelpInput,
    key: string,
  ): Promise<{ readonly encounter: Encounter; readonly help: HelpView }> {
    const res = await this.client.help({
      campaignId: input.campaignId,
      encounterId: input.encounterId,
      idempotencyKey: key,
      combatantId: input.helperId,
      kind: input.kind,
      allyId: input.allyId,
      taskKey: input.taskKey,
      targetId: input.targetId,
    });
    return { encounter: need(res.encounter, 'Help'), help: need(res.help, 'Help') };
  }

  /** The open group check, or the latest closed one, as this player reads it; `null` when there is none. */
  async groupCheck(campaignId: string): Promise<GroupCheckView | null> {
    return (await this.client.getGroupCheck({ campaignId })).groupCheck ?? null;
  }

  /** The player's own roll for the open group check (once). Throws `InspirationAsked` when the character holds a Bardic
   * Inspiration die: the d20 is rolled and kept, and the session panel asks whether to use it. */
  async rollGroupCheck(
    campaignId: string,
    groupCheckId: string,
    die: CheckDie,
    key: string,
  ): Promise<GroupCheckView> {
    const res = await this.client.rollGroupCheck({
      campaignId,
      idempotencyKey: key,
      groupCheckId,
      roll: rollInput(die),
    });
    if (res.inspirationOffer) {
      // The roll waits for the answer about a Bardic Inspiration die: the session panel asks it.
      throw new InspirationAsked();
    }
    return need(res.groupCheck, 'RollGroupCheck');
  }

  // ---- the master's calls (W7-X): the same service, the other side of the table ----

  /** "Encerrar disputa": ends a contest that waits with no result; the action it spent stays spent. */
  async closeContest(
    campaignId: string,
    encounterId: string,
    contestId: string,
    key: string,
  ): Promise<Encounter> {
    const res = await this.client.closeContest({
      campaignId,
      encounterId,
      idempotencyKey: key,
      contestId,
    });
    return need(res.encounter, 'CloseContest');
  }

  /** "Soltar": lets the grappled creature go (the grappler whenever it likes, or the master). */
  async releaseGrapple(
    campaignId: string,
    encounterId: string,
    grappledId: string,
    key: string,
  ): Promise<Encounter> {
    const res = await this.client.releaseGrapple({
      campaignId,
      encounterId,
      idempotencyKey: key,
      grappledId,
    });
    return need(res.encounter, 'ReleaseGrapple');
  }

  /** The master's decision on a Hide: applied (with the creatures that see the hider clearly) or refused with a reason. */
  async resolveHide(
    campaignId: string,
    encounterId: string,
    decision: HideDecision,
    key: string,
  ): Promise<{ readonly encounter: Encounter; readonly attempt: HideAttemptView }> {
    const res = await this.client.resolveHide({
      campaignId,
      encounterId,
      idempotencyKey: key,
      attemptId: decision.attemptId,
      refuse: decision.refuse,
      refusal: decision.refusal,
      seesClearlyIds: [...decision.seesClearlyIds],
    });
    return {
      encounter: need(res.encounter, 'ResolveHide'),
      attempt: need(res.attempt, 'ResolveHide'),
    };
  }

  /** The master ends a combatant's hiding for every creature ("Revelar"): the circumstances revealed it. */
  async revealHider(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    key: string,
  ): Promise<Encounter> {
    const res = await this.client.revealHider({
      campaignId,
      encounterId,
      idempotencyKey: key,
      combatantId,
    });
    return need(res.encounter, 'RevealHider');
  }

  /** Takes a Help back (the master, when it should not hold any more). */
  async clearHelp(
    campaignId: string,
    encounterId: string,
    helpId: string,
    key: string,
  ): Promise<Encounter> {
    const res = await this.client.clearHelp({
      campaignId,
      encounterId,
      idempotencyKey: key,
      helpId,
    });
    return need(res.encounter, 'ClearHelp');
  }

  /** Marks a creature surprised or not, before the combat begins. */
  async setSurprised(
    campaignId: string,
    encounterId: string,
    combatantId: string,
    surprised: boolean,
    key: string,
  ): Promise<{ readonly encounter: Encounter; readonly surprise: SurpriseView | undefined }> {
    const res = await this.client.setSurprised({
      campaignId,
      encounterId,
      idempotencyKey: key,
      combatantId,
      surprised,
    });
    return { encounter: need(res.encounter, 'SetSurprised'), surprise: res.surprise };
  }

  /** The app's suggestion of who is surprised, one entry for each combatant that is not defeated. */
  async surpriseSuggestion(
    campaignId: string,
    encounterId: string,
  ): Promise<readonly SurpriseSuggestion[]> {
    return (await this.client.getSurpriseSuggestion({ campaignId, encounterId })).suggestions;
  }

  /**
   * Asks for a check or a saving throw (`skillKey`: "skill:x", "ability:x" or "save:x"). `characterIds` empty asks every
   * living character; `group` is the "Working Together" verdict (unset: the server decides). `dc` 0 is none; `showDc`
   * lets the players read passed and failed.
   */
  async requestGroupCheck(
    campaignId: string,
    request: {
      readonly skillKey: string;
      readonly dc: number;
      readonly showDc: boolean;
      readonly characterIds?: readonly string[];
      readonly group?: boolean;
    },
    key: string,
  ): Promise<GroupCheckView> {
    const res = await this.client.requestGroupCheck({
      campaignId,
      idempotencyKey: key,
      skillKey: request.skillKey,
      dc: request.dc,
      showDc: request.showDc,
      characterIds: [...(request.characterIds ?? [])],
      group: request.group,
    });
    return need(res.groupCheck, 'RequestGroupCheck');
  }

  /** The master's roll for a character that has not answered. */
  async rollForPlayer(
    campaignId: string,
    groupCheckId: string,
    characterId: string,
    die: CheckDie,
    key: string,
  ): Promise<GroupCheckView> {
    const res = await this.client.rollForPlayer({
      campaignId,
      idempotencyKey: key,
      groupCheckId,
      characterId,
      roll: rollInput(die),
    });
    return need(res.groupCheck, 'RollForPlayer');
  }

  /** Ends the group check: whoever did not answer counts as failed, and the verdict is worked out. */
  async closeGroupCheck(
    campaignId: string,
    groupCheckId: string,
    key: string,
  ): Promise<GroupCheckView> {
    const res = await this.client.closeGroupCheck({
      campaignId,
      idempotencyKey: key,
      groupCheckId,
    });
    return need(res.groupCheck, 'CloseGroupCheck');
  }
}
