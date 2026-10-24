import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { describe, expect, it, vi } from 'vitest';

import { LiveSessionSource } from '../../../core/play/live-session.types';
import { SessionHeader } from './session-header';
import { openSessionMenu, sessionMenuItem } from './session-header-testing';

function make(inputs: Record<string, unknown>, source: unknown = {}) {
  TestBed.configureTestingModule({
    providers: [provideRouter([]), { provide: LiveSessionSource, useValue: source }],
  });
  const fixture = TestBed.createComponent(SessionHeader);
  fixture.componentRef.setInput('campaignId', 'c1');
  fixture.componentRef.setInput('campaignName', 'Torre');
  fixture.componentRef.setInput('session', {
    sessionId: 's1',
    sessionNumber: 4,
    startedAt: new Date(),
  });
  for (const [key, value] of Object.entries(inputs)) {
    fixture.componentRef.setInput(key, value);
  }
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const button = (text: string) =>
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(text));
  return { fixture, el, button };
}

describe('SessionHeader the bar', () => {
  it('is one row: the back link, "Sessão 4", "Ao vivo desde …" and "Dados"', () => {
    const { el } = make({});
    expect(el.querySelector('h1')?.textContent).toBe('Sessão 4');
    const back = el.querySelector('a.bar__back');
    expect(back?.getAttribute('aria-label')).toBe('Voltar para a campanha Torre');
    expect(back?.getAttribute('href')).toBe('/campaigns/c1');
    expect(el.querySelector('.bar__status')?.textContent).toMatch(/Ao vivo\s*desde \d\d:\d\d/);
    expect(el.querySelector('.js-dice')).not.toBeNull();
    expect(el.querySelector('.bar__menu')).toBeNull();
    expect(el.querySelector('.bar__start')).toBeNull();
  });

  it('gives the master "Iniciar combate" outside a combat, and not inside one', () => {
    const outside = make({ isMaster: true });
    expect(outside.button('Iniciar combate')).toBeDefined();
    TestBed.resetTestingModule();
    const inside = make({ isMaster: true, inCombat: true });
    expect(inside.button('Iniciar combate')).toBeUndefined();
  });

  it('asks the page to start a combat, and waits while it may not', () => {
    const { fixture, button } = make({ isMaster: true });
    const asked = vi.fn();
    fixture.componentInstance.startCombat.subscribe(asked);
    button('Iniciar combate')!.click();
    expect(asked).toHaveBeenCalledTimes(1);
    fixture.componentRef.setInput('startOff', true);
    fixture.detectChanges();
    button('Iniciar combate')!.click();
    expect(asked).toHaveBeenCalledTimes(1);
  });
});

describe('SessionHeader ending the session', () => {
  function setup() {
    let finish!: () => void;
    const endSession = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    const made = make({ isMaster: true }, { endSession });
    return { ...made, endSession, finish: () => finish() };
  }

  it('has the "Sessão" menu with the link and the end', async () => {
    const { fixture, el } = setup();
    expect(el.querySelector('.bar__menu')?.getAttribute('aria-label')).toBe('Sessão');
    openSessionMenu(el);
    fixture.detectChanges();
    expect(sessionMenuItem('Copiar link da sessão')).toBeDefined();
    expect(sessionMenuItem('Encerrar sessão')).toBeDefined();
  });

  it('asks in place with Cancelar focused, and Cancelar goes back before the master confirms', async () => {
    const { fixture, el, button } = setup();
    openSessionMenu(el);
    fixture.detectChanges();
    sessionMenuItem('Encerrar sessão')!.click();
    fixture.detectChanges();
    await fixture.whenStable();
    const ask = el.querySelector('[role="group"][data-confirm]');
    expect(ask?.textContent).toContain('Encerrar a sessão 4? Ela não reabre depois.');
    expect(document.activeElement).toBe(button('Cancelar'));
    button('Cancelar')!.click();
    fixture.detectChanges();
    expect(el.querySelector('[data-confirm]')).toBeNull();
  });

  it('keeps the confirmation, and Cancelar dead, while the end is on its way', async () => {
    const { fixture, el, button, endSession, finish } = setup();
    openSessionMenu(el);
    fixture.detectChanges();
    sessionMenuItem('Encerrar sessão')!.click();
    fixture.detectChanges();
    button('Confirmar encerramento')!.click();
    fixture.detectChanges();
    expect(endSession).toHaveBeenCalledTimes(1);
    const cancel = button('Cancelar')!;
    expect(cancel.disabled).toBe(true);
    cancel.dispatchEvent(new Event('click'));
    fixture.detectChanges();
    expect(el.querySelector('[data-confirm]')).not.toBeNull();
    expect(button('Confirmar encerramento')).toBeDefined();
    finish();
    await fixture.whenStable();
  });
});

describe('SessionHeader the "Dados" button', () => {
  function setup(inputs: Record<string, unknown> = {}) {
    const made = make(inputs);
    const opened = vi.fn();
    made.fixture.componentInstance.openDice.subscribe(opened);
    return { ...made, opened, dice: () => made.el.querySelector<HTMLButtonElement>('.js-dice') };
  }

  it.each([true, false])(
    'is there for the master and the player alike (master: %s)',
    (isMaster) => {
      const { dice } = setup({ isMaster });
      expect(dice()?.querySelector('.js-dice-text')?.textContent?.trim()).toBe('Dados');
      expect(dice()?.getAttribute('aria-disabled')).not.toBe('true');
    },
  );

  it('asks the page to open the dice when tapped', () => {
    const { dice, opened } = setup();
    dice()!.click();
    expect(opened).toHaveBeenCalledTimes(1);
  });

  it('waits while the server rolls: "Rolando…", and a tap does nothing', () => {
    const { dice, opened } = setup({ rolling: true });
    expect(dice()?.querySelector('.js-dice-text')?.textContent?.trim()).toBe('Rolando…');
    expect(dice()?.getAttribute('aria-disabled')).toBe('true');
    dice()!.click();
    expect(opened).not.toHaveBeenCalled();
  });

  it('is off while the stream reconnects, and the notice under the bar says why (the pill stays)', () => {
    const { el, dice, opened } = setup({ connection: 'reconnecting', lastUpdate: null });
    expect(dice()?.getAttribute('aria-disabled')).toBe('true');
    const notice = el.querySelector('.bar__offline');
    expect(notice?.textContent).toContain('Reconectando…');
    expect(notice?.textContent).toContain('Não dá para rolar sem conexão.');
    expect(dice()?.getAttribute('aria-describedby')).toBe(notice?.querySelector('p')?.id);
    // The pill stays: it is the stream's state the notice explains.
    expect(el.querySelector('.bar__status')).not.toBeNull();
    dice()!.click();
    expect(opened).not.toHaveBeenCalled();
  });
});
