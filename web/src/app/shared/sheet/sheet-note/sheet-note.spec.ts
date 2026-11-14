import { TestBed } from '@angular/core/testing';

import { SheetNote } from './sheet-note';

describe('SheetNote', () => {
  it('puts the icon before the words and mutes them by default', () => {
    const fixture = TestBed.createComponent(SheetNote);
    fixture.componentRef.setInput('icon', 'block');
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    expect(el.querySelector('mat-icon')!.textContent).toBe('block');
    expect(el.classList.contains('sheet-note--muted')).toBe(true);
  });

  it('keeps the ink when not muted', () => {
    const fixture = TestBed.createComponent(SheetNote);
    fixture.componentRef.setInput('icon', 'info');
    fixture.componentRef.setInput('muted', false);
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).classList.contains('sheet-note--muted')).toBe(
      false,
    );
  });
});
