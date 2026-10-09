import { TestBed } from '@angular/core/testing';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Subject } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, ReactionKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ContestStatus,
  GrappleViewSchema,
  HideAttemptStatus,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { ContestClient } from '../../../../core/combat/contest-client';
import {
  FakeContestClient,
  checkRoll,
  contestState,
  contestView,
  hideAttempt,
  skillOption,
  textOf,
} from '../../../../core/combat/contest-testing';
import { ContestMaster } from './contest-master';

const combatants = [
  combatant({ id: 't', label: 'Toren', kind: CombatantKind.PLAYER }),
  combatant({ id: 'b', label: 'Brisa', kind: CombatantKind.PLAYER }),
  combatant({ id: 'h', label: 'Hobgoblin' }),
  combatant({ id: 'g1', label: 'Goblin 1' }),
];

const contestWindow = (id: string, contestId: string, forYou = true) =>
  reactionWindow({
    id,
    kind: ReactionKind.CONTEST,
    forYou,
    prompt: { case: 'contest', value: { contestId } } as never,
  });

function setup(over: Record<string, unknown> = {}, facts = contestState()) {
  const api = new FakeContestClient();
  api.stateResponse = facts;
  const opened = vi.fn(() => {
    const done = new Subject<undefined>();
    return { afterClosed: () => done, afterDismissed: () => done };
  });
  TestBed.configureTestingModule({
    providers: [
      { provide: ContestClient, useValue: api.as() },
      { provide: MatDialog, useValue: { open: opened } },
      { provide: MatBottomSheet, useValue: { open: opened } },
    ],
  });
  const fixture = TestBed.createComponent(ContestMaster);
  const e = encounter({ combatants, currentCombatantId: 'h', ...over } as never);
  const state = new CombatState();
  state.encounter.set(e);
  const ref = fixture.componentRef;
  ref.setInput('encounter', e);
  ref.setInput('campaignId', 'camp');
  ref.setInput('state', state);
  ref.setInput('diceMode', DiceMode.PLAYERS_CHOOSE);
  ref.setInput('preference', DicePreference.APP);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const settle = async () => {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  const button = (name: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      textOf(b).includes(name),
    );
  return { api, fixture, el, settle, button, opened, state };
}

describe('ContestMaster, the master side of the contests of a combat', () => {
  it('reads the contest facts of the combat as the master', async () => {
    const { api, settle } = setup();
    await settle();
    expect(api.calls).toContain('state');
  });

  it('draws a card for each CONTEST window that is his, and none for the ones that are not', async () => {
    const waiting = contestView({
      id: 'ct1',
      youAnswer: true,
      initiatorRoll: checkRoll(),
      answerOptions: [skillOption({ suggested: true })],
    });
    const { el, settle } = setup(
      {
        reactionWindows: [
          contestWindow('w1', 'ct1'),
          contestWindow('w2', 'ct2', false),
          contestWindow('w3', 'unknown'),
        ],
      },
      contestState({ contests: [waiting, contestView({ id: 'ct2' })] }),
    );
    await settle();
    const titles = Array.from(el.querySelectorAll('app-contest-card h2')).map((h) => textOf(h));
    expect(titles).toEqual(['Toren tenta agarrar o Hobgoblin']);
  });

  it('draws a card for each Hide that waits', async () => {
    const { el, settle } = setup(
      {},
      contestState({
        hideAttempts: [
          hideAttempt({
            id: 'a',
            hiderId: 'b',
            status: HideAttemptStatus.PENDING,
            roll: checkRoll({ total: 19 }),
          }),
          hideAttempt({ id: 'z', hiderId: 'b', status: HideAttemptStatus.APPLIED }),
        ],
      }),
    );
    await settle();
    const titles = Array.from(el.querySelectorAll('app-hide-master-card h2')).map((h) => textOf(h));
    expect(titles).toEqual(['Brisa tenta se esconder: Furtividade 19']);
  });

  it('lists who holds whom, with the escape DC only he reads, and "Soltar" lets go once', async () => {
    const { api, el, settle, button } = setup(
      { currentCombatantId: 't' },
      contestState({
        grapples: [create(GrappleViewSchema, { grappledId: 'b', grapplerId: 'h', escapeDc: 16 })],
      }),
    );
    await settle();
    const text = textOf(el);
    expect(text).toContain('Agarrões');
    expect(text).toContain('O Hobgoblin segura Brisa');
    expect(text).toContain('CD de escape 16');
    button('Soltar')!.click();
    await settle();
    expect(api.released).toEqual([{ grappledId: 'b', key: expect.any(String) }]);
    expect(textOf(el)).toContain('Agarrão solto.');
  });

  it('offers "Agarrar ou empurrar por um NPC" on an NPC turn only, and opens the sheet', async () => {
    const npc = setup();
    await npc.settle();
    npc.button('Agarrar ou empurrar por um NPC')!.click();
    expect(npc.opened).toHaveBeenCalledTimes(1);
    TestBed.resetTestingModule();
    const player = setup({ currentCombatantId: 't' });
    await player.settle();
    expect(player.button('Agarrar ou empurrar por um NPC')).toBeUndefined();
    expect(player.el.querySelector('[data-testid="grapples"]')).toBeNull();
  });

  it('draws nothing when the combat is not active', async () => {
    const { el, settle, api } = setup({ status: 1 });
    await settle();
    expect(api.calls).not.toContain('state');
    expect(textOf(el)).toBe('');
  });

  it('a settled contest has no card even if its window were still listed', async () => {
    const { el, settle } = setup(
      { reactionWindows: [contestWindow('w1', 'ct1')] },
      contestState({
        contests: [contestView({ id: 'ct1', status: ContestStatus.RESOLVED, youAnswer: false })],
      }),
    );
    await settle();
    expect(el.querySelectorAll('app-contest-card')).toHaveLength(0);
  });
});
