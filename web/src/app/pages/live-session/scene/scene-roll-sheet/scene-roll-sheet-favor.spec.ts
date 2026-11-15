import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { FavorBlockedReason } from '../../../../../gen/meurpg/play/v1/favor_pb';
import { SceneRollSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { FavorClient } from '../../../../core/play/favor-client';
import { FavorState } from '../../../../core/play/favor-state';
import { FakeFavorClient, favorRefused, openFavor } from '../../../../core/play/favor-testing';
import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, playerScene } from '../../../../core/play/scene-testing';
import { SceneRollSheet, type SceneRollSheetData } from './scene-roll-sheet';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();
const made = create(SceneRollSchema, {
  id: 'r1',
  actionId: 'a1',
  characterName: 'Pensantus',
  roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 17 },
  mode: RollMode.NORMAL,
});

describe('SceneRollSheet: the Inspiração', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(holders: Record<string, string> = { pens: 'Pensantus' }, fail?: unknown) {
    TestBed.resetTestingModule();
    const api = new FakeSceneClient();
    const sent: { key: string; spend: boolean | undefined; maneuver: string | undefined }[] = [];
    api.roll = (async (
      _c: string,
      _a: string,
      _die: unknown,
      key: string,
      _extra?: readonly number[],
      maneuver?: string,
      spend?: boolean,
    ) => {
      sent.push({ key, spend, maneuver });
      if (fail) {
        throw fail;
      }
      return made;
    }) as never;
    const state = new SceneState(
      () => api.get(),
      () => false,
    );
    const data: SceneRollSheetData = {
      campaignId: 'c1',
      action: playerScene().actions[0],
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: SceneClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
        { provide: FavorClient, useValue: new FakeFavorClient().asClient() },
      ],
    });
    openFavor(TestBed.inject(FavorState), holders);
    const fixture = TestBed.createComponent(SceneRollSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const box = () => el.querySelector<HTMLInputElement>('app-favor-spend input[type=checkbox]');
    const rollInApp = async () => {
      [...el.querySelectorAll('button')]
        .find((b) => plain(b.textContent).includes('Rolar no app'))!
        .click();
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    return { fixture, el, sent, box, rollInApp };
  }

  it('shows no box when the player has none', () => {
    expect(setup({}).box()).toBeNull();
  });

  it('sends spend_favor when ticked and nothing about it when not', async () => {
    const off = setup();
    await off.rollInApp();
    expect(off.sent[0].spend).toBe(false);

    const on = setup();
    on.box()!.click();
    on.fixture.detectChanges();
    await on.rollInApp();
    expect(on.sent[0].spend).toBe(true);
  });

  it('unticks the box after the spend and reads who has the Inspiração again', async () => {
    const s = setup();
    const refresh = vi.spyOn(TestBed.inject(FavorState), 'refresh').mockResolvedValue();
    s.box()!.click();
    s.fixture.detectChanges();
    await s.rollInApp();
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('says the refusal and unticks the box', async () => {
    const s = setup({ pens: 'Pensantus' }, favorRefused(FavorBlockedReason.ADDS_NOTHING));
    s.box()!.click();
    s.fixture.detectChanges();
    await s.rollInApp();
    expect(plain(s.el.querySelector('[role=alert]')?.textContent)).toContain(
      'A Inspiração não mudaria esta rolagem.',
    );
    expect(s.box()!.checked).toBe(false);
  });
});
