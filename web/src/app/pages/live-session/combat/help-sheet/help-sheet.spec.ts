import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { CombatantKind, CombatantSide } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ContestBlockedReason,
  ContestBlockedSchema,
  HelpKind,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { CombatState } from '../../../../core/combat/combat-state';
import { ContestClient } from '../../../../core/combat/contest-client';
import { FakeContestClient, textOf } from '../../../../core/combat/contest-testing';
import type { HelpAlly, HelpTarget } from '../../../../core/combat/help-view';
import { HelpSheet, type HelpSheetData } from './help-sheet';

const alien = combatant({ id: 'h', label: 'Hobgoblin' });
const goblin = combatant({ id: 'g', label: 'Goblin 1' });
const toren: HelpAlly = { id: 'v', label: 'Toren', who: { label: 'Toren', player: true } };
const tavo: HelpAlly = { id: 'a', label: 'Tavo', who: { label: 'Tavo', player: true } };
const near: HelpTarget = {
  id: 'h',
  label: 'Hobgoblin',
  who: { label: 'Hobgoblin', player: false },
  distanceFt: 5,
  reachable: true,
};
const far: HelpTarget = {
  id: 'g',
  label: 'Goblin 1',
  who: { label: 'Goblin 1', player: false },
  distanceFt: 15,
  reachable: false,
};

describe('HelpSheet (board W7-Xc 9)', () => {
  let api: FakeContestClient;
  let close: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(over: Partial<HelpSheetData> = {}) {
    api = new FakeContestClient();
    close = vi.fn();
    const state = new CombatState();
    state.encounter.set(
      encounter({
        combatants: [
          combatant({
            id: 'o',
            label: 'Orla',
            kind: CombatantKind.PLAYER,
            side: CombatantSide.PARTY,
          }),
          alien,
          goblin,
        ],
      }),
    );
    api.encounterAnswer = state.encounter()!;
    const data: HelpSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      helperId: 'o',
      allies: [tavo, toren],
      targets: [near, far],
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
    const fixture = TestBed.createComponent(HelpSheet);
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
    return { fixture, el, button, settle, text: () => textOf(el) };
  }

  it('starts on the ally, with "Ajudar em um teste" and the first ally chosen', () => {
    const { el, text } = setup();
    expect(textOf(el.querySelector('h2')!)).toBe('Ajudar em um teste');
    expect(text()).toContain('Ação');
    expect(text()).toContain('Tavo');
    expect(text()).toContain('Toren');
    expect(el.querySelector<HTMLInputElement>('input[name="help-ally"]')?.checked).toBe(true);
    expect(Array.from(el.querySelectorAll('.steps__name')).map((n) => n.textContent)).toEqual([
      'Aliado',
      'Tarefa',
    ]);
  });

  it('lists the tasks for the chosen ally, each saying the advantage, and keeps the button off until one is chosen', async () => {
    const { el, button, settle, text } = setup();
    button('Continuar')!.click();
    await settle();
    expect(text()).toContain('Tavo · Percepção Vantagem no próximo teste de Percepção dele.');
    expect(text()).toContain('Tavo · Arcanismo');
    expect(text()).toContain(
      'Vale até o próximo teste de Tavo para essa tarefa, ou até o início do seu próximo turno.',
    );
    expect(button('Ajudar Tavo')!.getAttribute('aria-disabled')).toBe('true');
    button('Ajudar Tavo')!.click();
    await settle();
    expect(api.helps).toEqual([]);
    expect(el.querySelectorAll('input[name="help-task"]').length).toBe(18);
  });

  it('helps with a task: the request has the ally and the check’s key, under one key', async () => {
    const { el, button, settle, text } = setup();
    button('Continuar')!.click();
    await settle();
    const perception = Array.from(
      el.querySelectorAll<HTMLInputElement>('input[name="help-task"]'),
    ).find((r) => textOf(r.closest('label')!).startsWith('Tavo · Percepção'))!;
    perception.click();
    await settle();
    button('Ajudar Tavo')!.click();
    await settle();
    expect(api.helps[0].input).toEqual({
      campaignId: 'c',
      encounterId: 'enc',
      helperId: 'o',
      kind: HelpKind.CHECK,
      allyId: 'a',
      taskKey: 'skill:perception',
      targetId: '',
    });
    expect(api.helps[0].key).toMatch(/^[0-9a-f-]{36}$/);
    expect(text()).toContain('Você ajudou Tavo. Ele terá vantagem no próximo teste de Percepção.');
    button('Fechar')!.click();
    expect(close).toHaveBeenCalledWith(true);
  });

  it('switches to "Ajudar um ataque": the target within 1,5 m is chosen, the farther one is refused with the reason', async () => {
    const { el, button, settle, text } = setup();
    // The kind is a segmented choice.
    Array.from(el.querySelectorAll<HTMLInputElement>('app-segmented input'))
      .find((i) => i.value === 'attack')!
      .click();
    await settle();
    expect(textOf(el.querySelector('h2')!)).toBe('Ajudar um ataque');
    button('Continuar')!.click();
    await settle();
    expect(text()).toContain(
      'O alvo precisa estar a até 1,5 m de você. O primeiro ataque de Tavo contra ele terá vantagem.',
    );
    expect(text()).toContain('Tavo ataca o Hobgoblin O Hobgoblin está a 1,5 m de você.');
    expect(text()).toContain(
      'Tavo ataca o Goblin 1 A 4,5 m de você. Longe demais: no máximo 1,5 m.',
    );
    const radios = el.querySelectorAll<HTMLInputElement>('input[name="help-target"]');
    expect(radios[0].checked).toBe(true);
    expect(radios[1].disabled).toBe(true);
    button('Ajudar Tavo')!.click();
    await settle();
    expect(api.helps[0].input).toMatchObject({
      kind: HelpKind.ATTACK,
      allyId: 'a',
      targetId: 'h',
      taskKey: '',
    });
    expect(text()).toContain(
      'Você ajudou Tavo. O primeiro ataque dele contra o Hobgoblin terá vantagem.',
    );
  });

  it('goes back to the ally from the task', async () => {
    const { el, button, settle } = setup();
    button('Continuar')!.click();
    await settle();
    button('Voltar')!.click();
    await settle();
    expect(el.querySelector('input[name="help-ally"]')).toBeTruthy();
  });

  it('says what the server refused, and stays on the list', async () => {
    const { el, button, settle, text } = setup();
    button('Continuar')!.click();
    await settle();
    el.querySelectorAll<HTMLInputElement>('input[name="help-task"]')[0].click();
    await settle();
    api.error = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: ContestBlockedSchema,
        value: create(ContestBlockedSchema, { reason: ContestBlockedReason.NOT_AN_ALLY }),
      },
    ]);
    button('Ajudar Tavo')!.click();
    await settle();
    expect(text()).toContain('Só dá para ajudar um aliado que ainda está de pé.');
    expect(button('Ajudar Tavo')).toBeTruthy();
  });

  it('says so when there is nobody to help', () => {
    const { text } = setup({ allies: [] });
    expect(text()).toContain('Não há nenhum aliado para ajudar.');
  });
});
