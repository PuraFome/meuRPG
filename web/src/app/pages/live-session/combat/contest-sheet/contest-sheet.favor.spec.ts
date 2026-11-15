import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CheckOptionSchema,
  ContestAttackOptionKind,
  ContestAttackOptionSchema,
  ContestPurpose,
  ContestTargetSchema,
  RollModeKind,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import { CreatureSize } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import { FakeContestClient, textOf } from '../../../../core/combat/contest-testing';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { ContestSheet, type ContestSheetData } from './contest-sheet';

const table = () =>
  encounter({
    combatants: [
      combatant({
        id: 't',
        label: 'Toren',
        kind: CombatantKind.PLAYER,
        side: CombatantSide.PARTY,
        size: CreatureSize.MEDIUM,
      } as never),
      combatant({ id: 'h', label: 'Hobgoblin' }),
    ],
  });

describe('ContestSheet: the Inspiração on the initiator’s d20', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  async function setupAtRoll(holders: Record<string, string> = { 'char-t': 'Toren' }) {
    TestBed.resetTestingModule();
    const api = new FakeContestClient();
    api.encounterAnswer = table();
    const state = new CombatState();
    state.encounter.set(table());
    const data: ContestSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      initiatorId: 't',
      purpose: ContestPurpose.GRAPPLE,
      attack: create(ContestAttackOptionSchema, {
        kind: ContestAttackOptionKind.GRAPPLE,
        replacesAttack: true,
        enabled: true,
        targets: [
          create(ContestTargetSchema, {
            combatantId: 'h',
            size: CreatureSize.MEDIUM,
            distanceFt: 5,
            eligible: true,
          }),
        ],
        rollOption: create(CheckOptionSchema, {
          modifier: 5,
          known: true,
          mode: RollModeKind.NORMAL,
        }),
      }),
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
      contests: new ContestState(),
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
    const fixture = TestBed.createComponent(ContestSheet);
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
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    button('Rolar a disputa')!.click();
    await settle();
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
    return { fixture, el, api, button, settle, box };
  }

  it('shows no box when the character has none', async () => {
    expect((await setupAtRoll({})).box()).toBeNull();
  });

  it('sends spend_favor when ticked and nothing about it when not', async () => {
    const off = await setupAtRoll();
    off.button('Rolar no app')!.click();
    await off.settle();
    expect(off.api.started[0].input.spendFavor).toBeUndefined();

    const on = await setupAtRoll();
    on.box()!.click();
    on.fixture.detectChanges();
    expect(textOf(on.el)).toContain('pela Inspiração');
    on.button('Rolar')!.click();
    await on.settle();
    expect(on.api.started[0].input.spendFavor).toBe(true);
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = await setupAtRoll();
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    s.button('no app')!.click();
    await s.settle();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('says the refusal and unticks the box', async () => {
    const s = await setupAtRoll();
    s.api.error = favorRefused(FavorBlockedReason.ADDS_NOTHING);
    s.box()!.click();
    s.fixture.detectChanges();
    s.button('no app')!.click();
    await s.settle();
    expect(textOf(s.el.querySelector('[role=alert]')!)).toContain(
      'A Inspiração não mudaria esta rolagem.',
    );
    expect(s.box()!.checked).toBe(false);
  });
});
