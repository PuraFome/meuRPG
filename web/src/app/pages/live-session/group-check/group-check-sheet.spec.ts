import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CheckOptionSchema,
  GroupCheckMemberViewSchema,
} from '../../../../gen/meurpg/play/v1/contest_types_pb';
import { ContestClient } from '../../../core/combat/contest-client';
import {
  FakeContestClient,
  checkRoll,
  groupCheck,
  textOf,
} from '../../../core/combat/contest-testing';
import { GroupCheckState } from '../../../core/combat/group-check-state';
import { GroupCheckSheet, type GroupCheckSheetData } from './group-check-sheet';

const member = (over: object = {}) =>
  create(GroupCheckMemberViewSchema, { characterId: 'b', name: 'Brisa', ...over });

const asking = () =>
  groupCheck({
    youRoll: true,
    yourOption: create(CheckOptionSchema, { modifier: 7, known: true }),
    members: [member()],
  });

describe('GroupCheckSheet (board W7-Xc 10)', () => {
  let api: FakeContestClient;
  let close: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(view = asking(), over: Partial<GroupCheckSheetData> = {}) {
    api = new FakeContestClient();
    close = vi.fn();
    const state = new GroupCheckState();
    state.apply(view);
    const data: GroupCheckSheetData = {
      campaignId: 'c',
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state,
      ...over,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: ContestClient, useValue: api.as() },
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
    return { fixture, el, button, settle, text: () => textOf(el), state };
  }

  it('says what the master asked, the player’s own bonus and the two ways to roll', () => {
    const { el, text, button } = setup();
    expect(textOf(el.querySelector('h2')!)).toBe('Teste em grupo');
    expect(text()).toContain('Furtividade');
    expect(text()).toContain('O mestre pediu um teste de Furtividade de todo o grupo.');
    expect(text()).toContain('Seu teste: +7.');
    expect(button('Rolar no app')).toBeTruthy();
    expect(button('Digitar o resultado')).toBeTruthy();
    expect(Array.from(el.querySelectorAll('.steps__name')).map((n) => n.textContent)).toEqual([
      'Rolar',
      'Resultado',
    ]);
    // Never the DC.
    expect(text()).not.toMatch(/CD/);
  });

  it('rolls the own d20 once, under one key, and waits for the master', async () => {
    const { el, button, settle, text, state } = setup();
    api.group = groupCheck({
      youRoll: false,
      members: [
        member({
          answered: true,
          roll: checkRoll({
            skill: 0,
            skillKey: 'skill:stealth',
            faces: [12],
            modifier: 7,
            total: 19,
          }),
        }),
      ],
    });
    button('Rolar no app')!.click();
    await settle();
    expect(api.groupRolls).toHaveLength(1);
    expect(api.groupRolls[0].die).toEqual({ inApp: true });
    expect(api.groupRolls[0].key).toMatch(/^[0-9a-f-]{36}$/);
    expect(el.querySelector('.roll__box')?.textContent).toBe('12');
    expect(el.querySelector('.roll__total')?.textContent).toBe('19');
    expect(textOf(el.querySelector('.roll__formula')!)).toBe('1d20 (12) + 7');
    expect(textOf(el.querySelector('[data-testid="group-wait"]')!)).toBe(
      'Esperando o mestre. O resultado do grupo aparece quando ele encerrar o teste.',
    );
    expect(button('Fechar a folha')).toBeTruthy();
    expect(state.own()?.answered).toBe(true);
    expect(text()).not.toMatch(/passou|falhou/i);
  });

  it('sends the typed d20', async () => {
    const { el, button, settle } = setup();
    button('Digitar o resultado')!.click();
    await settle();
    const field = el.querySelector<HTMLInputElement>('input[type="text"]')!;
    field.value = '12';
    field.dispatchEvent(new Event('input'));
    await settle();
    button('Confirmar')!.click();
    await settle();
    expect(api.groupRolls[0].die).toEqual({ faces: [12] });
  });

  it('says "Passou" and the group’s verdict only when the master showed the DC and closed the check', () => {
    const closed = groupCheck({
      open: false,
      showDc: true,
      verdictKnown: true,
      groupPassed: false,
      members: [
        member({
          answered: true,
          roll: checkRoll({ faces: [12], modifier: 7, total: 19 }),
          passedKnown: true,
          passed: true,
        }),
      ],
    });
    const { text } = setup(closed);
    expect(text()).toContain('Passou');
    expect(text()).toContain('O grupo falhou.');
  });

  it('says only that the master closed it when the DC stays with the master', () => {
    const closed = groupCheck({
      open: false,
      members: [
        member({ answered: true, roll: checkRoll({ faces: [12], modifier: 7, total: 19 }) }),
      ],
    });
    const { text, el } = setup(closed);
    expect(text()).toContain('O mestre encerrou o teste.');
    expect(text()).not.toMatch(/passou|falhou/i);
    expect(el.querySelector('[data-testid="group-wait"]')).toBeNull();
  });

  it('reads the check again when the roll is refused because the check changed', async () => {
    const { button, settle, text } = setup();
    api.error = new Error('closed');
    button('Rolar no app')!.click();
    await settle();
    expect(api.calls).toContain('groupCheck');
    expect(text()).toContain('Não deu para rolar o teste em grupo');
  });

  it('hides on "Fechar a folha"', async () => {
    const { button, settle } = setup(
      groupCheck({
        members: [
          member({ answered: true, roll: checkRoll({ faces: [12], modifier: 7, total: 19 }) }),
        ],
      }),
    );
    await settle();
    button('Fechar a folha')!.click();
    expect(close).toHaveBeenCalledWith(true);
  });
});
