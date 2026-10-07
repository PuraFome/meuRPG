import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { SceneBlockedReason, SceneBlockedSchema } from '../../../../../gen/meurpg/play/v1/scene_pb';
import { SceneClient } from '../../../../core/play/scene-client';
import { SceneState } from '../../../../core/play/scene-state';
import { FakeSceneClient, playerScene, sceneAction } from '../../../../core/play/scene-testing';
import { SceneRollSheet, type SceneRollSheetData } from './scene-roll-sheet';

describe('SceneRollSheet', () => {
  let api: FakeSceneClient;
  let close: ReturnType<typeof vi.fn>;

  // jsdom has no scrolling: the sheet scrolls its body to the top on an error.
  beforeEach(() => {
    Element.prototype.scrollTo = vi.fn() as never;
  });

  function setup(overrides: Partial<SceneRollSheetData> = {}) {
    api = new FakeSceneClient();
    close = vi.fn();
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
      ...overrides,
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: SceneClient, useValue: api },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close } },
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
    const type = (text: string) => {
      const field = el.querySelector<HTMLInputElement>('input')!;
      field.value = text;
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
    };
    return { fixture, el, button, type, settle, state };
  }

  it("shows the check and the player's own bonus, with the app's roll as the one filled button", () => {
    const { el, button } = setup();
    expect(el.querySelector('h2')?.textContent).toBe('Procurar pistas na carroça');
    expect(el.textContent).toContain('Investigação');
    expect(el.textContent).toContain('bônus +6');
    expect(el.textContent?.replace(/\u00a0/g, ' ')).toContain('1d20 + 6');
    expect(el.textContent).toContain('O mestre vê o resultado e diz o que acontece.');
    expect(button('Rolar no app')?.classList).not.toContain('pick__main--quiet');
    expect(button('Digitar o resultado')?.classList).toContain('pick__link');
    // Never a DC or a pass/fail.
    expect(el.textContent).not.toMatch(/CD|passou/i);
  });

  it('says "Passou · CD 12" or "Não passou · CD 12" under the total only when the master shows the DC (RN-20)', async () => {
    const shown = setup({ action: { ...playerScene().actions[0], dc: 12 } });
    shown.button('Rolar no app')!.click();
    await shown.settle();
    api.made = { ...api.made, passed: true } as never;
    expect(shown.el.querySelector('.res__pass')).toBeNull();
    TestBed.resetTestingModule();
    const again = setup({ action: { ...playerScene().actions[0], dc: 12 } });
    api.made = { ...api.made, passed: false } as never;
    again.button('Rolar no app')!.click();
    await again.settle();
    expect(again.el.querySelector('.res__pass')?.textContent?.replace(/\u00a0/g, ' ')).toContain(
      'Não passou · CD 12',
    );
    expect(again.el.querySelector('.res__pass.mr-tag--danger mat-icon')?.textContent).toBe('close');
  });

  it('opens on the typed way when the player prefers their own dice', () => {
    const { el } = setup({ preference: DicePreference.PHYSICAL });
    expect(el.querySelector('input')).toBeNull();
    const [app, typed] = Array.from(el.querySelectorAll('button')).filter((b) =>
      b.classList.contains('pick__main'),
    );
    expect(app.classList).toContain('pick__main--quiet');
    expect(typed.classList).not.toContain('pick__main--quiet');
  });

  it('rolls in the app once, with a key made once, and shows only the total', async () => {
    const { el, button, settle } = setup();
    button('Rolar no app')!.click();
    await settle();
    expect(api.calls.filter((c) => c.startsWith('roll '))).toHaveLength(1);
    expect(api.calls[0]).toMatch(/^roll a1 app [0-9a-f-]{36}$/);
    expect(el.querySelector('.res__sum')?.textContent).toBe('17');
    expect(el.textContent?.replace(/\u00a0/g, ' ')).toContain('1d20 (11) + 6 = 17');
    expect(el.textContent).toContain('Seu total em Investigação');
    expect(el.textContent).toContain('O mestre vê o resultado.');
    expect(el.textContent).toContain('rolado no app');
    expect(el.textContent).not.toMatch(/CD|passou/i);
    // The row under the sheet is read again (it turns into "Rolada").
    expect(api.calls).toContain('get');
    expect(document.activeElement?.textContent).toContain('Voltar à cena');
  });

  it('types a die: 1 to 20, the live total, the button only valid inside', async () => {
    const { fixture, el, button, type, settle } = setup();
    button('Digitar o resultado')!.click();
    fixture.detectChanges();
    expect(el.querySelector('h2')?.textContent).toBe('Digite o resultado do dado');
    expect(el.querySelector('.type__bonus')?.textContent?.replace(/\u00a0/g, ' ')).toBe(
      '+ 6 de bônus',
    );
    type('27');
    expect(el.querySelector('[role="alert"]')?.textContent?.replace(/\u00a0/g, ' ')).toContain(
      'Digite um número de 1 a 20',
    );
    expect(button('Confirmar')?.getAttribute('aria-disabled')).toBe('true');
    type('0');
    expect(el.querySelector('[role="alert"]')).not.toBeNull();
    type('11');
    expect(el.querySelector('.type__sum')?.textContent?.replace(/\u00a0/g, ' ')).toContain(
      '11 + 6 = 17 · dado físico',
    );
    button('Confirmar 11')!.click();
    await settle();
    expect(api.calls[0]).toMatch(/^roll a1 11 /);
  });

  it('hides the way the campaign does not allow', () => {
    expect(setup({ diceMode: DiceMode.APP }).el.textContent).not.toContain('Digitar o resultado');
    TestBed.resetTestingModule();
    const typedOnly = setup({ diceMode: DiceMode.PHYSICAL }).el;
    expect(typedOnly.textContent).not.toContain('Rolar no app');
    expect(typedOnly.querySelector('input')).not.toBeNull();
  });

  it('says why a roll was refused, from the typed reason', async () => {
    const { el, button, settle } = setup();
    api.failWith = new ConnectError('x', Code.FailedPrecondition, undefined, [
      {
        desc: SceneBlockedSchema,
        value: create(SceneBlockedSchema, { reason: SceneBlockedReason.ALREADY_ROLLED }),
      },
    ]);
    button('Rolar no app')!.click();
    await settle();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain(
      'Você não tem mais tentativas nessa ação',
    );
    expect(el.querySelector('.res')).toBeNull();
  });

  it('resends the same key when a lost answer is tried again', async () => {
    const { el, button, settle } = setup();
    api.failWith = new ConnectError('x', Code.Unavailable);
    button('Rolar no app')!.click();
    await settle();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('o servidor não respondeu');
    api.failWith = null;
    button('Rolar no app')!.click();
    await settle();
    const keys = api.calls.filter((c) => c.startsWith('roll ')).map((c) => c.split(' ')[3]);
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
  });

  it('closes with whether a roll was made', async () => {
    const { el, button, settle } = setup();
    el.querySelector<HTMLButtonElement>('.frame__close')!.click();
    expect(close).toHaveBeenLastCalledWith(false);
    button('Rolar no app')!.click();
    await settle();
    button('Voltar à cena')!.click();
    expect(close).toHaveBeenLastCalledWith(true);
  });

  it('names a check with no name of its own by the check, and the kind under it', () => {
    const { el } = setup({
      action: sceneAction('a4', 'Percepção', { bonus: -1, key: 'skill:perception' }),
    });
    expect(el.querySelector('h2')?.textContent).toBe('Percepção');
    expect(el.querySelector('.frame__sub')?.textContent?.replace(/\u00a0/g, ' ')).toBe(
      'Perícia · bônus −1',
    );
    expect(el.textContent?.replace(/\u00a0/g, ' ')).toContain('1d20 − 1');
  });
});
