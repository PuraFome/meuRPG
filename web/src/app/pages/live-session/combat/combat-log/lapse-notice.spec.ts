import { TestBed } from '@angular/core/testing';

import { LapseNoticeCard } from './lapse-notice';

describe('LapseNoticeCard', () => {
  function make(text: string) {
    const fixture = TestBed.createComponent(LapseNoticeCard);
    fixture.componentRef.setInput('text', text);
    fixture.detectChanges();
    return fixture;
  }

  it('keeps a polite status region there before and after the words, with the words in a notice with an icon', () => {
    const fixture = make('');
    const el = fixture.nativeElement as HTMLElement;
    const live = el.querySelector('[role="status"]')!;
    expect(live.getAttribute('aria-live')).toBe('polite');
    expect(live.textContent?.trim()).toBe('');
    fixture.componentRef.setInput(
      'text',
      'A sua concentração acabou. O Clarão na Lâmina acabou sem golpe.',
    );
    fixture.detectChanges();
    const notice = live.querySelector('[data-testid="lapse-notice"]')!;
    expect(notice.textContent).toContain('A sua concentração acabou.');
    expect(notice.querySelector('mat-icon[aria-hidden="true"]')).not.toBeNull();
  });

  it('says it was understood with the button, and nothing when there is no text', () => {
    const fixture = make('x');
    const out: number[] = [];
    fixture.componentInstance.dismiss.subscribe(() => out.push(1));
    const button = (fixture.nativeElement as HTMLElement).querySelector('button')!;
    expect(button.textContent?.trim()).toBe('Entendi');
    button.click();
    expect(out).toEqual([1]);
    fixture.componentRef.setInput('text', '');
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).querySelector('button')).toBeNull();
  });
});
