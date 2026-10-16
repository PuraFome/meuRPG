import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  LuckyOfferSchema,
  OutsideInspirationOfferSchema,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import {
  CheckOptionSchema,
  GroupCheckMemberViewSchema,
} from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { RollGroupCheckResponseSchema } from '../../../../gen/meurpg/play/v1/contests_pb';
import {
  AnswerOutsideInspirationResponseSchema,
  AnswerOutsideLuckyResponseSchema,
} from '../../../../gen/meurpg/play/v1/resources_pb';
import { ContestClient } from '../../../core/combat/contest-client';
import {
  FakeContestClient,
  checkRoll,
  groupCheck,
  textOf,
} from '../../../core/combat/contest-testing';
import { GroupCheckState } from '../../../core/combat/group-check-state';
import { LUCKY_TRAIT } from '../../../core/combat/lucky';
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
const lucky = create(LuckyOfferSchema, {
  holdId: 'h1',
  luckyIndex: 0,
  d20: { diceCount: 1, diceSides: 20, faces: [1], modifier: 7, total: 8 },
});

/** The check after an answer: the player's own roll with the new d20, waiting for the master. */
const writtenCheck = (to: number) =>
  create(RollGroupCheckResponseSchema, {
    groupCheck: groupCheck({
      members: [
        member({
          answered: true,
          roll: checkRoll({
            skill: 0,
            skillKey: 'skill:stealth',
            faces: [to],
            modifier: 7,
            total: to + 7,
            rerolled: [{ index: 0, from: 1, to, sourceKey: LUCKY_TRAIT }],
          }),
        }),
      ],
    }),
  });

const answered = (to: number) =>
  create(AnswerOutsideLuckyResponseSchema, {
    result: { case: 'groupCheck', value: writtenCheck(to) },
  });

describe('GroupCheckSheet: the halfling’s Lucky question', () => {
  let api: FakeContestClient;
  const answerOutsideLucky = vi.fn();
  const answerOutsideInspiration = vi.fn();

  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
    answerOutsideLucky.mockReset();
    answerOutsideInspiration.mockReset();
  });

  function setup() {
    api = new FakeContestClient();
    api.lucky = lucky;
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
        {
          provide: ResourceClient,
          useValue: { answerOutsideLucky, answerOutsideInspiration },
        },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: false } },
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
    return { el, button, settle, text: () => textOf(el), state };
  }

  it('asks after a d20 of 1, on the sheet, before anything is written', async () => {
    const { button, settle, text, state } = setup();
    button('Rolar no app')!.click();
    await settle();
    expect(text()).toContain('Tirou 1: rolar de novo?');
    expect(text()).toContain('Seu d20 (+7 de bônus)');
    expect(button('Ficar com o 1')).toBeTruthy();
    expect(text()).not.toContain('Esperando o mestre');
    expect(state.own()?.answered).toBe(false);
  });

  it('"Rolar de novo" writes the check with the new d20 and says "Sortudo: 1 → 14"', async () => {
    const { button, settle, text, state } = setup();
    answerOutsideLucky.mockResolvedValue(answered(14));
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    expect(answerOutsideLucky).toHaveBeenCalledWith(
      'c',
      'h1',
      true,
      { inApp: true },
      expect.any(String),
    );
    expect(state.own()?.answered).toBe(true);
    expect(text()).toContain('Sortudo: 1 → 14');
    expect(text()).toContain('Esperando o mestre');
    expect(text()).not.toContain('Tirou 1: rolar de novo?');
  });

  it('after the new d20, a Bardic Inspiration die is asked about on the same sheet', async () => {
    const { button, settle, text, state } = setup();
    answerOutsideLucky.mockResolvedValue(
      create(AnswerOutsideLuckyResponseSchema, {
        result: {
          case: 'groupCheck',
          value: create(RollGroupCheckResponseSchema, {
            inspirationOffer: create(OutsideInspirationOfferSchema, {
              holdId: 'h1',
              sides: 6,
              fromName: 'Orla',
              d20: {
                diceCount: 1,
                diceSides: 20,
                faces: [9],
                modifier: 7,
                total: 16,
                rerolled: [{ index: 0, from: 1, to: 9, sourceKey: LUCKY_TRAIT }],
              },
            }),
          }),
        },
      }),
    );
    answerOutsideInspiration.mockResolvedValue(
      create(AnswerOutsideInspirationResponseSchema, {
        result: { case: 'groupCheck', value: writtenCheck(9) },
      }),
    );
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    expect(text()).toContain('Usar a Inspiração de Bardo (d6)?');
    expect(text()).toContain('Sortudo: 1 → 9');
    button('Guardar o dado')!.click();
    await settle();
    expect(answerOutsideInspiration).toHaveBeenCalledWith(
      'c',
      'h1',
      false,
      null,
      expect.any(String),
    );
    expect(state.own()?.answered).toBe(true);
  });

  it('keeps the key of an answer that failed, so a retry never rolls the d20 twice', async () => {
    const { button, settle, text } = setup();
    answerOutsideLucky.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    button('Rolar no app')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    button('Rolar de novo')!.click();
    await settle();
    const keys = answerOutsideLucky.mock.calls.map((c) => c[4] as string);
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
    expect(text()).toContain('Tirou 1: rolar de novo?');
  });
});
