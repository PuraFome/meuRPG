import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { FavorNotice } from './favor-notice';

function mount(on: boolean) {
  const fixture = TestBed.createComponent(FavorNotice);
  fixture.componentRef.setInput('on', on);
  fixture.detectChanges();
  return fixture;
}

describe('FavorNotice', () => {
  it('keeps its polite live region in the page with nothing in it', () => {
    const el = mount(false).nativeElement as HTMLElement;
    const live = el.querySelector('[role=status]');
    expect(live?.getAttribute('aria-live')).toBe('polite');
    expect((live?.textContent ?? '').trim()).toBe('');
  });

  it('says the master gave it, and what to do', () => {
    const el = mount(true).nativeElement as HTMLElement;
    expect(el.querySelector('[role=status]')?.textContent).toContain(
      'O mestre deu Inspiração a você. Use antes de rolar um d20 seu.',
    );
  });

  it('is dismissed by its close button', () => {
    const fixture = mount(true);
    const dismissed = vi.fn();
    fixture.componentInstance.dismissed.subscribe(dismissed);
    const button = (fixture.nativeElement as HTMLElement).querySelector<HTMLButtonElement>(
      'button.close',
    )!;
    expect(button.getAttribute('aria-label')).toBe('Dispensar o aviso');
    button.click();
    expect(dismissed).toHaveBeenCalledTimes(1);
  });
});
