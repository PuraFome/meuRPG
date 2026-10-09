import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SceneRollSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { SceneClient, type SceneDie } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, playerScene } from '../../../../core/play/scene-testing';
import { SceneRollSheet, type SceneRollSheetData } from './scene-roll-sheet';

const made = create(SceneRollSchema, {
  id: 'r1',
  actionId: 'a1',
  characterName: 'Pensantus',
  roll: { diceCount: 1, diceSides: 20, faces: [11], modifier: 6, total: 22, physical: true },
});

describe('SceneRollSheet: the d4 an effect adds to a physical roll', () => {
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(taking = 1) {
    const api = new FakeSceneClient();
    const sent: { die: SceneDie; key: string; extra: readonly number[] | undefined }[] = [];
    api.roll = async (
      _c: string,
      _a: string,
      die: SceneDie,
      key: string,
      extra?: readonly number[],
    ) => {
      sent.push({ die, key, extra });
      if (!('inApp' in die) && (extra ?? []).length !== taking) {
        throw new ConnectError(
          `the roll takes ${taking} more die(s): type their faces in extra_die_faces`,
          Code.InvalidArgument,
        );
      }
      return made;
    };
    const state = new SceneState(
      () => api.get(),
      () => false,
    );
    const data: SceneRollSheetData = {
      campaignId: 'c1',
      action: playerScene().actions[0],
      diceMode: DiceMode.PHYSICAL,
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
    const confirm = async () => {
      Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
        .find((b) => b.textContent?.includes('Confirmar'))!
        .click();
      await settle();
    };
    return { el, sent, settle, fill, confirm };
  }

  it('sends nothing extra until the server says the roll takes a die, then asks for it and sends it', async () => {
    const { el, sent, settle, fill, confirm } = setup();
    expect(el.querySelector('app-extra-dice')).toBeNull();
    fill(0, '11');
    await confirm();
    expect(sent[0]).toMatchObject({ die: { face: 11 }, extra: [] });
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Esta rolagem leva mais um d4: role-o e digite o resultado.',
    );
    expect(el.querySelector('app-extra-dice label')?.textContent).toContain('Resultado do d4');

    // The d4 is the first input of the sheet, the d20 (still typed) the second.
    await settle();
    fill(0, '3');
    await confirm();
    expect(sent).toHaveLength(2);
    expect(sent[1]).toMatchObject({ die: { face: 11 }, extra: [3] });
    expect(sent[1].key).not.toBe(sent[0].key);
    expect(el.querySelector('.res__sum')?.textContent).toBe('22');
  });

  it('holds a typed roll back while a d4 asked for is empty', async () => {
    const { el, sent, settle, fill, confirm } = setup();
    fill(0, '11');
    await confirm();
    await settle();
    fill(1, '11');
    await confirm();
    expect(sent).toHaveLength(1);
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Digite o resultado do d4 antes de confirmar.',
    );
  });
});
