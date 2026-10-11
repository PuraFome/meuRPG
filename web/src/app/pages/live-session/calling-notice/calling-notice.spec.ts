import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { TurnAlert } from '../../../core/combat/turn-alert';
import { CallingNotice } from './calling-notice';

function mount(on: boolean) {
  const fixture = TestBed.createComponent(CallingNotice);
  fixture.componentRef.setInput('on', on);
  fixture.detectChanges();
  return fixture;
}

describe('CallingNotice', () => {
  it('keeps its polite live region in the page with nothing in it', () => {
    const el = mount(false).nativeElement as HTMLElement;
    const live = el.querySelector('[role=status]');
    expect(live?.getAttribute('aria-live')).toBe('polite');
    expect((live?.textContent ?? '').trim()).toBe('');
  });

  it('says the master is calling', () => {
    const el = mount(true).nativeElement as HTMLElement;
    expect(el.querySelector('[role=status]')?.textContent).toContain(
      'O mestre está chamando você.',
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

  it('shakes while the alert shakes, and shows the still outline under reduced motion', () => {
    const fixture = mount(true);
    const alert = TestBed.inject(TurnAlert);
    const box = (fixture.nativeElement as HTMLElement).querySelector('.mr-notice')!;
    expect(box.classList.contains('mr-nudge')).toBe(false);
    alert.shaking.set(true);
    fixture.detectChanges();
    expect(box.classList.contains('mr-nudge')).toBe(true);
    alert.shaking.set(false);
    alert.highlighted.set(true);
    fixture.detectChanges();
    expect(box.classList.contains('mr-nudge--still')).toBe(true);
  });
});
