import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ContestSkill } from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import {
  FakeContestClient,
  contestView,
  skillOption,
  textOf,
} from '../../../../core/combat/contest-testing';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { ContestAnswerSheet, type ContestAnswerData } from './contest-answer-sheet';

const table = () =>
  encounter({
    combatants: [
      combatant({ id: 'h', label: 'Hobgoblin' }),
      combatant({ id: 'b', label: 'Brisa', kind: CombatantKind.PLAYER, side: CombatantSide.PARTY }),
    ],
  });

describe('ContestAnswerSheet: the Inspiração on the defender’s d20', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(holders: Record<string, string> = { 'char-b': 'Brisa' }) {
    TestBed.resetTestingModule();
    const api = new FakeContestClient();
    api.encounterAnswer = table();
    const contest = contestView({
      initiatorId: 'h',
      defenderId: 'b',
      youAnswer: true,
      answerOptions: [
        skillOption({ skill: ContestSkill.ATHLETICS, modifier: 0 }),
        skillOption({ skill: ContestSkill.ACROBATICS, modifier: 7 }),
      ],
    });
    const state = new CombatState();
    state.encounter.set(table());
    const contests = new ContestState();
    contests.applyContest(contest);
    const data: ContestAnswerData = {
      campaignId: 'c',
      encounterId: 'enc',
      contestId: contest.id,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
      contests,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: ContestClient, useValue: api.as() },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
        { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
      ],
    });
    openFavor(TestBed.inject(FavorState), holders);
    const fixture = TestBed.createComponent(ContestAnswerSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const button = (name: string) =>
      [...el.querySelectorAll<HTMLButtonElement>('button')].find((b) =>
        b.textContent?.includes(name),
      );
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
    return { fixture, el, api, button, settle, box };
  }

  it('shows no box when the character has none', () => {
    expect(setup({}).box()).toBeNull();
  });

  it('sends spend_favor when ticked and nothing about it when not', async () => {
    const off = setup();
    off.button('Rolar no app')!.click();
    await off.settle();
    expect(off.api.responded[0].input.spendFavor).toBeUndefined();

    const on = setup();
    on.box()!.click();
    on.fixture.detectChanges();
    expect(textOf(on.el)).toContain('pela Inspiração');
    on.button('no app')!.click();
    await on.settle();
    expect(on.api.responded[0].input.spendFavor).toBe(true);
  });

  it('never spends when the roll is left to the master', async () => {
    const s = setup();
    s.box()!.click();
    s.fixture.detectChanges();
    s.button('Deixar o mestre rolar por mim')!.click();
    await s.settle();
    expect(s.api.responded[0].input.spendFavor).toBeUndefined();
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = setup();
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    s.button('no app')!.click();
    await s.settle();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('says the refusal and unticks the box', async () => {
    const s = setup();
    s.api.error = favorRefused(FavorBlockedReason.HAS_NONE);
    s.box()!.click();
    s.fixture.detectChanges();
    s.button('no app')!.click();
    await s.settle();
    expect(textOf(s.el.querySelector('[role=alert]')!)).toContain('Você não tem Inspiração.');
    expect(s.box()?.checked ?? false).toBe(false);
  });
});
