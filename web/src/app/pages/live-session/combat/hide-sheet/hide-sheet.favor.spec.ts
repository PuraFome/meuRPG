import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CheckOptionSchema,
  RollModeKind,
  RollNoteSchema,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import { FakeContestClient, textOf } from '../../../../core/combat/contest-testing';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { HideSheet, type HideSheetData } from './hide-sheet';

const table = () =>
  encounter({
    combatants: [
      combatant({ id: 'b', label: 'Brisa', kind: CombatantKind.PLAYER, side: CombatantSide.PARTY }),
    ],
  });

describe('HideSheet: the Inspiração (a check sheet, 581-07)', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(
    opts: { holders?: Record<string, string>; notes?: unknown[]; mode?: RollModeKind } = {},
  ) {
    TestBed.resetTestingModule();
    const api = new FakeContestClient();
    api.encounterAnswer = table();
    const state = new CombatState();
    state.encounter.set(table());
    const data: HideSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      combatantId: 'b',
      actionKey: 'standard:hide',
      economy: 'Ação',
      option: create(CheckOptionSchema, {
        modifier: 7,
        known: true,
        mode: opts.mode ?? RollModeKind.NORMAL,
        notes: (opts.notes ?? []) as never,
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
    openFavor(TestBed.inject(FavorState), opts.holders ?? { 'char-b': 'Brisa' });
    const fixture = TestBed.createComponent(HideSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(name),
      );
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]')!;
    return { fixture, el, api, button, settle, box };
  }

  it('shows no box when the character has none', () => {
    expect(setup({ holders: {} }).el.querySelector('app-favor-spend .favor')).toBeNull();
  });

  it('sends spend_favor when ticked, and nothing about it when not', async () => {
    const plain = setup();
    plain.button('Rolar no app')!.click();
    await plain.settle();
    expect(plain.api.hides).toHaveLength(1);
    expect(plain.api.hides[0].spendFavor).toBeUndefined();

    const s = setup();
    s.box().click();
    s.fixture.detectChanges();
    expect(textOf(s.el)).toContain('pela Inspiração');
    s.button('no app')!.click();
    await s.settle();
    expect(s.api.hides[0].spendFavor).toBe(true);
  });

  it('is dashed with the reason when the check already has advantage', () => {
    const s = setup({
      mode: RollModeKind.ADVANTAGE,
      notes: [create(RollNoteSchema, { kind: 'help', labelPt: 'Ajuda de Tor', advantage: true })],
    });
    expect(s.box().getAttribute('aria-disabled')).toBe('true');
    expect(textOf(s.el)).toContain('ela já tem vantagem (Ajuda de Tor)');
  });

  it('says the refusal and unticks the box', async () => {
    const s = setup();
    s.api.error = favorRefused(FavorBlockedReason.HAS_NONE);
    s.box().click();
    s.fixture.detectChanges();
    s.button('no app')!.click();
    await s.settle();
    expect(textOf(s.el.querySelector('[role=alert]')!)).toContain('Você não tem Inspiração.');
    expect(s.box()?.checked ?? false).toBe(false);
  });
});
