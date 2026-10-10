import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { AnswerOutsideInspirationResponseSchema } from '../../../../../gen/meurpg/play/v1/resources_pb';
import { RollSceneCheckResponseSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import {
  FakeSceneClient,
  inspirationOffer,
  playerScene,
  sceneRoll,
} from '../../../../core/play/scene-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { SceneRollSheet, type SceneRollSheetData } from './scene-roll-sheet';

describe('SceneRollSheet with a Bardic Inspiration die', () => {
  const used = sceneRoll('r1', 'a1', 'Tavo', 22, {
    roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 22 },
    bonusDice: [{ sourceKey: 'feature:bardic-inspiration-d6', sides: 8, face: 5, used: true }],
  });
  const answered = create(AnswerOutsideInspirationResponseSchema, {
    result: { case: 'sceneCheck', value: create(RollSceneCheckResponseSchema, { roll: used }) },
  });

  function setup() {
    Element.prototype.scrollTo = vi.fn() as never;
    const api = new FakeSceneClient();
    api.offer = inspirationOffer();
    const resources = { answerOutsideInspiration: vi.fn().mockResolvedValue(answered) };
    const data: SceneRollSheetData = {
      campaignId: 'c1',
      action: playerScene().actions[0],
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: new SceneState(
        () => api.get(),
        () => false,
      ),
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: SceneClient, useValue: api },
        { provide: ResourceClient, useValue: resources },
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
    const button = (name: string) =>
      Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
        b.textContent?.includes(name),
      );
    return { el, button, settle, resources };
  }

  it('asks about the die after the d20 and shows no result until it is answered', async () => {
    const { el, button, settle } = setup();
    button('Rolar no app')!.click();
    await settle();
    expect(el.textContent).toContain('Usar a Inspiração de Bardo (d8)?');
    expect(el.textContent).toContain('Dada por Orla');
    expect(el.textContent).not.toContain('Seu total em');
    expect(button('Guardar o dado')).toBeTruthy();
  });

  it('uses the die rolled in the app and then shows the total with the die in the formula', async () => {
    const { el, button, settle, resources } = setup();
    button('Rolar no app')!.click();
    await settle();
    button('Usar e rolar o d8')!.click();
    await settle();
    expect(resources.answerOutsideInspiration).toHaveBeenCalledWith(
      'c1',
      'h1',
      true,
      { inApp: true },
      expect.any(String),
    );
    expect(el.textContent).toContain('1d20 (11) + 6 + d8 (5) = 22');
    expect(el.textContent).toContain('Voltar à cena');
  });

  it('keeps the die when the player says so', async () => {
    const { button, settle, resources } = setup();
    button('Rolar no app')!.click();
    await settle();
    button('Guardar o dado')!.click();
    await settle();
    expect(resources.answerOutsideInspiration).toHaveBeenCalledWith(
      'c1',
      'h1',
      false,
      null,
      expect.any(String),
    );
  });
});
