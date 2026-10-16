import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { OutsideInspirationOfferSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  AnswerOutsideLuckyResponseSchema,
  ResourceBlockedReason,
  ResourceBlockedSchema,
} from '../../../../../gen/meurpg/play/v1/resources_pb';
import { RollSceneCheckResponseSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { LUCKY_TRAIT } from '../../../../core/combat/lucky';
import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import {
  FakeSceneClient,
  luckyOffer,
  playerScene,
  sceneRoll,
} from '../../../../core/play/scene-testing';
import { ResourceClient } from '../../../../core/resources/resources-client';
import { SceneRollSheet, type SceneRollSheetData } from './scene-roll-sheet';

const flat = (n: Element | null | undefined) =>
  n?.textContent?.replace(/\s+/g, ' ').trim().replace(/ /g, ' ') ?? '';

/** The scene check written after the Lucky answer: the new d20 (or the 1 kept), and what Lucky rolled again. */
function written(to: number | null) {
  const face = to ?? 1;
  return create(AnswerOutsideLuckyResponseSchema, {
    result: {
      case: 'sceneCheck',
      value: create(RollSceneCheckResponseSchema, {
        roll: sceneRoll('r1', 'a1', 'Pipo', face + 6, {
          roll: {
            diceCount: 1,
            diceSides: 20,
            faces: [face],
            modifier: 6,
            total: face + 6,
            rerolled: to === null ? [] : [{ index: 0, from: 1, to, sourceKey: LUCKY_TRAIT }],
          },
        }),
      }),
    },
  });
}

describe('SceneRollSheet: the halfling’s Lucky question, "Tirou 1: rolar de novo?"', () => {
  function setup(over: { diceMode?: DiceMode; faces?: number[] } = {}) {
    Element.prototype.scrollTo = vi.fn() as never;
    const api = new FakeSceneClient();
    api.lucky = luckyOffer(over.faces ?? [1], 6, over.faces && over.faces.length > 1 ? 1 : 0);
    const resources = {
      answerOutsideLucky: vi.fn(),
      answerOutsideInspiration: vi.fn(),
    };
    const data: SceneRollSheetData = {
      campaignId: 'c1',
      action: playerScene().actions[0],
      diceMode: over.diceMode ?? DiceMode.PLAYERS_CHOOSE,
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
        flat(b).includes(name),
      );
    const type = (value: string) => {
      const field = el.querySelector('input.type__field') as HTMLInputElement;
      field.value = value;
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
    };
    return { el, button, settle, type, api, resources };
  }

  it('asks after a d20 of 1 and before the result: the 1 is there, no total and no "passou"', async () => {
    const { el, button, settle } = setup();
    button('Rolar no app')!.click();
    await settle();
    const text = flat(el);
    expect(text).toContain('Tirou 1: rolar de novo?');
    expect(text).toContain(
      'Sortudo: você pode rolar esse d20 de novo e fica com o novo resultado.',
    );
    expect(text).toContain('Seu d20 (+6 de bônus). O resultado só aparece depois da sua escolha.');
    expect(text).not.toContain('Seu total em');
    expect(text).not.toMatch(/Passou|Não passou/);
    expect(button('Rolar de novo')).toBeTruthy();
    expect(button('Ficar com o 1')).toBeTruthy();
  });

  it('"Rolar de novo" answers with the d20 rolled in the app, then shows the total and "Sortudo: 1 → 14"', async () => {
    const { el, button, settle, resources } = setup();
    resources.answerOutsideLucky.mockResolvedValue(written(14));
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    expect(resources.answerOutsideLucky).toHaveBeenCalledWith(
      'c1',
      'h1',
      true,
      { inApp: true },
      expect.any(String),
    );
    const text = flat(el);
    expect(text).not.toContain('Tirou 1: rolar de novo?');
    expect(text).toContain('Sortudo: 1 → 14');
    expect(text).toContain('Seu total em');
    expect(button('Voltar à cena')).toBeTruthy();
  });

  it('"Ficar com o 1" keeps it: the roll is written with the 1, and nothing says Lucky rolled', async () => {
    const { el, button, settle, resources } = setup();
    resources.answerOutsideLucky.mockResolvedValue(written(null));
    button('Rolar no app')!.click();
    await settle();
    button('Ficar com o 1')!.click();
    await settle();
    expect(resources.answerOutsideLucky).toHaveBeenCalledWith(
      'c1',
      'h1',
      false,
      null,
      expect.any(String),
    );
    const text = flat(el);
    expect(text).toContain('Seu total em');
    expect(text).not.toContain('Sortudo:');
  });

  it('with real dice the player types the new d20, 1 to 20', async () => {
    const { el, button, settle, type, resources } = setup({ diceMode: DiceMode.PHYSICAL });
    resources.answerOutsideLucky.mockResolvedValue(written(15));
    type('1');
    button('Confirmar 1')!.click();
    await settle();
    expect(flat(el)).toContain('Tirou 1: rolar de novo?');
    expect(flat(el)).toContain('1 a 20');
    type('15');
    button('Confirmar 15')!.click();
    await settle();
    expect(resources.answerOutsideLucky).toHaveBeenCalledWith(
      'c1',
      'h1',
      true,
      { face: 15 },
      expect.any(String),
    );
    expect(flat(el)).toContain('Sortudo: 1 → 15');
  });

  it('with advantage shows both dice and names the one that showed the 1', async () => {
    const { el, button, settle } = setup({ faces: [1, 13] });
    button('Rolar no app')!.click();
    await settle();
    const text = flat(el);
    expect(text).toContain('1 e 13');
    expect(text).toContain(
      'Sortudo: você pode rolar de novo o dado que deu 1 e fica com o novo resultado.',
    );
  });

  it('after the new d20, a Bardic Inspiration die the character holds is asked about, on the new roll', async () => {
    const { el, button, settle, resources } = setup();
    resources.answerOutsideLucky.mockResolvedValue(
      create(AnswerOutsideLuckyResponseSchema, {
        result: {
          case: 'sceneCheck',
          value: create(RollSceneCheckResponseSchema, {
            inspirationOffer: create(OutsideInspirationOfferSchema, {
              holdId: 'h1',
              sides: 8,
              fromName: 'Orla',
              d20: {
                diceCount: 1,
                diceSides: 20,
                faces: [6],
                modifier: 6,
                total: 12,
                rerolled: [{ index: 0, from: 1, to: 6, sourceKey: LUCKY_TRAIT }],
              },
            }),
          }),
        },
      }),
    );
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    const text = flat(el);
    expect(text).not.toContain('Tirou 1: rolar de novo?');
    expect(text).toContain('Usar a Inspiração de Bardo (d8)?');
    expect(text).toContain('Sortudo: 1 → 6');
    expect(text).not.toContain('Seu total em');
    expect(button('Guardar o dado')).toBeTruthy();
  });

  it('keeps the key of an answer that failed, so a retry never rolls the d20 twice', async () => {
    const { el, button, settle, resources } = setup();
    resources.answerOutsideLucky.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    const keys = resources.answerOutsideLucky.mock.calls.map((c) => c[4] as string);
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
    expect(flat(el.querySelector('[role=alert]'))).toContain('o servidor não respondeu');
    expect(flat(el)).toContain('Tirou 1: rolar de novo?');
  });

  it('says another roll waits for the Lucky answer when the server refuses with LUCKY_PENDING', async () => {
    const { el, button, settle, api } = setup();
    api.lucky = null;
    vi.spyOn(api, 'roll').mockRejectedValue(
      new ConnectError('pending', Code.FailedPrecondition, undefined, [
        {
          desc: ResourceBlockedSchema,
          value: create(ResourceBlockedSchema, { reason: ResourceBlockedReason.LUCKY_PENDING }),
        },
      ]),
    );
    button('Rolar no app')!.click();
    await settle();
    expect(flat(el.querySelector('[role=alert]'))).toContain(
      'Responda primeiro à pergunta do Sortudo',
    );
  });
});
