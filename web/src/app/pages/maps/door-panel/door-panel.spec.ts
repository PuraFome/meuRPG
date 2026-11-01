import { TestBed } from '@angular/core/testing';

import { DoorPanel } from './door-panel';

describe('DoorPanel: the question when someone stands there', () => {
  it('keeps the notice without a live role, and the focused title points at its text', () => {
    const fixture = TestBed.createComponent(DoorPanel);
    fixture.componentRef.setInput('standing', ['Toren']);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const notice = el.querySelector('.mr-notice--warning')!;
    expect(notice.hasAttribute('role')).toBe(false);
    const title = el.querySelector('app-map-ask h3')!;
    expect(title.getAttribute('aria-describedby')).toBe(notice.id);
    expect(notice.id).not.toBe('');
  });
});
