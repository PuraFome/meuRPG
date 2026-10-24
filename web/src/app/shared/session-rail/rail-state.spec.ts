import { TestBed } from '@angular/core/testing';

import { RailState } from './rail-state';

describe('RailState', () => {
  function setup(inputs: Record<string, unknown>) {
    const fixture = TestBed.createComponent(RailState);
    for (const [key, value] of Object.entries(inputs)) {
      fixture.componentRef.setInput(key, value);
    }
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('says it is loading in a polite status', () => {
    const { el } = setup({ kind: 'loading', text: 'Lendo a conversa…' });
    expect(el.querySelector('[role="status"]')?.textContent).toBe('Lendo a conversa…');
    expect(el.querySelector('.rs__ring')?.getAttribute('aria-hidden')).toBe('true');
  });

  it('says what failed in an alert, with the bold sentence and "Tentar de novo"', () => {
    const { fixture, el } = setup({
      kind: 'error',
      text: 'Não deu para carregar a conversa.',
      more: 'Confira a conexão.',
    });
    const alert = el.querySelector('[role="alert"]');
    expect(alert?.querySelector('strong')?.textContent).toBe('Não deu para carregar a conversa.');
    expect(alert?.textContent).toContain('Confira a conexão.');
    const retried = vi.fn();
    fixture.componentInstance.retry.subscribe(retried);
    el.querySelector<HTMLButtonElement>('button')!.click();
    expect(retried).toHaveBeenCalledTimes(1);
    expect(el.querySelector('button')?.textContent).toContain('Tentar de novo');
  });

  it('says the panel is empty with an icon and the sentence', () => {
    const { el } = setup({
      kind: 'empty',
      text: 'A conversa começa quando o mestre narrar a cena.',
    });
    expect(el.querySelector('.rs__text')?.textContent).toContain('A conversa começa');
    expect(el.querySelector('mat-icon')?.textContent).toBe('chat_bubble_outline');
    expect(el.querySelector('[role="alert"], [role="status"]')).toBeNull();
  });
});
