import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CheckOptionSchema,
  GroupCheckMemberViewSchema,
  RollModeKind,
} from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { FavorBlockedReason } from '../../../../gen/meurpg/play/v1/favor_pb';
import { ContestClient } from '../../../core/combat/contest-client';
import { FakeContestClient, groupCheck, textOf } from '../../../core/combat/contest-testing';
import { GroupCheckState } from '../../../core/combat/group-check-state';
import { FavorClient } from '../../../core/play/favor-client';
import { FavorState } from '../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../core/play/favor-testing';
import { GroupCheckSheet, type GroupCheckSheetData } from './group-check-sheet';

const view = () =>
  groupCheck({
    youRoll: true,
    yourOption: create(CheckOptionSchema, { modifier: 7, known: true, mode: RollModeKind.NORMAL }),
    members: [create(GroupCheckMemberViewSchema, { characterId: 'b', name: 'Brisa' })],
  });

describe('GroupCheckSheet: the Inspiração', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(holders: Record<string, string> = { b: 'Brisa' }) {
    TestBed.resetTestingModule();
    const api = new FakeContestClient();
    const state = new GroupCheckState();
    state.apply(view());
    const data: GroupCheckSheetData = {
      campaignId: 'c',
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
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
    const fixture = TestBed.createComponent(GroupCheckSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
    const roll = async () => {
      [...el.querySelectorAll<HTMLButtonElement>('button')]
        .find((b) => textOf(b).includes('no app'))!
        .click();
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    return { fixture, el, api, box, roll };
  }

  it('shows no box when the player has none', () => {
    expect(setup({}).box()).toBeNull();
  });

  it('sends spend_favor when ticked and nothing about it when not', async () => {
    const off = setup();
    await off.roll();
    expect(off.api.groupRolls[0].spendFavor).toBeUndefined();

    const on = setup();
    on.box()!.click();
    on.fixture.detectChanges();
    expect(textOf(on.el)).toContain('pela Inspiração');
    await on.roll();
    expect(on.api.groupRolls[0].spendFavor).toBe(true);
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = setup();
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    await s.roll();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('says the refusal and unticks the box', async () => {
    const s = setup();
    s.api.error = favorRefused(FavorBlockedReason.HAS_NONE);
    s.box()!.click();
    s.fixture.detectChanges();
    await s.roll();
    expect(textOf(s.el.querySelector('[role=alert]')!)).toContain('Você não tem Inspiração.');
    expect(s.box()?.checked ?? false).toBe(false);
  });
});
