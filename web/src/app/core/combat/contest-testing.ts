import { type MessageInitShape, create } from '@bufbuild/protobuf';

import type { Encounter } from '../../../gen/meurpg/play/v1/combat_pb';
import {
  type CheckRoll,
  CheckRollSchema,
  type ContestSkillOption,
  ContestSkillOptionSchema,
  type ContestView,
  ContestKind,
  ContestPurpose,
  ContestSkill,
  ContestStatus,
  ContestViewSchema,
  ContestWaitFor,
  type GroupCheckView,
  GroupCheckViewSchema,
  type HelpView,
  HelpViewSchema,
  type HideAttemptView,
  HideAttemptViewSchema,
  RollModeKind,
} from '../../../gen/meurpg/play/v1/contest_types_pb';
import {
  type GetContestStateResponse,
  GetContestStateResponseSchema,
} from '../../../gen/meurpg/play/v1/contests_pb';
import type {
  ContestClient,
  HelpInput,
  RespondContestInput,
  StartContestInput,
} from './contest-client';
import { encounter } from './combat-testing';

/** A d20 check as a contest, Hide or a group check rolled it: the d20 that counts, the modifier and the total. */
export function checkRoll(over: MessageInitShape<typeof CheckRollSchema> = {}): CheckRoll {
  return create(CheckRollSchema, {
    skill: ContestSkill.ATHLETICS,
    skillKey: 'skill:athletics',
    faces: [15],
    modifier: 5,
    total: 20,
    mode: RollModeKind.NORMAL,
    ...over,
  });
}

/** A contest of Toren (`t`) against the Hobgoblin (`h`) that waits for the master's roll, unless `over` says otherwise. */
export function contestView(over: MessageInitShape<typeof ContestViewSchema> = {}): ContestView {
  return create(ContestViewSchema, {
    id: 'ct1',
    kind: ContestKind.CONTEST,
    purpose: ContestPurpose.GRAPPLE,
    status: ContestStatus.AWAITING_DEFENDER,
    initiatorId: 't',
    defenderId: 'h',
    waitingFor: ContestWaitFor.MASTER,
    ...over,
  });
}

export function skillOption(
  over: MessageInitShape<typeof ContestSkillOptionSchema> = {},
): ContestSkillOption {
  return create(ContestSkillOptionSchema, {
    skill: ContestSkill.ATHLETICS,
    modifier: 0,
    known: true,
    mode: RollModeKind.NORMAL,
    ...over,
  });
}

export function hideAttempt(over: MessageInitShape<typeof HideAttemptViewSchema> = {}): HideAttemptView {
  return create(HideAttemptViewSchema, { id: 'hd1', hiderId: 'b', ...over });
}

export function helpView(over: MessageInitShape<typeof HelpViewSchema> = {}): HelpView {
  return create(HelpViewSchema, { id: 'hp1', helperId: 'o', allyId: 'v', ...over });
}

export function groupCheck(over: MessageInitShape<typeof GroupCheckViewSchema> = {}): GroupCheckView {
  return create(GroupCheckViewSchema, {
    id: 'gc1',
    skillKey: 'skill:stealth',
    skillNamePt: 'Furtividade',
    open: true,
    ...over,
  });
}

export function contestState(
  over: MessageInitShape<typeof GetContestStateResponseSchema> = {},
): GetContestStateResponse {
  return create(GetContestStateResponseSchema, over);
}

/** A `ContestClient` for specs: every call is recorded with its request and key, and answers what the spec set. */
export class FakeContestClient {
  readonly calls: string[] = [];
  readonly started: { input: StartContestInput; key: string }[] = [];
  readonly responded: { input: RespondContestInput; key: string }[] = [];
  readonly shoves: { contestId: string; outcome: number; key: string }[] = [];
  readonly hides: { actionKey: string; die: unknown; key: string }[] = [];
  readonly helps: { input: HelpInput; key: string }[] = [];
  readonly groupRolls: { die: unknown; key: string }[] = [];
  /** What the next calls answer; a spec sets them, or `error` to make the call fail. */
  contest: ContestView = contestView();
  attempt: HideAttemptView = hideAttempt();
  group: GroupCheckView | null = groupCheck();
  stateResponse: GetContestStateResponse = contestState();
  encounterAnswer: Encounter = encounter();
  error: Error | null = null;

  private fail(): void {
    if (this.error) {
      throw this.error;
    }
  }

  state(): Promise<GetContestStateResponse> {
    this.calls.push('state');
    return Promise.resolve(this.stateResponse);
  }

  start(input: StartContestInput, key: string) {
    this.calls.push('start');
    this.started.push({ input, key });
    this.fail();
    return Promise.resolve({ encounter: this.encounterAnswer, contest: this.contest });
  }

  respond(input: RespondContestInput, key: string) {
    this.calls.push('respond');
    this.responded.push({ input, key });
    this.fail();
    return Promise.resolve({ encounter: this.encounterAnswer, contest: this.contest });
  }

  resolveShove(_c: string, _e: string, contestId: string, outcome: number, key: string) {
    this.calls.push('resolveShove');
    this.shoves.push({ contestId, outcome, key });
    this.fail();
    return Promise.resolve({ encounter: this.encounterAnswer, contest: this.contest });
  }

  hide(_c: string, _e: string, _who: string, actionKey: string, die: unknown, key: string) {
    this.calls.push('hide');
    this.hides.push({ actionKey, die, key });
    this.fail();
    return Promise.resolve({ encounter: this.encounterAnswer, attempt: this.attempt });
  }

  help(input: HelpInput, key: string) {
    this.calls.push('help');
    this.helps.push({ input, key });
    this.fail();
    return Promise.resolve({ encounter: this.encounterAnswer, help: helpView() });
  }

  groupCheck(): Promise<GroupCheckView | null> {
    this.calls.push('groupCheck');
    return Promise.resolve(this.group);
  }

  rollGroupCheck(_c: string, _id: string, die: unknown, key: string) {
    this.calls.push('rollGroupCheck');
    this.groupRolls.push({ die, key });
    this.fail();
    return Promise.resolve(this.group as GroupCheckView);
  }

  /** For `{ provide: ContestClient, useValue: fake.as() }`. */
  as(): ContestClient {
    return this as unknown as ContestClient;
  }
}

/** What a person reads of an element: the text with a space between the blocks and no icon names, spaces squeezed. */
export function textOf(el: Element): string {
  const parts: string[] = [];
  const inline = new Set(['B', 'STRONG', 'I', 'EM', 'A', 'SMALL']);
  const walk = (node: Node): void => {
    if (node.nodeType === Node.TEXT_NODE) {
      parts.push(node.textContent ?? '');
      return;
    }
    if (node.nodeName === 'MAT-ICON') {
      return;
    }
    node.childNodes.forEach(walk);
    if (!inline.has(node.nodeName)) {
      parts.push(' ');
    }
  };
  walk(el);
  return parts.join('').replace(/\u00a0/g, ' ').replace(/\s+/g, ' ').trim();
}
