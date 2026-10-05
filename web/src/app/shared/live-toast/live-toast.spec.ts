import { TestBed } from '@angular/core/testing';

import { LiveToast } from './live-toast';

describe('LiveToast', () => {
  it('draws a toast in a polite live region with a 44 px close that dismisses it', () => {
    const fixture = TestBed.createComponent(LiveToast);
    fixture.componentRef.setInput('toasts', [{ id: 7, icon: 'visibility', title: 'Você notou uma armadilha.', text: 'Fosso escondido, no mapa.' }]);
    const gone: number[] = [];
    fixture.componentInstance.dismissed.subscribe((id) => gone.push(id));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('[role=status][aria-live=polite]')?.textContent).toContain('Você notou uma armadilha.');
    el.querySelector<HTMLButtonElement>('button[aria-label="Dispensar o aviso"]')!.click();
    expect(gone).toEqual([7]);
  });

  it('keeps the live region when there is no toast, so a screen reader hears the next one', () => {
    const fixture = TestBed.createComponent(LiveToast);
    fixture.componentRef.setInput('toasts', []);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('[role=status]')).toBeTruthy();
  });
});
