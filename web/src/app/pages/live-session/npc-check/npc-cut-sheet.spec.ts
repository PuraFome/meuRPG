import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { vi } from 'vitest';

import { CuttingWordsAnsweredSchema } from '../../../../gen/meurpg/play/v1/resources_pb';
import { CuttingFlow } from '../../../core/npc-check/cutting-flow';
import { cuttingOffer } from '../../../core/npc-check/npc-check-testing';
import { NpcCutSheet, type NpcCutData } from './npc-cut-sheet';

const answered = (cut: boolean, face = 4) =>
  create(CuttingWordsAnsweredSchema, { cut, usesLeft: 2, die: cut ? { diceSides: 8, faces: [face] } : undefined });

function setup(typed: boolean, answer = vi.fn(() => Promise.resolve(answered(true)))) {
  const flow = new CuttingFlow({ answer }, () => 'c1', () => 'k');
  flow.sync([cuttingOffer()]);
  const close = vi.fn();
  const data: NpcCutData = { flow, typed };
  TestBed.configureTestingModule({
    providers: [
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close, disableClose: true } },
    ],
  });
  const fixture = TestBed.createComponent(NpcCutSheet);
  const el = fixture.nativeElement as HTMLElement;
  fixture.detectChanges();
  const flush = async () => {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  const text = () => (el.textContent ?? '').replace(/\s+/g, ' ');
  const button = (words: string) => Array.from(el.querySelectorAll('button')).find((b) => (b.textContent ?? '').includes(words));
  return { fixture, el, flow, flush, text, button, answer, close };
}

describe('NpcCutSheet', () => {
  it('asks the question with what it costs and what the bard does not read', () => {
    const { el, text } = setup(false);
    expect(el.querySelector('h2')?.textContent).toBe('O Capitão bandido faz um teste');
    expect(text()).toContain('Usar Palavras de Interrupção? Você tira o seu dado de Inspiração de Bardo (d8) do teste do Capitão bandido.');
    expect(text()).toContain('Gasta 1 Inspiração de Bardo');
    expect(text()).toContain('Você tem 3');
    expect(text()).toContain('Não gasta reação');
    expect(text()).toContain('Qual é o teste, o total e a CD ficam com o mestre.');
    expect(text()).toContain('A cena espera a sua resposta. O mestre pode seguir sem ela.');
  });

  it('reads nothing of the creature\'s roll, no clock and no "Sem efeito"', () => {
    const { text, el } = setup(false);
    expect(text()).not.toMatch(/d20|CD \d|Sem efeito|\d+ s\b|segundos/);
    expect(el.querySelector('[role="timer"]')).toBeNull();
  });

  it('starts on "Deixar passar", the safe button', () => {
    const { button } = setup(false);
    expect(button('Deixar passar')?.hasAttribute('data-initial-focus')).toBe(true);
  });

  it('cuts with the app die and reads its own die and the uses left', async () => {
    const { flush, button, text, answer } = setup(false);
    button('Cortar')?.click();
    await flush();
    expect(answer).toHaveBeenCalledWith('c1', 'hold-1', { cut: true, die: { inApp: true } }, 'k');
    expect(text()).toContain('Palavras de Interrupção usadas');
    expect(text()).toContain('Você cortou: −4');
    expect(text()).toContain('Restam 2');
    expect(text()).toContain('O total do teste e o resultado ficam com o mestre, que conta na cena.');
    expect(button('Voltar à cena')).toBeTruthy();
  });

  it('with a physical die asks the face first: "Cortar" waits, then reads "Cortar com 4"', async () => {
    const { el, flush, button, text, answer } = setup(true);
    expect(text()).toContain('Número que saiu no seu d8 (1 a 8)');
    expect(button('Cortar')?.getAttribute('aria-disabled')).toBe('true');
    const field = el.querySelector<HTMLInputElement>('app-text-field input')!;
    field.value = '9';
    field.dispatchEvent(new Event('input'));
    await flush();
    expect(text()).toContain('Digite um número de 1 a 8.');
    field.value = '4';
    field.dispatchEvent(new Event('input'));
    await flush();
    button('Cortar com 4')?.click();
    await flush();
    expect(answer.mock.calls[0]).toEqual(['c1', 'hold-1', { cut: true, die: { face: 4 } }, 'k']);
  });

  it('lets it pass: nothing is rolled and the sheet closes', async () => {
    const { flush, button, answer, close } = setup(false, vi.fn(() => Promise.resolve(answered(false))));
    button('Deixar passar')?.click();
    await flush();
    expect(answer).toHaveBeenCalledWith('c1', 'hold-1', { cut: false }, 'k');
    expect(close).toHaveBeenCalledWith(true);
  });

  it('says the master went on without the answer, and only offers to close', async () => {
    const { flow, flush, button, text } = setup(false);
    flow.sync([]);
    await flush();
    expect(text()).toContain('O mestre seguiu sem a sua resposta. Nada foi gasto.');
    expect(button('Cortar')).toBeUndefined();
    expect(button('Fechar')).toBeTruthy();
  });

  it('says the question was already answered when the answer came late', async () => {
    const { flush, button, text } = setup(false, vi.fn(() => Promise.reject(new ConnectError('x', Code.FailedPrecondition))));
    button('Cortar')?.click();
    await flush();
    expect(text()).toContain('Essa pergunta já foi respondida. Nada foi gasto.');
    expect(button('Fechar')).toBeTruthy();
  });

  it('says it could not cut and keeps both answers open', async () => {
    const { el, flush, button } = setup(false, vi.fn(() => Promise.reject(new ConnectError('x', Code.Unavailable))));
    button('Cortar')?.click();
    await flush();
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('Não deu para cortar agora. Nada foi gasto. Tente de novo.');
    expect(button('Deixar passar')).toBeTruthy();
    expect(button('Cortar')).toBeTruthy();
  });

  it('names the creature "uma criatura" when the bard\'s player does not know it', () => {
    const flow = new CuttingFlow({ answer: vi.fn() }, () => 'c1');
    flow.sync([cuttingOffer({ rollerLabel: 'uma criatura' })]);
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: { flow, typed: false } },
        { provide: MatDialogRef, useValue: { close: vi.fn(), disableClose: true } },
      ],
    });
    const fixture = TestBed.createComponent(NpcCutSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('h2')?.textContent).toBe('Uma criatura faz um teste');
    expect(el.textContent).toContain('do teste da criatura');
  });
});
