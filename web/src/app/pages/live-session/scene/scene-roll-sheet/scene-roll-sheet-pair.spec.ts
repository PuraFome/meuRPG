import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  AdvantageSourceKind,
  AdvantageSourceSchema,
  RollMode,
} from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import { SceneRollSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { SceneClient, type SceneDie } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, playerScene } from '../../../../core/play/scene-testing';
import { SceneRollSheet, type SceneRollSheetData } from './scene-roll-sheet';

const pairRoll = create(SceneRollSchema, {
  id: 'r1',
  actionId: 'a1',
  characterName: 'Pensantus',
  roll: {
    diceCount: 2,
    diceSides: 20,
    faces: [7, 15],
    modifier: 6,
    total: 21,
    countedIndex: 1,
    physical: true,
  },
  mode: RollMode.ADVANTAGE,
  sources: [
    create(AdvantageSourceSchema, {
      kind: AdvantageSourceKind.RAGE_STRENGTH,
      effect: RollMode.ADVANTAGE,
      textPt: 'Fúria: vantagem em testes de Força',
    }),
  ],
});

describe('SceneRollSheet: a roll that takes two d20', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(diceMode = DiceMode.PHYSICAL) {
    const api = new FakeSceneClient();
    const sent: { die: SceneDie; key: string }[] = [];
    api.roll = async (_c: string, _a: string, die: SceneDie, key: string) => {
      sent.push({ die, key });
      if (!('faces' in die) && !('inApp' in die)) {
        throw new ConnectError(
          'the roll takes 2 d20: type 2 face(s) in d20_faces',
          Code.InvalidArgument,
        );
      }
      return pairRoll;
    };
    const state = new SceneState(
      () => api.get(),
      () => false,
    );
    const data: SceneRollSheetData = {
      campaignId: 'c1',
      action: playerScene().actions[0],
      diceMode,
      preference: DicePreference.PHYSICAL,
      state,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: SceneClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(SceneRollSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      for (let i = 0; i < 3; i++) {
        await fixture.whenStable();
        fixture.detectChanges();
      }
    };
    const fill = (index: number, text: string) => {
      const field = el.querySelectorAll<HTMLInputElement>('input')[index];
      field.value = text;
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
    };
    return { el, sent, settle, fill, fixture };
  }

  it('starts with one field, switches to two when the server asks, and sends both faces', async () => {
    const { el, sent, settle, fill, fixture } = setup();
    expect(el.querySelectorAll('input')).toHaveLength(1);
    fill(0, '7');
    el.querySelector<HTMLButtonElement>('button[type="submit"], .pick__main')?.click();
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => b.textContent?.includes('Confirmar'))!
      .click();
    await settle();

    expect(sent[0].die).toEqual({ face: 7 });
    expect(el.querySelector('[role="alert"]')).toBeNull();
    expect(el.textContent).toContain('leva 2 d20');
    const labels = Array.from(el.querySelectorAll('label'), (l) => l.textContent?.trim());
    expect(labels).toEqual(['Primeiro d20', 'Segundo d20']);

    fill(0, '7');
    fill(1, '15');
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await settle();

    expect(sent[1].die).toEqual({ faces: [7, 15] });
    expect(sent[1].key).not.toBe(sent[0].key);
    expect(el.querySelector('.res__sum')?.textContent).toBe('21');
    fixture.detectChanges();
  });

  it('shows the mode, both dice with the counted one in words and the sources after the roll', async () => {
    const { el, settle, fill } = setup();
    fill(0, '7');
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => b.textContent?.includes('Confirmar'))!
      .click();
    await settle();
    fill(0, '7');
    fill(1, '15');
    el.querySelector('form')!.dispatchEvent(new Event('submit'));
    await settle();

    expect(el.querySelector('.mode__word')?.textContent).toBe('Vantagem');
    const dice = Array.from(el.querySelectorAll('.die'), (d) =>
      d.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(dice).toEqual(['7 não conta', '15 conta']);
    expect(el.querySelector('.mode__sources')?.textContent).toContain(
      'Fúria: vantagem em testes de Força',
    );
    // The formula is the die that counts.
    expect(el.querySelector('.res__formula')?.textContent).toContain('15');
  });
});
