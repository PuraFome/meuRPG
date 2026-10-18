import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { describe, expect, it, vi } from 'vitest';

import { LiveSessionSource } from '../../../core/play/live-session.types';
import { SessionHeader } from './session-header';

describe('SessionHeader ending the session', () => {
  function setup() {
    let finish!: () => void;
    const endSession = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: LiveSessionSource, useValue: { endSession } }],
    });
    const fixture = TestBed.createComponent(SessionHeader);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('campaignName', 'Torre');
    fixture.componentRef.setInput('isMaster', true);
    fixture.componentRef.setInput('session', {
      sessionId: 's1',
      sessionNumber: 4,
      startedAt: new Date(),
    });
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const button = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes(text));
    return { fixture, el, button, endSession, finish: () => finish() };
  }

  it('control: Cancelar goes back to "Encerrar sessão" before the master confirms', () => {
    const { fixture, button } = setup();
    button('Encerrar sessão')!.click();
    fixture.detectChanges();
    button('Cancelar')!.click();
    fixture.detectChanges();
    expect(button('Encerrar sessão')).toBeDefined();
  });

  it('keeps the confirmation, and Cancelar dead, while the end is on its way', async () => {
    const { fixture, button, endSession, finish } = setup();
    button('Encerrar sessão')!.click();
    fixture.detectChanges();
    button('Confirmar encerramento')!.click();
    fixture.detectChanges();
    expect(endSession).toHaveBeenCalledTimes(1);
    const cancel = button('Cancelar')!;
    expect(cancel.disabled).toBe(true);
    cancel.dispatchEvent(new Event('click'));
    fixture.detectChanges();
    expect(button('Encerrar sessão')).toBeUndefined();
    expect(button('Confirmar encerramento')).toBeDefined();
    finish();
    await fixture.whenStable();
  });
});

describe('SessionHeader the "Dados" button', () => {
  function setup(inputs: Record<string, unknown> = {}) {
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: LiveSessionSource, useValue: {} }],
    });
    const fixture = TestBed.createComponent(SessionHeader);
    fixture.componentRef.setInput('campaignId', 'c1');
    fixture.componentRef.setInput('campaignName', 'Torre');
    fixture.componentRef.setInput('session', {
      sessionId: 's1',
      sessionNumber: 6,
      startedAt: new Date(),
    });
    for (const [key, value] of Object.entries(inputs)) {
      fixture.componentRef.setInput(key, value);
    }
    const opened = vi.fn();
    fixture.componentInstance.openDice.subscribe(opened);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, el, opened, dice: () => el.querySelector<HTMLButtonElement>('.js-dice') };
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

  it('is off while the stream reconnects, and the notice says why', () => {
    const { el, dice, opened } = setup({ connection: 'reconnecting', lastUpdate: null });
    expect(dice()?.getAttribute('aria-disabled')).toBe('true');
    const notice = el.querySelector('.head__offline');
    expect(notice?.textContent).toContain('Reconectando…');
    expect(notice?.textContent).toContain('Não dá para rolar sem conexão.');
    expect(dice()?.getAttribute('aria-describedby')).toBe(notice?.querySelector('p')?.id);
    dice()!.click();
    expect(opened).not.toHaveBeenCalled();
  });
});
