import { TestBed } from '@angular/core/testing';

import { FictionNotice } from './fiction-notice';

describe('FictionNotice', () => {
  it('shows the exact PRIV-21 wording, as a note', () => {
    TestBed.configureTestingModule({ imports: [FictionNotice] });
    const fixture = TestBed.createComponent(FictionNotice);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;

    const note = el.querySelector('[role="note"]');
    expect(note).toBeTruthy();
    expect(note?.textContent?.trim()).toContain('É ficção: não escreva dados reais de pessoas.');
  });
});
