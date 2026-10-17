// The Bardic Inspiration die on a group check: asked on the sheet after the halfling's Lucky answer, or, with no Lucky,
// left to the session panel (the sheet closes). The die is used rolled in the app or typed, or kept.
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CheckOptionSchema,
  GroupCheckMemberViewSchema,
} from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { RollGroupCheckResponseSchema } from '../../../../gen/meurpg/play/v1/contests_pb';
import { ContestClient, InspirationAsked } from '../../../core/combat/contest-client';
import {
  FakeContestClient,
  checkRoll,
  groupCheck,
  textOf,
} from '../../../core/combat/contest-testing';
import { GroupCheckState } from '../../../core/combat/group-check-state';
import { LUCKY_TRAIT } from '../../../core/combat/lucky';
import { heldAnswer, heldDie, heldLucky } from '../../../core/held-rolls/held-roll-testing';
import { ResourceClient } from '../../../core/resources/resources-client';
import { GroupCheckSheet, type GroupCheckSheetData } from './group-check-sheet';

const member = (over: object = {}) =>
  create(GroupCheckMemberViewSchema, { characterId: 'p', name: 'Pipo', ...over });

const asking = () =>
  groupCheck({
    youRoll: true,
    yourOption: create(CheckOptionSchema, { modifier: 7, known: true }),
    members: [member()],
  });

/** The d20 of 1 a halfling rolled, kept for the Lucky question. */
const lucky = heldLucky([1], 7);

/** The die's question on the new d20 Lucky rolled (a d6 from Orla). */
const dieAfterLucky = heldAnswer({
  result: {
    case: 'groupCheck',
    value: { heldRollOffer: heldDie(9, 7, 6, { holdId: 'h1' }, 1) },
  },
});

/** The check written with the d20 and, when the die was used, its face in the roll. */
const written = (dieFace = 0) =>
  heldAnswer({
    result: {
      case: 'groupCheck',
      value: create(RollGroupCheckResponseSchema, {
        groupCheck: groupCheck({
          members: [
            member({
              answered: true,
              roll: checkRoll({
                skill: 0,
                skillKey: 'skill:stealth',
                faces: [9],
                modifier: 7,
                total: 16 + dieFace,
                rerolled: [{ index: 0, from: 1, to: 9, sourceKey: LUCKY_TRAIT }],
              }),
            }),
          ],
        }),
      }),
    },
  });

describe('GroupCheckSheet: the Bardic Inspiration die', () => {
  let api: FakeContestClient;
  const answerHeldRoll = vi.fn();
  const close = vi.fn();

  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
    answerHeldRoll.mockReset();
    close.mockReset();
  });

  function setup() {
    api = new FakeContestClient();
    const state = new GroupCheckState();
    state.apply(asking());
    const data: GroupCheckSheetData = {
      campaignId: 'c',
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: ContestClient, useValue: api.as() },
        { provide: ResourceClient, useValue: { answerHeldRoll } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
      ],
    });
    const fixture = TestBed.createComponent(GroupCheckSheet);
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
        textOf(b).includes(name),
      );
    /** Rolls the d20 in the app and answers Lucky's question, so the die's comes. */
    const toTheDie = async () => {
      api.lucky = lucky;
      answerHeldRoll.mockResolvedValueOnce(dieAfterLucky);
      button('Rolar no app')!.click();
      await settle();
      button('Rolar de novo')!.click();
      await settle();
    };
    return { el, button, settle, toTheDie, text: () => textOf(el), state, fixture };
  }

  it('leaves the question of a die held without a Lucky 1 to the session panel: the sheet closes with no error', async () => {
    const { button, settle, text } = setup();
    api.error = new InspirationAsked();
    button('Rolar no app')!.click();
    await settle();
    expect(close).toHaveBeenCalledWith(true);
    expect(text()).not.toContain('Não deu para');
    expect(answerHeldRoll).not.toHaveBeenCalled();
  });

  it('asks after Lucky, on the new d20, with nothing written yet', async () => {
    const { toTheDie, text, state, button } = setup();
    await toTheDie();
    expect(text()).toContain('Usar a Inspiração de Bardo (d6)?');
    expect(text()).toContain('Dada por Orla');
    expect(text()).not.toContain('Tirou 1: rolar de novo?');
    expect(button('Guardar o dado')).toBeTruthy();
    expect(state.own()?.answered).toBe(false);
  });

  it('"Usar e rolar o d6" uses the die rolled in the app and writes the check', async () => {
    const { toTheDie, button, settle, text, state } = setup();
    await toTheDie();
    answerHeldRoll.mockResolvedValue(written(4));
    button('Usar e rolar o d6')!.click();
    await settle();
    expect(answerHeldRoll).toHaveBeenLastCalledWith(
      'c',
      'h1',
      { kind: 'die', use: true, roll: { inApp: true } },
      expect.any(String),
    );
    expect(state.own()?.answered).toBe(true);
    expect(text()).toContain('Esperando o mestre');
    expect(text()).not.toContain('Usar a Inspiração de Bardo (d6)?');
  });

  it('with real dice the player types the d6, 1 to 6', async () => {
    const { toTheDie, button, settle, el, fixture } = setup();
    await toTheDie();
    answerHeldRoll.mockResolvedValue(written(5));
    button('Digitar o dado')!.click();
    await settle();
    const input = el.querySelector<HTMLInputElement>('input[type="number"]')!;
    expect(input.max).toBe('6');
    expect(button('Usar o dado digitado')!.getAttribute('aria-disabled')).toBe('true');
    input.value = '5';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    button('Usar o dado digitado')!.click();
    await settle();
    expect(answerHeldRoll).toHaveBeenLastCalledWith(
      'c',
      'h1',
      { kind: 'die', use: true, roll: { face: 5 } },
      expect.any(String),
    );
  });

  it('"Guardar o dado" keeps it: the check is written without the die', async () => {
    const { toTheDie, button, settle, state } = setup();
    await toTheDie();
    answerHeldRoll.mockResolvedValue(written());
    button('Guardar o dado')!.click();
    await settle();
    expect(answerHeldRoll).toHaveBeenLastCalledWith(
      'c',
      'h1',
      { kind: 'die', use: false, roll: null },
      expect.any(String),
    );
    expect(state.own()?.answered).toBe(true);
  });

  it('keeps the key of an answer that failed, so a retry never spends the die twice, and says why', async () => {
    const { toTheDie, button, settle, text } = setup();
    await toTheDie();
    answerHeldRoll.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    button('Guardar o dado')!.click();
    await settle();
    button('Guardar o dado')!.click();
    await settle();
    const keys = answerHeldRoll.mock.calls.slice(1).map((c) => c[3] as string);
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
    expect(text()).toContain('Usar a Inspiração de Bardo (d6)?');
    expect(text()).toContain('Não deu para responder');
  });

  it('ignores a second tap while the answer is on the way', async () => {
    const { toTheDie, button, settle } = setup();
    await toTheDie();
    let release!: (v: unknown) => void;
    answerHeldRoll.mockReturnValue(new Promise((r) => (release = r)));
    button('Guardar o dado')!.click();
    button('Guardar o dado')!.click();
    await settle();
    expect(answerHeldRoll).toHaveBeenCalledTimes(2);
    release(written());
    await settle();
  });
});
