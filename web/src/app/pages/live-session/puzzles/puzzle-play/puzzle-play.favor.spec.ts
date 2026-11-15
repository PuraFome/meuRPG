import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import { textOf } from '../../../../core/format/text-testing';
import { SceneChecks } from '../../../../core/maps/scene-actions';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { PuzzleSessionState } from '../../../../core/puzzles/puzzle-session';
import { PuzzlesClient } from '../../../../core/puzzles/puzzles-client';
import {
  FakePuzzlesClient,
  NOW,
  asClient,
  fakeChecks,
  hintAnswer,
  playerRun,
  riddlePuzzle,
} from '../../../../core/puzzles/puzzles-testing';
import { nextTurn } from '../../../../../test-settle';
import { PuzzlePlayPage } from './puzzle-play';

afterEach(() => {
  vi.useRealTimers();
});

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [PuzzlePlayPage],
  template: `<app-puzzle-play campaignId="camp-1" puzzleId="a" [session]="session" [state]="state" ownName="Toren" [diceMode]="1" [dicePreference]="1" [reconnecting]="reconnecting()" />`,
})
class Host {
  session = { sessionId: 's7', sessionNumber: 7, startedAt: new Date() };
  state!: PuzzleSessionState;
  reconnecting = signal(false);
}

describe('PuzzlePlayPage: the Inspiração on a hint by a skill check', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(NOW);
  });

  const run = (partial = {}) =>
    playerRun(riddlePuzzle('a', 'A porta da Cripta pergunta'), {
      hintByCheck: true,
      hintSkillKey: 'skill:investigation',
      canTryHint: true,
      ...partial,
    });

  async function render(holders: Record<string, string> = { pens: 'Toren' }) {
    TestBed.resetTestingModule();
    const api = new FakePuzzlesClient();
    api.playerRunResult = run();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: PuzzlesClient, useValue: api },
        { provide: SceneChecks, useValue: fakeChecks },
        { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
      ],
    });
    openFavor(TestBed.inject(FavorState), holders);
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.state = new PuzzleSessionState(
      asClient(api),
      () => 'camp-1',
      () => false,
    );
    document.body.append(fixture.nativeElement);
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        fixture.detectChanges();
        await fixture.whenStable();
        await nextTurn();
      }
      fixture.detectChanges();
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
    const tryHint = async () => {
      [...el.querySelectorAll<HTMLElement>('button')]
        .find((b) => b.textContent?.includes('Tentar uma dica'))!
        .click();
      await settle();
    };
    return { api, el, box, settle, tryHint, fixture };
  }

  it('shows no box when the player has none', async () => {
    expect((await render({})).box()).toBeNull();
  });

  it('sends spend_favor when ticked and nothing about it when not', async () => {
    const off = await render();
    off.api.hintResult = () => hintAnswer(run({ revision: 2, canTryHint: false }), false, 9);
    await off.tryHint();
    expect(off.api.hintSpends).toEqual([false]);

    const on = await render();
    on.api.hintResult = () => hintAnswer(run({ revision: 2, canTryHint: false }), false, 9);
    on.box()!.click();
    on.fixture.detectChanges();
    await on.tryHint();
    expect(on.api.hintSpends).toEqual([true]);
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = await render();
    s.api.hintResult = () => hintAnswer(run({ revision: 2, canTryHint: false }), false, 9);
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    await s.tryHint();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('says the refusal and unticks the box', async () => {
    const s = await render();
    s.api.hintResult = () => {
      throw favorRefused(FavorBlockedReason.ADDS_NOTHING);
    };
    s.box()!.click();
    s.fixture.detectChanges();
    await s.tryHint();
    expect(textOf(s.el)).toContain('A Inspiração não mudaria esta rolagem.');
    expect(s.box()!.checked).toBe(false);
  });
});
